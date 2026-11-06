// Package ui draws Remit's window. It owns presentation and input handling and
// keeps domain rules in internal/core.
package ui

import "github.com/egoist/mygo/ui"

// App is the state the window shows. The view rebuilds the interface from it
// on every frame; handlers change it.
type App struct {
	library string
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
		ui.Text(c, "Remit").FontSize(22).Bold()
		ui.Text(c, a.library).TextColor(t.TextMuted)
	})
}
