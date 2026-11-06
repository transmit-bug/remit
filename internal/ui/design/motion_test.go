package design_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// The eight moments carry the durations the design spec's table gives them, and
// nothing else: motion is spec, tuned here.
func TestMomentDurations(t *testing.T) {
	moments := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		// A spring's duration is its own field.
		{"enter", design.Enter.Spring.Duration, 220 * time.Millisecond},
		{"mode switch", design.ModeSwitch.Duration, 180 * time.Millisecond},
		{"reveal", design.Reveal.Spring.Duration, 260 * time.Millisecond},
		{"grade", design.Grade.Duration, 180 * time.Millisecond},
		{"row", design.Row.Spring.Duration, 200 * time.Millisecond},
		{"hover", design.Hover.Duration, 120 * time.Millisecond},
		{"press", design.Press.Duration, 90 * time.Millisecond},
		{"panel", design.Panel.Spring.Duration, 200 * time.Millisecond},
		{"count", design.Count.Duration, 300 * time.Millisecond},
	}
	for _, m := range moments {
		if m.got != m.want {
			t.Errorf("%s takes %v, want %v", m.name, m.got, m.want)
		}
	}
}

// Only the entrance bounces, and only a little: the rest settle where they land,
// so nothing overshoots the line the eye is following.
func TestOnlyTheEntranceBounces(t *testing.T) {
	if b := design.Enter.Spring.Bounce; b <= 0 || b > 0.2 {
		t.Errorf("the entrance bounces %v, want a light bounce", b)
	}
	for _, c := range []struct {
		name   string
		bounce float64
	}{
		{"reveal", design.Reveal.Spring.Bounce},
		{"row", design.Row.Spring.Bounce},
		{"panel", design.Panel.Spring.Bounce},
	} {
		if c.bounce != 0 {
			t.Errorf("%s bounces %v, want a smooth spring", c.name, c.bounce)
		}
	}
}

// The distances the moments move by are the spec's: a 6 DIP rise, a press to
// 0.98, a panel from 0.96, and a 40 ms stagger.
func TestMomentDistances(t *testing.T) {
	if design.Rise != 6 {
		t.Errorf("the rise is %v DIP, want 6", design.Rise)
	}
	if design.PressScale != 0.98 {
		t.Errorf("a pressed control scales to %v, want 0.98", design.PressScale)
	}
	if design.PanelScale != 0.96 {
		t.Errorf("a panel arrives at %v, want 0.96", design.PanelScale)
	}
	if design.EnterStagger != 40*time.Millisecond {
		t.Errorf("the entrance staggers by %v, want 40ms", design.EnterStagger)
	}
}

// A preset played as keyframes runs forward in time and ends at rest, so a run
// that changes plays once and stops.
func TestFramesRunForward(t *testing.T) {
	frames := map[string][]ui.Keyframe{
		"entrance": design.EnterFrames,
		"mode":     design.ModeFrames,
		"panel":    design.PanelFrames,
	}
	for name, fs := range frames {
		if len(fs) < 2 {
			t.Errorf("%s has %d frames, want at least a start and an end", name, len(fs))
			continue
		}
		if fs[0].At != 0 || fs[0].Value != 0 {
			t.Errorf("%s starts at %v after %v, want 0 after 0", name, fs[0].Value, fs[0].At)
		}
		last := fs[len(fs)-1]
		if last.Value != 1 {
			t.Errorf("%s ends at %v, want 1", name, last.Value)
		}
		for i := 1; i < len(fs); i++ {
			if fs[i].At <= fs[i-1].At {
				t.Errorf("%s frame %d is at %v, not after %v", name, i, fs[i].At, fs[i-1].At)
			}
		}
		if last.At != design.ModeSwitch.Duration && name == "mode" {
			t.Errorf("the mode switch plays for %v, its preset says %v", last.At, design.ModeSwitch.Duration)
		}
	}
}
