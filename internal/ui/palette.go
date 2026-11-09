package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// paletteRowsPerGroup caps the notes and the cards one query lists: the launcher
// is scanned, not browsed, so a group longer than a glance belongs to the search
// page, and the cap keeps a frame off the whole library.
const paletteRowsPerGroup = 8

// paletteSection is one of the palette's three groups, in the order it lists
// them: what the app can do, the notes it holds, then their cards.
type paletteSection uint8

const (
	paletteCommands paletteSection = iota
	paletteNotes
	paletteCards
)

// paletteAction is one command the palette lists.
type paletteAction struct {
	label string
	run   func()
}

// paletteEntry is one row of the palette: the group it belongs to, the action it
// runs, and the second line a card carries — the note its prompt came from.
type paletteEntry struct {
	section paletteSection
	action  paletteAction
	hint    string
}

// paletteRow is one line the list draws: a group's title, or an entry.
type paletteRow struct {
	header bool
	title  string
	entry  paletteEntry
}

// paletteState is the command palette: open or not, what has been typed, and
// which of the rows the keys are on.
type paletteState struct {
	open  bool
	query string
	// sel is the row the keys are on. A group's title is never it: the list
	// steps over headers, and so does every move this file makes.
	sel   int
	field ui.Handle
	list  ui.ListState
}

// openPalette opens the palette on a fresh query and asks for the focus before
// the dialog is built: AutoFocus would take it before the dialog recorded what
// had it, and Escape would then have nowhere to give it back.
func (a *App) openPalette(c *ui.Context) {
	a.anim.palette++
	a.palette.open = true
	a.palette.query = ""
	a.palette.sel = 0
	a.palette.field.Focus(c)
}

// paletteActions lists what the app can do from anywhere, with the block
// actions of the note open in the editor. They are listed whenever the library
// holds a note; each runs against the note the editor has loaded, loading it
// first when the user has not been there yet.
func (a *App) paletteActions(c *ui.Context) []paletteAction {
	msgs := m(c)
	actions := []paletteAction{
		{label: msgs.ActionReviewCards(), run: func() { a.setMode(modeReview) }},
		{label: msgs.ActionReading(), run: func() { a.setMode(modeReading) }},
		{label: msgs.ActionEditNotes(), run: func() { a.setMode(modeNotes) }},
		{label: msgs.BrowseTitle(), run: func() { a.setMode(modeBrowse) }},
		{label: msgs.StatsTitle(), run: func() { a.setMode(modeStats) }},
		{label: msgs.NewNote(), run: func() { a.newNote(c) }},
		{label: msgs.OpenSettings(), run: func() { a.openSettings() }},
	}
	if a.lib == nil {
		return actions
	}
	actions = append(actions,
		paletteAction{label: msgs.ImportTitle(), run: func() { a.openImport(c) }},
		paletteAction{label: msgs.ExportTitle(), run: func() { a.openExport(c) }},
		// A note's shape is typed, not commanded: paragraphs are separated by
		// blank lines. Making a card from the paragraph at the caret stays a
		// command until the typing gesture lands (#94).
		paletteAction{label: msgs.MakeCard(), run: func() { a.makeCardAtCaret(c) }},
		// The note open in the notebook is the one the reading queue takes: any
		// note can be read, so the command is where the user already is.
		paletteAction{label: msgs.ActionAddReading(), run: func() {
			a.addReading(c, a.openNoteID())
		}},
	)
	// The importers plugins declare, beside the app's own (#153): one entry
	// per enabled plugin's format, named as its label spells it.
	for _, ref := range a.importers() {
		entry := ref
		actions = append(actions, paletteAction{
			label: a.pluginText(c, entry.Plugin, string(entry.Entry.Label)),
			run:   func() { a.importWithPluginEntry(c, entry) },
		})
	}
	// The plugins' commands last, and only the ones their when shows now: the
	// palette lists what the window can do, not what every plugin can do.
	return append(actions, a.pluginPaletteActions(c)...)
}

