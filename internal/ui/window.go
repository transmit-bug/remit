package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/about"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/internal/ui/screens/review"
)

// toolbar is the window's one row of controls, the same in every mode so that
// switching never moves what the user is aiming at. Before a library is open
// there is nothing to switch between, so the row holds the app's name alone.
func (a *App) toolbar(c *ui.Context, tk design.Tokens) ui.Element {
	msgs := m(c)
	toolbar := ui.Toolbar(c, func() {
		ui.Text(c, Name).FontSize(tk.TypeHeading).Bold()
		if a.lib == nil {
			ui.Spacer(c)
			return
		}

		chosen := int(a.mode)
		if ui.Segmented(c, &chosen, msgs.ModeNotes(), msgs.ModeReview(), msgs.ModeBrowse(), msgs.ModeStats(), msgs.ModeSettings()).
			Label(msgs.ModeLabel()).Changed() {
			a.setMode(mode(chosen))
		}

		ui.Spacer(c)
		if a.mode == modeNotes {
			view := int(a.notes.View)
			if ui.Segmented(c, &view, msgs.ViewEdit(), msgs.ViewSplit(), msgs.ViewRead()).
				Label(msgs.ViewMode()).Changed() {
				a.notes.View = notes.ViewMode(view)
			}
			ui.Button(c, "").Label(msgs.PanelTitle()).Tooltip(msgs.PanelTitle()).
				Children(func() { tk.Icon(c, "notebook-pen", design.IconSmall) }).
				OnClick(func() { a.notes.TogglePanel(c) })
		}
		ui.Button(c, "").Label(msgs.More()).Tooltip(msgs.More()).
			Children(func() { tk.Icon(c, "ellipsis", design.IconSmall) }).
			Menu(func(menu *ui.Menu) {
				if menu.Item(msgs.ImportAnki()).Chosen() {
					a.importAnkiPackage(c)
				}
				if menu.Item(msgs.MakeCard()).Chosen() {
					a.makeCardAtCaret(c)
				}
				if menu.Item(msgs.OpenSettings()).Chosen() {
					a.openSettings()
				}
			})
	}).Label(msgs.Toolbar(Name)).
		Padding(tk.SpaceS, tk.SpaceL).
		Background(tk.Surface)
	return toolbar
}

// shortcuts declares the app's keys. They are window shortcuts, so they wait
// while the palette is open. The settings page is the app's, not the library's,
// so its key works before one is open; with no library the rest has no action
// yet.
func (a *App) shortcuts(c *ui.Context) {
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { a.openSettings() })
	if a.mode == modeSettings {
		c.OnShortcut(0, ui.KeyEscape, func() { a.closeSettings() })
	}
	if a.lib == nil {
		return
	}
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.openPalette(c) })
	c.OnShortcut(ui.Cmd, ui.Key1, func() { a.setMode(modeReview) })
	c.OnShortcut(ui.Cmd, ui.Key2, func() { a.setMode(modeNotes) })
	c.OnShortcut(ui.Cmd, ui.Key3, func() { a.setMode(modeBrowse) })
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		if a.mode == modeNotes && a.notes.Draft.ID != "" {
			// The notebook's find looks in the note that is open; the library's
			// search is the browse page's.
			a.notes.FindOpen = true
			return
		}
		a.focusSearch(c)
	})
	c.OnShortcut(ui.Cmd, ui.KeyN, func() { a.newNote(c) })
	c.OnShortcut(ui.Cmd, ui.KeyD, func() { a.openDailyNote(c) })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyO, func() { a.showPanel(notes.PanelOutline) })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyB, func() { a.showPanel(notes.PanelBacklinks) })

	switch a.mode {
	case modeReview:
		// A card that takes a typed answer owns the keyboard until it is
		// checked: Space is a space there, and the grades only come back once
		// the answer is on screen, so a digit typed into the input is not a grade.
		c.OnShortcut(0, ui.KeySpace, func() {
			if !a.typing() {
				a.review.Reveal()
			}
		})
		if a.review.Answered() {
			c.OnShortcut(0, ui.Key1, func() { a.gradeCurrent(c, review.Again) })
			c.OnShortcut(0, ui.Key2, func() { a.gradeCurrent(c, review.Hard) })
			c.OnShortcut(0, ui.Key3, func() { a.gradeCurrent(c, review.Good) })
			c.OnShortcut(0, ui.Key4, func() { a.gradeCurrent(c, review.Easy) })
		}
		c.OnShortcut(0, ui.KeyU, func() { a.undo(c) })
		c.OnShortcut(0, ui.KeyS, func() { a.suspendCurrent(c) })
		c.OnShortcut(0, ui.KeyE, a.editCurrent)
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	case modeBrowse, modeStats:
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	}
}

// ShowAbout opens the About window above parent, or brings it back to the
// front when it is already open. Opening a window is assembly, so it lives
// here and the page lives in its own package (ADR 0009).
func (a *App) ShowAbout(parent *mygo.Window) {
	if a.aboutWin != nil {
		a.aboutWin.Focus()
		return
	}
	state := about.State{Name: Name, Version: Version}
	a.aboutWin = mygo.NewWindow(mygo.WindowOptions{
		Title:         "About " + Name,
		Parent:        parent,
		Width:         about.Width,
		Height:        about.Height,
		DisableResize: true,
		Content:       ui.View(func(c *ui.Context) { about.View(c, state, about.Deps{}) }),
	})
	a.aboutWin.OnClosed(func() { a.aboutWin = nil })
}

// focusSearch shows browse and puts the caret in its search field, from any
// mode: the one way to search the library by text.
func (a *App) focusSearch(c *ui.Context) {
	a.setMode(modeBrowse)
	a.browse.Focus(c)
}
