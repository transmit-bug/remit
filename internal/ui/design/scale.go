package design

// The scale every view sizes itself by: one comfortable density, four radii,
// the type scale, and the frame's heights and breaks. Views name these through
// the window's tokens rather than spelling out a number.
const (
	SpaceXS  float32 = 4
	SpaceS   float32 = 8
	SpaceM   float32 = 12
	SpaceL   float32 = 16
	SpaceXL  float32 = 24
	SpaceXXL float32 = 32

	RadiusControl float32 = 6
	RadiusCard    float32 = 10
	RadiusPanel   float32 = 14
	// RadiusBar is a heat cell's or a chart bar's corner.
	RadiusBar float32 = 3

	// The type scale, largest to smallest. Body is the paper's own size and the
	// card's answer; everything the user operates sits at UI; Label and Caption
	// name and annotate what UI says.
	TypeDisplay float32 = 34
	TypeH1      float32 = 27
	TypeH2      float32 = 22
	TypeBody    float32 = 18
	TypeUI      float32 = 13
	TypeLabel   float32 = 12
	TypeCaption float32 = 11
	TypeMono    float32 = 13

	// DisplayNarrow is a page title's size once the window is at
	// BreakpointSmall or below, so the title does not crowd the paper.
	DisplayNarrow float32 = 30

	// The paper's measure and the room beside it.
	PaperWidth float32 = 760
	Gutter     float32 = 48
	// GutterNarrow is that room at BreakpointSmall and below.
	GutterNarrow float32 = 20

	// ReadingWidth, ReviewWidth, SettingsWidth and BrowseWidth are the page
	// measures the full-window screens use. They live here so a screen never
	// invents its own column and the review summary shares the card's axis.
	ReadingWidth  float32 = 760
	ReviewWidth   float32 = 680
	SettingsWidth float32 = 660
	BrowseWidth   float32 = 1080
	// ListWidth is a list page's measure, narrower than a table's.
	ListWidth float32 = 860
	// WelcomeWidth is the start page's wider card: the one place a door is
	// worth more than a sentence.
	WelcomeWidth float32 = 640
	// SummaryWidth is the completed review card inside ReviewWidth, and
	// MessageWidth is a short page's sentence and action.
	SummaryWidth float32 = 520
	MessageWidth float32 = 520
	// PanelWidth is a document's side panel: outline or backlinks.
	PanelWidth float32 = 288
	// NotesColumnWidth is the notes column's width until its divider moves.
	// A document's split starts here and the user's drag is remembered.
	NotesColumnWidth float32 = 260

	// The statistics calendar: one day's box in the overview's strip and on
	// the heatmap page, the room between two boxes, and the column the
	// weekday names get. The box is a square, so its side is both sizes.
	HeatCell      float32 = 12
	HeatCellLarge float32 = 20
	HeatGap       float32 = 3
	HeatWeekday   float32 = 34
	// FilterPaneWidth is the filter rail the card table keeps on its left.
	FilterPaneWidth float32 = 200
	// The card table's fixed columns: the checkbox that chooses a row, the two
	// counts read right-aligned, and the date and the name that are read as
	// words. Every other column shares what the row leaves.
	CheckColumnWidth float32 = 40
	CountColumnWidth float32 = 76
	DateColumnWidth  float32 = 112
	NameColumnWidth  float32 = 150
	// MeterWidth is a row's paragraph progress bar: wide enough to read as a
	// fraction, narrow enough to leave the title the room it needs (spec §4.5).
	MeterWidth float32 = 120
	// SourceLabelWidth is the column a card's context names its three paragraphs
	// in — before, this one, after — so their texts start on one line.
	SourceLabelWidth float32 = 76

	// The frame: a rail, a top bar, a sub-navigation and a status line around
	// the stage. RailWidth is the rail open, RailCollapsed the icon strip it
	// folds to below BreakpointNarrow.
	RailWidth     float32 = 240
	RailCollapsed float32 = 64
	// FrameHeight is the window's top bar, SubnavHeight the area's pages,
	// StatusbarHeight the line under the stage, BarHeight the format bar.
	FrameHeight     float32 = 60
	SubnavHeight    float32 = 44
	StatusbarHeight float32 = 30
	BarHeight       float32 = 44

	// The window's two breaks: at or below Narrow the rail folds, at or below
	// Small the paper tightens.
	BreakpointNarrow float32 = 980
	BreakpointSmall  float32 = 760

	// The three icon sizes: a button's, a control's, and a page's.
	IconSmall  float32 = 16
	IconMedium float32 = 20
	IconLarge  float32 = 24

	ControlHeight float32 = 32
	// Hit is the smallest thing a pointer may be asked for, taller than a
	// control so that an icon-only button still has room around it.
	Hit      float32 = ControlHeight + 2
	Hairline float32 = 1

	// Line heights, by role: a display line, a heading's, the chrome's, and the
	// paper's.
	LineDisplay    float32 = 1.34
	LineTight      float32 = 1.3
	LineHeading    float32 = 1.45
	LineHeight     float32 = 1.45
	TextLineHeight float32 = 1.85
)
