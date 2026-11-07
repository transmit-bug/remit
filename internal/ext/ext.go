// Package ext runs Remit's extensions: separate programs the user enables per
// library. An extension speaks newline-delimited JSON on stdin and stdout, and
// is trusted as a local process — the app does not sandbox it, and the protocol
// cannot change the app's state except through what the app asks it to do
// (decision #9).
package ext

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Protocol is the version of the wire protocol this package speaks.
const Protocol = 1

// Timeout is how long one call may take before the extension is considered
// hung. A hung extension is killed rather than retried: it would block the
// review flow.
const Timeout = time.Second

// Open starts the extension on the substrate its manifest names: a js
// extension runs in this process under the default limits, anything else runs
// as a process speaking the protocol. The decision lives here, where the
// manifest is read, so the app above asks once and gets a runner either way.
// host is what the extension may reach; nil is a runner that reaches nothing.
func Open(e Extension, host Host) (Hook, error) {
	if Runtime(e.Manifest.Runtime) == RuntimeJS {
		return OpenJS(e.Dir, host, JSLimits{})
	}
	return Start(e)
}

// Extension is a discovered extension: its directory and its manifest.
type Extension struct {
	Dir      string
	Manifest Manifest
}

// Discover reads the extensions under root, one directory each. A directory
// without a readable manifest is reported and skipped: a broken extension must
// not take the others down.
func Discover(root string) ([]Extension, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ext: read %s: %w", root, err)
	}
	var out []Extension
	var problems []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		ext, err := Load(dir)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		out = append(out, ext)
	}
	return out, errors.Join(problems...)
}

// Load reads one extension's manifest.
func Load(dir string) (Extension, error) {
	data, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		return Extension{}, fmt.Errorf("ext: %s: %w", dir, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Extension{}, fmt.Errorf("ext: %s: manifest: %w", dir, err)
	}
	if err := Validate(m); err != nil {
		return Extension{}, fmt.Errorf("ext: %s: %w", dir, err)
	}
	return Extension{Dir: dir, Manifest: m}, nil
}

// StartOptions configure a started extension. The zero value is the app's own:
// a call may take Timeout and no longer.
type StartOptions struct {
	Timeout time.Duration
}

// Runner is a started extension. Calls are serialized: the protocol is one
// request and one response at a time.
type Runner struct {
	ext     Extension
	cmd     *exec.Cmd
	in      *json.Encoder
	out     *bufio.Reader
	mu      sync.Mutex
	seq     int64
	dead    bool
	timeout time.Duration
}

// Start runs the extension.
func Start(ext Extension, opts ...StartOptions) (*Runner, error) {
	options := StartOptions{Timeout: Timeout}
	if len(opts) > 0 {
		options = opts[0]
		if options.Timeout <= 0 {
			options.Timeout = Timeout
		}
	}
	if len(ext.Manifest.Entrypoint) == 0 {
		return nil, fmt.Errorf("ext: %s has no entrypoint", ext.Manifest.Name)
	}
	cmd := exec.Command(ext.Manifest.Entrypoint[0], ext.Manifest.Entrypoint[1:]...)
	cmd.Dir = ext.Dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("ext: %s: stdin: %w", ext.Manifest.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ext: %s: stdout: %w", ext.Manifest.Name, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ext: start %s: %w", ext.Manifest.Name, err)
	}
	return &Runner{
		ext:     ext,
		cmd:     cmd,
		in:      json.NewEncoder(stdin),
		out:     bufio.NewReader(stdout),
		timeout: options.Timeout,
	}, nil
}

// Name is the extension's manifest name.
func (r *Runner) Name() string { return r.ext.Manifest.Name }

type request struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type response struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Call sends one request and decodes its result into out, or returns the
// extension's error. A call that takes longer than Timeout kills the process
// and marks the runner dead: a hung extension is a broken extension.
func (r *Runner) Call(method string, params, out any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dead {
		return fmt.Errorf("ext: %s is not running", r.Name())
	}
	r.seq++
	req := request{ID: r.seq, Method: method, Params: params}
	if err := r.in.Encode(req); err != nil {
		r.kill()
		return fmt.Errorf("ext: %s: send %s: %w", r.Name(), method, err)
	}

	type readResult struct {
		line []byte
		err  error
	}
	done := make(chan readResult, 1)
	go func() {
		line, err := r.out.ReadBytes('\n')
		done <- readResult{line: line, err: err}
	}()

	select {
	case got := <-done:
		if got.err != nil {
			r.kill()
			return fmt.Errorf("ext: %s: read %s: %w", r.Name(), method, got.err)
		}
		var resp response
		if err := json.Unmarshal(got.line, &resp); err != nil {
			return fmt.Errorf("ext: %s: malformed reply to %s: %w", r.Name(), method, err)
		}
		if resp.ID != req.ID {
			return fmt.Errorf("ext: %s: reply to %s came back as id %d", r.Name(), method, resp.ID)
		}
		if resp.Error != "" {
			return fmt.Errorf("ext: %s: %s: %s", r.Name(), method, resp.Error)
		}
		if out == nil || len(resp.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("ext: %s: decode %s result: %w", r.Name(), method, err)
		}
		return nil
	case <-time.After(r.timeout):
		r.kill()
		return fmt.Errorf("ext: %s: %s timed out after %s", r.Name(), method, r.timeout)
	}
}

// Describe asks the extension what it is, in its own words.
func (r *Runner) Describe() (json.RawMessage, error) {
	var out json.RawMessage
	err := r.Call("describe", nil, &out)
	return out, err
}

// Close stops the extension.
func (r *Runner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dead || r.cmd.Process == nil {
		return nil
	}
	r.dead = true
	_ = r.cmd.Process.Signal(os.Interrupt)
	// The process exiting because we asked is not a failure.
	_ = r.cmd.Wait()
	return nil
}

func (r *Runner) kill() {
	if r.dead {
		return
	}
	r.dead = true
	if r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
		_ = r.cmd.Wait()
	}
}
