package transfer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

func TestRoundTripCarriesTheAssets(t *testing.T) {
	source := makeLibrary(t)
	lib, err := store.Open(source, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	rel, err := lib.PutAsset([]byte("picture-bytes"), "png")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}
	_, err = lib.SaveNote(core.Note{Title: "With picture", Blocks: []core.Block{
		{Text: "look at ![](" + rel + ")"},
	}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	archive := pack(t, source)
	defer archive.Close()

	target := filepath.Join(t.TempDir(), "copy")
	if err := Read(archive, size(t, archive.Name()), target); err != nil {
		t.Fatalf("Read: %v", err)
	}

	// The asset file travelled, under the name the note references.
	data, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("asset in copy: %v", err)
	}
	if string(data) != "picture-bytes" {
		t.Errorf("asset bytes = %q, want the stored bytes", data)
	}
}
