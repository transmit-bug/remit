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

// isReview reports whether an event is a review — a card answered or a note read
// — rather than a change to the reading queue's membership, which is a record of
// intent and not of memory work.
func isReview(ev schedule.Event) bool { return ev.Kind == schedule.Reviewed }

// ReviewsPerDay counts live reviews per day in [from, to). It reads the reviews
// in that range out of the log, so its cost follows the range, not the library.
// A reading session counts here as a card's answer does: one log, one picture.
func ReviewsPerDay(lib *store.Library, from, to time.Time) []DayCount {
	events, err := lib.LiveReviews(from, to)
	if err != nil {
		return nil // a log that cannot be read has no reviews to count
	}
	counts := map[string]int{}
	for _, ev := range events {
		if !isReview(ev) {
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

	events, err := lib.LiveReviews(time.Time{}, time.Time{})
	if err != nil {
		return out // a log that cannot be read predicts nothing
	}

	applier := schedule.NewApplier(params)
	for _, ev := range events {
		if !isReview(ev) {
			continue
		}
		prev, had := applier.State(ev.Entry())
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

// Summary is what a finished queue is worth reporting: how many cards were
// reviewed today, how many come due tomorrow, and how many of today's answers
// were right. Zero reviews leaves Accuracy at zero rather than dividing by it.
type Summary struct {
	Reviewed    int
	DueTomorrow int
	Accuracy    float64
}

// Summarize reads today's live reviews out of the log and the next day out of
// the schedule. The day is the calendar day of now, in now's location.
func Summarize(lib *store.Library, now time.Time) Summary {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)

	var out Summary
	if events, err := lib.LiveReviews(start, end); err == nil {
		pass := 0
		for _, ev := range events {
			if !isReview(ev) {
				continue
			}
			out.Reviewed++
			if ev.Rating != schedule.Again {
				pass++
			}
		}
		if out.Reviewed > 0 {
			out.Accuracy = float64(pass) / float64(out.Reviewed)
		}
	}
	if next := Forecast(lib, end, 1); len(next) > 0 {
		out.DueTomorrow = next[0].Count
	}
	return out
}

// ForecastAll is what the statistics page plots: the cards due on each of the
// next days and the reading queue's due notes on the same series, because a
// memory's schedule is one picture whichever way it was made.
func ForecastAll(lib *store.Library, from time.Time, days int) []DayCount {
	counts := map[string]int{}
	for _, day := range Forecast(lib, from, days) {
		counts[day.Day] += day.Count
	}
	for _, day := range ReadingForecast(lib, from, days) {
		counts[day.Day] += day.Count
	}
	return sorted(counts)
}

// ReadingForecast counts the reading queue's notes due on each of the next
// days, starting at from. A note that was never read is due now, not later, so
// it never lands in a forecast of the days ahead.
func ReadingForecast(lib *store.Library, from time.Time, days int) []DayCount {
	counts := map[string]int{}
	day := func(t time.Time) string { return t.In(from.Location()).Format("2006-01-02") }
	queued, err := lib.ReadingQueue()
	if err != nil {
		return nil
	}
	for _, entry := range queued {
		if entry.State.Reviews == 0 || entry.State.Due.Before(from) {
			continue
		}
		offset := int(entry.State.Due.In(from.Location()).Truncate(24*time.Hour).Sub(from.Truncate(24*time.Hour)).Hours() / 24)
		if offset < 0 || offset >= days {
			continue
		}
		counts[day(entry.State.Due)]++
	}
	return sorted(counts)
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
