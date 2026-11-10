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
	RadiusControl, RadiusCard, RadiusPanel, RadiusBar  float32
	// The type scale: the document's display, a page's heading and a panel's,
	// the paper's body, the chrome's UI, then label and caption. Mono is for
	// paths, templates and logs.
	TypeDisplay, TypeH1, TypeH2, TypeBody float32
	TypeUI, TypeLabel, TypeCaption        float32
	TypeMono                              float32
	// DisplayNarrow is the display size from BreakpointSmall down.
	DisplayNarrow float32
	Control       float32
	// Door is one of the welcome screen's three ways in: taller than a control,
	// so a name and the line under it sit on one face.
	Door float32
	// Hit is the smallest pointer target, a control plus its ring of room.
	Hit      float32
	Hairline float32

	// The paper and the frame around it.
	PaperWidth, Gutter, GutterNarrow, FrameHeight, BarHeight float32
	// SubnavHeight and StatusbarHeight are the two rows the frame adds under
	// the top bar and below the stage.
	SubnavHeight, StatusbarHeight float32
	// RailWidth is the navigation rail open, RailCollapsed the strip it folds
	// to; PanelWidth is a document's side panel.
	RailWidth, RailCollapsed, PanelWidth float32
	// The page measures the full-window screens share; SummaryWidth is a
	// dialog's, as wide as a short page's sentence.
	ListWidth, SettingsWidth, BrowseWidth, WelcomeWidth, SummaryWidth float32
	// NotesColumnWidth and FilterPaneWidth are the two working columns: a
	// document's notes list, and the card table's filters. TableFloor is the
	// narrowest the table goes beside its filters.
	NotesColumnWidth, FilterPaneWidth, TableFloor float32
	// The card table's fixed columns: a row's checkbox, a count, a date and a
	// name.
	CheckColumnWidth, CountColumnWidth, DateColumnWidth, NameColumnWidth float32
	// MeterWidth is a row's paragraph progress bar, and TagFieldWidth the bulk
	// bar's tag field.
	MeterWidth, TagFieldWidth float32
	// The statistics calendar: a day's box in the overview's strip and on the
	// heatmap page's grid, the room between two, and the column the weekday
	// names get.
	HeatCell, HeatCellLarge, HeatGap, HeatWeekday float32
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

	scale := t.FontSize / TypeUI
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
		RadiusPanel: RadiusPanel, RadiusBar: RadiusBar,

		TypeDisplay: TypeDisplay * scale, TypeH1: TypeH1 * scale,
		TypeH2: TypeH2 * scale, TypeBody: TypeBody * scale,
		TypeUI: TypeUI * scale, TypeLabel: TypeLabel * scale,
		TypeCaption: TypeCaption * scale, TypeMono: TypeMono * scale,
		DisplayNarrow: DisplayNarrow * scale,

		Control: ControlHeight, Door: DoorHeight, Hit: Hit, Hairline: Hairline,

		PaperWidth: PaperWidth, Gutter: Gutter, GutterNarrow: GutterNarrow,
		FrameHeight: FrameHeight, BarHeight: BarHeight,
		SubnavHeight: SubnavHeight, StatusbarHeight: StatusbarHeight,
		RailWidth: RailWidth, RailCollapsed: RailCollapsed, PanelWidth: PanelWidth,
		ListWidth: ListWidth, SettingsWidth: SettingsWidth,
		BrowseWidth: BrowseWidth, WelcomeWidth: WelcomeWidth,
		SummaryWidth:     SummaryWidth,
		NotesColumnWidth: NotesColumnWidth, FilterPaneWidth: FilterPaneWidth,
		TableFloor:       TableFloor,
		CheckColumnWidth: CheckColumnWidth, CountColumnWidth: CountColumnWidth,
		DateColumnWidth: DateColumnWidth, NameColumnWidth: NameColumnWidth,
		MeterWidth:    MeterWidth,
		TagFieldWidth: TagFieldWidth,
		HeatCell:      HeatCell, HeatCellLarge: HeatCellLarge,
		HeatGap: HeatGap, HeatWeekday: HeatWeekday,
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
