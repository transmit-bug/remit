package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// licenseURL is where the About page's license link goes.
const licenseURL = "https://www.gnu.org/licenses/agpl-3.0.html"

// The About window's size in DIPs. It does not resize, so this is its only
// size.
const (
	aboutWidth  = 360
	aboutHeight = 240
)

// About draws the About page: the app's name and version, and its license. It
// reads no state; what it shows is the app's identity.
func About(c *ui.Context) {
	t, palette := appTheme(c.Theme())
	c.SetTheme(t)
	tk := newTokens(t, palette)

	ui.Column(c).Fill().Center().Gap(tk.spaceS).Padding(tk.spaceXXL).Children(func() {
		ui.Text(c, Name).FontSize(tk.typeTitle).Bold()
		ui.Text(c, m(c).Version(Version)).FontSize(tk.typeBody).TextColor(tk.textMuted)
		ui.Link(c.Key("license"), m(c).LicenseLink(), licenseURL)
	})
}

// ShowAbout opens the About window above parent, or brings it back to the
// front when it is already open.
func (a *App) ShowAbout(parent *mygo.Window) {
	if a.aboutWin != nil {
		a.aboutWin.Focus()
		return
	}
	a.aboutWin = mygo.NewWindow(mygo.WindowOptions{
		Title:         "About " + Name,
		Parent:        parent,
		Width:         aboutWidth,
		Height:        aboutHeight,
		DisableResize: true,
		Content:       ui.View(About),
	})
	a.aboutWin.OnClosed(func() { a.aboutWin = nil })
}
