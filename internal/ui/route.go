package ui

import (
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/browse"
)

// ShowNote opens the notebook at a note the library holds, and does nothing when
// it does not: a link from outside is a request, not a command.
func (a *App) ShowNote(id core.ID) {
	a.reload()
	for _, note := range a.data.notes {
		if note.ID == id {
			a.notes.Selected = string(id)
			a.setArea(areaWrite, subNotes)
			return
		}
	}
}

// ShowCard opens the recall queue with one card first, so a link to a card shows
// that card rather than the start of the queue.
func (a *App) ShowCard(id core.ID) {
	a.reload()
	if _, ok := a.data.byID[id]; !ok {
		return
	}
	a.review.Reset()
	a.review.Only(id)
	a.setArea(areaRecall, subRecallQueue)
}

// ShowToday opens the window on today.
func (a *App) ShowToday() {
	a.reload()
	a.setArea(areaToday, subNone)
}

// ShowRead opens the read area's queue: the notes that are due now.
func (a *App) ShowRead() {
	a.reload()
	a.setArea(areaRead, subReadQueue)
}

// ShowRecall opens the recall area's queue: today's cards.
func (a *App) ShowRecall() {
	a.reload()
	a.setArea(areaRecall, subRecallQueue)
}

// ShowQueue enters the recall queue, the link remit://queue has always named.
func (a *App) ShowQueue() {
	a.reload()
	a.setArea(areaRecall, subRecallQueue)
}

// ShowReading enters the reading queue, with the notes that are due now.
func (a *App) ShowReading() {
	a.reload()
	a.setArea(areaRead, subReadQueue)
}

// Search shows the search page on a query from outside the window and puts the
// caret in its field. A query that finds nothing leaves its empty state up,
// which says the search ran.
func (a *App) Search(query string) {
	a.browse.Query, a.browse.Filter, a.browse.Selected = query, browse.All, -1
	a.setArea(areaWrite, subSearch)
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
