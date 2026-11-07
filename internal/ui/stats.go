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

	// heatWeeks is how far the review heatmap looks back: half a year of
	// weeks, the calendar shape decision #11 asks for.
	heatWeeks = 26
	// heatCell is one day's box and heatGap what separates two of them. ui/chart
	// has no heatmap at v0.8.1, so the calendar is a grid of tinted ui.Boxes;
	// the two sizes are the cell grid's, named here rather than in the view.
	// The cell is as tall as the small type's line, so a weekday name beside
	// it sits on its own row.
	heatCell float32 = 14
	heatGap  float32 = 3
)

// dayCount is one day of a series as a plot reads it: the day itself, so the
// axis needs no parse back, and how many reviews or cards it holds.
type dayCount struct {
	day   time.Time
	count int
}

// heatDay is one square of the review heatmap: its count, and whether it is
// still ahead of today and so has no box yet.
type heatDay struct {
	day    time.Time
	count  int
	future bool
}

// statsState is the statistics page's data, derived from the store when the
// page opens and after every write.
type statsState struct {
	heat      [][]heatDay // one column per week, Monday first
	heatMax   int
	heatTotal int
	heatFrom  time.Time
	heatTo    time.Time
	retention stats.Retention
	forecast  []dayCount
}

// loadStats derives the page's datasets: the review heatmap, the retention
// picture and the forecast. It is a load path: the view reads what it left, so
// no frame replays the review log.
func (a *App) loadStats() {
	if a.lib == nil {
		a.stats = statsState{}
		return
	}
	now := time.Now()
	today := startOfDay(now)
	heat, most, total, from, to := a.heatWeeks(today)
	a.stats = statsState{
		heat:      heat,
		heatMax:   most,
		heatTotal: total,
		heatFrom:  from,
		heatTo:    to,
		retention: stats.RetentionOf(a.lib, schedule.DefaultParams()),
		forecast:  dense(stats.Forecast(a.lib, now, statsDays), today),
	}
}

// heatWeeks reads the reviews of the last heatWeeks weeks into calendar
// columns, one per week, and names the busiest day, the total and the days the
// map covers.
func (a *App) heatWeeks(today time.Time) (weeks [][]heatDay, most, total int, from, to time.Time) {
	from = startOfWeek(today).AddDate(0, 0, -7*(heatWeeks-1))
	to = from.AddDate(0, 0, 7*heatWeeks)
	counts := stats.ReviewsPerDay(a.lib, from, to)
	byDay := make(map[string]int, len(counts))
	for _, c := range counts {
		byDay[c.Day] = c.Count
	}

	weeks = make([][]heatDay, heatWeeks)
	for w := range weeks {
		week := make([]heatDay, 7)
		for d := range week {
			day := from.AddDate(0, 0, w*7+d)
			count := byDay[day.Format("2006-01-02")]
			week[d] = heatDay{day: day, count: count, future: day.After(today)}
			most = max(most, count)
			total += count
		}
		weeks[w] = week
	}
	return weeks, most, total, from, today
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

// statsView draws the statistics page: the year's reviews as a heatmap, then
// the retention and forecast plots. A library with nothing in a series says so
// instead of plotting an axis with no values.
func (a *App) statsView(c *ui.Context, tk tokens) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.spaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(statsWidth).Gap(tk.spaceXL).Children(func() {
				ui.Text(c, m(c).StatsTitle()).FontSize(tk.typeTitle).Bold()
				a.heatmapSection(c, tk)
				a.retentionSection(c, tk)
				a.forecastSection(c, tk)
			})
		})
	})
}

