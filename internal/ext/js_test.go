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

// manifestFor is a js manifest that declares the capabilities given, carrying
// every field the js contract requires.
func manifestFor(name string, caps ...string) string {
	body, err := json.Marshal(map[string]any{
		"id":           "test." + name,
		"name":         name,
		"version":      "1.0.0",
		"apiVersion":   1,
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

	escaping := writeJS(t, `{"id":"test.bad","name":"bad","apiVersion":1,"runtime":"js","entry":"../../etc/passwd"}`, `export function one() { return 1; }`)
	if _, err := ext.OpenJS(escaping, ext.JSLimits{}); err == nil {
		t.Error("an entry outside the extension was accepted")
	}
}

// A warmed pool serves calls without making another runtime, and two calls at
// once share the pool's depth without deadlocking.
func TestJSPoolWarmAndReuse(t *testing.T) {
	dir := writeJS(t, manifestFor("pooled"), `export function one() { return 1; }`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if err := runner.Warm(); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if got := runner.Runtimes(); got != 2 {
		t.Fatalf("warm made %d runtimes, want 2", got)
	}
	if err := runner.Warm(); err != nil {
		t.Fatalf("second warm: %v", err)
	}

	var one int
	if err := runner.Call("one", nil, &one); err != nil {
		t.Fatalf("call: %v", err)
	}
	if one != 1 {
		t.Errorf("one answered %d", one)
	}
	if got := runner.Runtimes(); got != 2 {
		t.Errorf("a warmed pool made another runtime: %d", got)
	}

	// Two calls at once: both borrow, both return, neither blocks the other.
	done := make(chan error, 2)
	for range 2 {
		go func() {
			var n int
			done <- runner.Call("one", nil, &n)
		}()
	}
	for range 2 {
		if err := <-done; err != nil {
			t.Errorf("concurrent call: %v", err)
		}
	}
	if got := runner.Runtimes(); got != 2 {
		t.Errorf("two calls over a warm pool made %d runtimes, want 2", got)
	}
}

// A pooled runtime is kept, not thrown away: module-level state survives from
// one call to the next. This is the trade ADR 0006 accepted for not paying a
// load per call, written down as a test so a change to the pool is a decision.
func TestJSPoolKeepsModuleState(t *testing.T) {
	dir := writeJS(t, manifestFor("counting"), `
let seen = 0;
export function tick() { seen += 1; return seen; }
`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	var seen int
	for i := 1; i <= 3; i++ {
		if err := runner.Call("tick", nil, &seen); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		if seen != i {
			t.Fatalf("tick %d answered %d: the runtime was replaced", i, seen)
		}
	}
}

// Three failed calls in a row stop the extension; the fourth answers
// ErrDisabled without touching a runtime. A success in between clears the
// streak.
func TestJSThreeStrikesDisable(t *testing.T) {
	dir := writeJS(t, manifestFor("flaky"), `
export function go(ok) {
  if (!ok) throw new Error("no");
  return 1;
}
`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	fail := false
	pass := true
	for i := range 2 {
		if err := runner.Call("go", fail, nil); err == nil {
			t.Fatalf("call %d succeeded", i)
		}
	}
	if got := runner.Failures(); got != 2 {
		t.Fatalf("failures = %d, want 2", got)
	}
	if err := runner.Call("go", pass, nil); err != nil {
		t.Fatalf("success after failures: %v", err)
	}
	if got := runner.Failures(); got != 0 {
		t.Fatalf("a success left the streak at %d", got)
	}

	for i := range 3 {
		if err := runner.Call("go", fail, nil); err == nil {
			t.Fatalf("strike %d succeeded", i)
		}
	}
	if !runner.Disabled() {
		t.Fatal("three strikes left the runner enabled")
	}
	before := runner.Runtimes()
	if err := runner.Call("go", pass, nil); !errors.Is(err, ext.ErrDisabled) {
		t.Fatalf("a disabled runner answered %v, want ErrDisabled", err)
	}
	if got := runner.Runtimes(); got != before {
		t.Errorf("the disabled call made a runtime: %d, want %d", got, before)
	}
}

// Warming is when a broken module is heard from, not the first review.
func TestJSWarmSurfacesLoadErrors(t *testing.T) {
	// A module that throws while evaluating: compileJS accepts it (it parses),
	// the load does not.
	dir := writeJS(t, manifestFor("boom"), `throw new Error("boom");`)
	runner, err := ext.OpenJS(dir, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if err := runner.Warm(); err == nil {
		t.Error("warming a module that throws at load succeeded")
	}
}
