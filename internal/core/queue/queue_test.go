package queue

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

var now = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

func library(t *testing.T) *store.Library {
	t.Helper()
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return lib
}

func cards(t *testing.T, lib *store.Library, n int) []core.Card {
	t.Helper()
	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: blocksOf(n)})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	out := make([]core.Card, 0, n)
	for _, b := range note.Blocks {
		card, err := firstCard(lib, b.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		out = append(out, card)
	}
	return out
}

func blocksOf(n int) []core.Block {
	out := make([]core.Block, n)
	for i := range out {
		out[i] = core.Block{Text: "q :: a"}
	}
	return out
}

func TestNewCardsComeLastAndOnlySoMany(t *testing.T) {
	lib := library(t)
	all := cards(t, lib, 3)
	// Reviewed yesterday, so today's new-card allowance is untouched.
	if err := lib.Review(all[0].ID, schedule.Good, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}

	got, err := Today(lib, now, Options{NewPerDay: 1})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	// The reviewed card is scheduled into the future, so the queue is the
	// new-card allowance alone.
	if len(got) != 1 {
		t.Fatalf("queue = %d cards, want the one allowed new card", len(got))
	}
	if got[0].ID == all[0].ID {
		t.Error("the reviewed card came back before it was due")
	}
	if state, _ := lib.CardState(got[0].ID); state.Reviews != 0 {
		t.Error("the allowed card is not new")
	}
}