// heatmapSection is the review heatmap: one small box per day, weeks as
// columns and weekdays as rows, tinted by how many reviews the day holds. It
// is composed from stock widgets because ui/chart has no heatmap at v0.8.1;
// see RECONCILE.md.
func (a *App) heatmapSection(c *ui.Context, tk tokens) {
	ui.Column(c).Gap(tk.spaceS).Children(func() {
		if a.stats.heatTotal == 0 {
			statsHeading(c, tk, m(c).StatsHeat(), "")
			ui.Text(c, m(c).StatsHeatEmpty()).FontSize(tk.typeBody).TextColor(tk.TextMuted)
			return
		}
		statsHeading(c, tk, m(c).StatsHeat(),
			m(c).StatsHeatRange(a.stats.heatFrom, a.stats.heatTo, a.stats.heatTotal))

		ui.Row(c).Gap(heatGap).AlignItems(ui.Start).Label(m(c).StatsHeat()).Children(func() {
			ui.Column(c).Gap(heatGap).Children(func() {
				for _, day := range a.stats.heat[0] {
					ui.Text(c, m(c).StatsHeatWeekday(day.day)).NoWrap().
						Width(heatWeekdayWidth).Height(heatCell).LineHeight(1).
						FontSize(tk.typeSmall).TextColor(tk.TextMuted)
				}
			})
			for _, week := range a.stats.heat {
				ui.Column(c).Gap(heatGap).Children(func() {
					for _, day := range week {
						a.heatCell(c, tk, day)
					}
				})
			}
		})
		a.heatLegend(c, tk)
	})
}

// heatWeekdayWidth is the column the weekday names get, wide enough for the
// longest name a locale writes at the small size.
const heatWeekdayWidth float32 = 34

// heatCell is one day: a tinted box for a counted day, a spacer for one still
// ahead of today.
func (a *App) heatCell(c *ui.Context, tk tokens, day heatDay) {
	box := ui.Box(c).Width(heatCell).Height(heatCell).Radius(tk.radiusBar)
	switch {
	case day.future:
		return
	case day.count == 0:
		box.Background(tk.sunken)
	default:
		box.Background(tk.heatFill(heatLevel(day.count, a.stats.heatMax)))
		label := m(c).StatsDayCount(day.day, day.count)
		box.Tooltip(label).Label(label)
	}
}

// heatLegend names the two ends of the tint scale, so the boxes read as
// counts rather than decoration.
func (a *App) heatLegend(c *ui.Context, tk tokens) {
	ui.Row(c).Gap(heatGap).AlignItems(ui.Center).Children(func() {
		ui.Text(c, m(c).StatsHeatLess()).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
		for level := 0; level <= 4; level++ {
			ui.Box(c).Width(heatCell).Height(heatCell).Radius(tk.radiusBar).
				Background(tk.heatFill(level))
		}
		ui.Text(c, m(c).StatsHeatMore()).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
	})
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
	return clamp((4*count+most-1)/most, 1, 4)
}

// retentionSection is how often a card was recalled, per range of the chance
// the schedule predicted. A range no review fell in has no bar.
func (a *App) retentionSection(c *ui.Context, tk tokens) {
	r := a.stats.retention
	ui.Column(c).Gap(tk.spaceS).Children(func() {
		if r.Reviews == 0 {
			statsHeading(c, tk, m(c).StatsRetention(), "")
			ui.Text(c, m(c).StatsRetentionEmpty()).FontSize(tk.typeBody).TextColor(tk.TextMuted)
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
					Background(tk.Accent).Radius(tk.radiusBar).
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
			ui.Text(c, m(c).StatsForecastEmpty()).FontSize(tk.typeBody).TextColor(tk.TextMuted)
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
			).Background(tk.Accent).Radius(tk.radiusBar)
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
		ui.Text(c, summary).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
	})
}

// mutedAxis labels a plot's ticks in the small muted type.
func mutedAxis(c *ui.Context, tk tokens) func(chart.Side, chart.Tick) {
	return func(_ chart.Side, tick chart.Tick) {
		ui.Text(c, tick.Label).NoWrap().FontSize(tk.typeSmall).TextColor(tk.TextMuted)
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
