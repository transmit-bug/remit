// Package recall is the recall area's queue page (S27) and the dialog that
// narrows a review session to one note or one tag (S28).
//
// The page answers what is due, how long that takes, which notes the cards come
// from and how the last half year went; the dialog says what a sitting draws
// from. Both read the app's answer — the counts, the rows, the history — and
// report what the user chose through Deps: nothing here reads the store, a
// clock or the disk.
package recall

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// NoteRow is one note's share of the queue: what the note is called, how much it
// holds, and how much of it is waiting under the page's filter. The app derives
// it from the queue it derived for the session, so a row and a sitting agree.
type NoteRow struct {
	Note  core.ID
	Title string
	// Blocks is how many paragraphs the note holds, Cards how many cards follow
	// them, and Hard how many of those keep coming back wrong.
	Blocks, Cards, Hard int
	// Due is how many of the note's cards the page's filter keeps.
	Due int
}

// Note is a note the scope dialog can name.
type Note struct {
	ID    core.ID
	Title string
}

// ScopeCounts is how many cards each scope would put on the session: the whole
// library, one note, one tag. The app derives it once per load, so the dialog's
// live count is a read.
type ScopeCounts struct {
	All   int
	Notes map[core.ID]int
	Tags  map[string]int
}

// Count is what a scope would take.
func (c ScopeCounts) Count(scope queue.Scope) int {
	switch scope.Kind {
	case queue.ScopeNote:
		return c.Notes[scope.Note]
	case queue.ScopeTag:
		return c.Tags[scope.Tag]
	}
	return c.All
}

// State is the page's own memory: the filter and the list's place, and the scope
// dialog's draft while it is open. The app fills the rest when it loads the
// queue and after every write; a frame only reads it.
type State struct {
	// Day is the moment the queue was built at: the day the page's "today" is,
	// and the last square of its heat strip.
	Day time.Time
	// Loaded is set once the app has answered: before that the page says it is
	// loading rather than that the day is empty.
	Loaded bool
	// Err is why the queue could not be loaded, "" when it did. The page says so
	// and offers to ask again.
	Err string
	// Filter is the page's own filter: which cards the rows count.
	Filter Filter

	list ui.ListState

	// The scope dialog: whether it is open, which part of it shows, and the draft
	// it would apply.
	open    bool
	segment int
	draft   queue.Scope
}

// Deps is what the page asks of the app: the queue it derived, the history
// under it, and one function per thing the user can do. Each runs on the UI
// thread; the app does the reading and the writing.
type Deps struct {
	// Rows are the notes holding cards the queue would take under the page's
	// filter, in the order the session would meet them.
	Rows func() []NoteRow
	// Counts is what the session's scope holds: how many cards, by kind.
	Counts func() queue.Counts
	// Scope is what the session draws from, which the scope button names.
	Scope func() queue.Scope
	// ScopeNotes are the notes the dialog can name, and ScopeTags the tags.
	ScopeNotes func() []Note
	ScopeTags  func() []string
	// ScopeCounts is how many cards each choice would take.
	ScopeCounts func() ScopeCounts
	// History is the reviews of the last half year, for the page's heat strip.
	History func() []stats.DayCount
	// Tomorrow is how many cards come due the day after.
	Tomorrow func() int
	// Lost is how many cards lost the paragraph they followed: they are not in
	// the queue, so the Lost filter names them rather than listing them.
	Lost func() int
	// SavedFilters are the filters the user kept, oldest first.
	SavedFilters func() []Filter

	// Start enters the review session with the scope on screen.
	Start func()
	// ReviewNote narrows the session to one note and enters it: what a row does.
	ReviewNote func(core.ID)
	// SetFilter narrows the page's rows and leaves the session's scope alone.
	SetFilter func(Filter)
	// SaveFilter keeps the filter as a chip; RemoveFilter takes one away.
	SaveFilter   func(Filter)
	RemoveFilter func(Filter)
	// ApplyScope takes the dialog's scope and enters the session.
	ApplyScope func(queue.Scope)
	// Relink shows the lost cards, where each one is pointed at another
	// paragraph: the way out of the Lost filter.
	Relink func()
	// Reload asks for the queue again: the action of a failed load.
	Reload func()
}

