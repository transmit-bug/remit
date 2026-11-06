package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// lostCardWindow opens the window on a card whose block was deleted, in browse,
// with the Lost filter shown — the state whose repair is the re-link dialog.
func lostCardWindow(t *testing.T) (*ui.Tester, testLibrary) {
	t.Helper()
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	chooseFromMenu(t, tt, 1, "Delete block")

	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(320 * time.Millisecond) // the card's exit
	if err := tt.Click("Lost"); err != nil {
		t.Fatalf("show the lost cards: %v", err)
	}
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

// Icons are drawn beside their text at the smallest window the app opens at, and
// nothing they sit in spills outside it.
func TestIconsFitTheSmallestWindow(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)
	tt.Advance(320 * time.Millisecond) // the card's entry

	inside := func(label string) {
		t.Helper()
		box, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %q; texts %q", label, tt.Texts())
		}
		if box.X < 0 || box.Y < 0 || box.X+box.W > 720 || box.Y+box.H > 520 {
			t.Errorf("%q box %+v is outside the 720x520 window", label, box)
		}
	}
	// Review takes the window whole: the card is all that is there.
	if !tt.HasText("Show answer") {
		t.Fatalf("the queue did not open; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	// The toolbar's new-note button carries the plus beside its text.
	inside("New note")
	// A note of the sidebar carries a page icon.
	inside("Memory and recall")
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
