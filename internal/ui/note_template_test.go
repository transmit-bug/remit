package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/locales"
)

// saveTemplate writes a note template into the library the app has open: a
// reading-excerpt shape with one marker block and one card-bearing paragraph.
func saveTemplate(t *testing.T, a *App) core.ID {
	t.Helper()
	template, err := a.lib.SaveNote(core.Note{Title: "Reading excerpt", Blocks: []core.Block{
		{Text: "#template"},
		{Text: "Chapter asks :: chapter answers"},
	}})
	if err != nil {
		t.Fatalf("save template: %v", err)
	}
	a.reload()
	return template.ID
}

// With a template in the library the notebook hides it from the list and offers
// it beside the new-note button; the copy it starts is in the list, holds the
// template's text under new IDs, and the template is still there for the next
// note.
func TestNewNoteFromTemplate(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	templateID := saveTemplate(t, a)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2) // notes

	if tt.HasText("Reading excerpt") {
		t.Fatalf("the template is listed like a document; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.NewFromTemplate()); err != nil {
		t.Fatalf("the template menu did not open: %v; texts %q", err, tt.Texts())
	}
	if err := tt.ChooseMenuItem("Reading excerpt"); err != nil {
		t.Fatalf("the template was not chosen: %v; items %q", err, tt.Menu())
	}
	tt.Advance(320 * time.Millisecond)

	// The copy: selected, holding the template's paragraph, in the list.
	copiedID, ok := a.data.byTitle["untitled note"]
	if !ok {
		t.Fatalf("no copy was written; notes %q", tt.Texts())
	}
	copied, ok := a.lib.Note(copiedID)
	if !ok {
		t.Fatal("the copy's ID does not resolve")
	}
	if len(copied.Blocks) != 1 {
		t.Fatalf("the copy holds %d blocks, want the template's one content block", len(copied.Blocks))
	}
	if copied.Blocks[0].Text != "Chapter asks :: chapter answers" {
		t.Fatalf("the copy's paragraph reads %q", copied.Blocks[0].Text)
	}
	if copied.Blocks[0].ID == "" {
		t.Fatal("the copy's paragraph has no ID of its own")
	}
	// The copy is on screen with its paragraph (the editor draws the card
	// marker, not the raw ::), and the file on disk holds the words.
	if !tt.HasText("Untitled note") {
		t.Fatalf("the copy is not in the list; texts %q", tt.Texts())
	}
	source, err := os.ReadFile(filepath.Join(a.dir, "notes", "untitled-note-"+string(copied.ID)+".md"))
	if err != nil {
		t.Fatalf("read the copy's file: %v", err)
	}
	if !strings.Contains(string(source), "Chapter asks :: chapter answers") {
		t.Fatalf("the copy's file lost the template's paragraph:\n%s", source)
	}
	if strings.Contains(string(source), core.BlockMarker(templateID)) {
		t.Errorf("the copy still points at the template's block")
	}
	if got := a.notes.Selected; got != string(copied.ID) {
		t.Fatalf("the selection is %q, want the copy %q", got, copied.ID)
	}

	// The template survives, still hidden from the list, still offered again.
	template, ok := a.data.note(templateID)
	if !ok || len(template.Blocks) != 2 {
		t.Fatalf("the template did not survive the copy")
	}
	if err := tt.Click(msgs.NewFromTemplate()); err != nil {
		t.Fatalf("the template menu did not open again: %v", err)
	}
	if err := tt.ChooseMenuItem("Reading excerpt"); err != nil {
		t.Fatalf("a second copy was refused: %v; items %q", err, tt.Menu())
	}
	tt.Advance(320 * time.Millisecond)
	copies, err := a.lib.SearchNotes("Chapter asks")
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 2 {
		t.Fatalf("searching the copy's text found %d notes, want the two copies", len(copies))
	}
}

// No templates, no door: the notebook row is the plain new-note button, and
// nothing about the search or the list changes.
func TestNoTemplatesNoMenu(t *testing.T) {
	msgs := locales.In("en")
	_, _, tt := openApp(t)
	tt.Key(ui.Cmd, ui.Key2)

	if _, ok := tt.Find(msgs.NewFromTemplate()); ok {
		t.Fatalf("the template door shows with no templates; texts %q", tt.Texts())
	}
	if !tt.HasText("Memory and recall") {
		t.Fatalf("the plain notebook is gone; texts %q", tt.Texts())
	}
}