// View draws the page: its head with the scope and the one action, what the
// sitting costs, the filter row, the notes it comes from and the half year
// behind it. The scope dialog is the window's overlay, so the app draws it with
// the other overlays (ScopeDialog): an overlay belongs to the window's root,
// not to the column a page is built in.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())

	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.ListWidth).Grow(1).Gap(tk.SpaceL).Children(func() {
			head(c, tk, msgs, s, d)
			switch {
			case s.Err != "":
				failed(c, tk, msgs, s.Err, d)
			case !s.Loaded:
				loading(c, tk, msgs)
			default:
				body(c, s, tk, msgs, d)
			}
		})
	})
}

// head is the page's name, the scope the session draws from, and the one action
// the page carries. A scope with nothing due disables it and says why beside it:
// a disabled control shows no tooltip, so the reason is on screen.
func head(c *ui.Context, tk design.Tokens, msgs locales.Messages, s *State, d Deps) {
	loaded := s.Loaded && s.Err == ""
	counts := queue.Counts{}
	if loaded && d.Counts != nil {
		counts = d.Counts()
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Wrap().Children(func() {
		ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, msgs.RecallTitle()).FontSize(tk.TypeH1).Bold()
			ui.Text(c, msgs.RecallSubtitle()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		})
		scopeButton(c, tk, msgs, s, d)
		if loaded && counts.Remaining == 0 {
			ui.Text(c, msgs.RecallStartEmpty()).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted)
			ui.PrimaryButton(c, msgs.RecallStart()).MinHeight(tk.Control).Disabled(true)
			return
		}
		start := ui.PrimaryButton(c, msgs.RecallStart()).MinHeight(tk.Control)
		if loaded && d.Start != nil {
			start.OnClick(d.Start)
		} else if !loaded {
			start.Disabled(true)
		}
	})
}

// scopeButton names the scope the session draws from and opens the dialog that
// changes it. The name is the whole library, the note's title, or the tag.
func scopeButton(c *ui.Context, tk design.Tokens, msgs locales.Messages, s *State, d Deps) {
	scope := queue.Scope{}
	if d.Scope != nil {
		scope = d.Scope()
	}
	label := scopeLabel(msgs, scope, d)
	button := ui.Button(c, label).Label(msgs.ReviewScopeMenu() + ": " + label).
		Tooltip(msgs.ReviewScopeMenu()).MinHeight(tk.Control)
	button.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceXS).Children(func() {
			tk.Icon(c, "layers", design.IconSmall).TextColor(tk.TextMuted)
			ui.Text(c, label).FontSize(tk.TypeUI).SingleLine()
		})
	})
	button.OnClick(func() {
		s.draft = scope
		s.segment = int(scope.Kind)
		s.open = true
	})
}

// scopeLabel names the scope in this frame's language.
func scopeLabel(msgs locales.Messages, scope queue.Scope, d Deps) string {
	switch scope.Kind {
	case queue.ScopeNote:
		for _, note := range d.ScopeNotes() {
			if note.ID == scope.Note {
				return note.Title
			}
		}
	case queue.ScopeTag:
		return "#" + scope.Tag
	}
	return msgs.ReviewScope()
}

// body is the queue itself: what the sitting costs, the rows the filter keeps,
// and the half year behind them. A scope with nothing due says so once and
// stops; a filter that matches nothing says that instead, since the queue is not
// empty.
func body(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, d Deps) {
	counts := d.Counts()
	if counts.Remaining == 0 {
		kit.Empty(c, "layers", msgs.RecallEmpty(), func() {
			ui.Text(c, msgs.RecallTomorrow(d.Tomorrow())).FontSize(tk.TypeUI).
				TextColor(tk.TextMuted)
		})
		heat(c, tk, msgs, s, d)
		return
	}
	loadCard(c, tk, msgs, counts)
	filters(c, s, tk, msgs, d)
	savedChips(c, s, tk, msgs, d)
	// A lost card knows the paragraph it followed, not the note it lived in, so
	// the Lost filter counts them and offers the one place they can be repaired
	// rather than pretending they group by note.
	if s.Filter == FilterLost {
		lost(c, tk, msgs, d)
		heat(c, tk, msgs, s, d)
		return
	}
	rows := d.Rows()
	if len(rows) == 0 {
		ui.Text(c, msgs.RecallNoMatch()).FontSize(tk.TypeUI).
			TextColor(tk.TextMuted)
		heat(c, tk, msgs, s, d)
		return
	}
	kit.SectionHead(c, tk, msgs.RecallByNote(), msgs.RecallByNoteHint(), nil)
	s.list.Key = func(row int) any { return rows[row].Note }
	s.list.Label = func(row int) string { return rows[row].Title }
	ui.Divider(c)
	ui.List(c, &s.list, len(rows), func(i int) {
		rowView(c, tk, msgs, rows[i], d)
	}).Grow(1).Label(msgs.RecallByNote())
	heat(c, tk, msgs, s, d)
}

