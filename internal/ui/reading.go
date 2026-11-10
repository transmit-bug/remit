package ui

import (
	"errors"
	"log/slog"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/screens/read"
	"github.com/transmit-bug/remit/internal/ui/screens/reading"
)

// readingView draws the reading session: the screen itself lives in its own
// package, and this hands it the queue the app derived and the app's own writes
// (ADR 0004).
func (a *App) readingView(c *ui.Context) {
	reading.View(c, &a.reading, a.readingDeps(c))
}

// readView draws the read area's page: today's queue, or the archive. The rows
// are what the last load derived, so the page reads a snapshot and never the
// store.
func (a *App) readView(c *ui.Context) {
	read.View(c, &a.read, a.readDeps(c))
}

// readDeps is what the read area's pages ask of the app: the rows it derived,
// the rhythm under them, and what a row can do. Each runs on the UI thread.
func (a *App) readDeps(c *ui.Context) read.Deps {
	return read.Deps{
		Queue: func() []read.Item { return a.readPages.queue },
		Archive: func() (archived, dismissed []read.Item) {
			return a.readPages.archived, a.readPages.dismissed
		},
		Counts: func() read.Counts {
			return read.Counts{Due: len(a.readPages.queue), Queued: a.readPages.queued}
		},
		Now:      time.Now,
		Start:    func() { a.startReading("") },
		Continue: func(id core.ID) { a.startReading(id) },
		Add:      func() { a.setArea(areaWrite, subNoteList) },
		Open:     a.openNote,
		Restore:  func(id core.ID) { a.restoreReading(c, id) },
		Reload:   a.reload,
	}
}

