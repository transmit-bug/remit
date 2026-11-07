// Package about is the About page: the app's name and version, and its
// license. It reads no library and asks the app for nothing, so State and Deps
// are all it needs of the window that opens it.
package about

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// The window's size in DIPs. It does not resize, so this is its only size: the
// app opens the window at these numbers.
const (
	Width  = 360
	Height = 240
)

// LicenseURL is where the license link goes.
const LicenseURL = "https://www.gnu.org/licenses/agpl-3.0.html"

// State is what the page shows: the app's identity. The app owns both; the
// page reads them.
type State struct {
	Name    string
	Version string
}

// Deps is what the page asks of the app, which is nothing: the About page has
// no action beyond its license link, which the window opens.
type Deps struct{}

// View draws the About page: the name, the version and the license.
func View(c *ui.Context, s State, _ Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	c.SetTheme(tk.Theme)

	ui.Column(c).Fill().Center().Gap(tk.SpaceS).Padding(tk.SpaceXXL).Children(func() {
		ui.Text(c, s.Name).FontSize(tk.TypeTitle).Bold()
		ui.Text(c, msgs.Version(s.Version)).FontSize(tk.TypeBody).TextColor(tk.TextMuted)
		ui.Link(c.Key("license"), msgs.LicenseLink(), LicenseURL)
	})
}
