package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// The plugin directory is cmd's: discovery lists the plugins the app ships
// first, then what the user installed, each with the roles its manifest
// implements, and a role's name starts the runner it names.
func TestPluginServiceDiscoversAndOpens(t *testing.T) {
	s := newPluginService(filepath.Join("..", "..", "examples", "plugins"))
	found, err := s.Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(found) < 3 {
		t.Fatalf("discovered %d plugins, want the two built-ins and the example", len(found))
	}
	if found[0].Name != "cards" || !found[0].Builtin || !slices.Contains(found[0].Roles, core.RoleCardsDerive) {
		t.Errorf("the first plugin is %+v, want the built-in cards with its role", found[0])
	}
	if found[1].Name != "scheduler" || !found[1].Builtin || !slices.Contains(found[1].Roles, core.RoleReviewSchedule) {
		t.Errorf("the second plugin is %+v, want the built-in scheduler with its role", found[1])
	}

	var example *appui.Plugin
	for i := range found {
		if found[i].Name == "cards-mini" {
			example = &found[i]
		}
	}
	if example == nil {
		t.Fatalf("the example plugin is not discovered: %+v", found)
	}
	if example.Builtin || !slices.Contains(example.Roles, core.RoleCardsDerive) {
		t.Errorf("the example is %+v, want a user plugin that derives", example)
	}
	if example.Version != "0.1.0" || example.ID != "example.cards.mini" {
		t.Errorf("the example's manifest reads as %+v", example)
	}

	runner, err := s.Open("cards-mini")
	if err != nil {
		t.Fatalf("Open(cards-mini): %v", err)
	}
	if runner.Name() != "cards-mini" {
		t.Errorf("Open answered the runner named %q, want cards-mini", runner.Name())
	}
	if again, _ := s.Open("cards-mini"); again != runner {
		t.Error("the service started the same plugin twice")
	}
	if _, err := s.Open("ghost"); err == nil {
		t.Error("Open(ghost) answered a runner for a plugin that is not installed")
	}
}

// A role whose plugin is the app's own opens the runner the app ships, by
// name, so the role path is the same as any other.
func TestPluginServiceOpensTheBuiltins(t *testing.T) {
	s := newPluginService(t.TempDir())
	for _, name := range []string{"cards", "scheduler"} {
		runner, err := s.Open(name)
		if err != nil {
			t.Fatalf("Open(%s): %v", name, err)
		}
		if runner.Name() != name {
			t.Errorf("Open(%s) answered %q", name, runner.Name())
		}
	}
}

