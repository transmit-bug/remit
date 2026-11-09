package kit

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// Segment is one part of a ratio bar: what it is worth, the name its legend
// gives it, and its colour. The legend is the caller's, because a name is a
// string in the frame's language.
type Segment struct {
	Label  string
	Value  float64
	Colour ui.Color
}

// ProgressRatio draws a bar whose width divides between its segments' values,
// in the order given: the time a library spent reading, reviewing and taking
// excerpts, as one strip rather than three numbers to add up.
//
// Composed rather than taken from a widget. ui.Meter reports a single value in
// a range; ui/chart's stacking applies to column and area marks, not to a
// one-row share bar. Checked against MyGo v0.8.1.
func ProgressRatio(c *ui.Context, segments []Segment) ui.Element {
	t := c.Theme()
	total := 0.0
	for _, s := range segments {
		total += max(s.Value, 0)
	}
	bar := ui.Row(c).FillWidth().Height(design.SpaceS).Radius(design.RadiusBar).Clip()
	if total <= 0 {
		return bar.Background(design.PaletteOf(t.Dark).Sunken)
	}
	return bar.Children(func() {
		for _, s := range segments {
			if s.Value <= 0 {
				continue
			}
			ui.Box(c).Grow(float32(s.Value)).FillHeight().Background(s.Colour)
		}
	})
}