// paletteEntries is the palette's three groups as the query narrows them: the
// commands — the app's, then the plugins' — then the notes and the cards whose
// text matches, each group capped.
func (a *App) paletteEntries(c *ui.Context, query string) []paletteEntry {
	msgs := m(c)
	entries := make([]paletteEntry, 0, 24)
	add := func(section paletteSection, action paletteAction, hint string) bool {
		if !paletteMatches(action.label, query) {
			return false
		}
		entries = append(entries, paletteEntry{section: section, action: action, hint: hint})
		return true
	}
	for _, action := range a.paletteActions(c) {
		add(paletteCommands, action, "")
	}
	// A note is its title, and the one without a title says so rather than
	// leaving an unnamed row.
	notes := 0
	for _, note := range a.data.notes {
		title := note.Title
		if strings.TrimSpace(title) == "" {
			title = msgs.UntitledNote()
		}
		if !paletteMatches(title, query) {
			continue
		}
		id := note.ID
		if add(paletteNotes, paletteAction{label: title, run: func() { a.ShowNote(id) }}, "") {
			if notes++; notes == paletteRowsPerGroup {
				break
			}
		}
	}
	// A card reads as the paragraph asks: its prompt, and the note it lives in.
	cards := 0
	for _, card := range a.data.cards {
		prompt, _, _ := a.faces(card)
		if strings.TrimSpace(prompt) == "" {
			continue
		}
		hint := ""
		if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
			hint = a.data.titleOf(note)
		}
		if !paletteMatches(prompt+" "+hint, query) {
			continue
		}
		id := card.ID
		if add(paletteCards, paletteAction{label: prompt, run: func() { a.ShowCard(id) }}, hint) {
			if cards++; cards == paletteRowsPerGroup {
				break
			}
		}
	}
	return entries
}

// paletteRows groups the entries under their titles. A group with no entry
// behind it is not drawn at all, and neither is its title: the palette shows
// what matches, not an empty section.
func paletteRows(c *ui.Context, entries []paletteEntry) []paletteRow {
	msgs := m(c)
	titles := [...]string{msgs.PaletteCommands(), msgs.PaletteNotes(), msgs.PaletteCards()}
	rows := make([]paletteRow, 0, len(entries)+len(titles))
	group := paletteSection(0)
	for i, entry := range entries {
		if i == 0 || entry.section != group {
			group = entry.section
			rows = append(rows, paletteRow{header: true, title: titles[group]})
		}
		rows = append(rows, paletteRow{entry: entry})
	}
	return rows
}

// paletteEntryRow answers the nearest entry row to row: the row itself when it
// holds an entry, else the one after it, else the one before. -1 when the rows
// hold none.
func paletteEntryRow(rows []paletteRow, row int) int {
	for i := row; i < len(rows); i++ {
		if i >= 0 && !rows[i].header {
			return i
		}
	}
	for i := min(row, len(rows)-1); i >= 0; i-- {
		if !rows[i].header {
			return i
		}
	}
	return -1
}

