package ui

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
)

const (
	// statsDays is how many days each time series covers: a month back and a
	// month ahead.
	statsDays = 30
	// statsChartHeight is one plot's height; three of them scroll at the
	// smallest window rather than squeeze the rows.
	statsChartHeight float32 = 150
	// statsWidth caps the page's measure, as reviewWidth does review's.
	statsWidth float32 = 760
)

// dayCount is one day of a series as a plot reads it: the day itself, so the
// axis needs no parse back, and how many reviews or cards it holds.
type dayCount struct {
	day   time.Time
	count int
}

// statsState is the statistics page's data, derived from the store when the
// page opens and after every write.
type statsState struct {
	reviews   []dayCount
	retention stats.Retention
	forecast  []dayCount
}

// loadStats derives the page's three datasets. It is a load path: the view
// reads what it left, so no frame replays the review log.
func (a *App) loadStats() {
	if a.lib == nil {
		a.stats = statsState{}
		return
	}
	now := time.Now()
	today := startOfDay(now)
	a.stats = statsState{
		reviews: dense(stats.ReviewsPerDay(a.lib, today.AddDate(0, 0, -(statsDays-1)), today.AddDate(0, 0, 1)),
			today.AddDate(0, 0, -(statsDays-1))),
		retention: stats.RetentionOf(a.lib, schedule.DefaultParams()),
		forecast:  dense(stats.Forecast(a.lib, now, statsDays), today),
	}
}

// startOfDay is midnight of t's own day, where a series starts.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// dense fills the days a sparse count list leaves out with zeros, so a plot
// draws the whole range rather than only the days that have a count.
func dense(counts []stats.DayCount, from time.Time) []dayCount {
	byDay := make(map[string]int, len(counts))
	for _, c := range counts {
		byDay[c.Day] = c.Count
	}
	out := make([]dayCount, statsDays)
	for i := range out {
		day := from.AddDate(0, 0, i)
		out[i] = dayCount{day: day, count: byDay[day.Format("2006-01-02")]}
	}
	return out
}

// statsView draws the statistics page: three plots of what the store's log
// says. A library with nothing in a series says so instead of plotting an axis
// with no values.
func (a *App) statsView(c *ui.Context, tk tokens) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.spaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(statsWidth).Gap(tk.spaceXL).Children(func() {
				ui.Text(c, m(c).StatsTitle()).FontSize(tk.typeTitle).Bold()
				a.reviewsSection(c, tk)
				a.retentionSection(c, tk)
				a.forecastSection(c, tk)
			})
		})
	})
}

// reviewsSection is how many reviews were done on each of the last 30 days.
func (a *App) reviewsSection(c *ui.Context, tk tokens) {
	total := 0
	for _, day := range a.stats.reviews {
		total += day.count
	}
	ui.Column(c).Gap(tk.spaceS).Children(func() {
		if total == 0 {
			statsHeading(c, tk, m(c).StatsReviews(), "")
			ui.Text(c, m(c).StatsReviewsEmpty()).FontSize(tk.typeBody).TextColor(tk.textMuted)
			return
		}
		statsHeading(c, tk, m(c).StatsReviews(), m(c).StatsReviewsRange(total))
		a.dayPlot(c, tk, a.stats.reviews, m(c).StatsReviews(), m(c).StatsDayCount)
	})
}

// retentionSection is how often a card was recalled, per range of the chance
// the schedule predicted. A range no review fell in has no bar.
func (a *App) retentionSection(c *ui.Context, tk tokens) {
	r := a.stats.retention
	ui.Column(c).Gap(tk.spaceS).Children(func() {
		if r.Reviews == 0 {
			statsHeading(c, tk, m(c).StatsRetention(), "")
			ui.Text(c, m(c).StatsRetentionEmpty()).FontSize(tk.typeBody).TextColor(tk.textMuted)
			return
		}
		statsHeading(c, tk, m(c).StatsRetention(),
			m(c).StatsRetentionSummary(m(c).StatsPercent(float64(r.Passes)/float64(r.Reviews)), r.Reviews))

		labels := make([]string, len(r.Buckets))
		for i, b := range r.Buckets {
			labels[i] = m(c).StatsBucket(int(math.Round(b.Low*100)), min(100, int(math.Round(b.High*100))))
		}
		x := chart.NewBand(labels...).Padding(0.3, 0.12)
		y := chart.NewLinear(0, 1)
		y.Format = func(v float64) string { return m(c).StatsPercent(v) }

		axis := mutedAxis(c, tk)
		chart.Plot(c, chart.Axes{Left: y.Ticks(4), Bottom: x.Ticks(), Label: axis}, func(area ui.Element) {
			for i, b := range r.Buckets {
				if b.Reviews == 0 {
					continue
				}
				rate := float64(b.Passes) / float64(b.Reviews)
				chart.Rect(c, x.Band(i), y.Span(0, rate)).
					Background(tk.accent).Radius(tk.radiusControl / 2).
					Tooltip(m(c).StatsBucketReviews(labels[i], b.Reviews))
			}
		}).Height(statsChartHeight).Gap(tk.spaceS).Label(m(c).StatsRetention())
	})
}

