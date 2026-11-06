// Package design is Remit's design language: the two appearances (paper and
// ink), the theme a window draws with, the scale views size themselves by, and
// the icon set they draw with.
//
// The icons are Lucide's (lucide.dev), vendored in icons/ from
// lucide-static v0.460.0 under the ISC license Lucide ships them with. Only the
// icons the app uses are vendored; they are embedded in the binary and parsed
// once, so the app makes no request for one.
//
// The package imports nothing of the app, so every view may read it.
package design

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Palette is one appearance, named for what each colour is in the product
// rather than for where it is used.
type Palette struct {
	Page       ui.Color // the window's face
	Card       ui.Color // what sits on the page
	Sunken     ui.Color // a field's face, one step below the page
	Border     ui.Color
	Text       ui.Color
	Muted      ui.Color // secondary text
	Faint      ui.Color // hints and disabled text
	Accent     ui.Color
	AccentText ui.Color // text and icons on the accent

	BorderStrong ui.Color // the edge a field shows when it holds input
	Success      ui.Color
	Warning      ui.Color
	Danger       ui.Color
	Mask         ui.Color // behind a panel or a dialog
	// Shadow lifts a card off the page; ShadowPanel lifts what floats over it.
	Shadow      ui.Color
	ShadowPanel ui.Color
}

// Light is the paper appearance: a warm off-white page, so a long note is
// easier on the eye than a full white one.
func Light() Palette {
	p := Palette{
		Page:    ui.Hex("#FAFAF8"),
		Card:    ui.Hex("#FFFFFF"),
		Sunken:  ui.Hex("#F2F0EB"),
		Border:  ui.Hex("#E7E4DD"),
		Text:    ui.Hex("#1B1B19"),
		Muted:   ui.Hex("#6E6B64"),
		Accent:  ui.Hex("#2C5282"),
		Success: ui.Hex("#1F6B3A"),
		Warning: ui.Hex("#8A5B00"),
		Danger:  ui.Hex("#B3261E"),
		Mask:    ui.RGBA(0, 0, 0, 0.28),

		Shadow:      ui.RGBA(0, 0, 0, 0.07),
		ShadowPanel: ui.RGBA(0, 0, 0, 0.16),
	}
	return derived(p)
}

// Dark is the ink appearance: a near-black page, so a card still sits above it
// instead of dissolving into it.
func Dark() Palette {
	p := Palette{
		Page:    ui.Hex("#17181A"),
		Card:    ui.Hex("#1E1F22"),
		Sunken:  ui.Hex("#131416"),
		Border:  ui.Hex("#2E3034"),
		Text:    ui.Hex("#F1F0EC"),
		Muted:   ui.Hex("#9A9993"),
		Accent:  ui.Hex("#7AA2D4"),
		Success: ui.Hex("#7BD88F"),
		Warning: ui.Hex("#E9B949"),
		Danger:  ui.Hex("#FF8A80"),
		Mask:    ui.RGBA(0, 0, 0, 0.5),

		Shadow:      ui.RGBA(0, 0, 0, 0.32),
		ShadowPanel: ui.RGBA(0, 0, 0, 0.45),
	}
	return derived(p)
}

// PaletteOf is the appearance the desktop asked for.
func PaletteOf(dark bool) Palette {
	if dark {
		return Dark()
	}
	return Light()
}

// derived fills in the colours that follow from an appearance's own, so that
// the two palettes above name only what is chosen by hand.
func derived(p Palette) Palette {
	p.Faint = p.Muted.Mix(p.Page, 0.35)
	p.BorderStrong = p.Border.Mix(p.Text, 0.22)
	p.AccentText = inkOn(p.Accent, p.Page)
	return p
}

// Theme resolves Remit's window theme. The caller decides only the appearance:
// the palette, the accent and the radii are Remit's own, so the desktop's
// accent colour never reaches the window.
//
// The font size is the desktop's default; a caller that follows the user's text
// size overrides it with the scale the desktop reported.
func Theme(dark bool) *ui.Theme {
	p := PaletteOf(dark)
	selection := float32(0.25)
	if dark {
		selection = 0.4
	}
	return &ui.Theme{
		Dark:           dark,
		Background:     p.Page,
		Surface:        p.Card,
		SurfaceHover:   p.Card.Mix(p.Text, 0.07),
		SurfacePressed: p.Card.Mix(p.Text, 0.14),
		Border:         p.Border,
		Text:           p.Text,
		TextMuted:      p.Muted,
		Accent:         p.Accent,
		AccentHover:    p.Accent.Mix(p.Text, 0.16),
		AccentPressed:  p.Accent.Mix(p.Text, 0.28),
		AccentText:     p.AccentText,
		Danger:         p.Danger,
		Warning:        p.Warning,
		Success:        p.Success,
		Selection:      p.Accent.Alpha(selection),
		Focus:          p.Accent,
		Scrollbar:      p.Text.Alpha(0.32),
		Radius:         RadiusControl,
		Spacing:        SpaceXS,
		FontSize:       ui.LightTheme().FontSize,
	}
}

// inkOn is the text colour with the most contrast on fill: white, or the
// palette's own page colour where white would not stand out.
func inkOn(fill, page ui.Color) ui.Color {
	white := ui.RGB(255, 255, 255)
	if contrast(white, fill) >= contrast(page, fill) {
		return white
	}
	return page
}

// contrast is the WCAG ratio between two colours, from 1 to 21.
func contrast(a, b ui.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(c ui.Color) float64 {
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

func linear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
