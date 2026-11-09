package stats

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// unknown is what a percentage is worth before there is a review to divide by:
// a dash, which says "not measured" rather than the 0% the division would.
const unknown = "—"

// overviewPage is S36: the four numbers, the production, the calendar strip,
// the time share, the retention trend, the coming fortnight and the plugins'
// own sections. It is read-only: nothing on it writes, so nothing on it is a
// control but the plugins' buttons.
func overviewPage(c *ui.Context, s State, d Deps, msgs locales.Messages, tk design.Tokens) {
	pageHead(c, tk, msgs.StatsTitle(), msgs.StatsOverviewSub())
	numbers(c, s, msgs, tk)
	productionSection(c, s, msgs, tk)
	calendarSection(c, s, msgs, tk)
	timeShareSection(c, s, msgs, tk)
	trendSection(c, s, msgs, tk)
	upcomingSection(c, s, msgs, tk)
	blocks(c, d, tk)
}

// numbers is the four numbers the overview opens with: how well the library
// remembers — against the chance its presets aim at — and what today holds.
func numbers(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	ui.Row(c).FillWidth().Wrap().Gap(tk.SpaceXL).AlignItems(ui.Start).Children(func() {
		value := unknown
		if rate, ok := s.Rate(); ok {
			value = msgs.StatsPercent(rate)
		}
		// The goal is worth reading before there is a rate to compare with it.
		detail := ""
		if s.Target > 0 {
			detail = msgs.StatsRetentionTarget(msgs.StatsPercent(s.Target))
		}
		kit.StatDetail(c, value, msgs.StatsRetentionLabel(), detail)
		kit.Stat(c, msgs.StatsNumber(s.Today.Due), msgs.StatsDueToday())
		kit.Stat(c, msgs.StatsNumber(s.Today.Reading), msgs.StatsNotesToRead())
		kit.Stat(c, msgs.StatsNumber(s.Today.Reviewed), msgs.StatsCardsToday())
	})
}

// productionSection is what the library has made: the text it holds, the
// excerpts taken out of it, and the cards that came of them.
func productionSection(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	p := s.Production
	if p.Characters == 0 && p.Cards == 0 && p.Excerpts == 0 {
		quiet(c, tk, msgs.StatsProduction(), msgs.StatsProductionEmpty())
		return
	}
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionHead(c, tk, msgs.StatsProduction(), "")
		ui.Row(c).FillWidth().Wrap().Gap(tk.SpaceXL).AlignItems(ui.Start).Children(func() {
			kit.Stat(c, msgs.StatsNumber(p.Characters), msgs.StatsCharacters())
			if p.Excerpts > 0 {
				kit.StatDetail(c, msgs.StatsNumber(p.Excerpts), msgs.StatsExcerpts(),
					msgs.StatsCardsPerExcerpt(trimFloat(p.CardsPerExcerpt)))
			} else {
				kit.Stat(c, msgs.StatsNumber(p.Excerpts), msgs.StatsExcerpts())
			}
			if p.Notes > 0 {
				kit.StatDetail(c, msgs.StatsNumber(p.Cards), msgs.StatsCards(),
					msgs.StatsCardsFromNotes(p.Notes))
			} else {
				kit.Stat(c, msgs.StatsNumber(p.Cards), msgs.StatsCards())
			}
		})
	})
}

// calendarSection is the overview's copy of the calendar: the same squares as
// the heatmap page, small enough to sit beside everything else.
func calendarSection(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	if s.HeatTotal == 0 {
		quiet(c, tk, msgs.StatsReviewsAndReading(), msgs.StatsHeatEmpty())
		return
	}
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionHead(c, tk, msgs.StatsReviewsAndReading(),
			msgs.StatsHeatRange(s.HeatFrom, s.HeatTo, s.HeatTotal))
		calendar(c, s, msgs, tk, tk.HeatCell, true)
	})
}

// timeShareSection is how the range's minutes divide between reading,
// reviewing and taking excerpts: one strip, and the three numbers beneath it so
// no segment's share rests on its colour.
func timeShareSection(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	share := s.TimeShare
	if share.Read+share.Review+share.Excerpt == 0 {
		quiet(c, tk, msgs.StatsTimeShare(), msgs.StatsTimeShareEmpty())
		return
	}
	segments := []kit.Segment{
		{Label: msgs.StatsShareRead(), Value: float64(share.Read), Colour: tk.Accent},
		{Label: msgs.StatsShareReview(), Value: float64(share.Review), Colour: tk.Accent.Mix(tk.Surface, 0.35)},
		{Label: msgs.StatsShareExcerpt(), Value: float64(share.Excerpt), Colour: tk.Accent.Mix(tk.Surface, 0.65)},
	}
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionHead(c, tk, msgs.StatsTimeShare(), "")
		kit.ProgressRatio(c, segments)
		ui.Row(c).Wrap().Gap(tk.SpaceL).AlignItems(ui.Center).Children(func() {
			for _, segment := range segments {
				shareKey(c, tk, msgs, segment)
			}
		})
		note(c, tk, msgs.StatsTimeShareNote())
	})
}

