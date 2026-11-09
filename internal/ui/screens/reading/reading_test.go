package reading_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/reading"
	"github.com/transmit-bug/remit/locales"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// fake is the app the screen talks to: the notes it shows and the calls it made,
// with no library, no clock and no disk behind it.
type fake struct {
	notes    map[core.ID]reading.Note
	sources  map[string]core.ID
	reads    []readCall
	skipped  []core.ID
	dropped  []core.ID
	edited   []core.ID
	excerpts []excerptCall
	opened   []core.ID
	// ended counts the session's terminal decisions, and queued are the notes
	// their undo put back.
	ended  map[string]int
	queued []core.ID
	review int
	home   int
}

type readCall struct {
	note     core.ID
	rating   reading.Rating
	position core.ID
}

type excerptCall struct {
	note  core.ID
	block core.ID
	card  bool
}

func (f *fake) deps() reading.Deps {
	return reading.Deps{
		Note: func(id core.ID) (reading.Note, bool) {
			note, ok := f.notes[id]
			return note, ok
		},
		Read: func(id core.ID, r reading.Rating, position core.ID) {
			f.reads = append(f.reads, readCall{note: id, rating: r, position: position})
		},
		Skip:     func(id core.ID) { f.skipped = append(f.skipped, id) },
		Drop:     func(id core.ID) { f.dropped = append(f.dropped, id) },
		EditNote: func(id core.ID) { f.edited = append(f.edited, id) },
		Archive:  func() { f.ended["archive"]++ },
		Dismiss:  func() { f.ended["dismiss"]++ },
		Postpone: func() { f.ended["postpone"]++ },
		QueueNote: func(id core.ID) {
			f.queued = append(f.queued, id)
		},
		Excerpt: func(note, block core.ID, card bool) {
			f.excerpts = append(f.excerpts, excerptCall{note: note, block: block, card: card})
		},
		Source: func(block core.Block) (core.ID, bool) {
			for _, link := range core.Links(block.Text) {
				if note, ok := f.sources[core.NormalizeTitle(link.Title)]; ok {
					return note, true
				}
			}
			return "", false
		},
		Open:   func(id core.ID) { f.opened = append(f.opened, id) },
		Review: func() { f.review++ },
		Home:   func() { f.home++ },
	}
}

// note builds a note of n paragraphs, each named so a test can find it. Its
// blocks carry real IDs, so a reference link in one of them resolves.
func note(id core.ID, n int) reading.Note {
	out := reading.Note{ID: id, Title: "Note " + string(id)}
	for i := 0; i < n; i++ {
		out.Blocks = append(out.Blocks, core.Block{
			ID:   core.NewID(core.BlockKind),
			Text: fmt.Sprintf("Paragraph %d of %s", i+1, id),
		})
	}
	return out
}

// open builds the screen on a queue of notes, in the order they were given.
func open(t *testing.T, notes ...reading.Note) (*fake, *reading.State, reading.Deps) {
	t.Helper()
	f := &fake{notes: map[core.ID]reading.Note{}, sources: map[string]core.ID{}, ended: map[string]int{}}
	order := make([]core.ID, 0, len(notes))
	for _, n := range notes {
		f.notes[n.ID] = n
		f.sources[core.NormalizeTitle(n.Title)] = n.ID
		order = append(order, n.ID)
	}
	state := &reading.State{}
	state.Sync(order)
	return f, state, f.deps()
}

