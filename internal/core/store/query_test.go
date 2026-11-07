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
		{Text: "::: type remit.basic\nFront: a form field\nBack: a form answer\n:::"},
	}})
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

	// An edit is the only way a card's own text enters the index: what it did
	// not edit is still the block's text, and the word "rewritten" is nowhere
	// but the override.
	edited := cards[0]
	edited.Fields = map[string]string{"Front": "rewritten 7% tighter_phrasing"}
	edited.Edited = true
	if err := lib.SaveCard(edited); err != nil {
		t.Fatalf("save edited card: %v", err)
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
		{"a bare percent matches only the literal percents", "%", 2},
		{"a bare underscore matches only the literal underscores", "_", 2},
		{"empty", "   ", 0},
		{"an edited field value", "rewritten", 1},
		{"an edited field value case insensitively", "REWRITTEN", 1},
		{"a percent in an edited field value is literal", "7%", 1},
		{"an underscore in an edited field value is literal", "tighter_phrasing", 1},
		{"a field row in the block body", "form field", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSearch(t, lib, tt.query, tt.want)
		})
	}

	t.Run("a rebuilt index finds an edit", func(t *testing.T) {
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("rebuild: %v", err)
		}
		assertSearch(t, lib, "rewritten", 1)
		assertSearch(t, lib, "tighter_phrasing", 1)
	})

	t.Run("clearing an edit drops its text", func(t *testing.T) {
		cleared := edited
		cleared.Fields = nil
		cleared.Edited = false
		if err := lib.SaveCard(cleared); err != nil {
			t.Fatalf("save cleared card: %v", err)
		}
		assertSearch(t, lib, "rewritten", 0)
		assertSearch(t, lib, "tighter_phrasing", 0)
		// The block the card renders from is still what a search finds.
		assertSearch(t, lib, "spacing", 1)
	})
}

func assertSearch(t *testing.T, lib *store.Library, query string, want int) {
	t.Helper()
	got, err := lib.SearchCards(query)
	if err != nil {
		t.Fatalf("SearchCards(%q): %v", query, err)
	}
	if len(got) != want {
		t.Errorf("SearchCards(%q) = %d cards, want %d", query, len(got), want)
	}
}

// Notes and cards search the same way: the index, a block's text, a user's
// phrase taken literally. A note is what the note's blocks hold.
func TestSearchNotes(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()

	biology, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{
		{Text: "spacing beats cramming :: yes"},
		{Text: "half of 50% is 25% :: maths"},
		{Text: "under_score matters :: yes"},
	}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if _, err := lib.SaveNote(core.Note{Title: "Physics", Blocks: []core.Block{{Text: "entropy grows :: almost always"}}}); err != nil {
		t.Fatalf("second note: %v", err)
	}

	tests := []struct {
		name  string
		query string
		want  string // the title of the one note a query matches, empty for none
	}{
		{"case insensitively", "SPACING", "Biology"},
		{"a substring", "cramm", "Biology"},
		{"nothing", "quantum", ""},
		{"percent is literal", "50%", "Biology"},
		{"underscore is literal", "under_score", "Biology"},
		{"a bare percent matches nothing literal", "%", "Biology"},
		{"empty", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lib.SearchNotes(tt.query)
			if err != nil {
				t.Fatalf("SearchNotes(%q): %v", tt.query, err)
			}
			if tt.want == "" {
				if len(got) != 0 {
					t.Errorf("SearchNotes(%q) = %d notes, want none", tt.query, len(got))
				}
				return
			}
			if len(got) != 1 || got[0].Title != tt.want {
				t.Fatalf("SearchNotes(%q) = %+v, want the %s note", tt.query, got, tt.want)
			}
			if got[0].ID != biology.ID || len(got[0].Blocks) != len(biology.Blocks) {
				t.Errorf("SearchNotes(%q) = %q with %d blocks, want note %q with %d",
					tt.query, got[0].ID, len(got[0].Blocks), biology.ID, len(biology.Blocks))
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
	cards, err := lib.CreateCards(blockID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return cards[0], nil
}
