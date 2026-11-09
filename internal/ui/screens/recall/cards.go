// Package recall is the recall area's pages over the card library: the card
// table, the library's presets and the review history. Each reads what the app
// derived before the frame — rows, facets, presets and events — and reports what
// the user chose; the app owns every write and every store query.
package recall

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// The card table's columns, by the id its layout and its sort read.
const (
	columnCheck    = "check"
	columnPrompt   = "prompt"
	columnNote     = "note"
	columnType     = "type"
	columnStatus   = "status"
	columnInterval = "interval"
	columnLapses   = "lapses"
	columnDue      = "due"
)

// CardRow is one card as the table lists it: what it asks, the note it came
// from, the type it renders through and the schedule it is on. Lost is a card
// whose block is gone, which has no prompt to show.
type CardRow struct {
	ID       core.ID
	Prompt   string
	Note     string
	Type     string
	Status   core.Status
	Interval int
	Lapses   int
	Due      time.Time
	Lost     bool
}

// Facet is one filter the left pane offers: the key that names it, how many
// cards it holds, and whether it is the one narrowing the table. Its name is
// the app's, because a type's name is library data and a status's is a word.
type Facet struct {
	Key    string
	Count  int
	Chosen bool
}

// FacetGroup is one group of filters — the card's status, its type, or where its
// text comes from — by the key that names it in the frame's language.
type FacetGroup struct {
	Key   string
	Items []Facet
}

// CardFilter is the table's narrowing by the filter pane: a status, a type or a
// link state, any of them absent. The search field adds to it rather than
// replacing it.
type CardFilter struct {
	Status *core.Status
	Type   core.ID
	Ref    *core.LinkState
}

// The facet keys the filter pane reports: the unfiltered table, a lifecycle
// state, a type, and where a card's text comes from.
const (
	facetAll = "all"
)

// Key is the facet that stands for f, in the form the pane reports it back.
func (f CardFilter) Key() string {
	switch {
	case f.Status != nil:
		return "status:" + f.Status.String()
	case f.Type != "":
		return "type:" + string(f.Type)
	case f.Ref != nil:
		return "link:" + f.Ref.String()
	}
	return facetAll
}

// ParseFilterKey is the filter a facet's key names. A key this build does not
// know is the unfiltered table, so a stale facet never hides every card.
func ParseFilterKey(key string) CardFilter {
	if rest, ok := cutKey(key, "status:"); ok {
		if status, found := core.ParseStatus(rest); found {
			return CardFilter{Status: &status}
		}
		return CardFilter{}
	}
	if rest, ok := cutKey(key, "type:"); ok && rest != "" {
		return CardFilter{Type: core.ID(rest)}
	}
	if rest, ok := cutKey(key, "link:"); ok {
		switch rest {
		case core.LinkLive.String():
			ref := core.LinkLive
			return CardFilter{Ref: &ref}
		case core.LinkFrozen.String():
			ref := core.LinkFrozen
			return CardFilter{Ref: &ref}
		case core.LinkLost.String():
			ref := core.LinkLost
			return CardFilter{Ref: &ref}
		}
	}
	return CardFilter{}
}

// cutKey splits "prefix:rest", reporting whether key starts with prefix.
func cutKey(key, prefix string) (string, bool) {
	if len(key) < len(prefix) || key[:len(prefix)] != prefix {
		return "", false
	}
	return key[len(prefix):], true
}

// pick is the facet a pass chose: a key and whether this pass chose one. The
// unfiltered table is a choice too, which is why it cannot be a zero CardFilter.
type pick struct {
	key string
	set bool
}

