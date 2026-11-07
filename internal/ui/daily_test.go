package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
)

// ⌘D opens today's note and writes it the first time; the second time finds the
// note it wrote.
func TestDailyNoteOpensToday(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	today := dailyTitle(time.Now())
	tt.Key(ui.Cmd, ui.KeyD)
	tt.Advance(320 * time.Millisecond)
	if a.mode != modeNotes {
		t.Fatalf("⌘D left the notebook for %v", a.mode)
	}
	if !tt.HasText(today) {
		t.Fatalf("today's note is not on screen; texts %q", tt.Texts())
	}
	note, ok := a.Library().Note(core.ID(a.notes.Loaded))
	if !ok || note.Title != today {
		t.Fatalf("the editor holds %q, want today's note", a.notes.Loaded)
	}
	notes := 0
	for _, n := range a.Library().Notes() {
		if n.Title == today {
			notes++
		}
	}
	if notes != 1 {
		t.Fatalf("the library holds %d notes for today, want 1", notes)
	}

	// It is a note like any other: typed into, it reaches the file.
	if err := tt.Click("Note"); err != nil {
		t.Fatal(err)
	}
	tt.Type("a thought for today")
	tt.Advance(320 * time.Millisecond)
	files, err := os.ReadDir(filepath.Join(seed.dir, "notes"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range files {
		if strings.Contains(file.Name(), today) {
			source, err := os.ReadFile(filepath.Join(seed.dir, "notes", file.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(source), "a thought for today") {
				found = true
			}
		}
	}
	if !found {
		t.Error("what was typed into today's note did not reach a file named for the day")
	}

	// Asking again opens the same note, not a second one.
	tt.Key(ui.Cmd, ui.KeyD)
	tt.Advance(320 * time.Millisecond)
	if got := len(a.Library().Notes()); got != 3+1 {
		t.Errorf("the library holds %d notes, want the seeded three and today's", got)
	}
}

// The editor keeps a formula's source; setting it is the review screen's job,
// which its own package tests.
func TestFormulaStaysSourceInTheEditor(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.Library().SaveNote(core.Note{Title: "Formula", Blocks: []core.Block{
		{Text: `the area of a circle :: $\pi r^2$`},
	}}); err != nil {
		t.Fatal(err)
	}
	a.reload()

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	notesMode(t, a, tt)
	openNote(t, tt, "Formula")
	if a.notes.Doc == nil || !strings.Contains(a.notes.Doc.Text().String(), `$\pi r^2$`) {
		t.Fatalf("the editor lost the source; texts %q", tt.Texts())
	}
}
