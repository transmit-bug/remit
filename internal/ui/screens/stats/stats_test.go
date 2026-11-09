package stats_test

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/schedule"
	corestats "github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/screens/stats"
	"github.com/transmit-bug/remit/locales"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// today is the day the pages are shown: a Sunday, so the calendar's last column
// is the week that is still running.
var (
	today    = time.Date(2026, 11, 8, 15, 4, 0, 0, time.UTC)
	midnight = time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
)

// here is a week of work the pages can show: reviews on three days, both kinds
// of activity, two excerpts and the cards they made, today's numbers, and two
// cards due in two days.
func here() stats.Input {
	return stats.Input{
		Now: today,
		Reviews: []corestats.DayCount{
			{Day: "2026-11-08", Count: 2},
			{Day: "2026-11-07", Count: 1},
			{Day: "2026-11-06", Count: 1},
		},
		History: []corestats.HistoryEvent{
			{At: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC), Kind: corestats.KindCard, Rating: schedule.Good},
			{At: time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC), Kind: corestats.KindCard, Rating: schedule.Again},
			{At: time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC), Kind: corestats.KindReading, Rating: schedule.Good},
			{At: time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC), Kind: corestats.KindCard, Rating: schedule.Good},
		},
		Retention:  corestats.Retention{Buckets: []corestats.Bucket{{Low: 0.9, High: 1.01, Reviews: 3, Passes: 3}}, Reviews: 3, Passes: 3},
		Target:     0.9,
		Forecast:   []corestats.DayCount{{Day: "2026-11-10", Count: 2}},
		Production: corestats.Production{Characters: 1200, Notes: 2, Excerpts: 2, Cards: 4, CardsPerExcerpt: 2},
		TimeShare:  corestats.TimeShare{Read: 6, Review: 2, Excerpt: 4},
		Due:        3,
		Reading:    1,
		Reviewed:   4,
	}
}

// data is the pages shown over a week of work.
func data() stats.State { return stats.Build(here()) }

// bare is the same pages on a library that holds nothing yet.
func bare() stats.State { return stats.Build(stats.Input{Now: today}) }

// at draws one page and lets it settle.
func at(t *testing.T, page stats.Page, s stats.State, w, h int) *ui.Tester {
	t.Helper()
	return draw(t, page, s, stats.Deps{}, w, h)
}

