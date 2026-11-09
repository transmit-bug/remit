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

// ProgressOptions is a bar of one value: how far along it is from 0 to 1, the
// room it takes, the tone its level chooses, and the name assistive technology
// reads it by. A width or height of zero is the row's width and the theme's bar
// height.
type ProgressOptions struct {
	Value  float64
	Levels *ui.MeterLevels
	Width  float32
	Height float32
	Label  string
}

// Progress draws a bar that fills to its value once, when it mounts, over
// design.Progress: the room it takes is its own from the first frame and only
// the fill grows, so a bar arriving moves nothing around it. A value that
// changes after that is drawn at once — the bar fills once, not on every
// change.
//
// Composed rather than taken from a widget: ui.Meter paints a fixed fraction and
// offers no way to animate its fill (checked against MyGo v0.8.1). The face is
// the meter's — the track in the theme's border, the fill in the colour its
// levels choose — so a page reads the same bar whether it grows or not.
func Progress(c *ui.Context, o ProgressOptions) ui.Element {
	t := c.Theme()
	value := min(max(o.Value, 0), 1)
	tone := progressTone(t, value, o.Levels)
	height := o.Height
	if height <= 0 {
		height = t.Space(2)
	}
	bar := ui.Box(c).Height(height).Radius(design.RadiusBar).
		Background(t.Border).Clip().Role(ui.RoleMeter)
	if o.Width > 0 {
		bar.Width(o.Width)
	} else {
		bar.FillWidth()
	}
	if o.Label != "" {
		bar.Label(o.Label)
	}
	// A value read from the context belongs to the build; the drawing runs later
	// and only paints what this frame decided.
	backwards := c.Direction() == ui.RightToLeft
	fill := bar.Keyframes("fill", design.ProgressFrames)
	bar.Draw(func(p *ui.Painter, r ui.Rect) {
		w := r.W * float32(value) * fill
		if w <= 0 {
			return
		}
		x := r.X
		if backwards {
			x = r.X + r.W - w
		}
		p.Fill(ui.Rect{X: x, Y: r.Y, W: w, H: r.H}, tone, design.RadiusBar)
	})
	return bar
}

// progressTone is the fill's colour: the meter's own reading of its levels, so a
// bar past its budget is the same tone either widget draws it in.
func progressTone(t *ui.Theme, value float64, levels *ui.MeterLevels) ui.Color {
	if levels == nil || levels.Warning == levels.Critical {
		return t.Success
	}
	low := levels.Critical < levels.Warning
	switch {
	case !low && value >= levels.Critical, low && value <= levels.Critical:
		return t.Danger
	case !low && value >= levels.Warning, low && value <= levels.Warning:
		return t.Warning
	}
	return t.Success
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
