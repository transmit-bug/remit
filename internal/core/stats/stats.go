// Package stats answers the three questions the statistics views ask: how many
// reviews were done per day, how well recall matched what the schedule
// predicted, and how much work is coming.
package stats

import (
	"sort"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// DayCount is one calendar day's count.
type DayCount struct {
	Day   string
	Count int
}

// live returns the reviews the log still counts: void events and the reviews
// they cancel are both left out.
func live(events []schedule.Event) []schedule.Event {
	voided := map[int64]bool{}
	for _, ev := range events {
		if ev.IsVoid() {
			voided[ev.Void] = true
		}
	}
	out := make([]schedule.Event, 0, len(events))
	for _, ev := range events {
		if ev.IsVoid() || voided[ev.Seq] {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// ReviewsPerDay counts live reviews per day in [from, to).
func ReviewsPerDay(lib *store.Library, from, to time.Time) []DayCount {
	counts := map[string]int{}
	for _, ev := range live(lib.Events()) {
		if ev.At.Before(from) || !ev.At.Before(to) {
			continue
		}
		counts[ev.At.In(from.Location()).Format("2006-01-02")]++
	}
	return sorted(counts)
}

// Bucket is one range of predicted retention and what actually happened.
type Bucket struct {
	Low     float64
	High    float64
	Reviews int
	Passes  int
}

// Retention is the recall picture: how often a card was remembered, grouped by
// how likely the schedule thought it was.
type Retention struct {
	Buckets []Bucket
	Reviews int
	Passes  int
}

// RetentionOf replays the log and compares each review's outcome with the
// forgetting curve's prediction at that moment. A review with no previous
// state has nothing to predict and is skipped; a card answered Again is a
// miss, Hard, Good and Easy are passes.
func RetentionOf(lib *store.Library, params schedule.Params) Retention {
	buckets := []Bucket{{Low: 0, High: 0.7}, {Low: 0.7, High: 0.8}, {Low: 0.8, High: 0.9}, {Low: 0.9, High: 1.01}}
	var out Retention
	out.Buckets = buckets

	applier := schedule.NewApplier(params)
	for _, ev := range live(lib.Events()) {
		prev, had := applier.State(ev.Card)
		if had && prev.Reviews > 0 {
			elapsed := ev.At.Sub(prev.LastReview).Hours() / 24
			predicted := params.PredictedRetention(elapsed, prev.Memory.Stability)
			pass := ev.Rating != schedule.Again
			for i := range out.Buckets {
				if predicted >= out.Buckets[i].Low && predicted < out.Buckets[i].High {
					out.Buckets[i].Reviews++
					if pass {
						out.Buckets[i].Passes++
					}
					break
				}
			}
			out.Reviews++
			if pass {
				out.Passes++
			}
		}
		if err := applier.Apply(ev); err != nil {
			continue
		}
	}
	return out
}

// Forecast counts cards due on each of the next days, starting at from.
func Forecast(lib *store.Library, from time.Time, days int) []DayCount {
	counts := map[string]int{}
	day := func(t time.Time) string { return t.In(from.Location()).Format("2006-01-02") }
	for _, card := range lib.Cards() {
		if !card.Status.Scheduled() {
			continue
		}
		if _, link := card.Ref.Text(lib); link == core.LinkLost {
			continue
		}
		state, ok := lib.CardState(card.ID)
		if !ok || state.Reviews == 0 || state.Due.Before(from) {
			continue
		}
		offset := int(state.Due.In(from.Location()).Truncate(24*time.Hour).Sub(from.Truncate(24*time.Hour)).Hours() / 24)
		if offset < 0 || offset >= days {
			continue
		}
		counts[day(state.Due)]++
	}
	return sorted(counts)
}

func sorted(counts map[string]int) []DayCount {
	out := make([]DayCount, 0, len(counts))
	for day, n := range counts {
		out = append(out, DayCount{Day: day, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}
