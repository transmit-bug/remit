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

	cards, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{
		{Kind: core.KindCloze, Ordinal: 0, Prompt: "The […] blank and the second one.", Answer: "first"},
		{Kind: core.KindCloze, Ordinal: 1, Prompt: "The first blank and the […].", Answer: "second"},
	})
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
	// The faces a card asks are the card plugin's shaping; the store keeps the
	// place it was told to keep.

	both, err := lib.CreateCards(note.Blocks[1].ID, []core.Derived{
		{Kind: core.KindBidirectional, Ordinal: 0, Prompt: "the gaps between reviews", Answer: "spacing"},
		{Kind: core.KindBidirectional, Ordinal: 1, Prompt: "spacing", Answer: "the gaps between reviews"},
	})
	if err != nil || len(both) != 2 {
		t.Fatalf("the both-ways paragraph made %v (%v)", both, err)
	}
	if both[1].Kind != core.KindBidirectional || both[1].Ordinal != 1 {
		t.Errorf("the reversed card is %v/%d", both[1].Kind, both[1].Ordinal)
	}

	if _, err := lib.CreateCards(note.Blocks[3].ID, nil); !errors.Is(err, store.ErrNothingToAsk) {
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
