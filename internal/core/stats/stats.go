// Package stats answers the three questions the statistics views ask: how many
// reviews were done per day, how well recall matched what the schedule
// predicted, and how much work is coming.
package stats

import (
	"sort"
	"time"

	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// DayCount is one calendar day's count.
type DayCount struct {
	Day   string
	Count int
}

// ReviewsPerDay counts live reviews per day in [from, to). It reads the reviews
// in that range out of the log, so its cost follows the range, not the library.
func ReviewsPerDay(lib *store.Library, from, to time.Time) []DayCount {
	events, err := lib.LiveReviews(from, to)
	if err != nil {
		return nil // a log that cannot be read has no reviews to count
	}
	counts := map[string]int{}
	for _, ev := range events {
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

	events, err := lib.LiveReviews(time.Time{}, time.Time{})
	if err != nil {
		return out // a log that cannot be read predicts nothing
	}

	applier := schedule.NewApplier(params)
	for _, ev := range events {
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

// Forecast counts cards due on each of the next days, starting at from. It asks
// the store for the cards a queue can hold, with the schedule each one has, so
// no card file is opened and no log line replayed.
func Forecast(lib *store.Library, from time.Time, days int) []DayCount {
	counts := map[string]int{}
	day := func(t time.Time) string { return t.In(from.Location()).Format("2006-01-02") }
	scheduled, err := lib.Scheduled()
	if err != nil {
		return nil
	}
	for _, card := range scheduled {
		state := card.State
		if state.Reviews == 0 || state.Due.Before(from) {
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
