package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/locales"
)

// configOf reads the library's remit.json as the file holds it.
func configOf(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "remit.json"))
	if err != nil {
		t.Fatalf("read remit.json: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("remit.json: %v", err)
	}
	return config
}

// The settings page's plugins and types are the library's: approving a plugin
// and choosing a role rewrite remit.json, and saving a type goes through the
// store's write path to types/<id>.json. The page never writes; the app does.
func TestSettingsPluginAndRoleWritesReachTheLibrary(t *testing.T) {
	msgs := locales.In("en")
	discovered := []Plugin{
		{Name: "cards", ID: "remit.cards.standard", Version: "1.0.0", Roles: []string{core.RoleCardsDerive}, Builtin: true, Enabled: true},
		{Name: "scheduler", ID: "remit.scheduler.default", Version: "1.0.0", Roles: []string{core.RoleReviewSchedule}, Builtin: true, Enabled: true},
		{Name: "acme.cards", ID: "acme.cards", Version: "0.3.0", Capabilities: []string{"notes:read"}, Roles: []string{core.RoleCardsDerive}},
	}
	a := settingsApp(t, Options{Discover: func() ([]Plugin, error) { return discovered, nil }})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	a.openSettings()
	settle(tt)

	reveal(t, tt, 1024, 720, msgs.PluginEnable("acme.cards"))
	if err := tt.Click(msgs.PluginEnable("acme.cards")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.lib.EnabledExtensions(); len(got) != 1 || got[0] != "acme.cards" {
		t.Fatalf("the library enables %v, want acme.cards", got)
	}
	config := configOf(t, a.dir)
	extensions, _ := config["extensions"].([]any)
	if len(extensions) != 1 || extensions[0] != "acme.cards" {
		t.Fatalf("remit.json extensions = %v, want acme.cards", config["extensions"])
	}

	reveal(t, tt, 1024, 720, "acme.cards 0.3.0")
	if err := tt.Click("acme.cards 0.3.0"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	roles, _ := configOf(t, a.dir)["roles"].(map[string]any)
	if roles[core.RoleCardsDerive] != "acme.cards" {
		t.Fatalf("remit.json roles = %v, want cards.derive on acme.cards", roles)
	}
	if got := a.lib.Roles()[core.RoleCardsDerive]; got != "acme.cards" {
		t.Errorf("the library reads cards.derive as %q, want acme.cards", got)
	}
}

// Saving a type from the page writes the type file the store owns, and the
// app's own catalog shows it: the page asked the app, the app asked the store.
func TestSettingsTypeWriteReachesTheLibrary(t *testing.T) {
	msgs := locales.In("en")
	a := settingsApp(t, Options{})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	a.openSettings()
	settle(tt)

	reveal(t, tt, 1024, 720, msgs.TypeFrontTemplate())
	if err := tt.Click(msgs.TypeFrontTemplate()); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("{{Back}}?")
	tt.Frame()
	reveal(t, tt, 1024, 720, msgs.TypeSave())
	if err := tt.Click(msgs.TypeSave()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	typ, ok := a.lib.Type(core.TypeBasic)
	if !ok || typ.Front != "{{Back}}?" {
		t.Fatalf("the library's Basic type is %+v, want the edited front template", typ)
	}
	data, err := os.ReadFile(filepath.Join(a.dir, "types", string(core.TypeBasic)+".json"))
	if err != nil {
		t.Fatalf("read the type file: %v", err)
	}
	var file struct {
		Front string `json:"front"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("type file: %v", err)
	}
	if file.Front != "{{Back}}?" {
		t.Errorf("types/remit.basic.json front = %q, want the saved template", file.Front)
	}
	if !a.prefs.HasLibrary {
		t.Error("the page does not know a library is open")
	}
}

// A library that names no roles reads the built-ins, and the settings page is
// handed exactly that: the app's snapshot, not the store, is what a frame sees.
func TestSettingsRolesSnapshot(t *testing.T) {
	a := settingsApp(t, Options{})
	a.openSettings()
	d := a.settingsDeps()
	roles := d.Roles()
	if roles[core.RoleCardsDerive] != store.DefaultCardsPlugin ||
		roles[core.RoleReviewSchedule] != store.DefaultSchedulerPlugin {
		t.Fatalf("the page is handed roles %v, want the built-ins", roles)
	}
	roles[core.RoleCardsDerive] = "somebody.else"
	if got := d.Roles()[core.RoleCardsDerive]; got != store.DefaultCardsPlugin {
		t.Errorf("the page changed the app's roles to %q through the snapshot", got)
	}
}
