package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// View draws the window. It reads state and returns; anything that changes
// state belongs in a handler.
func (a *App) View(c *ui.Context) {
	tk := design.NewTokens(c.Theme())
	c.SetTheme(tk.Theme)

	// Window shortcuts belong to the window, not to the column they are built
	// beside, so that they work with no control focused.
	a.shortcuts(c)

	ui.Column(c).Fill().Background(tk.Background).LineHeight(design.LineHeight).Children(func() {
		// Review takes the window whole: no navigation, no list, just the card
		// and its status bar. Everywhere else the first frame arrives in two
		// beats — the toolbar, then the main area behind it.
		if !a.takingOver() {
			toolbar := a.toolbar(c, tk)
			entering := toolbar.Keyframes("enter", design.EnterFrames)
			toolbar.Opacity(entering).Translate(0, design.Rise*(1-entering))
		}

		// One screen replacing another fades in and rises, played by the count
		// the mode handler keeps.
		main := ui.Column(c).Grow(1)
		switchRise := main.Keyframes("mode", design.ModeFrames, ui.KeyframeOptions{Run: a.anim.mode})
		main.Opacity(switchRise).Translate(0, design.Rise*(1-switchRise)).Children(func() {
			switch {
			case a.openErr != nil:
				a.libraryErrorView(c, tk)
			case a.mode == modeSettings:
				a.settingsView(c, tk)
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
		})

		a.paletteView(c, tk)
		a.relinkView(c, tk)
		a.cardEditView(c, tk)
	})

	// A row chosen in the note's panel moves the caret after the pass: the
	// editor above keeps its own view state, and a jump made inside the build
	// is written back over by it in the next frame.
	if a.pendingJump >= 0 {
		a.jumpInDocument(c, a.pendingJump)
		a.pendingJump = -1
	}
}

// setMode switches the window's face and loads what the new one reads. What a
// mode derives from the store is loaded here, never in its view, and leaving
// the settings page this way is the same as any other switch.
func (a *App) setMode(m mode) {
	if m != a.mode {
		a.anim.mode++
	}
	a.mode = m
	a.loadCurrent()
}
