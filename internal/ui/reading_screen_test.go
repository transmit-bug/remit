package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/locales"
)

// readingApp is a window on a library with two notes in the reading queue, both
// added just now: the queue the screen reads is the store's answer.
func readingApp(t *testing.T) (*App, testLibrary) {
	t.Helper()
	a, seed := newTestApp(t)
	if err := a.lib.AddToReading(seed.memory.ID, time.Now()); err != nil {
		t.Fatalf("add to reading: %v", err)
	}
	if err := a.lib.AddToReading(seed.writing.ID, time.Now()); err != nil {
		t.Fatalf("add to reading: %v", err)
	}
	a.reload()
	return a, seed
}

// openReading puts the window in the reading session, entered from the read
// area the way the Start button enters it.
func openReading(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	a.startReading("")
	if a.session != sessionReading {
		t.Fatal("the reading session did not open")
	}
	tt := ui.NewTester(a.View, w, h)
	settle(tt)
	return tt
}

// The read area's page is the queue: the notes due to read, with the session one
// gesture away. The session's own controls are not on the page — they belong to
// the session the page opens.
func TestTheReadingQueueIsItsOwnArea(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, seed := readingApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			tt.Key(ui.Cmd, ui.Key3)
			tt.Advance(320 * time.Millisecond) // the page's entrance

			for _, want := range []string{
				msgs.ReadQueueTitle(), seed.memory.Title, seed.writing.Title, msgs.ReadStart(),
			} {
				if !tt.HasText(want) {
					t.Fatalf("the reading queue lost %q; texts %q", want, tt.Texts())
				}
			}
			// The review queue is the other area: nothing of it is on this page.
			if tt.HasText("Show answer") {
				t.Error("the review page is up on the read area")
			}
			// The session's controls are not on the queue page.
			if tt.HasText(msgs.Excerpt()) {
				t.Error("the session's actions are on the queue page")
			}
		})
	}

	t.Run("controls", func(t *testing.T) {
		msgs := locales.In("en")
		a, _ := readingApp(t)
		tt := ui.NewTester(a.View, 1024, 720)
		settle(tt)
		tt.Key(ui.Cmd, ui.Key3)
		tt.Advance(320 * time.Millisecond)
		assertControl(t, tt, 1024, 720, msgs.ReadStart())
		assertControl(t, tt, 1024, 720, msgs.ReadAdd())
	})
}

// A session records what it read and how hard it was: the note leaves the screen,
// the queue shrinks, and the log carries the difficulty and the block the reader
// stopped at.
func TestReadingSessionWritesTheLog(t *testing.T) {
	msgs := locales.In("en")
	a, seed := readingApp(t)
	tt := openReading(t, a, 1024, 720)
	if !tt.HasText(msgs.ReadingProgress(0, 2)) {
		t.Fatalf("the session does not start at none of two; texts %q", tt.Texts())
	}

	// J moves the reading position to the second paragraph, which is what the
	// session records.
	tt.Key(0, ui.KeyJ)
	if !tt.HasText(msgs.ReadingWhere(2, len(seed.memory.Blocks))) {
		t.Fatalf("the position did not move; texts %q", tt.Texts())
	}
	// 2 is "just right": the note is read, not skipped.
	tt.Key(0, ui.Key2)

	events, err := a.lib.ReadingEvents(seed.memory.ID)
	if err != nil {
		t.Fatalf("ReadingEvents: %v", err)
	}
	var read []schedule.Event
	for _, ev := range events {
		if ev.Kind == schedule.Reviewed {
			read = append(read, ev)
		}
	}
	if len(read) != 1 {
		t.Fatalf("the log holds %d reading sessions, want 1", len(read))
	}
	if read[0].Rating != schedule.Good {
		t.Errorf("rating = %v, want Good for just right", read[0].Rating)
	}
	if read[0].Position != seed.memory.Blocks[1].ID {
		t.Errorf("position = %q, want the second paragraph", read[0].Position)
	}

	if !tt.HasText(msgs.ReadingProgress(1, 2)) {
		t.Errorf("the session did not move on; texts %q", tt.Texts())
	}
	if got, _ := a.reading.Current(); got != seed.writing.ID {
		t.Errorf("the note on screen is %q, want the second note", got)
	}
	if !tt.HasText("Writing cards") {
		t.Errorf("the second note is not on screen; texts %q", tt.Texts())
	}

	// The session is in the plain-text log, not only in the index.
	logged, err := os.ReadFile(filepath.Join(a.dir, "reviews.jsonl"))
	if err != nil {
		t.Fatalf("read reviews.jsonl: %v", err)
	}
	log := string(logged)
	if !strings.Contains(log, `"note":"`+string(seed.memory.ID)+`"`) ||
		!strings.Contains(log, `"position":"`+string(seed.memory.Blocks[1].ID)+`"`) {
		t.Errorf("reviews.jsonl does not hold the reading session:\n%s", log)
	}
}

