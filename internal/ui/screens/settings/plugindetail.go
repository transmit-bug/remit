package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// pluginDetailView draws S47, one plugin's details, over the plugins page: what
// approving it grants, how far it is trusted, the switch that grants or
// withdraws it, the contributions failures took out with the way back, and the
// pages it contributes.
//
// It is one dialog over the list rather than a row that grows: the list stays
// scannable, and the details keep the dialog's keyboard, focus and Escape.
// Everything it shows is the list's own data — the same Plugin the row was drawn
// from and the same pages — so the two cannot disagree.
func pluginDetailView(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	if s.Detail == "" {
		return
	}
	p, ok := pluginNamed(d.pluginList(), s.Detail)
	if !ok {
		s.Detail = ""
		return
	}
	var enable enableRequest
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.ArrivePanel(kit.StylePanel(c, panel.Width(tk.SettingsWidth).
			Padding(tk.SpaceL).Gap(tk.SpaceM)).
			Label(msgs.SettingsPluginDetailsOf(p.Name)), 0)
		kit.ArriveScrim(backdrop, 0)
		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, p.Name).FontSize(tk.TypeH2).Bold().Grow(1)
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { s.Detail = "" })
			})
			ui.Column(c).Gap(tk.SpaceM).Children(func() {
				detailBody(c, s, d, msgs, tk, p, &enable)
			})
		})
	})
	if !open {
		s.Detail = ""
	}
	if enable.name != "" {
		d.enable(c, enable.name, enable.on)
	}
}

// detailBody is what the sheet says about one plugin, in one column: what its id
// is, how far it is trusted, what approving it grants, the switch, what it could
// not use, and the pages it contributes.
func detailBody(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens, p Plugin, enable *enableRequest) {
	ui.Text(c, msgs.SettingsPluginID()+": "+p.ID+" · "+p.Version).
		Font("monospace").FontSize(tk.TypeMono).TextColor(tk.TextMuted).SingleLine()

	ui.Field(c, msgs.SettingsPluginTrust(), func() {
		ui.Text(c, trustOf(msgs, p)).FontSize(tk.TypeUI)
	}).Description(trustBody(msgs, p))

	ui.Field(c, msgs.SettingsPluginCapabilities(), func() {
		if len(p.Capabilities) == 0 {
			muted(c, tk, msgs.PluginNoCapabilities())
			return
		}
		ui.Row(c).Gap(tk.SpaceXS).AlignItems(ui.Center).Wrap().Children(func() {
			for _, capability := range p.Capabilities {
				ui.Badge(c, capability)
			}
		})
	})

	if len(p.Roles) == 0 {
		muted(c, tk, msgs.PluginNoRoles())
	} else {
		ui.Text(c, msgs.SettingsPluginRoles()+": "+join(p.Roles)).
			FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	}

	// The same grant the row carries: a built-in is the app's own and cannot be
	// withdrawn, so it states that rather than offering a switch that would
	// change nothing.
	if p.Builtin {
		ui.Text(c, msgs.PluginBuiltin()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	} else {
		on := p.Enabled
		if ui.Switch(c, &on).Label(msgs.PluginEnable(p.Name)).Changed() {
			enable.name, enable.on = p.Name, on
		}
	}

	pluginProblems(c, d, msgs, tk, p)
	detailPages(c, s, d, msgs, tk, p)
}

// detailPages names the pages one plugin contributed. The actions that open them
// stay on the list's own page list: one control per page, so the sheet cannot
// offer a second button that reads the same to a screen reader.
func detailPages(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens, p Plugin) {
	var titles []string
	for _, page := range s.Pages {
		if page.Plugin == p.Name {
			titles = append(titles, d.pluginMessage(c, page.Plugin, page.Title))
		}
	}
	if len(titles) == 0 {
		ui.Text(c, msgs.SettingsPluginPages()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		muted(c, tk, msgs.SettingsPluginNoPages())
		return
	}
	ui.Text(c, msgs.SettingsPluginPages()+": "+join(titles)).
		FontSize(tk.TypeUI).TextColor(tk.TextMuted)
}

// pluginNamed answers one plugin of the list by its manifest name.
func pluginNamed(plugins []Plugin, name string) (Plugin, bool) {
	for _, p := range plugins {
		if p.Name == name {
			return p, true
		}
	}
	return Plugin{}, false
}

// trustOf names a plugin's trust level in one line: the app's own build, or one
// this library installed and approved.
func trustOf(msgs locales.Messages, p Plugin) string {
	if p.Builtin {
		return msgs.SettingsPluginTrustBuiltin()
	}
	return msgs.SettingsPluginTrustThirdParty()
}

// trustBody says what that level means for the switch beside it.
func trustBody(msgs locales.Messages, p Plugin) string {
	if p.Builtin {
		return msgs.SettingsPluginTrustBuiltinBody()
	}
	return msgs.SettingsPluginTrustThirdPartyBody()
}
