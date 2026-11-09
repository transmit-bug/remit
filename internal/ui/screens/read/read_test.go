package read_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/read"
	"github.com/transmit-bug/remit/locales"
)

// The two sizes the window holds: the one it opens at and the smallest it goes.
var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// now is the clock every page is drawn against, so a "today" in a row reads the
// same on a busy machine as on an idle one.
var now = time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

// fake is the app the pages talk to: the rows it derived, and the calls it was
// asked for, with no store behind it.
type fake struct {
	archived, dismissed []read.Item
	queue               []read.Item
	started             int
	added               int
	reloaded            int
	continued           []core.ID
	opened              []core.ID
	restored            []core.ID
}

func (f *fake) deps() read.Deps {
	return read.Deps{
		Queue:   func() []read.Item { return f.queue },
		Archive: func() ([]read.Item, []read.Item) { return f.archived, f.dismissed },
		Counts: func() read.Counts {
			return read.Counts{Due: len(f.queue), Queued: len(f.queue) + 2}
		},
		Now:      func() time.Time { return now },
		Start:    func() { f.started++ },
		Continue: func(id core.ID) { f.continued = append(f.continued, id) },
		Add:      func() { f.added++ },
		Open:     func(id core.ID) { f.opened = append(f.opened, id) },
		Restore:  func(id core.ID) { f.restored = append(f.restored, id) },
		Reload:   func() { f.reloaded++ },
	}
}

// item is a note as the app hands it over: nine paragraphs, read to the fifth
// today.
func item(id core.ID, title string) read.Item {
	return read.Item{Note: id, Title: title, Blocks: 9, At: 5,
		LastSeen: now.Add(-2 * time.Hour), Due: now.Add(-3 * time.Hour)}
}

// draw shows one page and lets it settle.
func draw(t *testing.T, page read.Page, f *fake, w, h int) *ui.Tester {
	t.Helper()
	state := &read.State{Loaded: true}
	state.Show(page)
	tt := ui.NewTester(func(c *ui.Context) { read.View(c, state, f.deps()) }, w, h)
	tt.Frame()
	tt.Advance(320 * time.Millisecond) // the rows' arrival: 180 ms plus five 24 ms steps
	return tt
}