// CardsState is the card table's own state: the query and filter the user
// chose, the sort and the columns, and the rows chosen for a bulk action. The
// rows themselves are the app's, so the screen keeps only the choice inside
// them.
type CardsState struct {
	Query      string     // what the search field holds
	CardFilter CardFilter // what the filter pane narrows the table to
	Sort       ui.SortOrder
	Selected   int    // the row the keys are on, -1 for none
	Tag        string // the tag the bulk bar adds

	list   ui.ListState
	field  ui.Handle
	tagBox ui.Handle

	// hidden is the columns the user switched off, by the id the table knows
	// them by.
	hidden map[string]bool
	// checks are the rows chosen, one box per card; a box is the truth, so a
	// click on it needs no second write.
	checks map[core.ID]*bool
	// toggles mirror the filter pane's facets, which the app derives.
	toggles map[string]*bool
	// syntax says whether the search syntax help is open.
	syntax bool
	// narrow is the filter the top select shows while the pane is hidden, by
	// the label the select holds, and narrowSeen is the label it reported last,
	// so one choice is asked for once.
	narrow, narrowSeen string
	// pane is the filter pane's width until its divider moves.
	pane float32
}

// NewCards returns the card table on its default sort, due first.
func NewCards() CardsState {
	return CardsState{
		Selected: -1,
		Sort:     ui.SortOrder{Column: columnDue},
		pane:     design.FilterPaneWidth,
		hidden:   map[string]bool{},
		checks:   map[core.ID]*bool{},
		toggles:  map[string]*bool{},
	}
}

// Keep keeps the choice inside n rows, so a reload that drops rows leaves the
// keys inside the table.
func (s *CardsState) Keep(n int) { s.Selected = min(s.Selected, n-1) }

// Focus puts the caret in the search field.
func (s *CardsState) Focus(c ...*ui.Context) { s.field.Focus(c...) }

// ClearSelection unchecks every row, after a bulk action wrote what it chose.
func (s *CardsState) ClearSelection() {
	for _, box := range s.checks {
		*box = false
	}
}

// check is the box of one row, made the first time the row shows.
func (s *CardsState) check(id core.ID) *bool {
	box, ok := s.checks[id]
	if !ok {
		box = new(bool)
		s.checks[id] = box
	}
	return box
}

// toggle is the pressed state of one facet, mirrored from the app's answer.
func (s *CardsState) toggle(key string) *bool {
	on, ok := s.toggles[key]
	if !ok {
		on = new(bool)
		s.toggles[key] = on
	}
	return on
}

