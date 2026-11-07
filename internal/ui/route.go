package ui

import (
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/browse"
)

// ShowNote opens the editor at a note the library holds, and does nothing when
// it does not: a link from outside is a request, not a command.
func (a *App) ShowNote(id core.ID) {
	a.reload()
	for _, note := range a.data.notes {
		if note.ID == id {
			a.notes.Selected = string(id)
			a.mode = modeNotes
			return
		}
	}
}

// ShowCard opens review with one card first, so a link to a card shows that
// card rather than the start of the queue.
func (a *App) ShowCard(id core.ID) {
	a.reload()
	if _, ok := a.data.byID[id]; !ok {
		return
	}
	a.review.Reset()
	a.review.Only(id)
	a.mode = modeReview
}

// ShowQueue enters review with today's queue.
func (a *App) ShowQueue() {
	a.reload()
	a.mode = modeReview
}

// Search shows browse on a query from outside the window and puts the caret in
// its field. A query that finds nothing leaves its empty state up, which says
// the search ran.
func (a *App) Search(query string) {
	a.browse.Query, a.browse.Filter, a.browse.Selected = query, browse.All, -1
	a.mode = modeBrowse
	a.reload()
	a.browse.Focus()
}

// OnRemaining registers a function told how many cards remain whenever the
// queue is rebuilt; the tray shows that number. Setting it again replaces the
// previous one.
func (a *App) OnRemaining(fn func(int)) {
	a.remaining = fn
	if fn != nil {
		fn(a.review.Remaining())
	}
}
