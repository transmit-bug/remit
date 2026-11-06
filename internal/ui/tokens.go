package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// Remit runs at one comfortable density: a 1.5 line height, a 32 DIP control,
// radii of 6, 10 and 14, and the spacing and type scales below. The values are
// the design language's; views name them here so that none spells one out.
const (
	spaceXS  = design.SpaceXS
	spaceS   = design.SpaceS
	spaceM   = design.SpaceM
	spaceL   = design.SpaceL
	spaceXL  = design.SpaceXL
	spaceXXL = design.SpaceXXL

	radiusControl = design.RadiusControl
	radiusCard    = design.RadiusCard
	radiusPanel   = design.RadiusPanel

	typeTitle   = design.TypeTitle
	typeHeading = design.TypeHeading
	typeBody    = design.TypeBody
	typeSmall   = design.TypeSmall
	typeMono    = design.TypeMono

	controlHeight = design.ControlHeight
	hairline      = design.Hairline
	lineHeight    = design.LineHeight
)

// tokens is the semantic set the views draw with: the colours resolved from
// the window's theme, and the density above scaled by the desktop's text size.
type tokens struct {
	// Surfaces, from the page to what sits in it.
	surface       ui.Color
	surfaceRaised ui.Color
	surfaceSunken ui.Color
	scrim         ui.Color

	// Borders.
	border       ui.Color
	borderStrong ui.Color

	// Text.
	text      ui.Color
	textMuted ui.Color
	textFaint ui.Color
	onAccent  ui.Color

	// Meaning.
	accent  ui.Color
	success ui.Color
	warning ui.Color
	danger  ui.Color

	// The four review ratings.
	reviewAgain ui.Color
	reviewHard  ui.Color
	reviewGood  ui.Color
	reviewEasy  ui.Color

	// Metrics, from the scale above.
	spaceXS, spaceS, spaceM, spaceL, spaceXL, spaceXXL float32
	radiusControl, radiusCard, radiusPanel             float32
	typeTitle, typeHeading, typeBody, typeSmall        float32
	typeMono                                           float32
	control, hairline                                  float32
}

// appTheme resolves Remit's appearance for this frame: the design language's
// palette, at the text scale the desktop reported. The desktop decides light or
// dark and how large text is; nothing else of it reaches the window.
func appTheme(base *ui.Theme) (*ui.Theme, design.Palette) {
	t := design.Theme(base.Dark)
	t.FontSize = base.FontSize
	return t, design.PaletteOf(base.Dark)
}

// newTokens resolves the semantic tokens from the appearance p and its theme t.
// Type sizes keep the desktop's text scale by following the theme's font size,
// without letting the body text become the size of a control's label.
func newTokens(t *ui.Theme, p design.Palette) tokens {
	scale := t.FontSize / typeSmall
	if scale <= 0 {
		scale = 1
	}
	return tokens{
		surface:       p.Page,
		surfaceRaised: p.Card,
		surfaceSunken: p.Sunken,
		scrim:         p.Mask,

		border:       p.Border,
		borderStrong: p.BorderStrong,

		text:      p.Text,
		textMuted: p.Muted,
		textFaint: p.Faint,
		onAccent:  p.AccentText,

		accent:  p.Accent,
		success: p.Success,
		warning: p.Warning,
		danger:  p.Danger,

		reviewAgain: p.Danger,
		reviewHard:  p.Warning,
		reviewGood:  p.Success,
		reviewEasy:  p.Accent,

		spaceXS: spaceXS, spaceS: spaceS, spaceM: spaceM,
		spaceL: spaceL, spaceXL: spaceXL, spaceXXL: spaceXXL,
		radiusControl: radiusControl, radiusCard: radiusCard, radiusPanel: radiusPanel,
		typeTitle: typeTitle * scale, typeHeading: typeHeading * scale,
		typeBody: typeBody * scale, typeSmall: typeSmall * scale,
		typeMono: typeMono * scale,
		control:  controlHeight, hairline: hairline,
	}
}

// ratingFill is the face of a grade button for its rating colour: the quiet
// paper with a tint of the colour, so the label keeps its contrast.
func (tk tokens) ratingFill(r ui.Color) ui.Color {
	return tk.surfaceRaised.Mix(r, 0.16)
}

// ratingEdge is that face's border.
func (tk tokens) ratingEdge(r ui.Color) ui.Color {
	return tk.border.Mix(r, 0.5)
}

// heatFill is one step of the review heatmap's scale: the quiet paper with
// more and more accent in it, from the legend's empty box to its busiest.
func (tk tokens) heatFill(level int) ui.Color {
	if level <= 0 {
		return tk.surfaceSunken
	}
	return tk.surfaceSunken.Mix(tk.accent, 0.16*float32(min(level, 4)))
}

// linkFill and linkInk colour a badge that reports a card's link state.
func (tk tokens) linkFill(r ui.Color) ui.Color { return r.Alpha(0.12) }
func (tk tokens) linkInk(r ui.Color) ui.Color  { return r.Mix(tk.text, 0.35) }

// m is the catalogs in the language this frame shows, so the view follows the
// window's locale and a locale switch never needs a second code path.
func m(c *ui.Context) locales.Messages { return locales.In(c.Locale()) }
