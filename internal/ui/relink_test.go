package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
)

// lostCardInReview opens the browse list filtered to Lost and enters review on
// its one lost card, where the repair is offered.
func lostCardInReview(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	tt.Frame()
	tt.Advance(320 * time.Millisecond) // the opening card's entry
	tt.Key(ui.Cmd, ui.Key3)
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
	a, seed := seedBrowseLibrary(t)
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

// A lost card's browse row offers the same repair, and the Lost filter is
// empty once the card points at a block again.
func TestRelinkFromBrowseRow(t *testing.T) {
	a, seed := seedBrowseLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Advance(320 * time.Millisecond) // the opening card's entry
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(320 * time.Millisecond) // the review card's exit

	if err := tt.Click("Lost"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Re-link"); err != nil {
		t.Fatalf("the lost row offers no repair: %v", err)
	}
	if err := tt.Click("Fresh card :: answer"); err != nil {
		t.Fatal(err)
	}

	card, _ := a.Library().Card(seed.lost.ID)
	if card.Ref.Block == seed.lost.Ref.Block {
		t.Error("the re-linked card still points at its old block")
	}
	if _, _, state := a.faces(card); state != core.LinkLive {
		t.Errorf("the re-linked card is %s, want live", state)
	}
	if !tt.HasText("No cards match.") {
		t.Fatalf("the Lost filter still lists the card; texts %q", tt.Texts())
	}
	if tt.HasText(lostBody) {
		t.Errorf("the repaired card still shows the lost body; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // the toast
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Chinese labels of the picker are wider than the English ones; at the
// smallest window they still sit inside it.
func TestRelinkPickerAtMinimumInChinese(t *testing.T) {
	a, _ := seedBrowseLibrary(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	tt.Advance(320 * time.Millisecond) // the opening card's entry
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(320 * time.Millisecond) // the review card's exit

	if err := tt.Click("已失联"); err != nil {
		t.Fatal(err)
	}
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
