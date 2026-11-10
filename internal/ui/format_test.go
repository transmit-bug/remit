package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// The format bar reports what the selection holds and changes it: selecting the
// note and clicking Bold has to reach the document, which a hand-built row of
// buttons with a bool would not.
func TestFormatBarBoldsTheSelection(t *testing.T) {
	msgs := locales.In("en")
	// Wide enough for the panel beside the paper: the writing view is the one
	// that holds it.
	a, tt, _ := headedNote(t, 1440, 900)

	if err := tt.Click(msgs.NoteBody()); err != nil {
		t.Fatalf("the document is not typeable: %v", err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	if f := a.notes.Doc.Formatting(); f.Bold == richtext.FormatOn {
		t.Fatal("the note starts bold, so the test proves nothing")
	}
	if err := tt.Click(msgs.FmtBold()); err != nil {
		t.Fatalf("the format bar has no bold control: %v", err)
	}
	tt.Frame()

	if got := a.notes.Doc.Formatting().Bold; got != richtext.FormatOn {
		t.Errorf("the selection is %v after clicking Bold, want on", got)
	}
	tt.Advance(320 * time.Millisecond) // the document saves what it holds
	if md := a.notes.Doc.Text().Markdown(); !strings.Contains(md, "**") {
		t.Errorf("the bold did not reach the document:\n%s", md)
	}
}

// The document shows the note written in, written beside the same note as it
// reads, or read whole — one switch, three panes.
func TestViewModesSwitchPanes(t *testing.T) {
	msgs := locales.In("en")
	// Wide enough for the panel beside the paper: the writing view is the one
	// that holds it.
	a, tt, _ := headedNote(t, 1440, 900)

	if err := tt.Click(msgs.ViewRead()); err != nil {
		t.Fatalf("the view switch has no reading mode: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if _, ok := tt.Find(msgs.NoteBody()); ok {
		t.Error("the document is still there in reading view")
	}
	if !tt.HasText("The first part") {
		t.Errorf("the reading view does not show the note; texts %q", tt.Texts())
	}
	if _, ok := tt.Find(msgs.Close()); ok {
		t.Error("the note's panel is open in reading view")
	}

	if err := tt.Click(msgs.ViewSplit()); err != nil {
		t.Fatalf("the view switch has no split mode: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	editor, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the document is missing in split view; texts %q", tt.Texts())
	}
	if !tt.HasText("The first part") {
		t.Errorf("the reading pane is missing in split view; texts %q", tt.Texts())
	}
	// The two halves share the room: the reading pane is a second document to
	// the right of the editor, and neither takes the other's.
	reading, ok := tt.Find("The first part")
	if !ok {
		t.Fatalf("the reading pane has no text; texts %q", tt.Texts())
	}
	if reading.X < editor.X+editor.W-editor.W/4 {
		t.Errorf("the reading pane at %v is not beside the editor at %v", reading, editor)
	}
	mid := editor.X + editor.W/2
	if reading.X < mid {
		t.Errorf("the reading pane starts at %v, inside the editor's half ending at %v", reading.X, mid)
	}

	if err := tt.Click(msgs.ViewEdit()); err != nil {
		t.Fatalf("the view switch has no writing mode: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if _, ok := tt.Find(msgs.NoteBody()); !ok {
		t.Errorf("the document is missing in writing view; texts %q", tt.Texts())
	}
	if _, ok := tt.Find(msgs.Close()); !ok {
		t.Errorf("the note's panel did not come back; texts %q", tt.Texts())
	}
	if a.notes.View != notes.Edit {
		t.Errorf("the document is in view %v, want writing", a.notes.View)
	}
}
