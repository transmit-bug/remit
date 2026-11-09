// Package start is the window before a library is open: where the notes would
// live, one action that makes a library there, and one that opens a library
// already on disk. It reads no store and no settings — the app names the place
// and owns both actions — so nothing is created until the user says so.
package start

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// State is what the screen shows: the folder a library would go in, when the
// app has one to name.
type State struct {
	Dir string
}

// Deps is what the screen asks of the app. Both actions run on the UI thread;
// writing the library and the system's dialog stay in the app, so the screen
// only reports that the user clicked.
type Deps struct {
	// Dir is the place a library would go, asked when State names none.
	Dir func() string
	// Create makes a library where the screen names it, opens it and remembers
	// it for the next start.
	Create func(*ui.Context)
	// Choose asks the system for a folder and opens the library in it, making
	// one when the folder holds none.
	Choose func(*ui.Context)
}

// View draws the screen: where the notes would live, and the two ways to
// begin.
func View(c *ui.Context, s State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())

	dir := s.Dir
	if dir == "" && d.Dir != nil {
		dir = d.Dir()
	}

	ui.Column(c).Grow(1).Center().Gap(tk.SpaceM).Padding(tk.SpaceXXL).Children(func() {
		tk.Icon(c, "book-open", design.IconLarge).TextColor(tk.TextMuted)
		ui.Text(c, msgs.StartTitle()).FontSize(tk.TypeH2).Bold()
		ui.Text(c, msgs.StartBody()).
			FontSize(tk.TypeUI).TextColor(tk.TextMuted).MaxWidth(design.MessageWidth)
		if dir != "" {
			ui.Text(c, dir).FontSize(tk.TypeUI).TextColor(tk.TextFaint).
				MaxWidth(design.MessageWidth).SingleLine()
		}
		ui.Row(c).Gap(tk.SpaceM).Children(func() {
			ui.PrimaryButton(c, msgs.CreateLibrary()).MinHeight(tk.Control).
				OnClick(func() { d.Create(c) })
			ui.Button(c, msgs.OpenLibrary()).MinHeight(tk.Control).
				OnClick(func() { d.Choose(c) })
		})
	})
}
