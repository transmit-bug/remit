package kit

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// Stagger is a list's rows arriving behind each other: the first fades in and
// rises into place, and each row after it starts RowStagger later, for at most
// MaxStaggerRows rows — the rest arrive with the last of them, rather than
// waiting on content that is already there.
//
// Play it on the element that holds the rows, before they are built, and give
// each row its place as it is built:
//
//	col := ui.Column(c)
//	arrive := kit.Stagger(col, "notes")
//	col.Children(func() {
//		ui.List(c, &s.list, len(rows), func(i int) {
//			kit.Arrive(ui.Row(c), arrive(i))
//		})
//	})
//
// The playhead belongs to the holding element, so the list arrives when it
// appears and nothing replays it: not a rebuild, not a scroll, not a keystroke.
// Key tells one list's arrival from another's on the same element.
func Stagger(holder ui.Element, key string) func(row int) float32 {
	tl := holder.Timeline(key, ui.TimelineOptions{})
	if tl == nil {
		return func(int) float32 { return 1 }
	}
	shown := make([]float32, design.MaxStaggerRows)
	for i := range shown {
		shown[i] = tl.Tween(design.RowTween(), ui.AtTime(time.Duration(i)*design.RowStagger))
	}
	return func(row int) float32 {
		return shown[min(max(row, 0), len(shown)-1)]
	}
}

// Arrive puts a row where its list has it: shown is how far the row has come,
// from 0 as it starts to 1 once it is in its place.
func Arrive(row ui.Element, shown float32) ui.Element {
	return row.Opacity(shown).Translate(0, design.Rise*(1-shown))
}

// ArrivePanel brings a panel in over the page: it scales from PanelScale to 1
// while it fades in, over design.Panel. A run of zero plays it once, when the
// panel appears; a run the app counts plays it again whenever that moves.
func ArrivePanel(panel ui.Element, run uint64) ui.Element {
	arriving := panel.Keyframes("arrive", design.PanelFrames, ui.KeyframeOptions{Run: run})
	scale := design.PanelScale + (1-design.PanelScale)*arriving
	return panel.Opacity(arriving).Scale(scale, scale)
}

// ArriveScrim fades the backdrop under a panel in with it, from the same run, so
// that a dialog and the scrim it sits on arrive as one.
func ArriveScrim(backdrop ui.Element, run uint64) ui.Element {
	return backdrop.Opacity(backdrop.Keyframes("arrive", design.PanelFrames, ui.KeyframeOptions{Run: run}))
}

// HoverFace eases a face's colours as the pointer finds it: the hover preset on
// the element whose own colours say where the pointer is. It is the app's own
// faces only — a widget's hover is the widget's.
//
// MyGo keeps a colour easing while the desktop asks for less motion, since a
// colour moves nothing; the design asks for nothing to move at all, so the face
// changes at once instead.
func HoverFace(c *ui.Context, e ui.Element) ui.Element {
	if c.Preferences().ReduceMotion {
		return e
	}
	return e.Transition(ui.ElementTransition{Colors: true, Animation: &design.Hover})
}

// Reveal unfolds an element where it appears — an answer, a summary, the find
// bar — over design.Reveal: it rises Rise DIPs and fades in, once. A parent that
// did not exist in the frame before brings its children in with it, so a reveal
// belongs to what appears inside something that was there.
func Reveal(c *ui.Context, e ui.Element) ui.Element {
	if c.Preferences().ReduceMotion {
		return e
	}
	return e.Transition(ui.ElementTransition{
		Colors:    true,
		Animation: &design.Reveal,
		Enter:     &ui.Motion{Y: design.Rise},
	})
}

// Press shrinks a control to PressScale while it is held, on the face and
// never in the room it takes. It is the app's own controls only — a widget's
// press is the widget's.
func Press(e ui.Element, held bool) ui.Element {
	pose := ui.Pose{Scale: 1}
	if held {
		pose.Scale = design.PressScale
	}
	return e.AnimatePose(pose, design.Press)
}