// chosenRows is the ids of the rows the user checked, in table order.
func chosenRows(rows []CardRow, s *CardsState) []core.ID {
	var ids []core.ID
	for _, row := range rows {
		if box, ok := s.checks[row.ID]; ok && *box {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

// Chosen is the rows the user checked, in table order: what a bulk action acts
// on, and what the plugin contract's browse.selection asks about. It is the
// screen's own state, so the app asks rather than guessing at the checks.
func (s *CardsState) Chosen(rows []CardRow) []core.ID { return chosenRows(rows, s) }

// CardsDeps is what the card table asks of the app: the rows and facets it
// derived, the reload a query, a filter or a sort asks for, and the writes a
// row and the bulk bar carry. Each runs on the UI thread.
type CardsDeps struct {
	// Rows are the cards the query and filter found, in the sort's order.
	Rows func() []CardRow
	// Total is how many cards the query matches, which is the page's badge.
	Total func() int
	// Facets are the filter pane's three groups, with their counts.
	Facets func() []FacetGroup
	// FacetLabel names one facet in the frame's language: a status, a type, or
	// a link state.
	FacetLabel func(key string) string
	// Reload derives the rows again: the query changed.
	Reload func()
	// Sort takes the order the user clicked, and derives the rows again.
	Sort func(ui.SortOrder)
	// ChooseFilter takes the facet the user chose, and derives the rows again.
	ChooseFilter func(CardFilter)
	// Open shows the card's note, or the card in review when its block is gone.
	Open func(core.ID)
	// Suspend, SetDue, Freeze and AddTag are the bulk writes, over the rows
	// chosen; SetDue names a day offset from today.
	Suspend func([]core.ID)
	SetDue  func([]core.ID, int)
	Freeze  func([]core.ID)
	AddTag  func([]core.ID, string)

	// Items are the browse.row slot's items, as the library load cached them,
	// asked for where the action row draws them: an item that names
	// browse.selection reads the rows chosen this frame, not the last one.
	// PluginText resolves one of a plugin's messages in the frame's language,
	// and Run answers one.
	Items      func() []contrib.MenuRef
	PluginText func(*ui.Context, string, string) string
	Run        func(*ui.Context, contrib.CommandRef)
}

// CardsView draws the card table: the search and its syntax, the filters on the
// left, the table of cards, and the bulk bar over the rows chosen.
func CardsView(c *ui.Context, s *CardsState, d CardsDeps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	rows := d.Rows()
	s.Keep(len(rows))
	before := s.Sort
	reload, open, picked := false, -1, pick{}
	chosen := chosenRows(rows, s)
	wide := stageWidth(c, tk) >= design.FilterPaneWidth+tk.TableFloor
	if s.pane <= 0 {
		s.pane = tk.FilterPaneWidth
	}

	ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.BrowseWidth).Grow(1).Gap(tk.SpaceM).Children(func() {
			pageHead(c, tk, msgs.RecallCardsTitle(), msgs.RecallCardsBody(), d.Total())

			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				field := ui.SearchField(c, &s.Query).
					Placeholder(msgs.RecallCardsSearch()).Label(msgs.RecallCardsSearch()).
					MinHeight(tk.Control).Grow(1).Bind(&s.field)
				if field.Changed() {
					reload = true
				}
				if ui.Button(c, msgs.RecallCardsSyntax()).MinHeight(tk.Control).
					Tooltip(msgs.RecallCardsSyntaxHelp()).Clicked() {
					s.syntax = !s.syntax
				}
				columnsMenu(c, s, tk, msgs)
				pluginActions(c, d, tk, msgs)
			})

			if s.syntax {
				kit.Card(c, func() {
					ui.Column(c).Padding(tk.SpaceM).Gap(tk.SpaceXS).Children(func() {
						ui.Text(c, msgs.RecallCardsSyntaxHelp()).FontSize(tk.TypeLabel).
							TextColor(tk.TextMuted)
					})
				}).FillWidth()
			}

			if len(chosen) > 0 {
				bulkBar(c, s, d, tk, msgs, chosen)
			}

			groups := d.Facets()
			if !wide {
				narrowFilter(c, s, tk, msgs, groups, d.FacetLabel, &picked)
			}
			cards := func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					if len(rows) == 0 {
						emptyCards(c, tk, msgs, s.Query != "")
						return
					}
					table(c, s, tk, msgs, rows, d, &open, tableRoom(c, wide, tk))
				})
			}
			if wide {
				// The pane is the split's first side: the user sets its width once
				// and the table keeps the rest, which is what a table needs.
				ui.Split(c, &s.pane, func() {
					ui.Column(c).Fill().Padding(tk.SpaceM).Background(tk.Surface).
						Children(func() {
							filterPane(c, s, tk, msgs, groups, d.FacetLabel, &picked)
						})
				}, cards).Grow(1).Label(msgs.RecallCardsFilters())
			} else {
				cards()
			}
		})
	})

	// The effects of this pass, once the fields have taken what was typed.
	if reload {
		d.Reload()
		c.Invalidate()
	}
	if s.Sort != before {
		d.Sort(s.Sort)
		c.Invalidate()
	}
	if picked.set {
		d.ChooseFilter(ParseFilterKey(picked.key))
		c.Invalidate()
	}
	if open >= 0 && open < len(rows) {
		d.Open(rows[open].ID)
	}
}

// pageHead is a page's title, the line under it and its count. A count of -1
// draws no badge: a page that holds no rows of its own says so in its body.
func pageHead(c *ui.Context, tk design.Tokens, title, body string, count int) {
	ui.Row(c).FillWidth().AlignItems(ui.End).Gap(tk.SpaceM).Children(func() {
		ui.Column(c).Gap(tk.SpaceXS).Grow(1).Children(func() {
			ui.Text(c, title).FontSize(tk.TypeH1).Bold()
			if body != "" {
				ui.Text(c, body).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			}
		})
		if count >= 0 {
			ui.Badge(c, strconv.Itoa(count))
		}
	})
}

// stageWidth is the room the page has beside the rail: the card table's filter
// pane only shows when the table still has its measure next to it.
func stageWidth(c *ui.Context, tk design.Tokens) float32 {
	w, _ := c.Size()
	if w <= design.BreakpointNarrow {
		return w - tk.RailCollapsed
	}
	return w - tk.RailWidth
}

