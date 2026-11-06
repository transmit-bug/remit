// Package kit is the few controls Remit's design language composes and the
// screens draw more than once: a card, a panel above it, an empty page, a keycap
// and a statistic. Each is the composition the screens would otherwise repeat;
// none of them re-implements behaviour a MyGo widget already has, and none
// brings an illustration or a drawn picture of its own.
package kit

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// StyleCard dresses e as a card: the card colour, the card radius and a small
// shadow, with no border — the step in tone from the page separates it.
func StyleCard(c *ui.Context, e ui.Element) ui.Element {
	p := design.PaletteOf(c.Theme().Dark)
	return e.Background(p.Card).Radius(design.RadiusCard).
		Shadow(0, 1, 3, 0, p.Shadow)
}

// Card is a face that sits on the page, holding what fn builds.
func Card(c *ui.Context, fn func()) ui.Element {
	return StyleCard(c, ui.Column(c).Children(fn))
}

// StylePanel dresses e as what floats over the page: the card colour under a
// larger shadow, at the panel radius.
func StylePanel(c *ui.Context, e ui.Element) ui.Element {
	p := design.PaletteOf(c.Theme().Dark)
	return e.Background(p.Card).Radius(design.RadiusPanel).
		Shadow(0, 8, 24, 0, p.ShadowPanel)
}

// Panel is a face that floats over the page — the command palette, a dialog —
// holding what fn builds.
func Panel(c *ui.Context, fn func()) ui.Element {
	return StylePanel(c, ui.Column(c).Children(fn))
}

// Empty is a place with nothing in it yet: the icon that names what is missing,
// one sentence, and the single action that fills it. A nil action leaves the
// sentence alone, for a page the user cannot fill from here.
func Empty(c *ui.Context, icon, text string, action func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Grow(1).Center().Gap(design.SpaceM).
		Padding(design.SpaceXXL).Children(func() {
		ui.Icon(c, design.Icon(icon)).
			Width(design.IconLarge).Height(design.IconLarge).
			TextColor(t.TextMuted)
		ui.Text(c, text).FontSize(design.TypeHeading).TextColor(t.TextMuted)
		if action != nil {
			action()
		}
	})
}

// StateBadge is a pill that reports a name and the way a card stands — lost,
// frozen, suspended — as its symbol and its word, tinted by the colour that
// state carries so that the word keeps its contrast.
func StateBadge(c *ui.Context, icon, text string, colour ui.Color) ui.Element {
	t := c.Theme()
	return ui.Badge(c, "").
		Background(colour.Alpha(0.12)).TextColor(colour.Mix(t.Text, 0.35)).
		Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(design.SpaceXS).Children(func() {
				ui.Icon(c, design.Icon(icon)).
					Width(design.IconSmall).Height(design.IconSmall)
				ui.Text(c, text).SingleLine()
			})
		})
}

// Keycap is a key's name on the face of a key, as the grade keys show 1 to 4:
// quiet and small, so the label beside it carries the meaning.
func Keycap(c *ui.Context, key string) ui.Element {
	t := c.Theme()
	p := design.PaletteOf(t.Dark)
	return ui.Text(c, key).FontSize(design.TypeSmall).TextColor(t.TextMuted).
		Background(p.Sunken).Radius(design.RadiusControl/2).
		Padding(0, design.SpaceXS).MinWidth(design.SpaceL + design.SpaceXS)
}

// Stat is one number, and the name of what it counts.
func Stat(c *ui.Context, value, label string) ui.Element {
	t := c.Theme()
	return ui.Column(c).Gap(design.SpaceXS).Children(func() {
		ui.Text(c, value).FontSize(design.TypeTitle).Bold()
		ui.Text(c, label).FontSize(design.TypeSmall).TextColor(t.TextMuted)
	})
}