// draw is at with the app's side of the page: the plugin items and the command
// one of them runs.
func draw(t *testing.T, page stats.Page, s stats.State, d stats.Deps, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { stats.View(c, page, s, d) }, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// noText reports the first of wants the page does not say.
func noText(tt *ui.Tester, wants ...string) string {
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range wants {
		if !strings.Contains(texts, want) {
			return want
		}
	}
	return ""
}

// Build reads the app's answers into the calendar, the trend and the streak:
// Monday-first columns, a total, and the longest run of days with a review.
func TestBuildReadsTheCalendarTheTrendAndTheStreak(t *testing.T) {
	s := data()
	from, _ := stats.HeatRange(today)

	if len(s.Heat) != 26 {
		t.Fatalf("the calendar has %d columns, want 26", len(s.Heat))
	}
	if s.Heat[0][0].Day != from || s.Heat[0][0].Day.Weekday() != time.Monday {
		t.Errorf("the first square is %s (%s), want Monday %s", s.Heat[0][0].Day, s.Heat[0][0].Day.Weekday(), from)
	}
	if s.HeatTotal != 4 || s.HeatMax != 2 {
		t.Errorf("the calendar counts %d reviews, busiest %d, want 4 and 2", s.HeatTotal, s.HeatMax)
	}
	if s.Streak != 3 {
		t.Errorf("the longest run is %d days, want 3", s.Streak)
	}
	if got := s.HeatFrom; got != from {
		t.Errorf("the calendar starts %s, want %s", got, from)
	}
	// The last column is the week today falls in: its Sunday is today, and no
	// square is drawn for a day the log cannot hold yet.
	last := s.Heat[len(s.Heat)-1]
	if last[6].Day != midnight || last[6].Future {
		t.Errorf("the calendar ends on %+v, want today and not ahead", last[6])
	}

	if len(s.Trend) != 26 {
		t.Fatalf("the trend has %d weeks, want 26", len(s.Trend))
	}
	week := s.Trend[len(s.Trend)-1]
	if week.Reviews != 4 || week.Passes != 3 {
		t.Errorf("this week's trend is %+v, want 4 reviews and 3 passes", week)
	}
	if rate := week.Rate(); rate != 0.75 {
		t.Errorf("this week's rate is %v, want 0.75", rate)
	}
	if len(s.Forecast) != stats.ForecastDays || s.Forecast[0].Day != midnight {
		t.Fatalf("the forecast covers %d days from %s, want %d from today", len(s.Forecast), s.Forecast[0].Day, stats.ForecastDays)
	}
	if got := s.Forecast[2].Count; got != 2 {
		t.Errorf("the forecast holds %d cards in two days, want 2", got)
	}
	if peak, ok := s.Peak(); !ok || peak.Count != 2 {
		t.Errorf("the peak is %+v (found %v), want 2 cards", peak, ok)
	}
}

// The overview is the four numbers, what the library made, the calendar's
// strip, how the time divided, the trend and the fortnight — all from State.
func TestOverviewDrawsTheNumbersTheProductionAndTheShare(t *testing.T) {
	msgs := locales.In("en")
	s := data()
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := at(t, stats.Overview, s, size.w, size.h)
			want := []string{
				msgs.StatsTitle(), msgs.StatsOverviewSub(),
				msgs.StatsRetentionLabel(), "100%", msgs.StatsRetentionTarget(msgs.StatsPercent(0.9)),
				msgs.StatsDueToday(), msgs.StatsNotesToRead(), msgs.StatsCardsToday(),
				msgs.StatsProduction(), msgs.StatsCharacters(), msgs.StatsExcerpts(), msgs.StatsCards(),
				msgs.StatsCardsPerExcerpt("2"), msgs.StatsCardsFromNotes(2),
				msgs.StatsReviewsAndReading(), msgs.StatsHeatRange(s.HeatFrom, s.HeatTo, s.HeatTotal),
				msgs.StatsTimeShare(), msgs.StatsShareMinutes(msgs.StatsShareRead(), 6),
				msgs.StatsShareMinutes(msgs.StatsShareReview(), 2),
				msgs.StatsShareMinutes(msgs.StatsShareExcerpt(), 4),
				msgs.StatsTimeShareNote(),
				msgs.StatsRetention(), msgs.StatsRetentionSummary(msgs.StatsPercent(1), 3),
				msgs.StatsForecastShort(), msgs.CardCount(2),
			}
			if missing := noText(tt, want...); missing != "" {
				t.Fatalf("the overview is missing %q; texts %q", missing, tt.Texts())
			}
			for _, empty := range []string{
				msgs.StatsHeatEmpty(), msgs.StatsProductionEmpty(),
				msgs.StatsTimeShareEmpty(), msgs.StatsRetentionEmpty(), msgs.StatsForecastEmpty(stats.OverviewForecastDays),
			} {
				if strings.Contains(strings.Join(tt.Texts(), "\n"), empty) {
					t.Errorf("the overview shows the empty state %q beside its data", empty)
				}
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// The heatmap page says how wide its range is, what it remembered and how long
// the longest run was, and that the days come from the log rather than the
// index.
func TestHeatmapNamesItsRangeAndSource(t *testing.T) {
	msgs := locales.In("en")
	s := data()
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := at(t, stats.HeatmapPage, s, size.w, size.h)
			if missing := noText(tt,
				msgs.StatsHeat(), msgs.StatsHeatSub(),
				msgs.StatsWeeks(26), msgs.StatsRange(),
				msgs.StatsRangeSpanTotal(s.HeatFrom, s.HeatTo, 4),
				msgs.StatsPercent(1), msgs.StatsRetentionLabel(),
				msgs.StatsDays(3), msgs.StatsLongestStreak(),
				msgs.StatsHeatLess(), msgs.StatsHeatMore(),
				msgs.StatsHeatSource(),
			); missing != "" {
				t.Fatalf("the heatmap page is missing %q; texts %q", missing, tt.Texts())
			}
			// Every day the log counts has a square of its own, named with the
			// day and its count.
			for _, week := range s.Heat {
				for _, day := range week {
					if day.Count == 0 {
						continue
					}
					if want := msgs.StatsDayCount(day.Day, day.Count); !tt.HasText(want) {
						t.Errorf("the calendar has no square for %q; texts %q", want, tt.Texts())
					}
				}
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// The forecast page names today, the peak and tomorrow, and says why a reading
// note and a card stand on one schedule.
func TestForecastNamesTheThreeDaysAndWhyTheyMerge(t *testing.T) {
	msgs := locales.In("en")
	s := data()
	peak, _ := s.Peak()
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := at(t, stats.ForecastPage, s, size.w, size.h)
			if missing := noText(tt,
				msgs.StatsForecastTitle(), msgs.StatsForecastSub(),
				msgs.StatsToday(), msgs.StatsPeak(), msgs.StatsPeakDay(peak.Day),
				msgs.StatsTomorrow(), msgs.StatsForecastMerged(),
			); missing != "" {
				t.Fatalf("the forecast page is missing %q; texts %q", missing, tt.Texts())
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// The overview's last sections are the plugins': one per plugin with an item in
// the slot, its items as buttons, and each button the command it names.
func TestBlockSectionsComeFromTheSlot(t *testing.T) {
	deps := stats.Deps{
		Items: []contrib.MenuRef{
			menu("acme.report", "report", "acme.report.stats", "Stats block"),
			menu("acme.leeches", "leeches", "acme.leeches.top", "Leech report"),
			menu("acme.report", "report", "acme.report.weekly", "Weekly review"),
		},
		Run: func(*ui.Context, contrib.CommandRef) {},
	}
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := draw(t, stats.Overview, data(), deps, size.w, size.h)
			if missing := noText(tt, "report", "leeches", "Stats block", "Leech report", "Weekly review"); missing != "" {
				t.Fatalf("the plugin sections are missing %q; texts %q", missing, tt.Texts())
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
	// A library whose slot holds nothing draws no heading for it.
	tt := draw(t, stats.Overview, data(), stats.Deps{}, 1024, 720)
	if got := len(tt.Texts()); got == 0 {
		t.Fatal("the overview drew nothing")
	}
	for _, gone := range []string{"report", "leeches"} {
		if tt.HasText(gone) {
			t.Errorf("the overview drew the plugin %q with no item in the slot", gone)
		}
	}
}

// menu is one contributed item as the app's index hands it over.
func menu(plugin, name, command, title string) contrib.MenuRef {
	return contrib.MenuRef{CommandRef: contrib.CommandRef{
		Plugin:  plugin,
		Name:    name,
		Command: contrib.Command{ID: command, Title: contrib.Message(title)},
	}}
}

// A library with nothing in it says what would fill each section rather than
// plotting axes with no values.
func TestEmptyPagesSayWhatWouldFillThem(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			overview := at(t, stats.Overview, bare(), size.w, size.h)
			if missing := noText(overview,
				msgs.StatsTitle(), msgs.StatsHeatEmpty(), msgs.StatsProductionEmpty(),
				msgs.StatsTimeShareEmpty(), msgs.StatsRetentionEmpty(),
				msgs.StatsForecastEmpty(stats.OverviewForecastDays),
			); missing != "" {
				t.Fatalf("the empty overview is missing %q; texts %q", missing, overview.Texts())
			}
			if overview.HasText(msgs.StatsHeatLess()) {
				t.Error("the empty overview drew the calendar's legend")
			}

			heat := at(t, stats.HeatmapPage, bare(), size.w, size.h)
			if missing := noText(heat,
				msgs.StatsHeatEmpty(), msgs.StatsRangeSpanTotal(bare().HeatFrom, bare().HeatTo, 0), msgs.StatsHeatSource(),
			); missing != "" {
				t.Fatalf("the empty heatmap is missing %q; texts %q", missing, heat.Texts())
			}

			forecast := at(t, stats.ForecastPage, bare(), size.w, size.h)
			if missing := noText(forecast,
				msgs.StatsForecastEmpty(stats.ForecastDays), msgs.StatsForecastMerged(),
			); missing != "" {
				t.Fatalf("the empty forecast is missing %q; texts %q", missing, forecast.Texts())
			}
			for _, tt := range []*ui.Tester{overview, heat, forecast} {
				if tt.FramePending() {
					t.Error("an empty page still requests frames after it should settle")
				}
			}
		})
	}
}

// The three pages say their own words in Chinese at the smallest window, and
// nothing they draw leaves it.
func TestZhHansAtMinimum(t *testing.T) {
	msgs := locales.In("zh-Hans")
	s := data()
	pages := map[stats.Page][]string{
		stats.Overview: {msgs.StatsTitle(), msgs.StatsOverviewSub(), msgs.StatsProduction(),
			msgs.StatsTimeShare(), msgs.StatsRetention(), msgs.StatsForecastShort()},
		stats.HeatmapPage: {msgs.StatsHeat(), msgs.StatsHeatSub(), msgs.StatsRange(),
			msgs.StatsLongestStreak(), msgs.StatsHeatSource()},
		stats.ForecastPage: {msgs.StatsForecastTitle(), msgs.StatsForecastSub(),
			msgs.StatsToday(), msgs.StatsPeak(), msgs.StatsTomorrow(), msgs.StatsForecastMerged()},
	}
	for page, want := range pages {
		tt := ui.NewTester(func(c *ui.Context) { stats.View(c, page, s, stats.Deps{}) }, 720, 520)
		tt.SetLocale("zh-Hans")
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		if missing := noText(tt, want...); missing != "" {
			t.Fatalf("page %d is missing %q at 720×520 in zh-Hans; texts %q", page, missing, tt.Texts())
		}
		if tt.FramePending() {
			t.Errorf("page %d still requests frames after it should settle", page)
		}
	}
}
