package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Choosing a note in the list shows that note's blocks.
func TestNotesList(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit

	if err := tt.Click("Writing cards"); err != nil {
		t.Fatal(err)
	}
	if a.notes.Doc == nil || !strings.Contains(a.notes.Doc.Text().String(), "A good card asks") {
		t.Fatalf("the editor did not follow the list; texts %q", tt.Texts())
	}
	if strings.Contains(a.notes.Doc.Text().String(), "When to review") {
		t.Error("the editor still shows the note chosen before")
	}
	if !tt.HasText("Note panel") {
		t.Errorf("the note's panel is missing; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
