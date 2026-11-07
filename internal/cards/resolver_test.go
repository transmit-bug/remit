package cards_test

import (
	"testing"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// newResolver opens the built-in plugin under a resolver, the way the app
// does, with the types it answers against.
func newResolver(t *testing.T) *cards.Resolver {
	t.Helper()
	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the built-in card plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return cards.New(runner, core.BuiltinTypes())
}

func TestDeriveAnswersTypedCards(t *testing.T) {
	r := newResolver(t)

	got, ok := r.Derive("spacing :: the gaps between reviews")
	if !ok || len(got) != 1 {
		t.Fatalf("a separator made %+v (ok %v)", got, ok)
	}
	first := got[0]
	if first.Type != core.TypeBasic || first.Ordinal != 0 ||
		first.Fields["Front"] != "spacing" || first.Fields["Back"] != "the gaps between reviews" {
		t.Errorf("answered %+v", first)
	}
	if first.Producer != (core.Producer{ID: "remit.cards.standard", Version: "1.0.0"}) {
		t.Errorf("producer = %+v", first.Producer)
	}

	got, ok = r.Derive("The {{first}} blank and the {{second}} one.")
	if !ok || len(got) != 2 {
		t.Fatalf("a two-blank paragraph made %+v (ok %v)", got, ok)
	}
	for i, d := range got {
		if d.Type != core.TypeCloze || d.Ordinal != i || d.Fields["Text"] == "" {
			t.Errorf("blank %d answered %+v", i, d)
		}
	}

	if got, ok = r.Derive("just prose"); !ok || len(got) != 0 {
		t.Errorf("prose made %+v (ok %v)", got, ok)
	}
}

// A typed block answers its type, and fields the type does not carry are
// dropped with a warning rather than a refusal.
func TestDeriveAnswersTypedBlocks(t *testing.T) {
	r := newResolver(t)
	got, ok := r.Derive("::: type remit.basic\nFront: asked\nBack: answered\nNope: dropped\n:::")
	if !ok || len(got) != 1 {
		t.Fatalf("a typed block made %+v (ok %v)", got, ok)
	}
	if got[0].Type != core.TypeBasic || got[0].Fields["Front"] != "asked" || got[0].Fields["Nope"] != "" {
		t.Errorf("the typed block answered %+v", got[0])
	}

	// A type the library does not hold is dropped, not guessed.
	if got, ok = r.Derive("::: type no.such\nA: b\n:::"); !ok || len(got) != 0 {
		t.Errorf("an unknown type made %+v (ok %v)", got, ok)
	}
}

// A nil plugin is allowed: nothing derives, and a card reads its paragraph's
// own words.
func TestWithoutPlugin(t *testing.T) {
	r := cards.New(nil, core.BuiltinTypes())
	if _, ok := r.Derive("a :: b"); ok {
		t.Error("no plugin answered")
	}
	card := core.Card{Type: core.TypeBasic, Ref: core.Reference{Block: "b1"}}
	got := r.Face(card, core.BlockSet{"b1": {ID: "b1", Text: "asked :: answered"}})
	if got.Front != "asked :: answered" || got.Back != "" {
		t.Errorf("without a plugin the card reads %q / %q, unsplit", got.Front, got.Back)
	}
}

// The faces a card takes, as the golden set froze their shapes: this is the
// door the app, the agent API and the golden set share.
func TestFace(t *testing.T) {
	r := newResolver(t)
	blocks := core.BlockSet{
		"b1": {ID: "b1", Text: "Spacing beats cramming :: reviewing just before you forget strengthens recall."},
		"b2": {ID: "b2", Text: "The {{first}} blank and the {{second}} one."},
		"b3": {ID: "b3", Text: "spacing ::: the gaps between reviews"},
		"b4": {ID: "b4", Text: "a paragraph with no separator at all"},
	}
	tests := []struct {
		name  string
		card  core.Card
		front string
		back  string
		state core.LinkState
	}{
		{"live basic asks its paragraph",
			core.Card{Type: core.TypeBasic, Ref: core.Reference{Block: "b1"}},
			"Spacing beats cramming", "reviewing just before you forget strengthens recall.", core.LinkLive},
		{"an edited field replaces its side",
			core.Card{Type: core.TypeBasic, Ref: core.Reference{Block: "b1"},
				Fields: map[string]string{"Front": "asked another way"}, Edited: true},
			"asked another way", "reviewing just before you forget strengthens recall.", core.LinkLive},
		{"a lost card with an edit keeps it",
			core.Card{Type: core.TypeBasic, Ref: core.Reference{Block: "gone"},
				Fields: map[string]string{"Front": "still asked"}, Edited: true},
			"still asked", "", core.LinkLost},
		{"a lost card without an edit asks nothing it cannot",
			core.Card{Type: core.TypeBasic, Ref: core.Reference{Block: "gone"}},
			"", "", core.LinkLost},
		{"a frozen card asks its snapshot",
			core.Card{Type: core.TypeBasic, Ref: core.Reference{Block: "b1",
				Frozen: &core.Snapshot{Text: "old prompt :: old answer"}}},
			"old prompt", "old answer", core.LinkFrozen},
		{"the first blank is the gap",
			core.Card{Type: core.TypeCloze, Ref: core.Reference{Block: "b2"}, Ordinal: 0},
			"The […] blank and the second one.", "The first blank and the second one.", core.LinkLive},
		{"the second blank is the gap",
			core.Card{Type: core.TypeCloze, Ref: core.Reference{Block: "b2"}, Ordinal: 1},
			"The first blank and the […] one.", "The first blank and the second one.", core.LinkLive},
		{"a blank that is gone asks what it can",
			core.Card{Type: core.TypeCloze, Ref: core.Reference{Block: "b2"}, Ordinal: 5},
			"The first blank and the second one.", "The first blank and the second one.", core.LinkLive},
		{"both ways asks as written",
			core.Card{Type: core.TypeBidirectional, Ref: core.Reference{Block: "b3"}, Ordinal: 0},
			"spacing", "the gaps between reviews", core.LinkLive},
		{"both ways asks reversed",
			core.Card{Type: core.TypeBidirectional, Ref: core.Reference{Block: "b3"}, Ordinal: 1},
			"the gaps between reviews", "spacing", core.LinkLive},
		{"a paragraph that asks nothing reads unsplit",
			core.Card{Type: core.TypeBasic, Ref: core.Reference{Block: "b4"}},
			"a paragraph with no separator at all", "", core.LinkLive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Face(tt.card, blocks)
			if got.Front != tt.front || got.Back != tt.back {
				t.Errorf("Face = %q / %q, want %q / %q", got.Front, got.Back, tt.front, tt.back)
			}
			if got.State != tt.state {
				t.Errorf("State = %v, want %v", got.State, tt.state)
			}
		})
	}
}

// A type whose templates are broken — a user's edit — renders the field list
// rather than a blank or a crash.
func TestFaceFallsBackOnABrokenTemplate(t *testing.T) {
	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the built-in card plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	broken := core.BuiltinTypes()
	broken[0].Front = "{{Nope"
	r := cards.New(runner, broken)
	card := core.Card{Type: core.TypeBasic,
		Ref: core.Reference{Block: "b1"}}
	got := r.Face(card, core.BlockSet{"b1": {ID: "b1", Text: "asked :: answered"}})
	if got.Front == "" || got.Back == "" {
		t.Errorf("a broken template left a blank face: %q / %q", got.Front, got.Back)
	}
}
