package ext

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fakePath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "remit-ext-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fakePath = filepath.Join(dir, "fakeext")
	build := exec.Command("go", "build", "-o", fakePath, "./testdata/fakeext")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build fake extension:", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// writeExtension creates an extension directory with the given manifest body.
func writeExtension(t *testing.T, root, name, manifest string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return dir
}

func startFake(t *testing.T) *Runner {
	t.Helper()
	dir := writeExtension(t, t.TempDir(), "fake", `{"name":"fake","version":"1","protocol":1,"entrypoint":["`+fakePath+`"]}`)
	ext, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// The machine can be busy with every other package's tests: the fake speaks
	// at once, and this only waits for it to be scheduled.
	r, err := Start(ext, StartOptions{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestDiscoverSkipsBrokenExtensions(t *testing.T) {
	root := t.TempDir()
	writeExtension(t, root, "good", `{"name":"good","version":"1","protocol":1,"entrypoint":["`+fakePath+`"]}`)
	writeExtension(t, root, "broken", `{"name":"broken"`)
	writeExtension(t, root, "wrongproto", `{"name":"old","version":"1","protocol":99,"entrypoint":["x"]}`)

	got, err := Discover(root)
	if err == nil {
		t.Error("Discover reported no problem for two broken extensions")
	}
	if len(got) != 1 || got[0].Manifest.Name != "good" {
		t.Errorf("Discover returned %+v, want only the good one", got)
	}
}

func TestDiscoverMissingRootIsEmpty(t *testing.T) {
	got, err := Discover(filepath.Join(t.TempDir(), "nope"))
	if err != nil || len(got) != 0 {
		t.Errorf("Discover(missing) = (%v, %v), want empty and no error", got, err)
	}
}

func TestCallRoundTrip(t *testing.T) {
	r := startFake(t)

	var described struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	raw, err := r.Describe()
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if err := json.Unmarshal(raw, &described); err != nil {
		t.Fatalf("decode describe: %v", err)
	}
	if described.Name != "fake" || described.Version != "1" {
		t.Errorf("describe = %+v", described)
	}

	var echo struct {
		Echo string `json:"echo"`
	}
	if err := r.Call("anything", nil, &echo); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if echo.Echo != "anything" {
		t.Errorf("echo = %q", echo.Echo)
	}
}

func TestExtensionErrorIsReturned(t *testing.T) {
	r := startFake(t)
	err := r.Call("fail", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no thanks") {
		t.Fatalf("Call(fail) = %v, want the extension's message", err)
	}
}

func TestTimeoutKillsTheExtension(t *testing.T) {
	// This one wants the timeout to arrive, so it sets its own budget rather
	// than the generous one the other tests use while the machine is loaded.
	dir := writeExtension(t, t.TempDir(), "slow", `{"name":"slow","version":"1","protocol":1,"entrypoint":["`+fakePath+`"]}`)
	ext, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r, err := Start(ext, StartOptions{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	err = r.Call("slow", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Call(slow) = %v, want a timeout", err)
	}
	if err := r.Call("anything", nil, nil); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Errorf("call after a timeout = %v, want the runner marked dead", err)
	}
}

func TestCrashIsReported(t *testing.T) {
	r := startFake(t)
	if err := r.Call("crash", nil, nil); err == nil {
		t.Fatal("Call(crash) succeeded, want an error")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	r := startFake(t)
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// Open routes by the manifest's runtime: a js extension runs in this process,
// an exec extension as a process — and the caller holds either as a Hook.
func TestOpenDispatchesByRuntime(t *testing.T) {

	// js: dispatched through Open, the extension receives on_event.
	jsDir := writeExtension(t, t.TempDir(), "noting", `{"id":"test.noting","name":"noting","version":"1.0.0","apiVersion":1,"runtime":"js","entry":"main.js","implements":{"on_event":1}}`)
	if err := os.WriteFile(filepath.Join(jsDir, "main.js"), []byte("export function on_event(event) { return { got: event.kind + \":\" + event.id }; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(jsDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	jsRunner, err := Open(loaded, nil)
	if err != nil {
		t.Fatalf("Open(js): %v", err)
	}
	defer jsRunner.Close()
	if _, isJS := jsRunner.(*JSRunner); !isJS {
		t.Fatalf("a js manifest came back as %T", jsRunner)
	}
	var noted struct {
		Got string `json:"got"`
	}
	if err := jsRunner.Call("on_event", map[string]any{"kind": "card_graded", "id": "card_7"}, &noted); err != nil {
		t.Fatalf("on_event: %v", err)
	}
	if noted.Got != "card_graded:card_7" {
		t.Errorf("on_event answered %+v", noted)
	}

	// exec: the process path is unchanged — the fake still answers.
	execDir := writeExtension(t, t.TempDir(), "fake", `{"name":"fake","version":"1","protocol":1,"entrypoint":["`+fakePath+`"]}`)
	loaded, err = Load(execDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	execRunner, err := Open(loaded, nil)
	if err != nil {
		t.Fatalf("Open(exec): %v", err)
	}
	defer execRunner.Close()
	if _, isExec := execRunner.(*Runner); !isExec {
		t.Fatalf("an exec manifest came back as %T", execRunner)
	}
	var echoed struct {
		Echo string `json:"echo"`
	}
	if err := execRunner.Call("on_event", nil, &echoed); err != nil {
		t.Fatalf("on_event over the pipe: %v", err)
	}
	if echoed.Echo != "on_event" {
		t.Errorf("the pipe answered %q", echoed.Echo)
	}
}
