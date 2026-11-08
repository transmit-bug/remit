package suggest

import (
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

func newLib(t *testing.T) *store.Library {
	t.Helper()
	lib, err := store.Create(t.TempDir(), store.Options{})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib
}

func TestApplyWritesNotesAndTheirCards(t *testing.T) {
	lib := newLib(t)
	producer := core.Producer{ID: "example.importer.obsidian", Version: "0.1.0"}

	report, err := Apply(lib, Suggestions{
		Notes: []Note{{Title: "Machine Learning", Blocks: []string{"Supervised learning", "Unlabeled"}}},
		Cards: []Card{
			{Note: 0, Block: 0, Type: "remit.basic", Ordinal: 0, Fields: map[string]string{"Front": "Supervised learning", "Back": "Learning from labeled examples."}},
		},
	}, producer)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if report.Notes != 1 || report.Cards != 1 || report.Dropped != 0 {
		t.Fatalf("report = %+v, want 1 note and 1 card written", report)
	}

	notes := lib.Notes()
	if len(notes) != 1 || notes[0].Title != "Machine Learning" {
		t.Fatalf("library holds %+v, want the suggested note", notes)
	}
	if len(lib.Cards()) != 1 {
		t.Fatalf("library holds %d cards, want 1", len(lib.Cards()))
	}
	card := lib.Cards()[0]
	if card.Producer.ID != producer.ID || card.Producer.Version != producer.Version {
		t.Errorf("card producer = %+v, want the importer's stamp", card.Producer)
	}
}

func TestApplyDropsSuggestionsThatDoNotHoldUp(t *testing.T) {
	lib := newLib(t)

	report, err := Apply(lib, Suggestions{
		Notes: []Note{
			{Title: "Real note", Blocks: []string{"one block"}},
			{}, // no title, no blocks: not a note
		},
		Cards: []Card{
			{Note: 0, Block: 0, Type: "remit.basic", Ordinal: 0},
			{Note: 1, Block: 0, Type: "remit.basic", Ordinal: 0}, // its note was dropped
			{Note: 0, Block: 7, Type: "remit.basic", Ordinal: 1}, // no such block
			{Note: 0, Block: 0, Type: "remit.nosuch", Ordinal: 2},
		},
	}, core.Producer{ID: "x"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if report.Notes != 1 {
		t.Errorf("notes = %d, want 1", report.Notes)
	}
	if report.Cards != 1 || report.Dropped != 3 {
		t.Errorf("cards = %d, dropped = %d, want 1 and 3", report.Cards, report.Dropped)
	}
	if len(report.Warnings) < 3 {
		t.Errorf("warnings %q, want one per dropped card and note", report.Warnings)
	}
	for _, warning := range report.Warnings {
		if !strings.Contains(warning, "[") {
			t.Errorf("warning %q does not name its suggestion's index", warning)
		}
	}
	if len(lib.Notes()) != 1 || len(lib.Cards()) != 1 {
		t.Errorf("library holds %d notes and %d cards, want the one note and card that held up", len(lib.Notes()), len(lib.Cards()))
	}
}

func TestApplyWithNoLibraryIsAnError(t *testing.T) {
	if _, err := Apply(nil, Suggestions{}, core.Producer{}); err == nil {
		t.Error("Apply(nil, …) wrote nothing and said nothing")
	}
}
