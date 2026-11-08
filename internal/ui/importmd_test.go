package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/importmd"
)

// The palette's Markdown import brings plain files in as notes: the picker
// runs, the library grows, the window reloads onto what arrived, and the
// report counts it.
func TestImportMarkdownFromThePalette(t *testing.T) {
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	file := filepath.Join(t.TempDir(), "imported.md")
	if err := os.WriteFile(file, []byte("# Imported\n\nA fresh paragraph.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	picked := 0
	a := NewApp(Options{LibraryDir: dir, ChooseMarkdown: func() ([]string, error) {
		picked++
		return []string{file}, nil
	}})
	notes := len(a.Library().Notes())

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("markdown")
	tt.Key(0, ui.KeyEnter)

	if picked != 1 {
		t.Fatalf("the file picker ran %d times", picked)
	}
	if got := len(a.Library().Notes()); got != notes+1 {
		t.Fatalf("the library holds %d notes, want %d", got, notes+1)
	}
	if !tt.HasText("Imported 1 notes, skipped 0.") {
		t.Fatalf("the report is not on the window; texts %q", tt.Texts())
	}
	found := false
	for _, note := range a.Library().Notes() {
		if note.Title == "Imported" {
			found = true
		}
	}
	if !found {
		t.Fatal("the imported note is not in the library")
	}
	_ = seed
	tt.Advance(5 * time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A canceled Markdown import writes nothing and reports nothing.
func TestImportMarkdownCanceled(t *testing.T) {
	dir := t.TempDir()
	seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir, ChooseMarkdown: func() ([]string, error) { return nil, nil }})
	notes := len(a.Library().Notes())

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("markdown")
	tt.Key(0, ui.KeyEnter)

	if got := len(a.Library().Notes()); got != notes {
		t.Errorf("a canceled import wrote %d notes", got-notes)
	}
	if tt.HasText("Imported") {
		t.Errorf("a canceled import reported one; texts %q", tt.Texts())
	}
}

// A file that will not import says why where the action was.
func TestImportMarkdownFailure(t *testing.T) {
	dir := t.TempDir()
	seedLibrary(t, dir)
	a := NewApp(Options{
		LibraryDir:     dir,
		ChooseMarkdown: func() ([]string, error) { return []string{"/tmp/broken.md"}, nil },
		ImportMarkdown: func(string) (importmd.Report, error) {
			return importmd.Report{}, os.ErrPermission
		},
	})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("markdown")
	tt.Key(0, ui.KeyEnter)

	if !tt.HasText("The import failed") {
		t.Fatalf("the failure is not on the window; texts %q", tt.Texts())
	}
}

// Exporting the open note writes its file with the marker a card points at,
// and the report counts one file.
func TestExportOpenNote(t *testing.T) {
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	out := filepath.Join(t.TempDir(), "md")

	folders := 0
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		folders++
		return out, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	// Open the writing note from the notebook's list.
	tt.Click("Writing cards")
	tt.Advance(320 * time.Millisecond)
	if a.notes.Doc == nil || !strings.Contains(a.notes.Doc.Text().String(), "A good card asks") {
		t.Fatalf("the note did not open; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("export note")
	tt.Key(0, ui.KeyEnter)

	if folders != 1 {
		t.Fatalf("the folder picker ran %d times", folders)
	}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != 1 {
		t.Fatalf("the export wrote %d files (%v)", len(entries), err)
	}
	src, err := os.ReadFile(filepath.Join(out, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(src), "# Writing cards\n") {
		t.Errorf("the file does not start with the title: %q", src)
	}
	if !strings.Contains(string(src), " ^block_") {
		t.Errorf("the file carries no block marker: %q", src)
	}
	_ = seed
	if !tt.HasText("Exported 1 files, skipped 0.") {
		t.Errorf("the report is not on the window; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second)
}

// Exporting with no note open says so instead of asking for a folder.
func TestExportNoteWithoutANote(t *testing.T) {
	dir := t.TempDir()
	seedLibrary(t, dir)
	asked := 0
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		asked++
		return t.TempDir(), nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	// Stay in review: the editor holds no note yet.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("export note")
	tt.Key(0, ui.KeyEnter)

	if asked != 0 {
		t.Error("the folder picker ran with no note to export")
	}
	if !tt.HasText("Open a note first") {
		t.Errorf("the nudge is not on the window; texts %q", tt.Texts())
	}
}

// Exporting the library writes one file per note, and a second export of an
// unchanged library is the same files.
func TestExportAllNotes(t *testing.T) {
	dir := t.TempDir()
	seedLibrary(t, dir)
	out := filepath.Join(t.TempDir(), "md")

	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		return out, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("export all")
	tt.Key(0, ui.KeyEnter)

	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("the export wrote %d files, want 3", len(entries))
	}
	if !tt.HasText("Exported 3 files, skipped 0.") {
		t.Errorf("the report is not on the window; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second)
}