// loadCard is what the sitting costs before it is spent: the minutes in the
// display face, one line naming them, and the queue split into the three kinds
// of card a session meets — new, learning, and review.
func loadCard(c *ui.Context, tk design.Tokens, msgs locales.Messages, counts queue.Counts) {
	minutes := int(minutesOf(counts.Remaining) + 0.5)
	kit.Card(c, func() {
		ui.Column(c).FillWidth().Padding(tk.SpaceL).Gap(tk.SpaceM).Children(func() {
			ui.Text(c, msgs.TodayMinutes(minutes)).FontSize(tk.TypeH1).Bold()
			ui.Text(c, msgs.RecallLoad()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			kit.StackBar(c, []kit.StackPart{
				{Value: float32(counts.New), Colour: tk.Accent},
				{Value: float32(counts.Learning), Colour: tk.Warning},
				{Value: float32(counts.Due), Colour: tk.Success},
			}, msgs.RecallRatioLabel(counts.New, counts.Learning, counts.Due))
			ui.Row(c).Gap(tk.SpaceL).Wrap().Children(func() {
				ratioLegend(c, tk, tk.Accent, msgs.RecallRatioNew(counts.New))
				ratioLegend(c, tk, tk.Warning, msgs.RecallRatioLearning(counts.Learning))
				ratioLegend(c, tk, tk.Success, msgs.RecallRatioReview(counts.Due))
			})
		})
	}).FillWidth()
}

// minutesOf is how long a queue of n cards takes, in whole minutes: the same
// estimate Today prices its review line with, so the two pages agree.
func minutesOf(n int) float64 {
	return queue.Estimate(0, n).Minutes()
}

// ratioLegend names one part of the bar: its colour, its name and its count.
func ratioLegend(c *ui.Context, tk design.Tokens, colour ui.Color, label string) {
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceXS).Children(func() {
		ui.Box(c).Width(tk.SpaceS).Height(tk.SpaceS).Radius(tk.RadiusBar).
			Background(colour)
		ui.Text(c, label).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
	})
}

// filters is the page's own filter: six toggles, one pressed, and the action
// that keeps the chosen one as a chip. It narrows the rows, never the session's
// scope, which is the scope button's job.
func filters(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, d Deps) {
	ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Wrap().Children(func() {
		ui.ToggleGroup(c, func() {
			for f := FilterAll; f < filterCount; f++ {
				f := f
				on := s.Filter == f
				if ui.Toggle(c, &on, f.label(msgs)).Changed() && d.SetFilter != nil {
					d.SetFilter(f)
				}
			}
		}).Label(msgs.RecallFilters())
		if d.SaveFilter != nil {
			ui.Button(c, msgs.RecallSaveFilter()).MinHeight(tk.Control).
				OnClick(func() { d.SaveFilter(s.Filter) })
		}
	})
}

// savedChips are the filters the user kept: one chip each, which applies the
// filter, and a button that takes the chip away. A saved filter is the app's
// memory of the choice, not a second queue.
func savedChips(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, d Deps) {
	if d.SavedFilters == nil || d.RemoveFilter == nil {
		return
	}
	saved := d.SavedFilters()
	if len(saved) == 0 {
		return
	}
	ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Text(c, msgs.RecallSaved()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		for _, f := range saved {
			f := f
			label := f.label(msgs)
			chip := ui.ButtonBase(c.Key("filter:"+f.Name())).
				Padding(tk.SpaceXS, tk.SpaceS).Radius(tk.RadiusControl).
				Background(tk.Sunken).Label(label).
				OnClick(func() {
					if d.SetFilter != nil {
						d.SetFilter(f)
					}
				})
			chip.Children(func() {
				ui.Text(c, label).FontSize(tk.TypeUI).SingleLine()
			})
			remove := ui.Button(c, "").Label(msgs.RecallRemoveFilter(label)).
				Tooltip(msgs.RecallRemoveFilter(label)).
				MinWidth(tk.Hit).MinHeight(tk.Control).Justify(ui.Center).
				Radius(tk.RadiusControl).TextColor(tk.TextMuted)
			remove.Children(func() { tk.Icon(c, "x", design.IconSmall) })
			remove.OnClick(func() { d.RemoveFilter(f) })
		}
	})
}

