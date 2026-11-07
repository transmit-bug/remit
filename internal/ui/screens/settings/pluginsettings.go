package settings

import (
	"strconv"

	"github.com/egoist/mygo/ui"

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

// pluginProblems lists what a plugin's manifest declared that this host cannot
// use, and how many of its entries failed and were hidden.
func pluginProblems(c *ui.Context, d Deps, msgs locales.Messages, tk design.Tokens, p Plugin) {
	if len(p.Problems) > 0 {
		ui.Text(c, msgs.PluginProblems()).FontSize(tk.TypeSmall).TextColor(tk.Danger)
		for _, problem := range p.Problems {
			ui.Text(c, problem).FontSize(tk.TypeSmall).TextColor(tk.Danger).SingleLine()
		}
	}
	if p.Failures == 0 {
		return
	}
	ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
		ui.Text(c, msgs.PluginFailures(p.Failures)).FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
		ui.Button(c, msgs.PluginReenable()).MinHeight(tk.Control).
			OnClick(func() { d.reenable(c, p.Name) })
	})
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
