package main

import (
	"errors"
	"testing"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/settings"
	appui "github.com/transmit-bug/remit/internal/ui"
	"github.com/transmit-bug/remit/locales"
)

// Applying a shortcut binds the new one and only then gives the old one back;
// a combination that is taken leaves the working one bound, which is what the
// page's error promises.
func TestHotkeyBinder(t *testing.T) {
	bound := map[string]bool{}
	var pressed int
	h := &hotkeyBinder{
		register: func(accelerator string, fn func()) error {
			if accelerator == "Taken" {
				return errors.New("shortcut is already registered")
			}
			bound[accelerator] = true
			return nil
		},
		unregister: func(accelerator string) { delete(bound, accelerator) },
		onPress:    func() { pressed++ },
	}

	if err := h.apply(settings.DefaultHotkey); err != nil {
		t.Fatalf("bind the default: %v", err)
	}
	if !bound[settings.DefaultHotkey] {
		t.Fatalf("the default shortcut is not bound: %v", bound)
	}

	if err := h.apply("CmdOrCtrl+Shift+J"); err != nil {
		t.Fatalf("rebind: %v", err)
	}
	if bound[settings.DefaultHotkey] {
		t.Error("the old shortcut is still bound after a rebind")
	}
	if !bound["CmdOrCtrl+Shift+J"] {
		t.Error("the new shortcut is not bound")
	}

	// A taken combination is refused, and the working binding survives.
	if err := h.apply("Taken"); err == nil {
		t.Fatal("binding a taken accelerator succeeded")
	}
	if !bound["CmdOrCtrl+Shift+J"] {
		t.Error("a refused accelerator gave back the working one")
	}
	if h.current != "CmdOrCtrl+Shift+J" {
		t.Errorf("the binder reports %q as bound", h.current)
	}

	// Binding what is already bound changes nothing, and an empty accelerator
	// is refused rather than left to the platform.
	if err := h.apply("CmdOrCtrl+Shift+J"); err != nil {
		t.Fatalf("rebinding the same accelerator: %v", err)
	}
	if err := h.apply(""); err == nil {
		t.Error("an empty accelerator was bound")
	}

	h.onPress()
	if pressed != 1 {
		t.Errorf("the press handler ran %d times", pressed)
	}
}

// The tray menu starts a review or a reading session, opens the window, pauses
// reminders and quits, in that order, and its pause item is the notification
// setting: flipping it writes settings.json, and a change made on the page moves
// it back.
func TestTrayMenu(t *testing.T) {
	data := t.TempDir()
	a := appui.NewApp(appui.Options{DataDir: data})
	shown := 0
	menu, pause := trayMenu(a, func() { shown++ })

	items := menu.Items()
	if len(items) != 7 {
		t.Fatalf("tray menu has %d items, want 7", len(items))
	}
	if got := items[0].Label; got != locales.ActionReviewCards() {
		t.Errorf("first item is %q, want the review action", got)
	}
	if got := items[1].Label; got != locales.ActionReading() {
		t.Errorf("second item is %q, want the reading action", got)
	}
	if got := items[2].Label; got != locales.OpenRemit() {
		t.Errorf("third item is %q, want Open Remit", got)
	}
	if items[3].Type != mygo.MenuItemSeparator || items[5].Type != mygo.MenuItemSeparator {
		t.Error("the menu's separators are not where they belong")
	}
	if items[4] != pause || pause.Type != mygo.MenuItemCheckbox {
		t.Fatalf("the pause item is not the menu's checkbox: %+v", items[4])
	}
	if got := pause.Label; got != locales.PauseReminders() {
		t.Errorf("pause item is %q", got)
	}
	// Reminders are off by default, so the switch reads "paused".
	if !pause.Checked {
		t.Error("the pause item says reminders are running while they are off")
	}
	if got := items[6].Label; got != locales.QuitRemit() {
		t.Errorf("last item is %q, want Quit", got)
	}

	items[0].Click(items[0], nil)
	if shown != 1 {
		t.Errorf("Start review brought the window forward %d times", shown)
	}
	items[1].Click(items[1], nil)
	if shown != 2 {
		t.Errorf("the reading action brought the window forward %d times", shown)
	}

	// MyGo toggles a checkbox before Click runs: unchecked pause means the
	// user just asked for reminders.
	items[4].Checked = false
	items[4].Click(items[4], nil)
	saved, err := settings.Load(data)
	if err != nil {
		t.Fatalf("settings.Load: %v", err)
	}
	if !saved.Notifications {
		t.Error("un-pausing the tray item did not turn reminders on")
	}
	if !a.Settings().Notifications {
		t.Error("the app does not hold the reminder setting it wrote")
	}

	// The same setting changed elsewhere (the settings page) moves the item.
	follow := &trayUI{pause: pause}
	follow.followSettings(settings.Settings{Notifications: false})
	if !pause.Checked {
		t.Error("the pause item did not follow the setting back to paused")
	}
}

// The tray's update item names the version, and opens the releases page of the
// repository in mygo.json: a browser, not a request of ours.
func TestTrayUpdateItem(t *testing.T) {
	var opened string
	open := func(url string) error { opened = url; return nil }

	item := updateMenuItem("0.2.0", open)
	if got, want := item.Label, "Update to v0.2.0"; got != want {
		t.Errorf("label = %q, want %q", got, want)
	}
	item.Click(item, nil)
	if opened != releasesURL() {
		t.Errorf("opened %q, want %q", opened, releasesURL())
	}
	if got, want := releasesURL(), "https://github.com/"+releasesRepo+"/releases"; got != want {
		t.Errorf("releases URL = %q, want %q", got, want)
	}

	// The item goes above Quit, where a menu whose contents change keeps its
	// stable actions at the ends.
	menu, _ := trayMenu(appui.NewApp(appui.Options{DataDir: t.TempDir()}), func() {})
	tray := &trayUI{menu: menu}
	tray.addUpdate("0.2.0", open)
	items := menu.Items()
	if len(items) != 8 {
		t.Fatalf("menu has %d items after the update was added", len(items))
	}
	if got := items[5].Label; got != "Update to v0.2.0" {
		t.Errorf("the update item is at %d (%q), want above Quit", 5, got)
	}
	if got := items[7].Label; got != locales.QuitRemit() {
		t.Errorf("Quit moved to %q", got)
	}
}
