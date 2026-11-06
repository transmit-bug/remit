package ext_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/ext"
)

// writeJS lays out an extension in a temporary directory and returns it.
func writeJS(t *testing.T, manifest string, source string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// manifestFor is a js manifest that declares the capabilities given.
func manifestFor(name string, caps ...string) string {
	body, err := json.Marshal(map[string]any{
		"name":         name,
		"version":      "1.0.0",
		"protocol":     1,
		"runtime":      "js",
		"entry":        "main.js",
		"capabilities": caps,
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

// A js extension is loaded, called and answered like any other: the same
// manifest, the same call shape.
func TestJSCall(t *testing.T) {
	dir := writeJS(t, manifestFor("cards"), `
export function describe() {
  return { name: "cards", protocol: 1 };
}
export function cards(block) {
  const text = block.text || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{ block: block.id, front: text.slice(0, at).trim(), back: text.slice(at + 2).trim() }];
}
`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if runner.Name() != "cards" {
		t.Errorf("name is %q", runner.Name())
	}

	var described struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	}
	if err := runner.Call("describe", nil, &described); err != nil {
		t.Fatalf("describe: %v", err)
	}
	if described.Name != "cards" || described.Protocol != 1 {
		t.Errorf("describe answered %+v", described)
	}

	var cards []struct {
		Block string `json:"block"`
		Front string `json:"front"`
		Back  string `json:"back"`
	}
	err = runner.Call("cards", map[string]any{"id": "block_1", "text": "spacing :: the gaps"}, &cards)
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != 1 || cards[0].Front != "spacing" || cards[0].Back != "the gaps" || cards[0].Block != "block_1" {
		t.Errorf("cards answered %+v", cards)
	}
}

// The host object is what the manifest declares and nothing else: an extension
// reaches no door it did not name.
func TestJSCapabilitiesGateTheHost(t *testing.T) {
	source := `
export function echo(x) { return remit.echo(x); }
`
	without := writeJS(t, manifestFor("quiet"), source)
	quiet, err := ext.OpenJS(without, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer quiet.Close()
	var echoed string
	if err := quiet.Call("echo", "hello", &echoed); err == nil {
		t.Errorf("an undeclared capability reached the host: %q", echoed)
	}
	if quiet.Has(ext.CapEcho) {
		t.Error("Has reports a capability the manifest never declared")
	}

	with := writeJS(t, manifestFor("loud", ext.CapEcho), source)
	loud, err := ext.OpenJS(with, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer loud.Close()
	if err := loud.Call("echo", "hello", &echoed); err != nil {
		t.Fatalf("declared capability failed: %v", err)
	}
	if echoed != "hello" {
		t.Errorf("echo answered %q, want hello", echoed)
	}
}

// A hook that never returns is interrupted, and the caller gets its time back.
func TestJSTimeout(t *testing.T) {
	dir := writeJS(t, manifestFor("spin"), `export function spin() { for (;;) {} }`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	start := time.Now()
	err = runner.Call("spin", nil, nil)
	if !errors.Is(err, ext.ErrTimeout) {
		t.Fatalf("call answered %v, want a timeout", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("the interrupt took %v", took)
	}
}

// A hook that allocates without end hits the limit and is reported, not allowed
// to take the process down.
func TestJSMemoryLimit(t *testing.T) {
	dir := writeJS(t, manifestFor("hungry"), `
export function hungry() {
  const kept = [];
  for (;;) kept.push(new Array(4096).fill(7));
}
`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{Memory: 2 << 20, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	err = runner.Call("hungry", nil, nil)
	if !errors.Is(err, ext.ErrMemoryLimit) && !errors.Is(err, ext.ErrTimeout) {
		t.Fatalf("call answered %v, want the memory limit or the time", err)
	}
}

// Dynamic code is off: a hook cannot build code to run.
func TestJSNoDynamicCode(t *testing.T) {
	dir := writeJS(t, manifestFor("dynamic"), `export function dynamic() { return eval("1 + 1"); }`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if err := runner.Call("dynamic", nil, nil); err == nil {
		t.Error("eval ran in an extension")
	}
}

// A hook the extension does not carry, and an entry that points outside its own
// directory, are refused.
func TestJSRefusesWhatItCannotRun(t *testing.T) {
	dir := writeJS(t, manifestFor("small"), `export function one() { return 1; }`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if err := runner.Call("two", nil, nil); !errors.Is(err, ext.ErrNoHook) {
		t.Errorf("a missing hook answered %v", err)
	}

	escaping := writeJS(t, `{"name":"bad","protocol":1,"runtime":"js","entry":"../../etc/passwd"}`, `export function one() { return 1; }`)
	if _, err := ext.OpenJS(escaping, ext.JSLimits{}); err == nil {
		t.Error("an entry outside the extension was accepted")
	}
}
