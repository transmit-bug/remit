package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// windowSizes are the sizes every mode has to hold at: the one the window
// opens at and the smallest it goes.
var windowSizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// The two modes are one window: the notes list beside the note, or the queue
// alone, switched by the real keys.
func TestModes(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a := NewApp()
			tt := ui.NewTester(a.View, size.w, size.h)
			tt.Frame()

			for _, want := range []string{"Notebook", "Memory and recall", "Make card"} {
				if !tt.HasText(want) {
					t.Fatalf("notes mode: missing %q; texts %q", want, tt.Texts())
				}
			}

			tt.Key(ui.Cmd, ui.Key1)
			for _, want := range []string{"Show answer", "Spacing beats cramming", "5 left"} {
				if !tt.HasText(want) {
					t.Fatalf("review mode: missing %q; texts %q", want, tt.Texts())
				}
			}
			for _, gone := range []string{"Notebook", "Make card", "A daily habit"} {
				if tt.HasText(gone) {
					t.Errorf("review mode still shows %q", gone)
				}
			}

			tt.Key(ui.Cmd, ui.Key2)
			if !tt.HasText("Notebook") || !tt.HasText("Make card") {
				t.Fatalf("notes mode did not come back; texts %q", tt.Texts())
			}
			if tt.HasText("Show answer") {
				t.Error("review stayed up after Cmd+2")
			}
			tt.Advance(200 * time.Millisecond) // the card's exit
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}

			// Esc leaves review too.
			tt.Key(ui.Cmd, ui.Key1)
			tt.Key(0, ui.KeyEscape)
			if !tt.HasText("Notebook") {
				t.Fatalf("Esc did not return to notes; texts %q", tt.Texts())
			}
		})
	}
}

