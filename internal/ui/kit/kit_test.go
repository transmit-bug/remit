package kit_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// appearances are the two the window draws in; every control has to hold in
// both.
var appearances = []struct {
	name string
	dark bool
}{
	{"paper", false},
	{"ink", true},
}

// at draws view once in an appearance and hands back the tester.
func at(t *testing.T, dark bool, view func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(view, w, h)
	tt.SetDark(dark)
	tt.Frame()
	tt.Advance(300 * time.Millisecond) // the window's entrance
	return tt
}

// Each control draws in both appearances, and settles.
func TestEveryControlInBothAppearances(t *testing.T) {
	views := map[string]func(*ui.Context){
		"card": func(c *ui.Context) {
			kit.Card(c, func() { ui.Text(c, "Card body") })
		},
		"panel": func(c *ui.Context) {
			kit.Panel(c, func() { ui.Text(c, "Panel body") })
		},
		"empty": func(c *ui.Context) {
			kit.Empty(c, "coffee", "Nothing to review.", nil)
		},
		"keycap": func(c *ui.Context) {
			ui.Row(c).Gap(design.SpaceXS).Children(func() {
				kit.Keycap(c, "1")
				ui.Text(c, "Again")
			})
		},
		"stat": func(c *ui.Context) {
			kit.Stat(c, "42", "reviews this week")
		},
		"stat detail": func(c *ui.Context) {
			kit.StatDetail(c, "83%", "Remembered", "target 90%")
		},
		"heatmap": func(c *ui.Context) {
			kit.Heatmap(c, heatWeeks(), 3, kit.HeatmapOptions{
				Cell: 12, Gap: design.HeatGap, Weekdays: true,
				WeekdayWidth: design.HeatWeekday,
				Weekday:      func(time.Time) string { return "Mon" },
				Tooltip:      func(kit.HeatDay) string { return "3 reviews" },
				Label:        "Review heatmap",
				Less:         "Less", More: "More",
			})
		},
	}
	for _, a := range appearances {
		for name, view := range views {
			t.Run(a.name+"/"+name, func(t *testing.T) {
				tt := at(t, a.dark, view, 480, 320)
				if len(tt.Texts()) == 0 {
					t.Fatalf("the %s drew no text; texts %q", name, tt.Texts())
				}
				if tt.FramePending() {
					t.Errorf("the %s never settled", name)
				}
			})
		}
	}
}

// A card draws what it holds and nothing of its own: no border, no label, no
// extra sentence.
func TestCardHoldsItsContent(t *testing.T) {
	for _, a := range appearances {
		t.Run(a.name, func(t *testing.T) {
			tt := at(t, a.dark, func(c *ui.Context) {
				kit.Card(c, func() {
					ui.Text(c, "Spacing beats cramming")
					ui.Text(c, "answer")
				})
			}, 480, 320)
			for _, want := range []string{"Spacing beats cramming", "answer"} {
				if !tt.HasText(want) {
					t.Errorf("the card lost %q; texts %q", want, tt.Texts())
				}
			}
			if got, want := len(tt.Texts()), 2; got != want {
				t.Errorf("the card draws %d texts, want only its own %d: %q", got, want, tt.Texts())
			}
		})
	}
}

// An empty place is a sentence and one action: the action works, and nothing
// else is drawn in place of content.
func TestEmptyIsOneAction(t *testing.T) {
	for _, a := range appearances {
		t.Run(a.name, func(t *testing.T) {
			clicks := 0
			tt := at(t, a.dark, func(c *ui.Context) {
				kit.Empty(c, "coffee", "Nothing to review.", func() {
					ui.PrimaryButton(c, "Back to notes").MinHeight(design.ControlHeight).
						OnClick(func() { clicks++ })
				})
			}, 480, 320)
			if !tt.HasText("Nothing to review.") || !tt.HasText("Back to notes") {
				t.Fatalf("the empty place is missing its sentence or action; texts %q", tt.Texts())
			}
			if err := tt.Click("Back to notes"); err != nil {
				t.Fatal(err)
			}
			if clicks != 1 {
				t.Errorf("the action ran %d times, want 1", clicks)
			}
		})
	}
}

