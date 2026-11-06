package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// lineHeight is the one line height the views draw at.
const lineHeight = design.LineHeight

// tokens is what a view draws with: the appearance's theme, and the product
// vocabulary the theme does not carry — the four review ratings, the heat
// scale, a card's link state, the elevation of a card and a panel, and the
// density every view sizes itself by. A colour the theme already names is read
// from the theme, so there is no second palette here.
type tokens struct {
	*ui.Theme

	// sunken is a field's face, one step below the page.
	sunken ui.Color
	// shadow lifts a card; shadowPanel lifts what floats over the page.
	shadow, shadowPanel ui.Color
	// textFaint is for hints, and for what cannot be acted on now.
	textFaint ui.Color
	// borderStrong is the edge a field shows when it holds input.
	borderStrong ui.Color
	// scrim dims the page behind a panel.
	scrim ui.Color

	// The four review ratings.
	reviewAgain ui.Color
	reviewHard  ui.Color
	reviewGood  ui.Color
	reviewEasy  ui.Color

	// Metrics, from the design language's scale.
	spaceXS, spaceS, spaceM, spaceL, spaceXL, spaceXXL float32
	radiusControl, radiusCard, radiusPanel             float32
	// radiusBar is a heat cell's or a chart bar's corner: half a control's,
	// so the smallest square still reads as rounded.
	radiusBar                                   float32
	typeTitle, typeHeading, typeBody, typeSmall float32
	typeMono                                    float32
	control, hairline                           float32
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
	scale := t.FontSize / design.TypeSmall
	if scale <= 0 {
		scale = 1
	}
	return tokens{
		Theme:        t,
		sunken:       p.Sunken,
		shadow:       p.Shadow,
		shadowPanel:  p.ShadowPanel,
		textFaint:    p.Faint,
		borderStrong: p.BorderStrong,
		scrim:        p.Mask,

		reviewAgain: t.Danger,
		reviewHard:  t.Warning,
		reviewGood:  t.Success,
		reviewEasy:  t.Accent,

		spaceXS: design.SpaceXS, spaceS: design.SpaceS, spaceM: design.SpaceM,
		spaceL: design.SpaceL, spaceXL: design.SpaceXL, spaceXXL: design.SpaceXXL,
		radiusControl: design.RadiusControl, radiusCard: design.RadiusCard,
		radiusPanel: design.RadiusPanel, radiusBar: design.RadiusControl / 2,
		typeTitle: design.TypeTitle * scale, typeHeading: design.TypeHeading * scale,
		typeBody: design.TypeBody * scale, typeSmall: design.TypeSmall * scale,
		typeMono: design.TypeMono * scale,
		control:  design.ControlHeight, hairline: design.Hairline,
	}
}

// card is a face that sits on the page: the card colour under a small shadow,
// with no border — the step in tone does the separating, not a line.
func (tk tokens) card(e ui.Element) ui.Element {
	return e.Background(tk.Surface).Radius(tk.radiusCard).Shadow(0, 1, 3, 0, tk.shadow)
}

// panel floats over the page, as the command palette and a dialog do: the card
// colour under a larger shadow.
func (tk tokens) panel(e ui.Element) ui.Element {
	return e.Background(tk.Surface).Radius(tk.radiusPanel).Shadow(0, 8, 24, 0, tk.shadowPanel)
}

// ratingFill is the face of a grade button for its rating colour: the quiet
// paper with a tint of the colour, so the label keeps its contrast.
func (tk tokens) ratingFill(r ui.Color) ui.Color {
	return tk.Surface.Mix(r, 0.16)
}

// ratingEdge is that face's border.
func (tk tokens) ratingEdge(r ui.Color) ui.Color {
	return tk.Border.Mix(r, 0.5)
}

// heatFill is one step of the review heatmap's scale: the quiet paper with
// more and more accent in it, from the legend's empty box to its busiest.
func (tk tokens) heatFill(level int) ui.Color {
	if level <= 0 {
		return tk.sunken
	}
	return tk.sunken.Mix(tk.Accent, 0.16*float32(min(level, 4)))
}

// linkFill and linkInk colour a badge that reports a card's link state.
func (tk tokens) linkFill(r ui.Color) ui.Color { return r.Alpha(0.12) }
func (tk tokens) linkInk(r ui.Color) ui.Color  { return r.Mix(tk.Text, 0.35) }

// m is the catalogs in the language this frame shows, so the view follows the
// window's locale and a locale switch never needs a second code path.
func m(c *ui.Context) locales.Messages { return locales.In(c.Locale()) }
