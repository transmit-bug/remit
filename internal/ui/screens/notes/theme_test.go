package notes

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// The written note follows the desktop's text size like the chrome around it:
// turning text up makes the body larger, not only the controls.
func TestNoteFontFollowsDesktopTextScale(t *testing.T) {
	plain := design.NewTokens(&ui.Theme{FontSize: design.TypeUI})
	scaled := design.NewTokens(&ui.Theme{FontSize: design.TypeUI * 1.5})

	if got, want := noteFont(scaled).Size, noteFont(plain).Size*1.5; math.Abs(float64(got-want)) > 0.001 {
		t.Errorf("body at 1.5x is %v, want %v", got, want)
	}
	if noteFont(scaled).LineHeight != design.TextLineHeight {
		t.Errorf("body line height is %v, want the paper's %v", noteFont(scaled).LineHeight, design.TextLineHeight)
	}
}
