package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/locales"
)

// headedNote writes a note whose paragraphs open with headings, and opens it.
func headedNote(t *testing.T, w, h int) (*App, *ui.Tester, core.Note) {
	t.Helper()
	a, _ := newTestApp(t)
	note, err := a.Library().SaveNote(core.Note{Title: "Headed", Blocks: []core.Block{
		{Text: "# The first part"},
		{Text: "spacing :: the gaps"},
		{Text: "## The second part"},
		{Text: "testing :: recall"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt := ui.NewTester(a.View, w, h)
	settle(tt)
	notesMode(t, a, tt)
	openNote(t, tt, "Headed")
	return a, tt, note
}

// Choosing an outline entry puts the caret on that paragraph, which is what
// scrolls it into view.
func TestOutlineJumpsToItsParagraph(t *testing.T) {
	a, tt, _ := headedNote(t, 1024, 720)

	if err := tt.Click("The second part"); err != nil {
		t.Fatalf("the outline does not offer the heading: %v", err)
	}
	// The second heading is the note's third paragraph, and the caret is in it.
	if at, ok := a.notes.ParagraphAtCaret(); !ok || at != 2 {
		t.Errorf("the caret is in paragraph %d, want the heading's own", at+1)
	}
	if !tt.Focused("Note") {
		t.Error("the document did not take the focus after the jump")
	}
}

// The outline marks the heading the caret is under, so a long note says where
// the writer is. A hand-built list of buttons had no current row at all.
func TestOutlineFollowsTheCaret(t *testing.T) {
	a, tt, _ := headedNote(t, 1024, 720)

	// The editor opens with the caret at the end, under the second heading.
	if got := a.notes.OutlineCurrent; got != 1 {
		t.Fatalf("the outline marks row %d, want the second heading", got)
	}
	a.notes.Doc.SetSelection(0, 0)
	tt.Frame()
	if got := a.notes.OutlineCurrent; got != 0 {
		t.Errorf("the outline marks row %d after the caret moved to the top, want the first heading", got)
	}
}

// A backlink names the note it is in and the paragraph; choosing it opens that
// note there.
func TestBacklinkOpensTheSourceAtItsParagraph(t *testing.T) {
	msgs := locales.In("en")
	a, tt, _ := headedNote(t, 1024, 720)

	source, err := a.Library().SaveNote(core.Note{Title: "Habits", Blocks: []core.Block{
		{Text: "a paragraph before"},
		{Text: "see [[Headed]] for the headings"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	// Back to the headed note, and to its backlinks.
	openNote(t, tt, "Headed")
	a.reload()
	tt.Key(ui.Cmd|ui.Shift, ui.KeyB)

	label := msgs.BacklinkAt("Habits", 2)
	if !tt.HasText(label) {
		t.Fatalf("the panel does not show the backlink %q; texts %q", label, tt.Texts())
	}
	if err := tt.Click(label); err != nil {
		t.Fatalf("choose the backlink: %v", err)
	}
	tt.Advance(320 * time.Millisecond)

	if a.notes.Loaded != string(source.ID) {
		t.Fatalf("the editor holds %q, want the source note", a.notes.Loaded)
	}
	if at, ok := a.notes.ParagraphAtCaret(); !ok || at != 1 {
		t.Errorf("the caret is in paragraph %d, want the paragraph that links here", at+1)
	}
}

// The panel scrolls on its own: the carets and the document stay where they
// were, and the rows it holds are inside its own scroll region.
func TestPanelScrollsOnItsOwn(t *testing.T) {
	a, tt, _ := headedNote(t, 1024, 720)

	_, caretBefore := a.notes.Doc.Selection()
	before, ok := tt.Find("Note") // the document's own box
	if !ok {
		t.Fatalf("the document is not drawn; texts %q", tt.Texts())
	}
	row, ok := tt.Find("The second part")
	if !ok {
		t.Fatalf("the outline has no rows; texts %q", tt.Texts())
	}
	tt.Wheel(row.X+row.W/2, row.Y+row.H/2, 0, -20)
	tt.Advance(200 * time.Millisecond)

	after, ok := tt.Find("Note")
	if !ok || after.X != before.X || after.W != before.W {
		t.Errorf("the document moved from %+v to %+v", before, after)
	}
	if _, caret := a.notes.Doc.Selection(); caret != caretBefore {
		t.Errorf("scrolling the panel moved the caret from %d to %d", caretBefore, caret)
	}
	if strings.Contains(a.notes.Doc.Text().String(), "  ") {
		t.Error("the document's text changed while the panel scrolled")
	}
}
