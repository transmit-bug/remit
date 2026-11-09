// Package stats is the statistics page: the reviews of the past half year as a
// heatmap, how well recall matched what the schedule predicted, and what the
// next month holds. Every number on it is loaded before the frame draws, so the
// page reads State and never touches the store.
package stats

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

const (
	// chartHeight is one plot's height; three of them scroll at the smallest
	// window rather than squeeze the rows.
	chartHeight float32 = 150
	// heatCell is one day's box and heatGap what separates two of them. ui/chart
	// has no heatmap at v0.8.1, so the calendar is a grid of tinted ui.Boxes;
	// the two sizes are the cell grid's, named here rather than in the view.
	// The cell is as tall as the small type's line, so a weekday name beside
	// it sits on its own row.
	heatCell float32 = 14
	heatGap  float32 = 3
	// heatWeekdayWidth is the column the weekday names get, wide enough for the
	// longest name a locale writes at the small size.
	heatWeekdayWidth float32 = 34
)

// Deps is what the page asks of the app, which is nothing: the heatmap, the
// retention picture and the forecast are all loaded before the page is drawn.
type Deps struct{}

// View draws the statistics page: the year's reviews as a heatmap, then the
// retention and forecast plots. A series with nothing in it says so instead of
// plotting an axis with no values.
func View(c *ui.Context, s State, _ Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.PaperWidth).Gap(tk.SpaceXL).Children(func() {
				ui.Text(c, msgs.StatsTitle()).FontSize(tk.TypeTitle).Bold()
				heatmapSection(c, s, msgs, tk)
				retentionSection(c, s, msgs, tk)
				forecastSection(c, s, msgs, tk)
			})
		})
	})
}

// heatmapSection is the review heatmap: one small box per day, weeks as
// columns and weekdays as rows, tinted by how many reviews the day holds. It
// is composed from stock widgets because ui/chart has no heatmap at v0.8.1;
// see RECONCILE.md.
func heatmapSection(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		if s.HeatTotal == 0 {
			statsHeading(c, tk, msgs.StatsHeat(), "")
			ui.Text(c, msgs.StatsHeatEmpty()).FontSize(tk.TypeBody).TextColor(tk.TextMuted)
			return
		}
		statsHeading(c, tk, msgs.StatsHeat(),
			msgs.StatsHeatRange(s.HeatFrom, s.HeatTo, s.HeatTotal))

		ui.Row(c).Gap(heatGap).AlignItems(ui.Start).Label(msgs.StatsHeat()).Children(func() {
			ui.Column(c).Gap(heatGap).Children(func() {
				for _, day := range s.Heat[0] {
					ui.Text(c, msgs.StatsHeatWeekday(day.Day)).NoWrap().
						Width(heatWeekdayWidth).Height(heatCell).LineHeight(1).
						FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
				}
			})
			for _, week := range s.Heat {
				ui.Column(c).Gap(heatGap).Children(func() {
					for _, day := range week {
						heatCellBox(c, tk, msgs, day, s.HeatMax)
					}
				})
			}
		})
		heatLegend(c, tk, msgs)
	})
}

// heatCellBox is one day: a tinted box for a counted day, a spacer for one
// still ahead of today.
func heatCellBox(c *ui.Context, tk design.Tokens, msgs locales.Messages, day HeatDay, most int) {
	box := ui.Box(c).Width(heatCell).Height(heatCell).Radius(tk.RadiusBar)
	switch {
	case day.Future:
		return
	case day.Count == 0:
		box.Background(tk.Sunken)
	default:
		box.Background(tk.HeatFill(heatLevel(day.Count, most)))
		label := msgs.StatsDayCount(day.Day, day.Count)
		box.Tooltip(label).Label(label)
	}
}

// heatLegend names the two ends of the tint scale, so the boxes read as
// counts rather than decoration.
func heatLegend(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	ui.Row(c).Gap(heatGap).AlignItems(ui.Center).Children(func() {
		ui.Text(c, msgs.StatsHeatLess()).FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
		for level := 0; level <= 4; level++ {
			ui.Box(c).Width(heatCell).Height(heatCell).Radius(tk.RadiusBar).
				Background(tk.HeatFill(level))
		}
		ui.Text(c, msgs.StatsHeatMore()).FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
	})
}

