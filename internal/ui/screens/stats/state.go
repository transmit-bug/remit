package stats

import (
	"time"

	corestats "github.com/transmit-bug/remit/internal/core/stats"
)

const (
	// ForecastDays is how many days ahead the forecast covers.
	ForecastDays = 30
	// heatWeeks is how far the review heatmap looks back: half a year of
	// weeks, the calendar shape decision #11 asks for.
	heatWeeks = 26
)

// DayCount is one day of a series as a plot reads it: the day itself, so the
// axis needs no parse back, and how many reviews or cards it holds.
type DayCount struct {
	Day   time.Time
	Count int
}

// HeatDay is one square of the review heatmap: its count, and whether it is
// still ahead of today and so has no box yet.
type HeatDay struct {
	Day    time.Time
	Count  int
	Future bool
}

// State is the statistics page's data, derived from the store when the page
// opens and after every write.
type State struct {
	Heat      [][]HeatDay // one column per week, Monday first
	HeatMax   int
	HeatTotal int
	HeatFrom  time.Time
	HeatTo    time.Time
	Retention corestats.Retention
	Forecast  []DayCount
}

// HeatRange is the span the heatmap covers: the heatWeeks weeks up to and
// including the one today falls in. The app asks for it before loading the
// reviews, so the page's boxes and its counts cover the same days.
func HeatRange(now time.Time) (from, to time.Time) {
	from = startOfWeek(now).AddDate(0, 0, -7*(heatWeeks-1))
	return from, from.AddDate(0, 0, 7*heatWeeks)
}

// Build derives the page's data from what the app loaded: today, the reviews
// per day in HeatRange, the retention picture and the sparse forecast. It reads
// no store and no clock, so a test hands it a day and the counts.
func Build(now time.Time, reviews []corestats.DayCount, retention corestats.Retention, forecast []corestats.DayCount) State {
	today := startOfDay(now)
	from, _ := HeatRange(now)
	heat, most, total := heatColumns(today, from, reviews)
	return State{
		Heat:      heat,
		HeatMax:   most,
		HeatTotal: total,
		HeatFrom:  from,
		HeatTo:    today,
		Retention: retention,
		Forecast:  dense(forecast, today),
	}
}

// heatColumns reads the reviews of the last heatWeeks weeks into calendar
// columns, one per week, and names the busiest day and the total. from is the
// first day the map covers, as HeatRange names it.
func heatColumns(today, from time.Time, reviews []corestats.DayCount) (weeks [][]HeatDay, most, total int) {
	byDay := make(map[string]int, len(reviews))
	for _, c := range reviews {
		byDay[c.Day] = c.Count
	}

	weeks = make([][]HeatDay, heatWeeks)
	for w := range weeks {
		week := make([]HeatDay, 7)
		for d := range week {
			day := from.AddDate(0, 0, w*7+d)
			count := byDay[day.Format("2006-01-02")]
			week[d] = HeatDay{Day: day, Count: count, Future: day.After(today)}
			most = max(most, count)
			total += count
		}
		weeks[w] = week
	}
	return weeks, most, total
}

// startOfWeek is midnight of the Monday of t's week, where a heatmap column
// starts.
func startOfWeek(t time.Time) time.Time {
	t = startOfDay(t)
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}

// startOfDay is midnight of t's own day, where a series starts.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// dense fills the days a sparse count list leaves out with zeros, so a plot
// draws the whole range rather than only the days that have a count.
func dense(counts []corestats.DayCount, from time.Time) []DayCount {
	byDay := make(map[string]int, len(counts))
	for _, c := range counts {
		byDay[c.Day] = c.Count
	}
	out := make([]DayCount, ForecastDays)
	for i := range out {
		day := from.AddDate(0, 0, i)
		out[i] = DayCount{Day: day, Count: byDay[day.Format("2006-01-02")]}
	}
	return out
}

// heatLevel puts a day's count on the legend's four steps: any review is at
// least one, and the busiest day in the map is four.
func heatLevel(count, most int) int {
	if count <= 0 {
		return 0
	}
	if most <= 0 {
		return 1
	}
	return min(max((4*count+most-1)/most, 1), 4)
}