// columnsMenu is the columns switch: every column of the table, ticked while it
// shows. A column the user switched off is not built, so the table gives its
// room to the others.
func columnsMenu(c *ui.Context, s *CardsState, tk design.Tokens, msgs locales.Messages) {
	ui.PopoverMenuButton(c, msgs.RecallCardsColumns(), func(m *ui.Menu) {
		for _, col := range cardColumns(tk, msgs) {
			col := col
			if col.ID == columnCheck || col.ID == columnPrompt {
				continue // the box that chooses and the prompt always show
			}
			if m.Item(col.Title).Checked(!s.hidden[col.ID]).Chosen() {
				s.hidden[col.ID] = !s.hidden[col.ID]
			}
		}
	}).MinHeight(tk.Control).Label(msgs.RecallCardsColumns())
}

// items are the slot's items, empty when the app behind the screen has none.
func (d CardsDeps) items() []contrib.MenuRef {
	if d.Items == nil {
		return nil
	}
	return d.Items()
}

// pluginActions is the browse.row slot: the actions plugins add for the rows on
// screen. A library whose plugins add none says so beside the disabled button
// rather than leaving a control that looks pressable and is not.
func pluginActions(c *ui.Context, d CardsDeps, tk design.Tokens, msgs locales.Messages) {
	label := msgs.RecallCardsActions()
	items := d.items()
	if len(items) == 0 {
		ui.Button(c, label).MinHeight(tk.Control).Disabled(true)
		ui.Text(c, msgs.RecallCardsNoActions()).FontSize(tk.TypeCaption).
			TextColor(tk.TextFaint).SingleLine()
		return
	}
	ui.PopoverMenuButton(c, label, func(m *ui.Menu) {
		for _, item := range items {
			item := item
			name := string(item.Command.Title)
			if d.PluginText != nil {
				name = d.PluginText(c, item.Plugin, name)
			}
			if m.Item(name).Chosen() && d.Run != nil {
				d.Run(c, item.CommandRef)
			}
		}
	}).MinHeight(tk.Control).Label(label)
}

// filterPane is the left column: the three groups of filters, each row a name
// and its count, and the chosen one pressed. A row is ui.ToggleBase — the
// lookless toggle carries the pressed state, its keyboard and its name, which a
// hand-built button row would have to rebuild (checked against MyGo v0.8.1:
// ui.Sidebar would take the focus the table needs, and ui.ToggleGroup joins its
// toggles as one segmented control, which a list of counts is not).
func filterPane(c *ui.Context, s *CardsState, tk design.Tokens, msgs locales.Messages, groups []FacetGroup, label func(string) string, picked *pick) {
	ui.Column(c).Padding(tk.SpaceM).Gap(tk.SpaceL).Children(func() {
		ui.Text(c, msgs.RecallCardsFilters()).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted).SingleLine()
		for _, group := range groups {
			ui.Column(c).Gap(tk.SpaceXS).Children(func() {
				ui.Text(c, groupTitle(msgs, group.Key)).FontSize(tk.TypeCaption).
					TextColor(tk.TextFaint).SingleLine()
				for _, item := range group.Items {
					facetRow(c, s, tk, item, label, picked)
				}
			})
		}
	})
}

// groupTitle names one filter group in the frame's language.
func groupTitle(msgs locales.Messages, key string) string {
	switch key {
	case "status":
		return msgs.RecallFilterStatus()
	case "type":
		return msgs.RecallFilterType()
	case "link":
		return msgs.RecallFilterLink()
	}
	return ""
}

// facetRow is one filter: its name, its count, and the press that narrows the
// table to it — or widens it back when it is the chosen one.
func facetRow(c *ui.Context, s *CardsState, tk design.Tokens, item Facet, label func(string) string, picked *pick) {
	name := label(item.Key)
	on := s.toggle(item.Key)
	*on = item.Chosen
	row := ui.ToggleBase(c, on).Label(name).
		MinHeight(tk.Hit).Padding(tk.SpaceXS, tk.SpaceS).Radius(tk.RadiusControl).
		Justify(ui.Start)
	if item.Chosen {
		row.Background(tk.Accent.Alpha(0.10))
	} else if row.Hovered() {
		row.Background(tk.SurfaceHover)
	}
	key := item.Key
	row.OnClick(func() {
		if item.Chosen {
			*picked = pick{key: facetAll, set: true}
			return
		}
		*picked = pick{key: key, set: true}
	})
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
			ui.Text(c, name).FontSize(tk.TypeUI).SingleLine()
			ui.Spacer(c)
			ui.Text(c, strconv.Itoa(item.Count)).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted)
		})
	})
}

