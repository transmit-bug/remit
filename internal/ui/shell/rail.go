package shell

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// rail draws the navigation: the brand, the six areas, the plugin pages under
// a hairline, and the control that folds it. Below BreakpointNarrow it is an
// icon strip, and each item names itself in its tooltip.
func rail(c *ui.Context, tk design.Tokens, st *State, d Deps) {
	msgs := locales.In(c.Locale())
	folded := collapsed(c, st)
	width := tk.RailWidth
	if folded {
		width = tk.RailCollapsed
	}
	ui.Column(c).Width(width).Shrink(0).AlignItems(ui.Stretch).
		Background(tk.Surface).BorderWidth(0, tk.Hairline, 0, 0).BorderColor(tk.Border).
		Children(func() {
			brand(c, tk, folded)
			// The items take every row they can: the sidebar grows, which is
			// what keeps the fold control at the rail's foot.
			railItems(c, tk, d, folded)
			if !narrow(c) {
				ui.Divider(c)
				foldButton(c, tk, msgs, st, st.Folded)
			}
		})
}

// brand is the quiet identity at the head of the rail: a mark whose colour is
// the product's accent, then the wordmark. It is not a button; the areas under
// it are where it points.
func brand(c *ui.Context, tk design.Tokens, folded bool) {
	ui.Row(c).Height(tk.FrameHeight).Padding(0, tk.SpaceL).Shrink(0).
		AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		ui.Box(c).Size(tk.SpaceXL+2, tk.SpaceXL+2).Radius(tk.RadiusControl).
			Background(tk.Accent).Center().Children(func() {
			tk.Icon(c, "notebook-pen", design.IconSmall).TextColor(tk.AccentText)
		})
		if !folded {
			ui.Text(c, "Remit").FontSize(tk.TypeUI).FontWeight(650).
				LetterSpacing(-0.15).SingleLine()
		}
	})
}

// railItems draws the areas and, under a hairline, the pages plugins
// contributed. ui.Sidebar carries the keyboard: it is one tab stop, and the
// arrows, Home, End and an item's first letter choose within it.
func railItems(c *ui.Context, tk design.Tokens, d Deps, folded bool) {
	msgs := locales.In(c.Locale())
	chosen := string(d.Current)
	sidebar := ui.Sidebar(c, &chosen, func() {
		for _, item := range d.Areas {
			railItem(c, tk, item, folded)
		}
		if len(d.Pages) > 0 {
			ui.Divider(c)
			ui.SidebarSection(c, msgs.PluginPages(), nil, func() {
				for _, page := range d.Pages {
					railItem(c, tk, page, folded)
				}
			})
		}
	}).Grow(1).Label(msgs.AreaNav())
	if sidebar.Changed() && chosen != string(d.Current) && d.OnSelect != nil {
		d.OnSelect(Area(chosen))
	}
}

// railItem is one area: its icon, its label, and its count when there is one.
// Folded, the label is the tooltip and the accessible name, and the badge keeps
// one digit so that the strip stays a strip.
func railItem(c *ui.Context, tk design.Tokens, item Item, folded bool) {
	label := item.Label
	if folded {
		label = ""
	}
	var icon *ui.SVG
	if item.Icon != "" {
		icon = design.Icon(item.Icon)
	}
	row := ui.SidebarItem(c, string(item.ID), icon, label).Label(item.Label)
	if folded {
		row.Tooltip(item.Label)
	}
	if item.Count > 0 {
		row.Children(func() {
			ui.Badge(c, badgeText(item.Count, folded))
		})
	}
}

// badgeText is a count as a badge reads it: the number, or one digit and a plus
// when the rail is folded.
func badgeText(n int, folded bool) string {
	if folded && n > 9 {
		return "9+"
	}
	return strconv.Itoa(n)
}

// foldButton is the rail's own control: it folds the rail to an icon strip and
// brings it back, and says which it will do.
func foldButton(c *ui.Context, tk design.Tokens, msgs locales.Messages, st *State, folded bool) {
	label, icon := msgs.RailFold(), "arrow-left"
	if folded {
		label, icon = msgs.RailUnfold(), "panel-left"
	}
	ui.Button(c, "").Label(label).Tooltip(label).
		Height(tk.Hit).Justify(ui.Start).Radius(0).
		Padding(0, tk.SpaceL).TextColor(tk.TextMuted).
		OnClick(func() { st.Folded = !st.Folded }).Children(func() {
		tk.Icon(c, icon, design.IconSmall)
		ui.Text(c, label).FontSize(tk.TypeUI).TextColor(tk.TextMuted).SingleLine()
	})
}
