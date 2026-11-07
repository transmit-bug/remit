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
		{Type: core.TypeCloze, Ordinal: 0,
			Fields:   map[string]string{"Text": "The {{first}} blank and the {{second}} one."},
			Producer: core.Producer{ID: "remit.cards.standard", Version: "1.0.0"}},
		{Type: core.TypeCloze, Ordinal: 1,
			Fields: map[string]string{"Text": "The {{first}} blank and the {{second}} one."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("the cloze made %d cards, want 2", len(cards))
	}
	for i, card := range cards {
		if card.Type != core.TypeCloze || card.Ordinal != i {
			t.Errorf("card %d is %v/%d, want cloze/%d", i, card.Type, card.Ordinal, i)
		}
	}
	// A card persists only what identifies it — type, ordinal, producer. The
	// field values the pass computed are resolved from the block at render
	// time, so they are not on disk.
	if cards[0].Fields != nil || cards[0].Edited {
		t.Errorf("the derived fields were stored: %+v", cards[0])
	}
	if cards[0].Producer.ID != "remit.cards.standard" || cards[0].Producer.Version != "1.0.0" {
		t.Errorf("the producer was not kept: %+v", cards[0].Producer)
	}

	both, err := lib.CreateCards(note.Blocks[1].ID, []core.Derived{
		{Type: core.TypeBidirectional, Ordinal: 0},
		{Type: core.TypeBidirectional, Ordinal: 1},
	})
	if err != nil || len(both) != 2 {
		t.Fatalf("the both-ways paragraph made %v (%v)", both, err)
	}
	if both[1].Type != core.TypeBidirectional || both[1].Ordinal != 1 {
		t.Errorf("the reversed card is %v/%d", both[1].Type, both[1].Ordinal)
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
	seen := map[core.ID]int{}
	for _, card := range all {
		seen[card.Type]++
	}
	if seen[core.TypeCloze] != 2 || seen[core.TypeBidirectional] != 2 {
		t.Errorf("types after reopening: %+v", seen)
	}
}
