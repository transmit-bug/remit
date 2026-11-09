package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// lastLogLine is the last event written to a library's log: the line a reader,
// a checker and the next open all see.
func lastLogLine(t *testing.T, dir string) string {
	t.Helper()
	log := strings.TrimSpace(string(read(t, filepath.Join(dir, "reviews.jsonl"))))
	lines := strings.Split(log, "\n")
	return lines[len(lines)-1]
}

// appendLogLine writes one line by hand, the way a person editing the log would.
func appendLogLine(t *testing.T, dir, line string) {
	t.Helper()
	file, err := os.OpenFile(filepath.Join(dir, "reviews.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open the log: %v", err)
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		file.Close()
		t.Fatalf("append to the log: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close the log: %v", err)
	}
}

// sameReading compares two places in the reading queue: the schedule, where the
// reader stopped, and the three flags plus the postponed time.
func sameReading(a, b store.Reading) bool {
	return a.Note == b.Note && sameState(a.State, b.State) &&
		a.Position == b.Position && a.Queued == b.Queued &&
		a.Archived == b.Archived && a.Dismissed == b.Dismissed && a.Until.Equal(b.Until)
}

// Every kind of reading event is one JSON line, and the line decodes back to the
// event that wrote it: the log is the record, for a person and for the next open.
func TestReadingLogRoundTripsEveryKind(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "first paragraph", "second paragraph")
	position := note.Blocks[1].ID
	until := clk.Now().Add(72 * time.Hour)

	steps := []struct {
		name  string
		write func() error
		kind  schedule.Kind
	}{
		{"queued", func() error { return lib.AddToReading(note.ID, clk.Now()) }, schedule.Queued},
		{"reviewed", func() error { return lib.ReadNote(note.ID, schedule.Good, position, clk.Now()) }, schedule.Reviewed},
		{"postponed", func() error { return lib.PostponeReading(note.ID, until, clk.Now()) }, schedule.Postponed},
		{"archived", func() error { return lib.ArchiveReading(note.ID, clk.Now()) }, schedule.Archived},
		{"dismissed", func() error { return lib.DismissReading(note.ID, clk.Now()) }, schedule.Dismissed},
		{"queued again", func() error { return lib.AddToReading(note.ID, clk.Now()) }, schedule.Queued},
	}
	for i, step := range steps {
		if err := step.write(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		line := lastLogLine(t, dir)
		event, ok, err := store.DecodeLogLine([]byte(line))
		if err != nil || !ok {
			t.Fatalf("decode the %s line: %v, ok = %v", step.name, err, ok)
		}
		if event.Seq != int64(i+1) || event.Kind != step.kind || event.Note != note.ID {
			t.Errorf("the %s line reads as %+v", step.name, event)
		}
		if step.kind == schedule.Postponed {
			if !event.Until.Equal(until) {
				t.Errorf("the postponed line reads until %v, want %v", event.Until, until)
			}
			if want := `"until":"` + until.UTC().Format(time.RFC3339Nano) + `"`; !strings.Contains(line, want) {
				t.Errorf("the postponed line does not carry %s:\n%s", want, line)
			}
			continue
		}
		if strings.Contains(line, `"until"`) {
			t.Errorf("the %s line carries an until:\n%s", step.name, line)
		}
	}
	// A non-rating event names the note alone: no card and no rating belong on it.
	archived := `"note":"` + string(note.ID) + `","kind":"archived"`
	for _, line := range strings.Split(string(read(t, filepath.Join(dir, "reviews.jsonl"))), "\n") {
		if !strings.Contains(line, archived) {
			continue
		}
		if strings.Contains(line, `"card"`) || strings.Contains(line, `"rating"`) {
			t.Errorf("an archived line carries a card or a rating:\n%s", line)
		}
	}
}