// Review runs on keys: reveal, grade, the next card, undo, suspend and the
// remaining count, on the real view.
func TestReviewKeys(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key1)

	answer := "reviewing just before you forget strengthens recall."
	if !tt.HasText("Spacing beats cramming") {
		t.Fatalf("review did not open on the first card; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	if !tt.HasText(answer) {
		t.Fatalf("Space did not reveal the answer; texts %q", tt.Texts())
	}

	// Grading advances at once, to the lost card.
	tt.Key(0, ui.Key1)
	if tt.HasText(answer) {
		t.Error("the graded card's answer is still shown")
	}
	for _, want := range []string{"Lost", "The note block this card came from is gone.", "4 left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after grading; texts %q", want, tt.Texts())
		}
	}

	// A card cannot be graded before it is revealed.
	tt.Key(0, ui.Key3)
	if !tt.HasText("4 left") || !tt.HasText("Lost") {
		t.Error("a grade moved the queue before the answer was revealed")
	}

	// Undo goes back one card and says so once.
	tt.Key(0, ui.KeyU)
	for _, want := range []string{answer, "5 left", "Grade undone"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after undo; texts %q", want, tt.Texts())
		}
	}

	// Grade past the lost card, then suspend it, and the frozen card shows its
	// snapshot instead of the block's current text.
	tt.Key(0, ui.Key3)
	tt.Key(0, ui.KeyS)
	for _, want := range []string{"Frozen", "A good card asks", "3 left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after suspend; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("one question with one answer.") {
		t.Error("the frozen card follows its block instead of its snapshot")
	}
	tt.Key(0, ui.KeySpace)
	if !tt.HasText("one question at a time.") {
		t.Fatalf("the frozen snapshot's answer is missing; texts %q", tt.Texts())
	}

	// Easy grades and moves on.
	tt.Key(0, ui.Key4)
	for _, want := range []string{"The testing effect", "2 left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after grading; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText(answer) {
		t.Error("the answer stayed up on the next card")
	}

	// E opens the note the card came from.
	tt.Key(0, ui.KeyE)
	if !tt.HasText("Notebook") || !tt.HasText("Make card") {
		t.Fatalf("E did not return to the editor; texts %q", tt.Texts())
	}
	if !tt.HasText("Memory and recall") {
		t.Errorf("E did not open the card's note; texts %q", tt.Texts())
	}

	tt.Advance(5 * time.Second) // past the undo toast and the card's exit
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// The palette opens on Cmd+K, filters as the user types, runs what is chosen
// and returns the focus where it was.
func TestPalette(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()

	const field = "Block 1 · Memory and recall"
	if err := tt.Click(field); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused(field) {
		t.Fatalf("the block field did not take the focus; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.KeyK)
	for _, want := range []string{"Review cards", "Edit notes", "New note"} {
		if !tt.HasText(want) {
			t.Fatalf("palette is missing %q; texts %q", want, tt.Texts())
		}
	}

	tt.Type("new")
	if !tt.HasText("New note") {
		t.Fatalf("the filtered palette lost the action; texts %q", tt.Texts())
	}
	for _, gone := range []string{"Review cards", "Edit notes"} {
		if tt.HasText(gone) {
			t.Errorf("filter kept %q", gone)
		}
	}

	tt.Key(0, ui.KeyEscape)
	for _, gone := range []string{"Search actions", "Review cards", "Edit notes"} {
		if tt.HasText(gone) {
			t.Errorf("palette stayed open after Esc: %q", gone)
		}
	}
	if !tt.Focused(field) {
		t.Errorf("Esc did not return the focus to the block field; texts %q", tt.Texts())
	}

	// Typing filters, Enter runs the action on the selection.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("new")
	tt.Key(0, ui.KeyEnter)
	if tt.HasText("Search actions") {
		t.Error("the palette stayed open after running its action")
	}
	if !tt.HasText("Untitled note") {
		t.Fatalf("New note did not add a note; texts %q", tt.Texts())
	}
	if !tt.Focused("Block 1 · Untitled note") {
		t.Errorf("a new note's first block did not take the caret; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Editing a block's text edits the block itself, so the card that follows it
// shows the new sides.
func TestEditBlock(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()

	const field = "Block 1 · Memory and recall"
	if err := tt.Click(field); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("What is spacing? :: Practice spread over several days.")

	tt.Key(ui.Cmd, ui.Key1)
	if !tt.HasText("What is spacing?") {
		t.Fatalf("the edit did not reach the card; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	if !tt.HasText("Practice spread over several days.") {
		t.Fatalf("the edited answer is missing; texts %q", tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Choosing a note in the list shows that note's blocks.
func TestNotesList(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()

	if err := tt.Click("Writing cards"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Block 1 · Writing cards") {
		t.Fatalf("the editor did not follow the list; texts %q", tt.Texts())
	}
	if tt.HasText("Block 1 · Memory and recall") {
		t.Error("the editor still shows the note chosen before")
	}
	if tt.HasText("3 blocks · 2 cards") == false {
		t.Errorf("the note's block and card counts are missing; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A note's block becomes a card, split at its first ::.
func TestMakeCard(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()

	if err := tt.Click("Make card"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Card made from block 3") {
		t.Fatalf("no outcome for making a card; texts %q", tt.Texts())
	}
	if !tt.HasText("1 card") {
		t.Fatalf("the block is not marked as referenced; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // past the toast
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// At the smallest window every control is on screen and clickable, not only
// present as text.
func TestMinimumSizeControls(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 720, 520)
	tt.Frame()

	for _, label := range []string{"New note", "Notebook", "Memory and recall", "Block 1 · Memory and recall", "Make card"} {
		assertControl(t, tt, 720, 520, label)
	}

	if err := tt.Click("Make card"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Card made from block 3") {
		t.Fatalf("Make card was not clickable at the minimum; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // let the toast go

	tt.Key(ui.Cmd, ui.Key1)
	assertControl(t, tt, 720, 520, "Show answer")
	if err := tt.Click("Show answer"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Again", "Hard", "Good", "Easy", "Undo (U)", "Suspend (S)", "Edit note (E)"} {
		assertControl(t, tt, 720, 520, label)
	}
	if err := tt.Click("Good"); err != nil {
		t.Fatal(err)
	}
	assertControl(t, tt, 720, 520, "Suspend card")

	tt.Key(ui.Cmd, ui.KeyK)
	assertControl(t, tt, 720, 520, "Search actions")
	assertControl(t, tt, 720, 520, "Review cards")
	if err := tt.Click("Review cards"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("Search actions") {
		t.Error("the palette row was not clickable at the minimum")
	}
	tt.Advance(200 * time.Millisecond) // the card's transition

	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// One page in the dark: the same views draw from the dark theme.
func TestDark(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 1024, 720)
	tt.SetDark(true)
	tt.Frame()
	if !tt.HasText("Notebook") || !tt.HasText("Make card") {
		t.Fatalf("dark notes mode is incomplete; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.Key1)
	tt.Key(0, ui.KeySpace)
	if !tt.HasText("Show answer") && !tt.HasText("reviewing just before you forget strengthens recall.") {
		t.Fatalf("dark review mode lost its content; texts %q", tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// The About page names the app, shows its version and links its license at the
// size the window opens at.
func TestAbout(t *testing.T) {
	tt := ui.NewTester(About, aboutWidth, aboutHeight)
	tt.Frame()

	for _, want := range []string{Name, "Version " + Version, "GNU AGPL-3.0"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("GNU AGPL-3.0"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 1 || got[0] != licenseURL {
		t.Errorf("license link opened %q, want %q", got, licenseURL)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Packaging reads mygo.json's version and the About page shows Version: the
// two must not drift apart.
func TestVersionMatchesConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("mygo.json: %v", err)
	}
	if cfg.Version != Version {
		t.Errorf("mygo.json version %q, package Version %q", cfg.Version, Version)
	}
}

// assertControl checks that s has a box of its own inside the window, which is
// what makes it visible and clickable rather than only present as text.
func assertControl(t *testing.T, tt *ui.Tester, w, h int, s string) {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("%q has no box; texts %q", s, tt.Texts())
	}
	if r.W <= 0 || r.H <= 0 {
		t.Errorf("%q has an empty box %.1fx%.1f", s, r.W, r.H)
	}
	if r.X < 0 || r.Y < 0 || r.X+r.W > float32(w) || r.Y+r.H > float32(h) {
		t.Errorf("%q is outside the window: %.1f,%.1f %.1fx%.1f in %dx%d", s, r.X, r.Y, r.W, r.H, w, h)
	}
}