// at draws the screen once at a size and lets it settle.
func at(t *testing.T, view func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(view, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// view is the screen bound to a state and the app behind it.
func view(state *reading.State, deps reading.Deps) func(*ui.Context) {
	return func(c *ui.Context) { reading.View(c, state, deps) }
}

// The note reads whole: its title, every paragraph, where the reader is in it,
// and the keys that move the session — at both sizes the window opens at.
func TestNoteOnScreen(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			_, state, deps := open(t, note("note_a", 3))
			tt := at(t, view(state, deps), size.w, size.h)

			for _, want := range []string{
				"Note note_a",
				"Paragraph 1 of note_a", "Paragraph 3 of note_a",
				msgs.ReadingWhere(1, 3),
				msgs.ReadingProgress(0, 1),
				msgs.ReadingTooHard(), msgs.ReadingJustRight(), msgs.ReadingTooShallow(),
				msgs.Excerpt(), msgs.Skip(), msgs.RemoveReading(),
			} {
				if !tt.HasText(want) {
					t.Errorf("missing %q; texts %q", want, tt.Texts())
				}
			}
			if state.Remaining() != 1 {
				t.Errorf("remaining = %d, want 1", state.Remaining())
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// A note that was left mid-way opens where it was left: the reading position is
// the block the last session stopped at.
func TestResumeAtTheStoredPosition(t *testing.T) {
	msgs := locales.In("en")
	left := note("note_a", 3)
	left.Position = left.Blocks[1].ID
	_, state, deps := open(t, left)
	tt := at(t, view(state, deps), 1024, 720)

	if !tt.HasText(msgs.ReadingWhere(2, 3)) {
		t.Fatalf("the screen did not resume at the second paragraph; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.ReadingCurrent(2)) {
		t.Error("the paragraph the reader is on is not named as the reading position")
	}
	if got := state.Cursor(left); got != left.Blocks[1].ID {
		t.Errorf("cursor = %q, want the block the note was left at", got)
	}

	// Moving goes on from there, and stops at the last paragraph.
	state.Move(left, 1)
	if got := state.Cursor(left); got != left.Blocks[2].ID {
		t.Errorf("a move of one from the second paragraph gave %q", got)
	}
	state.Move(left, 5)
	if got := state.Cursor(left); got != left.Blocks[2].ID {
		t.Errorf("moving past the last paragraph wrapped to %q", got)
	}
	state.Move(left, -5)
	if got := state.Cursor(left); got != left.Blocks[0].ID {
		t.Errorf("moving before the first paragraph gave %q", got)
	}
}

// Excerpting takes the block at the reading position into a note of its own —
// with a card when the checkbox asks for one — and leaves the reader where they
// were, with the focus back on the paragraph they are reading.
func TestExcerptLeavesTheReaderInPlace(t *testing.T) {
	msgs := locales.In("en")
	passage := note("note_a", 3)
	f, state, deps := open(t, passage)
	tt := at(t, view(state, deps), 1024, 720)

	state.Move(passage, 1)
	tt.Frame()
	if !tt.HasText(msgs.ReadingWhere(2, 3)) {
		t.Fatalf("the move did not reach the second paragraph; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.Excerpt()); err != nil {
		t.Fatal(err)
	}
	if len(f.excerpts) != 1 {
		t.Fatalf("the app was asked for %d excerpts, want 1", len(f.excerpts))
	}
	if got := f.excerpts[0]; got.note != "note_a" || got.block != passage.Blocks[1].ID || got.card {
		t.Errorf("excerpted %+v, want the second paragraph of note_a without a card", got)
	}
	tt.Frame()
	if !tt.Focused(msgs.ReadingCurrent(2)) {
		t.Error("the focus did not come back to the reading position")
	}
	if !tt.HasText(msgs.ReadingWhere(2, 3)) {
		t.Errorf("the excerpt moved the reader; texts %q", tt.Texts())
	}

	// The checkbox makes the same gesture a card too.
	if err := tt.Click(msgs.ExcerptWithCard()); err != nil {
		t.Fatal(err)
	}
	if !state.ExcerptWithCard() {
		t.Fatal("the checkbox did not take")
	}
	if err := tt.Click(msgs.Excerpt()); err != nil {
		t.Fatal(err)
	}
	if len(f.excerpts) != 2 || !f.excerpts[1].card {
		t.Errorf("the second excerpt = %+v, want a card too", f.excerpts)
	}
}

// Recording how the note digested hands the app the difficulty and the block the
// reader stopped at, which is what the schedule and the next session read.
func TestReadingDifficultyRecordsThePosition(t *testing.T) {
	msgs := locales.In("en")
	passage := note("note_a", 3)
	f, state, deps := open(t, passage)
	tt := at(t, view(state, deps), 1024, 720)
	state.Move(passage, 2)
	tt.Frame()

	if err := tt.Click(msgs.ReadingTooShallow()); err != nil {
		t.Fatal(err)
	}
	if len(f.reads) != 1 {
		t.Fatalf("the app was told %d readings, want 1", len(f.reads))
	}
	want := readCall{note: "note_a", rating: reading.TooShallow, position: passage.Blocks[2].ID}
	if f.reads[0] != want {
		t.Errorf("reading = %+v, want %+v", f.reads[0], want)
	}

	// The three difficulties are the schedule's Again, Good and Easy.
	for r, want := range map[reading.Rating]string{
		reading.TooHard: "again", reading.JustRight: "good", reading.TooShallow: "easy",
	} {
		if got := r.Schedule().String(); got != want {
			t.Errorf("%v schedules as %q, want %q", r, got, want)
		}
	}
}

// Skipping, removing, editing and leaving reach the app: the screen writes
// nothing itself.
func TestActionsReachTheApp(t *testing.T) {
	msgs := locales.In("en")
	f, state, deps := open(t, note("note_a", 2), note("note_b", 2))
	tt := at(t, view(state, deps), 1024, 720)

	for _, s := range []string{msgs.Skip(), msgs.RemoveReading(), msgs.ReadingEdit()} {
		if err := tt.Click(s); err != nil {
			t.Fatalf("clicking %q: %v", s, err)
		}
	}
	if len(f.skipped) != 1 || f.skipped[0] != "note_a" {
		t.Errorf("skipped = %v, want note_a", f.skipped)
	}
	if len(f.dropped) != 1 || f.dropped[0] != "note_a" {
		t.Errorf("dropped = %v, want note_a", f.dropped)
	}
	if len(f.edited) != 1 || f.edited[0] != "note_a" {
		t.Errorf("edited = %v, want note_a", f.edited)
	}
	if !tt.HasText(msgs.ReadingProgress(0, 2)) {
		t.Errorf("the progress line does not count both notes; texts %q", tt.Texts())
	}
}

// A block that carries a reference to another note shows the way back, and the
// click opens that note.
func TestReferenceOpensTheSource(t *testing.T) {
	msgs := locales.In("en")
	source := note("note_src", 1)
	child := reading.Note{ID: "note_child", Title: "Excerpt of note_src", Blocks: []core.Block{{
		ID:   "block_note_child_0",
		Text: "a passage\n[[" + source.Title + "#^" + string(source.Blocks[0].ID) + "]]",
	}}}
	f, state, deps := open(t, child)
	// The source note is in the library the app reads, though not in the queue.
	f.sources[core.NormalizeTitle(source.Title)] = source.ID
	tt := at(t, view(state, deps), 1024, 720)

	if !tt.HasText(msgs.OpenSource()) {
		t.Fatalf("the reference shows no way back; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.OpenSource()); err != nil {
		t.Fatal(err)
	}
	if len(f.opened) != 1 || f.opened[0] != source.ID {
		t.Errorf("opened %v, want the source note", f.opened)
	}
}

// Nothing due is a state of its own: it says so, and offers the other queue and
// the notebook.
func TestEmptyQueue(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f, state, deps := open(t)
			tt := at(t, view(state, deps), size.w, size.h)
			if !tt.HasText(msgs.NoReading()) {
				t.Fatalf("the empty state says nothing; texts %q", tt.Texts())
			}
			if err := tt.Click(msgs.ModeReview()); err != nil {
				t.Fatal(err)
			}
			if f.review != 1 {
				t.Errorf("the review action ran %d times, want 1", f.review)
			}
			if err := tt.Click(msgs.BackToNotes()); err != nil {
				t.Fatal(err)
			}
			if f.home != 1 {
				t.Errorf("the notes action ran %d times, want 1", f.home)
			}
		})
	}
}

// The queue is the store's, the order the session's: a note that left the queue
// leaves the screen, and one that became due joins the end.
func TestSyncKeepsTheOrder(t *testing.T) {
	_, state, _ := open(t, note("note_a", 1), note("note_b", 1), note("note_c", 1))
	head, _ := state.Current()
	if head != "note_a" {
		t.Fatalf("head = %q, want note_a", head)
	}

	state.Sync([]core.ID{"note_c", "note_a", "note_d"})
	if got := state.Remaining(); got != 3 {
		t.Fatalf("remaining = %d, want 3", got)
	}
	if head, _ := state.Current(); head != "note_a" {
		t.Errorf("the note on screen moved to %q", head)
	}

	// The note on screen leaving the queue puts the next one up, and the
	// position of the note before is forgotten with it.
	state.Move(note("note_a", 1), 0)
	state.Sync([]core.ID{"note_c", "note_d"})
	if head, _ := state.Current(); head != "note_c" {
		t.Errorf("head = %q after the note on screen left, want note_c", head)
	}
}

// The screen is one language at a time: the Chinese catalog reaches every word
// of it, at the smallest window.
func TestChineseAtTheMinimumSize(t *testing.T) {
	msgs := locales.In("zh-Hans")
	_, state, deps := open(t, note("note_a", 2))
	tt := ui.NewTester(view(state, deps), 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)

	for _, want := range []string{
		msgs.ReadingTooHard(), msgs.ReadingJustRight(), msgs.ReadingTooShallow(),
		msgs.Excerpt(), msgs.Skip(), msgs.ReadArchive(), msgs.ReadDismiss(), msgs.ReadPostpone(),
		msgs.BackToNotes(), msgs.ReadingWhere(1, 2), "Note note_a", "Paragraph 1 of note_a",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q; texts %q", want, tt.Texts())
		}
	}
}

// Ending a note — archiving it, dismissing it, postponing it — is one gesture
// with three meanings: the app writes the event, the session moves on without
// recording an answer, and the toast says what happened with the way back.
func TestEndingANoteTellsAndUndoes(t *testing.T) {
	msgs := locales.In("en")
	for _, decision := range []struct {
		name  string
		label string
		told  string
		key   string
	}{
		{"archive", msgs.ReadArchive(), msgs.ReadArchived("Note note_a"), "archive"},
		{"dismiss", msgs.ReadDismiss(), msgs.ReadDismissed("Note note_a"), "dismiss"},
		{"postpone", msgs.ReadPostpone(), msgs.ReadPostponed("Note note_a"), "postpone"},
	} {
		t.Run(decision.name, func(t *testing.T) {
			f, state, deps := open(t, note("note_a", 2), note("note_b", 2))
			tt := at(t, view(state, deps), 1024, 720)

			if err := tt.Click(decision.label); err != nil {
				t.Fatalf("the session has no %q action: %v", decision.label, err)
			}
			tt.Frame()
			if f.ended[decision.key] != 1 {
				t.Errorf("the app was asked to %s %d times, want 1", decision.name, f.ended[decision.key])
			}
			if len(f.reads) != 0 {
				t.Errorf("ending a note recorded %d answers, want none", len(f.reads))
			}
			if head, _ := state.Current(); head != "note_b" {
				t.Errorf("the session is on %q, want the next note", head)
			}
			if !tt.HasText(decision.told) {
				t.Fatalf("the decision said nothing; texts %q", tt.Texts())
			}

			if err := tt.Click(msgs.ReadUndo()); err != nil {
				t.Fatalf("the toast has no undo: %v", err)
			}
			if len(f.queued) != 1 || f.queued[0] != "note_a" {
				t.Errorf("the undo queued %v, want the note that was ended", f.queued)
			}
		})
	}
}

// The session moves past a note the store already took out of its queue: ending
// the note on screen must not skip the one after it.
func TestEndingANoteTheStoreAlreadyDropped(t *testing.T) {
	msgs := locales.In("en")
	f, state, deps := open(t, note("note_a", 1), note("note_b", 1), note("note_c", 1))
	tt := at(t, view(state, deps), 1024, 720)

	// The reload behind the write drops the note, as the store does; the session
	// must stay on the next one whether or not it ever held the ended note.
	// The store's answer, as the reload behind the write applies it.
	deps.Archive = func() {
		f.ended["archive"]++
		state.Sync([]core.ID{"note_b", "note_c"})
	}
	if err := tt.Click(msgs.ReadArchive()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if head, _ := state.Current(); head != "note_b" {
		t.Errorf("the session is on %q, want note_b", head)
	}
	if got := state.Remaining(); got != 2 {
		t.Errorf("remaining = %d, want 2", got)
	}
}

// A paragraph's marks say what it already carries: the cards that quote it, the
// excerpt taken from it, and whether it is where the last session stopped.
func TestParagraphsCarryTheirMarks(t *testing.T) {
	msgs := locales.In("en")
	passage := note("note_a", 3)
	passage.Marks = map[core.ID]reading.Mark{
		passage.Blocks[0].ID: {Cards: 2},
		passage.Blocks[1].ID: {Excerpted: true, LastStop: true},
	}
	_, state, deps := open(t, passage)
	tt := at(t, view(state, deps), 1024, 720)

	for _, want := range []string{msgs.ReadCardsMark(2), msgs.ReadExcerptedMark(), msgs.ReadLastStopMark()} {
		if !tt.HasText(want) {
			t.Errorf("the paragraph marks lost %q; texts %q", want, tt.Texts())
		}
	}
}

// The action row keeps its two groups readable at the smallest window: the three
// difficulties, and what to do with the note itself.
func TestActionRowAtTheMinimumSize(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			_, state, deps := open(t, note("note_a", 3))
			tt := at(t, view(state, deps), size.w, size.h)
			for _, label := range []string{
				msgs.ReadingTooHard(), msgs.ReadingJustRight(), msgs.ReadingTooShallow(),
				msgs.ExcerptWithCard(), msgs.Excerpt(), msgs.Skip(),
				msgs.ReadArchive(), msgs.ReadDismiss(), msgs.ReadPostpone(),
				msgs.RemoveReading(), msgs.ReadingEdit(), msgs.BackToNotes(),
			} {
				r, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%q has no box at %dx%d; texts %q", label, size.w, size.h, tt.Texts())
				}
				if r.W <= 0 || r.H <= 0 {
					t.Errorf("%q has an empty box %.1fx%.1f", label, r.W, r.H)
				}
			}
			if tt.FramePending() {
				t.Error("the view still requests frames after it should settle")
			}
		})
	}
}
