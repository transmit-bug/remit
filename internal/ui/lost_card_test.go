package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
)

// A lost card says it lost its source and offers exactly the two ways out: point
// it at another block, or take it out of the queue.
func TestLostCardOffersTwoActions(t *testing.T) {
	a, _ := seedRelinkLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	lostCardInReview(t, a, tt)

	for _, want := range []string{lostBody, "Re-link", "Suspend card"} {
		if !tt.HasText(want) {
			t.Fatalf("the lost card is missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("Show answer") || tt.HasText("Again") {
		t.Error("a lost card offers to review an answer it does not have")
	}
}

// Re-linking a lost card puts it back in the queue, and it grades like any other
// card: one new review in the log, one more review on the card.
func TestRelinkedCardGradesInReview(t *testing.T) {
	a, seed := seedRelinkLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	lostCardInReview(t, a, tt)

	if err := tt.Click("Re-link"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Fresh")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(320 * time.Millisecond) // the re-linked card's entry

	before, ok := a.Library().CardState(seed.lost.ID)
	if !ok {
		t.Fatal("the re-linked card lost its history")
	}
	events := len(a.Library().Events())

	linked, _ := a.Library().Card(seed.lost.ID)
	_, answer, _ := a.faces(linked)
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond) // the answer unfolding
	if !tt.HasText(answer) {
		t.Fatalf("the re-linked card does not show its new answer %q; texts %q", answer, tt.Texts())
	}
	tt.Key(0, ui.Key3)                 // Good
	tt.Advance(320 * time.Millisecond) // the graded card's exit

	after, ok := a.Library().CardState(seed.lost.ID)
	if !ok {
		t.Fatal("grading the re-linked card left no state")
	}
	if after.Reviews != before.Reviews+1 {
		t.Errorf("the card has %d reviews, want %d", after.Reviews, before.Reviews+1)
	}
	if got := len(a.Library().Events()); got != events+1 {
		t.Errorf("the log holds %d events, want %d", got, events+1)
	}
	card, _ := a.Library().Card(seed.lost.ID)
	if _, _, state := a.faces(card); state != core.LinkLive {
		t.Errorf("the graded card is %s, want live", state)
	}
	if id, ok := a.review.Current(); ok && id == seed.lost.ID {
		t.Errorf("the graded card is still on screen; texts %q", tt.Texts())
	}
}

// Suspending a lost card takes it out of the queue without touching its
// history: the reviews stay, and the card does not come back.
func TestSuspendedLostCardLeavesTheQueue(t *testing.T) {
	a, seed := seedRelinkLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	lostCardInReview(t, a, tt)

	before, ok := a.Library().CardState(seed.lost.ID)
	if !ok || before.Reviews == 0 {
		t.Fatalf("the seed card has no reviews: %+v", before)
	}
	events := len(a.Library().Events())

	if err := tt.Click("Suspend card"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond)

	card, ok := a.Library().Card(seed.lost.ID)
	if !ok {
		t.Fatal("suspending deleted the card")
	}
	if card.Status != core.StatusSuspended {
		t.Errorf("the card is %s, want suspended", card.Status)
	}
	after, _ := a.Library().CardState(seed.lost.ID)
	if after.Reviews != before.Reviews {
		t.Errorf("suspending changed the reviews: %d, was %d", after.Reviews, before.Reviews)
	}
	if got := len(a.Library().Events()); got != events {
		t.Errorf("suspending changed the log: %d events, was %d", got, events)
	}

	// It is out of the queue, and stays out across a restart.
	due, err := a.Library().Due(time.Now(), 100)
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	for _, c := range due {
		if c.ID == seed.lost.ID {
			t.Fatal("the suspended card is still in the day's queue")
		}
	}
	next := NewApp(Options{LibraryDir: a.Settings().LibraryDir})
	restarted := ui.NewTester(next.View, 1024, 720)
	settle(restarted)
	if restarted.HasText(lostBody) {
		t.Error("the suspended card came back after a restart")
	}
}