// The queue page is the notes due today, each with where it resumes, how far a
// session got, and the way back in; the rhythm of the whole queue is under them.
// Both sizes hold it.
func TestQueuePageListsWhatsDue(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f := &fake{queue: []read.Item{
				item("note_a", "Memory and recall"),
				item("note_b", "Writing cards"),
			}}
			tt := draw(t, read.Queue, f, size.w, size.h)

			for _, want := range []string{
				msgs.ReadQueueTitle(), "Memory and recall", "Writing cards",
				msgs.ReadRowPlace(5, 9), msgs.ReadRowStopped(msgs.ReadDayToday()),
				msgs.ReadContinue(), msgs.ReadStart(), msgs.ReadAdd(),
				msgs.ReadRhythm(), msgs.ReadRhythmDue(), msgs.ReadRhythmQueued(),
				msgs.ReadingTooHard(), msgs.ReadingJustRight(), msgs.ReadingTooShallow(),
			} {
				if !tt.HasText(want) {
					t.Errorf("missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// The page's controls reach the app: Start opens the session, a row's Continue
// opens it at that note, and Add goes where a note is queued.
func TestQueuePageActions(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{queue: []read.Item{item("note_a", "Memory and recall"), item("note_b", "Writing cards")}}
	tt := draw(t, read.Queue, f, 1024, 720)

	if err := tt.Click(msgs.ReadContinue() + " Writing cards"); err != nil {
		t.Fatal(err)
	}
	if len(f.continued) != 1 || f.continued[0] != "note_b" {
		t.Errorf("continued %v, want the row clicked", f.continued)
	}
	if err := tt.Click(msgs.ReadStart()); err != nil {
		t.Fatal(err)
	}
	if f.started != 1 {
		t.Errorf("start ran %d times, want 1", f.started)
	}
	if err := tt.Click(msgs.ReadAdd()); err != nil {
		t.Fatal(err)
	}
	if f.added != 1 {
		t.Errorf("add ran %d times, want 1", f.added)
	}
}

// An empty queue says so, offers the one action that fills it, and Start is
// disabled: a session with nothing to read is not a place to be.
func TestEmptyQueueIsNotASession(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{}
	tt := draw(t, read.Queue, f, 1024, 720)

	if !tt.HasText(msgs.ReadQueueEmpty()) {
		t.Fatalf("the empty queue says nothing; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.ReadStart()); err != nil {
		t.Fatalf("start has no box: %v", err)
	}
	if f.started != 0 {
		t.Error("a disabled Start opened a session")
	}
	// The empty state carries the action too, because it is the way out of it.
	if err := tt.Click(msgs.ReadAddNote()); err != nil {
		t.Fatalf("the empty state offers nothing to add: %v", err)
	}
	if f.added != 1 {
		t.Errorf("add ran %d times, want 1", f.added)
	}
}

// The archive page is two sections — put away, and not worth reading — each with
// how many notes it holds, and every row opens the note or puts it back.
func TestArchivePageListsBothSections(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{
		archived:  []read.Item{item("note_a", "Memory and recall")},
		dismissed: []read.Item{item("note_b", "Writing cards")},
	}
	tt := draw(t, read.Archive, f, 1024, 720)

	for _, want := range []string{
		msgs.ReadArchiveTitle(), msgs.ReadArchivedSection(), msgs.ReadDismissedSection(),
		msgs.ReadNotesMark(1), "Memory and recall", "Writing cards",
		msgs.ReadOpen(), msgs.ReadRestore(), msgs.ReadRowPlace(5, 9),
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q; texts %q", want, tt.Texts())
		}
	}

	if err := tt.Click(msgs.ReadOpen() + " Memory and recall"); err != nil {
		t.Fatal(err)
	}
	if len(f.opened) != 1 || f.opened[0] != "note_a" {
		t.Errorf("opened %v, want the archived note", f.opened)
	}
	if err := tt.Click(msgs.ReadRestore() + " Writing cards"); err != nil {
		t.Fatal(err)
	}
	if len(f.restored) != 1 || f.restored[0] != "note_b" {
		t.Errorf("restored %v, want the dismissed note", f.restored)
	}
}

// An archive with nothing in it says so, and how a note gets there.
func TestArchiveEmptySaysHowANoteArrives(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := draw(t, read.Archive, &fake{}, size.w, size.h)
			for _, want := range []string{msgs.ReadArchiveEmpty(), msgs.ReadArchiveHint()} {
				if !tt.HasText(want) {
					t.Errorf("the empty archive lost %q; texts %q", want, tt.Texts())
				}
			}
		})
	}
}

// Before the first answer the page says it is loading; a load that failed says
// why and offers the one action that asks again. Neither is an empty page.
func TestLoadStates(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{}

	loading := &read.State{}
	tt := ui.NewTester(func(c *ui.Context) { read.View(c, loading, f.deps()) }, 1024, 720)
	tt.Frame()
	if !tt.HasText(msgs.ReadLoading()) {
		t.Errorf("a page before its load says nothing; texts %q", tt.Texts())
	}
	if tt.HasText(msgs.ReadQueueEmpty()) {
		t.Error("a page before its load claims the queue is empty")
	}

	failed := &read.State{Loaded: true, Err: "the index is gone"}
	tt = ui.NewTester(func(c *ui.Context) { read.View(c, failed, f.deps()) }, 1024, 720)
	tt.Frame()
	if !tt.HasText(msgs.ReadLoadFailed("the index is gone")) {
		t.Errorf("a failed load says nothing; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.ReadRetry()); err != nil {
		t.Fatal(err)
	}
	if f.reloaded != 1 {
		t.Errorf("the retry ran %d times, want 1", f.reloaded)
	}
}

// A row's day is today, yesterday, or its date — never a number to count back
// from — and a note no session read says so instead of pretending.
func TestRowsNameTheirDay(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{queue: []read.Item{
		item("note_a", "Read today"),
		{Note: "note_b", Title: "Read yesterday", Blocks: 4, At: 2,
			LastSeen: now.Add(-26 * time.Hour), Due: now.Add(-26 * time.Hour)},
		{Note: "note_c", Title: "Never read", Blocks: 3, Due: now.Add(-2 * time.Hour)},
	}}
	tt := draw(t, read.Queue, f, 1024, 720)

	for _, want := range []string{
		msgs.ReadRowStopped(msgs.ReadDayToday()),
		msgs.ReadRowStopped(msgs.ReadDayYesterday()),
		msgs.ReadRowUnread(), msgs.ReadRowQueued(msgs.ReadDayToday()),
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q; texts %q", want, tt.Texts())
		}
	}
}

// New Year's Day has a yesterday like any other day: the row says so instead of
// falling back to a date no reader counts back from.
func TestYesterdayAcrossTheNewYear(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{queue: []read.Item{{Note: "note_a", Title: "Read last year", Blocks: 9, At: 4,
		LastSeen: time.Date(2026, 12, 31, 20, 0, 0, 0, time.UTC)}}}
	deps := f.deps()
	deps.Now = func() time.Time { return time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC) }
	state := &read.State{Loaded: true}
	tt := ui.NewTester(func(c *ui.Context) { read.View(c, state, deps) }, 1024, 720)
	tt.Frame()

	if !tt.HasText(msgs.ReadRowStopped(msgs.ReadDayYesterday())) {
		t.Errorf("the row does not name yesterday across the new year; texts %q", tt.Texts())
	}
}

// The pages are one language at a time: the Chinese catalog reaches every word of
// both, at the smallest window.
func TestChineseAtTheMinimumSize(t *testing.T) {
	msgs := locales.In("zh-Hans")

	f := &fake{queue: []read.Item{item("note_a", "记忆与回忆")}}
	tt := ui.NewTester(func(c *ui.Context) {
		state := &read.State{Loaded: true}
		read.View(c, state, f.deps())
	}, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	for _, want := range []string{
		msgs.ReadQueueTitle(), msgs.ReadStart(), msgs.ReadAdd(), msgs.ReadContinue(),
		msgs.ReadRowPlace(5, 9), msgs.ReadRhythmDue(), "记忆与回忆",
	} {
		if !tt.HasText(want) {
			t.Errorf("the queue page lost %q; texts %q", want, tt.Texts())
		}
	}

	archived := &fake{archived: []read.Item{item("note_a", "记忆与回忆")}}
	tt = ui.NewTester(func(c *ui.Context) {
		state := &read.State{Loaded: true}
		state.Show(read.Archive)
		read.View(c, state, archived.deps())
	}, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	for _, want := range []string{
		msgs.ReadArchiveTitle(), msgs.ReadArchivedSection(), msgs.ReadOpen(),
		msgs.ReadRestore(), "记忆与回忆",
	} {
		if !tt.HasText(want) {
			t.Errorf("the archive page lost %q; texts %q", want, tt.Texts())
		}
	}
}