// The block a session stopped at is the block the next session opens at: the app
// hands the screen the position the store replayed.
func TestReadingResumesWhereItStopped(t *testing.T) {
	a, seed := readingApp(t)
	tt := openReading(t, a, 1024, 720)
	tt.Key(0, ui.KeyJ) // the second paragraph
	tt.Key(0, ui.Key1) // too hard: it comes back soon

	// The note left the queue for now, but where it stopped is kept.
	position, ok := a.lib.ReadingPosition(seed.memory.ID)
	if !ok || position != seed.memory.Blocks[1].ID {
		t.Fatalf("the store kept %q (%v), want the second paragraph", position, ok)
	}
	note, ok := a.readingNote(seed.memory.ID)
	if !ok {
		t.Fatal("the reading screen cannot read a note the library holds")
	}
	if note.Position != seed.memory.Blocks[1].ID {
		t.Errorf("the screen was handed position %q, want the second paragraph", note.Position)
	}
}

// Excerpting is the bridge from reading to remembering: the paragraph becomes a
// child note that points back at it, and the source lists it as a backlink.
func TestExcerptFromTheReadingScreen(t *testing.T) {
	msgs := locales.In("en")
	a, seed := readingApp(t)
	tt := openReading(t, a, 1024, 720)

	before := len(a.data.notes)
	if err := tt.Click(msgs.Excerpt()); err != nil {
		t.Fatal(err)
	}
	if got := len(a.data.notes); got != before+1 {
		t.Fatalf("the library holds %d notes, want one more", got)
	}
	var child core.Note
	for _, note := range a.data.notes {
		if note.Title == msgs.ExcerptTitle(seed.memory.Title) {
			child = note
		}
	}
	if child.ID == "" {
		t.Fatalf("the excerpt is not in the library; texts %q", tt.Texts())
	}
	if len(child.Blocks) != 1 || !strings.HasPrefix(child.Blocks[0].Text, "Spacing beats cramming ::") {
		t.Fatalf("the excerpt does not quote the paragraph: %+v", child.Blocks)
	}
	links := core.Links(child.Blocks[0].Text)
	if len(links) != 1 || links[0].Block != seed.memory.Blocks[0].ID {
		t.Fatalf("the excerpt does not point at its source paragraph: %+v", links)
	}
	backlinks := a.lib.Backlinks(seed.memory.ID)
	if len(backlinks) == 0 || backlinks[0].Note != child.ID {
		t.Fatalf("the source's backlinks = %+v, want the excerpt", backlinks)
	}

	// The same gesture from the keyboard, with the checkbox set, makes a card of
	// the excerpt's block: the two paths are one.
	if err := tt.Click(msgs.ExcerptWithCard()); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyE)
	withCard := core.Note{}
	for _, note := range a.data.notes {
		if note.Title == msgs.ExcerptTitle(seed.memory.Title) && note.ID != child.ID {
			withCard = note
		}
	}
	if withCard.ID == "" {
		t.Fatal("the second excerpt is not in the library")
	}
	if a.data.cardsForBlock(withCard.Blocks[0].ID) == 0 {
		t.Error("the excerpt with a card made no card")
	}
}

// Nothing keeps a note in the queue that the user took out, and skipping counts
// as neither a reading nor a failure.
func TestReadingSkipAndRemove(t *testing.T) {
	msgs := locales.In("en")
	a, seed := readingApp(t)
	tt := openReading(t, a, 1024, 720)

	if err := tt.Click(msgs.Skip()); err != nil {
		t.Fatal(err)
	}
	events, err := a.lib.ReadingEvents(seed.memory.ID)
	if err != nil {
		t.Fatalf("ReadingEvents: %v", err)
	}
	for _, ev := range events {
		if ev.Kind == schedule.Reviewed {
			t.Error("a skipped note recorded a reading")
		}
	}
	if got, _ := a.reading.Current(); got != seed.writing.ID {
		t.Fatalf("skipping did not move on; on screen %q", got)
	}

	if err := tt.Click(msgs.RemoveReading()); err != nil {
		t.Fatal(err)
	}
	queued, err := a.lib.ReadingQueue()
	if err != nil {
		t.Fatalf("ReadingQueue: %v", err)
	}
	for _, entry := range queued {
		if entry.Note == seed.writing.ID {
			t.Errorf("the removed note is still in the reading queue: %+v", entry)
		}
	}
	// The session is done: the screen says so and offers the other queue.
	if !tt.HasText(msgs.NoReading()) {
		t.Errorf("the screen does not say the session is over; texts %q", tt.Texts())
	}
	assertControl(t, tt, 1024, 720, msgs.ModeReview())
}

