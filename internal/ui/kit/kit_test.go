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

// A keycap sits on the key face at the smallest window and does not overlap the
// label beside it.
func TestKeycapFitsTheSmallestWindow(t *testing.T) {
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
			if key.X < 0 || key.Y < 0 || key.X+key.W > 720 || key.Y+key.H > 520 {
				t.Errorf("the keycap box %+v is outside the 720x520 window", key)
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
