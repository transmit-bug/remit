package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// TestCurrentIndexReadsNoSources is what the scale work is about, and the one
// thing a library's public surface cannot show: how much of the disk an open
// touched. It is a package test because the count is the store's own
// bookkeeping, not behaviour a caller can observe.
func TestCurrentIndexReadsNoSources(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	lib, err := Create(dir, Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}, {Text: "q2 :: a2"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	if err := lib.Review(card.ID, schedule.Good, now); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Everything the library wrote is in the index, so opening reads neither a
	// note, nor a card, nor a line of the log.
	reopened, err := Open(dir, Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reopened.Close()
	if reopened.sources != (sourceReads{}) {
		t.Errorf("opening a current library read %+v, want no source file", reopened.sources)
	}
	if state, ok := reopened.CardState(card.ID); !ok || state.Reviews != 1 {
		t.Errorf("state after the reopen = (%+v, %v), want the one review", state, ok)
	}

	// A log that only grew costs the lines after the watermark, and those lines
	// are what an open pays for: no card file, no note, one pass over the tail.
	line := `{"seq":2,"card":"` + string(card.ID) + `","rating":"easy","at":"2026-11-06T10:00:00Z"}` + "\n"
	file, err := os.OpenFile(filepath.Join(dir, logName), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := file.WriteString(line); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := reopened.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	incremental, err := Open(dir, Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open with a longer log: %v", err)
	}
	defer incremental.Close()
	if incremental.sources.notes != 0 || incremental.sources.cards != 0 || incremental.sources.log != 1 {
		t.Errorf("replaying a one-line tail read %+v, want the log once and nothing else", incremental.sources)
	}
	if state, ok := incremental.CardState(card.ID); !ok || state.Reviews != 2 {
		t.Errorf("state after the tail = (%+v, %v), want two reviews", state, ok)
	}
	if due, err := incremental.Due(now, 0); err != nil || len(due) != 0 {
		t.Errorf("Due after the tail = (%v, %v), want nothing", due, err)
	}
}

// firstCard makes one basic card for a block, which is all this test needs: the
// shapes a paragraph makes are the card plugin's decision, and the store only
// stores what it is told to.
func firstCard(lib *Library, blockID core.ID) (core.Card, error) {
	cards, err := lib.CreateCards(blockID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return cards[0], nil
}

// A frozen card's snapshot is part of what the card shows, so the index carries
// it: a listing read back from the index — the one the window reads, with no
// card file opened — still holds the words the card was frozen at, and a rebuild
// of the index derives them again from the card file.
func TestFrozenSnapshotSurvivesTheIndex(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	lib, err := Create(dir, Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	frozen := card
	frozen.Ref = frozen.Ref.Freeze(lib, now)
	if err := lib.SaveCard(frozen); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	check := func(how string, lib *Library) {
		t.Helper()
		cards := lib.Cards()
		if len(cards) != 1 || cards[0].ID != card.ID {
			t.Fatalf("%s: the listing holds %d cards, want the frozen one", how, len(cards))
		}
		snapshot := cards[0].Ref.Frozen
		if snapshot == nil {
			t.Fatalf("%s: the listing lost the snapshot of %s", how, cards[0].ID)
		}
		if snapshot.Text != "q :: a" || !snapshot.At.Equal(now) {
			t.Errorf("%s: snapshot = %+v, want the block as it was at %v", how, snapshot, now)
		}
	}

	reopened, err := Open(dir, Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if reopened.sources.cards != 0 {
		t.Errorf("the reopen read %d card files, want the index alone", reopened.sources.cards)
	}
	check("after a reopen", reopened)
	if err := reopened.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if err := RebuildIndex(dir, Options{Now: func() time.Time { return now }}); err != nil {
		t.Fatalf("rebuild the index: %v", err)
	}
	rebuilt, err := Open(dir, Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open after the rebuild: %v", err)
	}
	defer rebuilt.Close()
	check("after a rebuild", rebuilt)
}
