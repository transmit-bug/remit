package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/plugin"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/locales"
)

// roleRunners is a supervisor whose directory holds the manifests the tests
// name and whose runners they built: a role resolves through it the way a role
// resolves through the app's, and the fallback is the app's own.
func roleRunners(t *testing.T, runners map[string]*ext.JSRunner) *plugin.Supervisor {
	t.Helper()
	dir := t.TempDir()
	held := make(map[string]ext.Hook, len(runners))
	for name, runner := range runners {
		manifest := ext.Manifest{
			ID:         "test." + name,
			Name:       name,
			Version:    "1",
			APIVersion: 1,
			Runtime:    "js",
			Entry:      "main.js",
			Implements: map[string]int{"deriveBlock": 1, "policy": 1},
		}
		if err := installManifest(t, dir, manifest); err != nil {
			t.Fatalf("install %q: %v", name, err)
		}
		held[name] = runner
	}
	return pluginsWith(t, dir, held)
}

// cardRolePlugin opens a card plugin whose deriveBlock tags the front, so which
// plugin derived a card is what the card reads.
func cardRolePlugin(t *testing.T, name, tag string) *ext.JSRunner {
	t.Helper()
	source := fmt.Sprintf(`
export function deriveBlock(block) {
  const text = (block && block.text) || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{ typeId: "remit.basic", ordinal: 0,
    fields: { Front: %q + " " + text.slice(0, at).trim(), Back: text.slice(at + 2).trim() } }];
}`, tag)
	return roleRunner(t, name, `"deriveBlock":1`, source)
}

// scheduleRolePlugin opens a scheduling plugin with the policy its libraries
// open with: the retention, and the family-version every review is stamped with.
func scheduleRolePlugin(t *testing.T, name, family, version string, retention float64) *ext.JSRunner {
	t.Helper()
	source := fmt.Sprintf(`export function policy() { return { family: %q, version: %q, retention: %v }; }`,
		family, version, retention)
	return roleRunner(t, name, `"policy":1`, source)
}