// The numbers the tray reports are both queues', so a reader is not drowned out
// by the cards that are due.
func TestReadingCountsReachAService(t *testing.T) {
	a, seed := readingApp(t)
	var got []Counts
	a.OnCounts(func(c Counts) { got = append(got, c) })
	if len(got) == 0 {
		t.Fatal("the service was told nothing")
	}
	if last := got[len(got)-1]; last.Cards != 4 || last.Reading != 2 {
		t.Fatalf("the service was told %+v, want 4 cards and 2 notes", last)
	}

	// Removing one note moves the reading count.
	if err := a.lib.RemoveFromReading(seed.memory.ID, time.Now()); err != nil {
		t.Fatalf("RemoveFromReading: %v", err)
	}
	a.reload()
	if last := got[len(got)-1]; last.Reading != 1 {
		t.Errorf("after one removal the service was told %d notes, want 1", last.Reading)
	}
}

// The screen is reachable from the palette, so a note can be queued from the
// notebook with no pointer and no second entry.
func TestPaletteCommandsForReading(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	// The command acts on the note open in the notebook, so the notebook shows
	// it first.
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)

	tt.Key(ui.Cmd, ui.KeyK)
	if err := tt.Click(msgs.ActionAddReading()); err != nil {
		t.Fatalf("the palette has no add-to-reading action: %v", err)
	}
	queued, err := a.lib.ReadingQueue()
	if err != nil {
		t.Fatalf("ReadingQueue: %v", err)
	}
	if len(queued) != 1 || queued[0].Note != a.notes.Draft.ID {
		t.Fatalf("the reading queue = %+v, want the note that was open", queued)
	}

	a.setMode(modeNotes)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	if err := tt.Click(msgs.ActionReading()); err != nil {
		t.Fatalf("the palette has no reading-queue action: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(a.data.titleOf(a.notes.Draft.ID)) {
		t.Errorf("the palette's reading command did not open the queue; texts %q", tt.Texts())
	}
}

// readPage puts the window on the read area, and then on one of its two pages:
// the queue it opens on, or the archive beside it.
func readPage(t *testing.T, tt *ui.Tester, page string) {
	t.Helper()
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click(page); err != nil {
		t.Fatalf("the read area has no %q page: %v", page, err)
	}
	tt.Advance(320 * time.Millisecond)
}

// The read area's queue page opens the session: Start takes the window whole,
// with the first note on screen and the session's own actions.
func TestReadingQueueOpensTheSession(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, seed := readingApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			tt.Key(ui.Cmd, ui.Key3)
			tt.Advance(320 * time.Millisecond)
			if !tt.HasText(msgs.ReadQueueTitle()) {
				t.Fatalf("the read area is not on its queue; texts %q", tt.Texts())
			}
			if err := tt.Click(msgs.ReadStart()); err != nil {
				t.Fatalf("the queue has no Start: %v", err)
			}
			tt.Frame()

			if a.session != sessionReading {
				t.Fatalf("Start left the window on session %v", a.session)
			}
			if !tt.HasText(msgs.ReadingProgress(0, 2)) {
				t.Errorf("the session does not start at none of two; texts %q", tt.Texts())
			}
			if !tt.HasText(seed.memory.Title) || !tt.HasText(msgs.Excerpt()) {
				t.Errorf("the first note is not on screen; texts %q", tt.Texts())
			}
			// The session takes the window whole: no rail, no sub-navigation.
			if _, ok := tt.Find(msgs.AreaToday()); ok {
				t.Error("the rail is up in the session")
			}
			if tt.HasText(msgs.ReadQueueTitle()) {
				t.Error("the queue page is still under the session")
			}
		})
	}
}

