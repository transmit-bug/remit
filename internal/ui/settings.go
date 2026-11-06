package ui

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/update"
)

// settingsWidth caps the settings page's measure, the same comfortable column
// the other full-window pages use.
const settingsWidth float32 = 560

// errCannotUpdate reports a build without update keys: MyGo's updater is
// disabled, so there is nothing to ask and nowhere to install from.
var errCannotUpdate = errors.New("this build cannot update itself")

// AgentStatus is what the agent API says about itself: where it listens, and
// the file that holds the bearer token. Both are the files cmd writes beside
// the token, so a client finds the API without a fixed port.
type AgentStatus struct {
	Address   string // host:port, empty while nothing is served
	TokenPath string // the file holding the token, empty when unknown
}

// AgentControls is what the settings page needs from whoever serves the agent
// API (cmd): start or stop the listener now, rotate the token, and say where
// it listens. It is an interface so that a test drives all three without
// binding a port. A nil AgentControls leaves the page saying so.
type AgentControls interface {
	// SetEnabled starts the listener, or stops it, and reports why it could
	// not. Starting one that already runs changes nothing.
	SetEnabled(on bool) error
	// RotateToken writes a fresh token and restarts the listener with it, so
	// the old token stops working at once.
	RotateToken() error
	// Status is where it listens and which file holds the token now.
	Status() AgentStatus
}

// prefsState is the settings page: the values it shows, copied from the
// settings when it opens and after every write, and what each action answered.
// Nothing typed here reaches the settings until an action is taken.
type prefsState struct {
	back mode // the face Esc goes back to

	notifications bool
	agentOn       bool
	updateCheck   bool
	hotkey        string

	hotkeyErr string // why the shortcut was not applied, "" when it was
	updateMsg string // the last update check's outcome
	agent     AgentStatus
}

// openSettings shows the settings page, remembering the face to come back to.
func (a *App) openSettings() {
	if a.mode != modeSettings {
		a.prefs.back = a.mode
	}
	a.mode = modeSettings
	a.syncPrefs()
}

// closeSettings returns to the face the page was opened from.
func (a *App) closeSettings() {
	back := a.prefs.back
	if back == modeSettings {
		back = modeNotes
	}
	a.setMode(back)
}

// syncPrefs copies the settings, and the agent API's own report, into what the
// page shows. Every write ends here, so a failed one leaves the page showing
// what the file holds rather than what the user clicked.
func (a *App) syncPrefs() {
	a.prefs.notifications = a.settings.Notifications
	a.prefs.agentOn = a.settings.AgentServer
	a.prefs.updateCheck = a.settings.UpdateCheck
	a.prefs.hotkey = a.settings.HotkeyAccelerator()
	a.prefs.agent = AgentStatus{}
	if a.agentAPI != nil {
		a.prefs.agent = a.agentAPI.Status()
	}
}

// settingsView is the settings page: the agent API, updates, the global
// shortcut and the daily reminder, each written as soon as it is changed.
func (a *App) settingsView(c *ui.Context, tk tokens) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.spaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(settingsWidth).Gap(tk.spaceL).Children(func() {
				ui.Text(c, m(c).SettingsTitle()).FontSize(tk.typeTitle).Bold()
				ui.Form(c, func() {
					a.agentFieldset(c, tk)
					a.updateFieldset(c, tk)
					a.hotkeyFieldset(c, tk)
					a.reminderFieldset(c, tk)
				})
			})
		})
	})
}