// retentionSection is how often a card was recalled, per range of the chance
// the schedule predicted. A range no review fell in has no bar.
func retentionSection(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	r := s.Retention
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		if r.Reviews == 0 {
			statsHeading(c, tk, msgs.StatsRetention(), "")
			ui.Text(c, msgs.StatsRetentionEmpty()).FontSize(tk.TypeBody).TextColor(tk.TextMuted)
			return
		}
		statsHeading(c, tk, msgs.StatsRetention(),
			msgs.StatsRetentionSummary(msgs.StatsPercent(float64(r.Passes)/float64(r.Reviews)), r.Reviews))

		labels := make([]string, len(r.Buckets))
		for i, b := range r.Buckets {
			labels[i] = msgs.StatsBucket(int(math.Round(b.Low*100)), min(100, int(math.Round(b.High*100))))
		}
		x := chart.NewBand(labels...).Padding(0.3, 0.12)
		y := chart.NewLinear(0, 1)
		y.Format = func(v float64) string { return msgs.StatsPercent(v) }

		axis := mutedAxis(c, tk)
		chart.Plot(c, chart.Axes{Left: y.Ticks(4), Bottom: x.Ticks(), Label: axis}, func(area ui.Element) {
			for i, b := range r.Buckets {
				if b.Reviews == 0 {
					continue
				}
				rate := float64(b.Passes) / float64(b.Reviews)
				chart.Rect(c, x.Band(i), y.Span(0, rate)).
					Background(tk.Accent).Radius(tk.RadiusBar).
					Tooltip(msgs.StatsBucketReviews(labels[i], b.Reviews))
			}
		}).Height(chartHeight).Gap(tk.SpaceS).Label(msgs.StatsRetention())
	})
}

// forecastSection is how many cards are due on each of the next 30 days.
func forecastSection(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	total := 0
	for _, day := range s.Forecast {
		total += day.Count
	}
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		if total == 0 {
			statsHeading(c, tk, msgs.StatsForecast(), "")
			ui.Text(c, msgs.StatsForecastEmpty()).FontSize(tk.TypeBody).TextColor(tk.TextMuted)
			return
		}
		statsHeading(c, tk, msgs.StatsForecast(), msgs.CardCount(total))
		dayPlot(c, tk, msgs, s.Forecast, msgs.StatsForecast(), msgs.StatsDueCount)
	})
}

// dayPlot draws one bar per day with its count, over a time axis: ui/chart's
// plot, time scale and rect marks, so nothing here paints a pixel. tooltip
// names a day's count for the pointer.
func dayPlot(c *ui.Context, tk design.Tokens, msgs locales.Messages, days []DayCount, label string, tooltip func(day time.Time, n int) string) {
	from, to := days[0].Day, days[len(days)-1].Day.AddDate(0, 0, 1)
	x := chart.NewTime(from, to)
	x.Format = func(t time.Time) string { return msgs.StatsDay(t) }

	_, hi := chart.Extent(counts(days)...)
	y := chart.NewLinear(0, max(1, hi))
	y.Format = func(v float64) string { return strconv.Itoa(int(math.Round(v))) }

	axis := mutedAxis(c, tk)
	chart.Plot(c, chart.Axes{Left: wholeTicks(y.Ticks(4)), Bottom: x.Ticks(5), Label: axis}, func(area ui.Element) {
		for _, day := range days {
			span := x.Span(day.Day, day.Day.AddDate(0, 0, 1))
			inset := (span.Hi - span.Lo) * 0.18
			bar := chart.Rect(c,
				chart.Span{Lo: span.Lo + inset, Hi: span.Hi - inset},
				y.Span(0, float64(day.Count)),
			).Background(tk.Accent).Radius(tk.RadiusBar)
			if day.Count > 0 {
				bar.Tooltip(tooltip(day.Day, day.Count))
			}
		}
	}).Height(chartHeight).Gap(tk.SpaceS).Label(label)
}

// statsHeading is one section's title and its one-line summary, when it has
// data to summarise.
func statsHeading(c *ui.Context, tk design.Tokens, title, summary string) {
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, title).FontSize(tk.TypeHeading).Bold()
		if summary == "" {
			return
		}
		ui.Spacer(c)
		ui.Text(c, summary).FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
	})
}

// mutedAxis labels a plot's ticks in the small muted type.
func mutedAxis(c *ui.Context, tk design.Tokens) func(chart.Side, chart.Tick) {
	return func(_ chart.Side, tick chart.Tick) {
		ui.Text(c, tick.Label).NoWrap().FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
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
func counts(days []DayCount) []float64 {
	out := make([]float64, len(days))
	for i, day := range days {
		out[i] = float64(day.Count)
	}
	return out
}