// forecastSection is how many cards are due on each of the next 30 days.
func (a *App) forecastSection(c *ui.Context, tk tokens) {
	total := 0
	for _, day := range a.stats.forecast {
		total += day.count
	}
	ui.Column(c).Gap(tk.spaceS).Children(func() {
		if total == 0 {
			statsHeading(c, tk, m(c).StatsForecast(), "")
			ui.Text(c, m(c).StatsForecastEmpty()).FontSize(tk.typeBody).TextColor(tk.textMuted)
			return
		}
		statsHeading(c, tk, m(c).StatsForecast(), cardCount(c, total))
		a.dayPlot(c, tk, a.stats.forecast, m(c).StatsForecast(), m(c).StatsDueCount)
	})
}

// dayPlot draws one bar per day with its count, over a time axis: ui/chart's
// plot, time scale and rect marks, so nothing here paints a pixel. tooltip
// names a day's count for the pointer.
func (a *App) dayPlot(c *ui.Context, tk tokens, days []dayCount, label string, tooltip func(day time.Time, n int) string) {
	from, to := days[0].day, days[len(days)-1].day.AddDate(0, 0, 1)
	x := chart.NewTime(from, to)
	x.Format = func(t time.Time) string { return m(c).StatsDay(t) }

	_, hi := chart.Extent(counts(days)...)
	y := chart.NewLinear(0, max(1, hi))
	y.Format = func(v float64) string { return strconv.Itoa(int(math.Round(v))) }

	axis := mutedAxis(c, tk)
	chart.Plot(c, chart.Axes{Left: wholeTicks(y.Ticks(4)), Bottom: x.Ticks(5), Label: axis}, func(area ui.Element) {
		for _, day := range days {
			span := x.Span(day.day, day.day.AddDate(0, 0, 1))
			inset := (span.Hi - span.Lo) * 0.18
			bar := chart.Rect(c,
				chart.Span{Lo: span.Lo + inset, Hi: span.Hi - inset},
				y.Span(0, float64(day.count)),
			).Background(tk.accent).Radius(tk.radiusControl / 2)
			if day.count > 0 {
				bar.Tooltip(tooltip(day.day, day.count))
			}
		}
	}).Height(statsChartHeight).Gap(tk.spaceS).Label(label)
}

// statsHeading is one section's title and its one-line summary, when it has
// data to summarise.
func statsHeading(c *ui.Context, tk tokens, title, summary string) {
	ui.Row(c).AlignItems(ui.Center).Gap(tk.spaceS).Children(func() {
		ui.Text(c, title).FontSize(tk.typeHeading).Bold()
		if summary == "" {
			return
		}
		ui.Spacer(c)
		ui.Text(c, summary).FontSize(tk.typeSmall).TextColor(tk.textMuted)
	})
}

// mutedAxis labels a plot's ticks in the small muted type.
func mutedAxis(c *ui.Context, tk tokens) func(chart.Side, chart.Tick) {
	return func(_ chart.Side, tick chart.Tick) {
		ui.Text(c, tick.Label).NoWrap().FontSize(tk.typeSmall).TextColor(tk.textMuted)
	}
}

// wholeTicks keeps a count axis on whole numbers: a plot of one or two reviews
// would otherwise label it 0.2, 0.4, ... between the values that can occur.
func wholeTicks(ticks []chart.Tick) []chart.Tick {
	out := ticks[:0]
	for _, tick := range ticks {
		if tick.Value == math.Trunc(tick.Value) {
			out = append(out, tick)
		}
	}
	return out
}

// counts is a series' counts, for the extent of its axis.
func counts(days []dayCount) []float64 {
	out := make([]float64, len(days))
	for i, day := range days {
		out[i] = float64(day.count)
	}
	return out
}
