package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/core/transfer"
	"github.com/transmit-bug/remit/locales"
)

// exportDialog opens the export dialog the way a user reaches it: the palette's
// export command, found by typing its name.
func exportDialog(t *testing.T, tt *ui.Tester, msgs locales.Messages) {
	t.Helper()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type(msgs.ExportTitle())
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
}

// The export dialog writes what it offers and says what it wrote: a folder of
// Markdown files for the whole library, one per note, with the report on the
// window where the action was.
func TestExportWritesAndReports(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	out := filepath.Join(t.TempDir(), "md")

	folders := 0
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		folders++
		return out, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)
	if !tt.HasText(msgs.ExportTitle()) {
		t.Fatalf("the export dialog did not open; texts %q", tt.Texts())
	}
	for _, format := range []string{msgs.ExportMarkdown(), msgs.ExportArchive(), msgs.ExportCards()} {
		if !tt.HasText(format) {
			t.Errorf("the dialog offers no %q format; texts %q", format, tt.Texts())
		}
	}

	if err := tt.Click(msgs.ExportRun()); err != nil {
		t.Fatalf("the dialog has no export button: %v", err)
	}
	tt.Frame()

	if folders != 1 {
		t.Fatalf("the folder picker ran %d times", folders)
	}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != 3 {
		t.Fatalf("the export wrote %d files (%v), want 3", len(entries), err)
	}
	if !tt.HasText(msgs.ExportReport(3, 0)) {
		t.Errorf("the export is not reported; texts %q", tt.Texts())
	}
	if _, open := tt.Find(msgs.ExportTitle()); open {
		t.Error("the dialog stayed open after the export")
	}
	tt.Advance(5 * time.Second) // past the toast
	if tt.FramePending() {
		t.Error("view requests frames after the window should settle")
	}
}

// The library archive is one file the library reads back: transfer.Write packs
// it, and transfer.Read unpacks the same sources an import would open.
func TestExportWritesTheLibraryArchive(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	out := t.TempDir()
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		return out, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)
	if err := tt.Click(msgs.ExportArchive()); err != nil {
		t.Fatalf("the archive format is not clickable: %v", err)
	}
	tt.Frame()
	if err := tt.Click(msgs.ExportRun()); err != nil {
		t.Fatalf("the dialog has no export button: %v", err)
	}
	tt.Frame()

	name := archiveName(dir)
	path := filepath.Join(out, name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("the export wrote no archive: %v", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	unpacked := filepath.Join(t.TempDir(), "unpacked")
	if err := transfer.Read(file, info.Size(), unpacked); err != nil {
		t.Fatalf("the archive does not read back: %v", err)
	}
	if _, err := os.Stat(filepath.Join(unpacked, "remit.json")); err != nil {
		t.Errorf("the archive holds no library: %v", err)
	}
	if !tt.HasText(msgs.ExportArchiveReport(name)) {
		t.Errorf("the archive is not reported; texts %q", tt.Texts())
	}
}

// Card CSV is not written, and the dialog says so beside the choice rather than
// leaving a control that looks like the others and does nothing.
func TestExportRefusesTheCSVItDoesNotHave(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	asked := 0
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		asked++
		return t.TempDir(), nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)

	if !tt.HasText(msgs.ExportNoCSV()) {
		t.Fatalf("the CSV is not refused in words; texts %q", tt.Texts())
	}
	info, ok := tt.Access(msgs.ExportCards())
	if !ok {
		t.Fatalf("the CSV choice is not on the window; texts %q", tt.Texts())
	}
	if !info.Disabled {
		t.Error("the CSV choice is not disabled")
	}
	// A disabled radio takes no click; if the click lands anywhere, the choice
	// must not move.
	_ = tt.Click(msgs.ExportCards())
	tt.Frame()
	if a.export.format != exportMarkdown {
		t.Errorf("a disabled choice was chosen: %v", a.export.format)
	}
	if asked != 0 {
		t.Errorf("the folder picker ran for a refused format")
	}
}

// An empty library offers nothing to export, and both formats say why.
func TestExportDisabledOnAnEmptyLibrary(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create an empty library: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close the empty library: %v", err)
	}
	a := NewApp(Options{LibraryDir: dir})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)
	if !tt.HasText(msgs.ExportEmptyLibrary()) {
		t.Fatalf("the dialog does not say why it is disabled; texts %q", tt.Texts())
	}
	info, ok := tt.Access(msgs.ExportMarkdown())
	if !ok {
		t.Fatalf("the Markdown choice is not on the window; texts %q", tt.Texts())
	}
	if !info.Disabled {
		t.Error("the Markdown choice is live on an empty library")
	}
}