func TestExcludedCardsAreNotInTheQueue(t *testing.T) {
	lib := library(t)
	all := cards(t, lib, 3)

	suspended := all[0]
	suspended.Status = core.StatusSuspended
	if err := lib.SaveCard(suspended); err != nil {
		t.Fatalf("save: %v", err)
	}
	reading := all[1]
	reading.Status = core.StatusReading
	if err := lib.SaveCard(reading); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Today(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	for _, card := range got {
		if card.ID == suspended.ID || card.ID == reading.ID {
			t.Errorf("queue holds a %s card", card.Status)
		}
	}
	if len(got) != 1 {
		t.Errorf("queue = %d cards, want only the scheduled one", len(got))
	}
}

func TestOrderIsStable(t *testing.T) {
	lib := library(t)
	_ = cards(t, lib, 4)

	first, err := Today(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	second, err := Today(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("queue length changed between calls: %d then %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("queue order changed at %d: %s then %s", i, first[i].ID, second[i].ID)
		}
	}
}

func TestCountsMatchTheQueue(t *testing.T) {
	lib := library(t)
	all := cards(t, lib, 3)
	if err := lib.Review(all[0].ID, schedule.Again, now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}

	counts, err := CountsOf(lib, now)
	if err != nil {
		t.Fatalf("CountsOf: %v", err)
	}
	cardsNow, err := Today(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if counts.Remaining != len(cardsNow) {
		t.Errorf("remaining %d, queue %d", counts.Remaining, len(cardsNow))
	}
	if counts.Remaining != counts.Due+counts.Learning+counts.New {
		t.Errorf("counts do not add up: %+v", counts)
	}
}

func TestLimitCapsTheQueue(t *testing.T) {
	lib := library(t)
	_ = cards(t, lib, 5)

	got, err := Today(lib, now, Options{Limit: 2})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("queue = %d cards, want the limit of 2", len(got))
	}
}

func TestNewPerDayCountsWhatStartedToday(t *testing.T) {
	lib := library(t)
	all := cards(t, lib, 3)
	// Two cards were started today, so a per-day allowance of two leaves no
	// room for another new card.
	for _, card := range all[:2] {
		if err := lib.Review(card.ID, schedule.Again, now.Add(-time.Hour)); err != nil {
			t.Fatalf("review: %v", err)
		}
	}

	got, err := Today(lib, now, Options{NewPerDay: 2})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	for _, card := range got {
		if state, ok := lib.CardState(card.ID); !ok || state.Reviews == 0 {
			if card.ID == all[2].ID {
				t.Error("a new card joined the queue after the daily allowance was spent")
			}
		}
	}
}

// firstCard makes one basic card for a block, which is all this test needs: the
// shapes a paragraph makes are the card plugin's decision, and the store only
// stores what it is told to.
func firstCard(lib *store.Library, blockID core.ID) (core.Card, error) {
	cards, err := lib.CreateCards(blockID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return cards[0], nil
}

// readingNotes queues notes at one moment and answers with them in the order
// they were made, so a test names the order the queue should hold.
func readingNotes(t *testing.T, lib *store.Library, at time.Time, titles ...string) []core.Note {
	t.Helper()
	out := make([]core.Note, 0, len(titles))
	for _, title := range titles {
		note, err := lib.SaveNote(core.Note{Title: title, Blocks: []core.Block{{Text: "a paragraph"}}})
		if err != nil {
			t.Fatalf("save note: %v", err)
		}
		if err := lib.AddToReading(note.ID, at); err != nil {
			t.Fatalf("add to reading: %v", err)
		}
		out = append(out, note)
	}
	return out
}

// The reading queue is the notes due now, soonest first: a note never read is
// due when it joined, and a note read comes back on the cadence reading gave
// it. Neither the order nor the counts turn on the store being the same object.
func TestReadingQueueOrder(t *testing.T) {
	lib := library(t)
	notes := readingNotes(t, lib, now.AddDate(0, 0, -10), "alpha", "beta", "gamma")

	// alpha was read four days ago and is due again; beta was read just now and
	// is scheduled ahead; gamma was never read, so it has been due since it
	// joined.
	for _, read := range []struct {
		note   core.Note
		at     time.Time
		rating schedule.Rating
	}{
		{notes[0], now.AddDate(0, 0, -4), schedule.Good},
		{notes[1], now, schedule.Easy},
	} {
		if err := lib.ReadNote(read.note.ID, read.rating, read.note.Blocks[0].ID, read.at); err != nil {
			t.Fatalf("ReadNote: %v", err)
		}
	}

	got, err := Reading(lib, now, Options{})
	if err != nil {
		t.Fatalf("Reading: %v", err)
	}
	order := make([]core.ID, len(got))
	for i, entry := range got {
		order[i] = entry.Note
	}
	if len(order) != 2 || order[0] != notes[2].ID || order[1] != notes[0].ID {
		t.Fatalf("reading queue = %v, want the unread note then the one read four days ago", order)
	}

	count, err := ReadingCount(lib, now)
	if err != nil {
		t.Fatalf("ReadingCount: %v", err)
	}
	if count != len(got) {
		t.Errorf("count = %d, queue = %d", count, len(got))
	}

	limited, err := Reading(lib, now, Options{Limit: 1})
	if err != nil {
		t.Fatalf("Reading with a limit: %v", err)
	}
	if len(limited) != 1 || limited[0].Note != order[0] {
		t.Errorf("limited queue = %+v, want the first entry alone", limited)
	}
}

// A note read today and scheduled for later is not in today's reading queue,
// and not counted either: the queue is a promise of what can be read now.
func TestReadingQueueLeavesWhatIsNotDue(t *testing.T) {
	lib := library(t)
	notes := readingNotes(t, lib, now, "alpha")
	if err := lib.ReadNote(notes[0].ID, schedule.Easy, notes[0].Blocks[0].ID, now); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	got, err := Reading(lib, now.Add(time.Minute), Options{})
	if err != nil {
		t.Fatalf("Reading: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("reading queue = %v, want nothing due a minute after it was read", got)
	}
	count, err := ReadingCount(lib, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ReadingCount: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// When the schedule says it is due, it comes back where it stopped.
	entry, ok := lib.ReadingFor(notes[0].ID)
	if !ok {
		t.Fatal("the read note left the reading queue for good")
	}
	later := entry.State.Due.Add(time.Minute)
	got, err = Reading(lib, later, Options{})
	if err != nil {
		t.Fatalf("Reading when it is due: %v", err)
	}
	if len(got) != 1 || got[0].Note != notes[0].ID {
		t.Fatalf("reading queue = %+v, want the note back", got)
	}
	if got[0].Position != notes[0].Blocks[0].ID {
		t.Errorf("position = %q, want the block it stopped at", got[0].Position)
	}
}
