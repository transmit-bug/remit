package design

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The eight moments the app moves in, each a named preset so that a screen
// names a cause rather than a duration, and the whole vocabulary is tuned in one
// place. Motion encodes cause only — where something came from, that it was
// clicked, that the screen changed — never decoration.
const (
	enterDuration  = 220 * time.Millisecond // the first frame, coming down
	modeDuration   = 180 * time.Millisecond // one mode leaving for another
	revealDuration = 260 * time.Millisecond // the answer unfolding
	gradeDuration  = 180 * time.Millisecond // one card out, the next in
	rowDuration    = 200 * time.Millisecond // a row folding or unfolding
	hoverDuration  = 120 * time.Millisecond // a face under the pointer
	pressDuration  = 90 * time.Millisecond  // a control taking a press
	panelDuration  = 200 * time.Millisecond // a panel or a dialog arriving
	countDuration  = 300 * time.Millisecond // a count rolling to its number
)

// EnterStagger is how long the main area waits behind the toolbar on the first
// frame, so that the two arrive one after the other rather than as one block.
const EnterStagger = 40 * time.Millisecond

// The distances and scales the moments move by.
const (
	// Rise is how far the window's content comes down as it appears, and how
	// far a new mode's screen rises.
	Rise float32 = 6
	// PressScale is how far a pressed control shrinks: it does not grow, and
	// never changes the room it takes.
	PressScale float32 = 0.98
	// PanelScale is how small a panel is as it arrives over the page.
	PanelScale float32 = 0.96
)

var (
	// Enter is the first frame: the toolbar, and the main area behind it,
	// come down Rise DIPs and fade in, with the slight bounce of a spring.
	Enter = ui.Animation{Spring: ui.SpringOptions{Duration: enterDuration, Bounce: 0.12}}
	// ModeSwitch is one screen replacing another: the root fades in and rises
	// Rise DIPs, easing out.
	ModeSwitch = ui.Animation{Duration: modeDuration, Ease: ui.EaseOut}
	// Reveal unfolds the answer to its height: a smooth spring, no bounce, so
	// the eye does not lose the line it was on.
	Reveal = ui.Animation{Spring: ui.SpringOptions{Duration: revealDuration}}
	// Grade sends the graded card up and fades it, and brings the next one up
	// from below.
	Grade = ui.Animation{Duration: gradeDuration, Ease: ui.EaseOut}
	// Row folds or unfolds a row's height in a list.
	Row = ui.Animation{Spring: ui.SpringOptions{Duration: rowDuration}}
	// Hover eases a face's colour under the pointer.
	Hover = ui.Animation{Duration: hoverDuration, Ease: ui.EaseOut}
	// Press shrinks a control to PressScale for as long as it is held.
	Press = ui.Animation{Duration: pressDuration, Ease: ui.EaseOut}
	// Panel brings a panel or a dialog in: a little larger, over its scrim,
	// on a smooth spring.
	Panel = ui.Animation{Spring: ui.SpringOptions{Duration: panelDuration}}
	// Count rolls a number to its new value instead of swapping the digits.
	Count = ui.Animation{Duration: countDuration, Ease: ui.EaseOut}
)

// EnterFrames is the entrance as a progress from 0 to 1, for the values a
// spring cannot drive through Keyframes: it overshoots a little on the way, as
// the spring would.
var EnterFrames = []ui.Keyframe{
	{Value: 0},
	{At: enterDuration - 40*time.Millisecond, Value: 1.01, Ease: ui.EaseOut},
	{At: enterDuration, Value: 1},
}

// ModeFrames is a mode switch as a progress from 0 to 1, easing out.
var ModeFrames = []ui.Keyframe{
	{Value: 0},
	{At: modeDuration, Value: 1, Ease: ui.EaseOut},
}

// PanelFrames is a panel arriving as a progress from 0 to 1: fast out of the
// gate, then settling.
var PanelFrames = []ui.Keyframe{
	{Value: 0},
	{At: panelDuration, Value: 1, Ease: ui.EaseOut},
}