// lost is the cards whose paragraph is gone from the library: how many, and the
// one action that opens them in the card table, where each is pointed at another
// paragraph. With none lost, the filter keeps nothing and says so as any other
// filter does.
func lost(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps) {
	count := 0
	if d.Lost != nil {
		count = d.Lost()
	}
	if count == 0 {
		ui.Text(c, msgs.RecallNoMatch()).FontSize(tk.TypeUI).
			TextColor(tk.TextMuted)
		return
	}
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Wrap().Children(func() {
		tk.Icon(c, "unlink", design.IconSmall).TextColor(tk.Warning)
		ui.Text(c, msgs.RecallLost(count)).FontSize(tk.TypeUI).
			TextColor(tk.TextMuted).Grow(1)
		if d.Relink != nil {
			ui.Button(c, msgs.RecallRelink()).MinHeight(tk.Control).OnClick(d.Relink)
		}
	})
}

// rowView is one note's share of the queue: its title, what it holds, and how
// much of it is waiting now. The whole row opens the session narrowed to it, so
// a note is reviewed without leaving the page for a dialog.
func rowView(c *ui.Context, tk design.Tokens, msgs locales.Messages, row NoteRow, d Deps) {
	rowButton := ui.Button(c, "").FillWidth().MinHeight(tk.Control*2).
		Padding(tk.SpaceM, tk.SpaceS).Radius(0).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border).
		Label(msgs.RecallRowOpen(row.Title))
	rowButton.Children(func() {
		ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(tk.SpaceM).Children(func() {
			tk.Icon(c, "notebook-pen", design.IconSmall).TextColor(tk.TextMuted)
			ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
				ui.Text(c, row.Title).FontSize(tk.TypeUI).SingleLine()
				ui.Text(c, msgs.RecallRowMeta(row.Blocks, row.Cards, row.Hard)).
					FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
			})
			ui.Badge(c, msgs.RecallRowDue(row.Due)).
				Background(tk.Accent.Alpha(0.12)).TextColor(tk.Accent)
		})
	})
	if d.ReviewNote != nil {
		rowButton.OnClick(func() { d.ReviewNote(row.Note) })
	}
}

// heat is the reviews of the last half year, as the statistics page draws them:
// one square per day, so a quiet month and a busy one read at a glance.
func heat(c *ui.Context, tk design.Tokens, msgs locales.Messages, s *State, d Deps) {
	weeks, most := HeatWeeks(s.Day, d.History())
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		kit.SectionHead(c, tk, msgs.RecallHeat(), "", nil)
		kit.Heatmap(c, weeks, most, kit.HeatmapOptions{
			Cell:  design.HeatCell,
			Gap:   design.HeatGap,
			Label: msgs.RecallHeatLabel(),
			Tooltip: func(day kit.HeatDay) string {
				return msgs.RecallHeatDay(day.Day, day.Count)
			},
		})
	})
}

// loading is the page before its first answer: the queue is on its way, and the
// page says so rather than looking like a day with nothing in it.
func loading(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	ui.Row(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).
		Gap(tk.SpaceS).Children(func() {
		ui.Spinner(c)
		ui.Text(c, msgs.RecallLoading()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	})
}

// failed is a queue that could not be built: what went wrong, and the one
// action that asks again.
func failed(c *ui.Context, tk design.Tokens, msgs locales.Messages, reason string, d Deps) {
	kit.Notice(c, "circle-alert", msgs.RecallLoadFailed(reason), tk.Danger, func() {
		if d.Reload != nil {
			ui.Button(c, msgs.RecallRetry()).MinHeight(tk.Control).OnClick(d.Reload)
		}
	})
}
