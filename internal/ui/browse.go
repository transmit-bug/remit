package ui

import (
	"log/slog"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
)

// browseFilter is which cards browse lists: every card, those the queue has
// for today, those never reviewed, the suspended ones, or those whose block is
// gone.
type browseFilter uint8

const (
	browseAll browseFilter = iota
	browseDue
	browseNew
	browseSuspended
	browseLost
)

// browseRow is one card as browse shows it: the prompt it renders from, the
// note that prompt lives in, and the state of its reference when that prompt
// is not live text.
type browseRow struct {
	card   core.Card
	prompt string
	note   string
	state  core.LinkState
}

// browseState is browse mode's state: the query and filter the user chose, the
// rows the load path derived from them, and where the keys are in the list.
type browseState struct {
	query    string
	filter   browseFilter
	selected int
	rows     []browseRow
	list     ui.ListState
	field    ui.Handle
}

// chosen is the row the keys are on.
func (b *browseState) chosen() (browseRow, bool) {
	if b.selected < 0 || b.selected >= len(b.rows) {
		return browseRow{}, false
	}
	return b.rows[b.selected], true
}

// loadBrowse derives the rows browse shows from the store: the cards the filter
// keeps, narrowed by the query when there is one. The mode switch and reload
// call it, so the view only reads rows.
func (a *App) loadBrowse() {
	if a.lib == nil {
		a.browse.rows, a.browse.selected = nil, -1
		return
	}
	cards := a.browseFilterCards()
	if query := strings.TrimSpace(a.browse.query); query != "" {
		found, err := a.lib.SearchCards(query)
		if err != nil {
			slog.Error("remit: search cards", "err", err)
			found = nil
		}
		cards = keepSearched(cards, found)
	}
	rows := make([]browseRow, 0, len(cards))
	for _, card := range cards {
		rows = append(rows, a.browseRowOf(card))
	}
	a.browse.rows = rows
	a.browse.selected = clamp(a.browse.selected, -1, len(rows)-1)
}

// browseFilterCards is the store's answer for the filter chosen: the queue's
// once-started cards when it is due, and the cards stored with a status or lost
// otherwise. A card the queue has never reviewed belongs to New, not to Due.
func (a *App) browseFilterCards() []core.Card {
	switch a.browse.filter {
	case browseDue:
		cards := make([]core.Card, 0, len(a.due))
		for _, id := range a.due {
			if card, ok := a.data.byID[id]; ok && card.Status != core.StatusNew {
				cards = append(cards, card)
			}
		}
		return cards
	case browseNew:
		return a.cardsWithStatus(core.StatusNew)
	case browseSuspended:
		return a.cardsWithStatus(core.StatusSuspended)
	case browseLost:
		cards, err := a.lib.LostCards()
		if err != nil {
			slog.Error("remit: lost cards", "err", err)
			return nil
		}
		return cards
	default:
		return a.data.cards
	}
}

// cardsWithStatus is one status query, logged rather than shown: a filter that
// fails has no rows, and the empty state says so.
func (a *App) cardsWithStatus(s core.Status) []core.Card {
	cards, err := a.lib.CardsWithStatus(s)
	if err != nil {
		slog.Error("remit: cards with status", "status", s.String(), "err", err)
		return nil
	}
	return cards
}

// browseRowOf reads one card into a row: its prompt and the note the prompt
// lives in, both empty when its block is gone.
func (a *App) browseRowOf(card core.Card) browseRow {
	prompt, _, state := card.Sides(a.data.blocks)
	row := browseRow{card: card, prompt: prompt, state: state}
	if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
		row.note = a.data.titleOf(note)
	}
	return row
}

// keepSearched narrows cards to those the search found, keeping the order the
// filter gave them.
func keepSearched(cards, found []core.Card) []core.Card {
	if len(found) == 0 {
		return nil
	}
	hit := make(map[core.ID]bool, len(found))
	for _, card := range found {
		hit[card.ID] = true
	}
	out := make([]core.Card, 0, len(cards))
	for _, card := range cards {
		if hit[card.ID] {
			out = append(out, card)
		}
	}
	return out
}