// readingDeps is what the screen asks of the app: the note to read, and what the
// user can do with it, each recorded by the app on the UI thread.
func (a *App) readingDeps(c *ui.Context) reading.Deps {
	// The terminal actions act on the note on screen, which is the one the session
	// shows when the frame asks; the keys and the buttons both come through here.
	current, _ := a.reading.Current()
	return reading.Deps{
		Note:     a.readingNote,
		Read:     func(id core.ID, r reading.Rating, position core.ID) { a.readNote(c, id, r, position) },
		Skip:     func(id core.ID) { a.skipReading(id) },
		Drop:     func(id core.ID) { a.dropReading(c, id) },
		Archive:  func() { a.archiveReading(c, current) },
		Dismiss:  func() { a.dismissReading(c, current) },
		Postpone: func() { a.postponeReading(c, current) },
		QueueNote: func(id core.ID) {
			if err := a.queueReading(id); err != nil {
				slog.Error("remit: restore reading", "err", err)
			}
		},
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
// the block the last session stopped at, and what each paragraph already
// carries.
func (a *App) readingNote(id core.ID) (reading.Note, bool) {
	note, ok := a.data.note(id)
	if !ok {
		return reading.Note{}, false
	}
	position := a.readingPositions[id]
	out := reading.Note{
		ID:       note.ID,
		Title:    note.Title,
		Blocks:   note.Blocks,
		Position: position,
		Marks:    map[core.ID]reading.Mark{},
	}
	for _, block := range note.Blocks {
		mark := reading.Mark{
			Cards:     a.data.cardsForBlock(block.ID),
			Excerpted: a.excerpted[block.ID],
			LastStop:  position != "" && block.ID == position,
		}
		if mark.Cards == 0 && !mark.Excerpted && !mark.LastStop {
			continue
		}
		out.Marks[block.ID] = mark
	}
	return out, true
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

// archiveReading puts a note away for good: it leaves the queue, keeps its
// history and the block it stopped at, and comes back only if it is queued again.
// The session reports the decision and offers the undo; this is the write.
func (a *App) archiveReading(c *ui.Context, id core.ID) {
	if a.lib == nil || id == "" {
		return
	}
	if err := a.lib.ArchiveReading(id, time.Now()); err != nil {
		a.report(c, "archive reading", err)
		return
	}
	a.reload()
}

// dismissReading says a note is not worth reading: the other terminal state, and
// the same way back.
func (a *App) dismissReading(c *ui.Context, id core.ID) {
	if a.lib == nil || id == "" {
		return
	}
	if err := a.lib.DismissReading(id, time.Now()); err != nil {
		a.report(c, "dismiss reading", err)
		return
	}
	a.reload()
}

// postponeReading brings a note back tomorrow: it keeps its place in the queue
// and its rating history, and today's queue stops holding it.
func (a *App) postponeReading(c *ui.Context, id core.ID) {
	if a.lib == nil || id == "" {
		return
	}
	now := time.Now()
	if err := a.lib.PostponeReading(id, startOfTomorrow(now), now); err != nil {
		a.report(c, "postpone reading", err)
		return
	}
	a.reload()
}

// startOfTomorrow is the midnight after the day now falls on, in now's own zone:
// a postponed note comes back with the day, not an hour later.
func startOfTomorrow(now time.Time) time.Time {
	year, month, day := now.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, now.Location())
}

// queueReading puts a note back in the reading queue, which is what undoing the
// session's terminal decisions and the archive's restore both do. A note the
// schedule had put off comes due now too, or it would join the queue and still
// not be on today's page.
func (a *App) queueReading(id core.ID) error {
	if a.lib == nil || id == "" {
		return nil
	}
	now := time.Now()
	if err := a.lib.AddToReading(id, now); err != nil {
		return err
	}
	if row, ok := a.lib.ReadingFor(id); ok && row.Due().After(now) {
		return a.lib.PostponeReading(id, now, now)
	}
	return nil
}

// restoreReading is the archive page's way back: a note that left the queue for
// good is queued again, and the reader is told it worked.
func (a *App) restoreReading(c *ui.Context, id core.ID) {
	if err := a.queueReading(id); err != nil {
		a.report(c, "restore reading", err)
		return
	}
	a.reload()
	c.Toast(m(c).ReadRestored())
}

// startReading enters the reading session, at the note the reader asked for or at
// the head of the queue. The session takes the window whole; the queue page stays
// behind it.
func (a *App) startReading(id core.ID) {
	if a.lib == nil {
		return
	}
	a.reading.Begin(id)
	if a.reading.Remaining() == 0 {
		return
	}
	a.session = sessionReading
	a.syncMode()
	a.loadCurrent()
}

// endCurrentReading is the key form of the session's terminal actions: the same
// gesture the buttons run, so the note leaves the screen and the reader is told
// with the undo either way.
func (a *App) endCurrentReading(c *ui.Context, decision reading.Decision) {
	note, ok := a.currentReadingNote()
	if !ok {
		return
	}
	a.reading.End(c, a.readingDeps(c), note, decision)
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

// readPages is the read area as a load derived it: the queue page's rows, the
// archive's two sections, and how many notes the reading queue holds at all.
// The pages read it, so no frame asks the store.
type readPages struct {
	queue     []read.Item
	archived  []read.Item
	dismissed []read.Item
	queued    int
}

// loadReading derives the reading session's queue, the block each note resumes
// at, and both read pages' rows. It is a load path: the screens read what it
// leaves, so no frame asks the store.
func (a *App) loadReading(now time.Time) {
	a.readingPositions = map[core.ID]core.ID{}
	a.readingCount = 0
	a.readPages = readPages{}
	a.read.Loaded, a.read.Err = true, ""
	if a.lib == nil {
		return
	}
	// The positions come from every queued note, due or not: a note read a
	// moment ago is where the reader left it, and opening it again resumes
	// there whether or not the schedule has brought it back.
	queued, err := a.lib.ReadingQueue()
	if err != nil {
		a.readFailed("reading queue", err)
		return
	}
	for _, entry := range queued {
		a.readingPositions[entry.Note] = entry.Position
	}
	due, err := queue.Reading(a.lib, now, queue.Options{})
	if err != nil {
		a.readFailed("reading queue", err)
		return
	}
	ids := make([]core.ID, 0, len(due))
	for _, entry := range due {
		ids = append(ids, entry.Note)
		a.readPages.queue = append(a.readPages.queue, a.readItem(entry))
	}
	a.reading.Sync(ids)
	a.readingCount = len(due)
	a.readPages.queued = len(queued)

	// The archive is the notes that left the queue for good, in the two sections
	// the archive page draws: put away after reading, and not worth reading.
	archive, err := a.lib.ReadingArchive()
	if err != nil {
		a.readFailed("reading archive", err)
		return
	}
	for _, entry := range archive {
		item := a.readItem(entry)
		if entry.Dismissed {
			a.readPages.dismissed = append(a.readPages.dismissed, item)
			continue
		}
		a.readPages.archived = append(a.readPages.archived, item)
	}
}

// readUnread records why the read area has no rows: the pages say so and offer to
// ask again, rather than showing an empty queue that is not empty.
func (a *App) readFailed(what string, err error) {
	slog.Error("remit: "+what, "err", err)
	a.read.Err = err.Error()
}

// readItem is one reading entry as the read area lists it: the note's name and
// length, the paragraph it resumes at, and when it was last read.
func (a *App) readItem(entry store.Reading) read.Item {
	item := read.Item{Note: entry.Note, Due: entry.Due(), LastSeen: entry.State.LastReview}
	note, ok := a.data.note(entry.Note)
	if !ok {
		return item
	}
	item.Title, item.Blocks = note.Title, len(note.Blocks)
	for i, block := range note.Blocks {
		if block.ID == entry.Position {
			item.At = i + 1
			break
		}
	}
	return item
}

// excerptedBlocks names every block something quotes: a paragraph's marks say
// which of them already became an excerpt. It is derived with the snapshot, so no
// frame reads the library's links.
func (a *App) excerptedBlocks() map[core.ID]bool {
	out := map[core.ID]bool{}
	for _, note := range a.data.notes {
		for _, block := range note.Blocks {
			for _, link := range core.Links(block.Text) {
				if link.Block != "" {
					out[link.Block] = true
				}
			}
		}
	}
	return out
}