// agentFieldset is the agent API's switch, and what the listener says about
// itself: the address it chose, the token file, and the way to withdraw a
// token that leaked.
func (a *App) agentFieldset(c *ui.Context, tk tokens) {
	toggled := false
	ui.Fieldset(c, m(c).SettingsAgent(), func() {
		ui.Field(c, m(c).SettingsAgentServe(), func() {
			ui.Column(c).Gap(tk.spaceS).Children(func() {
				toggled = ui.Switch(c, &a.prefs.agentOn).Changed()
				ui.Column(c).Gap(tk.spaceXS).Children(func() {
					if a.prefs.agent.Address == "" {
						ui.Text(c, m(c).SettingsAgentStopped()).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
					} else {
						ui.Text(c, m(c).SettingsAgentAddress(a.prefs.agent.Address)).
							FontSize(tk.typeSmall).TextColor(tk.TextMuted)
					}
					if a.prefs.agent.TokenPath != "" {
						ui.Text(c, m(c).SettingsTokenPath(a.prefs.agent.TokenPath)).
							FontSize(tk.typeSmall).TextColor(tk.textFaint).SingleLine()
					}
				})
				ui.Button(c, m(c).RotateToken()).MinHeight(tk.control).AlignSelf(ui.Start).
					OnClick(func() { a.rotateToken(c) })
			})
		}).Description(m(c).SettingsAgentBody())
	})
	if toggled {
		a.setAgent(c, a.prefs.agentOn)
	}
}

// updateFieldset is the update check's switch and the one action that asks
// now, with what the last answer was.
func (a *App) updateFieldset(c *ui.Context, tk tokens) {
	toggled := false
	ui.Fieldset(c, m(c).SettingsUpdates(), func() {
		ui.Field(c, m(c).SettingsCheckAtLaunch(), func() {
			ui.Column(c).Gap(tk.spaceS).Children(func() {
				toggled = ui.Switch(c, &a.prefs.updateCheck).Changed()
				ui.Row(c).Gap(tk.spaceM).AlignItems(ui.Center).Children(func() {
					ui.Button(c, m(c).CheckNow()).MinHeight(tk.control).
						OnClick(func() { a.checkNow(c) })
					if a.prefs.updateMsg != "" {
						ui.Text(c, a.prefs.updateMsg).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
					}
				})
			})
		}).Description(m(c).SettingsUpdatesBody())
	})
	if toggled {
		a.writePrefs(c, func(s *settings.Settings) { s.UpdateCheck = a.prefs.updateCheck })
	}
}

// hotkeyFieldset is the global shortcut: the accelerator as typed, the one
// action that binds it, and why the last attempt did not.
func (a *App) hotkeyFieldset(c *ui.Context, tk tokens) {
	ui.Fieldset(c, m(c).SettingsShortcut(), func() {
		ui.Field(c, m(c).SettingsShortcutLabel(), func() {
			ui.Row(c).Gap(tk.spaceS).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &a.prefs.hotkey).
					Placeholder(settings.DefaultHotkey).MinHeight(tk.control).Grow(1).
					Label(m(c).SettingsShortcutLabel())
				ui.Button(c, m(c).ApplyShortcut()).MinHeight(tk.control).
					OnClick(func() { a.applyHotkey(c) })
			})
		}).Description(m(c).SettingsShortcutBody()).Error(a.hotkeyError(c))
	})
}

// reminderFieldset is the daily reminder's switch. It is the same setting the
// tray's pause item flips.
func (a *App) reminderFieldset(c *ui.Context, tk tokens) {
	toggled := false
	ui.Fieldset(c, m(c).SettingsReminders(), func() {
		ui.Field(c, m(c).SettingsReminderLabel(), func() {
			toggled = ui.Switch(c, &a.prefs.notifications).Changed()
		}).Description(m(c).SettingsReminderBody())
	})
	if toggled {
		a.writePrefs(c, func(s *settings.Settings) { s.Notifications = a.prefs.notifications })
	}
}

// hotkeyError is the shortcut field's inline error, empty while the last
// attempt worked.
func (a *App) hotkeyError(c *ui.Context) string {
	if a.prefs.hotkeyErr == "" {
		return ""
	}
	return m(c).ShortcutUnavailable(a.prefs.hotkeyErr)
}

// NoteHotkeyFailure records why the accelerator could not be registered, so
// that the next visit to the settings page says it. A startup that cannot bind
// the shortcut must never be a silent one.
func (a *App) NoteHotkeyFailure(err error) {
	if err == nil {
		a.prefs.hotkeyErr = ""
		return
	}
	a.prefs.hotkeyErr = err.Error()
}

