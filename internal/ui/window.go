package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/review"
)

// toolbar is the window's one row of controls, the same in every mode so that
// switching never moves what the user is aiming at. Before a library is open
// there is nothing to switch between, so the row holds the app's name alone.
func (a *App) toolbar(c *ui.Context, tk tokens) ui.Element {
	msgs := m(c)
	toolbar := ui.Toolbar(c, func() {
		ui.Text(c, Name).FontSize(tk.typeHeading).Bold()
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
			view := int(a.view)
			if ui.Segmented(c, &view, msgs.ViewEdit(), msgs.ViewSplit(), msgs.ViewRead()).
				Label(msgs.ViewMode()).Changed() {
				a.view = viewMode(view)
			}
			ui.Button(c, "").Label(msgs.PanelTitle()).Tooltip(msgs.PanelTitle()).
				Children(func() { tk.icon(c, "notebook-pen", iconSmall) }).
				OnClick(func() { a.togglePanel(c) })
		}
		ui.Button(c, "").Label(msgs.More()).Tooltip(msgs.More()).
			Children(func() { tk.icon(c, "ellipsis", iconSmall) }).
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
		Padding(tk.spaceS, tk.spaceL).
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
		if a.mode == modeNotes && a.editor.draft.ID != "" {
			// The notebook's find looks in the note that is open; the library's
			// search is the browse page's.
			a.editor.findOpen = true
			return
		}
		a.focusSearch(c)
	})
	c.OnShortcut(ui.Cmd, ui.KeyN, func() { a.newNote(c) })
	c.OnShortcut(ui.Cmd, ui.KeyD, func() { a.openDailyNote(c) })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyO, func() { a.showPanel(panelOutline) })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyB, func() { a.showPanel(panelBacklinks) })

	switch a.mode {
	case modeReview:
		c.OnShortcut(0, ui.KeySpace, func() { a.review.Reveal() })
		c.OnShortcut(0, ui.Key1, func() { a.gradeCurrent(c, review.Again) })
		c.OnShortcut(0, ui.Key2, func() { a.gradeCurrent(c, review.Hard) })
		c.OnShortcut(0, ui.Key3, func() { a.gradeCurrent(c, review.Good) })
		c.OnShortcut(0, ui.Key4, func() { a.gradeCurrent(c, review.Easy) })
		c.OnShortcut(0, ui.KeyU, func() { a.undo(c) })
		c.OnShortcut(0, ui.KeyS, func() { a.suspendCurrent(c) })
		c.OnShortcut(0, ui.KeyE, a.editCurrent)
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	case modeBrowse, modeStats:
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	}
}

// focusSearch shows browse and puts the caret in its search field, from any
// mode: the one way to search the library by text.
func (a *App) focusSearch(c *ui.Context) {
	a.setMode(modeBrowse)
	a.browse.field.Focus(c)
}
