package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The palette opens on Cmd+K, filters as the user types, runs what is chosen
// and returns the focus where it was.
func TestPalette(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit

	const field = "Note title"
	if err := tt.Click(field); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused(field) {
		t.Fatalf("the note's title did not take the focus; texts %q", tt.Texts())
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
	if !tt.Focused("Note title") {
		t.Errorf("a new note's title did not take the caret; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