// applyHotkey binds the accelerator the field holds. A combination that is
// invalid or already taken leaves the old binding in place: the seam registers
// before it unregisters, and nothing is written unless that worked.
func (a *App) applyHotkey(c *ui.Context) {
	acc := strings.TrimSpace(a.prefs.hotkey)
	if acc == "" {
		acc = settings.DefaultHotkey
	}
	if a.bindHotkey == nil {
		a.prefs.hotkeyErr = m(c).SettingsUnavailable()
		return
	}
	if err := a.bindHotkey(acc); err != nil {
		a.prefs.hotkeyErr = err.Error()
		slog.Warn("remit: global shortcut not applied", "accelerator", acc, "err", err)
		return
	}
	a.prefs.hotkeyErr = ""
	if err := a.UpdateSettings(func(s *settings.Settings) { s.Hotkey = acc }); err != nil {
		a.report(c, "save settings", err)
		return
	}
	c.Toast(m(c).ShortcutApplied(acc))
}

// checkNow asks whether a newer version exists and says what came back. It
// asks only when the user turned the check on, and reports a build that cannot
// update as such rather than as being up to date.
func (a *App) checkNow(c *ui.Context) {
	a.prefs.updateMsg = ""
	if !a.settings.UpdateCheck {
		a.prefs.updateMsg = m(c).UpdateOff()
		return
	}
	if a.checkUpdate == nil {
		a.prefs.updateMsg = m(c).SettingsUnavailable()
		return
	}
	result, err := a.checkUpdate(context.Background(), a.settings)
	switch {
	case errors.Is(err, errCannotUpdate):
		a.prefs.updateMsg = m(c).UpdateCannot()
	case err != nil:
		a.prefs.updateMsg = m(c).UpdateFailed(err.Error())
		slog.Warn("remit: update check", "err", err)
	case result.Version == "":
		a.prefs.updateMsg = m(c).UpdateUpToDate()
	default:
		a.prefs.updateMsg = m(c).UpdateAvailable(result.Version)
		slog.Info("remit: newer version available", "version", result.Version, "running", Version)
	}
}

// rotateToken withdraws the agent API's token and reports the one now active,
// which is a new one on a restarted listener.
func (a *App) rotateToken(c *ui.Context) {
	if a.agentAPI == nil {
		c.Toast(m(c).SettingsUnavailable())
		return
	}
	if err := a.agentAPI.RotateToken(); err != nil {
		slog.Warn("remit: agent token", "err", err)
		c.Toast(m(c).TokenRotateFailed(err.Error()))
		return
	}
	a.syncPrefs()
	c.Toast(m(c).TokenRotated())
}

// setAgent turns the agent API on or off now, and remembers the choice for the
// next start only when the listener did as it was told.
func (a *App) setAgent(c *ui.Context, on bool) {
	if a.agentAPI == nil {
		a.syncPrefs()
		c.Toast(m(c).SettingsUnavailable())
		return
	}
	if err := a.agentAPI.SetEnabled(on); err != nil {
		slog.Warn("remit: agent api", "on", on, "err", err)
		a.syncPrefs()
		c.Toast(m(c).AgentFailed(err.Error()))
		return
	}
	// A choice that could not be remembered is undone: the port follows the
	// settings rather than serving what they deny.
	if !a.writePrefs(c, func(s *settings.Settings) { s.AgentServer = on }) && on {
		if err := a.agentAPI.SetEnabled(false); err != nil {
			slog.Warn("remit: agent api", "err", err)
		}
	}
	a.syncPrefs()
}

// writePrefs writes the page's change and reports a failure where the change
// was made; the page ends up showing what the file holds either way.
func (a *App) writePrefs(c *ui.Context, fn func(*settings.Settings)) bool {
	if err := a.UpdateSettings(fn); err != nil {
		a.report(c, "save settings", err)
		a.syncPrefs() // the file did not take it: show what it holds
		return false
	}
	return true
}

// checkForUpdate is the check the app runs: MyGo's, and a build that cannot
// update itself says so instead of reporting silence as being up to date.
func checkForUpdate(ctx context.Context, s settings.Settings) (update.Result, error) {
	if !mygo.Updater.Enabled() {
		return update.Result{}, errCannotUpdate
	}
	return update.Check(ctx, s)
}