// browseView draws browse mode: a search field, the filters, and the cards they
// found. The rows outlive the pass, so what the view derives from a keystroke is
// loaded after the fields have taken it.
func (a *App) browseView(c *ui.Context, tk tokens) {
	reload, open := false, -1

	ui.Column(c).Grow(1).Padding(tk.spaceXL).Gap(tk.spaceL).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m(c).BrowseTitle()).FontSize(tk.typeTitle).Bold()
			ui.Spacer(c)
			ui.Badge(c, cardCount(c, len(a.browse.rows)))
		})

		ui.Row(c).Gap(tk.spaceM).AlignItems(ui.Center).Children(func() {
			field := ui.SearchField(c, &a.browse.query).
				Placeholder(m(c).BrowseSearch()).Label(m(c).BrowseSearch()).
				MinHeight(tk.control).Grow(1).Bind(&a.browse.field)
			if field.Changed() {
				reload, open = true, -1
			}

			filter := int(a.browse.filter)
			if ui.Segmented(c, &filter,
				m(c).BrowseAll(), m(c).BrowseDue(), m(c).BrowseNew(), m(c).BrowseSuspended(), m(c).Lost(),
			).Label(m(c).BrowseFilters()).Changed() {
				a.browse.filter = browseFilter(filter)
				reload, open = true, -1
			}
		})

		a.browseRows(c, tk, &open)

		ui.Row(c).Gap(tk.spaceS).AlignItems(ui.Center).Children(func() {
			_, chosen := a.browse.chosen()
			if ui.Button(c, m(c).BrowseOpen()).MinHeight(tk.control).Disabled(!chosen).Clicked() {
				open = a.browse.selected
			}
			if len(a.browse.rows) == 0 {
				ui.Text(c, m(c).BrowseEmpty()).FontSize(tk.typeSmall).TextColor(tk.textMuted)
			}
		})
	})

	// The effects of this pass, once its fields have taken what was typed.
	if reload {
		a.loadBrowse()
		c.Invalidate()
	}
	if open >= 0 {
		a.openRow(c)
	}
}

// browseRows is the list of cards found: a row chooses on a click, and opens on
// Enter, a double click, or the Open button.
func (a *App) browseRows(c *ui.Context, tk tokens, open *int) {
	msgs := m(c)
	a.browse.list.Selected = &a.browse.selected
	a.browse.list.Key = func(row int) any { return a.browse.rows[row].card.ID }
	a.browse.list.Label = func(row int) string {
		if prompt := a.browse.rows[row].prompt; prompt != "" {
			return prompt
		}
		return msgs.Lost()
	}

	list := ui.List(c, &a.browse.list, len(a.browse.rows), func(i int) {
		a.browseRowView(c, tk, a.browse.rows[i])
	}).Grow(1).Label(msgs.BrowseTitle())
	if list.Submitted() {
		*open = a.browse.selected
	}
}

// browseRowView draws one card: its prompt, the note it came from, the state
// of its reference, and — for a lost one — the action that repairs it. It is a
// row of ui.List, not a widget of its own.
func (a *App) browseRowView(c *ui.Context, tk tokens, row browseRow) {
	ui.Row(c).Gap(tk.spaceM).AlignItems(ui.Center).Padding(tk.spaceS, tk.spaceM).Children(func() {
		ui.Column(c).Grow(1).Gap(tk.spaceXS).Children(func() {
			text, colour := row.prompt, tk.text
			if row.state == core.LinkLost {
				text, colour = m(c).LostBody(), tk.textMuted
			}
			ui.Text(c, text).FontSize(tk.typeBody).TextColor(colour).SingleLine()
			if row.note != "" {
				ui.Text(c, row.note).FontSize(tk.typeSmall).TextColor(tk.textMuted).SingleLine()
			}
		})
		switch {
		case row.state == core.LinkLost:
			stateBadge(c, tk, m(c).Lost(), tk.danger)
			ui.Button(c, m(c).Relink()).MinHeight(tk.control).
				OnClick(func() { a.openRelink(c, row.card.ID) })
		case row.state == core.LinkFrozen:
			stateBadge(c, tk, m(c).Frozen(), tk.warning)
		}
		if row.card.Status == core.StatusSuspended {
			ui.Badge(c, m(c).BrowseSuspended())
		}
	})
}

// stateBadge marks a row whose reference is not live text.
func stateBadge(c *ui.Context, tk tokens, label string, colour ui.Color) {
	ui.Badge(c, label).
		Background(tk.linkFill(colour)).TextColor(tk.linkInk(colour))
}

// openRow shows the row chosen: its note in the editor, or the card in review
// when its block is gone — where a lost card can still be suspended.
func (a *App) openRow(c *ui.Context) {
	row, ok := a.browse.chosen()
	if !ok {
		return
	}
	if note, ok := a.data.noteOfBlock(row.card.Ref.Block); ok {
		a.editor.selected = string(note)
		a.setMode(modeNotes)
		return
	}
	a.review.reset()
	a.review.queue = []core.ID{row.card.ID}
	a.setMode(modeReview)
}
