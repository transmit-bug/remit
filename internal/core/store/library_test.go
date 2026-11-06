package store_test

import (
	"errors"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// A paragraph derives as many cards as it asks, and a card remembers which one
// it is across a reopen.
func TestCreateCardsDerivesPlaces(t *testing.T) {
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{
		{Text: "The {{first}} blank and the {{second}} one."},
		{Text: "spacing ::: the gaps between reviews"},
		{Text: "prompt :: answer"},
		{Text: "a plain paragraph"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	cards, err := lib.CreateCards(note.Blocks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("the cloze made %d cards, want 2", len(cards))
	}
	for i, card := range cards {
		if card.Kind != core.KindCloze || card.Ordinal != i {
			t.Errorf("card %d is %v/%d, want cloze/%d", i, card.Kind, card.Ordinal, i)
		}
	}
	prompt, answer, _ := cards[0].Sides(core.BlockSet{note.Blocks[0].ID: note.Blocks[0]})
	if prompt != "The […] blank and the second one." || answer != "first" {
		t.Errorf("the first blank asks %q / %q", prompt, answer)
	}

	both, err := lib.CreateCards(note.Blocks[1].ID)
	if err != nil || len(both) != 2 {
		t.Fatalf("the both-ways paragraph made %v (%v)", both, err)
	}
	blocks := core.BlockSet{note.Blocks[1].ID: note.Blocks[1]}
	if p, a, _ := both[1].Sides(blocks); p != "the gaps between reviews" || a != "spacing" {
		t.Errorf("the reversed card asks %q / %q", p, a)
	}

	if _, err := lib.CreateCards(note.Blocks[3].ID); !errors.Is(err, store.ErrNothingToAsk) {
		t.Errorf("a plain paragraph made %v, want ErrNothingToAsk", err)
	}

	// Reopening keeps every place.
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = again.Close() })
	all := again.Cards()
	if len(all) != 4 {
		t.Fatalf("the library holds %d cards after reopening, want 4", len(all))
	}
	seen := map[core.Kind]int{}
	for _, card := range all {
		seen[card.Kind]++
	}
	if seen[core.KindCloze] != 2 || seen[core.KindBidirectional] != 2 || seen[core.KindBasic] != 0 {
		t.Errorf("kinds after reopening: %+v", seen)
	}
}
