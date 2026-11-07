package design

// The scale every view sizes itself by: one comfortable density, three radii,
// four type sizes plus monospace, and a 1.5 line height. Views name these
// through the window's tokens rather than spelling out a number.
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

	TypeTitle   float32 = 22
	TypeHeading float32 = 17
	TypeBody    float32 = 15
	TypeSmall   float32 = 13
	TypeMono    float32 = 14

	// The document's own scale, larger than the chrome's: a note is read at
	// arm's length, the controls around it at a glance.
	TypeDisplay float32 = 34
	TypeChrome  float32 = 13
	TypeLabel   float32 = 12
	TypeCaption float32 = 11

	// PaperWidth is the measure of a note and Gutter the room beside it: the
	// two together set how many characters a line holds.
	PaperWidth float32 = 760
	Gutter     float32 = 48
	// FrameHeight is the window's top bar, BarHeight the format bar.
	FrameHeight float32 = 64
	BarHeight   float32 = 44

	// The three icon sizes: a button's, a control's, and a page's.
	IconSmall  float32 = 16
	IconMedium float32 = 20
	IconLarge  float32 = 24

	ControlHeight float32 = 32
	Hairline      float32 = 1
	LineHeight    float32 = 1.5
	// TextLineHeight is the body's line height on the paper, roomier than
	// the chrome's.
	TextLineHeight float32 = 1.85
)
