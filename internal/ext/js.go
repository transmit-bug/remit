package ext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	DefaultJSMemory  = 64 << 20 // ADR 0006: 64 MiB per runtime.
)

// ErrTimeout is a call that outlived its budget and was interrupted.
var ErrTimeout = errors.New("ext: the extension used its time")

// ErrMemoryLimit is a call that asked for more memory than the app offers it.
var ErrMemoryLimit = errors.New("ext: the extension used its memory")

// ErrNoHook is a call for a hook the extension does not carry.
var ErrNoHook = errors.New("ext: the extension has no such hook")

// ErrDisabled is a call to an extension that failed three times in a row.
// Re-enabling is a fresh Open; that is the caller's call to make.
var ErrDisabled = errors.New("ext: the extension is disabled")

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

// JSRunner is an in-process extension. Its source is compiled once and loaded
// into a pool of runtimes that live for the runner's life, not one per call.
// That is a deliberate trade from ADR 0006: the old one-runtime-per-call
// design threw away everything a call left behind, while a pooled runtime
// keeps it — a module-level global one hook set is still there for the next
// call. What still does not cross a call is the memory budget (per runtime,
// pooled included) and the time budget (per call); runaway state is bounded
// by the pool's size and cleared only by a fresh Open.
type JSRunner struct {
	manifest Manifest
	module   *moejs.Module
	limits   JSLimits
	caps     map[string]bool
	// host is what the extension may reach through its capabilities; nil when
	// it was started without one.
	host Host

	// pool holds idle runtimes that have loaded the module. A channel is the
	// whole concurrency story: take is a receive, return is a send, a full
	// pool drops the runtime instead of blocking.
	pool     chan *moejs.Runtime
	mu       sync.Mutex
	warmed   bool
	made     atomic.Int64 // runtimes made, for the pool's own gauge
	failures int
	disabled bool
}

// runtimePoolSize is the pool depth ADR 0006 rules on: two per extension.
const runtimePoolSize = 2

// OpenJS loads the JavaScript extension in dir: the same manifest, the same
// capability rules and the same limits as any other, built in or not. host is
// what the extension may reach; nil is a runner that reaches nothing.
func OpenJS(dir string, host Host, limits JSLimits) (*JSRunner, error) {
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
	return compileJS(ext.Manifest, source, host, limits)
}

// OpenBuiltin compiles an extension the app ships. It is the same loader as
// OpenJS — the same manifest rules, the same capability set, the same limits —
// with the bytes read from the binary rather than from a directory, so that a
// built-in extension has no door a downloaded one does not.
func OpenBuiltin(manifest Manifest, source []byte, host Host, limits JSLimits) (*JSRunner, error) {
	if err := Validate(manifest); err != nil {
		return nil, fmt.Errorf("ext: builtin %s: %w", manifest.Name, err)
	}
	if Runtime(manifest.Runtime) != RuntimeJS {
		return nil, fmt.Errorf("ext: builtin %s: runtime %q, not js", manifest.Name, manifest.Runtime)
	}
	return compileJS(manifest, source, host, limits)
}

// compileJS is the one place a JavaScript extension is compiled and its
// capabilities read.
func compileJS(manifest Manifest, source []byte, host Host, limits JSLimits) (*JSRunner, error) {
	module, err := moejs.Compile(manifest.Entry, string(source))
	if err != nil {
		return nil, fmt.Errorf("ext: %s: compile: %w", manifest.Name, err)
	}
	caps := make(map[string]bool, len(manifest.Capabilities))
	for _, capability := range manifest.Capabilities {
		caps[capability] = true
	}
	return &JSRunner{
		manifest: manifest,
		module:   module,
		limits:   limits.withDefaults(),
		caps:     caps,
		host:     host,
		pool:     make(chan *moejs.Runtime, runtimePoolSize),
	}, nil
}

// Name is the extension's name.
func (r *JSRunner) Name() string { return r.manifest.Name }

// Has reports whether the extension declared a capability.
func (r *JSRunner) Has(capability string) bool { return r.caps[capability] }

// Runtimes is how many runtimes the runner has made. It is the pool's own
// gauge: a warmed pool serves calls without making another.
func (r *JSRunner) Runtimes() int {
	return int(r.made.Load())
}

// Identity is the extension's manifest id, the stable name a derived card's
// producer stamp carries.
func (r *JSRunner) Identity() string { return r.manifest.ID }

// Revision is the extension's own version string, the other half of a
// producer stamp.
func (r *JSRunner) Revision() string { return r.manifest.Version }

