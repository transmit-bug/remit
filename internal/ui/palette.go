package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// paletteWidth is the command palette's panel.
const paletteWidth float32 = 480

// paletteAction is one command the palette lists.
type paletteAction struct {
	label string
	run   func()
}

// paletteState is the command palette: open or not, what has been typed, and
// which of the matching actions the keys are on.
type paletteState struct {
	open  bool
	query string
	sel   int
	field ui.Handle
	list  ui.ListState
}

// openPalette opens the palette on a fresh query and asks for the focus before
// the dialog is built: AutoFocus would take it before the dialog recorded what
// had it, and Escape would then have nowhere to give it back.
func (a *App) openPalette(c *ui.Context) {
	a.anim.palette++
	a.palette.open = true
	a.palette.query = ""
	a.palette.sel = 0
	a.palette.field.Focus(c)
}

// paletteActions lists what the app can do from anywhere, with the block
// actions of the note open in the editor. They are listed whenever the library
// holds a note; each runs against the note the editor has loaded, loading it
// first when the user has not been there yet.
func (a *App) paletteActions(c *ui.Context) []paletteAction {
	actions := []paletteAction{
		{label: m(c).ActionReviewCards(), run: func() { a.setMode(modeReview) }},
		{label: m(c).ActionEditNotes(), run: func() { a.setMode(modeNotes) }},
		{label: m(c).BrowseTitle(), run: func() { a.setMode(modeBrowse) }},
		{label: m(c).StatsTitle(), run: func() { a.setMode(modeStats) }},
		{label: m(c).NewNote(), run: func() { a.newNote(c) }},
		{label: m(c).OpenSettings(), run: func() { a.openSettings() }},
	}
	if a.lib == nil {
		return actions
	}
	actions = append(actions,
		paletteAction{label: m(c).ImportAnki(), run: func() { a.importAnkiPackage(c) }},
		// A note's shape is typed, not commanded: paragraphs are separated by
		// blank lines. Making a card from the paragraph at the caret stays a
		// command until the typing gesture lands (#94).
		paletteAction{label: m(c).MakeCard(), run: func() { a.makeCardAtCaret(c) }},
	)
	return actions
}

// paletteView draws the palette over the window. DialogBase keeps the keyboard
// in it, Escape closes it and gives the focus back where it was.
func (a *App) paletteView(c *ui.Context, tk design.Tokens) {
	if !a.palette.open {
		return
	}
	items := filterActions(a.paletteActions(c), a.palette.query)
	sel := clamp(a.palette.sel, 0, len(items)-1)

	ui.DialogBase(c, &a.palette.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(paletteWidth).Padding(tk.SpaceL).Gap(tk.SpaceM))
		panel.Label(m(c).CommandPalette())
		// The palette arrives a little small over a scrim that fades in.
		arriving := panel.Keyframes("arrive", design.PanelFrames, ui.KeyframeOptions{Run: a.anim.palette})
		scale := design.PanelScale + (1-design.PanelScale)*arriving
		panel.Opacity(arriving).Scale(scale, scale)
		backdrop.Opacity(arriving)

		panel.Children(func() {
			field := ui.TextInput(c, &a.palette.query).
				Placeholder(m(c).SearchActions()).Bind(&a.palette.field).
				Label(m(c).SearchActions()).MinHeight(tk.Control)
			if field.Changed() {
				a.palette.sel = 0
			}
			if field.Submitted() {
				a.runPalette(items, sel)
			}
			c.OnShortcut(0, ui.KeyDown, func() { a.movePalette(len(items), 1) })
			c.OnShortcut(0, ui.KeyUp, func() { a.movePalette(len(items), -1) })

			if len(items) == 0 {
				ui.Text(c, m(c).NoActions()).FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
				return
			}
			// The rows are a list's: the choice, the keys and what assistive
			// technology reads come with it, and the filter is the query above.
			a.palette.sel = clamp(a.palette.sel, 0, len(items)-1)
			a.palette.list.Selected = &a.palette.sel
			a.palette.list.Label = func(row int) string { return items[row].label }
			rows := ui.List(c, &a.palette.list, len(items), func(i int) {
				ui.Text(c, items[i].label).FontSize(tk.TypeBody).Padding(tk.SpaceXS, tk.SpaceS)
			}).Grow(1).MaxHeight(tk.Control * 8).Label(m(c).CommandPalette())
			if rows.Changed() || rows.Submitted() {
				a.runPalette(items, a.palette.sel)
			}
		})
	})
}

// runPalette runs the action at sel and closes the palette.
func (a *App) runPalette(items []paletteAction, sel int) {
	if sel < 0 || sel >= len(items) {
		return
	}
	a.runPaletteAction(items[sel])
}

// runPaletteAction closes the palette, then runs the action.
func (a *App) runPaletteAction(act paletteAction) {
	a.palette.open, a.palette.query, a.palette.sel = false, "", 0
	act.run()
}

// movePalette moves the palette's selection within n actions.
func (a *App) movePalette(n, delta int) {
	if n == 0 {
		return
	}
	a.palette.sel = clamp(a.palette.sel+delta, 0, n-1)
}

// filterActions keeps the actions whose label holds query, ignoring case.
func filterActions(actions []paletteAction, query string) []paletteAction {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return actions
	}
	var out []paletteAction
	for _, a := range actions {
		if strings.Contains(strings.ToLower(a.label), q) {
			out = append(out, a)
		}
	}
	return out
}

// clamp keeps v between lo and hi; an empty range gives lo.
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return max(lo, min(v, hi))
}
