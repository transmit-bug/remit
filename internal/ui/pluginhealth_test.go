package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/plugin"
	"github.com/transmit-bug/remit/locales"
)

// brokenEntry is the plugin these tests stop: the callback the window calls
// throws, as a plugin with a bug does.
const brokenEntry = `
export function show() {
  throw new Error("the plugin is broken");
}
`

// openBroken opens a real js runner whose one callback throws: the engine's own
// three-strikes rule is what stops it, not a fake.
func openBroken(t *testing.T) *ext.JSRunner {
	t.Helper()
	dir := t.TempDir()
	manifest := ext.Manifest{
		ID: "test.broken", Name: "broken", Version: "1", APIVersion: 1,
		Runtime: "js", Entry: "main.js",
	}
	if err := installManifest(t, dir, manifest); err != nil {
		t.Fatalf("install: %v", err)
	}
	entry := filepath.Join(dir, manifest.Name, "main.js")
	if err := os.WriteFile(entry, []byte(brokenEntry), 0o644); err != nil {
		t.Fatalf("entry: %v", err)
	}
	runner, err := ext.OpenJS(filepath.Join(dir, manifest.Name), nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return runner
}

// stoppedApp opens the window on a library whose one plugin is stopped at both
// levels at once: three failed calls stopped the plugin, and three failed
// contributions hid one of its commands. Those are the two counts the settings
// page tells apart and the one action clears.
func stoppedApp(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	dir := t.TempDir()
	seedLibrary(t, dir)
	enablePlugin(t, dir, "report")
	manifest := ext.Manifest{
		ID: "acme.report", Name: "report", Version: "1", APIVersion: 1,
		Runtime: "js", Entry: "main.js", Contributes: json.RawMessage(reportContributes),
	}
	extDir := t.TempDir()
	if err := installManifest(t, extDir, manifest); err != nil {
		t.Fatalf("install the test plugin: %v", err)
	}
	runner := openBroken(t)
	a := NewApp(Options{
		LibraryDir: dir,
		Plugins:    pluginsWith(t, extDir, map[string]ext.Hook{"report": runner}),
	})
	a.runBackground = func(c *ui.Context, job func() func(*ui.Context)) { job()(c) }

	hook, err := a.plugins.Runner("report")
	if err != nil {
		t.Fatalf("Runner(report): %v", err)
	}
	for i := range 3 {
		if err := hook.Call("show", nil, nil); err == nil {
			t.Fatalf("call %d was answered by a plugin that throws", i+1)
		}
	}
	for range contrib.FailureLimit {
		a.ui.failures.Failed("acme.report", "acme.report.show", "the plugin threw")
	}
	return a, ui.NewTester(a.View, 1024, 720)
}

// The two counts are shown apart, and one Re-enable clears both: the plugin is
// running again and the hidden command is shown again. The regression this
// guards: a button that clears the hidden entries and leaves the plugin stopped,
// so the next failure puts the row back with no way to revive the plugin
// (issue #266, criteria 1 and 2).
func TestReenableClearsBothCounts(t *testing.T) {
	a, tt := stoppedApp(t)
	msgs := locales.In("en")

	settingsPage(t, tt, a, subSettingsPlugins)
	reveal(t, tt, 1024, 720, msgs.PluginReenable())
	if !tt.HasText(msgs.PluginHealthDisabled(3)) {
		t.Fatalf("the page does not say the plugin is stopped; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.PluginFailures(contrib.FailureLimit)) {
		t.Fatalf("the page does not say its command was hidden; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.PluginReenable()); err != nil {
		t.Fatalf("the re-enable action: %v", err)
	}
	tt.Frame()

	if got := a.plugins.Health("report"); got.State != plugin.HealthOK || got.Failures != 0 {
		t.Fatalf("Re-enable left the plugin at %+v, want it running with no count", got)
	}
	if a.ui.failures.Hidden("acme.report", "acme.report.show") {
		t.Error("Re-enable left the contributed command hidden")
	}
	if tt.HasText(msgs.PluginHealthDisabled(3)) {
		t.Errorf("the page still says the plugin is stopped; texts %q", tt.Texts())
	}
}
