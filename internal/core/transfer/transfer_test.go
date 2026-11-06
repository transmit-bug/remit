package transfer

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

var now = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

func makeLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "Memory", Blocks: []core.Block{{Text: "spacing :: beats cramming"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	if err := lib.Review(card.ID, schedule.Good, now); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return dir
}

// pack writes the archive to a file, which is what Read needs to address it.
func pack(t *testing.T, dir string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "library.remitlib")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	if err := Write(dir, f); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	return f
}

func TestRoundTripKeepsTheSchedule(t *testing.T) {
	source := makeLibrary(t)
	archive := pack(t, source)
	defer archive.Close()

	target := filepath.Join(t.TempDir(), "copy")
	if err := Read(archive, size(t, archive.Name()), target); err != nil {
		t.Fatalf("Read: %v", err)
	}

	lib, err := store.Open(target, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}
	defer lib.Close()

	if got := len(lib.Cards()); got != 1 {
		t.Fatalf("copy has %d cards, want 1", got)
	}
	// The reviewed card is not due yet, so the proof is its replayed state.
	card := lib.Cards()[0]
	state, ok := lib.CardState(card.ID)
	if !ok || state.Reviews != 1 {
		t.Fatalf("copy lost the review history: %+v (ok %v)", state, ok)
	}
	if !state.Due.After(now) {
		t.Errorf("copy's due date %v is not after the review at %v", state.Due, now)
	}
}

func TestArchiveDoesNotCarryTheIndex(t *testing.T) {
	source := makeLibrary(t)
	path := filepath.Join(t.TempDir(), "library.remitlib")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := Write(source, f); err != nil {
		t.Fatalf("Write: %v", err)
	}
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	for _, entry := range zr.File {
		if filepath.ToSlash(entry.Name) == ".remit" || len(entry.Name) > 6 && entry.Name[:6] == ".remit" {
			t.Errorf("archive contains derived data: %s", entry.Name)
		}
	}
}

func TestReadRefusesTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../escaped.txt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := w.Write([]byte("nope")); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "target")
	if err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()), dir); err == nil {
		t.Fatal("Read accepted an archive that escapes its directory")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escaped.txt")); err == nil {
		t.Error("Read wrote a file outside the target directory")
	}
}

func TestReadRefusesAnExistingLibrary(t *testing.T) {
	source := makeLibrary(t)
	archive := pack(t, source)
	defer archive.Close()

	if err := Read(archive, size(t, archive.Name()), source); err == nil {
		t.Fatal("Read overwrote an existing library")
	}
}

func size(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	return info.Size()
}

// firstCard makes one basic card for a block, which is all this test needs: the
// shapes a paragraph makes are the card plugin's decision, and the store only
// stores what it is told to.
func firstCard(lib *store.Library, blockID core.ID) (core.Card, error) {
	cards, err := lib.CreateCards(blockID, []core.Derived{{Kind: core.KindBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return cards[0], nil
}