// The keycap sits on the key face and does not overlap the label beside it.
func TestKeycapSitsBesideItsLabel(t *testing.T) {
	for _, a := range appearances {
		t.Run(a.name, func(t *testing.T) {
			tt := at(t, a.dark, func(c *ui.Context) {
				ui.Row(c).Gap(design.SpaceXS).AlignItems(ui.Center).Children(func() {
					kit.Keycap(c, "1")
					ui.Text(c, "Again")
				})
			}, 720, 520)

			key, ok := tt.Find("1")
			if !ok {
				t.Fatalf("the keycap is not drawn; texts %q", tt.Texts())
			}
			label, ok := tt.Find("Again")
			if !ok {
				t.Fatalf("the label is not drawn; texts %q", tt.Texts())
			}
			if key.X+key.W > label.X {
				t.Errorf("the keycap %+v overlaps its label %+v", key, label)
			}
		})
	}
}

// A statistic is its number and the name of what it counts, in that order.
func TestStatShowsValueAndLabel(t *testing.T) {
	tt := at(t, false, func(c *ui.Context) {
		kit.Stat(c, "42", "reviews this week")
	}, 480, 320)
	value, ok := tt.Find("42")
	if !ok {
		t.Fatalf("the value is not drawn; texts %q", tt.Texts())
	}
	label, ok := tt.Find("reviews this week")
	if !ok {
		t.Fatalf("the label is not drawn; texts %q", tt.Texts())
	}
	if label.Y < value.Y {
		t.Errorf("the label %+v sits above its value %+v", label, value)
	}
}

// heatWeeks is a calendar's worth of days, two of them counted, one still ahead.
func heatWeeks() [][]kit.HeatDay {
	weeks := make([][]kit.HeatDay, 4)
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	for w := range weeks {
		week := make([]kit.HeatDay, 7)
		for d := range week {
			week[d] = kit.HeatDay{Day: day}
			day = day.AddDate(0, 0, 1)
		}
		weeks[w] = week
	}
	weeks[0][2].Count = 3
	weeks[1][4].Count = 1
	weeks[3][6].Future = true
	return weeks
}

// A calendar names the days the log counts and the ends of its scale, and draws
// nothing for a day still ahead.
func TestHeatmapNamesItsCountedDays(t *testing.T) {
	tt := at(t, false, func(c *ui.Context) {
		kit.Heatmap(c, heatWeeks(), 3, kit.HeatmapOptions{
			Cell: 12, Gap: design.HeatGap, Weekdays: true,
			WeekdayWidth: design.HeatWeekday,
			Weekday:      func(time.Time) string { return "Mon" },
			Tooltip:      func(d kit.HeatDay) string { return d.Day.Format("01-02") },
			Label:        "Review heatmap",
			Less:         "Less", More: "More",
		})
	}, 480, 320)
	for _, want := range []string{"11-04", "11-13", "Mon", "Less", "More"} {
		if !tt.HasText(want) {
			t.Errorf("the calendar lost %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("11-30") {
		t.Error("the calendar named a day it draws nothing for")
	}
	if tt.FramePending() {
		t.Error("the calendar never settled")
	}
}

// Whole numbers on the scale: any review is at least one step, the busiest day
// is the fourth, and a calendar with no review has no step to take.
func TestHeatLevelCoversTheScale(t *testing.T) {
	for _, tc := range []struct {
		count, most, want int
	}{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 4},
		{5, 10, 2},
		{3, 0, 1},
	} {
		if got := kit.HeatLevel(tc.count, tc.most); got != tc.want {
			t.Errorf("HeatLevel(%d, %d) = %d, want %d", tc.count, tc.most, got, tc.want)
		}
	}
}

// The ratio bar divides its width by its segments' values, and a bar with
// nothing to divide says so by drawing its track and no segment.
func TestProgressRatioDividesItsWidth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		segments []kit.Segment
	}{
		{"three", []kit.Segment{{Label: "Read", Value: 6}, {Label: "Review", Value: 2}, {Label: "Excerpt", Value: 4}}},
		{"none", nil},
	} {
		for _, a := range appearances {
			t.Run(a.name+"/"+tc.name, func(t *testing.T) {
				tt := at(t, a.dark, func(c *ui.Context) {
					kit.ProgressRatio(c, tc.segments)
				}, 480, 320)
				if tt.FramePending() {
					t.Error("the bar never settled")
				}
			})
		}
	}
}
