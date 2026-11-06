package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"

	"github.com/transmit-bug/remit/internal/settings"
)

// notesMode opens the editor the way the user does, from the review queue the
// window opens on.
func notesMode(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
}

// openNote chooses a note from the sidebar, which is a title no field holds
// while another note is up.
func openNote(t *testing.T, tt *ui.Tester, title string) {
	t.Helper()
	if err := tt.Click(title); err != nil {
		t.Fatalf("choose note %q: %v", title, err)
	}
}

func TestEditNoteTitle(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesMode(t, a, tt)

	if err := tt.Click("Note title"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("A better habit")

	note, ok := a.Library().Note(seed.habit.ID)
	if !ok {
		t.Fatal("the note is gone")
	}
	if note.Title != "A better habit" {
		t.Fatalf("the store holds the title %q; wants %q", note.Title, "A better habit")
	}
	if !tt.HasText("A better habit") {
		t.Fatalf("the sidebar does not show the new title; texts %q", tt.Texts())
	}
	if tt.HasText("A daily habit") {
		t.Errorf("the old title is still on screen; texts %q", tt.Texts())
	}
	// A renamed note keeps its file: the slug is minted once.
	if _, err := os.Stat(filepath.Join(seed.dir, "notes", "a-daily-habit-"+string(seed.habit.ID)+".md")); err != nil {
		t.Errorf("the note file moved with its title: %v", err)
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

func TestSettingsKeepPreferences(t *testing.T) {
	data := t.TempDir()
	if err := settings.Save(data, settings.Settings{UpdateCheck: true, AgentServer: true}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "Remit")
	a := NewApp(Options{LibraryDir: dir, DataDir: data})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	if err := tt.Click("Create library"); err != nil {
		t.Fatal(err)
	}
	s, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.LibraryDir != dir {
		t.Errorf("the settings name %q, want %q", s.LibraryDir, dir)
	}
	if !s.UpdateCheck || !s.AgentServer {
		t.Errorf("remembering the library dropped a preference: %+v", s)
	}
}

// A note written with a heading, a list, code and an image keeps its shape and
// every marker across a save and a reopen.
func TestDocumentRoundTripsRichText(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	notesMode(t, a, tt)
	openNote(t, tt, "Writing cards")

	if err := tt.Click("Note"); err != nil {
		t.Fatalf("open the document: %v", err)
	}
	tt.Type("\n\n## Heading\n\n- item one\n- item two\n\n```\ncode line\n```\n\n![alt](img.png)")
	tt.Advance(320 * time.Millisecond)

	source, err := os.ReadFile(filepath.Join(seed.dir, "notes", "writing-cards-"+string(seed.writing.ID)+".md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Heading", "item one", "item two", "code line", "img.png"} {
		if !strings.Contains(string(source), want) {
			t.Errorf("the file lost %q:\n%s", want, source)
		}
	}
	for _, block := range seed.writing.Blocks {
		if !strings.Contains(string(source), core.BlockMarker(block.ID)) {
			t.Errorf("block %s lost its marker", block.ID)
		}
	}

	// Reopening the library shows the same document.
	next := NewApp(Options{LibraryDir: seed.dir})
	reopened := ui.NewTester(next.View, 1024, 720)
	settle(reopened)
	notesMode(t, next, reopened)
	openNote(t, reopened, "Writing cards")
	if next.editor.doc == nil {
		t.Fatal("the reopened note has no document")
	}
	text := next.editor.doc.Text().String()
	for _, want := range []string{"## Heading", "item one", "code line", "img.png"} {
		if !strings.Contains(text, want) {
			t.Errorf("the reopened document lost %q:\n%s", want, text)
		}
	}
}
