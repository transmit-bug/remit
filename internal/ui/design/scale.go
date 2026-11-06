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

	ControlHeight float32 = 32
	Hairline      float32 = 1
	LineHeight    float32 = 1.5
)
