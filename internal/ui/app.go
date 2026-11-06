// Package ui draws Remit's window. It owns presentation and input handling and
// keeps domain rules in internal/core.
package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Name and Version are the app's identity: the About page shows them, the
// window title and the menus use Name, and mygo.json carries Version for
// packaging. TestVersionMatchesConfig keeps the two in step.
const (
	Name    = "Remit"
	Version = "0.1.0"
)

// App is the state the window shows. The view rebuilds the interface from it
// on every frame; handlers change it.
type App struct {
	library  string
	aboutWin *mygo.Window // the open About window, or nil
}

// NewApp returns an app with no library open yet.
func NewApp() *App {
	return &App{library: "No library open"}
}

// View draws the window. It reads state and returns; anything that changes
// state belongs in a handler.
func (a *App) View(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(8).Padding(24).Children(func() {
		ui.Text(c, Name).FontSize(22).Bold()
		ui.Text(c, a.library).TextColor(t.TextMuted)
	})
}