// narrowFilter is the filter row at the top of a narrow window, where the pane
// has no room beside the table: one select over the same facets.
func narrowFilter(c *ui.Context, s *CardsState, tk design.Tokens, msgs locales.Messages, groups []FacetGroup, facetLabel func(string) string, picked *pick) {
	labels := []string{msgs.RecallCardsAll()}
	keys := []string{facetAll}
	for _, group := range groups {
		for _, item := range group.Items {
			labels = append(labels, facetLabel(item.Key)+" ("+strconv.Itoa(item.Count)+")")
			keys = append(keys, item.Key)
		}
	}
	if s.narrow == "" {
		s.narrow, s.narrowSeen = labels[0], labels[0]
	}
	label := msgs.RecallCardsFilters()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, label).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
		// A fixed width rather than the room left: MyGo v0.8.1 animates a
		// growing select's width as it first lays out, which never settles
		// under reduced motion. A filter row wants one column anyway.
		ui.Select(c, &s.narrow, labels).Label(label).MinHeight(tk.Control).
			Width(tk.NameColumnWidth)
	})
	// The select's value is one of the labels built here, so the facet it
	// names is the choice this pass reports — once, when it changed.
	if s.narrow == s.narrowSeen {
		return
	}
	s.narrowSeen = s.narrow
	for i, option := range labels {
		if option == s.narrow && i < len(keys) {
			*picked = pick{key: keys[i], set: true}
		}
	}
}

// bulkBar is the actions over the rows chosen: suspend, move the due date,
// freeze, and tag the notes they came from.
func bulkBar(c *ui.Context, s *CardsState, d CardsDeps, tk design.Tokens, msgs locales.Messages, chosen []core.ID) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
		Padding(tk.SpaceS, tk.SpaceM).Radius(tk.RadiusControl).
		Background(tk.Accent.Alpha(0.08)).Children(func() {
		ui.Text(c, msgs.RecallCardsSelected(len(chosen))).FontSize(tk.TypeUI).Bold()
		ui.Spacer(c)
		ui.Button(c, msgs.Suspend()).MinHeight(tk.Control).OnClick(func() {
			d.Suspend(chosen)
			s.ClearSelection()
		})
		ui.PopoverMenuButton(c, msgs.RecallBulkDue(), func(m *ui.Menu) {
			for _, option := range dueOptions(msgs) {
				option := option
				if m.Item(option.label).Chosen() {
					d.SetDue(chosen, option.days)
					s.ClearSelection()
				}
			}
		}).MinHeight(tk.Control).Label(msgs.RecallBulkDue())
		ui.Button(c, msgs.RecallBulkFreeze()).MinHeight(tk.Control).OnClick(func() {
			d.Freeze(chosen)
			s.ClearSelection()
		})
		ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceXS).Children(func() {
			input := ui.TextInput(c, &s.Tag).
				Placeholder(msgs.RecallBulkTagPlaceholder()).Label(msgs.RecallBulkTagPlaceholder()).
				MinHeight(tk.Control).Width(tk.NameColumnWidth).Bind(&s.tagBox)
			added := false
			if ui.Button(c, msgs.RecallBulkTagApply()).MinHeight(tk.Control).
				Disabled(s.Tag == "").Clicked() {
				added = true
			}
			if input.Submitted() && s.Tag != "" {
				added = true
			}
			if added {
				d.AddTag(chosen, s.Tag)
				s.Tag = ""
				s.ClearSelection()
			}
		})
	})
}

// dueOption is one choice of the due menu: its label and how many days from
// today it puts the cards on.
type dueOption struct {
	label string
	days  int
}

