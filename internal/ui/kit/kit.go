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

// StyleCard dresses e as a card on the page: the card colour and the card
// radius inside a 1px hairline. Only what floats over the page (StylePanel) is
// lifted, so a page of cards still reads as paper and pencil rather than as a
// stack of panels.
func StyleCard(c *ui.Context, e ui.Element) ui.Element {
	t := c.Theme()
	p := design.PaletteOf(t.Dark)
	return e.Background(p.Card).Radius(design.RadiusCard).Border(design.Hairline, t.Border)
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

// Empty is a place with nothing in it yet: the icon that names what is missing
// in a sunken tile, one sentence, and the single action that fills it. A nil
// action leaves the sentence alone, for a page the user cannot fill from here.
func Empty(c *ui.Context, icon, text string, action func()) ui.Element {
	t := c.Theme()
	p := design.PaletteOf(t.Dark)
	return ui.Column(c).Grow(1).Center().Gap(design.SpaceM).
		Padding(design.SpaceXXL).Children(func() {
		ui.Box(c).Size(design.SpaceXXL+design.SpaceS, design.SpaceXXL+design.SpaceS).
			Radius(design.SpaceM).Background(p.Sunken).Center().Children(func() {
			ui.Icon(c, design.Icon(icon)).
				Width(design.IconMedium).Height(design.IconMedium).
				TextColor(t.TextMuted)
		})
		ui.Text(c, text).FontSize(design.TypeUI).FontWeight(600)
		if action != nil {
			ui.Column(c).AlignItems(ui.Center).Gap(design.SpaceM).
				MaxWidth(design.MessageWidth).Children(action)
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
	return ui.Text(c, key).FontSize(design.TypeUI).TextColor(t.TextMuted).
		Background(p.Sunken).Radius(design.RadiusControl/2).
		Padding(0, design.SpaceXS).MinWidth(design.SpaceL + design.SpaceXS)
}

// Stat is one number, and the name of what it counts.
//
// Composed rather than taken from a widget: ui.Meter reports one value inside
// a range instead of naming a count, and ui.Badge carries a count but not the
// label a tile needs. Checked against MyGo v0.8.1.
func Stat(c *ui.Context, value, label string) ui.Element {
	return StatDetail(c, value, label, "")
}

// StatDetail is Stat with the line a tile needs to qualify its number — what a
// percentage is aiming at, or how many notes the cards came from. An empty
// detail draws nothing.
func StatDetail(c *ui.Context, value, label, detail string) ui.Element {
	t := c.Theme()
	p := design.PaletteOf(t.Dark)
	return ui.Column(c).Gap(design.SpaceXS).Children(func() {
		ui.Text(c, value).FontSize(design.TypeH2).Bold()
		ui.Text(c, label).FontSize(design.TypeUI).TextColor(t.TextMuted)
		if detail != "" {
			ui.Text(c, detail).FontSize(design.TypeLabel).TextColor(p.Faint)
		}
	})
}

// Notice is one line in a tone that says how to read it — what failed, what a
// page cannot do, or what a form just wrote — as an icon and the sentence on a
// wash of the tone, with an optional action beside it. The wash is faint enough
// that the sentence keeps its contrast on either appearance.
//
// Composed rather than taken from a widget: MyGo v0.8.1 has no callout, banner
// or notice (its catalogue lists none), and a Badge carries a count rather than
// a paragraph. The same icon-and-line is what the Today screen and the window's
// library error draw, so it is one helper here instead of a third copy.
func Notice(c *ui.Context, icon, text string, colour ui.Color, action func()) ui.Element {
	ink := colour.Mix(c.Theme().Text, 0.35)
	return ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(design.SpaceS).
		Padding(design.SpaceM, design.SpaceM).Radius(design.RadiusControl).
		Background(colour.Alpha(0.12)).Children(func() {
		ui.Icon(c, design.Icon(icon)).Width(design.IconSmall).Height(design.IconSmall).
			TextColor(ink)
		ui.Text(c, text).FontSize(design.TypeUI).TextColor(ink).Grow(1)
		if action != nil {
			action()
		}
	})
}

// PageHead is a page's name, the one line under it that says what the page
// holds, and the actions that belong to the page, on one row: H1 for the title,
// Label for the line, so the title keeps the hierarchy to itself.
//
// Composed rather than taken from a widget (checked against MyGo v0.8.1):
// ui.Toolbar is a window's bar with one tab stop and an overflow menu, and
// ui.Fieldset is a form's group without a title of its own.
func PageHead(c *ui.Context, tk design.Tokens, title, sub string, actions func()) ui.Element {
	return ui.Row(c).FillWidth().AlignItems(ui.End).Gap(tk.SpaceM).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, title).FontSize(tk.TypeH1).Bold()
			if sub != "" {
				ui.Text(c, sub).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			}
		})
		if actions != nil {
			actions()
		}
	})
}

// SectionHead is a section's name and the line beside it that says what it
// counts, with the section's own action pushed to the end: UI weight 600 for the
// name, Label for the hint, a step under the page's own head.
//
// Composed rather than taken from a widget (checked against MyGo v0.8.1):
// ui.Fieldset names a form group and carries no count or action, and ui.TabsBase
// is a choice of pages rather than a heading.
func SectionHead(c *ui.Context, tk design.Tokens, title, hint string, action func()) ui.Element {
	return ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, title).FontSize(tk.TypeUI).FontWeight(600)
		if hint != "" {
			ui.Text(c, hint).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).Grow(1)
		} else {
			ui.Spacer(c)
		}
		if action != nil {
			action()
		}
	})
}
