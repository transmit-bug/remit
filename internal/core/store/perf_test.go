package store_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// The v1 budget: a due-queue read and a review write stay well inside what a
// person notices, at a library size the storage research measured (#7: 0.3 ms
// and 4 ms at 100k cards). The ceiling is ten times the baseline, so the test
// catches a change of order of magnitude without failing on a slow laptop.
const (
	budgetRead   = 30 * time.Millisecond
	budgetWrite  = 40 * time.Millisecond
	defaultCards = 2000
)

// perfCards sets the synthetic library size. Two thousand keeps the test in
// seconds, which is what CI can afford; REMIT_PERF_CARDS=100000 runs the
// researched size, and that run is what the scale work must pass.
func perfCards() int {
	if v := os.Getenv("REMIT_PERF_CARDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultCards
}

func TestPerformanceBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("the perf budget builds a synthetic library")
	}
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()

	n := perfCards()
	cards := make([]core.ID, 0, n)
	built := time.Now()
	const perNote = 100
	for i := 0; i < n; i += perNote {
		blocks := make([]core.Block, 0, perNote)
		for j := 0; j < perNote && i+j < n; j++ {
			blocks = append(blocks, core.Block{Text: "question :: answer"})
		}
		note, err := lib.SaveNote(core.Note{Title: "bulk", Blocks: blocks})
		if err != nil {
			t.Fatalf("note at %d: %v", i, err)
		}
		for _, b := range note.Blocks {
			card, err := lib.CreateCard(b.ID)
			if err != nil {
				t.Fatalf("card at %d: %v", i, err)
			}
			cards = append(cards, card.ID)
		}
	}
	t.Logf("built %d cards in %s", len(cards), time.Since(built).Round(time.Millisecond))

	// Make every fourth card due, so the queue is not empty and not trivial.
	for i := 3; i < len(cards); i += 4 {
		if err := lib.Review(cards[i], schedule.Good, now.Add(-72*time.Hour)); err != nil {
			t.Fatalf("seed review: %v", err)
		}
	}

	read := time.Now()
	due, err := lib.Due(now, 0)
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	readFor := time.Since(read)
	if readFor > budgetRead {
		t.Errorf("due-queue read of %d cards took %s, budget %s", len(due), readFor, budgetRead)
	}

	write := time.Now()
	if err := lib.Review(cards[0], schedule.Good, now); err != nil {
		t.Fatalf("review: %v", err)
	}
	writeFor := time.Since(write)
	if writeFor > budgetWrite {
		t.Errorf("review write took %s, budget %s", writeFor, budgetWrite)
	}

	t.Logf("%d cards: due read %s (%d due), review write %s", len(cards), readFor, len(due), writeFor)
}