// A line the store would not write is a bad line: the kinds that change a note's
// place in the queue carry no card and no rating, and only a postpone carries an
// until, after the moment it was asked.
func TestLogRefusesBadReadingLines(t *testing.T) {
	const at = `"at":"2026-11-06T09:00:00Z"`
	note := `"note":"` + string(core.NewID("note")) + `"`
	card := `"card":"` + string(core.NewID("card")) + `"`

	tests := []struct {
		name string
		line string
	}{
		{"postponed without an until", `{"seq":1,` + note + `,"kind":"postponed",` + at + `}`},
		{
			name: "postponed before it was asked",
			line: `{"seq":1,` + note + `,"kind":"postponed","until":"2026-11-06T08:59:59Z",` + at + `}`,
		},
		{"queued with an until", `{"seq":1,` + note + `,"kind":"queued","until":"2026-11-07T09:00:00Z",` + at + `}`},
		{"archived with a card", `{"seq":1,` + card + `,"kind":"archived",` + at + `}`},
		{"archived without a note", `{"seq":1,"kind":"archived",` + at + `}`},
		{"archived with a rating", `{"seq":1,` + note + `,"kind":"archived","rating":"good",` + at + `}`},
		{"a review with an until", `{"seq":1,` + card + `,"rating":"good","until":"2026-11-07T09:00:00Z",` + at + `}`},
		{"an unknown kind", `{"seq":1,` + note + `,"kind":"deleted",` + at + `}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := store.DecodeLogLine([]byte(tt.line)); err == nil {
				t.Fatalf("DecodeLogLine accepted %s", tt.line)
			}
		})
	}

	// A postpone to the moment it was asked is not one into the past.
	ok := `{"seq":1,` + note + `,"kind":"postponed","until":"2026-11-06T09:00:00Z",` + at + `}`
	if _, event, err := store.DecodeLogLine([]byte(ok)); err != nil || !event {
		t.Errorf("DecodeLogLine refused a postpone to the moment it was asked: %v", err)
	}
}

// A bad line is not skipped: the log is the only record of what happened, so a
// library holding one does not open.
func TestBadUntilRefusesTheLibrary(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "para")
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	appendLogLine(t, dir, `{"seq":2,"note":"`+string(note.ID)+
		`","kind":"postponed","until":"2026-11-01T00:00:00Z","at":"2026-11-06T09:00:00Z"}`)

	if reopened, err := store.Open(dir, store.Options{Now: clk.Now}); err == nil {
		reopened.Close()
		t.Fatal("a library postponing a note into the past opened")
	}
}

// An event appended while the library was closed is applied on the next open,
// the way a crash between the log write and the index write recovers: the tail
// replay reads the new kinds as it reads the old ones.
func TestReadingTailReplayAppliesEveryKind(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "first", "second")
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	until := clk.Now().Add(72 * time.Hour)

	// The log grows while the library is closed, so the index is behind it.
	tail := []struct {
		line string
		want func(store.Reading) bool
	}{
		{
			`{"seq":2,"note":"` + string(note.ID) + `","kind":"postponed","until":"` +
				until.UTC().Format(time.RFC3339Nano) + `","at":"` + clk.Now().UTC().Format(time.RFC3339Nano) + `"}`,
			func(r store.Reading) bool { return r.Queued && r.State.Due.Equal(until) && r.Until.Equal(until) },
		},
		{
			`{"seq":3,"note":"` + string(note.ID) + `","kind":"archived","at":"` +
				clk.Now().UTC().Format(time.RFC3339Nano) + `"}`,
			func(r store.Reading) bool { return !r.Queued && r.Archived && r.State.Due.Equal(until) },
		},
	}
	for _, step := range tail {
		if err := lib.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		appendLogLine(t, dir, step.line)
		lib, err := store.Open(dir, store.Options{Now: clk.Now})
		if err != nil {
			t.Fatalf("Open with a tail: %v", err)
		}
		t.Cleanup(func() { lib.Close() })
		if got := readingOf(t, lib, note.ID); !step.want(got) {
			t.Errorf("the tail replay reads %+v", got)
		}
	}
}

// The four ways a note can leave the reading queue: an archive and a dismissal
// take it out of today's queue and stay in the library, and only being queued
// again brings it back.
func TestReadingTerminalStates(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	archived := saveNote(t, lib, "Archived read", "para")
	dismissed := saveNote(t, lib, "Dismissed read", "para")
	kept := saveNote(t, lib, "Kept read", "para")
	for _, note := range []core.Note{archived, dismissed, kept} {
		if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
			t.Fatalf("AddToReading: %v", err)
		}
	}

	tests := []struct {
		name  string
		note  core.Note
		write func() error
		flag  func(store.Reading) bool
	}{
		{"archived", archived, func() error { return lib.ArchiveReading(archived.ID, clk.Now()) },
			func(r store.Reading) bool { return r.Archived && !r.Dismissed }},
		{"dismissed", dismissed, func() error { return lib.DismissReading(dismissed.ID, clk.Now()) },
			func(r store.Reading) bool { return r.Dismissed && !r.Archived }},
	}
	for _, tt := range tests {
		if err := tt.write(); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		entry := readingOf(t, lib, tt.note.ID)
		if !tt.flag(entry) {
			t.Errorf("the %s note reads as %+v", tt.name, entry)
		}
		if entry.Queued {
			t.Errorf("the %s note is still in the queue", tt.name)
		}
	}

	queue, err := lib.ReadingQueue()
	if err != nil {
		t.Fatalf("ReadingQueue: %v", err)
	}
	if len(queue) != 1 || queue[0].Note != kept.ID {
		t.Fatalf("reading queue = %+v, want only the note that was kept", queue)
	}
	archive, err := lib.ReadingArchive()
	if err != nil {
		t.Fatalf("ReadingArchive: %v", err)
	}
	if len(archive) != 2 {
		t.Fatalf("archive = %+v, want the two terminal notes", archive)
	}
	for _, entry := range archive {
		if entry.Note != archived.ID && entry.Note != dismissed.ID {
			t.Errorf("the archive holds %q, which was never ended", entry.Note)
		}
		if entry.Queued {
			t.Errorf("the archive holds %q as queued", entry.Note)
		}
	}

	// The index is a cache: a rebuild and a reopen answer the same.
	rebuilt := reopen(t, lib, dir, clk)
	if err := rebuilt.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	after, err := rebuilt.ReadingArchive()
	if err != nil {
		t.Fatalf("ReadingArchive after a rebuild: %v", err)
	}
	if len(after) != len(archive) {
		t.Fatalf("the rebuild left %d archived notes, want %d", len(after), len(archive))
	}
	for i := range archive {
		if !sameReading(after[i], archive[i]) {
			t.Errorf("the rebuild changed %+v to %+v", archive[i], after[i])
		}
	}

	// Queued again is how a terminal state ends: the note comes back to today.
	if err := rebuilt.AddToReading(archived.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading after an archive: %v", err)
	}
	back := readingOf(t, rebuilt, archived.ID)
	if !back.Queued || back.Archived {
		t.Errorf("queued again reads as %+v, want it back in the queue", back)
	}
	if archive, err := rebuilt.ReadingArchive(); err != nil || len(archive) != 1 {
		t.Errorf("archive after queueing one back = %+v, %v", archive, err)
	}

	// The two terminal states replace each other rather than adding up: a note
	// is either archived or dismissed, never both.
	if err := rebuilt.DismissReading(archived.ID, clk.Now()); err != nil {
		t.Fatalf("DismissReading: %v", err)
	}
	if entry := readingOf(t, rebuilt, archived.ID); !entry.Dismissed || entry.Archived {
		t.Errorf("dismissing a queued note reads as %+v", entry)
	}
	if err := rebuilt.ArchiveReading(archived.ID, clk.Now()); err != nil {
		t.Fatalf("ArchiveReading after a dismissal: %v", err)
	}
	if entry := readingOf(t, rebuilt, archived.ID); !entry.Archived || entry.Dismissed {
		t.Errorf("archiving a dismissed note reads as %+v", entry)
	}

	// A note the log never put in the queue has nothing to end.
	stranger := saveNote(t, rebuilt, "Never queued", "para")
	if err := rebuilt.ArchiveReading(stranger.ID, clk.Now()); !errors.Is(err, store.ErrNoReading) {
		t.Errorf("archiving a note outside the queue = %v, want ErrNoReading", err)
	}
}

