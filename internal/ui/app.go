// Package ui draws Remit's window. It owns presentation and input handling and
// keeps domain rules in internal/core.
package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Name and Version are the app's identity: the About page shows them, the
// window title and the menus use Name, and mygo.json carries Version for
// packaging. TestVersionMatchesConfig keeps the two in step.
const (
	Name    = "Remit"
	Version = "0.1.0"
)

// mode is which of Remit's two faces the window shows: editing notes, or
// reviewing the queue pulled from them.
type mode uint8

const (
	modeNotes mode = iota
	modeReview
)

// App is the state the window shows. The view rebuilds the interface from it
// on every frame; handlers change it.
type App struct {
	library  *demoLibrary
	mode     mode
	review   reviewState
	editor   editorState
	palette  paletteState
	aboutWin *mygo.Window // the open About window, or nil
}

// NewApp returns an app on an in-memory demo library, in notes mode.
func NewApp() *App {
	a := &App{library: newDemoLibrary()}
	if len(a.library.notes) > 0 {
		a.editor.selected = string(a.library.notes[0].ID)
	}
	return a
}

// View draws the window. It reads state and returns; anything that changes
// state belongs in a handler.
func (a *App) View(c *ui.Context) {
	t := appTheme(c.Theme())
	c.SetTheme(t)
	tk := newTokens(t)

	// Window shortcuts belong to the window, not to the column they are built
	// beside, so that they work with no control focused.
	a.shortcuts(c)

	ui.Column(c).Fill().Background(tk.surface).LineHeight(lineHeight).Children(func() {
		a.toolbar(c, tk)
		switch a.mode {
		case modeReview:
			a.reviewView(c, tk)
		default:
			a.notesView(c, tk)
		}
		a.paletteView(c, tk)
	})
}
