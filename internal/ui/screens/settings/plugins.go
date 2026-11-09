package settings

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// Plugin is one plugin this build can run, as the pages show it: what its
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
	// Hidden names the contributions three failures took out, one line each, with
	// what failed: the row a reader finds beside Re-enable.
	Hidden []string
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

// pluginsPage is the plugins this library may run: what each declares, whether
// it is approved, its details, and what the host could not use or had to hide.
// Approving a plugin is the grant — the capabilities it declares are what the
// user agrees to — so the switch and the badges sit on one row. The roles the
// library plays, the pages the plugins contributed and the settings they
// declared follow.
func pluginsPage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	var enable enableRequest
	var role roleRequest
	var opened string
	var openPlugin PluginPage

	plugins := d.pluginList()
	// The page's heading names the list; its rows and the two roles follow in
	// the form, with only the groups beside them carrying a legend.
	switch {
	case !s.HasLibrary:
		muted(c, tk, msgs.SettingsPluginsNeedsLibrary())
	case len(plugins) == 0:
		muted(c, tk, msgs.PluginEmpty())
	default:
		for _, p := range plugins {
			pluginField(c, d, tk, msgs, p, &enable, &opened)
		}
		if s.PluginErr != "" {
			notice(c, tk, s.PluginErr)
		}
		roles := d.roles()
		roleField(c, msgs, tk, plugins, roles, core.RoleCardsDerive, msgs.PluginRoleCards(), msgs.PluginRoleCardsBody(), &role)
		roleField(c, msgs, tk, plugins, roles, core.RoleReviewSchedule, msgs.PluginRoleSchedule(), msgs.PluginRoleScheduleBody(), &role)
	}

	if len(plugins) > 0 {
		ui.Fieldset(c, msgs.SettingsPluginPages(), func() {
			pluginPages(c, s, d, msgs, tk, &openPlugin)
		})
	}
	pluginSettingsFieldset(c, s, d, msgs, tk)

	if enable.name != "" {
		d.enable(c, enable.name, enable.on)
	}
	if role.role != "" {
		d.chooseRole(c, role.role, role.who)
	}
	if opened != "" {
		s.Detail = opened
	}
	if openPlugin.Plugin != "" {
		d.openPage(c, openPlugin.Plugin, openPlugin.ID)
	}
}

// pluginField is one plugin: its name and whether it ships with the app, the
// capabilities approving it grants, the switch that grants them, and the details
// the user opens. A built-in carries a badge instead of a switch, because
// withdrawing the app's own card plugin is not a choice.
func pluginField(c *ui.Context, d Deps, tk design.Tokens, msgs locales.Messages, p Plugin, enable *enableRequest, opened *string) {
	on := p.Enabled
	name := p.Name
	ui.Column(c).Key(p.Name).Gap(tk.SpaceXS).Children(func() {
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			ui.Text(c, p.Name).Bold()
			if p.Builtin {
				ui.Badge(c, msgs.PluginBuiltin())
			}
			ui.Spacer(c)
			if !p.Builtin {
				if ui.Switch(c, &on).Label(msgs.PluginEnable(p.Name)).Changed() {
					enable.name, enable.on = name, on
				}
			}
		})
		if len(p.Capabilities) == 0 {
			ui.Text(c, msgs.PluginNoCapabilities()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		} else {
			ui.Row(c).Gap(tk.SpaceXS).AlignItems(ui.Center).Children(func() {
				for _, capability := range p.Capabilities {
					ui.Badge(c, capability)
				}
			})
		}
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, msgs.SettingsPluginDetails()).MinHeight(tk.Control).AlignSelf(ui.Start).
				Label(msgs.SettingsPluginDetailsOf(name)).Clicked() {
				*opened = name
			}
		})
		pluginProblems(c, d, msgs, tk, p)
	})
}

// pluginPages lists the pages the enabled plugins contributed, each with the one
// action that opens it. A plugin whose page failed three times is not here: it
// is in the problems above, with the way back.
func pluginPages(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens, open *PluginPage) {
	if len(s.Pages) == 0 {
		muted(c, tk, msgs.SettingsPluginNoPages())
		return
	}
	for _, page := range s.Pages {
		chosen := page
		// The page's title is the plugin's own message: the host resolves it in
		// the window's language, so the list never shows a %key%.
		title := d.pluginMessage(c, page.Plugin, page.Title)
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(tk.TypeUI).SingleLine()
			ui.Text(c, page.Plugin).FontSize(tk.TypeLabel).TextColor(tk.TextFaint).SingleLine()
			ui.Spacer(c)
			if ui.Button(c, msgs.SettingsPluginPageOpen()).MinHeight(tk.Control).
				Label(msgs.SettingsPluginPageOpenOf(title)).Clicked() {
				*open = chosen
			}
		})
	}
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

// join puts a list on one line, for a row that names the roles a plugin plays.
func join(names []string) string {
	out := ""
	for i, name := range names {
		if i > 0 {
			out += ", "
		}
		out += name
	}
	return out
}

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

func (d Deps) openPage(c *ui.Context, plugin, page string) {
	if d.OpenPage != nil {
		d.OpenPage(c, plugin, page)
	}
}
