package stats_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	corestats "github.com/transmit-bug/remit/internal/core/stats"
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

// today is the day the page is shown: a Sunday, so the heatmap's last column
// is the week that is still running.
var (
	today    = time.Date(2026, 11, 8, 15, 4, 0, 0, time.UTC)
	midnight = time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
)

// data is a page with something in all three blocks: reviews on two days, one
// review to compare with the schedule, and cards due in two days.
func data() stats.State {
	return stats.Build(today,
		[]corestats.DayCount{{Day: "2026-11-08", Count: 2}, {Day: "2026-11-07", Count: 1}},
		corestats.Retention{
			Buckets: []corestats.Bucket{
				{Low: 0, High: 0.7, Reviews: 1, Passes: 1},
				{Low: 0.7, High: 0.8},
				{Low: 0.8, High: 0.9},
				{Low: 0.9, High: 1.01},
			},
			Reviews: 1,
			Passes:  1,
		},
		[]corestats.DayCount{{Day: "2026-11-10", Count: 2}},
	)
}

// at draws the page at a size and lets it settle.
func at(t *testing.T, s stats.State, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { stats.View(c, s, stats.Deps{}) }, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// The page draws the three blocks from State alone: the heatmap with a square
// per counted day, the retention summary, and the forecast's total.
func TestStatsPage(t *testing.T) {
	msgs := locales.In("en")
	s := data()
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := at(t, s, size.w, size.h)

			for _, want := range []string{
				msgs.StatsTitle(),
				msgs.StatsHeat(), msgs.StatsHeatRange(s.HeatFrom, s.HeatTo, s.HeatTotal),
				msgs.StatsHeatWeekday(s.Heat[0][0].Day),
				msgs.StatsHeatLess(), msgs.StatsHeatMore(),
				msgs.StatsRetention(), "100% remembered over 1 review",
				msgs.StatsForecast(), "2 cards",
			} {
				if !tt.HasText(want) {
					t.Fatalf("the page is missing %q; texts %q", want, tt.Texts())
				}
			}
			for _, week := range s.Heat {
				for _, day := range week {
					if day.Count == 0 {
						continue
					}
					if want := msgs.StatsDayCount(day.Day, day.Count); !tt.HasText(want) {
						t.Errorf("the heatmap has no square for %q; texts %q", want, tt.Texts())
					}
				}
			}
			for _, empty := range []string{
				msgs.StatsHeatEmpty(), msgs.StatsRetentionEmpty(), msgs.StatsForecastEmpty(),
			} {
				if tt.HasText(empty) {
					t.Errorf("the page shows the empty state %q beside its data", empty)
				}
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// A series with nothing in it says so instead of plotting an axis with no
// values: the three blocks keep their headings and drop their legends.
func TestEmptyPage(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := at(t, stats.State{}, size.w, size.h)

			for _, want := range []string{
				msgs.StatsTitle(), msgs.StatsHeat(), msgs.StatsHeatEmpty(),
				msgs.StatsRetention(), msgs.StatsRetentionEmpty(),
				msgs.StatsForecast(), msgs.StatsForecastEmpty(),
			} {
				if !tt.HasText(want) {
					t.Fatalf("the empty page is missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.HasText(msgs.StatsHeatLess()) {
				t.Error("the empty page drew the heatmap's legend")
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// Build turns the app's sparse counts into the page's data: 26 Monday-first
// columns, the busiest day and the total, a square not yet due for the days
// after today, and one forecast day per day ahead.
func TestBuild(t *testing.T) {
	s := data()
	from, to := stats.HeatRange(today)

	if len(s.Heat) != 26 {
		t.Fatalf("the heatmap has %d columns, want 26", len(s.Heat))
	}
	if s.HeatFrom != from || s.HeatTo != midnight {
		t.Errorf("the map covers %s..%s, want %s..%s", s.HeatFrom, s.HeatTo, from, midnight)
	}
	if got := s.Heat[0][0]; got.Day != from || got.Day.Weekday() != time.Monday {
		t.Errorf("the first square is %s (%s), want Monday %s", got.Day, got.Day.Weekday(), from)
	}
	if s.HeatTotal != 3 || s.HeatMax != 2 {
		t.Errorf("the map counts %d reviews, busiest %d, want 3 and 2", s.HeatTotal, s.HeatMax)
	}

	last := s.Heat[len(s.Heat)-1]
	if last[6].Day.AddDate(0, 0, 1) != to {
		t.Errorf("the last column ends on %s, want the map to cover up to %s", last[6].Day, to)
	}
	for _, week := range s.Heat {
		for _, day := range week {
			if want := day.Day.After(midnight); day.Future != want {
				t.Errorf("%s is future=%v, want %v", day.Day, day.Future, want)
			}
		}
	}

	if len(s.Forecast) != stats.ForecastDays {
		t.Fatalf("the forecast covers %d days, want %d", len(s.Forecast), stats.ForecastDays)
	}
	if got := s.Forecast[0]; got.Day != midnight || got.Count != 0 {
		t.Errorf("the forecast starts at %+v, want today with nothing due", got)
	}
	if got := s.Forecast[2]; got.Count != 2 {
		t.Errorf("the forecast has %d cards in two days, want 2", got.Count)
	}
}

// In Chinese the page holds at the smallest window: each block's heading keeps
// a box of its own inside it.
func TestAtMinimumInChinese(t *testing.T) {
	msgs := locales.In("zh-Hans")
	tt := ui.NewTester(func(c *ui.Context) { stats.View(c, data(), stats.Deps{}) }, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	for _, label := range []string{msgs.StatsTitle(), msgs.StatsHeat(), msgs.StatsRetention(), msgs.StatsForecast()} {
		reveal(t, tt, 720, 520, label)
		r, ok := tt.Find(label)
		if !ok || r.W <= 0 || r.H <= 0 {
			t.Fatalf("%q has no box; texts %q", label, tt.Texts())
		}
		if r.X < 0 || r.Y < 0 || r.X+r.W > 720 || r.Y+r.H > 520 {
			t.Errorf("%q is outside the window: %.1f,%.1f %.1fx%.1f", label, r.X, r.Y, r.W, r.H)
		}
	}
	if tt.HasText(locales.In("en").StatsTitle()) {
		t.Error("the page kept its English title")
	}
	tt.Advance(300 * time.Millisecond)
	if tt.FramePending() {
		t.Error("view still requests frames after the locale switch")
	}
}

// reveal scrolls the page until s has a box inside the window, which is what
// makes a block below the fold visible.
func reveal(t *testing.T, tt *ui.Tester, w, h int, s string) {
	t.Helper()
	inside := func(r ui.Rect) bool {
		return r.W > 0 && r.H > 0 && r.X >= 0 && r.Y >= 0 &&
			r.X+r.W <= float32(w) && r.Y+r.H <= float32(h)
	}
	for i := 0; i < 20; i++ {
		if r, ok := tt.Find(s); ok && inside(r) {
			return
		}
		tt.Scroll(float32(w)/2, float32(h)/2, 0, 120)
	}
}
