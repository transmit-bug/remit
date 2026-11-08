package notes

import (
	"testing"

	"github.com/transmit-bug/remit/internal/core"
)

// Inserting or deleting a block around typed text leaves the identities of
// the blocks that did not change where they were, so cards keep reading the
// paragraphs they point at.
func TestBlocksOfDocumentKeepsIDsAroundStructuralEdits(t *testing.T) {
	old := []core.Block{
		{ID: "block-a", Text: "Alpha"},
		{ID: "block-b", Text: "Bravo"},
		{ID: "block-c", Text: "Charlie"},
	}

	t.Run("insert", func(t *testing.T) {
		got := blocksOfDocument("Alpha\n\nnew\n\nBravo\n\nCharlie", old)
		assertBlockIDs(t, got, "block-a", "", "block-b", "block-c")
	})
	t.Run("delete", func(t *testing.T) {
		got := blocksOfDocument("Alpha\n\nCharlie", old)
		assertBlockIDs(t, got, "block-a", "block-c")
	})
	t.Run("edit", func(t *testing.T) {
		got := blocksOfDocument("Alpha\n\nBravo, edited\n\nCharlie", old)
		assertBlockIDs(t, got, "block-a", "block-b", "block-c")
	})
}

func assertBlockIDs(t *testing.T, blocks []core.Block, want ...string) {
	t.Helper()
	if len(blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d", len(blocks), len(want))
	}
	for i, id := range want {
		if id == "" {
			if blocks[i].ID == "" {
				t.Errorf("block %d has no id, want a new one", i)
			}
			continue
		}
		if got := string(blocks[i].ID); got != id {
			t.Errorf("block %d has id %q, want %q", i, got, id)
		}
	}
}
