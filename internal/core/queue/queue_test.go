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
	cards, err := lib.CreateCards(blockID, []core.Derived{{Kind: core.KindBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return cards[0], nil
}