// A postpone moves the due date and nothing else: the note stays in the queue,
// keeps the session it was read in, and the index and a rebuild agree.
func TestPostponedReadingKeepsItsPlaceAndHistory(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "first", "second")
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	clk.advance(time.Hour)
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[1].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	before := readingOf(t, lib, note.ID)

	until := clk.Now().Add(72 * time.Hour)
	if err := lib.PostponeReading(note.ID, until, clk.Now()); err != nil {
		t.Fatalf("PostponeReading: %v", err)
	}
	after := readingOf(t, lib, note.ID)
	if !after.State.Due.Equal(until) {
		t.Errorf("due = %v, want the postponed %v", after.State.Due, until)
	}
	if !after.Until.Equal(until) {
		t.Errorf("until = %v, want %v", after.Until, until)
	}
	if !after.Queued || after.Archived || after.Dismissed {
		t.Errorf("the postpone changed the membership: %+v", after)
	}
	if after.State.Reviews != before.State.Reviews || !after.State.LastReview.Equal(before.State.LastReview) ||
		after.State.LastRating != before.State.LastRating || after.Position != before.Position {
		t.Errorf("the postpone changed the reading history: %+v, want %+v", after, before)
	}
	// Due is what the postpone asked for; the cadence it was going to take is
	// still in the log, and a session takes the due date back.
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote after a postpone: %v", err)
	}
	readAgain := readingOf(t, lib, note.ID)
	if !readAgain.Until.IsZero() || !readAgain.State.Due.After(clk.Now()) {
		t.Errorf("a session after a postpone reads as %+v", readAgain)
	}

	reopened := reopen(t, lib, dir, clk)
	if got := readingOf(t, reopened, note.ID); !sameReading(got, readAgain) {
		t.Errorf("reopening changed the postponed note: %+v, want %+v", got, readAgain)
	}
	if err := reopened.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got := readingOf(t, reopened, note.ID); !sameReading(got, readAgain) {
		t.Errorf("the rebuild changed the postponed note: %+v, want %+v", got, readAgain)
	}
}

