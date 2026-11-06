package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/locales"
)

const (
	// sidebarWidth is the notes column, a fixed 260 DIP column rather than a
	// pane the user resizes: review mode hides it instead of sharing space.
	sidebarWidth float32 = 260
	// paletteWidth is the command palette's panel.
	paletteWidth float32 = 480
)

// toolbar is the window's one row of controls, the same in every mode so that
// switching never moves what the user is aiming at.
func (a *App) toolbar(c *ui.Context, tk tokens) {
	ui.Toolbar(c, func() {
		ui.Text(c, Name).FontSize(tk.typeHeading).Bold()

		chosen := int(a.mode)
		if ui.Segmented(c, &chosen, m(c).ModeNotes(), m(c).ModeReview(), m(c).ModeBrowse(), m(c).ModeStats()).
			Label(m(c).ModeLabel()).Changed() {
			a.setMode(mode(chosen))
		}

		ui.Spacer(c)
		ui.Button(c, m(c).NewNote()).OnClick(func() { a.newNote(c) })
	}).Label(m(c).Toolbar(Name)).
		Padding(tk.spaceS, tk.spaceL).
		Background(tk.surface).
		BorderWidth(0, 0, tk.hairline, 0).
		BorderColor(tk.border)
}

// notesView is editing mode: the notes list beside the selected note.
func (a *App) notesView(c *ui.Context, tk tokens) {
	ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
		a.notesList(c, tk)
		ui.Divider(c)
		a.editorView(c, tk)
	})
}

// notesList is the source list of notes, with how many cards each one feeds.
func (a *App) notesList(c *ui.Context, tk tokens) {
	ui.Sidebar(c, &a.editor.selected, func() {
		ui.SidebarSection(c, m(c).Notebook(), nil, func() {
			for _, n := range a.data.notes {
				item := ui.SidebarItem(c, string(n.ID), nil, n.Title)
				if cards := a.data.cardsInNote(n); cards > 0 {
					item.Children(func() { ui.Badge(c, fmt.Sprint(cards)) })
				}
			}
		})
	}).Width(sidebarWidth)
}

// shortcuts declares the app's keys. They are window shortcuts, so they wait
// while the palette is open. With no library open nothing has an action yet.
func (a *App) shortcuts(c *ui.Context) {
	if a.lib == nil {
		return
	}
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.openPalette(c) })
	c.OnShortcut(ui.Cmd, ui.Key1, func() { a.setMode(modeReview) })
	c.OnShortcut(ui.Cmd, ui.Key2, func() { a.setMode(modeNotes) })
	c.OnShortcut(ui.Cmd, ui.Key3, func() { a.setMode(modeBrowse) })
	c.OnShortcut(ui.Cmd, ui.KeyF, func() { a.focusSearch(c) })
	c.OnShortcut(ui.Cmd, ui.KeyN, func() { a.newNote(c) })

	switch a.mode {
	case modeReview:
		c.OnShortcut(0, ui.KeySpace, func() { a.review.reveal() })
		c.OnShortcut(0, ui.Key1, func() { a.grade(c, ratingAgain) })
		c.OnShortcut(0, ui.Key2, func() { a.grade(c, ratingHard) })
		c.OnShortcut(0, ui.Key3, func() { a.grade(c, ratingGood) })
		c.OnShortcut(0, ui.Key4, func() { a.grade(c, ratingEasy) })
		c.OnShortcut(0, ui.KeyU, func() { a.undo(c) })
		c.OnShortcut(0, ui.KeyS, func() { a.suspend(c) })
		c.OnShortcut(0, ui.KeyE, a.editCurrent)
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	case modeBrowse, modeStats:
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	}
}

// setMode switches the window's face and loads what the new one reads. What a
// mode derives from the store is loaded here, never in its view.
func (a *App) setMode(m mode) {
	a.mode = m
	a.loadCurrent()
}

// focusSearch shows browse and puts the caret in its search field, from any
// mode: the one way to search the library by text.
func (a *App) focusSearch(c *ui.Context) {
	a.setMode(modeBrowse)
	a.browse.field.Focus(c)
}

