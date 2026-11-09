package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// Every moment the window plays stops: after its preset has run, the view asks
// for no more frames.
func TestMotionSettles(t *testing.T) {
	_, _, tt := openApp(t)

	// A screen change: the entrance of the screen it switches to.
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	if tt.FramePending() {
		t.Fatal("the screen's change never settled")
	}

	// A panel opening and closing: the palette arrives, leaves, and the view
	// rests. (The palette's field blinks its caret while it is open — the
	// framework's own loop, not this motion's.)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Advance(320 * time.Millisecond) // the palette's arrival
	tt.Key(0, ui.KeyEscape)
	tt.Advance(320 * time.Millisecond) // the palette's exit
	if tt.FramePending() {
		t.Fatal("the command palette's arrival never settled")
	}
}

// With the desktop's reduced motion asked for, a screen change does not travel:
// where the same switch runs for its preset's 180 ms otherwise, it is over by
// the next frame.
func TestReducedMotionSwitchesAtOnce(t *testing.T) {
	// Both windows start on the notes, so the switch leaves no review card
	// behind: only the screen's own entrance is in question.
	open := func(reduce bool) *ui.Tester {
		t.Helper()
		a, _ := newTestApp(t)
		tt := ui.NewTester(a.View, 1024, 720)
		if reduce {
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		}
		settle(tt)
		tt.Key(ui.Cmd, ui.Key2)
		tt.Advance(300 * time.Millisecond) // the review card's exit
		return tt
	}

	// The search page rather than the card table: a table's first layout keeps
	// a frame request of its own for a moment (MyGo v0.8.1), which would
	// drown the screen change this test is about.
	moving := open(false)
	searchPage(t, moving, "en") // notes to the search page
	moving.Frame()
	if !moving.FramePending() {
		t.Fatal("the screen change runs no motion at all: nothing to compare the reduced motion against")
	}

	still := open(true)
	searchPage(t, still, "en")
	still.Frame()
	if still.FramePending() {
		t.Error("a screen change still travels with reduced motion asked for")
	}
}

// A control's hover and press move it on the face, not in the layout: the box is
// the same size before, under the pointer, and pressed.
func TestHoverAndPressKeepTheLayout(t *testing.T) {
	_, _, tt := openApp(t)
	recallQueue(tt)

	// The grades are up only once the answer is.
	tt.Key(0, ui.KeySpace)
	tt.Advance(300 * time.Millisecond) // the answer unfolding
	box, ok := tt.Find("Again")
	if !ok {
		t.Fatalf("no grade button; texts %q", tt.Texts())
	}
	size := func() (float32, float32) {
		t.Helper()
		got, ok := tt.Find("Again")
		if !ok {
			t.Fatal("the grade button went away")
		}
		return got.W, got.H
	}

	x, y := box.X+box.W/2, box.Y+box.H/2
	tt.Move(x, y)
	tt.Advance(design.Hover.Duration)
	if w, h := size(); w != box.W || h != box.H {
		t.Errorf("hovering changed the grade button from %vx%v to %vx%v", box.W, box.H, w, h)
	}

	tt.Press(x, y)
	tt.Advance(design.Press.Duration)
	if w, h := size(); w != box.W || h != box.H {
		t.Errorf("pressing changed the grade button from %vx%v to %vx%v", box.W, box.H, w, h)
	}
	tt.Release(x, y)
}
