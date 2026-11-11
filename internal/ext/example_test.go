package ext_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
)

// The plugin the guide walks a reader through really loads and answers: it is
// run here, through the same loader and the same limits a downloaded plugin
// gets, so the example cannot rot.
func TestTheGuidesExamplePluginRuns(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "plugins", "cards-mini")
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("load the example: %v", err)
	}
	defer runner.Close()
	if runner.Name() != "cards-mini" {
		t.Errorf("the example names itself %q", runner.Name())
	}

	types := []any{map[string]any{"id": string(core.TypeBasic), "fields": []string{"Front", "Back"}, "version": 1}}
	var cards []struct {
		TypeID  string            `json:"typeId"`
		Ordinal int               `json:"ordinal"`
		Fields  map[string]string `json:"fields"`
	}
	block := map[string]any{"id": "block_1", "text": "spacing :: the gaps"}
	if err := runner.Call("deriveBlock", []any{block, types}, &cards); err != nil {
		t.Fatalf("deriveBlock: %v", err)
	}
	if len(cards) != 1 || cards[0].TypeID != string(core.TypeBasic) ||
		cards[0].Fields["Front"] != "spacing" || cards[0].Fields["Back"] != "the gaps" {
		t.Fatalf("the example answered %+v", cards)
	}

	// A paragraph without a separator asks nothing, and says so by answering
	// with no cards rather than by failing.
	plain := map[string]any{"id": "block_1", "text": "just prose"}
	cards = nil
	if err := runner.Call("deriveBlock", []any{plain, types}, &cards); err != nil {
		t.Fatalf("deriveBlock on prose: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("prose made %+v", cards)
	}
}

// The typed capability and the call sites' old path ask a plugin the same thing.
// The table's adapter is the only difference between them and it is not one a
// plugin can see, which is what lets #269 move the call sites without changing
// what a plugin answers: if the adapter spread deriveBlock's input differently,
// the plugin would read no text and answer no card.
func TestTheTypedCapabilityAsksAPluginWhatCallDoes(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "plugins", "cards-mini")
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("load the example: %v", err)
	}
	defer runner.Close()

	in := ext.DeriveIn{
		Block: ext.Block{ID: "block_1", Text: "spacing :: the gaps"},
		Types: []ext.CardType{{ID: string(core.TypeBasic), Fields: []string{"Front", "Back"}, Version: 1}},
	}
	typed, err := ext.HooksOf(runner).DeriveBlock(in)
	if err != nil {
		t.Fatalf("the typed capability: %v", err)
	}
	var called []ext.DerivedCard
	if err := runner.Call("deriveBlock", []any{in.Block, in.Types}, &called); err != nil {
		t.Fatalf("the old path: %v", err)
	}
	if len(typed) == 0 {
		t.Fatalf("the typed capability derived nothing from %q", in.Block.Text)
	}
	if !reflect.DeepEqual(typed, called) {
		t.Fatalf("the two paths disagree:\ntyped: %+v\ncall:  %+v", typed, called)
	}
}
