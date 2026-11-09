package shell

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// topbar is the window's first row: where the user is, and what they can do
// here. It holds only the current context — the rail carries the navigation —
// and ui.Toolbar keeps its controls to one tab stop, with the arrows between
// them and the ones that do not fit in its overflow menu.
func topbar(c *ui.Context, tk design.Tokens, st *State, d Deps) {
	msgs := locales.In(c.Locale())
	bar := ui.Toolbar(c, func() {
		if !narrow(c) {
			railToggle(c, tk, msgs, st, st.Folded)
		}
		if len(d.Crumbs) > 0 {
			trail(c, tk, msgs, d)
		}
		ui.Spacer(c)
		if d.OnPalette != nil {
			iconButton(c, tk, msgs.SearchActions(), "search", d.OnPalette)
		}
		if d.Actions != nil {
			d.Actions(c, tk)
		}
		if d.Overflow != nil {
			overflow(c, tk, msgs, d)
		}
	}).Label(msgs.Toolbar(d.Title)).
		Height(tk.FrameHeight).Padding(0, tk.SpaceL).
		Background(tk.Surface).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border)
	bar.Shrink(0)
}

// railToggle folds the rail and brings it back. The rail's own control is at its
// foot, where the pointer leaves it; this one is where the eye starts.
func railToggle(c *ui.Context, tk design.Tokens, msgs locales.Messages, st *State, folded bool) {
	label := msgs.RailFold()
	if folded {
		label = msgs.RailUnfold()
	}
	iconButton(c, tk, label, "panel-right", func() { st.Folded = !st.Folded })
}

// trail is the path to where the user is: Remit / 忆 / 卡片. The last segment is
// the page itself and is not a link; an earlier one goes back to that area.
func trail(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps) {
	chosen := len(d.Crumbs) - 1
	crumbs := ui.Breadcrumbs(c, d.Crumbs, &chosen).Label(msgs.Path())
	if crumbs.Changed() && chosen >= 0 && chosen < len(d.Crumbs)-1 && d.OnCrumb != nil {
		d.OnCrumb(chosen)
	}
}

// overflow is the bar's ⋯: the actions that are not worth a button here, and
// whatever the page's plugins contributed to it.
func overflow(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps) {
	iconButton(c, tk, msgs.More(), "ellipsis", nil).Menu(d.Overflow)
}

// iconButton is the bar's icon-only action: a named hit area, a quiet face, and
// nothing but the icon until the pointer rests on it.
func iconButton(c *ui.Context, tk design.Tokens, label, icon string, click func()) ui.Element {
	button := ui.Button(c, "").Label(label).Tooltip(label).
		MinWidth(tk.Hit).MinHeight(tk.Hit).Justify(ui.Center).
		Radius(tk.RadiusControl).TextColor(tk.TextMuted)
	button.Children(func() { tk.Icon(c, icon, design.IconSmall) })
	if click != nil {
		button.OnClick(click)
	}
	return button
}
