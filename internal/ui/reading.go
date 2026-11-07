package ui

import (
	"errors"
	"log/slog"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/screens/reading"
)

// readingView draws the reading session: the screen itself lives in its own
// package, and this hands it the queue the app derived and the app's own writes
// (ADR 0004).
func (a *App) readingView(c *ui.Context) {
	reading.View(c, &a.reading, a.readingDeps(c))
}

// readingDeps is what the screen asks of the app: the note to read, and what the
// user can do with it, each recorded by the app on the UI thread.
func (a *App) readingDeps(c *ui.Context) reading.Deps {
	return reading.Deps{
		Note: a.readingNote,
		Read: func(id core.ID, r reading.Rating, position core.ID) { a.readNote(c, id, r, position) },
		Skip: func(id core.ID) { a.skipReading(id) },
		Drop: func(id core.ID) { a.dropReading(c, id) },
		Excerpt: func(note, block core.ID, withCard bool) {
			a.excerptBlock(c, note, block, withCard)
		},
		Source: a.sourceOf,
		Open:   a.openNote,
		EditNote: func(id core.ID) {
			a.openNote(id)
		},
		Review: func() { a.setMode(modeReview) },
		Home:   func() { a.setMode(modeNotes) },
	}
}

// readingNote is the note the reading screen shows: its blocks in file order,
// and the block the last session stopped at.
func (a *App) readingNote(id core.ID) (reading.Note, bool) {
	note, ok := a.data.note(id)
	if !ok {
		return reading.Note{}, false
	}
	return reading.Note{
		ID:       note.ID,
		Title:    note.Title,
		Blocks:   note.Blocks,
		Position: a.readingPositions[id],
	}, true
}

// currentReadingNote is the note on the reading screen, when an action needs the
// note's own paragraphs.
func (a *App) currentReadingNote() (reading.Note, bool) {
	id, ok := a.reading.Current()
	if !ok {
		return reading.Note{}, false
	}
	return a.readingNote(id)
}

// readNote records a reading session — the difficulty and the block it stopped
// at — and moves to the next note. The note leaves the screen either way: what
// the session recorded is what brings it back.
func (a *App) readNote(c *ui.Context, id core.ID, r reading.Rating, position core.ID) {
	if a.lib == nil {
		return
	}
	if err := a.lib.ReadNote(id, r.Schedule(), position, time.Now()); err != nil {
		a.report(c, "read note", err)
		return
	}
	a.reading.Advance(true)
	a.reload()
	c.Announce(m(c).ReadingWaiting(a.reading.Remaining()))
}

// readCurrent is the digit key: it records how the note on screen digested.
func (a *App) readCurrent(c *ui.Context, r reading.Rating) {
	id, ok := a.reading.Current()
	if !ok {
		return
	}
	a.readNote(c, id, r, a.reading.Cursor(a.readingNoteOrEmpty(id)))
}

// skipReading moves past a note without recording anything, so an interrupted
// reading is not a failure the schedule counts.
func (a *App) skipReading(id core.ID) {
	if current, ok := a.reading.Current(); !ok || current != id {
		return
	}
	a.reading.Advance(false)
	a.reload()
}

// dropReading takes a note out of the reading queue and off the screen.
func (a *App) dropReading(c *ui.Context, id core.ID) {
	if a.lib == nil {
		return
	}
	if err := a.lib.RemoveFromReading(id, time.Now()); err != nil {
		a.report(c, "remove from reading", err)
		return
	}
	if current, ok := a.reading.Current(); ok && current == id {
		a.reading.Advance(false)
	}
	a.reload()
	c.Toast(m(c).ReadingRemoved())
}

// addReading puts a note in the reading queue, which is what the notebook's
// command does with the note open there.
func (a *App) addReading(c *ui.Context, id core.ID) {
	if a.lib == nil || id == "" {
		return
	}
	if err := a.lib.AddToReading(id, time.Now()); err != nil {
		a.report(c, "add to reading", err)
		return
	}
	a.reload()
	c.Toast(m(c).ReadingAdded())
}

