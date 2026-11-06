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
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(8).Padding(24).Children(func() {
		ui.Text(c, Name).FontSize(22).Bold()
		ui.Text(c, "Version "+Version).TextColor(t.TextMuted)
		ui.Link(c.Key("license"), "GNU AGPL-3.0", licenseURL)
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
