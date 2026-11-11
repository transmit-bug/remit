package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// startExtensions subscribes the plugin service's runners to this library's
// events: one per enabled name, asked for by manifest name and not started here.
// A broken or missing extension is reported and skipped: enabling one must never
// keep the app from opening.
//
// Asking the service is what makes a plugin that is both enabled and a role one
// runner: its events and its role hooks reach the same pool, so its own store has
// one writer (docs/specs/plugin-system.md, P1). The runners reach the library
// through the same source the loopback API uses, resolved per capability call, so
// one started before this library opened acts on it too.
//
// The window is told about every event too, so an open plugin page reloads what
// it shows: a page is what the library was when it loaded.
func startExtensions(lib *store.Library, app *appui.App, plugins *pluginService) {
	var runners []ext.Hook
	for _, name := range lib.EnabledExtensions() {
		runner, err := plugins.Enabled(name)
		if err != nil {
			slog.Warn("remit: extension did not start", "name", name, "err", err)
			continue
		}
		runners = append(runners, runner)
	}
	// Events are notifications, not vetoes: a failing extension is logged and
	// the review goes on. The handler is set even with no runner, because the
	// window's contributed pages read the library's events as their refresh.
	lib.SetEventHandler(func(ev store.Event) {
		for _, r := range runners {
			if err := r.Call("on_event", map[string]any{"kind": ev.Kind, "id": string(ev.ID)}, nil); err != nil {
				slog.Warn("remit: extension event failed", "extension", r.Name(), "err", err)
			}
		}
		if app != nil {
			app.LibraryEvent()
		}
	})
}

// pluginService is the plugin directory as the app reaches it: what discovery
// finds for the settings page, and the runner a manifest name resolves to —
// whichever path asks: a role, a contributed page's callback, or an enabled
// plugin's event handler. One name, one runner, for the service's life, so a
// plugin that is both enabled and a role is started once and its own store has
// one writer (docs/specs/plugin-system.md, P1). Reopening a library neither
// starts a plugin twice nor takes the runner a role holds away from it.
//
// The registry is cmd's for now; #265 lifts it into internal/plugin, and
// construct is the seam that lift takes.
type pluginService struct {
	root string
	mu   sync.Mutex
	// open is the runner each manifest name resolves to, whichever path asked
	// first. Guarded by mu.
	open map[string]ext.Hook
	// construct turns a manifest name into a running plugin: the app's own from
	// the binary, every other from the plugin directory on the substrate its
	// manifest names. It is a field so a test can count starts without an
	// engine, and so a supervisor can own construction without the registry
	// changing shape.
	construct func(name string) (ext.Hook, error)
	// processTimeout is how long one call to a process plugin may take; zero is
	// the app's, ext.Timeout. A test raises it: its fake still has to be
	// scheduled on a machine running the whole suite.
	processTimeout time.Duration
	// host is the library the plugins act on, replaced when one opens. A runner
	// reads it through currentHost on every capability call. It is an atomic
	// pointer rather than s.mu because a plugin may reach its host while Open
	// holds the lock — loading a module that calls a capability at its top
	// level — and a lock there would deadlock with itself.
	host atomic.Pointer[cardSource]
}

// newPluginService serves the plugins installed under root.
func newPluginService(root string) *pluginService {
	s := &pluginService{root: root, open: map[string]ext.Hook{}}
	s.construct = s.newHook
	return s
}

// Discover lists the plugins this build can run: the two it ships, then what the
// user installed. A plugin that will not load is reported and left out, so the
// page lists what works.
func (s *pluginService) Discover() ([]appui.Plugin, error) {
	out := make([]appui.Plugin, 0, 4)
	var problems []error
	for _, manifest := range []func() (ext.Manifest, error){builtin.CardsManifest, builtin.SchedulerManifest} {
		m, err := manifest()
		if err != nil {
			problems = append(problems, err)
			continue
		}
		out = append(out, pluginOf(ext.Extension{Manifest: m}, true))
	}
	found, err := ext.Discover(s.root)
	for _, e := range found {
		out = append(out, pluginOf(e, false))
	}
	problems = append(problems, err)
	return out, errors.Join(problems...)
}

// Manifests lists what the plugin directory holds, with what each manifest
// declares, without starting any of it: the window parses the enabled plugins'
// pages, commands, menus and settings from these, so a plugin it does not run
// still costs nothing.
func (s *pluginService) Manifests() ([]ext.Extension, error) {
	out := make([]ext.Extension, 0, 4)
	var problems []error
	for _, manifest := range []func() (ext.Manifest, error){builtin.CardsManifest, builtin.SchedulerManifest} {
		m, err := manifest()
		if err != nil {
			problems = append(problems, err)
			continue
		}
		out = append(out, ext.Extension{Manifest: m})
	}
	found, err := ext.Discover(s.root)
	out = append(out, found...)
	problems = append(problems, err)
	return out, errors.Join(problems...)
}

// Open starts the plugin a manifest name names, for the role that chose it. It
// answers the runner the service already holds when an enabled plugin started
// one: one name, one runner, so the role's hooks and the plugin's events reach the
// same pool and the plugin's own store has one writer.
//
// A role runs on js only (ADR 0006), and the refusal comes before anything
// starts: an exec plugin in the enabled list is heard, never chosen for a role.
func (s *pluginService) Open(name string) (*ext.JSRunner, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hook, ok := s.open[name]
	if !ok {
		if err := s.jsOnly(name); err != nil {
			return nil, err
		}
		var err error
		if hook, err = s.start(name); err != nil {
			return nil, err
		}
		s.open[name] = hook
	}
	runner, ok := hook.(*ext.JSRunner)
	if !ok {
		return nil, errRoleRuntime(name)
	}
	if err := runner.Warm(); err != nil {
		slog.Warn("remit: a role's plugin did not warm", "name", name, "err", err)
	}
	return runner, nil
}

