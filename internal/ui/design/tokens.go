package design

import (
	"github.com/egoist/mygo/ui"
)

// Tokens is what a view draws with: the appearance's theme, and the product
// vocabulary the theme does not carry — the four review ratings, the heat
// scale, a card's link state, the elevation of a card and a panel, and the
// density every view sizes itself by. A colour the theme already names is read
// from the theme, so there is no second palette here.
type Tokens struct {
	*ui.Theme

	// Sunken is a field's face, one step below the page.
	Sunken ui.Color
	// Shadow lifts a card; ShadowPanel lifts what floats over the page.
	Shadow, ShadowPanel ui.Color
	// TextFaint is for hints, and for what cannot be acted on now.
	TextFaint ui.Color
	// BorderStrong is the edge a field shows when it holds input.
	BorderStrong ui.Color
	// Scrim dims the page behind a panel.
	Scrim ui.Color

	// The four review ratings.
	ReviewAgain ui.Color
	ReviewHard  ui.Color
	ReviewGood  ui.Color
	ReviewEasy  ui.Color

	// Metrics, from the design language's scale.
	SpaceXS, SpaceS, SpaceM, SpaceL, SpaceXL, SpaceXXL float32
	RadiusControl, RadiusCard, RadiusPanel             float32
	// RadiusBar is a heat cell's or a chart bar's corner: half a control's,
	// so the smallest square still reads as rounded.
	RadiusBar                                   float32
	TypeTitle, TypeHeading, TypeBody, TypeSmall float32
	TypeMono                                    float32
	Control, Hairline                           float32
	// The document's scale and the paper it sits on.
	TypeDisplay, TypeChrome, TypeLabel, TypeCaption float32
	PaperWidth, Gutter, FrameHeight, BarHeight      float32
}

// NewTokens resolves Remit's appearance for this frame: the palette at the text
// scale the desktop reported, and the semantic vocabulary the views read. The
// desktop decides light or dark and how large text is; nothing else of it
// reaches the window. The theme is the Tokens' own — a caller that must set it
// hands tk.Theme to the context.
//
// Type sizes keep the desktop's text scale by following the theme's font size,
// without letting the body text become the size of a control's label.
func NewTokens(base *ui.Theme) Tokens {
	t := Theme(base.Dark)
	t.FontSize = base.FontSize
	p := PaletteOf(base.Dark)

	scale := t.FontSize / TypeSmall
	if scale <= 0 {
		scale = 1
	}
	return Tokens{
		Theme:        t,
		Sunken:       p.Sunken,
		Shadow:       p.Shadow,
		ShadowPanel:  p.ShadowPanel,
		TextFaint:    p.Faint,
		BorderStrong: p.BorderStrong,
		Scrim:        p.Mask,

		ReviewAgain: t.Danger,
		ReviewHard:  t.Warning,
		ReviewGood:  t.Success,
		ReviewEasy:  t.Accent,

		SpaceXS: SpaceXS, SpaceS: SpaceS, SpaceM: SpaceM,
		SpaceL: SpaceL, SpaceXL: SpaceXL, SpaceXXL: SpaceXXL,
		RadiusControl: RadiusControl, RadiusCard: RadiusCard,
		RadiusPanel: RadiusPanel, RadiusBar: RadiusControl / 2,
		TypeTitle: TypeTitle * scale, TypeHeading: TypeHeading * scale,
		TypeBody: TypeBody * scale, TypeSmall: TypeSmall * scale,
		TypeMono: TypeMono * scale,
		Control:  ControlHeight, Hairline: Hairline,

		TypeDisplay: TypeDisplay * scale, TypeChrome: TypeChrome * scale,
		TypeLabel: TypeLabel * scale, TypeCaption: TypeCaption * scale,
		PaperWidth: PaperWidth, Gutter: Gutter,
		FrameHeight: FrameHeight, BarHeight: BarHeight,
	}
}

// Icon draws the named icon at size DIPs, in the text colour of whatever holds
// it.
func (tk Tokens) Icon(c *ui.Context, name string, size float32) ui.Element {
	return ui.Icon(c, Icon(name)).Width(size).Height(size)
}

// IconText is an icon and a label on one line, for a button or a badge: the
// icon takes the colour of the text around it.
func (tk Tokens) IconText(c *ui.Context, name, text string, size float32) ui.Element {
	return ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceXS).Children(func() {
		tk.Icon(c, name, size)
		ui.Text(c, text).SingleLine()
	})
}

// RatingFill is the face of a grade button for its rating colour: the quiet
// paper with a tint of the colour, so the label keeps its contrast.
func (tk Tokens) RatingFill(r ui.Color) ui.Color {
	return tk.Surface.Mix(r, 0.16)
}

// RatingEdge is that face's border.
func (tk Tokens) RatingEdge(r ui.Color) ui.Color {
	return tk.Border.Mix(r, 0.5)
}

// HeatFill is one step of the review heatmap's scale: the quiet paper with more
// and more accent in it, from the legend's empty box to its busiest.
func (tk Tokens) HeatFill(level int) ui.Color {
	if level <= 0 {
		return tk.Sunken
	}
	return tk.Sunken.Mix(tk.Accent, 0.16*float32(min(level, 4)))
}

// LinkFill and LinkInk colour a badge that reports a card's link state.
func (tk Tokens) LinkFill(r ui.Color) ui.Color { return r.Alpha(0.12) }
func (tk Tokens) LinkInk(r ui.Color) ui.Color  { return r.Mix(tk.Text, 0.35) }
