package shell

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// subnav is the row of the area's pages: text tabs with the chosen one marked
// by a 2 DIP accent underline. ui.TabsBase is the whole of the keyboard — the
// row is one tab stop, the arrows and Home and End move between pages — and a
// page with none hides the row rather than leaving an empty band.
func subnav(c *ui.Context, tk design.Tokens, d Deps) {
	if len(d.Subnav) == 0 {
		return
	}
	msgs := locales.In(c.Locale())
	chosen := 0
	for i, page := range d.Subnav {
		if page.ID == d.CurrentSub {
			chosen = i
			break
		}
	}
	tabs := ui.TabsBase(c, &chosen, len(d.Subnav))
	list := tabs.List.Label(msgs.Subnav()).
		Height(tk.SubnavHeight-tk.Hairline).Shrink(0).
		AlignItems(ui.Stretch).Gap(tk.SpaceXS).Padding(0, tk.SpaceL).
		Background(tk.Surface).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border)
	list.Children(func() {
		for i, page := range d.Subnav {
			tab(c, tk, tabs.Tab(i), page, i == chosen)
		}
	})
	if list.Changed() && chosen >= 0 && chosen < len(d.Subnav) && d.OnSub != nil {
		d.OnSub(d.Subnav[chosen].ID)
	}
}

// tab is one page's word, its count when it has one, and the underline that
// says it is the page on screen. The underline grows and shrinks on
// design.ModeSwitch as the choice moves, so the mark follows the eye rather
// than blinking to the new page.
func tab(c *ui.Context, tk design.Tokens, tab ui.Element, page Sub, chosen bool) {
	tab = kit.HoverFace(c, tab.Padding(0, tk.SpaceM).Justify(ui.Center).Radius(0).Label(page.Label))
	target := float32(0)
	if chosen {
		target = 1
	}
	mark := tab.AnimateTo("underline", target, design.ModeSwitch)
	colour := tk.TextMuted
	if chosen {
		colour = tk.Text
	}
	if tab.Hovered() && !chosen {
		tab.Background(tk.SurfaceHover)
	}
	tab.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, page.Label).FontSize(tk.TypeUI).
				FontWeight(fontWeight(chosen)).TextColor(colour).SingleLine()
			if page.Count > 0 {
				ui.Badge(c, badgeText(page.Count, false))
			}
		})
	})
	tab.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if mark <= 0 {
			return
		}
		room := max(1, r.W-2*tk.SpaceM)
		p.Fill(ui.Rect{
			X: r.X + (r.W-room*mark)/2,
			Y: r.Y + r.H - 2,
			W: room * mark,
			H: 2,
		}, tk.Accent, 1)
	})
}

// fontWeight is the weight the chosen page's word takes, so the underline is not
// the only thing saying which page is up.
func fontWeight(chosen bool) int {
	if chosen {
		return 600
	}
	return 400
}