// Enabled answers the runner an enabled plugin's events go to, whichever runtime
// its manifest names: the enabled list takes both (ADR 0006), so an exec plugin is
// started here as a process speaking the protocol and hears events exactly as a js
// plugin does. It is not warmed — a plugin the app only listens to pays nothing
// until an event asks it — and it registers under its name like any other, so an
// enabled plugin that is also a role is constructed once.
func (s *pluginService) Enabled(name string) (ext.Hook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if hook, ok := s.open[name]; ok {
		return hook, nil
	}
	hook, err := s.start(name)
	if err != nil {
		return nil, err
	}
	s.open[name] = hook
	return hook, nil
}

// SetLibrary makes lib the library the plugins' capability calls act on. Runners
// already open are kept: each resolves its host through currentHost on every
// call, so a plugin started before a library opened — which a role's must be,
// the scheduling policy has to be in place before the index replays — works once
// it does, and a library switch is seen without a second start.
func (s *pluginService) SetLibrary(lib *store.Library) {
	host, err := newCardSource(lib)
	if err != nil {
		slog.Warn("remit: extensions: the card plugin did not start", "err", err)
	}
	s.host.Store(&host)
}

// currentHost is the library a capability call acts on now; nil when no library
// is open yet. It is the provider every runner is given.
func (s *pluginService) currentHost() ext.Host {
	if host := s.host.Load(); host != nil {
		return *host
	}
	return nil
}

// start builds one plugin through the construct seam and hands a js runner the
// service's host provider: a role's hooks are pure — derive a card, answer a
// policy — and a contributed command or page reads the library, so the library a
// capability call reaches is the one open then, not the one open at construction,
// which for a role's plugin is none. A process plugin holds no host: its protocol
// has no capabilities (ADR 0006).
func (s *pluginService) start(name string) (ext.Hook, error) {
	hook, err := s.construct(name)
	if err != nil {
		return nil, err
	}
	if runner, ok := hook.(*ext.JSRunner); ok {
		runner.UseHost(s.currentHost)
	}
	return hook, nil
}

// jsOnly is the roles' rule, checked before anything starts: a role runs on js
// only, and a plugin whose manifest names the process runtime is refused rather
// than started and then refused.
func (s *pluginService) jsOnly(name string) error {
	// The app's own run on js, and come from the binary rather than the directory.
	if name == store.DefaultCardsPlugin || name == store.DefaultSchedulerPlugin {
		return nil
	}
	e, ok := s.find(name)
	if !ok {
		return fmt.Errorf("remit: no plugin named %q", name)
	}
	if ext.Runtime(e.Manifest.Runtime) != ext.RuntimeJS {
		return errRoleRuntime(name)
	}
	return nil
}

// errRoleRuntime is a role naming a plugin no role can run: the two runtimes are
// not interchangeable, and every hook a role carries is a js one (ADR 0006).
func errRoleRuntime(name string) error {
	return fmt.Errorf("remit: %q is not a js plugin, and no role runs on another runtime", name)
}

// newHook builds the runner a manifest name names on the substrate its manifest
// names: the app's own plugins come from the binary, every other from the plugin
// directory, and ext.Open keeps the js-versus-process decision where the manifest
// is read.
func (s *pluginService) newHook(name string) (ext.Hook, error) {
	switch name {
	case store.DefaultCardsPlugin:
		return builtin.Cards(nil, ext.JSLimits{})
	case store.DefaultSchedulerPlugin:
		return builtin.Scheduler(nil, ext.JSLimits{})
	}
	e, ok := s.find(name)
	if !ok {
		return nil, fmt.Errorf("remit: no plugin named %q", name)
	}
	return ext.Open(e, nil, ext.StartOptions{Timeout: s.processTimeout})
}

// find answers the installed extension a manifest name names, as discovery sees
// it.
func (s *pluginService) find(name string) (ext.Extension, bool) {
	found, err := ext.Discover(s.root)
	if err != nil {
		slog.Warn("remit: plugin discovery", "err", err)
	}
	for _, candidate := range found {
		if candidate.Manifest.Name == name {
			return candidate, true
		}
	}
	return ext.Extension{}, false
}

// runCheckExtension answers the -check-ext flag, the name this command had
// before `remit ext check` existed: it prints what one plugin directory declares
// that this build cannot use, and exits non-zero when it found any. It answers
// whether it ran at all, as the library commands do.
func runCheckExtension(dir string, out io.Writer) (int, bool) {
	if dir == "" {
		return extOK, false
	}
	return extCheck(dir, out), true
}

// pluginOf is one manifest as the settings page lists it.
func pluginOf(e ext.Extension, builtin bool) appui.Plugin {
	roles := make([]string, 0, len(core.Roles()))
	for _, role := range core.Roles() {
		if ext.Plays(e, role) {
			roles = append(roles, role)
		}
	}
	return appui.Plugin{
		Name:         e.Manifest.Name,
		ID:           e.Manifest.ID,
		Version:      e.Manifest.Version,
		Capabilities: append([]string(nil), e.Manifest.Capabilities...),
		Roles:        roles,
		Builtin:      builtin,
	}
}
