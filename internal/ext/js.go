package ext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/MoeForge/moejs"
)

// Runtime names the substrate an extension runs on. A manifest that names none
// runs as a process, which is what extensions were before there was a second
// substrate.
type Runtime string

const (
	// RuntimeExec is a separate program speaking the protocol on its pipes.
	RuntimeExec Runtime = "exec"
	// RuntimeJS is JavaScript run in this process, in a runtime of its own.
	RuntimeJS Runtime = "js"
)

// The limits one call to an in-process extension runs under. They are the
// app's, not the extension's: an extension cannot widen them.
const (
	DefaultJSTimeout = time.Second
	DefaultJSMemory  = 32 << 20
)

// ErrTimeout is a call that outlived its budget and was interrupted.
var ErrTimeout = errors.New("ext: the extension used its time")

// ErrMemoryLimit is a call that asked for more memory than the app offers it.
var ErrMemoryLimit = errors.New("ext: the extension used its memory")

// ErrNoHook is a call for a hook the extension does not carry.
var ErrNoHook = errors.New("ext: the extension has no such hook")

// JSLimits bound one call to an in-process extension.
type JSLimits struct {
	Timeout time.Duration
	Memory  int64
}

// withDefaults fills what the caller left zero.
func (l JSLimits) withDefaults() JSLimits {
	if l.Timeout <= 0 {
		l.Timeout = DefaultJSTimeout
	}
	if l.Memory <= 0 {
		l.Memory = DefaultJSMemory
	}
	return l
}

// Hook is what a started extension can do, whichever substrate it runs on: the
// same call shape in process and over a pipe, so that a built-in extension gets
// no path a downloaded one does not have.
type Hook interface {
	Name() string
	Call(method string, params, out any) error
	Close() error
}

// JSRunner is an in-process extension. Its source is compiled once and run in a
// fresh runtime for every call: nothing a call leaves behind — a global, a runaway
// allocation — reaches the next one.
type JSRunner struct {
	dir      string
	manifest Manifest
	module   *moejs.Module
	limits   JSLimits
	caps     map[string]bool
}

// OpenJS loads the JavaScript extension in dir: the same manifest, the same
// capability rules and the same limits as any other, built in or not.
func OpenJS(dir string, limits JSLimits) (*JSRunner, error) {
	ext, err := Load(dir)
	if err != nil {
		return nil, err
	}
	if Runtime(ext.Manifest.Runtime) != RuntimeJS {
		return nil, fmt.Errorf("ext: %s: runtime %q, not js", dir, ext.Manifest.Runtime)
	}
	entry := filepath.Join(dir, ext.Manifest.Entry)
	source, err := os.ReadFile(entry)
	if err != nil {
		return nil, fmt.Errorf("ext: %s: %w", entry, err)
	}
	return compileJS(ext.Manifest, source, limits)
}

// OpenBuiltin compiles an extension the app ships. It is the same loader as
// OpenJS — the same manifest rules, the same capability set, the same limits —
// with the bytes read from the binary rather than from a directory, so that a
// built-in extension has no door a downloaded one does not.
func OpenBuiltin(manifest Manifest, source []byte, limits JSLimits) (*JSRunner, error) {
	if err := Validate(manifest); err != nil {
		return nil, fmt.Errorf("ext: builtin %s: %w", manifest.Name, err)
	}
	if Runtime(manifest.Runtime) != RuntimeJS {
		return nil, fmt.Errorf("ext: builtin %s: runtime %q, not js", manifest.Name, manifest.Runtime)
	}
	return compileJS(manifest, source, limits)
}

// compileJS is the one place a JavaScript extension is compiled and its
// capabilities read.
func compileJS(manifest Manifest, source []byte, limits JSLimits) (*JSRunner, error) {
	module, err := moejs.Compile(manifest.Entry, string(source))
	if err != nil {
		return nil, fmt.Errorf("ext: %s: compile: %w", manifest.Name, err)
	}
	caps := make(map[string]bool, len(manifest.Capabilities))
	for _, capability := range manifest.Capabilities {
		caps[capability] = true
	}
	return &JSRunner{manifest: manifest, module: module, limits: limits.withDefaults(), caps: caps}, nil
}

// Name is the extension's name.
func (r *JSRunner) Name() string { return r.manifest.Name }

