package settings

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// aboutPage is what this build is: its name, version and license, where its
// library lives, what that library holds, and what the app is built on. It is
// the settings page rather than a second window, so a screenshot of the app can
// say which build it was and which plaintext directory it read.
func aboutPage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	kit.Card(c, func() {
		ui.Column(c).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, s.AppName).FontSize(tk.TypeH1).Bold()
			ui.Text(c, msgs.SettingsVersion(s.Version)).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			ui.Text(c, msgs.SettingsAboutTagline()).FontSize(tk.TypeLabel).TextColor(tk.TextFaint)
			ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
				if d.OpenLicense != nil {
					ui.Button(c, msgs.LicenseLink()).MinHeight(tk.Control).
						OnClick(func() { d.openLicense(c) })
				}
			})
		})
	}).Padding(tk.SpaceL)

	ui.Fieldset(c, msgs.SettingsAboutLibrary(), func() {
		if s.LibraryDir == "" {
			muted(c, tk, msgs.SettingsAboutNoLibrary())
			return
		}
		ui.Text(c, s.LibraryDir).Font("monospace").FontSize(tk.TypeMono).
			TextColor(tk.Text).SingleLine()
	})

	ui.Row(c).Gap(tk.SpaceXXL).AlignItems(ui.Start).Children(func() {
		kit.Stat(c, count(s.Notes), msgs.SettingsAboutNotes())
		kit.Stat(c, count(s.Cards), msgs.SettingsAboutCards())
		kit.Stat(c, count(s.DueNow), msgs.SettingsAboutQueue())
	})

	ui.Fieldset(c, msgs.SettingsCredits(), func() {
		muted(c, tk, msgs.SettingsCreditsBody())
	})
}

func (d Deps) openLicense(c *ui.Context) {
	if d.OpenLicense != nil {
		d.OpenLicense(c)
	}
}

// count is a number of notes or cards as the page prints it.
func count(n int) string { return strconv.Itoa(n) }
