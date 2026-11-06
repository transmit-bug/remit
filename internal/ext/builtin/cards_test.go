package builtin_test

import (
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// derived is one card the plugin answered.
type derived struct {
	Kind    string `json:"kind"`
	Ordinal int    `json:"ordinal"`
	Front   string `json:"front"`
	Back    string `json:"back"`
}

// The texts the golden set freezes, and the shape each one takes.
var texts = []string{
	"Spacing beats cramming :: reviewing just before you forget strengthens recall.",
	"one block :: that divides :: twice",
	"a paragraph with no separator at all",
	"The {{first}} blank and the {{second}} one.",
	"spacing ::: the gaps between reviews",
	"",
	"{{only}}",
	"a :: b ::: c",
}

// cards opens the built-in plugin.
func cards(t *testing.T) *ext.JSRunner {
	t.Helper()
	runner, err := builtin.Cards(ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the built-in card plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return runner
}

// The plugin answers every text the golden set freezes exactly as the content
// model does: the same cards, the same order, the same faces.
func TestCardsMatchTheGoldenFaces(t *testing.T) {
	runner := cards(t)
	for _, text := range texts {
		t.Run(text, func(t *testing.T) {
			var got []derived
			if err := runner.Call("derive", map[string]any{"id": "block_1", "text": text}, &got); err != nil {
				t.Fatalf("derive: %v", err)
			}
			want := core.Derive(text)
			if len(got) != len(want) {
				t.Fatalf("the plugin answered %d cards, the model %d: %+v", len(got), len(want), got)
			}
			for i, w := range want {
				if got[i].Kind != w.Kind.String() || got[i].Ordinal != w.Ordinal {
					t.Errorf("card %d is %s/%d, want %s/%d", i, got[i].Kind, got[i].Ordinal, w.Kind, w.Ordinal)
				}
				if got[i].Front != w.Prompt || got[i].Back != w.Answer {
					t.Errorf("card %d asks %q / %q, want %q / %q", i, got[i].Front, got[i].Back, w.Prompt, w.Answer)
				}
			}
		})
	}
}

// The faces a card derives agree with what the review screen shows for the same
// card: the plugin's front and back are the model's two sides.
func TestCardsMatchTheModelSides(t *testing.T) {
	runner := cards(t)
	for _, text := range texts {
		var got []derived
		if err := runner.Call("derive", map[string]any{"id": "block_1", "text": text}, &got); err != nil {
			t.Fatalf("derive %q: %v", text, err)
		}
		blocks := core.BlockSet{"block_1": {ID: "block_1", Text: text}}
		for _, card := range got {
			kind, _ := core.ParseKind(card.Kind)
			prompt, answer, _ := core.Card{Ref: core.Reference{Block: "block_1"}, Kind: kind, Ordinal: card.Ordinal}.Sides(blocks)
			if prompt != card.Front || answer != card.Back {
				t.Errorf("%q %s/%d: the model asks %q / %q, the plugin %q / %q",
					text, card.Kind, card.Ordinal, prompt, answer, card.Front, card.Back)
			}
		}
	}
}

// The built-in plugin is held to the rules every extension is: a manifest this
// build cannot run is refused, and it declares no capability it does not need.
func TestBuiltinFollowsTheExtensionRules(t *testing.T) {
	manifest, err := builtin.CardsManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Capabilities) != 0 {
		t.Errorf("the card plugin declares %v; it reads what it is given", manifest.Capabilities)
	}
	if !builtin.IsBuiltin(manifest.Name) {
		t.Errorf("the plugin names itself %q", manifest.Name)
	}

	wrong := manifest
	wrong.Protocol = 99
	if _, err := ext.OpenBuiltin(wrong, []byte("export function derive() { return []; }"), ext.JSLimits{}); err == nil {
		t.Error("a built-in manifest with an unknown protocol was accepted")
	}
	loose := manifest
	loose.Entry = "../cards/main.js"
	if _, err := ext.OpenBuiltin(loose, []byte("export function derive() { return []; }"), ext.JSLimits{}); err == nil {
		t.Error("a built-in entry outside its own directory was accepted")
	}
}

// The plugin answers a card's faces from its paragraph, kind and ordinal: the
// same sides the model's own reading gives, card for card.
func TestCardsFacesMatchTheModel(t *testing.T) {
	runner := cards(t)
	for _, text := range texts {
		for _, kind := range []string{"basic", "cloze", "bidirectional"} {
			for ordinal := 0; ordinal < 3; ordinal++ {
				var got struct {
					Front string `json:"front"`
					Back  string `json:"back"`
				}
				params := map[string]any{"text": text, "kind": kind, "ordinal": ordinal}
				if err := runner.Call("faces", params, &got); err != nil {
					t.Fatalf("faces %q %s/%d: %v", text, kind, ordinal, err)
				}
				parsed, _ := core.ParseKind(kind)
				blocks := core.BlockSet{"block_1": {ID: "block_1", Text: text}}
				wantPrompt, wantAnswer, _ := core.Card{
					Ref: core.Reference{Block: "block_1"}, Kind: parsed, Ordinal: ordinal,
				}.Sides(blocks)
				if got.Front != wantPrompt || got.Back != wantAnswer {
					t.Errorf("%q %s/%d: the plugin asks %q / %q, the model %q / %q",
						text, kind, ordinal, got.Front, got.Back, wantPrompt, wantAnswer)
				}
			}
		}
	}
}
