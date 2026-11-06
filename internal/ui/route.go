package ui

import "github.com/transmit-bug/remit/internal/core"

// ShowNote opens the editor at a note the library holds, and does nothing when
// it does not: a link from outside is a request, not a command.
func (a *App) ShowNote(id core.ID) {
	a.reload()
	for _, note := range a.data.notes {
		if note.ID == id {
			a.editor.selected = string(id)
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
	a.review.reset()
	a.review.queue = []core.ID{id}
	a.mode = modeReview
}

// ShowQueue enters review with today's queue.
func (a *App) ShowQueue() {
	a.reload()
	a.mode = modeReview
}

// OnRemaining registers a function told how many cards remain whenever the
// queue is rebuilt; the tray shows that number. Setting it again replaces the
// previous one.
func (a *App) OnRemaining(fn func(int)) {
	a.remaining = fn
	if fn != nil {
		fn(a.review.remaining())
	}
}