// The empty queue's way out is the notes list: from there a note is queued, and
// the queue page lists it.
func TestReadingQueueAddsANoteFromTheList(t *testing.T) {
	msgs := locales.In("en")
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(msgs.ReadQueueEmpty()) {
		t.Fatalf("the empty queue says nothing; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.ReadAdd()); err != nil {
		t.Fatalf("the empty queue offers no way to add: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.area != areaWrite || a.sub != subNotes {
		t.Fatalf("Add went to %v/%v, want the notes list", a.area, a.sub)
	}
	if !tt.HasText(seed.memory.Title) {
		t.Fatalf("the notes list does not show the library; texts %q", tt.Texts())
	}

	// A note from that list, queued by the command that acts on the note open.
	if err := tt.Click(seed.memory.Title); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	if err := tt.Click(msgs.ActionAddReading()); err != nil {
		t.Fatalf("the palette has no add-to-reading action: %v", err)
	}
	readPage(t, tt, msgs.QueuePage())

	if !tt.HasText(seed.memory.Title) {
		t.Errorf("the queue page does not list the note that was added; texts %q", tt.Texts())
	}
	queued, err := a.lib.ReadingQueue()
	if err != nil {
		t.Fatalf("ReadingQueue: %v", err)
	}
	if len(queued) != 1 || queued[0].Note != seed.memory.ID {
		t.Fatalf("the reading queue = %+v, want the note added from the list", queued)
	}
}

// The archive's Restore puts a note back in the reading queue, and it is listed
// again the next time the queue page is drawn.
func TestArchiveRestoresANoteToTheQueue(t *testing.T) {
	msgs := locales.In("en")
	a, seed := readingApp(t)
	if err := a.lib.ArchiveReading(seed.memory.ID, time.Now()); err != nil {
		t.Fatalf("ArchiveReading: %v", err)
	}
	a.reload()
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	readPage(t, tt, msgs.ArchivePage())

	for _, want := range []string{msgs.ReadArchiveTitle(), msgs.ReadArchivedSection(), seed.memory.Title} {
		if !tt.HasText(want) {
			t.Fatalf("the archive page lost %q; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click(msgs.ReadRestore() + " " + seed.memory.Title); err != nil {
		t.Fatalf("the archived row has no Restore: %v", err)
	}
	tt.Advance(320 * time.Millisecond)

	row, ok := a.lib.ReadingFor(seed.memory.ID)
	if !ok || !row.Queued || row.Archived || row.Dismissed {
		t.Fatalf("after Restore the note is %+v, want it queued and in neither terminal state", row)
	}
	archived, err := a.lib.ReadingArchive()
	if err != nil {
		t.Fatalf("ReadingArchive: %v", err)
	}
	for _, entry := range archived {
		if entry.Note == seed.memory.ID {
			t.Error("the restored note is still in the archive")
		}
	}
	readPage(t, tt, msgs.QueuePage())
	if !tt.HasText(seed.memory.Title) {
		t.Errorf("the queue page does not list the restored note; texts %q", tt.Texts())
	}
}

// A dismissed note is out of the queue until it is restored, and the archive is
// where it waits.
func TestDismissedNotesStayOutUntilRestored(t *testing.T) {
	msgs := locales.In("en")
	a, seed := readingApp(t)
	if err := a.lib.DismissReading(seed.memory.ID, time.Now()); err != nil {
		t.Fatalf("DismissReading: %v", err)
	}
	a.reload()
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(320 * time.Millisecond)

	// Out of today's queue: the other note is still there, this one is not.
	if tt.HasText(seed.memory.Title) {
		t.Errorf("a dismissed note is still on the queue page; texts %q", tt.Texts())
	}
	if !tt.HasText(seed.writing.Title) {
		t.Errorf("dismissing one note took the other with it; texts %q", tt.Texts())
	}

	// In the archive's own section, with the way back.
	readPage(t, tt, msgs.ArchivePage())
	if !tt.HasText(msgs.ReadDismissedSection()) || !tt.HasText(seed.memory.Title) {
		t.Fatalf("the dismissed note is not in the archive; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.ReadRestore() + " " + seed.memory.Title); err != nil {
		t.Fatalf("the dismissed row has no Restore: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	row, ok := a.lib.ReadingFor(seed.memory.ID)
	if !ok || !row.Queued || row.Dismissed {
		t.Fatalf("after Restore the note is %+v, want it queued and not dismissed", row)
	}
	readPage(t, tt, msgs.QueuePage())
	if !tt.HasText(seed.memory.Title) {
		t.Errorf("the restored note is not back on the queue page; texts %q", tt.Texts())
	}
}

// Archiving from the session writes the terminal event, records no answer, moves
// the session on, and tells the reader with the undo.
func TestReadingSessionArchivesANote(t *testing.T) {
	msgs := locales.In("en")
	a, seed := readingApp(t)
	tt := openReading(t, a, 1024, 720)

	tt.Key(0, ui.KeyA)
	tt.Frame()

	row, ok := a.lib.ReadingFor(seed.memory.ID)
	if !ok || !row.Archived || row.Queued {
		t.Fatalf("after A the note is %+v, want archived and out of the queue", row)
	}
	if current, _ := a.reading.Current(); current != seed.writing.ID {
		t.Errorf("the session is on %q, want the next note", current)
	}
	events, err := a.lib.ReadingEvents(seed.memory.ID)
	if err != nil {
		t.Fatalf("ReadingEvents: %v", err)
	}
	for _, ev := range events {
		if ev.Kind == schedule.Reviewed {
			t.Error("archiving a note recorded an answer")
		}
	}

	// The toast says what happened, and its Undo puts the note back.
	if !tt.HasText(msgs.ReadArchived(seed.memory.Title)) {
		t.Fatalf("archiving said nothing; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.ReadUndo()); err != nil {
		t.Fatalf("the toast has no undo: %v", err)
	}
	tt.Frame()
	row, ok = a.lib.ReadingFor(seed.memory.ID)
	if !ok || !row.Queued || row.Archived {
		t.Errorf("after the undo the note is %+v, want it queued again", row)
	}
}

// Postponing from the session keeps the note in the queue and brings it back the
// next day, not sooner and not later.
func TestReadingSessionPostponesToTomorrow(t *testing.T) {
	a, seed := readingApp(t)
	tt := openReading(t, a, 1024, 720)
	before := time.Now()

	tt.Key(0, ui.KeyP)
	tt.Frame()

	row, ok := a.lib.ReadingFor(seed.memory.ID)
	if !ok || !row.Queued {
		t.Fatalf("a postponed note is %+v, want it still in the queue", row)
	}
	want := startOfTomorrow(before)
	if !row.Until.Equal(want) {
		t.Errorf("postponed until %s, want %s", row.Until.UTC().Format(time.RFC3339), want.UTC().Format(time.RFC3339))
	}
	if !row.Due().After(time.Now()) {
		t.Errorf("the postponed note is due %s, which is not tomorrow", row.Due())
	}
	if current, ok := a.reading.Current(); ok && current == seed.memory.ID {
		t.Error("the postponed note is still on screen")
	}
	// Out of today's queue, which is what the postponement means.
	for _, entry := range a.readPages.queue {
		if entry.Note == seed.memory.ID {
			t.Error("the postponed note is still in today's queue page")
		}
	}
}

// Dismissing from the session says so: the note leaves the queue, the session
// moves on, and the toast names the decision.
func TestReadingSessionDismissesAndTells(t *testing.T) {
	msgs := locales.In("en")
	a, seed := readingApp(t)
	tt := openReading(t, a, 1024, 720)

	tt.Key(0, ui.KeyD)
	tt.Frame()

	row, ok := a.lib.ReadingFor(seed.memory.ID)
	if !ok || !row.Dismissed || row.Queued {
		t.Fatalf("after D the note is %+v, want dismissed and out of the queue", row)
	}
	if current, _ := a.reading.Current(); current != seed.writing.ID {
		t.Errorf("the session is on %q, want the next note", current)
	}
	if !tt.HasText(msgs.ReadDismissed(seed.memory.Title)) {
		t.Errorf("dismissing said nothing; texts %q", tt.Texts())
	}
}

// The read area is one language at a time: the queue and the archive reach every
// word of the Chinese catalog at the smallest window.
func TestReadAreaInChineseAtTheMinimumSize(t *testing.T) {
	msgs := locales.In("zh-Hans")
	a, seed := readingApp(t)
	if err := a.lib.ArchiveReading(seed.writing.ID, time.Now()); err != nil {
		t.Fatalf("ArchiveReading: %v", err)
	}
	a.reload()

	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	settle(tt)
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(320 * time.Millisecond)
	for _, want := range []string{msgs.ReadQueueTitle(), msgs.ReadStart(), seed.memory.Title, msgs.ReadContinue()} {
		if !tt.HasText(want) {
			t.Errorf("the queue page lost %q; texts %q", want, tt.Texts())
		}
	}

	readPage(t, tt, msgs.ArchivePage())
	for _, want := range []string{msgs.ReadArchiveTitle(), msgs.ReadArchivedSection(), seed.writing.Title, msgs.ReadRestore()} {
		if !tt.HasText(want) {
			t.Errorf("the archive page lost %q; texts %q", want, tt.Texts())
		}
	}
}