// Warm fills the pool at activation, so the first call pays no load cost and a
// syntax error surfaces while enabling rather than mid-review (ADR 0006).
// Warming twice is a no-op. A runner that was never warmed still works: Call
// loads lazily.
func (r *JSRunner) Warm() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.warmed {
		return nil
	}
	r.warmed = true
	for range runtimePoolSize {
		rt, err := r.newRuntime()
		if err != nil {
			return err
		}
		r.pool <- rt
	}
	return nil
}

// newRuntime makes a runtime with the host's globals and the module loaded.
func (r *JSRunner) newRuntime() (*moejs.Runtime, error) {
	rt := moejs.NewRuntime(moejs.Options{
		MemoryLimit:        r.limits.Memory,
		DisableDynamicCode: true,
	})
	r.made.Add(1)
	for name, value := range r.globals(rt) {
		if err := rt.SetGlobal(name, value); err != nil {
			return nil, fmt.Errorf("ext: %s: global %s: %w", r.manifest.Name, name, err)
		}
	}
	if err := rt.Load(r.module); err != nil {
		return nil, fmt.Errorf("ext: %s: load: %w", r.manifest.Name, err)
	}
	return rt, nil
}

// acquire takes an idle runtime from the pool, or makes one when the pool is
// empty: a caller beyond the pool's depth runs on a runtime of its own.
func (r *JSRunner) acquire() (*moejs.Runtime, error) {
	select {
	case rt := <-r.pool:
		return rt, nil
	default:
		return r.newRuntime()
	}
}

// put resets a runtime and returns it to the pool. A pool that is full — more
// callers than depth — drops it instead of blocking.
func (r *JSRunner) put(rt *moejs.Runtime) {
	rt.ClearInterrupt()
	rt.ReleaseCallData() // the idle runtime lets go of this call's arguments
	select {
	case r.pool <- rt:
	default:
	}
}

// Failures is how many calls in a row have failed, for the UI and the log.
func (r *JSRunner) Failures() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.failures
}

// Disabled reports whether the extension was stopped by three consecutive
// failures. Starting it again is a fresh Open — the caller's decision, not the
// runner's.
func (r *JSRunner) Disabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.disabled
}

// Close drops the pooled runtimes. There is nothing else to release.
func (r *JSRunner) Close() error {
	for {
		select {
		case <-r.pool:
		default:
			return nil
		}
	}
}

// fail records one failed call and disables the runner at three in a row.
func (r *JSRunner) fail() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures++
	if r.failures >= 3 {
		r.disabled = true
	}
}

// succeeded clears the streak: three strikes only count when consecutive.
func (r *JSRunner) succeeded() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures = 0
}

// Call runs one hook with JSON-shaped parameters and decodes its result into
// out. A hook that loops, allocates without end or throws is reported, not
// propagated: the review flow goes on either way. Three failures in a row
// disable the runner; a disabled runner answers ErrDisabled without touching
// a runtime.
func (r *JSRunner) Call(method string, params, out any) error {
	if r.Disabled() {
		return fmt.Errorf("%w: %s", ErrDisabled, r.manifest.Name)
	}
	hook, err := r.module.Hook(method)
	if err != nil {
		return fmt.Errorf("%w: %s.%s: %v", ErrNoHook, r.manifest.Name, method, err)
	}

	rt, err := r.acquire()
	if err != nil {
		return err
	}
	defer r.put(rt)

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
			r.fail()
			return ErrTimeout
		}
	}
	if res.err != nil {
		err := r.classify(res.err)
		r.fail()
		return err
	}
	r.succeeded()
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

// arguments turns the call's parameters into the values a hook takes. A
// []any is spread: each element is one argument, so a hook that takes
// (block, types) is called with two. Anything else is the hook's one
// argument.
func (r *JSRunner) arguments(rt *moejs.Runtime, params any) ([]moejs.Value, error) {
	if params == nil {
		return nil, nil
	}
	spread, multi := params.([]any)
	if !multi {
		spread = []any{params}
	}
	out := make([]moejs.Value, 0, len(spread))
	for _, param := range spread {
		encoded, err := json.Marshal(param)
		if err != nil {
			return nil, fmt.Errorf("ext: %s: parameters: %w", r.manifest.Name, err)
		}
		value, err := rt.ParseJSON(encoded)
		if err != nil {
			return nil, fmt.Errorf("ext: %s: parameters: %w", r.manifest.Name, err)
		}
		out = append(out, value)
	}
	return out, nil
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
