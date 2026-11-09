package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// lostCardWindow opens the window on a card whose block was deleted, in the card
// table's Lost filter, and enters review on it — the state whose repair is the
// re-link dialog.
func lostCardWindow(t *testing.T) (*ui.Tester, testLibrary) {
	t.Helper()
	a, seed, tt := openApp(t)

	// Deleting a paragraph is an edit of the document now: save the note
	// without the block its card points at, and the card loses its source.
	note, ok := a.Library().Note(seed.memory.ID)
	if !ok {
		t.Fatal("the seeded note is gone")
	}
	note.Blocks = note.Blocks[1:]
	if _, err := a.Library().SaveNote(note); err != nil {
		t.Fatalf("drop the block: %v", err)
	}
	a.reload()

	cardsPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond) // the card's exit
	if err := tt.Click("Lost"); err != nil {
		t.Fatalf("show the lost cards: %v", err)
	}
	if err := tt.Click("The note block this card came from is gone."); err != nil {
		t.Fatalf("choose the lost card: %v", err)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	return tt, seed
}

// The re-link dialog's close button is a drawing, so its name is the only thing
// a screen reader has: it carries one, the pointer shows it, and Tab reaches it.
func TestIconOnlyButtonIsNamed(t *testing.T) {
	tt, _ := lostCardWindow(t)
	if err := tt.Click("Re-link"); err != nil {
		t.Fatalf("open the re-link dialog: %v", err)
	}
	if _, ok := tt.Find("Close"); !ok {
		t.Fatalf("the dialog's close button has no name; texts %q", tt.Texts())
	}

	reached := false
	for i := 0; i < 12 && !reached; i++ {
		tt.Key(0, ui.KeyTab)
		reached = tt.Focused("Close")
	}
	if !reached {
		t.Error("Tab never reaches the close button")
	}

	if err := tt.Click("Close"); err != nil {
		t.Fatalf("click the close button: %v", err)
	}
	if tt.HasText("Re-link card") {
		t.Errorf("the dialog stayed open; texts %q", tt.Texts())
	}
}
