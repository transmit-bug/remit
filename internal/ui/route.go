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

// ShowReading enters the reading queue: the other side of the dual mode, with
// the notes that are due now.
func (a *App) ShowReading() {
	a.reload()
	a.mode = modeReading
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

// Counts is what a service outside the window reports of the queues: the cards
// to review now, and the notes to read now. The tray and the daily reminder
// report both, so reading is not drowned out by reviewing.
type Counts struct {
	Cards   int
	Reading int
}

// OnCounts registers a function told the queues' lengths whenever they are
// rebuilt; the tray shows both numbers. Setting it again replaces the previous
// one.
func (a *App) OnCounts(fn func(Counts)) {
	a.counts = fn
	if fn != nil {
		fn(a.queueCounts())
	}
}

// queueCounts is what the window reports: the cards left in today's queue and
// the notes due in the reading queue. Both are the store's answer as the last
// reload derived it, not the session's place in it.
func (a *App) queueCounts() Counts {
	return Counts{Cards: a.review.Remaining(), Reading: a.readingCount}
}
