package ext_test

import (
	"path/filepath"
	"testing"

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

	var cards []struct {
		Kind    string `json:"kind"`
		Ordinal int    `json:"ordinal"`
		Front   string `json:"front"`
		Back    string `json:"back"`
	}
	if err := runner.Call("derive", map[string]any{"id": "block_1", "text": "spacing :: the gaps"}, &cards); err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(cards) != 1 || cards[0].Kind != "basic" || cards[0].Front != "spacing" || cards[0].Back != "the gaps" {
		t.Fatalf("the example answered %+v", cards)
	}

	var faces struct {
		Front string `json:"front"`
		Back  string `json:"back"`
	}
	if err := runner.Call("faces", map[string]any{"text": "spacing :: the gaps", "kind": "basic", "ordinal": 0}, &faces); err != nil {
		t.Fatalf("faces: %v", err)
	}
	if faces.Front != "spacing" || faces.Back != "the gaps" {
		t.Errorf("the example shaped %+v", faces)
	}

	// A paragraph without a separator asks nothing, and says so by answering
	// with no cards rather than by failing.
	cards = nil
	if err := runner.Call("derive", map[string]any{"id": "block_1", "text": "just prose"}, &cards); err != nil {
		t.Fatalf("derive on prose: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("prose made %+v", cards)
	}
}
