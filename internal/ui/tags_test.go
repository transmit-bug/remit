package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/locales"
)

// taggedNotebook opens the notebook on a library whose notes carry tags.
func taggedNotebook(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a, _ := newTestApp(t)
	if _, err := a.Library().SaveNote(core.Note{Title: "Spacing notes", Blocks: []core.Block{
		{Text: "the gaps between reviews #learning"},
		{Text: "and the queue #learning #spacing"},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Library().SaveNote(core.Note{Title: "Habit notes", Blocks: []core.Block{
		{Text: "one short sitting #habit"},
	}}); err != nil {
		t.Fatal(err)
	}
	// A hash in code is code, not a tag.
	if _, err := a.Library().SaveNote(core.Note{Title: "Code notes", Blocks: []core.Block{
		{Text: "```\n#notatag\n```"},
		{Text: "`#noranother` but #realcode"},
	}}); err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	notesMode(t, a, tt)
	return a, tt
}

// Tags are in the sidebar, and choosing one narrows the list to the notes that
// carry it; clearing brings the rest back.
func TestTagFilterNarrowsTheList(t *testing.T) {
	msgs := locales.In("en")
	a, tt := taggedNotebook(t)

	for _, chip := range []string{"#learning", "#habit", "#realcode"} {
		if !tt.HasText(chip) {
			t.Fatalf("the sidebar has no %q chip; texts %q", chip, tt.Texts())
		}
	}
	for _, gone := range []string{"#notatag", "#noranother"} {
		if tt.HasText(gone) {
			t.Errorf("code produced the tag %q", gone)
		}
	}

	if err := tt.Click("#habit"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Habit notes") {
		t.Errorf("the tagged note left the list; texts %q", tt.Texts())
	}
	for _, gone := range []string{"Spacing notes", "Code notes"} {
		if tt.HasText(gone) {
			t.Errorf("%q is still listed under #habit", gone)
		}
	}

	if err := tt.Click(msgs.ClearFilter()); err != nil {
		t.Fatalf("clear the filter: %v", err)
	}
	for _, want := range []string{"Habit notes", "Spacing notes", "Code notes"} {
		if !tt.HasText(want) {
			t.Errorf("clearing did not bring %q back; texts %q", want, tt.Texts())
		}
	}
	if a.notes.TagFilter != "" {
		t.Errorf("the filter is still %q", a.notes.TagFilter)
	}
}

// ⌘F looks in the note that is open and nowhere else: the notebook stays, and
// the match count is the open note's.
func TestFindStaysInTheOpenDocument(t *testing.T) {
	a, tt := taggedNotebook(t)
	openNote(t, tt, "Spacing notes")

	tt.Key(ui.Cmd, ui.KeyF)
	if tt.HasText(locales.In("en").BrowseTitle()) {
		t.Fatalf("⌘F went to the library's search; texts %q", tt.Texts())
	}
	if a.mode != modeNotes {
		t.Fatalf("⌘F left the notebook for %v", a.mode)
	}
	tt.Type("gaps")
	tt.Advance(320 * time.Millisecond)

	// The word is in this note once; the query selects it.
	anchor, caret := a.notes.Doc.Selection()
	text := a.notes.Doc.Text().String()
	at := strings.Index(strings.ToLower(text), "gaps")
	if at < 0 {
		t.Fatalf("the note does not hold the word: %q", text)
	}
	// The match is selected, so the anchor is its start and the caret its end.
	if anchor != at || caret != at+len("gaps") {
		t.Errorf("the selection is %d..%d, want the match at %d..%d", anchor, caret, at, at+len("gaps"))
	}
	// The other notes are not searched: their words are nowhere in the bar.
	if tt.HasText("1 of 2") {
		t.Error("the bar counted matches from another note")
	}
}

// The find bar closes on Escape and leaves the notebook where it was.
func TestFindCloses(t *testing.T) {
	a, tt := taggedNotebook(t)
	openNote(t, tt, "Spacing notes")
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("gaps")
	tt.Advance(200 * time.Millisecond)
	tt.Key(0, ui.KeyEscape)
	tt.Advance(200 * time.Millisecond)
	if a.notes.FindOpen {
		t.Error("the find bar stayed open")
	}
	if !tt.HasText("Spacing notes") {
		t.Errorf("the notebook emptied; texts %q", tt.Texts())
	}
}
