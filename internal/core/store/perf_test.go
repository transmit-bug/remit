package store_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core/queue"
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

	// fixturePerNote is how many blocks one fixture note holds, which is the
	// shape the storage research measured.
	fixturePerNote = 100
)

// perfCards sets the synthetic library size. Two thousand keeps the test in
// seconds, which is what CI runs on every push; REMIT_PERF_CARDS=100000 runs
// the researched size, and that run is what the scale work must pass.
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
	dir := t.TempDir()

	// A library is created empty, then its sources are written directly: a
	// hundred thousand cards created one card at a time through the store would
	// take longer than the measurement is worth.
	empty, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := empty.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	n := perfCards()
	built := time.Now()
	writeFixture(t, dir, n, now)
	t.Logf("wrote %d cards in %d notes and %d reviews in %s",
		n, (n+fixturePerNote-1)/fixturePerNote, n/4, time.Since(built).Round(time.Millisecond))

	// The first open derives the index from the sources: it reads every note,
	// every card file and the whole log. That is the rebuild path.
	rebuilding := time.Now()
	lib, err := store.Open(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open (rebuild): %v", err)
	}
	rebuildFor := time.Since(rebuilding)
	lib.Close()

	// The second open finds the index current, so it reads no card, no note and
	// no log line: that is the open a person pays for.
	opening := time.Now()
	lib, err = store.Open(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer lib.Close()
	openFor := time.Since(opening)
	t.Logf("open with the index current %s, open that rebuilt it %s",
		openFor.Round(time.Millisecond), rebuildFor.Round(time.Millisecond))
	if openFor >= rebuildFor {
		t.Errorf("an open with a current index took %s, a rebuild %s: it is still reading the sources",
			openFor, rebuildFor)
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

	// The first write after opening pays SQLite's first page writes and a cold
	// fsync, which is a startup cost rather than the steady state a budget
	// describes. Warm up, then take the median of five.
	cold := time.Now()
	if err := lib.Review(due[0].ID, schedule.Good, now); err != nil {
		t.Fatalf("review: %v", err)
	}
	coldFor := time.Since(cold)

	samples := make([]time.Duration, 0, 5)
	for i := 1; i <= 5; i++ {
		start := time.Now()
		if err := lib.Review(due[i].ID, schedule.Good, now); err != nil {
			t.Fatalf("review %d: %v", i, err)
		}
		samples = append(samples, time.Since(start))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	writeFor := samples[len(samples)/2]
	if writeFor > budgetWrite {
		t.Errorf("review write took %s (median of 5), budget %s", writeFor, budgetWrite)
	}

	// The queue and the statistics read the index, not the cards: they are what
	// a write re-derives, so their cost is part of the budget's story.
	today := time.Now()
	cards, err := queue.Today(lib, now, queue.Options{})
	if err != nil {
		t.Fatalf("today: %v", err)
	}
	todayFor := time.Since(today)
	counts := time.Now()
	if _, err := queue.CountsOf(lib, now); err != nil {
		t.Fatalf("counts: %v", err)
	}
	countsFor := time.Since(counts)

	t.Logf("%d cards: due read %s (%d due), review write %s (first write %s), queue %s (%d cards), counts %s",
		n, readFor.Round(time.Microsecond), len(due), writeFor.Round(time.Microsecond),
		coldFor.Round(time.Millisecond), todayFor.Round(time.Millisecond), len(cards), countsFor.Round(time.Millisecond))
}

// writeFixture writes a library's sources without the store: one note file per
// hundred blocks, one card file per block, and a log with every fourth card
// reviewed three days before now, so the queue is neither empty nor trivial.
func writeFixture(t *testing.T, dir string, cards int, now time.Time) {
	t.Helper()
	created := now.UTC().Format(time.RFC3339Nano)
	reviewedAt := now.Add(-72 * time.Hour).UTC().Format(time.RFC3339Nano)

	for start := 0; start < cards; start += fixturePerNote {
		end := min(start+fixturePerNote, cards)
		note := fmt.Sprintf("note_%032x", start/fixturePerNote)
		var body strings.Builder
		body.WriteString("# bulk\n\n")
		for i := start; i < end; i++ {
			fmt.Fprintf(&body, "question :: answer ^%s\n\n", blockID(i))
		}
		writeFixtureFile(t, filepath.Join(dir, "notes", "bulk-"+note+".md"), body.String())

		for i := start; i < end; i++ {
			card := map[string]string{
				"block":   blockID(i),
				"prompt":  "",
				"answer":  "",
				"status":  "new",
				"created": created,
			}
			data, err := json.Marshal(card)
			if err != nil {
				t.Fatalf("card %d: %v", i, err)
			}
			writeFixtureFile(t, filepath.Join(dir, "cards", cardID(i)+".json"), string(append(data, '\n')))
		}
	}

	path := filepath.Join(dir, "reviews.jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	log := bufio.NewWriter(file)
	seq := 0
	for i := 3; i < cards; i += 4 {
		seq++
		fmt.Fprintf(log, `{"seq":%d,"card":%q,"rating":"good","at":%q}`+"\n", seq, cardID(i), reviewedAt)
	}
	if err := log.Flush(); err != nil {
		t.Fatalf("log: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("log: %v", err)
	}
}

func writeFixtureFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", filepath.Base(path), err)
	}
}

func blockID(i int) string { return fmt.Sprintf("block_%032x", i) }

func cardID(i int) string { return fmt.Sprintf("card_%032x", i) }