// Undo takes back the answer, not the reading: voiding a session puts the due
// date and the position back where they were, and leaves the note queued.
func TestUndoReadingRestoresDueAndPosition(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "first", "second", "third")
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	clk.advance(time.Hour)
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	first := readingOf(t, lib, note.ID)

	clk.advance(72 * time.Hour)
	if err := lib.ReadNote(note.ID, schedule.Easy, note.Blocks[2].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if second := readingOf(t, lib, note.ID); sameState(second.State, first.State) {
		t.Fatal("the second session left the same schedule, so nothing was undone")
	}

	if err := lib.UndoReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("UndoReading: %v", err)
	}
	back := readingOf(t, lib, note.ID)
	if !sameState(back.State, first.State) {
		t.Errorf("after the undo the schedule is %+v, want %+v", back.State, first.State)
	}
	if back.Position != first.Position {
		t.Errorf("after the undo the position is %q, want %q", back.Position, first.Position)
	}
	if !back.Queued {
		t.Error("the undo took the note out of the reading queue")
	}
	// The undo is a line in the log, not a rewrite of it.
	if line := lastLogLine(t, dir); !strings.Contains(line, `"void":3`) {
		t.Errorf("the undo is not the log's last line:\n%s", line)
	}
	events, err := lib.ReadingEvents(note.ID)
	if err != nil {
		t.Fatalf("ReadingEvents: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("the note's live events = %+v, want the queue and one session", events)
	}

	// A rebuild derives the same answer, and reopening reads it back.
	rebuilt := reopen(t, lib, dir, clk)
	if err := rebuilt.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got := readingOf(t, rebuilt, note.ID); !sameReading(got, back) {
		t.Errorf("the rebuild reads %+v, want %+v", got, back)
	}

	// The queue and the per-note view agree, and the queue is where the note is.
	queue, err := rebuilt.ReadingQueue()
	if err != nil || len(queue) != 1 || !sameReading(queue[0], back) {
		t.Fatalf("reading queue = %+v, %v, want the undone note", queue, err)
	}
}

// Undo takes back one session, and a note with none reports it. A terminal state
// is not an answer: it is restored by queueing the note again.
func TestUndoReadingOfNothing(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "para")
	if err := lib.AddToReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	if err := lib.UndoReading(note.ID, clk.Now()); !errors.Is(err, store.ErrNoReview) {
		t.Errorf("undoing a note that was never read = %v, want ErrNoReview", err)
	}
	if err := lib.ArchiveReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("ArchiveReading: %v", err)
	}
	if err := lib.UndoReading(note.ID, clk.Now()); !errors.Is(err, store.ErrNoReview) {
		t.Errorf("undoing an archive = %v, want ErrNoReview", err)
	}
	if entry := readingOf(t, lib, note.ID); !entry.Archived {
		t.Errorf("the undo took the archive back: %+v", entry)
	}
}

// Undoing a session does not take back the decision that followed it: an archive
// stands, because a terminal state is restored by queueing, not by a void.
func TestUndoKeepsATerminalState(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "first", "second")
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[1].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if err := lib.ArchiveReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("ArchiveReading: %v", err)
	}
	if err := lib.UndoReading(note.ID, clk.Now()); err != nil {
		t.Fatalf("UndoReading: %v", err)
	}
	entry := readingOf(t, lib, note.ID)
	if !entry.Archived || entry.Queued {
		t.Errorf("after undoing the session the note reads as %+v, want it still archived", entry)
	}
	if entry.State.Reviews != 0 || entry.Position != "" {
		t.Errorf("the undo left reviews %d and position %q", entry.State.Reviews, entry.Position)
	}
}

