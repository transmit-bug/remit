package importmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/importmd"
	"github.com/transmit-bug/remit/internal/core/store"
)

func newLibrary(t *testing.T) (*store.Library, string) {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib, dir
}

func write(t *testing.T, path, src string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func noteByText(t *testing.T, lib *store.Library, part string) core.Note {
	t.Helper()
	for _, note := range lib.Notes() {
		for _, block := range note.Blocks {
			if strings.Contains(block.Text, part) {
				return note
			}
		}
	}
	t.Fatalf("no note mentions %q", part)
	return core.Note{}
}

// A plain Markdown file — no markers, foreign shape — becomes one note whose
// first H1 is the title and whose paragraphs are blocks with minted IDs.
func TestImportPlainMarkdown(t *testing.T) {
	lib, _ := newLibrary(t)
	path := write(t, filepath.Join(t.TempDir(), "chapters", "01-intro.md"),
		"# The Introduction\n\nFirst paragraph, with **bold**.\n\n- one\n- two\n\n```go\ncode()\n```\n")

	report, err := importmd.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 1 || report.Skipped != 0 {
		t.Fatalf("report = %+v, want 1 note, 0 skipped", report)
	}
	note := noteByText(t, lib, "First paragraph")
	if note.Title != "The Introduction" {
		t.Errorf("title = %q, want the H1", note.Title)
	}
	if len(note.Blocks) != 3 {
		t.Fatalf("blocks = %d, want paragraph, list, code", len(note.Blocks))
	}
	for _, block := range note.Blocks {
		if block.ID == "" || block.ID.Kind() != core.BlockKind {
			t.Errorf("block %q carries no minted ID", block.Text)
		}
	}
	if got := note.Blocks[0].Text; got != "First paragraph, with **bold**." {
		t.Errorf("first block = %q", got)
	}
	if got := note.Blocks[2].Text; !strings.Contains(got, "code()") {
		t.Errorf("code block = %q", got)
	}
}

// A file without an H1 takes the file's name as its title.
func TestImportTakesFileNameWithoutH1(t *testing.T) {
	lib, _ := newLibrary(t)
	path := write(t, filepath.Join(t.TempDir(), "grocery list.md"), "Just one line.\n")

	if _, err := importmd.Import(path, lib); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if note := noteByText(t, lib, "Just one line"); note.Title != "grocery list" {
		t.Errorf("title = %q, want the file name", note.Title)
	}
}

// The exported file imports back as the same note: identity is carried by the
// markers, the second run writes nothing, and references to the note's blocks
// still resolve.
func TestRoundTripReachesTheSameNote(t *testing.T) {
	lib, _ := newLibrary(t)
	source, err := lib.CreateNote("Round Trip")
	if err != nil {
		t.Fatal(err)
	}
	source.Blocks = []core.Block{
		{ID: core.NewID(core.BlockKind), Text: "One opening claim."},
		{ID: core.NewID(core.BlockKind), Text: "Second: with a separator"},
	}
	if _, err := lib.SaveNote(source); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "md")
	exported, err := importmd.Export(out, lib)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if exported.Files != 1 || exported.Skipped != 0 {
		t.Fatalf("export = %+v, want 1 file", exported)
	}

	// A reference like an excerpt's must survive the trip: the exported file
	// carries the marker the reference names.
	src, err := os.ReadFile(filepath.Join(out, "round-trip-"+string(source.ID)+".md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range source.Blocks {
		if !strings.Contains(string(src), " "+core.BlockMarker(block.ID)) {
			t.Errorf("exported source lacks marker %s: %q", block.ID, src)
		}
	}

	report, err := importmd.Import(out, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 0 || report.Skipped != 1 {
		t.Fatalf("reimport = %+v, want the same note found, nothing written", report)
	}
	back, ok := lib.Note(source.ID)
	if !ok {
		t.Fatal("the note vanished")
	}
	if len(back.Blocks) != 2 || back.Blocks[0].ID != source.Blocks[0].ID || back.Blocks[1].Text != source.Blocks[1].Text {
		t.Errorf("round trip changed the note: %+v", back.Blocks)
	}
}

// A marked file whose note the library does not know yet imports as a new
// note, keeping the markers it came with: an exchange between two libraries
// keeps identity.
func TestImportAdoptsMarkedFile(t *testing.T) {
	lib, _ := newLibrary(t)
	other, otherDir := newLibrary(t)
	made, err := other.CreateNote("From Elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	made.Blocks = []core.Block{{ID: core.NewID(core.BlockKind), Text: "Written elsewhere."}}
	saved, err := other.SaveNote(made)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "md")
	if _, err := importmd.Export(out, other); err != nil {
		t.Fatal(err)
	}
	_ = otherDir

	report, err := importmd.Import(out, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 1 {
		t.Fatalf("report = %+v, want 1 note", report)
	}
	note := noteByText(t, lib, "Written elsewhere")
	if note.ID == saved.ID {
		t.Error("the two libraries minted the same note ID")
	}
	if note.Blocks[0].ID != made.Blocks[0].ID {
		t.Errorf("block ID = %q, want the file's %q", note.Blocks[0].ID, made.Blocks[0].ID)
	}
}

// A titled file whose text matches what the library's same-titled note holds
// is skipped, not duplicated; a changed text updates that note.
func TestImportMatchesTitledNoteByText(t *testing.T) {
	lib, _ := newLibrary(t)
	made, err := lib.SaveNote(core.Note{Title: "Journal", Blocks: []core.Block{
		{ID: core.NewID(core.BlockKind), Text: "The day was long."},
	}})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	same := write(t, filepath.Join(dir, "journal.md"), "# Journal\n\nThe day was long.\n")

	report, err := importmd.Import(same, lib)
	if err != nil {
		t.Fatal(err)
	}
	if report.Notes != 0 || report.Skipped != 1 {
		t.Fatalf("report = %+v, want a skip", report)
	}
	if got := len(lib.Notes()); got != 1 {
		t.Fatalf("notes = %d, want the original only", got)
	}

	changed := write(t, filepath.Join(dir, "journal.md"), "# Journal\n\nThe day was short.\n")
	report, err = importmd.Import(changed, lib)
	if err != nil {
		t.Fatal(err)
	}
	if report.Notes != 1 {
		t.Fatalf("changed report = %+v, want 1 note written", report)
	}
	note := noteByText(t, lib, "The day was short")
	if note.ID != made.ID {
		t.Errorf("changed note ID = %s, want the original %s", note.ID, made.ID)
	}
	if len(note.Blocks) != 1 || note.Blocks[0].Text != "The day was short." {
		t.Errorf("blocks = %+v, want the file's text", note.Blocks)
	}
}

// A directory imports recursively and in name order.
func TestImportDirectory(t *testing.T) {
	lib, _ := newLibrary(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "b-second.md"), "# Second\n\nb.\n")
	write(t, filepath.Join(root, "nested", "a-first.md"), "# First\n\na.\n")
	write(t, filepath.Join(root, "not-markdown.txt"), "# not imported\n")

	report, err := importmd.Import(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	if report.Notes != 2 {
		t.Fatalf("report = %+v, want 2 notes", report)
	}
	if note := noteByText(t, lib, "a."); note.Title != "First" {
		t.Errorf("title = %q", note.Title)
	}
}

// A note with neither title nor text exports as nothing and is counted.
func TestExportSkipsEmptyNotes(t *testing.T) {
	lib, dir := newLibrary(t)
	if _, err := lib.CreateNote("   "); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "md")
	report, err := importmd.Export(out, lib)
	if err != nil {
		t.Fatal(err)
	}
	if report.Files != 0 || report.Skipped != 1 {
		t.Fatalf("report = %+v, want one skip", report)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("wrote %d files, want none", len(entries))
	}
	_ = dir
}

// ExportNote writes one note and answers with its path; a missing note is an
// error, not a skip.
func TestExportNote(t *testing.T) {
	lib, _ := newLibrary(t)
	note, err := lib.CreateNote("Solo")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "md")
	path, wrote, err := importmd.ExportNote(out, note.ID, lib)
	if err != nil || !wrote {
		t.Fatalf("ExportNote = %q, %v, want a file", path, err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(src), "# Solo\n") {
		t.Errorf("source = %q, want the title line", src)
	}
	if _, _, err := importmd.ExportNote(out, "note_00000000000000000000000000000000", lib); err == nil {
		t.Error("a missing note exported as nothing")
	}
}

// Import refuses what is not Markdown, the way a wrong extension fails early.
func TestImportRefusesNonMarkdown(t *testing.T) {
	lib, _ := newLibrary(t)
	path := write(t, filepath.Join(t.TempDir(), "notes.rtf"), "rich text")
	if _, err := importmd.Import(path, lib); err == nil {
		t.Fatal("imported a non-Markdown file")
	}
}

func TestNilLibrary(t *testing.T) {
	if _, err := importmd.Import(t.TempDir(), nil); err != importmd.ErrNoLibrary {
		t.Errorf("Import err = %v, want ErrNoLibrary", err)
	}
	if _, err := importmd.Export(t.TempDir(), nil); err != importmd.ErrNoLibrary {
		t.Errorf("Export err = %v, want ErrNoLibrary", err)
	}
}
