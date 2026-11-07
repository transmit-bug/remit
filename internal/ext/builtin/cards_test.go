package builtin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/template"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// derived is one card the plugin answered: a place in a paragraph, typed and
// with its field values.
type derived struct {
	TypeID  string            `json:"typeId"`
	Ordinal int               `json:"ordinal"`
	Fields  map[string]string `json:"fields"`
	Unknown []string          `json:"unknown"`
}

// frozen is the answer set #100 froze, read as the evidence it is: the
// plugin's cards are held to it, word for word, rather than to any rule this
// package still carries.
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

// cards opens the built-in plugin, and the types its answers are checked
// against.
func cards(t *testing.T) (*ext.JSRunner, []any) {
	t.Helper()
	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the built-in card plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	types := make([]any, 0, 3)
	for _, typ := range core.BuiltinTypes() {
		types = append(types, map[string]any{
			"id": string(typ.ID), "fields": typ.Fields, "version": typ.Version,
		})
	}
	return runner, types
}

// derive asks the plugin what a text makes.
func derive(t *testing.T, runner *ext.JSRunner, types []any, text string) []derived {
	t.Helper()
	var got []derived
	block := map[string]any{"id": "block_1", "text": text}
	if err := runner.Call("deriveBlock", []any{block, types}, &got); err != nil {
		t.Fatalf("deriveBlock: %v", err)
	}
	return got
}

// typeOfKind maps the frozen set's kind names onto type ids.
func typeOfKind(kind string) core.ID {
	switch kind {
	case "cloze":
		return core.TypeCloze
	case "bidirectional":
		return core.TypeBidirectional
	}
	return core.TypeBasic
}

// kindOfType maps a type id back to the name the frozen set freezes.
func kindOfType(id string) string {
	if id == string(core.TypeCloze) {
		return "cloze"
	}
	if id == string(core.TypeBidirectional) {
		return "bidirectional"
	}
	return "basic"
}

// renderFace answers one derived card's side through the host template
// engine — the same door the app draws a card through.
func renderFace(t *testing.T, typ core.Type, fields map[string]string, ordinal int, back bool) string {
	t.Helper()
	src := typ.Front
	if back {
		src = typ.Back
	}
	tpl, err := template.Compile(src)
	if err != nil {
		t.Fatalf("compile %s: %v", typ.ID, err)
	}
	side := template.SideFront
	if back {
		side = template.SideBack
	}
	out, err := tpl.Render(template.Input{Fields: fields, Ordinal: ordinal, Side: side})
	if err != nil {
		t.Fatalf("render %s: %v", typ.ID, err)
	}
	return out.Markdown
}

// Every card the plugin derives is a place the frozen set holds: the shapes
// cannot drift from the evidence, and the plugin cannot invent one.
func TestCardsMatchTheGoldenPlaces(t *testing.T) {
	runner, types := cards(t)
	set := frozenSet(t)

	seen := map[string]bool{}
	for _, face := range set.Faces {
		if face.Text == "" || seen[face.Text] {
			continue
		}
		seen[face.Text] = true
		t.Run(face.Text, func(t *testing.T) {
			for _, card := range derive(t, runner, types, face.Text) {
				kind := kindOfType(card.TypeID)
				if card.TypeID != string(typeOfKind(kind)) {
					t.Fatalf("kindOfType and typeOfKind disagree on %s", card.TypeID)
				}
				if !frozenHasPlace(set, face.Text, kind, card.Ordinal) {
					t.Errorf("the plugin answers %s/%d, a place the frozen set does not hold", kind, card.Ordinal)
				}
			}
		})
	}
}