// Again counts a lapse, the index caches the count, and a rebuild recounts it
// from the same log.
func TestLapsesCount(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Hard card", "question :: answer")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	steps := []struct {
		rating schedule.Rating
		lapses int
	}{
		{schedule.Good, 0},
		{schedule.Again, 1},
		{schedule.Hard, 1},
		{schedule.Again, 2},
	}
	for i, step := range steps {
		if err := lib.Review(card.ID, step.rating, clk.Now()); err != nil {
			t.Fatalf("Review(%v): %v", step.rating, err)
		}
		state, ok := lib.CardState(card.ID)
		if !ok {
			t.Fatalf("step %d: the card has no state", i)
		}
		if state.Lapses != step.lapses {
			t.Fatalf("after %v the lapses = %d, want %d", step.rating, state.Lapses, step.lapses)
		}
		clk.advance(48 * time.Hour)
	}

	// The index caches the count: a reopen with a current index reads the column,
	// and a rebuild recounts it from the log.
	reopened := reopen(t, lib, dir, clk)
	if state, ok := reopened.CardState(card.ID); !ok || state.Lapses != 2 {
		t.Errorf("the cached index reads lapses = %d (found %v), want 2", state.Lapses, ok)
	}
	if err := reopened.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if state, _ := reopened.CardState(card.ID); state.Lapses != 2 {
		t.Errorf("the rebuild counted %d lapses, want 2", state.Lapses)
	}
}

// The three views of a note's reading agree before and after a rebuild: the
// index caches what replay derives, and throws nothing away doing it.
func TestReadingAgreesAfterRebuild(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Long read", "first", "second", "third")
	other := saveNote(t, lib, "Other read", "para")

	for _, queued := range []core.ID{note.ID, other.ID} {
		if err := lib.AddToReading(queued, clk.Now()); err != nil {
			t.Fatalf("AddToReading: %v", err)
		}
	}

	writes := []struct {
		name string
		do   func() error
	}{
		{"read", func() error { return lib.ReadNote(note.ID, schedule.Good, note.Blocks[1].ID, clk.Now()) }},
		{"postponed", func() error { return lib.PostponeReading(note.ID, clk.Now().Add(96*time.Hour), clk.Now()) }},
		{"dropped", func() error { return lib.RemoveFromReading(other.ID, clk.Now()) }},
		{"other dismissed", func() error { return lib.DismissReading(other.ID, clk.Now()) }},
		{"archived", func() error { return lib.ArchiveReading(note.ID, clk.Now()) }},
		{"queued again", func() error { return lib.AddToReading(note.ID, clk.Now()) }},
	}
	for _, write := range writes {
		if err := write.do(); err != nil {
			t.Fatalf("%s: %v", write.name, err)
		}
		clk.advance(time.Hour)

		before, beforeQueue, beforeArchive := readingViews(t, lib, note.ID, other.ID)
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("%s: Rebuild: %v", write.name, err)
		}
		after, afterQueue, afterArchive := readingViews(t, lib, note.ID, other.ID)
		for i := range before {
			if !sameReading(before[i], after[i]) {
				t.Fatalf("%s: the rebuild changed the note from %+v to %+v", write.name, before[i], after[i])
			}
		}
		if !sameQueue(beforeQueue, afterQueue) || !sameQueue(beforeArchive, afterArchive) {
			t.Fatalf("%s: the rebuild changed the queue from %+v to %+v, the archive from %+v to %+v",
				write.name, beforeQueue, afterQueue, beforeArchive, afterArchive)
		}
	}

	// A reopen of the same directory answers the same as the library that wrote it.
	want, wantQueue, wantArchive := readingViews(t, lib, note.ID, other.ID)
	reopened := reopen(t, lib, dir, clk)
	got, gotQueue, gotArchive := readingViews(t, reopened, note.ID, other.ID)
	for i := range want {
		if !sameReading(want[i], got[i]) {
			t.Errorf("reopening changed the note from %+v to %+v", want[i], got[i])
		}
	}
	if !sameQueue(wantQueue, gotQueue) || !sameQueue(wantArchive, gotArchive) {
		t.Errorf("reopening changed the queue or the archive")
	}
}

// readingViews is everything a screen asks about two notes: the per-note view,
// the queue and the archive.
func readingViews(t *testing.T, lib *store.Library, notes ...core.ID) (found []store.Reading, queue, archive []store.Reading) {
	t.Helper()
	for _, note := range notes {
		entry, ok := lib.ReadingFor(note)
		if !ok {
			t.Fatalf("note %q has no place in the reading queue", note)
		}
		found = append(found, entry)
	}
	queue, err := lib.ReadingQueue()
	if err != nil {
		t.Fatalf("ReadingQueue: %v", err)
	}
	archive, err = lib.ReadingArchive()
	if err != nil {
		t.Fatalf("ReadingArchive: %v", err)
	}
	return found, queue, archive
}

func sameQueue(a, b []store.Reading) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameReading(a[i], b[i]) {
			return false
		}
	}
	return true
}
