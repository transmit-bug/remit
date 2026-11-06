package ui

import (
	"log/slog"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// startView is the window before a library is open: where the notes would
// live, one action that makes a library there, and one that opens a library
// already on disk. Nothing is created until the user says so.
func (a *App) startView(c *ui.Context, tk tokens) {
	ui.Column(c).Grow(1).Center().Gap(tk.spaceM).Padding(tk.spaceXXL).Children(func() {
		ui.Text(c, m(c).StartTitle()).FontSize(tk.typeTitle).Bold()
		ui.Text(c, m(c).StartBody()).
			FontSize(tk.typeBody).TextColor(tk.TextMuted).MaxWidth(messageWidth)
		if dir := a.startDir(); dir != "" {
			ui.Text(c, dir).FontSize(tk.typeSmall).TextColor(tk.textFaint).
				MaxWidth(messageWidth).SingleLine()
		}
		ui.Row(c).Gap(tk.spaceM).Children(func() {
			ui.PrimaryButton(c, m(c).CreateLibrary()).MinHeight(tk.control).
				OnClick(func() { a.createLibrary(c) })
			ui.Button(c, m(c).OpenLibrary()).MinHeight(tk.control).
				OnClick(func() { a.chooseLibrary(c) })
		})
	})
}

// startDir is the folder the start view names: the one the app was pointed at,
// or Remit's own place in Documents.
func (a *App) startDir() string {
	if a.dir != "" {
		return a.dir
	}
	return a.defaultLibrary
}

// createLibrary makes a library at the place the start view named, opens it,
// and remembers it for the next start.
func (a *App) createLibrary(c *ui.Context) {
	dir := a.startDir()
	if dir == "" {
		slog.Error("remit: create library: no place for it")
		c.Toast(m(c).NoLibraryPlace())
		return
	}
	a.install(c, dir)
}

// chooseLibrary asks the system for a folder and opens the library in it,
// making one when the folder holds none. A canceled dialog changes nothing.
func (a *App) chooseLibrary(c *ui.Context) {
	dir, err := a.pickLibrary()
	if err != nil {
		a.report(c, "choose library", err)
		return
	}
	if dir == "" {
		return
	}
	a.install(c, dir)
}

// pickLibraryDir is the start view's folder picker: the system's, attached to
// no window so it is a free-standing panel on every platform. It blocks until
// the user answers, which is why it is only ever called from a handler.
func pickLibraryDir() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:             locales.OpenLibrary(),
		ButtonLabel:       locales.OpenLibrary(),
		Directory:         true,
		CreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return paths[0], nil
}