// A typed block answers its type with the field values its lines carry, and
// names the fields the type does not know so the host can warn.
func TestCardsAnswerTypedBlocks(t *testing.T) {
	runner, types := cards(t)
	text := "::: type remit.basic\nFront: asked\nBack: answered\nNope: dropped\n:::"
	got := derive(t, runner, types, text)
	if len(got) != 1 {
		t.Fatalf("a typed block made %+v", got)
	}
	if got[0].TypeID != string(core.TypeBasic) || got[0].Ordinal != 0 {
		t.Fatalf("the typed block made %+v", got[0])
	}
	if got[0].Fields["Front"] != "asked" || got[0].Fields["Back"] != "answered" {
		t.Errorf("fields = %v", got[0].Fields)
	}
	if len(got[0].Unknown) != 1 || got[0].Unknown[0] != "Nope" {
		t.Errorf("unknown = %v, want [Nope]", got[0].Unknown)
	}

	// A type the library does not hold still answers — the host drops it and
	// says so; the plugin does not second-guess the library.
	strange := derive(t, runner, types, "::: type no.such\nA: b\n:::")
	if len(strange) != 1 || strange[0].TypeID != "no.such" {
		t.Fatalf("an unknown type answered %+v", strange)
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

// The plugin's derived fields, rendered through the type's templates, agree
// with the frozen set word for word: the shapes live in one place only if
// both doors answer alike. A face the set recorded with an override, a
// snapshot or a lost block is left out — those are the card's own, laid on
// top of what the plugin answers.
func TestCardsRenderLikeTheGoldens(t *testing.T) {
	runner, types := cards(t)
	set := frozenSet(t)
	compiled := map[core.ID]*template.Template{}
	for _, typ := range core.BuiltinTypes() {
		tpl, err := template.Compile(typ.Front)
		if err != nil {
			t.Fatalf("compile %s front: %v", typ.ID, err)
		}
		compiled[typ.ID] = tpl
		back, err := template.Compile(typ.Back)
		if err != nil {
			t.Fatalf("compile %s back: %v", typ.ID, err)
		}
		compiled[typ.ID+"?back"] = back
	}
	for _, face := range set.Faces {
		if face.OverridePrompt != "" || face.OverrideAnswer != "" || face.FrozenText != "" || face.Lost {
			continue
		}
		t.Run(face.Kind+"/"+face.Text, func(t *testing.T) {
			cards := derive(t, runner, types, face.Text)
			var match *derived
			for i := range cards {
				if kindOfType(cards[i].TypeID) == face.Kind && cards[i].Ordinal == face.Ordinal {
					match = &cards[i]
				}
			}
			if match == nil {
				// The paragraph does not ask this place (a blank that is gone,
				// a separator that is not there): that face is the resolver's
				// door, held by internal/golden, not the plugin's.
				t.Skipf("the text derives no %s/%d card", face.Kind, face.Ordinal)
			}
			typ, ok := typeByID(typeOfKind(face.Kind))
			if !ok {
				t.Fatalf("no builtin type for %s", face.Kind)
			}
			front := renderFace(t, typ, match.Fields, face.Ordinal, false)
			back := renderFace(t, typ, match.Fields, face.Ordinal, true)
			if front != face.Prompt || back != face.Answer {
				t.Errorf("rendered %q / %q, the frozen set %q / %q (%s)",
					front, back, face.Prompt, face.Answer, face.Proves)
			}
		})
	}
}

// typeByID answers the built-in type with the given id.
func typeByID(id core.ID) (core.Type, bool) {
	for _, typ := range core.BuiltinTypes() {
		if typ.ID == id {
			return typ, true
		}
	}
	return core.Type{}, false
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
	if _, err := ext.OpenBuiltin(wrong, []byte("export function deriveBlock() { return []; }"), nil, ext.JSLimits{}); err == nil {
		t.Error("a built-in manifest carrying a wire protocol was accepted")
	}
	loose := manifest
	loose.Entry = "../cards/main.js"
	if _, err := ext.OpenBuiltin(loose, []byte("export function deriveBlock() { return []; }"), nil, ext.JSLimits{}); err == nil {
		t.Error("a built-in entry outside its own directory was accepted")
	}
}
