package design_test

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// appearances pairs the desktop's setting with what it must resolve to.
var appearances = []struct {
	name  string
	dark  bool
	pal   func() design.Palette
	theme func() *ui.Theme
}{
	{"paper", false, design.Light, func() *ui.Theme { return design.Theme(false) }},
	{"ink", true, design.Dark, func() *ui.Theme { return design.Theme(true) }},
}

// The body text and the secondary text both read on the page and on a card in
// both appearances: 4.5:1 is the floor for text this size.
func TestTextContrast(t *testing.T) {
	surfaces := map[string]func(design.Palette) ui.Color{
		"page": func(p design.Palette) ui.Color { return p.Page },
		"card": func(p design.Palette) ui.Color { return p.Card },
	}
	texts := map[string]func(design.Palette) ui.Color{
		"text":  func(p design.Palette) ui.Color { return p.Text },
		"muted": func(p design.Palette) ui.Color { return p.Muted },
	}
	for _, a := range appearances {
		p := a.pal()
		for sName, surface := range surfaces {
			for tName, text := range texts {
				got := ratio(text(p), surface(p))
				if got < 4.5 {
					t.Errorf("%s: %s on %s is %.2f:1, want at least 4.5:1", a.name, tName, sName, got)
				}
			}
		}
	}
}

// The semantic colours are drawn as text as well as as tints, so they read on
// either surface.
func TestSemanticContrast(t *testing.T) {
	colors := map[string]func(design.Palette) ui.Color{
		"accent":  func(p design.Palette) ui.Color { return p.Accent },
		"success": func(p design.Palette) ui.Color { return p.Success },
		"warning": func(p design.Palette) ui.Color { return p.Warning },
		"danger":  func(p design.Palette) ui.Color { return p.Danger },
	}
	for _, a := range appearances {
		p := a.pal()
		for name, color := range colors {
			for surfaceName, surface := range map[string]ui.Color{"page": p.Page, "card": p.Card} {
				if got := ratio(color(p), surface); got < 4.5 {
					t.Errorf("%s: %s on %s is %.2f:1, want at least 4.5:1", a.name, name, surfaceName, got)
				}
			}
		}
	}
}

// Text on the accent stays readable whichever appearance picked the accent.
func TestAccentTextContrast(t *testing.T) {
	for _, a := range appearances {
		p := a.pal()
		if got := ratio(p.AccentText, p.Accent); got < 4.5 {
			t.Errorf("%s: accent text on accent is %.2f:1, want at least 4.5:1", a.name, got)
		}
	}
}

// An appearance is a palette plus the theme resolved from it; the caller's only
// decision is which appearance.
func TestThemeResolvesAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "paper", true: "ink"}[dark], func(t *testing.T) {
			p := design.PaletteOf(dark)
			th := design.Theme(dark)
			if th.Dark != dark {
				t.Errorf("Dark is %v, want %v", th.Dark, dark)
			}
			for name, pair := range map[string][2]ui.Color{
				"background":  {th.Background, p.Page},
				"surface":     {th.Surface, p.Card},
				"border":      {th.Border, p.Border},
				"text":        {th.Text, p.Text},
				"text muted":  {th.TextMuted, p.Muted},
				"accent":      {th.Accent, p.Accent},
				"accent text": {th.AccentText, p.AccentText},
				"success":     {th.Success, p.Success},
				"warning":     {th.Warning, p.Warning},
				"danger":      {th.Danger, p.Danger},
			} {
				if !same(pair[0], pair[1]) {
					t.Errorf("theme %s is %v, want the palette's %v", name, pair[0], pair[1])
				}
			}
			if th.Radius != design.RadiusControl {
				t.Errorf("radius is %v, want %v", th.Radius, design.RadiusControl)
			}
			if th.Spacing != design.SpaceXS {
				t.Errorf("spacing is %v, want %v", th.Spacing, design.SpaceXS)
			}
			if th.FontSize <= 0 {
				t.Errorf("font size is %v, want the desktop's default", th.FontSize)
			}
		})
	}
}

