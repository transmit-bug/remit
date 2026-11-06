// Package ui draws Remit's window. It owns presentation and input handling and
// keeps domain rules in internal/core.
package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/settings"
)

// Name and Version are the app's identity: the About page shows them, the
// window title and the menus use Name, and mygo.json carries Version for
// packaging. TestVersionMatchesConfig keeps the two in step.
const (
	Name    = "Remit"
	Version = "0.1.0"
)

// mode is which of Remit's faces the window shows: editing notes, reviewing
// the queue pulled from them, browsing every card, or the statistics page.
type mode uint8

const (
	modeNotes mode = iota
	modeReview
	modeBrowse
	modeStats
)

// App is the state the window shows. The view rebuilds the interface from it
// on every frame; handlers change it.
type App struct {
	lib     *store.Library // the open library, nil until open succeeds
	dir     string         // where the library lives, once resolved
	openErr error          // why the library did not open, or nil
	start   bool           // no library yet: the start view asks where to put one
	data    libraryData    // the library as the view reads it
	due     []core.ID      // today's queue, as the last reload derived it

	mode     mode
	review   reviewState
	editor   editorState
	browse   browseState
	stats    statsState
	palette  paletteState
	relink   relinkState
	aboutWin *mygo.Window // the open About window, or nil

	// settingsDir is where settings.json lives, empty when the app keeps no
	// settings; defaultLibrary is <Documents>/Remit, resolved once.
	settingsDir    string
	settings       settings.Settings
	defaultLibrary string
	pickLibrary    func() (string, error)

	// remaining is told the queue's length whenever it is rebuilt, so the
	// tray can show it without asking the window.
	remaining func(int)

	// opened is told about the library once it opens, so a service that
	// serves it (the agent API) can start without a second open.
	opened func(*store.Library)
}

// NewApp returns an app on the library in opts.LibraryDir, or on the one a
// previous run remembered in its settings, in review mode. With no library at
// either place it shows the start view — Create library or Open library… —
// rather than creating one the user did not ask for. A library that cannot be
// opened leaves the window showing that, with the one action that tries again;
// it never panics over a damaged library.
func NewApp(opts ...Options) *App {
	a := &App{
		browse:      browseState{selected: -1},
		editor:      editorState{addAfter: -1},
		relink:      relinkState{selected: -1},
		pickLibrary: pickLibraryDir,
	}
	if len(opts) > 0 {
		a.dir = opts[0].LibraryDir
		a.settingsDir = opts[0].DataDir
		if opts[0].ChooseLibrary != nil {
			a.pickLibrary = opts[0].ChooseLibrary
		}
	}
	if dir, err := DefaultLibraryDir(); err == nil {
		a.defaultLibrary = dir
	}
	a.loadSettings()
	a.open()
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
		switch {
		case a.openErr != nil:
			a.libraryErrorView(c, tk)
		case a.start:
			a.startView(c, tk)
		case a.mode == modeReview:
			a.reviewView(c, tk)
		case a.mode == modeBrowse:
			a.browseView(c, tk)
		case a.mode == modeStats:
			a.statsView(c, tk)
		default:
			a.notesView(c, tk)
		}
		a.paletteView(c, tk)
		a.relinkView(c, tk)
	})
}
