package ext_test

import (
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
)

// The plugin the guide walks a reader through really loads and answers: it is
// run here, through the same loader and the same limits a downloaded plugin
// gets, so the example cannot rot.
func TestTheGuidesExamplePluginRuns(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "plugins", "cards-mini")
	runner, err := ext.OpenJS(dir, ext.JSLimits{})
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
