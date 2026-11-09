// Package stats is the statistics area: three read-only pages over the same
// append-only review log. Overview says how much the library holds, how today
// looks and what is coming; Heatmap draws the past half year day by day;
// Forecast says what comes due over the next month. None of them writes: the
// log is the record, and these pages only read it back.
//
// The app loads everything before the frame draws — the pages read State and
// never touch the store — so a page has no loading state to show. A library
// whose index cannot be read answers zeros rather than an error: every section
// then says what it lacks instead of plotting an empty axis, which is the same
// degradation the rest of the window makes.
package stats

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// Page is which of the area's three pages is on screen. It is the app's sub
// navigation, resolved: the screen does not know the route's names.
type Page uint8

const (
	// Overview is the summary page: the numbers, the production, the calendar
	// strip, the time share, the trend and the month's first two weeks.
	Overview Page = iota
	// HeatmapPage is the calendar page: the past half year, large.
	HeatmapPage
	// ForecastPage is the month ahead, day by day.
	ForecastPage
)

// plotHeight is one plot's height. It is the plot's, not a column's: a page of
// three plots scrolls at the smallest window rather than squeezing them.
const plotHeight float32 = 150

// Deps is what the page asks of the app: the plugin items the stats.block slot
// holds, as the library load cached them, and the two functions that resolve
// and run one. A page never calls a plugin: the items are read from the index,
// and a command runs off the frame like any other contribution.
type Deps struct {
	// Items is the slot's items in drawing order, one section per plugin.
	Items []contrib.MenuRef
	// PluginText resolves one of a plugin's own messages in the frame's
	// language; nil shows the message as written.
	PluginText func(c *ui.Context, plugin, raw string) string
	// Run answers one of them; nil draws none as pressable.
	Run func(c *ui.Context, ref contrib.CommandRef)
}

// View draws the page that is up, inside the paper's measure.
func View(c *ui.Context, page Page, s State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.PaperWidth).Gap(tk.SpaceXL).Children(func() {
				switch page {
				case HeatmapPage:
					heatmapPage(c, s, msgs, tk)
				case ForecastPage:
					forecastPage(c, s, msgs, tk)
				default:
					overviewPage(c, s, d, msgs, tk)
				}
			})
		})
	})
}

// pageHead is every page's title and the one line under it that says what the
// page is.
func pageHead(c *ui.Context, tk design.Tokens, title, subtitle string) {
	ui.Column(c).Gap(tk.SpaceXS).Children(func() {
		ui.Text(c, title).FontSize(tk.TypeH1).Bold()
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		}
	})
}

// sectionHead is one section's title and its one-line summary, when it has
// something to summarise.
func sectionHead(c *ui.Context, tk design.Tokens, title, summary string) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, title).FontSize(tk.TypeH2).Bold()
		if summary == "" {
			return
		}
		ui.Spacer(c)
		ui.Text(c, summary).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	})
}

// note is a page's one-line explanation: where a number came from, or why two
// things share a chart. It is muted because it explains rather than reports.
func note(c *ui.Context, tk design.Tokens, text string) {
	ui.Text(c, text).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
}

// quiet is a section that has nothing to draw: the heading stays and the
// sentence says what would fill it.
func quiet(c *ui.Context, tk design.Tokens, title, text string) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionHead(c, tk, title, "")
		ui.Text(c, text).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	})
}

// mutedAxis labels a plot's ticks in the muted type, so the data is what the
// eye lands on.
func mutedAxis(c *ui.Context, tk design.Tokens) func(chart.Side, chart.Tick) {
	return func(_ chart.Side, tick chart.Tick) {
		ui.Text(c, tick.Label).NoWrap().FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
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
