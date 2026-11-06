package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/deeplink"
	appui "github.com/transmit-bug/remit/internal/ui"
	"github.com/transmit-bug/remit/locales"
)

// defaultHotkey starts a review from anywhere. Cmd on macOS, Ctrl elsewhere.
const defaultHotkey = "CmdOrCtrl+Shift+R"

// route applies a remit:// link to the window. A link is a request from
// another process, so an unparseable or unknown one is reported and dropped.
func route(a *appui.App, raw string) {
	r, err := deeplink.Parse(raw)
	if err != nil {
		slog.Warn("remit: deep link", "url", raw, "err", err)
		return
	}
	switch r.Kind {
	case deeplink.KindNote:
		a.ShowNote(r.ID)
	case deeplink.KindCard:
		a.ShowCard(r.ID)
	case deeplink.KindQueue:
		a.ShowQueue()
	case deeplink.KindSearch:
		a.Search(r.Query)
	}
}

// installDeepLinks answers remit:// links. Registered before Run, so the URL
// that started the app is delivered too, and a second instance forwards its
// URL here (single instance lock).
func installDeepLinks(a *appui.App) {
	mygo.App.OnOpenURL(func(raw string) { route(a, raw) })
}

// installHotkey binds the review shortcut. A combination another app already
// owns is reported and the default stays unbound: never a silent failure.
func installHotkey(a *appui.App) {
	if err := mygo.GlobalShortcut.Register(defaultHotkey, a.ShowQueue); err != nil {
		slog.Warn("remit: global shortcut unavailable", "shortcut", defaultHotkey, "err", err)
	}
}

// installTray adds the menu bar icon joined to the window and keeps its title
// in step with the queue.
func installTray(a *appui.App, show func()) *mygo.Tray {
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: locales.ActionReviewCards(), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.ShowQueue()
			show()
		}},
		mygo.Separator(),
		{Label: locales.OpenRemit(), Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
		mygo.Separator(),
		{Label: locales.QuitRemit(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "main",
		Icon:    trayIcon(),
		Title:   locales.TrayTitle(0),
		ToolTip: locales.TrayTooltip(),
		Menu:    menu,
	})
	if err != nil {
		slog.Warn("remit: tray unavailable", "err", err)
		return nil
	}
	a.OnRemaining(func(n int) { tray.SetTitle(locales.TrayTitle(n)) })
	return tray
}

// trayIcon reads the packaged tray icon, if the build shipped one. A title
// alone is a usable tray, so a missing icon is not an error.
func trayIcon() []byte {
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "icon.png"))
	if err != nil {
		return nil
	}
	return data
}
