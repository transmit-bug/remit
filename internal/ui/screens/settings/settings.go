// Package settings is the settings page: the agent API, the update check, the
// global shortcut, the daily reminder, the library's plugins and its card
// types, each written as soon as it changes. The page shows the app's state as
// a copy and reports every change through Deps; the app owns the writes, the
// shortcut binding, the listener and the store.
package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	appsettings "github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// width caps the page's measure, the same comfortable column the other
// full-window pages use.
const width float32 = 560

// Deps is what the page asks of the app: one function per thing the user can
// do. Each runs on the UI thread; the app writes the settings and tells the
// services that read them.
type Deps struct {
	// SetAgent starts or stops the agent API now, and remembers the choice.
	SetAgent func(*ui.Context, bool)
	// RotateToken withdraws the agent API's token, so the old one stops
	// working at once.
	RotateToken func(*ui.Context)
	// CheckNow asks whether a newer version exists.
	CheckNow func(*ui.Context)
	// ApplyHotkey binds an accelerator and remembers it.
	ApplyHotkey func(*ui.Context, string)
	// SetUpdateCheck remembers whether Remit checks for updates at launch.
	SetUpdateCheck func(*ui.Context, bool)
	// SetNotifications remembers whether the daily reminder is on.
	SetNotifications func(*ui.Context, bool)

	// Plugins is what discovery found, as the app last saw it: the built-ins
	// and what this user installed. An empty list is stated on the page as
	// such, never as an error.
	Plugins func() []Plugin
	// SetEnabled approves a plugin for this library, or withdraws it. It is
	// the grant: the capabilities the plugin declares apply from the next
	// library open.
	SetEnabled func(*ui.Context, string, bool)
	// Roles is the plugin each role names in this library, the built-in when
	// the library names none.
	Roles func() map[string]string
	// SetRole chooses the plugin for a role; "" returns to the built-in.
	SetRole func(*ui.Context, string, string)
	// Types is this library's card types, with how many cards reference each.
	Types func() []Type
	// SaveType writes a type, minting the ID of a new one and rewriting the
	// references a field rename touches. It answers the type's ID, "" when the
	// app refused the write and said why in the page's State.
	SaveType func(*ui.Context, core.Type) core.ID
	// DeleteType removes a type no card references, and says why in the
	// page's State when one does.
	DeleteType func(*ui.Context, core.ID)

	// SetPluginSetting writes one value into the plugin's own store, under its
	// id: the host owns the write, and the plugin reads the value itself.
	SetPluginSetting func(*ui.Context, string, string, string)
	// PluginMessage resolves one of a plugin's own messages in the frame's
	// language, so the page follows the window's locale rather than a snapshot.
	PluginMessage func(*ui.Context, string, string) string
	// Reenable gives a plugin's failed contributions another try: the counts go
	// back to zero and the hidden entries show again.
	Reenable func(*ui.Context, string)
}

// View draws the settings page: the agent API, updates, the global shortcut,
// the daily reminder, the plugin roles and the card types, each written as soon
// as it is changed.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(width).Gap(tk.SpaceL).Children(func() {
				ui.Text(c, msgs.SettingsTitle()).FontSize(tk.TypeTitle).Bold()
				ui.Form(c, func() {
					agentFieldset(c, s, d, msgs, tk)
					updateFieldset(c, s, d, msgs, tk)
					hotkeyFieldset(c, s, d, msgs, tk)
					reminderFieldset(c, s, d, msgs, tk)
					pluginsFieldset(c, s, d, msgs, tk)
					pluginSettingsFieldset(c, s, d, msgs, tk)
					typesFieldset(c, s, d, msgs, tk)
				})
			})
		})
	})
}

