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

// The default window shows three panes: the notes, the note, and the note's
// panel, with the note between the other two.
func TestThreeColumnsAtTheDefaultSize(t *testing.T) {
	msgs := locales.In("en")
	a, tt, _ := notesOnTheNotebook(t, 1024, 720)

	notes, ok := tt.Find("Notebook")
	if !ok {
		t.Fatalf("the notes column is missing; texts %q", tt.Texts())
	}
	if notes.X < 0 || notes.X+notes.W > a.notesWidth+1 {
		t.Errorf("the notes column reaches %v, want it within %v", notes.X+notes.W, a.notesWidth)
	}
	doc, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the document is missing; texts %q", tt.Texts())
	}
	panel, ok := tt.Find(msgs.Close())
	if !ok {
		t.Fatalf("the panel is missing; texts %q", tt.Texts())
	}
	if panel.X < doc.X+doc.W {
		t.Errorf("the panel starts at %v, left of the document ending at %v", panel.X, doc.X+doc.W)
	}
	if panel.X+panel.W > 1024 {
		t.Errorf("the panel reaches %v, past the window", panel.X+panel.W)
	}
	// The note itself is between them, and typeable.
	if err := tt.Click("Note"); err != nil {
		t.Fatalf("the document is not typeable: %v", err)
	}
}

// The notes column is a split's pane: the divider resizes it, and the width the
// user chose is kept when the window changes. A hand-built column could not
// pass this.
func TestSplitResizesTheNotesColumn(t *testing.T) {
	a, tt, _ := notesOnTheNotebook(t, 1024, 720)

	drag := a.notesWidth
	tt.Press(drag, 300)
	tt.Move(drag+30, 300)
	tt.Move(drag+80, 300)
	tt.Release(drag+80, 300)
	tt.Frame()

	if a.notesWidth < drag+40 {
		t.Fatalf("the notes column is %v wide after dragging to %v, want it resized", a.notesWidth, drag+80)
	}
	notes, ok := tt.Find("Notebook")
	if !ok {
		t.Fatalf("the notes column is missing; texts %q", tt.Texts())
	}
	if notes.X+notes.W > a.notesWidth+1 {
		t.Errorf("the notes column reaches %v, want it within %v", notes.X+notes.W, a.notesWidth)
	}

	// The size is the user's: a window change keeps it.
	chosen := a.notesWidth
	tt.SetSize(1400, 900)
	tt.Frame()
	if a.notesWidth != chosen {
		t.Errorf("the width moved from %v to %v when the window grew", chosen, a.notesWidth)
	}
}

// The smallest window puts the panel away, and the note keeps room to be typed.
func TestNarrowWindowDropsThePanel(t *testing.T) {
	msgs := locales.In("en")
	a, tt, seed := notesOnTheNotebook(t, 720, 520)

	if _, ok := tt.Find(msgs.Close()); ok {
		t.Error("the panel is open on the smallest window")
	}
	notes, ok := tt.Find("Notebook")
	if !ok {
		t.Fatalf("the notes column is missing; texts %q", tt.Texts())
	}
	if notes.X+notes.W > a.notesWidth+1 {
		t.Errorf("the notes column reaches %v, want it within %v", notes.X+notes.W, a.notesWidth)
	}
	doc, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the document is missing; texts %q", tt.Texts())
	}
	if doc.W < 300 {
		t.Errorf("the document is %v wide, too narrow to type in", doc.W)
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

// The notebook is elastic: the document and the notes tree take the height the
// window gives them, not the height their content happens to have. A window
// taller than the content used to leave both columns stopping halfway down.
func TestNotebookFillsTheWindowHeight(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range []struct {
		name string
		w, h int
	}{
		{"default", 1024, 720},
		{"tall", 1024, 1400},
		{"wide", 1600, 1000},
	} {
		t.Run(size.name, func(t *testing.T) {
			_, tt, _ := notesOnTheNotebook(t, size.w, size.h)
			assertNotebookFills(t, tt, msgs, size.w, size.h)
		})
	}

	t.Run("resized", func(t *testing.T) {
		_, tt, _ := notesOnTheNotebook(t, 1024, 720)
		tt.SetSize(1600, 1000)
		tt.Frame()
		assertNotebookFills(t, tt, msgs, 1600, 1000)
	})
}

// assertNotebookFills checks the document and the notes tree reach the window's
// bottom edge, allowing for the document's padding.
func assertNotebookFills(t *testing.T, tt *ui.Tester, msgs locales.Messages, w, h int) {
	t.Helper()
	note, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the document is missing; texts %q", tt.Texts())
	}
	// Under the document are the status line and the paper's own padding: at
	// most 96 DIPs from the window's bottom edge.
	if bottom := note.Y + note.H; bottom < float32(h)-96 {
		t.Errorf("the document ends at %v, want it to take the %v-high window it is given", bottom, h)
	}
	if note.X+note.W > float32(w) {
		t.Errorf("the document reaches %v, past the %v-wide window", note.X+note.W, w)
	}

	tree, ok := tt.AccessByRole(ui.RoleTree, "")
	if !ok {
		t.Fatalf("the notes tree is missing; texts %q", tt.Texts())
	}
	if bottom := tree.Bounds.Y + tree.Bounds.H; bottom < float32(h)-1 {
		t.Errorf("the notes tree ends at %v, want the %v-high window's bottom", bottom, h)
	}
}
