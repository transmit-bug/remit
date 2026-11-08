package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
	screenstats "github.com/transmit-bug/remit/internal/ui/screens/stats"
)

// View draws the window. It reads state and returns; anything that changes
// state belongs in a handler.
func (a *App) View(c *ui.Context) {
	// What a plugin's callback answered since the last frame is applied first,
	// so the frame that draws it draws the result. The work itself already ran
	// off this pass: no frame ever waits on a plugin.
	a.applyBackground(c)
	// The library's writes are the contract's event: drop what each page was
	// loaded against, so the page on screen asks again off this frame.
	if a.stale.Swap(false) {
		a.invalidatePages()
	}

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
				screensettings.View(c, &a.prefs, a.settingsDeps())
			case a.start:
				a.startView(c)
			case a.mode == modeReview, a.mode == modeReading:
				a.queueView(c, tk)
			case a.mode == modeBrowse:
				a.browseView(c)
			case a.mode == modeStats:
				screenstats.View(c, a.stats, screenstats.Deps{})
			case a.mode == modePlugin:
				a.pluginPageView(c)
			default:
				a.notesView(c)
			}
		})

		a.paletteView(c, tk)
		a.relinkView(c)
		a.cardEditView(c, tk)
	})
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

// queueView draws the two sides of the dual mode: the review queue and the
// reading queue under one switch, with how many notes wait to be read in sight
// from both, so reading is not drowned out by reviewing.
func (a *App) queueView(c *ui.Context, tk design.Tokens) {
	msgs := m(c)
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Justify(ui.Center).Padding(tk.SpaceXS, tk.SpaceL).
			Background(tk.Surface).BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border).
			Children(func() {
				ui.Row(c).Grow(1).MaxWidth(design.ReviewWidth).AlignItems(ui.Center).
					Gap(tk.SpaceS).Children(func() {
					chosen := 0
					if a.mode == modeReading {
						chosen = 1
					}
					if segmentedChoice(c, tk, &chosen, []string{msgs.ModeReview(), msgs.ModeReading()}, msgs.ModeLabel()) {
						if chosen == 1 {
							a.setMode(modeReading)
						} else {
							a.setMode(modeReview)
						}
					}
					if a.mode == modeReview && a.lib != nil {
						a.reviewScopeMenu(c, msgs)
					}
					ui.Spacer(c)
					ui.Text(c, msgs.ReadingWaiting(a.readingCount)).
						FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
				})
			})
		ui.Column(c).Grow(1).Children(func() {
			if a.mode == modeReading {
				a.readingView(c)
				return
			}
			a.reviewView(c, tk)
		})
	})
}