// The accent is Remit's, not the desktop's: the theme is resolved from the
// appearance alone, and it is a colour the framework's own themes do not have.
func TestAccentIsRemitOwn(t *testing.T) {
	for _, c := range []struct {
		name  string
		got   ui.Color
		want  ui.Color
		other ui.Color
	}{
		{"paper", design.Theme(false).Accent, design.Light().Accent, ui.LightTheme().Accent},
		{"ink", design.Theme(true).Accent, design.Dark().Accent, ui.DarkTheme().Accent},
	} {
		if !same(c.got, c.want) {
			t.Errorf("%s: accent is %v, want the palette's %v", c.name, c.got, c.want)
		}
		if same(c.got, c.other) {
			t.Errorf("%s: accent is the framework's default %v", c.name, c.other)
		}
	}
}

// Every colour the framework draws with is set: a missed field would fall back
// to a framework default and break the appearance in one corner of the window.
func TestThemeIsComplete(t *testing.T) {
	for _, a := range appearances {
		th := a.theme()
		fields := map[string]ui.Color{
			"background": th.Background, "surface": th.Surface,
			"surface hover": th.SurfaceHover, "surface pressed": th.SurfacePressed,
			"border": th.Border, "text": th.Text, "text muted": th.TextMuted,
			"accent": th.Accent, "accent hover": th.AccentHover,
			"accent pressed": th.AccentPressed, "accent text": th.AccentText,
			"danger": th.Danger, "warning": th.Warning, "success": th.Success,
			"selection": th.Selection, "focus": th.Focus, "scrollbar": th.Scrollbar,
		}
		for name, c := range fields {
			if c.A == 0 {
				t.Errorf("%s: %s has no alpha", a.name, name)
			}
		}
	}
}

// The scale is one density, three radii and the type scale, each size larger
// than the one after it.
func TestScale(t *testing.T) {
	radii := []float32{design.RadiusControl, design.RadiusCard, design.RadiusPanel}
	if want := []float32{6, 10, 14}; !equal(radii, want) {
		t.Errorf("radii are %v, want %v", radii, want)
	}
	types := []float32{
		design.TypeDisplay, design.TypeH1, design.TypeH2, design.TypeBody,
		design.TypeUI, design.TypeLabel, design.TypeCaption,
	}
	if want := []float32{34, 27, 22, 18, 13, 12, 11}; !equal(types, want) {
		t.Errorf("type sizes are %v, want %v", types, want)
	}
	spaces := []float32{design.SpaceXS, design.SpaceS, design.SpaceM, design.SpaceL, design.SpaceXL, design.SpaceXXL}
	for i := 1; i < len(spaces); i++ {
		if spaces[i] <= spaces[i-1] {
			t.Errorf("spaces are not ascending: %v", spaces)
		}
	}
	if design.LineHeight != 1.45 {
		t.Errorf("line height is %v, want 1.45", design.LineHeight)
	}
}

// derived colours are filled in, so a view never has to mix its own.
func TestDerivedColors(t *testing.T) {
	for _, a := range appearances {
		p := a.pal()
		for name, c := range map[string]ui.Color{
			"faint": p.Faint, "border strong": p.BorderStrong,
			"accent text": p.AccentText, "mask": p.Mask,
			"shadow": p.Shadow, "panel shadow": p.ShadowPanel,
		} {
			if c.A == 0 {
				t.Errorf("%s: %s has no alpha", a.name, name)
			}
		}
		if ratio(p.Faint, p.Page) <= 1 {
			t.Errorf("%s: faint text is the page's own colour", a.name)
		}
	}
}

// The three planes of the elevation are told apart: a card's shadow is lighter
// than the one under a panel that floats over it.
func TestElevation(t *testing.T) {
	for _, a := range appearances {
		p := a.pal()
		if p.Shadow.A >= p.ShadowPanel.A {
			t.Errorf("%s: a card's shadow %v is not lighter than a panel's %v", a.name, p.Shadow, p.ShadowPanel)
		}
		if p.ShadowPanel.A >= p.Mask.A {
			t.Errorf("%s: a panel's shadow is as heavy as the scrim behind it", a.name)
		}
	}
}

func same(a, b ui.Color) bool {
	return a.R == b.R && a.G == b.G && a.B == b.B && a.A == b.A
}

func equal(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ratio is the WCAG contrast ratio, written out here rather than borrowed from
// the package under test.
func ratio(a, b ui.Color) float64 {
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func lum(c ui.Color) float64 {
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func channel(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
