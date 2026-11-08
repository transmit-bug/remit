package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// writeBasicType replaces the built-in basic type with one whose front is
// front. Types are plain-text library data, so a test writes the file a user
// would edit, before the library opens.
func writeBasicType(t *testing.T, dir, front string) {
	t.Helper()
	data, err := json.MarshalIndent(map[string]any{
		"id":      "remit.basic",
		"name":    "Basic",
		"fields":  []string{"Front", "Back"},
		"front":   front,
		"back":    "{{Back}}",
		"version": 1,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "types"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "types", "remit.basic.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A template's {{hint:}} reaches the screen: the answer stays hidden, the
// hint's words arrive folded, and asking for them shows them.
func TestHintReachesTheScreen(t *testing.T) {
	dir := t.TempDir()
	writeBasicType(t, dir, "{{Front}}{{hint:Back}}")
	seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key1)

	const hint = "reviewing just before you forget strengthens recall."
	if !tt.HasText("Spacing beats cramming") {
		t.Fatalf("review did not open on the first card; texts %q", tt.Texts())
	}
	if tt.HasText(hint) {
		t.Fatal("the hint is open before it is asked for")
	}
	if err := tt.Click("Hint"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(hint) {
		t.Fatalf("the hint did not reach the screen; texts %q", tt.Texts())
	}
}

// A card whose type asks for a typed answer owns the keyboard until it is
// checked: a digit typed into the input is the answer's, not a grade, Space does
// not reveal the answer, Enter checks it, and the grades come back afterwards.
func TestTypedAnswerTakesTheKeys(t *testing.T) {
	dir := t.TempDir()
	writeBasicType(t, dir, "{{Front}}{{type:Back}}")
	seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key1)

	msgs := locales.In("en")
	if !tt.HasText(msgs.CheckAnswer()) {
		t.Fatalf("the typed card is not on screen; texts %q", tt.Texts())
	}

	// The input takes the digit and the space; neither grades nor reveals.
	tt.TypeKey(0, ui.Key4, "4")
	tt.TypeKey(0, ui.KeySpace, " ")
	const answer = "reviewing just before you forget strengthens recall."
	if !tt.HasText("4 cards left") {
		t.Fatalf("a key graded the card while its answer was being typed; texts %q", tt.Texts())
	}
	if tt.HasText(answer) {
		t.Fatal("Space revealed the answer while it was being typed")
	}
	field, ok := tt.Access(msgs.TypedAnswer())
	if !ok || field.Value != "4 " {
		t.Fatalf("the input holds %q (found %v), want %q", field.Value, ok, "4 ")
	}

	// With the focus off the input, Space must still leave the answer hidden:
	// a typed card is checked, not revealed.
	if err := tt.Click("Spacing beats cramming"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeySpace)
	if tt.HasText(answer) {
		t.Fatal("Space revealed the answer of a card that is waiting to be checked")
	}
	if field, _ := tt.Access(msgs.TypedAnswer()); field.Value != "4 " {
		t.Errorf("the input holds %q after Space left the focus, want %q", field.Value, "4 ")
	}

	// The space is one of the answer's characters, so an otherwise wrong answer
	// that holds one is Partial, not Wrong: the comparison is literal.
	if err := tt.Click(msgs.CheckAnswer()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{msgs.AnswerPartly(), answer, msgs.Easy()} {
		if !tt.HasText(want) {
			t.Fatalf("after checking, %q is missing; texts %q", want, tt.Texts())
		}
	}

	tt.Key(0, ui.Key3)
	tt.Advance(320 * time.Millisecond) // the graded card's exit
	if !tt.HasText("3 cards left") {
		t.Errorf("Good did not grade the checked card; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Review takes the window whole: no navigation and no notes list, only the dual
// mode's switch, the card and its status bar — and Esc gives the notebook back.
func TestReviewTakesTheWindow(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)

			// The toolbar's words are gone, so are the notes list's; the switch
			// between the two queues is all the navigation that is left.
			for _, gone := range []string{"New note", "Notebook", "Browse", "Stats"} {
				if tt.HasText(gone) {
					t.Errorf("the taking-over screen still shows %q", gone)
				}
			}
			for _, want := range []string{"Review", "Reading", "Show answer", "4 cards left"} {
				if !tt.HasText(want) {
					t.Fatalf("the screen lost %q; texts %q", want, tt.Texts())
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

// The status bar says what is left, how far along, and shows the keys, so the
// whole flow needs no pointer.
func TestReviewStatusBarCarriesTheKeys(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)

			if _, ok := tt.Find("4 cards left"); !ok {
				t.Fatalf("the status bar does not say what is left; texts %q", tt.Texts())
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
	_, _, tt := openApp(t)

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

// At the smallest window the card's content and its four grades are all still
// on screen.
func TestReviewAtTheSmallestWindow(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)

	if _, ok := tt.Find("Spacing beats cramming"); !ok {
		t.Fatalf("the prompt is not on screen; texts %q", tt.Texts())
	}

	// The grades fit too, once the answer is up.
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond)
	for _, label := range []string{"Again", "Hard", "Good", "Easy"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("the grade %q is missing at 720x520; texts %q", label, tt.Texts())
		}
	}
}

// The interval on each grade key is the scheduler's own answer: what the store
// says the answer would schedule, said on the key.
func TestGradeKeysShowTheSchedulesIntervals(t *testing.T) {
	a, _, tt := openApp(t)
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
	a, seed, tt := openApp(t)
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
	if !tt.HasText("Notebook") || a.notes.Loaded != string(seed.writing.ID) {
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
