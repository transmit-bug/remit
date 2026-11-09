// Package browse is the browse screen: every card the library holds, narrowed
// by a search and a filter, with the actions a card carries. The rows are the
// app's — it derives them from the store — so the screen reads them through
// Deps and reports what the user did; every write stays in the app.
package browse

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// Filter is which cards browse lists: every card, those the queue has for
// today, those never reviewed, the suspended ones, or those whose block is
// gone.
type Filter uint8

const (
	All Filter = iota
	Due
	New
	Suspended
	Lost
)

// Row is one card as browse shows it: the prompt it renders from, the note that
// prompt lives in, and the state of its reference when that prompt is not live
// text.
type Row struct {
	ID        core.ID
	Prompt    string
	Note      string
	State     core.LinkState
	Suspended bool
}

// State is browse's state: the query and filter the user chose, and the row the
// keys are on. The rows themselves are derived by the app, so the screen only
// keeps the choice inside them.
type State struct {
	Query    string // what the search field holds
	Filter   Filter // which cards the list is narrowed to
	Selected int    // the row the keys are on, -1 for none

	list  ui.ListState
	field ui.Handle
}

// Keep keeps the choice inside n rows, so that a reload that drops rows leaves
// the keys inside the list.
func (s *State) Keep(n int) { s.Selected = min(s.Selected, n-1) }

// Focus puts the caret in the search field. A link from outside the window has
// no context to give, and the request then waits for the field's first build.
func (s *State) Focus(c ...*ui.Context) { s.field.Focus(c...) }

// Deps is what browse asks of the app: the rows it derived from the store, the
// reload a keystroke or a filter asks for, and the actions a row carries. Each
// runs on the UI thread; the app does the reading and the writing.
type Deps struct {
	// Rows are the cards the query and filter found, as the app derived them.
	Rows func() []Row
	// Reload derives the rows again: the query or the filter changed.
	Reload func()
	// Open shows the card's note, or the card in review when its block is gone.
	Open func(core.ID)
	// Relink points a lost card at another block.
	Relink func(core.ID)
}

// View draws browse: a search field, the filters, and the cards they found. The
// rows outlive the pass, so what the view derives from a keystroke is loaded
// after the fields have taken it.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	rows := d.Rows()
	reload, open := false, -1

	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(design.BrowseWidth).Gap(tk.SpaceL).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, msgs.BrowseTitle()).FontSize(tk.TypeH2).Bold()
				ui.Spacer(c)
				ui.Badge(c, msgs.CardCount(len(rows)))
			})

			ui.Row(c).Gap(tk.SpaceM).AlignItems(ui.Center).Children(func() {
				field := ui.SearchField(c, &s.Query).
					Placeholder(msgs.BrowseSearch()).Label(msgs.BrowseSearch()).
					MinHeight(tk.Control).Grow(1).Bind(&s.field)
				if field.Changed() {
					reload, open = true, -1
				}

				filter := int(s.Filter)
				if ui.Segmented(c, &filter,
					msgs.BrowseAll(), msgs.BrowseDue(), msgs.BrowseNew(), msgs.BrowseSuspended(), msgs.Lost(),
				).Label(msgs.BrowseFilters()).Changed() {
					s.Filter = Filter(filter)
					reload, open = true, -1
				}
			})

			listView(c, s, tk, msgs, rows, d, &open)

			ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, msgs.BrowseOpen()).MinHeight(tk.Control).
					Disabled(s.Selected < 0 || s.Selected >= len(rows)).Clicked() {
					open = s.Selected
				}
				if len(rows) == 0 {
					ui.Text(c, msgs.BrowseEmpty()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
				}
			})
		})
	})

	// The effects of this pass, once its fields have taken what was typed.
	if reload {
		d.Reload()
		c.Invalidate()
	}
	if open >= 0 && open < len(rows) {
		d.Open(rows[open].ID)
	}
}

// listView is the list of cards found: a row chooses on a click, and opens on
// Enter, a double click, or the Open button.
func listView(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, rows []Row, d Deps, open *int) {
	s.list.Selected = &s.Selected
	s.list.Key = func(row int) any { return rows[row].ID }
	s.list.Label = func(row int) string {
		if prompt := rows[row].Prompt; prompt != "" {
			return prompt
		}
		return msgs.Lost()
	}

	list := ui.List(c, &s.list, len(rows), func(i int) {
		rowView(c, tk, msgs, rows[i], d.Relink)
	}).Grow(1).Label(msgs.BrowseTitle())
	if list.Submitted() {
		*open = s.Selected
	}
}

// rowView draws one card: its prompt, the note it came from, the state of its
// reference, and — for a lost one — the action that repairs it. It is a row of
// ui.List, not a widget of its own.
func rowView(c *ui.Context, tk design.Tokens, msgs locales.Messages, row Row, relink func(core.ID)) {
	ui.Row(c).Gap(tk.SpaceM).AlignItems(ui.Center).Padding(tk.SpaceS, tk.SpaceM).Children(func() {
		ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
			text, colour := row.Prompt, tk.Text
			if row.State == core.LinkLost {
				text, colour = msgs.LostBody(), tk.TextMuted
			}
			ui.Text(c, text).FontSize(tk.TypeUI).TextColor(colour).SingleLine()
			if row.Note != "" {
				ui.Text(c, row.Note).FontSize(tk.TypeUI).TextColor(tk.TextMuted).SingleLine()
			}
		})
		switch {
		case row.State == core.LinkLost:
			stateBadge(c, tk, msgs.Lost(), tk.Danger)
			ui.Button(c, msgs.Relink()).MinHeight(tk.Control).
				OnClick(func() { relink(row.ID) })
		case row.State == core.LinkFrozen:
			stateBadge(c, tk, msgs.Frozen(), tk.Warning)
		}
		if row.Suspended {
			ui.Badge(c, msgs.BrowseSuspended())
		}
	})
}

// stateBadge marks a row whose reference is not live text.
func stateBadge(c *ui.Context, tk design.Tokens, label string, colour ui.Color) {
	ui.Badge(c, label).
		Background(tk.LinkFill(colour)).TextColor(tk.LinkInk(colour))
}
