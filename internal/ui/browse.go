package ui

import (
	"log/slog"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/browse"
)

// loadBrowse derives the rows browse shows from the store: the cards the filter
// keeps, narrowed by the query when there is one. The mode switch and reload
// call it, so the view only reads rows.
func (a *App) loadBrowse() {
	if a.lib == nil {
		a.browseRows = nil
		a.browse.Keep(0)
		return
	}
	cards := a.browseFilterCards()
	if query := strings.TrimSpace(a.browse.Query); query != "" {
		found, err := a.lib.SearchCards(query)
		if err != nil {
			slog.Error("remit: search cards", "err", err)
			found = nil
		}
		cards = keepSearched(cards, found)
	}
	rows := make([]browse.Row, 0, len(cards))
	for _, card := range cards {
		rows = append(rows, a.browseRowOf(card))
	}
	a.browseRows = rows
	a.browse.Keep(len(rows))
}

// browseFilterCards is the store's answer for the filter chosen: the queue's
// once-started cards when it is due, and the cards stored with a status or lost
// otherwise. A card the queue has never reviewed belongs to New, not to Due.
func (a *App) browseFilterCards() []core.Card {
	switch a.browse.Filter {
	case browse.Due:
		cards := make([]core.Card, 0, len(a.due))
		for _, id := range a.due {
			if card, ok := a.data.byID[id]; ok && card.Status != core.StatusNew {
				cards = append(cards, card)
			}
		}
		return cards
	case browse.New:
		return a.cardsWithStatus(core.StatusNew)
	case browse.Suspended:
		return a.cardsWithStatus(core.StatusSuspended)
	case browse.Lost:
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
func (a *App) browseRowOf(card core.Card) browse.Row {
	prompt, _, state := a.faces(card)
	row := browse.Row{ID: card.ID, Prompt: prompt, State: state, Suspended: card.Status == core.StatusSuspended}
	if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
		row.Note = a.data.titleOf(note)
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

// browseView draws browse; the screen itself lives in its own package, and this
// hands it the rows the app derived and the actions it owns (ADR 0009).
func (a *App) browseView(c *ui.Context) {
	browse.View(c, &a.browse, a.browseDeps(c))
}

// browseDeps is what the screen asks of the app: the rows it derived, the
// reload the screen's query and filter ask for, and the two actions a row
// carries, each run here on the UI thread.
func (a *App) browseDeps(c *ui.Context) browse.Deps {
	return browse.Deps{
		Rows:   func() []browse.Row { return a.browseRows },
		Reload: a.loadBrowse,
		Open:   a.openBrowseRow,
		Relink: func(id core.ID) { a.openRelink(c, id) },
	}
}

// openBrowseRow shows the row chosen: its note in the editor, or the card in
// review when its block is gone — where a lost card can still be suspended.
func (a *App) openBrowseRow(id core.ID) {
	card, ok := a.data.card(id)
	if !ok {
		return
	}
	if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
		a.notes.Selected = string(note)
		a.setMode(modeNotes)
		return
	}
	a.review.Reset()
	a.review.Only(card.ID)
	a.setMode(modeReview)
}
