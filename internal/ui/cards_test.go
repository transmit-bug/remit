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

// newNoteTyped starts a fresh note and writes body into its document, which is
// where the card gesture happens.
func newNoteTyped(t *testing.T, a *App, tt *ui.Tester, body string) {
	t.Helper()
	notesMode(t, a, tt)
	if err := tt.Click("New note"); err != nil {
		t.Fatalf("new note: %v", err)
	}
	if err := tt.Click("Note"); err != nil {
		t.Fatalf("open the document: %v", err)
	}
	tt.Type(body)
	tt.Advance(320 * time.Millisecond)
}

// Typing the separator is what makes a paragraph a card: the badge appears with
// the count, and the card points at the paragraph. The library holds the card,
// never the paragraph's faces.
func TestSeparatorMakesACard(t *testing.T) {
	msgs := locales.In("en")
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	newNoteTyped(t, a, tt, "What is spacing? :: Practice spread over days.")

	badge := msgs.CardOnParagraph(1, 1)
	if !tt.HasText(badge) {
		t.Fatalf("the paragraph's badge did not appear; texts %q", tt.Texts())
	}
	cards := a.Library().Cards()
	if len(cards) != 5 {
		t.Fatalf("the library holds %d cards, want the seeded four and the new one", len(cards))
	}
	// The card reads its paragraph, and its faces live nowhere on disk.
	note, ok := a.Library().Note(a.editor.draft.ID)
	if !ok {
		t.Fatal("the new note is gone")
	}
	onDisk, err := os.ReadFile(filepath.Join(seed.dir, "notes", noteFile(a, note.ID)))
	if err == nil {
		if strings.Contains(string(onDisk), `"what is spacing?`) {
			t.Errorf("the note file holds a card face:\n%s", onDisk)
		}
	}
	card, ok := a.Library().Card(cards[len(cards)-1].ID)
	if !ok {
		t.Fatal("the new card is gone")
	}
	prompt, answer, state := card.Sides(a.Library())
	if state != core.LinkLive || prompt != "What is spacing?" || answer != "Practice spread over days." {
		t.Errorf("the card reads %q / %q (%s)", prompt, answer, state)
	}
}

// Taking the separator out again takes the card out of the queue: it is
// suspended, its history kept, and its badge is gone.
func TestRemovingTheSeparatorSuspendsTheCard(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	newNoteTyped(t, a, tt, "What is spacing? :: Practice spread over days.")

	cards := a.Library().Cards()
	if len(cards) == 0 {
		t.Fatal("no card was made")
	}
	made := cards[len(cards)-1]

	if err := tt.Click("Note"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("What is spacing?")
	tt.Advance(320 * time.Millisecond)

	after, ok := a.Library().Card(made.ID)
	if !ok {
		t.Fatal("the card was deleted; the rules keep it")
	}
	// Replacing the whole paragraph takes its marker with it, so the card loses
	// the block it pointed at rather than keeping an empty prompt.
	if _, _, state := after.Sides(a.Library()); state != core.LinkLost {
		t.Errorf("the card is %s, want lost", state)
	}
	if tt.HasText(locales.In("en").CardOnParagraph(1, 1)) {
		t.Errorf("the badge stayed after the separator left; texts %q", tt.Texts())
	}
}

// The badge edits the card's own prompt and answer, and what it writes is what
// the store holds.
func TestCardBadgeEditsOverrides(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	newNoteTyped(t, a, tt, "What is spacing? :: Practice spread over days.")

	cards := a.Library().Cards()
	if len(cards) == 0 {
		t.Fatal("no card was made")
	}
	made := cards[len(cards)-1]

	if err := tt.Click(msgs.CardOnParagraph(1, 1)); err != nil {
		t.Fatalf("open the card's overrides: %v", err)
	}
	if !tt.HasText(msgs.EditCard()) {
		t.Fatalf("the dialog did not open; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.CardPrompt()); err != nil {
		t.Fatal(err)
	}
	tt.Type("What is spaced practice?")
	if err := tt.Click(msgs.Save()); err != nil {
		t.Fatal(err)
	}

	after, ok := a.Library().Card(made.ID)
	if !ok {
		t.Fatal("the card is gone")
	}
	if after.Prompt != "What is spaced practice?" {
		t.Errorf("the override is %q, want what the dialog took", after.Prompt)
	}
	if prompt, _, _ := after.Sides(a.Library()); prompt != "What is spaced practice?" {
		t.Errorf("the card reads %q, want the override", prompt)
	}
}

// The badges sit above the document and stay inside the smallest window.
func TestCardBadgesFitTheSmallestWindow(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)
	newNoteTyped(t, a, tt, "What is spacing? :: Practice spread over days.")

	badge, ok := tt.Find(msgs.CardOnParagraph(1, 1))
	if !ok {
		t.Fatalf("the badge is not drawn; texts %q", tt.Texts())
	}
	if badge.X < 0 || badge.Y < 0 || badge.X+badge.W > 720 || badge.Y+badge.H > 520 {
		t.Errorf("the badge box %+v is outside the 720x520 window", badge)
	}
	editor, ok := tt.Find("Note")
	if !ok {
		t.Fatalf("the document is not drawn; texts %q", tt.Texts())
	}
	if badge.Y+badge.H > editor.Y {
		t.Errorf("the badge %+v covers the document %+v", badge, editor)
	}
}

// noteFile is the note's file name under notes/: the slug and the id.
func noteFile(a *App, id core.ID) string {
	entries, err := os.ReadDir(filepath.Join(a.Settings().LibraryDir, "notes"))
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "-"+string(id)+".md") {
			return entry.Name()
		}
	}
	return ""
}
