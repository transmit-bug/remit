package kit

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// HeatDay is one day of a calendar heatmap: how many reviews the log holds for
// it, and whether it is still ahead of today — a day with no box yet.
type HeatDay struct {
	Day    time.Time
	Count  int
	Future bool
}

// HeatmapOptions is what a calendar's caller decides: the box's size, the
// language's weekday names, the tooltip that names a day, and the two ends of
// the scale the legend shows.
type HeatmapOptions struct {
	// Cell is one day's square and Gap the room between two of them. Both
	// come from the design scale, so the overview's strip and the heatmap
	// page's grid differ by a token and not by a number.
	Cell, Gap float32
	// Weekdays names Monday, Wednesday and Friday in the left column, so the
	// rows read as a calendar. A strip too small to read a name beside leaves
	// them out and keeps the squares' own alignment.
	Weekdays bool
	// WeekdayWidth is that column's width: wide enough for the longest name a
	// locale writes at Label size.
	WeekdayWidth float32
	// Weekday is the name of a day's weekday in the frame's language; nil
	// draws no names, as a locale without them would.
	Weekday func(day time.Time) string
	// Tooltip names one day and its count; nil draws none.
	Tooltip func(day HeatDay) string
	// Label names the whole calendar for assistive technology.
	Label string
	// Less and More are the legend's two ends. Two empty strings draw no
	// legend, which is what a page with no reviews needs.
	Less, More string
}

// Heatmap draws calendar weeks as columns and weekdays as rows: one square per
// day, tinted by how many reviews it holds, the days ahead left empty.
//
// Composed rather than taken from a widget. ui/chart has no calendar heatmap at
// MyGo v0.8.1, and ui.GridView lays items out by a minimum width without the
// equal columns and week alignment a calendar needs. Checked against v0.8.1.
func Heatmap(c *ui.Context, weeks [][]HeatDay, most int, o HeatmapOptions) ui.Element {
	t := c.Theme()
	tk := design.NewTokens(t)
	return ui.Column(c).Gap(design.SpaceS).Children(func() {
		ui.Row(c).Gap(o.Gap).AlignItems(ui.Start).Label(o.Label).Children(func() {
			if o.Weekdays && o.Weekday != nil && len(weeks) > 0 && len(weeks[0]) == 7 {
				weekdayColumn(c, weeks[0], o)
			}
			for _, week := range weeks {
				ui.Column(c).Gap(o.Gap).Children(func() {
					for _, day := range week {
						box := ui.Box(c).Width(o.Cell).Height(o.Cell).Radius(design.RadiusBar)
						switch {
						case day.Future:
							return
						case day.Count == 0:
							box.Background(tk.Sunken)
						default:
							box.Background(tk.HeatFill(HeatLevel(day.Count, most)))
							if o.Tooltip != nil {
								label := o.Tooltip(day)
								box.Tooltip(label).Label(label)
							}
						}
					}
				})
			}
		})
		if o.Less != "" || o.More != "" {
			heatLegend(c, tk, o)
		}
	})
}

// weekdayColumn is the names beside the rows: enough of them to read the
// calendar, at the same height as a square so a name sits on its row.
func weekdayColumn(c *ui.Context, week []HeatDay, o HeatmapOptions) {
	ui.Column(c).Gap(o.Gap).Children(func() {
		for i, day := range week {
			name := ""
			// Monday, Wednesday and Friday name the rows: enough to read the
			// calendar, and no name so close to its neighbour that neither is
			// legible.
			if i < 5 && i%2 == 0 {
				name = o.Weekday(day.Day)
			}
			ui.Text(c, name).NoWrap().Width(o.WeekdayWidth).Height(o.Cell).
				LineHeight(1).FontSize(design.TypeLabel).
				TextColor(c.Theme().TextMuted)
		}
	})
}

// heatLegend names the two ends of the tint scale over the five steps between
// them, so a square reads as a count rather than as decoration.
func heatLegend(c *ui.Context, tk design.Tokens, o HeatmapOptions) {
	ui.Row(c).Gap(o.Gap).AlignItems(ui.Center).Children(func() {
		ui.Text(c, o.Less).FontSize(design.TypeUI).TextColor(tk.TextMuted)
		for level := 0; level <= 4; level++ {
			ui.Box(c).Width(o.Cell).Height(o.Cell).Radius(design.RadiusBar).
				Background(tk.HeatFill(level))
		}
		ui.Text(c, o.More).FontSize(design.TypeUI).TextColor(tk.TextMuted)
	})
}

// HeatLevel puts a day's count on the scale's four steps: any review is at
// least one, and the busiest day in the calendar is four. A calendar with no
// review at all has no scale to read.
func HeatLevel(count, most int) int {
	if count <= 0 {
		return 0
	}
	if most <= 0 {
		return 1
	}
	return min(max((4*count+most-1)/most, 1), 4)
}
