package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// Review takes the window whole: no navigation and no notes list, only the card
// and its status bar — and Esc gives the notebook back.
func TestReviewTakesTheWindow(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)

			// The toolbar's words are gone, so are the notes list's.
			for _, gone := range []string{"Mode", "New note", "Notebook"} {
				if tt.HasText(gone) {
					t.Errorf("the taking-over screen still shows %q", gone)
				}
			}
			for _, want := range []string{"Show answer", "4 cards left"} {
				if !tt.HasText(want) {
					t.Fatalf("the card lost %q; texts %q", want, tt.Texts())
				}
			}

			tt.Key(0, ui.KeyEscape)
			tt.Advance(320 * time.Millisecond) // the screen's entrance
			// The notebook is back, with its navigation.
			for _, want := range []string{"Notebook", "New note", "Memory and recall"} {
				if !tt.HasText(want) {
					t.Fatalf("Esc did not give the notebook back: missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// The status bar is one thin line at the top: what is left, how far along, and
// never taller than 28 DIP.
func TestReviewStatusBarIsThin(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)

			remaining, ok := tt.Find("4 cards left")
			if !ok {
				t.Fatalf("the status bar does not say what is left; texts %q", tt.Texts())
			}
			if remaining.Y+remaining.H > 28 {
				t.Errorf("the status bar's line reaches %v DIP, want 28 or less", remaining.Y+remaining.H)
			}
			// It holds the keys as well, so the whole flow needs no pointer.
			for _, key := range []string{"Space", "U", "S", "E", "Esc"} {
				if _, ok := tt.Find(key); !ok {
					t.Errorf("the status bar does not show the %q key; texts %q", key, tt.Texts())
				}
			}
		})
	}
}

// The whole review flow runs on keys: reveal, grade, undo, suspend, edit and
// leave — with no pointer and nothing else on screen.
func TestReviewIsKeyboardOnly(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	// Reveal, grade, and take the grade back.
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond) // the answer unfolding
	if !tt.HasText("reviewing just before you forget strengthens recall.") {
		t.Fatalf("Space did not reveal the answer; texts %q", tt.Texts())
	}
	tt.Key(0, ui.Key3)
	tt.Advance(320 * time.Millisecond) // the graded card's exit
	if !tt.HasText("3 cards left") {
		t.Errorf("grading did not move to the next card; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyU)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("4 cards left") {
		t.Errorf("undo did not put the card back; texts %q", tt.Texts())
	}

	// Suspend takes the card out of the queue without grading it.
	tt.Key(0, ui.KeyS)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("3 cards left") {
		t.Errorf("suspend did not take the card out of the queue; texts %q", tt.Texts())
	}

	// Edit the note behind the card, then leave.
	tt.Key(0, ui.KeyE)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("Notebook") || !tt.HasText("Memory and recall") {
		t.Fatalf("E did not open the note; texts %q", tt.Texts())
	}
	tt.Key(ui.Cmd, ui.Key1)
	tt.Advance(320 * time.Millisecond)
	tt.Key(0, ui.KeyEscape)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("Notebook") {
		t.Errorf("Esc did not leave review for the notes; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// At the smallest window the card keeps its room: its content stays inside a
// 24 DIP margin, and the screen is usable whole.
func TestReviewAtTheSmallestWindow(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)

	prompt, ok := tt.Find("Spacing beats cramming")
	if !ok {
		t.Fatalf("the prompt is not on screen; texts %q", tt.Texts())
	}
	if prompt.X < 24 || prompt.Y < 24 || prompt.X+prompt.W > 720-24 || prompt.Y+prompt.H > 520 {
		t.Errorf("the card's content %+v is outside the 24 DIP margin of a 720x520 window", prompt)
	}

	// The grades fit too, once the answer is up.
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond)
	for _, label := range []string{"Again", "Hard", "Good", "Easy"} {
		box, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the grade %q is missing at 720x520; texts %q", label, tt.Texts())
		}
		if box.X < 0 || box.X+box.W > 720 {
			t.Errorf("the grade %q box %+v is outside the window", label, box)
		}
	}
}

