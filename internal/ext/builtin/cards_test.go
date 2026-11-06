package builtin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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

// frozen is the answer set #100 froze, read as the evidence it is: the plugin's
// cards are held to it, word for word, rather than to any rule this package
// still carries.
type frozen struct {
	Note  string `json:"note"`
	Faces []struct {
		Proves  string `json:"proves"`
		Text    string `json:"text"`
		Kind    string `json:"kind"`
		Ordinal int    `json:"ordinal"`
		Prompt  string `json:"prompt"`
		Answer  string `json:"answer"`
		State   string `json:"state"`
		// What the card carries of its own, which the plugin does not answer.
		OverridePrompt string `json:"override_prompt"`
		OverrideAnswer string `json:"override_answer"`
		FrozenText     string `json:"frozen_text"`
		Lost           bool   `json:"lost"`
	} `json:"faces"`
}

// frozenSet reads the frozen answers the content model and the schedule were
// pinned with.
func frozenSet(t *testing.T) frozen {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "golden", "testdata", "golden.json"))
	if err != nil {
		t.Fatalf("read the frozen answers: %v", err)
	}
	var set frozen
	if err := json.Unmarshal(data, &set); err != nil {
		t.Fatalf("decode the frozen answers: %v", err)
	}
	if len(set.Faces) == 0 {
		t.Fatal("the frozen answers hold no faces")
	}
	return set
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

// Every card the plugin derives is a face the frozen set holds, word for word:
// the shapes cannot drift from the evidence, and the plugin cannot invent one.
func TestCardsMatchTheGoldenFaces(t *testing.T) {
	runner := cards(t)
	set := frozenSet(t)

	seen := map[string]bool{}
	for _, face := range set.Faces {
		if face.Text == "" || seen[face.Text] {
			continue
		}
		seen[face.Text] = true
		t.Run(face.Text, func(t *testing.T) {
			var got []derived
			if err := runner.Call("derive", map[string]any{"id": "block_1", "text": face.Text}, &got); err != nil {
				t.Fatalf("derive: %v", err)
			}
			for _, card := range got {
				if !frozenHasPlace(set, face.Text, card.Kind, card.Ordinal) {
					t.Errorf("the plugin answers %s/%d, a place the frozen set does not hold", card.Kind, card.Ordinal)
					continue
				}
				// Where the set froze the paragraph's own face — no override, no
				// snapshot — it froze the words too.
				if !frozenPlain(face.Text, set) {
					continue
				}
				if !frozenHas(set, face.Text, card) {
					t.Errorf("the plugin answers %s/%d %q / %q, which the frozen set does not hold",
						card.Kind, card.Ordinal, card.Front, card.Back)
				}
			}
		})
	}
}

// frozenHasPlace reports whether the set froze this place of this text.
func frozenHasPlace(set frozen, text, kind string, ordinal int) bool {
	for _, face := range set.Faces {
		if face.Text == text && face.Kind == kind && face.Ordinal == ordinal {
			return true
		}
	}
	return false
}

// frozenPlain reports whether any face of this text was frozen without an
// override, a snapshot or a lost block: only then is its wording the plugin's.
func frozenPlain(text string, set frozen) bool {
	for _, face := range set.Faces {
		if face.Text == text && face.OverridePrompt == "" && face.OverrideAnswer == "" &&
			face.FrozenText == "" && !face.Lost {
			return true
		}
	}
	return false
}

// frozenHas reports whether the frozen set holds this card of this text.
func frozenHas(set frozen, text string, card derived) bool {
	for _, face := range set.Faces {
		if face.Text != text || face.Kind != card.Kind || face.Ordinal != card.Ordinal {
			continue
		}
		if face.Prompt == card.Front && face.Answer == card.Back {
			return true
		}
	}
	return false
}

// The plugin's faces for a card that exists agree with the same frozen set: the
// shapes live in one place only if both doors answer alike. A face the set
// recorded with an override, a snapshot or a lost block is left out — those are
// the card's own, laid on top of what the plugin answers.
func TestCardsFacesMatchTheGoldens(t *testing.T) {
	runner := cards(t)
	set := frozenSet(t)
	for _, face := range set.Faces {
		if face.OverridePrompt != "" || face.OverrideAnswer != "" || face.FrozenText != "" || face.Lost {
			continue
		}
		t.Run(face.Kind+"/"+face.Text, func(t *testing.T) {
			var got struct {
				Front string `json:"front"`
				Back  string `json:"back"`
			}
			params := map[string]any{"text": face.Text, "kind": face.Kind, "ordinal": face.Ordinal}
			if err := runner.Call("faces", params, &got); err != nil {
				t.Fatalf("faces: %v", err)
			}
			if got.Front != face.Prompt || got.Back != face.Answer {
				t.Errorf("the plugin asks %q / %q, the frozen set %q / %q (%s)",
					got.Front, got.Back, face.Prompt, face.Answer, face.Proves)
			}
		})
	}
}

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
