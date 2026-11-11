package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
	"github.com/transmit-bug/remit/internal/plugin"
)

// A contributed page reads the library through the capability it declared: the
// runner the window is handed acts on the library the supervisor bound, not on
// none. Without it every remit.notes.* call fails and the page shows an empty
// library, which is the bug a screen test with its own host cannot see.
func TestExtensionsGiveAContributedPageTheLibrary(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	if _, err := lib.SaveNote(core.Note{
		Title:  "Leech farm",
		Blocks: []core.Block{{Text: "too big :: an answer"}},
	}); err != nil {
		t.Fatalf("save note: %v", err)
	}

	plugins := plugin.New(filepath.Join("..", "..", "examples", "plugins"))
	t.Cleanup(func() { _ = plugins.Close() })
	plugins.Bind(hostFor(lib), []string{"leeches"})

	runner, err := plugins.Runner("leeches")
	if err != nil {
		t.Fatalf("Runner(leeches): %v", err)
	}
	var page json.RawMessage
	if err := runner.Call(ext.CommandHook, map[string]any{"name": "leechesData", "context": map[string]any{}}, &page); err != nil {
		t.Fatalf("the page's data command: %v", err)
	}
	if !strings.Contains(string(page), "Leech farm") {
		t.Fatalf("the page did not read the library: %s", page)
	}
}

// The enabled list takes both runtimes (ADR 0006): an exec plugin starts when the
// library binds and still hears its events. The regression this guards: a bind
// that starts only js plugins drops a process plugin the library enabled, and its
// on_event never runs.
func TestAnEnabledProcessPluginHearsTheLibrarysEvents(t *testing.T) {
	requireShell(t)
	root := t.TempDir()
	installProcessPlugin(t, root, "probe")

	plugins := plugin.New(root)
	// The fake is a shell script; an app call gives it a second, and the whole
	// suite is running beside this test.
	plugins.ProcessTimeout = 10 * time.Second
	t.Cleanup(func() { _ = plugins.Close() })
	lib := openTestLibrary(t)
	if err := lib.SetEnabledExtensions([]string{"probe"}); err != nil {
		t.Fatalf("enable probe: %v", err)
	}
	plugins.Bind(hostFor(lib), lib.EnabledExtensions())
	startExtensions(lib, nil, plugins)

	if _, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{{Text: "q :: a"}}}); err != nil {
		t.Fatalf("save note: %v", err)
	}

	heard := readHeard(t, root, "probe")
	if !strings.Contains(heard, `"method":"on_event"`) {
		t.Fatalf("the process plugin heard %s, want on_event", heard)
	}
}

// A switch of libraries re-subscribes the same runners: the plugin hears the
// library it was bound to last, and its runner is not restarted. The regression
// this guards: a switch closing the runners the previous library had and leaving
// the events with a dead handler.
func TestASwitchKeepsTheRunnersTheWindowHearsFrom(t *testing.T) {
	root := t.TempDir()
	installPlugin(t, root, "acme")
	first, second := openTestLibrary(t), openTestLibrary(t)

	plugins := plugin.New(root)
	t.Cleanup(func() { _ = plugins.Close() })
	firstNames, secondNames := []string{"acme"}, []string{"acme"}
	if err := first.SetEnabledExtensions(firstNames); err != nil {
		t.Fatalf("enable acme: %v", err)
	}
	if err := second.SetEnabledExtensions(secondNames); err != nil {
		t.Fatalf("enable acme in the second library: %v", err)
	}

	plugins.Bind(hostFor(first), firstNames)
	startExtensions(first, nil, plugins)
	before, err := plugins.Runner("acme")
	if err != nil {
		t.Fatalf("Runner(acme): %v", err)
	}

	plugins.Bind(hostFor(second), secondNames)
	startExtensions(second, nil, plugins)
	if after, err := plugins.Runner("acme"); err != nil || after != before {
		t.Fatalf("the switch answered %v (%v), want the runner it had", after, err)
	}
}

// openTestLibrary opens an empty library a test drives the plugins against.
func openTestLibrary(t *testing.T) *store.Library {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib
}

// installPlugin writes one js plugin under root, so that the supervisor discovers
// it the way it discovers an installed one.
func installPlugin(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	manifest := fmt.Sprintf(`{"id":"test.%s","name":%q,"version":"1.0.0","apiVersion":1,"runtime":"js","entry":"main.js","implements":{"on_event":1}}`, name, name)
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte("export function on_event(event) {}\n"), 0o644); err != nil {
		t.Fatalf("entry: %v", err)
	}
}

// processScript is a whole exec plugin: it appends every request it hears to the
// log named by its first argument and answers it, echoing the id, so the host sees
// one answered call. It spawns nothing: the app gives a call one second.
const processScript = `#!/bin/sh
while IFS= read -r line; do
	printf '%s\n' "$line" >> "$1"
	id=${line#*\"id\":}
	id=${id%%,*}
	printf '{"id":%s,"result":null}\n' "${id:-0}"
done
`

// installProcessPlugin writes an exec plugin under root: a manifest naming the
// process runtime, and a script that logs what the app delivers to it.
func installProcessPlugin(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	script := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(script, []byte(processScript), 0o755); err != nil {
		t.Fatalf("plugin script: %v", err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"version":"1.0.0","protocol":1,"entrypoint":[%q,%q]}`,
		name, script, heardFile(root, name))
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
}

// heardFile is where the process plugin named by name writes what it heard. It
// lives beside the manifest, so a test finds it without a second hand-off — and a
// plugin that never started never made it.
func heardFile(root, name string) string { return filepath.Join(root, name, "heard.log") }

// readHeard answers what the process plugin named by name heard.
func readHeard(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(heardFile(root, name))
	if err != nil {
		t.Fatalf("the process plugin heard nothing: %v", err)
	}
	return string(data)
}

// requireShell skips a test whose fake plugin is a shell script.
func requireShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake process plugin is a shell script")
	}
}

// The names a library's default roles hold and the names this build ships are the
// same two strings. They are held apart because internal/core cannot depend on
// the extension substrate; a drift here would resolve a default role to a plugin
// nothing builds, and the library would open without the roles it names.
func TestTheShippedPluginNamesMatchTheLibrarysDefaults(t *testing.T) {
	for role, shipped := range map[string]string{
		core.RoleCardsDerive:    builtin.CardsName,
		core.RoleReviewSchedule: builtin.SchedulerName,
	} {
		def, ok := store.DefaultRole(role)
		if !ok || def != shipped {
			t.Errorf("the default for %q is %q, want the shipped %q", role, def, shipped)
		}
	}
}