// agentFieldset is the agent API's switch, and what the listener says about
// itself: the address it chose, the token file, and the way to withdraw a
// token that leaked.
func agentFieldset(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	toggled := false
	ui.Fieldset(c, msgs.SettingsAgent(), func() {
		ui.Field(c, msgs.SettingsAgentServe(), func() {
			ui.Column(c).Gap(tk.SpaceS).Children(func() {
				toggled = ui.Switch(c, &s.AgentOn).Changed()
				ui.Column(c).Gap(tk.SpaceXS).Children(func() {
					if s.Agent.Address == "" {
						ui.Text(c, msgs.SettingsAgentStopped()).FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
					} else {
						ui.Text(c, msgs.SettingsAgentAddress(s.Agent.Address)).
							FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
					}
					if s.Agent.TokenPath != "" {
						ui.Text(c, msgs.SettingsTokenPath(s.Agent.TokenPath)).
							FontSize(tk.TypeSmall).TextColor(tk.TextFaint).SingleLine()
					}
				})
				ui.Button(c, msgs.RotateToken()).MinHeight(tk.Control).AlignSelf(ui.Start).
					OnClick(func() { d.RotateToken(c) })
			})
		}).Description(msgs.SettingsAgentBody())
	})
	if toggled {
		d.SetAgent(c, s.AgentOn)
	}
}

// updateFieldset is the update check's switch and the one action that asks
// now, with what the last answer was.
func updateFieldset(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	toggled := false
	ui.Fieldset(c, msgs.SettingsUpdates(), func() {
		ui.Field(c, msgs.SettingsCheckAtLaunch(), func() {
			ui.Column(c).Gap(tk.SpaceS).Children(func() {
				toggled = ui.Switch(c, &s.UpdateCheck).Changed()
				ui.Row(c).Gap(tk.SpaceM).AlignItems(ui.Center).Children(func() {
					ui.Button(c, msgs.CheckNow()).MinHeight(tk.Control).
						OnClick(func() { d.CheckNow(c) })
					if s.UpdateMsg != "" {
						ui.Text(c, s.UpdateMsg).FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
					}
				})
			})
		}).Description(msgs.SettingsUpdatesBody())
	})
	if toggled {
		d.SetUpdateCheck(c, s.UpdateCheck)
	}
}

// hotkeyFieldset is the global shortcut: the accelerator as typed, the one
// action that binds it, and why the last attempt did not.
func hotkeyFieldset(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	ui.Fieldset(c, msgs.SettingsShortcut(), func() {
		ui.Field(c, msgs.SettingsShortcutLabel(), func() {
			ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &s.Hotkey).
					Placeholder(appsettings.DefaultHotkey).MinHeight(tk.Control).Grow(1).
					Label(msgs.SettingsShortcutLabel())
				ui.Button(c, msgs.ApplyShortcut()).MinHeight(tk.Control).
					OnClick(func() { d.ApplyHotkey(c, s.Hotkey) })
			})
		}).Description(msgs.SettingsShortcutBody()).Error(hotkeyError(msgs, s))
	})
}

// reminderFieldset is the daily reminder's switch. It is the same setting the
// tray's pause item flips.
func reminderFieldset(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	toggled := false
	ui.Fieldset(c, msgs.SettingsReminders(), func() {
		ui.Field(c, msgs.SettingsReminderLabel(), func() {
			toggled = ui.Switch(c, &s.Notifications).Changed()
		}).Description(msgs.SettingsReminderBody())
	})
	if toggled {
		d.SetNotifications(c, s.Notifications)
	}
}

// hotkeyError is the shortcut field's inline error, empty while the last
// attempt worked.
func hotkeyError(msgs locales.Messages, s *State) string {
	if s.HotkeyErr == "" {
		return ""
	}
	return msgs.ShortcutUnavailable(s.HotkeyErr)
}

// The rest of the page reads and writes through Deps, which may be partly
// empty: a build with no plugin service, or a test that drives one group, says
// so on the page rather than panicking in a frame.

func (d Deps) pluginList() []Plugin {
	if d.Plugins == nil {
		return nil
	}
	return d.Plugins()
}

func (d Deps) roles() map[string]string {
	if d.Roles == nil {
		return nil
	}
	return d.Roles()
}

func (d Deps) typeList() []Type {
	if d.Types == nil {
		return nil
	}
	return d.Types()
}

func (d Deps) enable(c *ui.Context, name string, on bool) {
	if d.SetEnabled != nil {
		d.SetEnabled(c, name, on)
	}
}

func (d Deps) chooseRole(c *ui.Context, role, name string) {
	if d.SetRole != nil {
		d.SetRole(c, role, name)
	}
}

func (d Deps) save(c *ui.Context, typ core.Type) core.ID {
	if d.SaveType == nil {
		return ""
	}
	return d.SaveType(c, typ)
}

func (d Deps) removeType(c *ui.Context, id core.ID) {
	if d.DeleteType != nil {
		d.DeleteType(c, id)
	}
}
