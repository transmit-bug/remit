package stats

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// heatmapPage is S37: the past half year as one calendar, the three numbers
// that frame it, and the sentence that says where the days come from.
func heatmapPage(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	pageHead(c, tk, msgs.StatsHeat(), msgs.StatsHeatSub())

	ui.Row(c).FillWidth().Wrap().Gap(tk.SpaceXL).AlignItems(ui.Start).Children(func() {
		kit.StatDetail(c, msgs.StatsWeeks(len(s.Heat)), msgs.StatsRange(),
			msgs.StatsRangeSpanTotal(s.HeatFrom, s.HeatTo, s.HeatTotal))
		value := unknown
		if rate, ok := s.Rate(); ok {
			value = msgs.StatsPercent(rate)
		}
		kit.Stat(c, value, msgs.StatsRetentionLabel())
		streak := unknown
		if s.HeatTotal > 0 {
			streak = msgs.StatsDays(s.Streak)
		}
		kit.Stat(c, streak, msgs.StatsLongestStreak())
	})

	if s.HeatTotal == 0 {
		ui.Text(c, msgs.StatsHeatEmpty()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	} else {
		calendar(c, s, msgs, tk, tk.HeatCellLarge, true)
	}
	note(c, tk, msgs.StatsHeatSource())
}

// calendar is the review heatmap: kit.Heatmap's squares, the days named for the
// pointer and the scale's legend. cell picks which of the two sizes the
// calendar is drawn at — the overview's strip or this page's grid.
func calendar(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens, cell float32, weekdays bool) {
	kit.Heatmap(c, s.Heat, s.HeatMax, kit.HeatmapOptions{
		Cell:         cell,
		Gap:          tk.HeatGap,
		Weekdays:     weekdays,
		WeekdayWidth: tk.HeatWeekday,
		Weekday:      msgs.StatsHeatWeekday,
		Tooltip: func(day kit.HeatDay) string {
			return msgs.StatsDayCount(day.Day, day.Count)
		},
		Label: msgs.StatsHeat(),
		Less:  msgs.StatsHeatLess(),
		More:  msgs.StatsHeatMore(),
	})
}