// Has reports whether the extension declared a capability.
func (r *JSRunner) Has(capability string) bool { return r.caps[capability] }

// Close has nothing to release: a runtime is made and thrown away per call.
func (r *JSRunner) Close() error { return nil }

// Call runs one hook with JSON-shaped parameters and decodes its result into
// out. A hook that loops, allocates without end or throws is reported, not
// propagated: the review flow goes on either way.
func (r *JSRunner) Call(method string, params, out any) error {
	hook, err := r.module.Hook(method)
	if err != nil {
		return fmt.Errorf("%w: %s.%s: %v", ErrNoHook, r.manifest.Name, method, err)
	}

	rt := moejs.NewRuntime(moejs.Options{
		MemoryLimit:        r.limits.Memory,
		DisableDynamicCode: true,
	})
	for name, value := range r.globals(rt) {
		if err := rt.SetGlobal(name, value); err != nil {
			return fmt.Errorf("ext: %s: global %s: %w", r.manifest.Name, name, err)
		}
	}
	if err := rt.Load(r.module); err != nil {
		return fmt.Errorf("ext: %s: load: %w", r.manifest.Name, err)
	}

	args, err := r.arguments(rt, params)
	if err != nil {
		return err
	}

	type result struct {
		value moejs.Value
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, err := rt.Call(hook, args...)
		done <- result{value: value, err: err}
	}()

	timer := time.NewTimer(r.limits.Timeout)
	defer timer.Stop()
	var res result
	select {
	case res = <-done:
	case <-timer.C:
		// The hook is still running: interrupt it and wait for it to unwind, so
		// that no goroutine is left running the extension's code.
		rt.Interrupt(ErrTimeout)
		res = <-done
		if res.err == nil {
			return ErrTimeout
		}
	}
	if res.err != nil {
		return r.classify(res.err)
	}
	if out == nil {
		return nil
	}
	encoded, err := rt.AppendJSON(nil, res.value)
	if err != nil {
		return fmt.Errorf("ext: %s: result: %w", r.manifest.Name, err)
	}
	if err := json.Unmarshal(encoded, out); err != nil {
		return fmt.Errorf("ext: %s: result: %w", r.manifest.Name, err)
	}
	return nil
}

// classify names what went wrong in terms the app acts on.
func (r *JSRunner) classify(err error) error {
	var interrupted *moejs.InterruptedError
	if errors.As(err, &interrupted) || errors.Is(err, ErrTimeout) {
		return fmt.Errorf("%w: %s", ErrTimeout, r.manifest.Name)
	}
	var memory *moejs.MemoryLimitError
	if errors.As(err, &memory) || errors.Is(err, moejs.ErrMemoryLimit) {
		return fmt.Errorf("%w: %s", ErrMemoryLimit, r.manifest.Name)
	}
	return fmt.Errorf("ext: %s: %w", r.manifest.Name, err)
}

// arguments turns the call's parameters into the values a hook takes.
func (r *JSRunner) arguments(rt *moejs.Runtime, params any) ([]moejs.Value, error) {
	if params == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("ext: %s: parameters: %w", r.manifest.Name, err)
	}
	value, err := rt.ParseJSON(encoded)
	if err != nil {
		return nil, fmt.Errorf("ext: %s: parameters: %w", r.manifest.Name, err)
	}
	return []moejs.Value{value}, nil
}

// Capabilities the host can offer an extension. An extension gets only the ones
// its manifest declares: there is no other door.
const (
	// CapEcho answers with what it was given, for a plugin's own tests.
	CapEcho = "remit.echo"
	// CapNow tells the extension the current time, as it is the app's to give.
	CapNow = "remit.now"
)

// globals is the host object an extension reaches: one function per declared
// capability, and nothing else.
func (r *JSRunner) globals(rt *moejs.Runtime) map[string]any {
	host := map[string]any{}
	if r.caps[CapEcho] {
		host["echo"] = moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			if len(args) == 0 {
				return rt.FromGo(nil)
			}
			return args[0], nil
		})
	}
	if r.caps[CapNow] {
		host["now"] = moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			value, err := rt.FromGo(time.Now().UTC().Format(time.RFC3339))
			return value, err
		})
	}
	return map[string]any{"remit": host}
}
