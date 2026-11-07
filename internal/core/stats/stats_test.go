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
		card, err := firstCard(lib, b.ID)
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

// The day's summary counts today's answers, how many came out right, and what
// is due tomorrow — from the log and the schedule, not from anything the view
// keeps.
func TestSummarize(t *testing.T) {
	lib, cards := library(t)
	// Three answers today: two right, one Again; and one answer yesterday,
	// which is not today's.
	for _, review := range []struct {
		card core.ID
		r    schedule.Rating
		at   time.Time
	}{
		{cards[0].ID, schedule.Good, day0},
		{cards[1].ID, schedule.Hard, day0.Add(2 * time.Hour)},
		{cards[0].ID, schedule.Again, day0.Add(4 * time.Hour)},
		{cards[1].ID, schedule.Good, day0.Add(-24 * time.Hour)},
	} {
		if err := lib.Review(review.card, review.r, review.at); err != nil {
			t.Fatalf("review: %v", err)
		}
	}

	got := Summarize(lib, day0.Add(6*time.Hour))
	if got.Reviewed != 3 {
		t.Errorf("counted %d reviews today, want 3", got.Reviewed)
	}
	if want := 2.0 / 3.0; got.Accuracy < want-0.001 || got.Accuracy > want+0.001 {
		t.Errorf("accuracy is %v, want %v", got.Accuracy, want)
	}
	// Due tomorrow, counted here from the schedule rather than from the
	// summary's own path.
	day := func(t time.Time) string { return t.In(time.UTC).Format("2006-01-02") }
	tomorrow := day0.AddDate(0, 0, 1)
	want := 0
	scheduled, err := lib.Scheduled()
	if err != nil {
		t.Fatalf("scheduled: %v", err)
	}
	for _, s := range scheduled {
		if day(s.State.Due) == day(tomorrow) {
			want++
		}
	}
	if got.DueTomorrow != want {
		t.Errorf("%d cards due tomorrow, want %d", got.DueTomorrow, want)
	}

	// A day with no answers divides by nothing.
	quiet := Summarize(lib, day0.AddDate(0, 0, 9))
	if quiet.Reviewed != 0 || quiet.Accuracy != 0 {
		t.Errorf("a day with no reviews reports %+v", quiet)
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

// Reading is the same kind of memory activity: a session counts on the heatmap
// and its due note lands in the forecast beside the cards.
func TestReadingJoinsTheHeatmapAndForecast(t *testing.T) {
	lib, cards := library(t)
	note, err := lib.SaveNote(core.Note{Title: "long read", Blocks: []core.Block{{Text: "a paragraph"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if err := lib.AddToReading(note.ID, day0); err != nil {
		t.Fatalf("add to reading: %v", err)
	}
	if err := lib.Review(cards[0].ID, schedule.Good, day0); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, day0.Add(time.Hour)); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}

	// Two activities on the first day: one card answered, one note read.
	counts := ReviewsPerDay(lib, day0, day0.Add(24*time.Hour))
	if len(counts) != 1 || counts[0].Count != 2 {
		t.Fatalf("heatmap = %+v, want the card and the reading on one day", counts)
	}

	// Both come due later, on the same series.
	cardDue, _ := lib.CardState(cards[0].ID)
	entry, _ := lib.ReadingFor(note.ID)
	all := ForecastAll(lib, day0.Add(24*time.Hour), 400)
	sum := 0
	for _, day := range all {
		sum += day.Count
	}
	if sum != 2 {
		t.Fatalf("forecast = %+v, want the card and the note", all)
	}
	// The card's own forecast stays the review session's question.
	if got := Forecast(lib, day0.Add(24*time.Hour), 400); len(got) != 1 || got[0].Day != cardDue.Due.Format("2006-01-02") {
		t.Errorf("card forecast = %+v, want only the card", got)
	}
	reading := ReadingForecast(lib, day0.Add(24*time.Hour), 400)
	if len(reading) != 1 || reading[0].Day != entry.State.Due.Format("2006-01-02") {
		t.Errorf("reading forecast = %+v, want only the note", reading)
	}
}
