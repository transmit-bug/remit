package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/importmd"
	"github.com/transmit-bug/remit/locales"
)

// The wizard's Markdown source brings plain files in as notes: the picker runs,
// the library grows, and the closing page counts what arrived.
func TestImportMarkdownFromTheWizard(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
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
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromMarkdown()); err != nil {
		t.Fatalf("the Markdown source is not clickable: %v", err)
	}
	tt.Frame()
	if picked != 1 {
		t.Fatalf("the file picker ran %d times", picked)
	}
	if err := tt.Click(msgs.ImportRun()); err != nil {
		t.Fatalf("the preview has no import button: %v", err)
	}
	tt.Frame()

	if got := len(a.Library().Notes()); got != notes+1 {
		t.Fatalf("the library holds %d notes, want %d", got, notes+1)
	}
	if !tt.HasText(msgs.ImportMdReport(1, 0)) {
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
	tt.Advance(5 * time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A file that will not import says why on the step that asked for it, and the
// wizard stays open: a failure is not a reason to close the dialog.
func TestImportMarkdownFailure(t *testing.T) {
	msgs := locales.In("en")
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
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromMarkdown()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(msgs.ImportRun()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if !tt.HasText(os.ErrPermission.Error()) {
		t.Fatalf("the failure is not on the window; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.ImportTitle()) {
		t.Error("the wizard closed on a failure")
	}
}
