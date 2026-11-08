package notes_test

import (
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// Opening, editing around and saving a note keeps a GFM table byte-for-byte
// and leaves every block ID where it was. The editor shows the table as an
// opaque read-only block instead of flattening its rows into prose.
func TestSavingKeepsMarkdownTableAndBlockIDs(t *testing.T) {
	table := "| Name | Size |\n| :--- | ---: |\n| alpha | 1 |"
	f := newFake()
	f.library = []notes.Note{{Note: core.Note{ID: "note-1", Title: "With a table", Blocks: []core.Block{
		{ID: "block-before", Text: "Before"},
		{ID: "block-table", Text: table},
		{ID: "block-after", Text: "After"},
	}}}}
	s, tt := at(t, f, 1024, 720)

	if err := tt.Click(locales.In("en").NoteBody()); err != nil {
		t.Fatal(err)
	}
	s.Doc.SetSelection(0, 0)
	tt.Frame()
	tt.Type("Edited ")
	tt.Frame()

	if len(f.documents) == 0 {
		t.Fatal("the edit was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	if len(saved.Blocks) != 3 {
		t.Fatalf("saved %d blocks, want 3: %q", len(saved.Blocks), blockTexts(saved.Blocks))
	}
	if got := string(saved.Blocks[1].ID); got != "block-table" {
		t.Errorf("the table has id %q, want block-table", got)
	}
	if got := saved.Blocks[1].Text; got != table {
		t.Errorf("the table was rewritten:\n got %q\nwant %q", got, table)
	}
	if got := string(saved.Blocks[2].ID); got != "block-after" {
		t.Errorf("the block after the table has id %q, want block-after", got)
	}
}

// Table-like text in a fenced code block is code: opening and saving it keeps
// the fence and the lines exactly as they were.
func TestSavingKeepsTableLikeCode(t *testing.T) {
	code := "```\n| Name | Size |\n| --- | --- |\n```"
	f := newFake()
	f.library = []notes.Note{{Note: core.Note{ID: "note-1", Title: "Code", Blocks: []core.Block{
		{ID: "block-before", Text: "Before"},
		{ID: "block-code", Text: code},
	}}}}
	s, tt := at(t, f, 1024, 720)

	if err := tt.Click(locales.In("en").NoteBody()); err != nil {
		t.Fatal(err)
	}
	s.Doc.SetSelection(6, 6)
	tt.Frame()
	tt.Type("!")
	tt.Frame()
	if len(f.documents) == 0 {
		t.Fatal("the edit was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	if got := saved.Blocks[1].Text; got != code {
		t.Errorf("the fenced table changed:\n got %q\nwant %q", got, code)
	}
}
