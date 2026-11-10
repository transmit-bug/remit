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
	if !tt.HasText(msgs.ImportMdReport(1, 0, 0)) {
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

// A file whose first H1 names a note the library already holds is read and
// counted, not written over: the keep switch is on when the wizard opens, and
// the completion says how many notes it left as they were.
func TestImportMarkdownKeepsExistingNotes(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	file := filepath.Join(t.TempDir(), "memory.md")
	if err := os.WriteFile(file, []byte("# Memory and recall\n\nA paragraph the note does not hold.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewApp(Options{LibraryDir: dir, ChooseMarkdown: func() ([]string, error) {
		return []string{file}, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromMarkdown()); err != nil {
		t.Fatalf("the Markdown source is not clickable: %v", err)
	}
	tt.Frame()

	// The preview says what the run will do with the notes already here.
	if !tt.HasText(msgs.ImportRowExisting()) || !tt.HasText(msgs.ImportExistingKept()) {
		t.Fatalf("the preview does not say existing notes are kept; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.ImportKeepHint()) {
		t.Errorf("the keep switch has no hint; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.ImportRun()); err != nil {
		t.Fatalf("the preview has no import button: %v", err)
	}
	tt.Frame()
	if !tt.HasText(msgs.ImportMdReport(0, 1, 0)) {
		t.Fatalf("the run does not report the kept note; texts %q", tt.Texts())
	}
	note, ok := a.Library().Note(seed.memory.ID)
	if !ok {
		t.Fatalf("the note is gone")
	}
	if len(note.Blocks) != 3 || note.Blocks[0].Text != "Spacing beats cramming :: reviewing just before you forget strengthens recall." {
		t.Errorf("the kept note was written over: %+v", note.Blocks)
	}
}

// With the keep switch off the file is the note's new saying: the note keeps its
// identity and takes the file's text, and the run counts the write.
func TestImportMarkdownOverwritesWhenKeepingIsOff(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	file := filepath.Join(t.TempDir(), "memory.md")
	if err := os.WriteFile(file, []byte("# Memory and recall\n\nA paragraph the note does not hold.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewApp(Options{LibraryDir: dir, ChooseMarkdown: func() ([]string, error) {
		return []string{file}, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromMarkdown()); err != nil {
		t.Fatalf("the Markdown source is not clickable: %v", err)
	}
	tt.Frame()
	flipSwitch(t, tt, msgs.ImportKeep())
	tt.Frame()
	if !tt.HasText(msgs.ImportExistingOverwritten()) {
		t.Fatalf("the preview does not say existing notes are replaced; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.ImportRun()); err != nil {
		t.Fatalf("the preview has no import button: %v", err)
	}
	tt.Frame()
	if !tt.HasText(msgs.ImportMdReport(1, 0, 0)) {
		t.Fatalf("the run does not report the write; texts %q", tt.Texts())
	}
	note, ok := a.Library().Note(seed.memory.ID)
	if !ok {
		t.Fatalf("the note lost its identity: %q is gone", seed.memory.ID)
	}
	if len(note.Blocks) != 1 || note.Blocks[0].Text != "A paragraph the note does not hold." {
		t.Errorf("the note did not take the file's text: %+v", note.Blocks)
	}
}

// The wizard's second step is the same window at its smallest size: the keep
// switch and the hint under it stay inside, beside the two options the wizard
// already had.
func TestImportWizardKeepsItsOptionsAtTheMinimumSize(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	file := filepath.Join(t.TempDir(), "memory.md")
	if err := os.WriteFile(file, []byte("# Imported\n\nA fresh paragraph.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewApp(Options{LibraryDir: dir, ChooseMarkdown: func() ([]string, error) {
		return []string{file}, nil
	}})

	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromMarkdown()); err != nil {
		t.Fatalf("the Markdown source is not clickable: %v", err)
	}
	tt.Frame()
	for _, label := range []string{msgs.ImportKeep(), msgs.ImportRebuild(), msgs.ImportQueue()} {
		info, ok := tt.AccessByRole(ui.RoleSwitch, label)
		if !ok {
			t.Fatalf("no switch is named %q; texts %q", label, tt.Texts())
		}
		if info.Bounds.Y+info.Bounds.H > 520 || info.Bounds.X+info.Bounds.W > 720 {
			t.Errorf("the %q switch is outside the window: %+v", label, info.Bounds)
		}
	}
	assertControl(t, tt, 720, 520, msgs.ImportKeepHint())
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
		ImportMarkdown: func(string, importmd.Options) (importmd.Report, error) {
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
