package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A library opens on today's review queue, and the notes are one key away:
// the two modes are one window, switched by the real keys.
func TestModes(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)

			// The first screen is the review queue (decision #11).
			for _, want := range []string{"Show answer", "Spacing beats cramming", "4 cards left"} {
				if !tt.HasText(want) {
					t.Fatalf("review mode: missing %q; texts %q", want, tt.Texts())
				}
			}
			for _, gone := range []string{"Notebook", "Make card", "A daily habit"} {
				if tt.HasText(gone) {
					t.Errorf("review mode shows %q", gone)
				}
			}

			tt.Key(ui.Cmd, ui.Key2)
			tt.Advance(320 * time.Millisecond) // the card's exit
			for _, want := range []string{"Notebook", "Memory and recall", "Note title"} {
				if !tt.HasText(want) {
					t.Fatalf("notes mode: missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.HasText("Show answer") {
				t.Error("review stayed up after Cmd+2")
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}

			// Cmd+1 comes back, and Esc leaves review for the notes.
			tt.Key(ui.Cmd, ui.Key1)
			if !tt.HasText("Show answer") {
				t.Fatalf("Cmd+1 did not return to review; texts %q", tt.Texts())
			}
			tt.Advance(320 * time.Millisecond) // the card's entry
			tt.Key(0, ui.KeyEscape)
			tt.Advance(320 * time.Millisecond) // the card's exit
			if !tt.HasText("Notebook") {
				t.Fatalf("Esc did not return to notes; texts %q", tt.Texts())
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// At the smallest window every control is on screen and clickable, not only
// present as text.
func TestMinimumSizeControls(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)

	// The queue the window opens on is usable at the smallest size.
	assertControl(t, tt, 720, 520, "Show answer")
	tt.Advance(320 * time.Millisecond) // the card's entry

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	for _, label := range []string{"New note", "Notebook", "A daily habit", "Note title"} {
		assertControl(t, tt, 720, 520, label)
	}

	// Making a card is a command now, reachable at the smallest window too.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("make card")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("Card made from block 1") {
		t.Fatalf("Make card did not run at the minimum; texts %q", tt.Texts())
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
	assertControl(t, tt, 720, 520, "Show answer")

	tt.Key(ui.Cmd, ui.KeyK)
	assertControl(t, tt, 720, 520, "Search actions")
	assertControl(t, tt, 720, 520, "Edit notes")
	tt.Advance(320 * time.Millisecond) // the palette's arrival
	if err := tt.Click("Edit notes"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond) // the screen's change
	if tt.HasText("Search actions") {
		t.Error("the palette stayed open after its row was clicked")
	}
	if !tt.HasText("Notebook") {
		t.Error("the palette's action did not run")
	}
	tt.Advance(320 * time.Millisecond) // the card's transition

	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// One page in the dark: the same views draw from the dark theme.
func TestDark(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	tt.SetDark(true)
	tt.Frame()
	if !tt.HasText("Show answer") {
		t.Fatalf("dark review mode is incomplete; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	if !tt.HasText("Notebook") || !tt.HasText("Note title") {
		t.Fatalf("dark notes mode is incomplete; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.Key1)
	tt.Key(0, ui.KeySpace)
	if !tt.HasText("reviewing just before you forget strengthens recall.") {
		t.Fatalf("dark review mode lost its content; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A library that cannot be opened is one line and one action, not a crash;
// the action opens it once the problem is gone.
func TestCorruptLibrary(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "remit.json")
	if err := os.WriteFile(config, []byte(`{"format": "remit/9"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	if !tt.HasText("Remit could not open its library") || !tt.HasText("remit/9") {
		t.Fatalf("the error state does not say what happened; texts %q", tt.Texts())
	}
	if tt.HasText("Notebook") {
		t.Error("the window drew a library it could not open")
	}
	assertControl(t, tt, 1024, 720, "Try again")

	if err := os.WriteFile(config, []byte(`{"format": "remit/2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Try again"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Today's review") || tt.HasText("Remit could not open its library") {
		t.Fatalf("trying again did not open the repaired library; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
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

// Every message comes from a catalog, so switching the language switches what
// the view says, without a restart and without a second code path.
func TestLocaleSwitchesTheLanguage(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)

	tt.SetLocale("zh-Hans")
	tt.Frame()
	for _, want := range []string{"显示答案", "撤销 (U)"} {
		if !tt.HasText(want) {
			t.Errorf("missing Chinese text %q; texts %q", want, tt.Texts())
		}
	}

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	for _, want := range []string{"笔记", "笔记本"} {
		if !tt.HasText(want) {
			t.Errorf("missing Chinese text %q in the notes; texts %q", want, tt.Texts())
		}
	}

	tt.SetLocale("en")
	tt.Frame()
	for _, want := range []string{"Notes", "Notebook", "New note"} {
		if !tt.HasText(want) {
			t.Errorf("missing English text %q after switching back; texts %q", want, tt.Texts())
		}
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after a locale switch")
	}
}