// dueOptions are the due menu's choices: tomorrow, a week, and a month — the
// three answers a hand-set date is worth in a review list.
func dueOptions(msgs locales.Messages) []dueOption {
	return []dueOption{
		{msgs.RecallDueTomorrow(), 1},
		{msgs.RecallDueInAWeek(), 7},
		{msgs.RecallDueInAMonth(), 30},
	}
}

// cardColumns is every column the table can show, in its order, whether or not
// the user switched it off.
func cardColumns(tk design.Tokens, msgs locales.Messages) []ui.TableColumn {
	// The fixed columns are sized in controls, as narrow as their content
	// reads: what is left is the prompt's and the note's, which is what the
	// table is for. The check column is named by its boxes rather than by a
	// header a reader cannot use.
	return []ui.TableColumn{
		{ID: columnCheck, Width: tk.Control, Fixed: true},
		{Title: msgs.RecallColumnPrompt(), ID: columnPrompt, Sortable: true},
		{Title: msgs.RecallColumnNote(), ID: columnNote},
		{Title: msgs.RecallColumnType(), ID: columnType, Width: tk.Control * 3},
		{Title: msgs.RecallColumnStatus(), ID: columnStatus, Width: tk.Control * 2.5},
		{Title: msgs.RecallColumnInterval(), ID: columnInterval, Width: tk.Control * 2.5, Align: ui.End, Sortable: true},
		{Title: msgs.RecallColumnLapses(), ID: columnLapses, Width: tk.Control*2 + tk.SpaceS, Align: ui.End, Sortable: true},
		{Title: msgs.RecallColumnDue(), ID: columnDue, Width: tk.Control * 3, Sortable: true},
	}
}

// visibleColumns is the columns the table builds: the ones the user left on,
// and — on a stage too narrow for all of them — only those a card can be read
// without. What a card asks keeps its room first; the columns that only
// annotate it fold in this order, the note it came from first because the card's
// own page names it, and the lapse count last because it is what "hard" means.
var foldOrder = []string{columnNote, columnInterval, columnStatus, columnType, columnLapses}

// tableRoom is the width the table itself gets: what the window holds beside
// the rail, less the page's padding and, when it stands as a column, the filter
// rail beside it.
func tableRoom(c *ui.Context, wide bool, tk design.Tokens) float32 {
	w, _ := c.Size()
	room := w - 2*tk.SpaceXL
	if w > design.BreakpointNarrow {
		room -= tk.RailWidth
	} else {
		room -= tk.RailCollapsed
	}
	if wide {
		room -= tk.FilterPaneWidth
	}
	return min(room, tk.BrowseWidth)
}

func (s *CardsState) visibleColumns(tk design.Tokens, msgs locales.Messages, room float32) ([]ui.TableColumn, []string) {
	all := cardColumns(tk, msgs)
	hidden := make(map[string]bool, len(s.hidden))
	for id, off := range s.hidden {
		hidden[id] = off
	}
	for _, id := range foldOrder {
		var fixed float32
		for _, col := range all {
			if hidden[col.ID] || col.ID == columnPrompt || col.ID == columnNote {
				continue
			}
			fixed += col.Width
		}
		need := float32(minPromptRoom)
		if !hidden[columnNote] {
			need += minNoteRoom
		}
		if fixed+need <= room {
			break
		}
		hidden[id] = true
	}
	cols := make([]ui.TableColumn, 0, len(all))
	ids := make([]string, 0, len(all))
	for _, col := range all {
		if hidden[col.ID] && col.ID != columnCheck && col.ID != columnPrompt {
			continue
		}
		cols = append(cols, col)
		ids = append(ids, col.ID)
	}
	return cols, ids
}

// minPromptRoom and minNoteRoom are the room a card's prompt and the note it
// came from are read in before another column is folded away.
const (
	minPromptRoom float32 = 220
	minNoteRoom   float32 = 130
)