// excerptBlock takes the text of a block out into a child note, and makes a card
// of the new block when the reader asked for one. The card goes through the same
// resolver the notebook uses: an excerpt is a note like any other.
func (a *App) excerptBlock(c *ui.Context, noteID, blockID core.ID, withCard bool) {
	if a.lib == nil {
		return
	}
	note, ok := a.data.note(noteID)
	if !ok {
		return
	}
	block, ok := note.Block(blockID)
	if !ok {
		return
	}
	child, err := a.lib.Excerpt(noteID, blockID, block.Text, m(c).ExcerptTitle(note.Title))
	if err != nil {
		a.report(c, "excerpt", err)
		return
	}
	madeCard := false
	if withCard && len(child.Blocks) > 0 {
		if derived, ok := a.deriveCards(child.Blocks[0].Text); ok {
			if _, err := a.lib.CreateCards(child.Blocks[0].ID, derived); err != nil {
				if !errors.Is(err, store.ErrNothingToAsk) {
					a.report(c, "make card", err)
				}
			} else {
				madeCard = true
			}
		}
	}
	a.reload()
	if madeCard {
		c.Toast(m(c).ExcerptSavedCard())
		return
	}
	c.Toast(m(c).ExcerptSaved())
}

// sourceOf is the note a block's reference link names: the way back from an
// excerpt to the passage it was taken from.
func (a *App) sourceOf(block core.Block) (core.ID, bool) {
	for _, link := range core.Links(block.Text) {
		if note, ok := a.data.byTitle[core.NormalizeTitle(link.Title)]; ok {
			return note, true
		}
		if link.Block != "" {
			if note, ok := a.data.noteOfBlock(link.Block); ok {
				return note, true
			}
		}
	}
	return "", false
}

// moveReading moves the reading position within the note on screen, which is the
// key a long note is read with.
func (a *App) moveReading(delta int) {
	note, ok := a.currentReadingNote()
	if !ok {
		return
	}
	a.reading.Move(note, delta)
}

// excerptCurrent is the excerpt key: it runs the same gesture the screen's
// button does, so the reading position and the focus come back the same way.
func (a *App) excerptCurrent(c *ui.Context) {
	note, ok := a.currentReadingNote()
	if !ok {
		return
	}
	a.reading.Excerpt(c, note, a.readingDeps(c))
}

// openNote shows a note in the notebook.
func (a *App) openNote(id core.ID) {
	if _, ok := a.data.note(id); !ok {
		return
	}
	a.notes.Selected = string(id)
	a.setMode(modeNotes)
}

// openNoteID is the note the notebook has open, if any: what a command that
// acts on "this note" acts on.
func (a *App) openNoteID() core.ID {
	return a.notes.Draft.ID
}

// skipCurrentReading is the skip key: it moves past the note on screen.
func (a *App) skipCurrentReading() {
	if id, ok := a.reading.Current(); ok {
		a.skipReading(id)
	}
}

// dropCurrentReading is the remove key: it takes the note on screen out of the
// reading queue.
func (a *App) dropCurrentReading(c *ui.Context) {
	if id, ok := a.reading.Current(); ok {
		a.dropReading(c, id)
	}
}

// editCurrentReading opens the note being read in the notebook.
func (a *App) editCurrentReading() {
	if id, ok := a.reading.Current(); ok {
		a.openNote(id)
	}
}

// readingNoteOrEmpty is the note on screen, or none when it is gone.
func (a *App) readingNoteOrEmpty(id core.ID) reading.Note {
	note, _ := a.readingNote(id)
	return note
}

// loadReading derives the reading queue and the block each note resumes at. It
// is a load path: the screen reads what it leaves, so no frame asks the store.
func (a *App) loadReading(now time.Time) {
	a.readingPositions = map[core.ID]core.ID{}
	a.readingCount = 0
	if a.lib == nil {
		return
	}
	// The positions come from every queued note, due or not: a note read a
	// moment ago is where the reader left it, and opening it again resumes
	// there whether or not the schedule has brought it back.
	queued, err := a.lib.ReadingQueue()
	if err != nil {
		slog.Error("remit: reading queue", "err", err)
		return
	}
	for _, entry := range queued {
		a.readingPositions[entry.Note] = entry.Position
	}
	due, err := queue.Reading(a.lib, now, queue.Options{})
	if err != nil {
		slog.Error("remit: reading queue", "err", err)
		return
	}
	ids := make([]core.ID, 0, len(due))
	for _, entry := range due {
		ids = append(ids, entry.Note)
	}
	a.reading.Sync(ids)
	a.readingCount = len(due)
}
