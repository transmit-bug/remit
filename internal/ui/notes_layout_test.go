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

// notesOnTheNotebook opens the notebook on the seeded library and returns the
// tester.
func notesOnTheNotebook(t *testing.T, w, h int) (*App, *ui.Tester, testLibrary) {
	t.Helper()
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, w, h)
	settle(tt)
	notesMode(t, a, tt)
	return a, tt, seed
}

// The default window shows three columns: the notes at 260, the note, and the
// note's panel at 280.
func TestThreeColumnsAtTheDefaultSize(t *testing.T) {
	_, tt, _ := notesOnTheNotebook(t, 1024, 720)

	notes, ok := tt.Find("Notebook")
	if !ok {
		t.Fatalf("the notes column is missing; texts %q", tt.Texts())
	}
	if notes.X < 0 || notes.X+notes.W > sidebarWidth+1 {
		t.Errorf("the notes column reaches %v, want it within %v", notes.X+notes.W, sidebarWidth)
	}
	panel, ok := tt.Find(locales.In("en").Close())
	if !ok {
		t.Fatalf("the panel is missing; texts %q", tt.Texts())
	}
	if panel.X < 1024-panelWidth-1 {
		t.Errorf("the panel starts at %v, want it at the right %v", panel.X, panelWidth)
	}
	// The note itself is between them, and typeable.
	if err := tt.Click("Note"); err != nil {
		t.Fatalf("the document is not typeable: %v", err)
	}
}

// The smallest window puts the panel away and narrows the notes, and the note is
// still typeable.
func TestNarrowWindowDropsThePanel(t *testing.T) {
	_, tt, seed := notesOnTheNotebook(t, 720, 520)

	if _, ok := tt.Find(locales.In("en").Close()); ok {
		t.Error("the panel is open on the smallest window")
	}
	notes, ok := tt.Find("Notebook")
	if !ok {
		t.Fatalf("the notes column is missing; texts %q", tt.Texts())
	}
	if notes.X+notes.W > sidebarNarrow+1 {
		t.Errorf("the notes column reaches %v, want it within %v", notes.X+notes.W, sidebarNarrow)
	}

	// The document keeps its room: typing reaches the file.
	openNote(t, tt, "Memory and recall")
	if err := tt.Click("Note"); err != nil {
		t.Fatalf("the document is not typeable: %v", err)
	}
	tt.Type("Still writable. ")
	tt.Advance(320 * time.Millisecond)
	source, err := os.ReadFile(filepath.Join(seed.dir, "notes", "memory-and-recall-"+string(seed.memory.ID)+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "Still writable.") {
		t.Errorf("the edit did not reach the file:\n%s", source)
	}
}

// The notes column starts with today and what is due, then every note newest
// first.
func TestTodayAndNewestFirst(t *testing.T) {
	msgs := locales.In("en")
	a, tt, seed := notesOnTheNotebook(t, 1024, 720)

	today, ok := tt.Find(msgs.Today())
	if !ok {
		t.Fatalf("the notes column has no today row; texts %q", tt.Texts())
	}
	if !tt.HasText("4") {
		t.Errorf("today does not show what is due; texts %q", tt.Texts())
	}

	// Saving a note moves it to the top of the list.
	openNote(t, tt, "A daily habit")
	if err := tt.Click("Note"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Edited. ")
	tt.Advance(320 * time.Millisecond)

	habit, ok := tt.Find("A daily habit")
	if !ok {
		t.Fatalf("the edited note left the list; texts %q", tt.Texts())
	}
	if habit.Y < today.Y {
		t.Error("the edited note is above today's row")
	}
	for _, other := range []string{"Memory and recall", "Writing cards"} {
		box, ok := tt.Find(other)
		if !ok {
			t.Fatalf("%q left the list; texts %q", other, tt.Texts())
		}
		if habit.Y > box.Y {
			t.Errorf("the edited note sits below %q, want it newest first", other)
		}
	}
	if a.editor.loaded != string(seed.habit.ID) {
		t.Errorf("the editor holds %q, want the edited note", a.editor.loaded)
	}
}

// The page no longer counts blocks: a note says what its cards are, not how many
// paragraphs it has.
func TestThePageCountsCardsNotBlocks(t *testing.T) {
	_, tt, _ := notesOnTheNotebook(t, 1024, 720)
	for _, text := range tt.Texts() {
		if strings.Contains(text, "blocks") {
			t.Errorf("the page still counts blocks: %q", text)
		}
	}
}