// newNote adds an empty note, selects it and puts the caret in its first
// block. The note exists on disk before it is shown, so what the user types
// next has somewhere to go.
func (a *App) newNote(c *ui.Context) {
	if a.lib == nil {
		return
	}
	note, err := a.lib.SaveNote(core.Note{
		ID:     core.NewID("note"),
		Title:  locales.UntitledNote(),
		Blocks: []core.Block{{ID: core.NewID("block")}},
	})
	if err != nil {
		a.report(c, "new note", err)
		return
	}
	a.reload()
	a.editor.selected = string(note.ID)
	a.setMode(modeNotes)
	a.editor.focusFirst = true
}

// editNote opens the note a card's block belongs to. A lost card has no note
// to open, and leaves the window where it is.
func (a *App) editNote(card core.Card) {
	note, ok := a.data.noteOfBlock(card.Ref.Block)
	if !ok {
		return
	}
	a.editor.selected = string(note)
	a.setMode(modeNotes)
}

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
}

// openPalette opens the palette on a fresh query and asks for the focus before
// the dialog is built: AutoFocus would take it before the dialog recorded what
// had it, and Escape would then have nowhere to give it back.
func (a *App) openPalette(c *ui.Context) {
	a.palette.open = true
	a.palette.query = ""
	a.palette.sel = 0
	a.palette.field.Focus(c)
}

// paletteActions lists what the app can do from anywhere.
func (a *App) paletteActions(c *ui.Context) []paletteAction {
	return []paletteAction{
		{label: m(c).ActionReviewCards(), run: func() { a.setMode(modeReview) }},
		{label: m(c).ActionEditNotes(), run: func() { a.setMode(modeNotes) }},
		{label: m(c).BrowseTitle(), run: func() { a.setMode(modeBrowse) }},
		{label: m(c).StatsTitle(), run: func() { a.setMode(modeStats) }},
		{label: m(c).NewNote(), run: func() { a.newNote(c) }},
	}
}

// paletteView draws the palette over the window. DialogBase keeps the keyboard
// in it, Escape closes it and gives the focus back where it was.
func (a *App) paletteView(c *ui.Context, tk tokens) {
	if !a.palette.open {
		return
	}
	items := filterActions(a.paletteActions(c), a.palette.query)
	sel := clamp(a.palette.sel, 0, len(items)-1)

	ui.DialogBase(c, &a.palette.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.scrim)
		panel.Width(paletteWidth).Padding(tk.spaceL).Gap(tk.spaceM).
			Background(tk.surfaceRaised).Border(tk.hairline, tk.borderStrong).
			Radius(tk.radiusPanel).Label(m(c).CommandPalette())

		panel.Children(func() {
			field := ui.TextInput(c, &a.palette.query).
				Placeholder(m(c).SearchActions()).Bind(&a.palette.field).
				Label(m(c).SearchActions()).MinHeight(tk.control)
			if field.Changed() {
				a.palette.sel = 0
			}
			if field.Submitted() {
				a.runPalette(items, sel)
			}
			c.OnShortcut(0, ui.KeyDown, func() { a.movePalette(len(items), 1) })
			c.OnShortcut(0, ui.KeyUp, func() { a.movePalette(len(items), -1) })

			if len(items) == 0 {
				ui.Text(c, m(c).NoActions()).FontSize(tk.typeSmall).TextColor(tk.textMuted)
			}
			for i, it := range items {
				row := ui.ButtonBase(c).Key(it.label).Gap(tk.spaceS).Justify(ui.Start).
					MinHeight(tk.control).Padding(tk.spaceXS, tk.spaceS).
					Radius(tk.radiusControl).Label(it.label)
				if i == sel {
					row.Background(tk.surfaceSunken)
				}
				row.OnClick(func() { a.runPaletteAction(it) })
				row.Children(func() { ui.Text(c, it.label).FontSize(tk.typeBody) })
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