// table draws the cards and reports the row the keys asked for. The table's own
// sort is the app's: a click on a header writes the order into the state, and
// the rows come back in it.
func table(c *ui.Context, s *CardsState, tk design.Tokens, msgs locales.Messages, rows []CardRow, d CardsDeps, open *int, room float32) {
	cols, ids := s.visibleColumns(tk, msgs, room)
	s.list.Key = func(row int) any { return rows[row].ID }
	s.list.Selected = &s.Selected
	s.list.Sort = &s.Sort
	s.list.Label = func(row int) string { return cardLabel(rows[row], msgs) }
	s.list.CellMenu = func(m *ui.Menu, row, col int) {
		if row < 0 || row >= len(rows) {
			return
		}
		card := rows[row]
		if m.Item(msgs.BrowseOpen()).ID("open").Chosen() {
			*open = row
		}
		if m.Item(msgs.Suspend()).ID("suspend").Chosen() {
			d.Suspend([]core.ID{card.ID})
		}
		if m.Item(msgs.RecallBulkFreeze()).ID("freeze").Chosen() {
			d.Freeze([]core.ID{card.ID})
		}
	}
	element := ui.Table(c, &s.list, cols, len(rows), func(row, col int) {
		if row < 0 || row >= len(rows) || col < 0 || col >= len(ids) {
			return
		}
		cardCell(c, s, tk, msgs, rows[row], ids[col])
	}).Grow(1).Label(msgs.RecallCardsTitle())
	if element.Submitted() {
		*open = s.Selected
	}
}

// cardCell draws one cell of one card.
func cardCell(c *ui.Context, s *CardsState, tk design.Tokens, msgs locales.Messages, card CardRow, id string) {
	switch id {
	case columnCheck:
		box := ui.Checkbox(c, s.check(card.ID), "")
		box.Label(msgs.RecallCardsSelect(cardLabel(card, msgs)))
	case columnPrompt:
		text, colour := card.Prompt, tk.Text
		if card.Lost {
			text, colour = msgs.LostBody(), tk.TextMuted
		}
		ui.Text(c, text).SingleLine().TextColor(colour)
	case columnNote:
		ui.Text(c, card.Note).SingleLine().TextColor(tk.TextMuted)
	case columnType:
		ui.Text(c, card.Type).SingleLine().TextColor(tk.TextMuted)
	case columnStatus:
		ui.Text(c, statusLabel(msgs, card.Status)).SingleLine().TextColor(tk.TextMuted)
	case columnInterval:
		ui.Text(c, intervalLabel(msgs, card.Interval)).SingleLine().TextColor(tk.TextMuted)
	case columnLapses:
		ui.Text(c, strconv.Itoa(card.Lapses)).TextColor(tk.TextMuted)
	case columnDue:
		ui.Text(c, msgs.RecallDueOn(card.Due)).SingleLine().TextColor(tk.TextMuted)
	}
}

// cardLabel names a card for the table's keyboard and its checkbox: what it
// asks, or the sentence a lost card shows instead.
func cardLabel(card CardRow, msgs locales.Messages) string {
	if card.Lost || card.Prompt == "" {
		return msgs.Lost()
	}
	return card.Prompt
}

// statusLabel names a card's lifecycle state.
func statusLabel(msgs locales.Messages, status core.Status) string {
	switch status {
	case core.StatusNew:
		return msgs.RecallStatusNew()
	case core.StatusLearning:
		return msgs.RecallStatusLearning()
	case core.StatusReview:
		return msgs.RecallStatusReview()
	case core.StatusSuspended:
		return msgs.RecallStatusSuspended()
	}
	return ""
}

// intervalLabel is a card's interval in days, a dash for one no review has
// touched.
func intervalLabel(msgs locales.Messages, days int) string {
	if days <= 0 {
		return "—"
	}
	return msgs.IntervalDays(days)
}

// emptyCards is a table with no rows: a library with no cards says how to make
// one, and a search that found none says what it searched.
func emptyCards(c *ui.Context, tk design.Tokens, msgs locales.Messages, searched bool) {
	if searched {
		kit.Empty(c, "search", msgs.RecallCardsEmpty(), nil)
		return
	}
	kit.Empty(c, "layers", msgs.RecallCardsNone(), func() {
		ui.Text(c, msgs.RecallCardsNoneBody()).FontSize(tk.TypeUI).
			TextColor(tk.TextMuted).MaxWidth(tk.SummaryWidth)
	})
}