// paletteStep is the entry row delta steps from row, in the direction delta
// names, staying where it is when there is none: the list's own movement, so
// the keys walk the entries and never a title.
func paletteStep(rows []paletteRow, row, delta int) int {
	if delta == 0 {
		return row
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	for i := row + step; i >= 0 && i < len(rows); i += step {
		if !rows[i].header {
			return i
		}
	}
	return row
}

// paletteEntryAt is the entry at row, when row holds one: a title or a row
// outside the list answers false.
func paletteEntryAt(rows []paletteRow, row int) (paletteEntry, bool) {
	if row < 0 || row >= len(rows) || rows[row].header {
		return paletteEntry{}, false
	}
	return rows[row].entry, true
}

// paletteMatches reports whether text holds the query, ignoring case. An empty
// query matches everything.
func paletteMatches(text, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	return q == "" || strings.Contains(strings.ToLower(text), q)
}

// paletteView draws the palette over the window. DialogBase keeps the keyboard
// in it, Escape closes it and gives the focus back where it was.
func (a *App) paletteView(c *ui.Context, tk design.Tokens) {
	if !a.palette.open {
		return
	}
	msgs := m(c)
	rows := paletteRows(c, a.paletteEntries(c, a.palette.query))

	ui.DialogBase(c, &a.palette.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(tk.SummaryWidth).Padding(tk.SpaceL).Gap(tk.SpaceM))
		panel.Label(msgs.CommandPalette())
		// The palette arrives a little small over a scrim that fades in.
		panel = kit.ArrivePanel(panel, a.anim.palette)
		kit.ArriveScrim(backdrop, a.anim.palette)

		panel.Children(func() {
			// The choice is resolved against this pass's rows before anything
			// reads it: a query narrows rows between passes, and the keys must
			// land on an entry that exists now, never on the one they were on.
			a.palette.sel = paletteEntryRow(rows, a.palette.sel)
			// A plain field rather than a search field: Escape must close the
			// palette in one keystroke, and a search field keeps it to clear its
			// text first.
			field := ui.TextInput(c, &a.palette.query).
				Placeholder(msgs.PaletteSearch()).Bind(&a.palette.field).
				Label(msgs.PaletteSearch()).MinHeight(tk.Control)
			if field.Changed() {
				a.palette.sel = paletteEntryRow(rows, 0)
			}
			if field.Submitted() {
				if entry, ok := paletteEntryAt(rows, a.palette.sel); ok {
					a.runPaletteEntry(entry)
				}
				return
			}
			c.OnShortcut(0, ui.KeyDown, func() { a.palette.sel = paletteStep(rows, a.palette.sel, 1) })
			c.OnShortcut(0, ui.KeyUp, func() { a.palette.sel = paletteStep(rows, a.palette.sel, -1) })

			if len(rows) == 0 {
				ui.Text(c, msgs.PaletteEmpty()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
				return
			}
			// The rows are a list's: the choice, the keys and what assistive
			// technology reads come with it, and the filter is the query above.
			// A group's title is a header row, which the list keeps out of the
			// choice and pins while its group scrolls under it. Twelve rows,
			// not eight: the host's own actions and the ones the plugins
			// declare stay all visible at the default window.
			a.palette.list.Selected = &a.palette.sel
			a.palette.list.Header = func(row int) bool { return rows[row].header }
			a.palette.list.Label = func(row int) string {
				if rows[row].header {
					return rows[row].title
				}
				return rows[row].entry.action.label
			}
			chosen := ui.List(c, &a.palette.list, len(rows), func(i int) {
				paletteRowView(c, tk, rows[i])
			}).Grow(1).MaxHeight(tk.Control * 12).Label(msgs.CommandPalette())
			if chosen.Changed() {
				if entry, ok := paletteEntryAt(rows, a.palette.sel); ok {
					a.runPaletteEntry(entry)
				}
			}
		})
	})
}

// paletteRowView draws one line of the palette: a group's title, or an entry —
// its text, and for a card the note its prompt came from. A title carries the
// panel's own face, since it stays where it is while its group scrolls.
func paletteRowView(c *ui.Context, tk design.Tokens, row paletteRow) {
	if row.header {
		ui.Text(c, row.title).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).
			Padding(tk.SpaceXS, tk.SpaceS).Background(tk.Surface)
		return
	}
	item := ui.Column(c).Gap(tk.SpaceXS).Padding(tk.SpaceXS, tk.SpaceS)
	item.Children(func() {
		ui.Text(c, row.entry.action.label).FontSize(tk.TypeUI).SingleLine()
		if row.entry.hint != "" {
			ui.Text(c, row.entry.hint).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted).SingleLine()
		}
	})
}

// runPaletteEntry closes the palette, then runs what was chosen.
func (a *App) runPaletteEntry(entry paletteEntry) {
	a.palette.open, a.palette.query, a.palette.sel = false, "", 0
	entry.action.run()
}
