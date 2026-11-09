package settings

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// Plugin is one plugin this build can run, as the page shows it: what its
// manifest declares, the roles it can play, and whether this library approved
// it. The built-ins ship with the app, so they are always on and cannot be
// withdrawn.
type Plugin struct {
	Name         string
	ID           string
	Version      string
	Capabilities []string // the capability names the manifest declares
	Roles        []string // the role names the manifest implements
	Builtin      bool
	Enabled      bool
	// Problems is what the plugin's contributes declared that this host cannot
	// use, one line each: the plugin still runs, and only those entries are
	// hidden (docs/specs/plugins/ui.md §8).
	Problems []string
	// Failures is how many of its contributed commands and pages have failed in
	// a row and been hidden.
	Failures int
}

// enableRequest is the approval one frame asked for, applied after the
// fieldset is built so the write happens once, not once per control.
type enableRequest struct {
	name string
	on   bool
}

// roleRequest is the role change one frame asked for.
type roleRequest struct {
	role string
	who  string
}

// pluginsFieldset is the plugin list with its approve switches, and the two
// roles each library plays with one plugin. Approving a plugin is the grant: the
// capabilities it declares are shown beside it. A role change and an approval
// both reach the app here and now, but a plugin starts on the next library
// open, so nothing reloads under the review on screen.
func pluginsFieldset(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	var enable enableRequest
	var role roleRequest
	ui.Fieldset(c, msgs.SettingsPlugins(), func() {
		if !s.HasLibrary {
			muted(c, tk, msgs.SettingsPluginsNeedsLibrary())
			return
		}
		muted(c, tk, msgs.SettingsPluginsBody())
		plugins := d.pluginList()
		if len(plugins) == 0 {
			muted(c, tk, msgs.PluginEmpty())
			return
		}
		roles := d.roles()
		for _, p := range plugins {
			pluginField(c, d, tk, msgs, p, &enable)
		}
		roleField(c, msgs, tk, plugins, roles, core.RoleCardsDerive, msgs.PluginRoleCards(), msgs.PluginRoleCardsBody(), &role)
		roleField(c, msgs, tk, plugins, roles, core.RoleReviewSchedule, msgs.PluginRoleSchedule(), msgs.PluginRoleScheduleBody(), &role)
		if s.PluginErr != "" {
			ui.Text(c, s.PluginErr).FontSize(tk.TypeUI).TextColor(tk.Danger)
		}
	})

	if enable.name != "" {
		d.enable(c, enable.name, enable.on)
	}
	if role.role != "" {
		d.chooseRole(c, role.role, role.who)
	}
}

// pluginField is one plugin: its name, the capabilities approving it grants,
// and the switch that grants them. A built-in carries a badge instead of a
// switch, because withdrawing the app's own card plugin is not a choice.
func pluginField(c *ui.Context, d Deps, tk design.Tokens, msgs locales.Messages, p Plugin, enable *enableRequest) {
	on := p.Enabled
	ui.Column(c).Gap(tk.SpaceXS).Children(func() {
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			ui.Text(c, p.Name).Bold()
			if p.Builtin {
				ui.Badge(c, msgs.PluginBuiltin())
			}
			ui.Spacer(c)
			ui.Text(c, p.Version+" · "+p.ID).FontSize(tk.TypeUI).TextColor(tk.TextFaint)
			if !p.Builtin && ui.Switch(c, &on).Label(msgs.PluginEnable(p.Name)).Changed() {
				enable.name, enable.on = p.Name, on
			}
		})
		if len(p.Capabilities) == 0 {
			ui.Text(c, msgs.PluginNoCapabilities()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		} else {
			ui.Row(c).Gap(tk.SpaceXS).Children(func() {
				for _, capability := range p.Capabilities {
					ui.Badge(c, capability)
				}
			})
		}
		pluginProblems(c, d, msgs, tk, p)
	})
}

// roleField is one role's radio group over the plugins that can play it. Only
// approved plugins are offered: a role chooses among what the user permitted.
func roleField(c *ui.Context, msgs locales.Messages, tk design.Tokens, plugins []Plugin, roles map[string]string, role, label, body string, request *roleRequest) {
	options := make([]Plugin, 0, len(plugins))
	for _, p := range plugins {
		if p.Enabled && slices.Contains(p.Roles, role) {
			options = append(options, p)
		}
	}
	chosen := roles[role]
	before := chosen
	ui.Field(c, label, func() {
		if len(options) == 0 {
			muted(c, tk, msgs.PluginEmpty())
			return
		}
		ui.RadioGroup(c, func() {
			for _, p := range options {
				ui.Radio(c, &chosen, p.Name, optionLabel(p))
			}
		}).Label(label)
	}).Description(body)
	if chosen != before {
		request.role, request.who = role, chosen
	}
}

// optionLabel names a plugin in a radio group: its version tells two builds of
// the same plugin apart, and tells the option from the plugin's own row.
func optionLabel(p Plugin) string {
	if p.Version == "" {
		return p.Name
	}
	return p.Name + " " + p.Version
}

// muted is one quiet line, the page's way of stating what is not there.
func muted(c *ui.Context, tk design.Tokens, text string) {
	ui.Text(c, text).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
}
