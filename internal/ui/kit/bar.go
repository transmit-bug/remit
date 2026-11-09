package kit

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// StackPart is one part of a stacked bar: how much of the whole it is, and the
// colour that names it. A part of zero draws nothing, so a bar of one kind is
// the whole bar.
type StackPart struct {
	Value  float32
	Colour ui.Color
}

// StackBar draws parts as one bar, each part's width its share of the whole, so
// that a queue reads as three kinds at once: new, learning and review.
//
// Composed rather than taken from a widget (checked against MyGo v0.8.1):
// ui.Meter reports one value in one colour its levels choose, so a bar of three
// named parts cannot be built from it without losing the names; ui/chart has no
// stacked bar. The parts are Boxes in a Row weighted by their value, and the
// Row's clip gives the bar its corner.
func StackBar(c *ui.Context, parts []StackPart, label string) ui.Element {
	bar := ui.Row(c).FillWidth().Height(design.SpaceS).Radius(design.RadiusBar).Clip().
		Label(label).Role(ui.RoleMeter)
	bar.Children(func() {
		for _, part := range parts {
			if part.Value <= 0 {
				continue
			}
			ui.Box(c).Grow(part.Value).FillHeight().Background(part.Colour)
		}
	})
	return bar
}
