package stats

import (
	"time"

	"github.com/transmit-bug/remit/internal/core/schedule"
	corestats "github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

const (
	// ForecastDays is how many days ahead the forecast covers.
	ForecastDays = 30
	// OverviewForecastDays is how many of them the overview shows: two weeks
	// answers "what is coming", and the month has a page of its own.
	OverviewForecastDays = 14
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

// Today is what the overview says about the day itself: the cards due now, the
// notes waiting to be read, and the cards the log says were answered today.
type Today struct {
	Due      int
	Reading  int
	Reviewed int
}

// Trend is one week of measured retention: the reviews the live log holds for
// it and how many of them were remembered. A week with no review keeps its
// place in the series with a zero count, so the line breaks rather than
// drawing a week that measured nothing.
type Trend struct {
	Week    time.Time
	Reviews int
	Passes  int
}

// Rate is how much of the week was remembered; a week with no review has no
// rate, and the plot leaves a gap there.
func (t Trend) Rate() float64 {
	if t.Reviews == 0 {
		return 0
	}
	return float64(t.Passes) / float64(t.Reviews)
}

// State is the statistics area's data, derived from the store when a page is
// opened and after every write. Every page reads it and none of them touches
// the store, so a frame is a draw and never a query.
type State struct {
	// Heat is the review calendar, one column per week, Monday first; HeatMax is
	// the busiest day's count, which the tint scale is read against.
	Heat      [][]kit.HeatDay
	HeatMax   int
	HeatTotal int
	HeatFrom  time.Time
	HeatTo    time.Time
	// Streak is the longest run of days with a review in the calendar.
	Streak int

	// Retention is the recall picture over the whole live log, Target the
	// chance this library's presets aim at, and Trend the same measure week by
	// week over the calendar.
	Retention corestats.Retention
	Target    float64
	Trend     []Trend

	Production corestats.Production
	TimeShare  corestats.TimeShare
	Today      Today

	// Forecast is one entry per day from today, dense: a day with nothing due
	// is a zero and not a hole in the plot.
	Forecast []DayCount
}

// Input is what the app loads for the area: one store visit's worth of
// answers, handed over before the frame that draws them.
type Input struct {
	Now time.Time
	// Reviews is the calendar's per-day counts, in HeatRange.
	Reviews []corestats.DayCount
	// History is the live log's reviews in HeatRange, which the trend reads.
	History    []corestats.HistoryEvent
	Retention  corestats.Retention
	Target     float64
	Forecast   []corestats.DayCount
	Production corestats.Production
	TimeShare  corestats.TimeShare
	// Today's numbers: cards due now, notes waiting to be read, and cards
	// answered today.
	Due      int
	Reading  int
	Reviewed int
}

// Build derives the area's data from what the app loaded: today, the reviews
// per day in HeatRange, the same range's history, and the numbers the three
// pages report. It reads no store and no clock, so a test hands it a day and
// the counts.
func Build(in Input) State {
	today := startOfDay(in.Now)
	from, _ := HeatRange(in.Now)
	heat, most, total, streak := heatColumns(today, from, in.Reviews)
	return State{
		Heat:       heat,
		HeatMax:    most,
		HeatTotal:  total,
		HeatFrom:   from,
		HeatTo:     today,
		Streak:     streak,
		Retention:  in.Retention,
		Target:     in.Target,
		Trend:      trend(in.History, from, in.Now.Location()),
		Production: in.Production,
		TimeShare:  in.TimeShare,
		Today:      Today{Due: in.Due, Reading: in.Reading, Reviewed: in.Reviewed},
		Forecast:   dense(in.Forecast, today),
	}
}

// Rate is the share of reviews that were remembered, and whether there is a
// review to divide by: a page with none says so instead of printing 0%.
func (s State) Rate() (float64, bool) {
	if s.Retention.Reviews == 0 {
		return 0, false
	}
	return float64(s.Retention.Passes) / float64(s.Retention.Reviews), true
}

// Peak is the busiest day of the forecast and its count; false when every day
// of it is empty.
func (s State) Peak() (DayCount, bool) {
	var peak DayCount
	for _, day := range s.Forecast {
		if day.Count > peak.Count {
			peak = day
		}
	}
	return peak, peak.Count > 0
}

// HeatRange is the span the heatmap covers: the heatWeeks weeks up to and
// including the one today falls in. The app asks for it before loading the
// reviews, so the page's boxes and its counts cover the same days.
func HeatRange(now time.Time) (from, to time.Time) {
	from = startOfWeek(now).AddDate(0, 0, -7*(heatWeeks-1))
	return from, from.AddDate(0, 0, 7*heatWeeks)
}

// heatColumns reads the reviews of the last heatWeeks weeks into calendar
// columns, one per week, and names the busiest day, the total and the longest
// run of days with a review. from is the first day the map covers, as
// HeatRange names it.
func heatColumns(today, from time.Time, reviews []corestats.DayCount) (weeks [][]kit.HeatDay, most, total, streak int) {
	byDay := make(map[string]int, len(reviews))
	for _, c := range reviews {
		byDay[c.Day] = c.Count
	}

	run := 0
	weeks = make([][]kit.HeatDay, heatWeeks)
	for w := range weeks {
		week := make([]kit.HeatDay, 7)
		for d := range week {
			day := from.AddDate(0, 0, w*7+d)
			count := byDay[day.Format("2006-01-02")]
			week[d] = kit.HeatDay{Day: day, Count: count, Future: day.After(today)}
			most = max(most, count)
			total += count
			if count > 0 {
				run++
				streak = max(streak, run)
			} else {
				run = 0
			}
		}
		weeks[w] = week
	}
	return weeks, most, total, streak
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

// trend divides the range's reviews into weeks, one entry per week from the
// calendar's first Monday: a week the log has nothing for keeps its place with
// a zero count, so the plot leaves a gap where nothing was measured.
func trend(history []corestats.HistoryEvent, from time.Time, loc *time.Location) []Trend {
	index := make(map[string]int, heatWeeks)
	out := make([]Trend, heatWeeks)
	for w := range out {
		week := from.AddDate(0, 0, w*7)
		out[w].Week = week
		index[week.Format("2006-01-02")] = w
	}
	for _, ev := range history {
		w, ok := index[startOfWeek(ev.At.In(loc)).Format("2006-01-02")]
		if !ok {
			continue
		}
		out[w].Reviews++
		if ev.Rating != schedule.Again {
			out[w].Passes++
		}
	}
	return out
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
