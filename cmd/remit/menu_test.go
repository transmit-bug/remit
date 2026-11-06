package main

import (
	"testing"

	"github.com/egoist/mygo"

	appui "github.com/transmit-bug/remit/internal/ui"
)

// The app menu's About item opens the app's About page, and the standard menus
// stay, with the shortcuts they carry.
func TestMenuBar(t *testing.T) {
	var about int
	menu := menuBar(func() { about++ })

	items := menu.Items()
	want := []mygo.MenuRole{
		mygo.RoleAppMenu, mygo.RoleFileMenu, mygo.RoleEditMenu, mygo.RoleViewMenu, mygo.RoleWindowMenu,
	}
	if len(items) != len(want) {
		t.Fatalf("menu has %d items, want %d", len(items), len(want))
	}
	for i, role := range want {
		if items[i].Role != role {
			t.Errorf("menu %d is %q, want %q", i, items[i].Role, role)
		}
	}

	sub := items[0].Submenu
	if len(sub) == 0 {
		t.Fatal("app menu is empty")
	}
	if got, want := sub[0].Label, "About "+appui.Name; got != want {
		t.Errorf("app menu starts with %q, want %q", got, want)
	}
	if sub[0].Role != "" {
		t.Errorf("About item has role %q; it would run that role as well as open the page", sub[0].Role)
	}
	sub[0].Click(sub[0], nil)
	if about != 1 {
		t.Errorf("About item opened %d pages, want 1", about)
	}
}
