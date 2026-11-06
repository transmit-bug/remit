package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// Remit runs at one comfortable density: a 1.5 line height, a 32 DIP control,
// radii of 6, 10 and 14, and the spacing and type scales below. Sizes and
// colours are named here so that no view spells one out.
const (
	spaceXS  float32 = 4
	spaceS   float32 = 8
	spaceM   float32 = 12
	spaceL   float32 = 16
	spaceXL  float32 = 24
	spaceXXL float32 = 32

	radiusControl float32 = 6
	radiusCard    float32 = 10
	radiusPanel   float32 = 14

	typeTitle   float32 = 22
	typeHeading float32 = 17
	typeBody    float32 = 15
	typeSmall   float32 = 13
	typeMono    float32 = 14

	controlHeight float32 = 32
	hairline      float32 = 1
	lineHeight    float32 = 1.5
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

// appTheme returns Remit's window theme: the desktop's, at Remit's density.
func appTheme(base *ui.Theme) *ui.Theme {
	t := *base
	t.Spacing = spaceXS
	t.Radius = radiusControl
	return &t
}

// newTokens resolves the semantic tokens from t. Type sizes keep the
// desktop's text scale by following the theme's font size, without letting the
// body text become the size of a control's label.
func newTokens(t *ui.Theme) tokens {
	scale := t.FontSize / typeSmall
	if scale <= 0 {
		scale = 1
	}
	return tokens{
		surface:       t.Background,
		surfaceRaised: t.Surface,
		surfaceSunken: sunken(t),
		scrim:         ui.RGBA(0, 0, 0, 0.28),

		border:       t.Border,
		borderStrong: t.Border.Mix(t.Text, 0.22),

		text:      t.Text,
		textMuted: t.TextMuted,
		textFaint: t.TextMuted.Mix(t.Background, 0.35),
		onAccent:  t.AccentText,

		accent:  t.Accent,
		success: t.Success,
		warning: t.Warning,
		danger:  t.Danger,

		reviewAgain: t.Danger,
		reviewHard:  t.Warning,
		reviewGood:  t.Success,
		reviewEasy:  t.Accent,

		spaceXS: spaceXS, spaceS: spaceS, spaceM: spaceM,
		spaceL: spaceL, spaceXL: spaceXL, spaceXXL: spaceXXL,
		radiusControl: radiusControl, radiusCard: radiusCard, radiusPanel: radiusPanel,
		typeTitle: typeTitle * scale, typeHeading: typeHeading * scale,
		typeBody: typeBody * scale, typeSmall: typeSmall * scale,
		typeMono: typeMono * scale,
		control:  controlHeight, hairline: hairline,
	}
}

// sunken is a field's face: one step below the page in either appearance.
func sunken(t *ui.Theme) ui.Color {
	if t.Dark {
		return t.Background.Mix(ui.RGB(0, 0, 0), 0.45)
	}
	return t.Background.Mix(t.Text, 0.06)
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

// linkFill and linkInk colour a badge that reports a card's link state.
func (tk tokens) linkFill(r ui.Color) ui.Color { return r.Alpha(0.12) }
func (tk tokens) linkInk(r ui.Color) ui.Color  { return r.Mix(tk.text, 0.35) }

// m is the catalogs in the language this frame shows, so the view follows the
// window's locale and a locale switch never needs a second code path.
func m(c *ui.Context) locales.Messages { return locales.In(c.Locale()) }
