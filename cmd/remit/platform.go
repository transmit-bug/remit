package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/deeplink"
	"github.com/transmit-bug/remit/internal/settings"
	appui "github.com/transmit-bug/remit/internal/ui"
	"github.com/transmit-bug/remit/locales"
)

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
	case deeplink.KindToday:
		a.ShowToday()
	case deeplink.KindRead:
		a.ShowRead()
	case deeplink.KindRecall:
		a.ShowRecall()
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

// hotkeyBinder owns the app's one global shortcut. It binds the new
// accelerator before giving the old one back, so a combination that is invalid
// or another app already owns leaves a working shortcut in place, which is what
// "Apply" promises when it shows an error instead.
type hotkeyBinder struct {
	current    string
	register   func(accelerator string, fn func()) error
	unregister func(accelerator string)
	onPress    func()
}

// newHotkeyBinder returns a binder on MyGo's global shortcuts.
func newHotkeyBinder(onPress func()) *hotkeyBinder {
	return &hotkeyBinder{
		register:   mygo.GlobalShortcut.Register,
		unregister: mygo.GlobalShortcut.Unregister,
		onPress:    onPress,
	}
}

// apply binds accelerator, and returns the previous one to the system only
// after the new one took. Binding the one already bound changes nothing.
func (h *hotkeyBinder) apply(accelerator string) error {
	if accelerator == "" {
		return errors.New("no accelerator given")
	}
	if accelerator == h.current {
		return nil
	}
	if err := h.register(accelerator, h.onPress); err != nil {
		return err
	}
	if h.current != "" {
		h.unregister(h.current)
	}
	h.current = accelerator
	return nil
}

// trayUI is the tray and the parts of it the app changes after it is built: the
// menu a newer version adds an item to, and the switch that pauses reminders.
type trayUI struct {
	tray  *mygo.Tray
	menu  *mygo.Menu
	pause *mygo.MenuItem
}

// trayMenu is the tray's menu: start a review, open the window, pause or resume
// the daily reminder, quit. The update item is added when there is a newer
// version to offer.
func trayMenu(a *appui.App, show func()) (*mygo.Menu, *mygo.MenuItem) {
	pause := &mygo.MenuItem{
		ID: "pause-reminders", Type: mygo.MenuItemCheckbox,
		Label: locales.PauseReminders(),
		// The item is the setting seen from the tray, so it shows the
		// reminder's own sense: checked means paused.
		Checked: !a.Settings().Notifications,
		Click: func(item *mygo.MenuItem, _ *mygo.Window) {
			// MyGo toggles a checkbox before Click runs: item.Checked is the
			// state the user just asked for.
			if err := a.UpdateSettings(func(s *settings.Settings) { s.Notifications = !item.Checked }); err != nil {
				slog.Warn("remit: pause reminders", "err", err)
				item.SetChecked(!a.Settings().Notifications)
			}
		},
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{ID: "review", Label: locales.ActionReviewCards(), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.ShowQueue()
			show()
		}},
		{ID: "reading", Label: locales.ActionReading(), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.ShowReading()
			show()
		}},
		{ID: "open", Label: locales.OpenRemit(), Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
		mygo.Separator(),
		pause,
		mygo.Separator(),
		{ID: "quit", Label: locales.QuitRemit(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
	return menu, pause
}

// installTray adds the menu bar icon joined to the window and keeps its title
// in step with the queue and its pause item in step with the settings. A tray
// the platform does not offer is reported, and leaves the app without one.
func installTray(a *appui.App, show func()) *trayUI {
	menu, pause := trayMenu(a, show)
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "main",
		Icon:    trayIcon(),
		Title:   locales.TrayTitle(0),
		ToolTip: locales.TrayTooltip(),
		Menu:    menu,
	})
	if err != nil {
		slog.Warn("remit: tray unavailable", "err", err)
		return &trayUI{menu: menu, pause: pause}
	}
	t := &trayUI{tray: tray, menu: menu, pause: pause}
	// The title reports both queues, so a reader is not drowned out by the
	// cards that are due; the notes' number joins it only when there is one,
	// and the tooltip always names both.
	a.OnCounts(func(counts appui.Counts) {
		title := locales.TrayTitle(counts.Cards)
		if counts.Reading > 0 {
			title = locales.TrayTitleReading(counts.Cards, counts.Reading)
		}
		tray.SetTitle(title)
		tray.SetToolTip(locales.TrayTooltipQueues(counts.Cards, counts.Reading))
	})
	a.OnSettings(t.followSettings)
	return t
}

// followSettings keeps the pause item in step with the setting, wherever the
// change came from: the page, or the item itself.
func (t *trayUI) followSettings(s settings.Settings) { t.pause.SetChecked(!s.Notifications) }

// addUpdate puts the newer version's item above Quit and says so in the
// tooltip. The menu carried none while the app was up to date.
func (t *trayUI) addUpdate(version string, open func(url string) error) {
	if t == nil {
		return
	}
	t.menu.Insert(len(t.menu.Items())-2, updateMenuItem(version, open))
	if t.tray != nil {
		t.tray.SetToolTip(locales.TrayTooltipUpdate(version))
	}
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
