package settings

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// Setting is one setting a plugin declared, as the host form draws it: the
// plugin it belongs to, the type it holds, its title, and the value this
// library has. The host owns the control and the write; the plugin never runs to
// edit its own settings (docs/specs/plugins/ui.md §6).
type Setting struct {
	// Plugin is the plugin's id, the namespace the value is written in, and
	// PluginName the manifest name the page labels it with.
	Plugin     string
	PluginName string
	Key        string
	// Type is the declared type: bool, number or text.
	Type string
	// Title is the plugin's own message, resolved in the frame's language
	// through Deps.PluginMessage.
	Title string
	// On, Number and Text hold the value; the one Type names is the one the
	// form reads and writes.
	On     bool
	Number float64
	Text   string
}

// settingRequest is the one write a fieldset asked for, applied after it was
// built so that the value the user changed is the value that is written.
type settingRequest struct {
	plugin string
	key    string
	value  string
}

// pluginSettingsFieldset is the settings plugins declared, in the page's host
// form: one field per setting, its control chosen by the declared type. A
// write is the app's, into the plugin's own store.
func pluginSettingsFieldset(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	if len(s.PluginSettings) == 0 {
		return
	}
	var write settingRequest
	ui.Fieldset(c, msgs.SettingsPluginOptions(), func() {
		muted(c, tk, msgs.SettingsPluginOptionsBody())
		for i := range s.PluginSettings {
			setting := &s.PluginSettings[i]
			label := d.pluginMessage(c, setting.Plugin, setting.Title)
			ui.Field(c, label, func() {
				switch setting.Type {
				case "bool":
					if ui.Switch(c, &setting.On).Changed() {
						write = settingRequest{plugin: setting.Plugin, key: setting.Key, value: strconv.FormatBool(setting.On)}
					}
				case "number":
					if ui.NumberInput(c, &setting.Number, -1e9, 1e9, numberStep(setting.Number)).Changed() {
						write = settingRequest{
							plugin: setting.Plugin,
							key:    setting.Key,
							value:  strconv.FormatFloat(setting.Number, 'f', -1, 64),
						}
					}
				default:
					if ui.TextInput(c, &setting.Text).MinHeight(tk.Control).Changed() {
						write = settingRequest{plugin: setting.Plugin, key: setting.Key, value: setting.Text}
					}
				}
			}).Description(setting.PluginName)
		}
	})
	if write.key != "" {
		d.setPluginSetting(c, write.plugin, write.key, write.value)
	}
}

// numberStep is the step a number setting moves by: one for a whole number, and
// a tenth for one the plugin wrote with decimals.
func numberStep(value float64) float64 {
	if value == float64(int64(value)) {
		return 1
	}
	return 0.1
}

// pluginProblems is what a plugin is not doing, in the two layers that are never
// merged: the plugin-level state three failed calls leave — with the reason in the
// plugin's own words and the roles a built-in plays instead — and then the
// contributed entries three failures hid. One Re-enable beside them, because one
// user action clears both counts (docs/specs/plugin-system.md P1: 重新启用是一个
// 用户动作、两个内部计数).
func pluginProblems(c *ui.Context, d Deps, msgs locales.Messages, tk design.Tokens, p Plugin) {
	if len(p.Problems) > 0 {
		ui.Text(c, msgs.PluginProblems()).FontSize(tk.TypeUI).TextColor(tk.Danger)
		for _, problem := range p.Problems {
			ui.Text(c, problem).FontSize(tk.TypeUI).TextColor(tk.Danger).SingleLine()
		}
	}
	pluginHealth(c, msgs, tk, p.Health)
	stopped := p.Health.State == HealthDisabled || p.Health.State == HealthFailed
	if p.Failures == 0 && !stopped {
		return
	}
	ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
		if p.Failures > 0 {
			ui.Text(c, msgs.PluginFailures(p.Failures)).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		}
		ui.Button(c, msgs.PluginReenable()).MinHeight(tk.Control).
			Label(msgs.PluginReenableOf(p.Name)).
			OnClick(func() { d.reenable(c, p.Name) })
	})
	// What the count is made of: each hidden contribution and why it went, so
	// Re-enable is an informed action rather than a blind retry.
	for _, hidden := range p.Hidden {
		ui.Text(c, hidden).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
	}
}

// pluginHealth is the plugin level of the row: the state in the frame's language,
// the reason under it in the plugin's own words — the host translates its labels,
// never a plugin's error — and the roles this library gave to a built-in.
func pluginHealth(c *ui.Context, msgs locales.Messages, tk design.Tokens, h PluginHealth) {
	line := healthLine(msgs, h)
	if line == "" {
		return
	}
	ui.Text(c, line).FontSize(tk.TypeUI).TextColor(tk.Danger)
	if h.Reason != "" {
		ui.Text(c, h.Reason).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
	}
	for _, fallback := range h.Fallback {
		ui.Text(c, msgs.PluginHealthFallback(roleLabel(msgs, fallback.Role), fallback.Player)).
			FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
	}
}

// healthLine is a plugin's plugin-level state as one sentence, "" while it is
// running: a row that says nothing is a plugin that is fine.
func healthLine(msgs locales.Messages, h PluginHealth) string {
	switch h.State {
	case HealthFailing:
		return msgs.PluginHealthFailing(h.Failures)
	case HealthDisabled:
		return msgs.PluginHealthDisabled(h.Failures)
	case HealthFailed:
		return msgs.PluginHealthFailed()
	}
	return ""
}

// roleLabel names a role the way the page does: the words its two role fields
// carry, so a fallback reads as a role and not as a manifest string.
func roleLabel(msgs locales.Messages, role string) string {
	switch role {
	case core.RoleCardsDerive:
		return msgs.PluginRoleCards()
	case core.RoleReviewSchedule:
		return msgs.PluginRoleSchedule()
	}
	return role
}

func (d Deps) pluginMessage(c *ui.Context, plugin, raw string) string {
	if d.PluginMessage == nil {
		return raw
	}
	return d.PluginMessage(c, plugin, raw)
}

func (d Deps) setPluginSetting(c *ui.Context, plugin, key, value string) {
	if d.SetPluginSetting != nil {
		d.SetPluginSetting(c, plugin, key, value)
	}
}

func (d Deps) reenable(c *ui.Context, name string) {
	if d.Reenable != nil {
		d.Reenable(c, name)
	}
}