// The interval on each grade key is the scheduler's own answer: what the store
// says the answer would schedule, said on the key.
func TestGradeKeysShowTheSchedulesIntervals(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond) // the answer unfolding

	id, ok := a.review.Current()
	if !ok {
		t.Fatal("no card on screen")
	}
	outlooks, ok := a.Library().Preview(id, time.Now())
	if !ok {
		t.Fatal("the store has no preview for the card on screen")
	}
	msgs := locales.In("en")
	want := func(w time.Duration) string {
		switch {
		case w < time.Hour:
			return msgs.IntervalMinutes(max(1, int(w.Round(time.Minute)/time.Minute)))
		case w < 24*time.Hour:
			return msgs.IntervalHours(max(1, int(w.Round(time.Hour)/time.Hour)))
		default:
			return msgs.IntervalDays(max(1, int(w.Round(24*time.Hour)/(24*time.Hour))))
		}
	}
	for _, o := range outlooks {
		if !tt.HasText(want(o.Wait)) {
			t.Errorf("the %v key does not show %q; texts %q", o.Rating, want(o.Wait), tt.Texts())
		}
	}
}

// Review runs on keys: reveal, grade, the next card, undo, suspend and the
// remaining count, on the real view over the real store.
func TestReviewKeys(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key1)

	answer := "reviewing just before you forget strengthens recall."
	if !tt.HasText("Spacing beats cramming") {
		t.Fatalf("review did not open on the first card; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	if !tt.HasText(answer) {
		t.Fatalf("Space did not reveal the answer; texts %q", tt.Texts())
	}

	// Grading advances at once, to the frozen card.
	tt.Key(0, ui.Key1)
	if tt.HasText(answer) {
		t.Error("the graded card's answer is still shown")
	}
	for _, want := range []string{"Frozen", "A good card asks", "3 cards left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after grading; texts %q", want, tt.Texts())
		}
	}

	// A card cannot be graded before it is revealed.
	tt.Key(0, ui.Key3)
	if !tt.HasText("3 cards left") || !tt.HasText("Frozen") {
		t.Error("a grade moved the queue before the answer was revealed")
	}

	// The frozen card shows its snapshot, not the block's current text.
	if tt.HasText("one question with one answer.") {
		t.Error("the frozen card follows its block instead of its snapshot")
	}
	tt.Key(0, ui.KeySpace)
	if !tt.HasText("one question at a time.") {
		t.Fatalf("the frozen snapshot's answer is missing; texts %q", tt.Texts())
	}

	// Undo goes back one card, answer shown, and says so once.
	tt.Key(0, ui.KeyU)
	for _, want := range []string{answer, "4 cards left", "Grade undone"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after undo; texts %q", want, tt.Texts())
		}
	}

	// Grade again, then Easy moves on without the answer coming back up.
	tt.Key(0, ui.Key1)
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key4)
	for _, want := range []string{"The testing effect", "2 cards left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after grading; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("one question at a time.") {
		t.Error("the answer stayed up on the next card")
	}

	// S suspends the card shown; it leaves the queue without a review.
	tt.Key(0, ui.KeyS)
	for _, want := range []string{"A block divides at", "1 card left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after suspend; texts %q", want, tt.Texts())
		}
	}
	suspended, err := os.ReadFile(filepath.Join(seed.dir, "cards", string(seed.testing.ID)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(suspended), `"status": "suspended"`) {
		t.Errorf("suspending did not reach the card file: %s", suspended)
	}

	// E opens the note the card came from.
	tt.Key(0, ui.KeyE)
	if !tt.HasText("Notebook") || a.editor.loaded != string(seed.writing.ID) {
		t.Fatalf("E did not open the card's note; texts %q", tt.Texts())
	}

	tt.Advance(5 * time.Second) // past the undo toast and the card's exit
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A grade is a review event in the library, so a later run finds the card
// scheduled rather than due.
func TestReviewPersists(t *testing.T) {
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key1)
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key3) // Good
	if !tt.HasText("3 cards left") {
		t.Fatalf("Good did not move the queue on; texts %q", tt.Texts())
	}

	// The log is the source of truth, and it holds the review.
	logged, err := os.ReadFile(filepath.Join(dir, "reviews.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), string(seed.spacing.ID)) || !strings.Contains(string(logged), `"rating":"good"`) {
		t.Fatalf("the review is not in the log: %s", logged)
	}

	// A second app on the same directory opens it without the graded card, so
	// the due date survived the write.
	next := NewApp(Options{LibraryDir: dir})
	tt = ui.NewTester(next.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key1)
	for _, want := range []string{"A good card asks", "3 cards left"} {
		if !tt.HasText(want) {
			t.Fatalf("reopened review is missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("Spacing beats cramming") {
		t.Error("the graded card is due again in the reopened library")
	}
}
