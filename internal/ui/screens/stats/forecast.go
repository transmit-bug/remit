package stats

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// forecastPage is S38: what comes due over the next month, day by day, with
// today, the busiest day and tomorrow named above the bars — and the sentence
// that says why a reading note and a card stand on one schedule.
func forecastPage(c *ui.Context, s State, msgs locales.Messages, tk design.Tokens) {
	pageHead(c, tk, msgs.StatsForecastTitle(), msgs.StatsForecastSub())

	total := 0
	for _, day := range s.Forecast {
		total += day.Count
	}
	if len(s.Forecast) == 0 || total == 0 {
		quiet(c, tk, msgs.StatsForecast(), msgs.StatsForecastEmpty(len(s.Forecast)))
		note(c, tk, msgs.StatsForecastMerged())
		return
	}

	peak, _ := s.Peak()
	ui.Row(c).FillWidth().Wrap().Gap(tk.SpaceXL).AlignItems(ui.Start).Children(func() {
		kit.Stat(c, msgs.StatsNumber(s.Forecast[0].Count), msgs.StatsToday())
		kit.StatDetail(c, msgs.StatsNumber(peak.Count), msgs.StatsPeak(), msgs.StatsPeakDay(peak.Day))
		kit.Stat(c, msgs.StatsNumber(tomorrow(s).Count), msgs.StatsTomorrow())
	})
	barPlot(c, tk, msgs, s.Forecast, msgs.StatsForecast())
	note(c, tk, msgs.StatsForecastMerged())
}

// tomorrow is the forecast's second day, or the first on a series of one: a
// page never reads past what the app loaded.
func tomorrow(s State) DayCount {
	if len(s.Forecast) == 0 {
		return DayCount{}
	}
	if len(s.Forecast) < 2 {
		return s.Forecast[0]
	}
	return s.Forecast[1]
}
