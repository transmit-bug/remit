package store_test

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

func TestSearchCards(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()

	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{
		{Text: "spacing beats cramming :: yes"},
		{Text: "half of 50% is 25% :: maths"},
		{Text: "under_score matters :: yes"},
	}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	for _, b := range note.Blocks {
		if _, err := firstCard(lib, b.ID); err != nil {
			t.Fatalf("card: %v", err)
		}
	}

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"case insensitively", "SPACING", 1},
		{"a substring", "cramm", 1},
		{"nothing", "quantum", 0},
		{"percent is literal", "50%", 1},
		{"underscore is literal", "under_score", 1},
		{"a bare percent matches nothing literal", "%", 1},
		{"empty", "   ", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lib.SearchCards(tt.query)
			if err != nil {
				t.Fatalf("SearchCards(%q): %v", tt.query, err)
			}
			if len(got) != tt.want {
				t.Errorf("SearchCards(%q) = %d cards, want %d", tt.query, len(got), tt.want)
			}
		})
	}
}

func TestCardsWithStatus(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()

	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}, {Text: "q2 :: a2"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	var cards []core.Card
	for _, b := range note.Blocks {
		card, err := firstCard(lib, b.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards = append(cards, card)
	}
	suspended := cards[0]
	suspended.Status = core.StatusSuspended
	if err := lib.SaveCard(suspended); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := lib.CardsWithStatus(core.StatusSuspended)
	if err != nil {
		t.Fatalf("CardsWithStatus: %v", err)
	}
	if len(got) != 1 || got[0].ID != suspended.ID {
		t.Errorf("suspended = %+v, want the one suspended card", got)
	}

	if got, err := lib.CardsWithStatus(core.StatusReading); err != nil || len(got) != 0 {
		t.Errorf("reading = %+v (%v), want none", got, err)
	}
}

// firstCard makes one basic card for a block, which is all this test needs: the
// shapes a paragraph makes are the card plugin's decision, and the store only
// stores what it is told to.
func firstCard(lib *store.Library, blockID core.ID) (core.Card, error) {
	cards, err := lib.CreateCards(blockID, []core.Derived{{Kind: core.KindBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return cards[0], nil
}