// roleRunner compiles one test plugin: a manifest declaring the hook given, and
// its entry beside it.
func roleRunner(t *testing.T, name, implements, source string) *ext.JSRunner {
	t.Helper()
	dir := t.TempDir()
	manifest := fmt.Sprintf(
		`{"id":%q,"name":%q,"version":"1","apiVersion":1,"runtime":"js","entry":"main.js","implements":{%s}}`,
		name, name, implements)
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{Timeout: time.Second})
	if err != nil {
		t.Fatalf("open the test plugin %q: %v", name, err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return runner
}

// nameRole approves a plugin in the library at dir and gives it a role, which is
// what the settings page writes: the library's own remit.json, before the window
// opens it.
func nameRole(t *testing.T, dir, role, name string) {
	t.Helper()
	lib, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	defer lib.Close()
	enabled := lib.EnabledExtensions()
	if len(enabled) == 0 || enabled[len(enabled)-1] != name {
		enabled = append(enabled, name)
	}
	if err := lib.SetEnabledExtensions(enabled); err != nil {
		t.Fatalf("enable %q: %v", name, err)
	}
	if err := lib.SetRole(role, name); err != nil {
		t.Fatalf("give %q the role %q: %v", name, role, err)
	}
}

// front is what the card asks, as the window renders it: the card table and the
// review both read this.
func front(a *App, card core.Card) string {
	prompt, _, _ := a.faces(card)
	return prompt
}

// Switching to another library derives its cards through the plugin that
// library names: the roles are the target's, resolved as it opens, not the ones
// the window opened with.
func TestSwitchDerivesThroughTheNewLibrarysCardRole(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	seed := seedLibrary(t, first)
	other := seedLibrary(t, second)
	nameRole(t, second, core.RoleCardsDerive, "acme.cards")
	service := roleRunners(t, map[string]*ext.JSRunner{"acme.cards": cardRolePlugin(t, "acme.cards", "ACME")})

	a := NewApp(Options{LibraryDir: first, Plugins: service})
	if got := front(a, seed.spacing); got != "Spacing beats cramming" {
		t.Fatalf("the first library's card reads %q, want the built-in plugin's answer", got)
	}

	a.OpenLibrary(second)
	if got := front(a, other.spacing); got != "ACME Spacing beats cramming" {
		t.Errorf("the card reads %q, want the new library's card plugin", got)
	}
}

// A switch brings the new library's scheduling plugin with it: the reviews
// written into it carry that plugin's name, not the one the window had.
func TestSwitchStampsReviewsWithTheNewLibrarysScheduler(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	seedLibrary(t, first)
	other := seedLibrary(t, second)
	nameRole(t, second, core.RoleReviewSchedule, "acme.sched")
	service := roleRunners(t, map[string]*ext.JSRunner{"acme.sched": scheduleRolePlugin(t, "acme.sched", "acme", "9", 0.84)})

	a := NewApp(Options{LibraryDir: first, Plugins: service})
	if got := a.Library().Scheduler(); got != "fsrs-1" {
		t.Fatalf("the first library schedules with %q, want the built-in fsrs-1", got)
	}

	a.OpenLibrary(second)
	if got := a.Library().Scheduler(); got != "acme-9" {
		t.Fatalf("the new library schedules with %q, want acme-9", got)
	}
	if err := a.Library().Review(other.spacing.ID, schedule.Good, time.Now()); err != nil {
		t.Fatalf("review: %v", err)
	}
	events := a.Library().Events()
	if len(events) == 0 {
		t.Fatal("the review wrote no record")
	}
	if got := events[len(events)-1].Scheduler; got != "acme-9" {
		t.Errorf("the record is stamped %q, want the new library's acme-9", got)
	}
}

// A library created from the start view opens with its scheduling plugin's
// policy: the retention the plugin asks for, and its name on the reviews, not
// the host's defaults.
func TestNewLibraryOpensWithItsSchedulersPolicy(t *testing.T) {
	service := roleRunners(t, map[string]*ext.JSRunner{"scheduler": scheduleRolePlugin(t, "scheduler", "policy", "3", 0.84)})
	a := NewApp(Options{LibraryDir: filepath.Join(t.TempDir(), "Remit"), Plugins: service})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	if err := tt.Click(locales.In("en").CreateLibrary()); err != nil {
		t.Fatalf("Create library: %v", err)
	}
	if a.Library() == nil {
		t.Fatal("the new library did not open")
	}
	if got := a.Library().ScheduleParams().DesiredRetention; got != 0.84 {
		t.Errorf("the new library schedules at retention %v, want the plugin's 0.84", got)
	}
	if got := a.Library().Scheduler(); got != "policy-3" {
		t.Errorf("the new library schedules with %q, want policy-3", got)
	}
}

// A library the window could not open holds no roles for it: the next library
// that opens derives and stamps with its own plugins, not with the ones
// resolved for the library that never opened.
func TestAFailedOpenHoldsNoRolesForTheLibraryItCouldNotOpen(t *testing.T) {
	// A library that names third-party plugins and is damaged below the config:
	// its roles are readable, the library is not.
	broken := t.TempDir()
	seedLibrary(t, broken)
	nameRole(t, broken, core.RoleCardsDerive, "acme.cards")
	nameRole(t, broken, core.RoleReviewSchedule, "acme.sched")
	if err := os.WriteFile(filepath.Join(broken, "types", "remit.basic.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := t.TempDir()
	seeded := seedLibrary(t, next)
	service := roleRunners(t, map[string]*ext.JSRunner{
		"acme.cards": cardRolePlugin(t, "acme.cards", "ACME"),
		"acme.sched": scheduleRolePlugin(t, "acme.sched", "acme", "9", 0.84),
	})

	a := NewApp(Options{LibraryDir: broken, Plugins: service})
	if a.lib != nil || a.openErr == nil {
		t.Fatalf("the damaged library opened: err %v", a.openErr)
	}
	if a.cards != nil || a.scheduler != nil {
		t.Fatalf("the window holds a role resolved for a library that did not open")
	}

	a.OpenLibrary(next)
	if a.Library() == nil || a.dir != next {
		t.Fatalf("the next library did not open: dir %q", a.dir)
	}
	if got := front(a, seeded.spacing); got != "Spacing beats cramming" {
		t.Errorf("the card reads %q, want the next library's own card plugin", got)
	}
	if got := a.Library().Scheduler(); got != "fsrs-1" {
		t.Errorf("the next library schedules with %q, want its own fsrs-1", got)
	}
}

// A switch to a library that will not open leaves the window on the library it
// had, with its roles: the target's roles are resolved before it opens, and a
// library that never opens commits none of them.
func TestAFailedSwitchLeavesTheLibraryAndItsRolesInPlace(t *testing.T) {
	kept := t.TempDir()
	seeded := seedLibrary(t, kept)
	broken := t.TempDir()
	seedLibrary(t, broken)
	nameRole(t, broken, core.RoleCardsDerive, "acme.cards")
	if err := os.WriteFile(filepath.Join(broken, "types", "remit.basic.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := roleRunners(t, map[string]*ext.JSRunner{"acme.cards": cardRolePlugin(t, "acme.cards", "ACME")})

	a := NewApp(Options{LibraryDir: kept, Plugins: service})
	a.OpenLibrary(broken)
	if a.lib == nil || a.dir != kept {
		t.Fatalf("the window left the library it had: dir %q", a.dir)
	}
	if got := front(a, seeded.spacing); got != "Spacing beats cramming" {
		t.Errorf("the card reads %q, want the library still on screen", got)
	}
}

// Opening a library from the list of libraries opened before resolves its
// roles the same way: its cards derive through its own plugin, and its reviews
// carry its own scheduler.
func TestRecentLibraryOpensWithItsOwnPlugins(t *testing.T) {
	dir := t.TempDir()
	seeded := seedLibrary(t, dir)
	nameRole(t, dir, core.RoleCardsDerive, "acme.cards")
	nameRole(t, dir, core.RoleReviewSchedule, "acme.sched")
	service := roleRunners(t, map[string]*ext.JSRunner{
		"acme.cards": cardRolePlugin(t, "acme.cards", "ACME"),
		"acme.sched": scheduleRolePlugin(t, "acme.sched", "acme", "9", 0.84),
	})
	data := t.TempDir()
	if err := settings.Save(data, settings.Settings{RecentLibraries: []string{dir}}); err != nil {
		t.Fatal(err)
	}

	a := NewApp(Options{DataDir: data, Plugins: service})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	if err := tt.Click(locales.In("en").StartRecentOpen(dir)); err != nil {
		t.Fatalf("the recent row is not a control: %v", err)
	}
	tt.Frame()

	if got := front(a, seeded.spacing); got != "ACME Spacing beats cramming" {
		t.Errorf("the card reads %q, want the library's own card plugin", got)
	}
	if got := a.Library().Scheduler(); got != "acme-9" {
		t.Errorf("the library schedules with %q, want acme-9", got)
	}
}
