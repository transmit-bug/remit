package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// lostBody is the sentence a lost card's row shows: its prompt has no text of
// its own, because the block it came from is gone.
const lostBody = "The note block this card came from is gone."

// relinkSeed is the library the repair tests read: a due card, a card never
// reviewed and a card whose paragraph is gone.
type relinkSeed struct {
	due, fresh, suspended, lost core.Card
}

// seedRelinkLibrary writes a library with one due card, one never reviewed, one
// suspended and one whose block is gone.
func seedRelinkLibrary(t *testing.T) (*App, relinkSeed) {
	t.Helper()
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	note, err := lib.SaveNote(core.Note{Title: "Filters", Blocks: []core.Block{
		{Text: "Due card :: answer"},
		{Text: "Fresh card :: answer"},
		{Text: "Suspended card :: answer"},
		{Text: "Lost card :: answer"},
	}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	card := func(i int) core.Card {
		c, err := firstCard(lib, note.Blocks[i].ID)
		if err != nil {
			t.Fatalf("create card: %v", err)
		}
		return c
	}
	due, fresh, suspended, lost := card(0), card(1), card(2), card(3)

	// Again a day ago leaves the card due now, in learning.
	if err := lib.Review(due.ID, schedule.Again, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("review due card: %v", err)
	}
	suspended.Status = core.StatusSuspended
	if err := lib.SaveCard(suspended); err != nil {
		t.Fatalf("suspend card: %v", err)
	}
	// Reviewed a day ago and scheduled ahead: lost, but neither new nor due.
	if err := lib.Review(lost.ID, schedule.Good, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("review lost card: %v", err)
	}
	// Deleting the paragraph the card was made from loses its reference.
	note.Blocks = note.Blocks[:3]
	if _, err := lib.SaveNote(note); err != nil {
		t.Fatalf("delete block: %v", err)
	}

	return NewApp(Options{LibraryDir: dir}), relinkSeed{due: due, fresh: fresh, suspended: suspended, lost: lost}
}

// lostCardInReview opens the card table filtered to Lost and enters review on
// its one lost card, where the repair is offered.
func lostCardInReview(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	tt.Frame()
	tt.Advance(320 * time.Millisecond) // the opening card's entry
	cardsPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond) // the review card's exit
	if err := tt.Click("Lost"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(lostBody); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
}

// A lost card is re-linked from its review view: the new block is what it
// reads, and its status, due date and reviews are untouched.
func TestRelinkFromReview(t *testing.T) {
	a, seed := seedRelinkLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	lostCardInReview(t, a, tt)

	if !tt.HasText("Re-link") || !tt.HasText(lostBody) {
		t.Fatalf("the lost card offers no repair; texts %q", tt.Texts())
	}
	before, ok := a.Library().CardState(seed.lost.ID)
	if !ok || before.Reviews == 0 {
		t.Fatalf("the seed card has no reviews: %+v", before)
	}
	wasCard, _ := a.Library().Card(seed.lost.ID)
	events := len(a.Library().Events())

	if err := tt.Click("Re-link"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Re-link card") || !tt.HasText("Search blocks") {
		t.Fatalf("the picker did not open; texts %q", tt.Texts())
	}
	tt.Type("Fresh")
	tt.Key(0, ui.KeyEnter)

	card, ok := a.Library().Card(seed.lost.ID)
	if !ok {
		t.Fatal("the card is gone")
	}
	if card.Ref.Block != seed.fresh.Ref.Block {
		t.Errorf("the card reads %q, want %q", card.Ref.Block, seed.fresh.Ref.Block)
	}
	if _, _, state := a.faces(card); state != core.LinkLive {
		t.Errorf("the re-linked card is %s, want live", state)
	}
	after, ok := a.Library().CardState(seed.lost.ID)
	if !ok || after.Reviews != before.Reviews || !after.Due.Equal(before.Due) || card.Status != wasCard.Status {
		t.Errorf("re-linking changed the card's history: %+v, then %+v", before, after)
	}
	if got := len(a.Library().Events()); got != events {
		t.Errorf("re-linking changed the review log: %d events, was %d", got, events)
	}
	// The card the user fixed stays on screen, reading its new block.
	if !tt.HasText("Fresh card") || tt.HasText(lostBody) {
		t.Fatalf("the review does not show the re-linked card; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // the toast
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Chinese labels of the picker are wider than the English ones; at the
// smallest window they still sit inside it.
func TestRelinkPickerAtMinimumInChinese(t *testing.T) {
	a, seed := seedRelinkLibrary(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	settle(tt)
	tt.Advance(320 * time.Millisecond) // the opening card's entry

	// A card whose block is gone opens in review, where the repair is offered.
	a.showCard(seed.lost.ID)
	tt.Frame()
	if err := tt.Click("重新关联"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"重新关联卡片", "搜索块", "Fresh card :: answer"} {
		assertControl(t, tt, 720, 520, label)
	}
	if err := tt.Click("Fresh card :: answer"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("卡片已重新关联") {
		t.Fatalf("the re-link was not reported; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // the toast
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