// A contributed page reads the library through the capability it declared: the
// runner the window is handed is opened with the open library's host, not
// without one. Without it every remit.notes.* call fails and the page shows an
// empty library, which is the bug a screen test with its own host cannot see.
func TestPluginServiceGivesAContributedPageTheLibrary(t *testing.T) {
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

	s := newPluginService(filepath.Join("..", "..", "examples", "plugins"))
	s.SetLibrary(lib)
	runner, err := s.Open("leeches")
	if err != nil {
		t.Fatalf("Open(leeches): %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	var page json.RawMessage
	if err := runner.Call(ext.CommandHook, map[string]any{"name": "leechesData", "context": map[string]any{}}, &page); err != nil {
		t.Fatalf("the page's data command: %v", err)
	}
	if !strings.Contains(string(page), "Leech farm") {
		t.Fatalf("the page did not read the library: %s", page)
	}
}

// A role's plugin starts before its library — the scheduling policy has to be in
// place before the index replays — so the library arrives after the runner does.
// The regression: SetLibrary closed every runner it had started and emptied the
// cache, leaving the runner a role held without a host and closed.
func TestPluginServiceKeepsARunnerAcrossTheLibraryBind(t *testing.T) {
	root := t.TempDir()
	installPlugin(t, root, "late",
		`{"id":"test.late","name":"late","version":"1.0.0","apiVersion":1,"runtime":"js","entry":"main.js","capabilities":["notes:read"]}`,
		`export function get(id) { return remit.notes.get(id); }`)
	s := newPluginService(root)

	runner, err := s.Open("late")
	if err != nil {
		t.Fatalf("Open(late): %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	// No library is open yet: the reason says just that, rather than the plugin
	// being broken or the runner having to be started again.
	var note json.RawMessage
	if err := runner.Call("get", "note_1", &note); !errors.Is(err, ext.ErrNoLibrary) {
		t.Fatalf("with no library the call answered %v, want ErrNoLibrary", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	saved, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	s.SetLibrary(lib)

	// The same runner, so the plugin started once: a second start would answer
	// a different pointer.
	if again, err := s.Open("late"); err != nil || again != runner {
		t.Fatalf("Open after the bind answered %v (%v), want the runner started before it", again, err)
	}
	var got struct {
		Title string `json:"title"`
	}
	if err := runner.Call("get", string(saved.ID), &got); err != nil {
		t.Fatalf("the same runner after the bind: %v", err)
	}
	if got.Title != "Biology" {
		t.Errorf("the runner read %+v after the bind", got)
	}
}

// A plugin that is both enabled and a role is one runner: the enabled list's
// events and the role's hooks reach the same one, so the plugin has one pool and
// one writer of its own store. The regression this guards: startExtensions opened
// its own copy beside the service's, and the plugin was constructed twice.
func TestPluginServiceStartsAnEnabledRolePluginOnce(t *testing.T) {
	root := t.TempDir()
	installPlugin(t, root, "both",
		`{"id":"test.both","name":"both","version":"1.0.0","apiVersion":1,"runtime":"js","entry":"main.js","implements":{"deriveBlock":1,"on_event":1}}`,
		"export function deriveBlock() { return [] }\nexport function on_event() {}\n")
	s := newPluginService(root)
	// The service's one door, counted: startExtensions no longer knows the plugin
	// directory, so this is the only way its runners can be started.
	starts := 0
	s.construct = func(name string) (ext.Hook, error) {
		starts++
		e, err := ext.Load(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		return ext.Open(e, nil)
	}

	lib := openTestLibrary(t)
	if err := lib.SetEnabledExtensions([]string{"both"}); err != nil {
		t.Fatalf("enable both: %v", err)
	}
	s.SetLibrary(lib)
	startExtensions(lib, nil, s)

	// The role path asks for the name the enabled list started.
	runner, err := s.Open("both")
	if err != nil {
		t.Fatalf("Open(both): %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	if starts != 1 {
		t.Fatalf("the plugin was constructed %d times, want once", starts)
	}
	if held := s.open["both"]; held != runner {
		t.Error("the service holds another runner than the one the role was answered")
	}
}

// The enabled list takes both runtimes (ADR 0006): an exec plugin still starts
// and still hears the library's events. The regression this guards: a registry
// that only builds js runners drops a process plugin the library enabled, and its
// on_event never runs.
func TestEnabledExecPluginStartsAndHearsEvents(t *testing.T) {
	requireShell(t)
	root := t.TempDir()
	installProcessPlugin(t, root, "probe")

	s := newPluginService(root)
	// The app gives a process plugin one second per call and the fake must still
	// be scheduled on a machine running the whole suite.
	s.processTimeout = 10 * time.Second
	lib := openTestLibrary(t)
	if err := lib.SetEnabledExtensions([]string{"probe"}); err != nil {
		t.Fatalf("enable probe: %v", err)
	}
	s.SetLibrary(lib)
	startExtensions(lib, nil, s)

	hook, err := s.Enabled("probe")
	if err != nil {
		t.Fatalf("Enabled(probe): %v", err)
	}
	t.Cleanup(func() { _ = hook.Close() })

	if _, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{{Text: "q :: a"}}}); err != nil {
		t.Fatalf("save note: %v", err)
	}

	heard := readHeard(t, root, "probe")
	if !strings.Contains(heard, `"method":"on_event"`) {
		t.Fatalf("the process plugin heard %s, want on_event", heard)
	}
}

// A role runs on js only (ADR 0006): a plugin whose manifest names the process
// runtime is refused, and refused before it starts — a role never spawns a process
// it cannot use. The regression this guards: one registry for both paths quietly
// making a role accept whatever the enabled list accepts.
func TestARoleRefusesAProcessPluginWithoutStartingIt(t *testing.T) {
	requireShell(t)
	root := t.TempDir()
	installProcessPlugin(t, root, "probe")
	s := newPluginService(root)

	_, err := s.Open("probe")
	if err == nil || !strings.Contains(err.Error(), "is not a js plugin") {
		t.Fatalf("Open(probe) answered %v, want the js-only refusal", err)
	}
	if data, err := os.ReadFile(heardFile(root, "probe")); err == nil {
		t.Fatalf("the process started to be refused: %s", data)
	}
	if _, ok := s.open["probe"]; ok {
		t.Error("the refused plugin was registered")
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

// installPlugin writes one js plugin under root, so that a service discovers it
// the way it discovers an installed one.
func installPlugin(t *testing.T, root, name, manifest, source string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(source), 0o644); err != nil {
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
