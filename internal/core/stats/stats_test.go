package stats

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

var day0 = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func library(t *testing.T) (*store.Library, []core.Card) {
	t.Helper()
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return day0 }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}, {Text: "q2 :: a2"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	var out []core.Card
	for _, b := range note.Blocks {
		card, err := lib.CreateCard(b.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		out = append(out, card)
	}
	return lib, out
}

func TestReviewsPerDay(t *testing.T) {
	lib, cards := library(t)
	for i, at := range []time.Time{day0, day0.Add(2 * time.Hour), day0.Add(24 * time.Hour)} {
		if err := lib.Review(cards[i%2].ID, schedule.Good, at); err != nil {
			t.Fatalf("review: %v", err)
		}
	}

	got := ReviewsPerDay(lib, day0, day0.Add(48*time.Hour))
	want := []DayCount{{Day: "2026-11-01", Count: 2}, {Day: "2026-11-02", Count: 1}}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestVoidedReviewsAreNotCounted(t *testing.T) {
	lib, cards := library(t)
	if err := lib.Review(cards[0].ID, schedule.Good, day0); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.UndoLast(cards[0].ID); err != nil {
		t.Fatalf("undo: %v", err)
	}

	if got := ReviewsPerDay(lib, day0, day0.Add(24*time.Hour)); len(got) != 0 {
		t.Errorf("an undone review was counted: %+v", got)
	}
	if r := RetentionOf(lib, schedule.DefaultParams()); r.Reviews != 0 {
		t.Errorf("an undone review entered retention: %+v", r)
	}
}

func TestRetentionCountsPassesAndMisses(t *testing.T) {
	lib, cards := library(t)
	if err := lib.Review(cards[0].ID, schedule.Good, day0); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.Review(cards[0].ID, schedule.Again, day0.Add(24*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}

	got := RetentionOf(lib, schedule.DefaultParams())
	if got.Reviews != 1 || got.Passes != 0 {
		t.Errorf("retention = %+v, want one review, no pass (the first has no prediction)", got)
	}
}

func TestForecastBucketsDueCards(t *testing.T) {
	lib, cards := library(t)
	if err := lib.Review(cards[0].ID, schedule.Good, day0); err != nil {
		t.Fatalf("review: %v", err)
	}
	state, ok := lib.CardState(cards[0].ID)
	if !ok || !state.Due.After(day0) {
		t.Fatalf("no due date after a review: %+v", state)
	}

	got := Forecast(lib, day0, 400)
	if len(got) != 1 || got[0].Count != 1 {
		t.Fatalf("forecast = %+v, want the one reviewed card", got)
	}
	if got[0].Day != state.Due.Format("2006-01-02") {
		t.Errorf("forecast day %s, want %s", got[0].Day, state.Due.Format("2006-01-02"))
	}
}
