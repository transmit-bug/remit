package recall

import (
	"time"

	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// heatWeeks is how many weeks the page's strip covers: half a year, which is
// long enough to read a rhythm and short enough to fit beside the rows.
const heatWeeks = 26

// HeatRange is the span the strip covers: the heatWeeks weeks up to and
// including the one day falls in, from the Monday the first column starts on.
func HeatRange(day time.Time) (from, to time.Time) {
	from = startOfWeek(day).AddDate(0, 0, -7*(heatWeeks-1))
	return from, from.AddDate(0, 0, 7*heatWeeks)
}

// HeatWeeks folds the log's per-day counts into the strip's calendar columns,
// one per week, and names the busiest day — the scale the squares are tinted on.
// A day ahead of the one the queue was built for is Future and draws nothing.
func HeatWeeks(day time.Time, reviews []stats.DayCount) (weeks [][]kit.HeatDay, most int) {
	byDay := make(map[string]int, len(reviews))
	for _, count := range reviews {
		byDay[count.Day] = count.Count
	}
	from, _ := HeatRange(day)
	today := startOfDay(day)
	weeks = make([][]kit.HeatDay, heatWeeks)
	for w := range weeks {
		week := make([]kit.HeatDay, 7)
		for d := range week {
			at := from.AddDate(0, 0, w*7+d)
			count := byDay[at.Format("2006-01-02")]
			week[d] = kit.HeatDay{Day: at, Count: count, Future: at.After(today)}
			most = max(most, count)
		}
		weeks[w] = week
	}
	return weeks, most
}

// startOfWeek is midnight of the Monday of t's week, where a column starts.
func startOfWeek(t time.Time) time.Time {
	t = startOfDay(t)
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
}

// startOfDay is midnight of t's own day, where the strip's days are counted.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