// shareKey is one segment of the time share's legend: its colour, its name and
// the minutes it is worth, so the strip reads without matching shades.
func shareKey(c *ui.Context, tk design.Tokens, msgs locales.Messages, s kit.Segment) {
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceXS).Children(func() {
		ui.Box(c).Width(tk.SpaceS).Height(tk.SpaceS).Radius(tk.RadiusBar).Background(s.Colour)
		ui.Text(c, msgs.StatsShareMinutes(s.Label, int(s.Value))).FontSize(tk.TypeUI)
	})
}

// trendSection is how well the range's reviews were remembered, week by week:
// the same measure the first number reports, drawn over the calendar's span.
func trendSection(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	rate, ok := s.Rate()
	if !ok {
		quiet(c, tk, msgs.StatsRetention(), msgs.StatsRetentionEmpty())
		return
	}
	measured := 0
	for _, week := range s.Trend {
		measured += week.Reviews
	}
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionHead(c, tk, msgs.StatsRetention(),
			msgs.StatsRetentionSummary(msgs.StatsPercent(rate), s.Retention.Reviews))
		if measured == 0 {
			note(c, tk, msgs.StatsTrendEmpty())
			return
		}
		trendPlot(c, tk, msgs, s)
	})
}

// trendPlot draws the weekly rate with ui/chart's time scale, an area under it
// and a line on top: a week the log has no review for is a gap in both rather
// than a dive to zero.
func trendPlot(c *ui.Context, tk design.Tokens, msgs locales.Messages, s State) {
	x := chart.NewTime(s.HeatFrom, s.HeatTo.AddDate(0, 0, 1))
	x.Format = msgs.StatsDay
	y := chart.NewLinear(0, 1)
	y.Format = msgs.StatsPercent

	xs, ys := make([]float32, len(s.Trend)), make([]float32, len(s.Trend))
	for i, week := range s.Trend {
		xs[i] = x.At(week.Week)
		ys[i] = float32(math.NaN())
		if week.Reviews > 0 {
			ys[i] = float32(week.Rate())
		}
	}
	axis := mutedAxis(c, tk)
	chart.Plot(c, chart.Axes{Left: y.Ticks(3), Bottom: x.Ticks(4), Label: axis}, func(area ui.Element) {
		area.Draw(func(p *ui.Painter, r ui.Rect) {
			p.FillPath(chart.Area(r, xs, nil, ys, chart.CurveMonotone), tk.Accent.Alpha(0.12))
			p.StrokePath(chart.Line(r, xs, ys, chart.CurveMonotone), 2*tk.Hairline, tk.Accent)
		})
	}).Height(plotHeight).Gap(tk.SpaceS).Label(msgs.StatsRetention())
}

// upcomingSection is the coming fortnight as bars: the forecast page's month,
// cut to the two weeks an overview can carry.
func upcomingSection(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	days := s.Forecast[:min(OverviewForecastDays, len(s.Forecast))]
	total := 0
	for _, day := range days {
		total += day.Count
	}
	if total == 0 {
		quiet(c, tk, msgs.StatsForecastShort(), msgs.StatsForecastEmpty(len(days)))
		return
	}
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionHead(c, tk, msgs.StatsForecastShort(), msgs.CardCount(total))
		barPlot(c, tk, msgs, days, msgs.StatsForecastShort())
	})
}

// barPlot draws one bar per day with its count, over a time axis: ui/chart's
// plot, time scale and rect marks, so nothing here paints a pixel. The tooltip
// names a day's count for the pointer.
func barPlot(c *ui.Context, tk design.Tokens, msgs locales.Messages, days []DayCount, label string) {
	if len(days) == 0 {
		return
	}
	x := chart.NewTime(days[0].Day, days[len(days)-1].Day.AddDate(0, 0, 1))
	x.Format = msgs.StatsDay

	values := make([]float64, len(days))
	for i, day := range days {
		values[i] = float64(day.Count)
	}
	_, hi := chart.Extent(values...)
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
				bar.Tooltip(msgs.StatsDueCount(day.Day, day.Count))
			}
		}
	}).Height(plotHeight).Gap(tk.SpaceS).Label(label)
}

// trimFloat writes an average without trailing zeros: "2" rather than "2.0",
// "2.5" when the division leaves a half.
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
