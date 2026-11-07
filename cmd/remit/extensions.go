package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// startExtensions runs the extensions this library enabled. A broken or
// missing extension is reported and skipped: enabling one must never keep the
// app from opening.
//
// The runners reach the library through the same source the loopback API uses:
// the store's read/write layer plus card derivation, so a plugin's cards.create
// makes what the app would.
//
// The window is told about every event too, so an open plugin page reloads what
// it shows: a page is what the library was when it loaded.
func startExtensions(lib *store.Library, app *appui.App) {
	enabled := lib.EnabledExtensions()
	host, err := newCardSource(lib)
	if err != nil {
		slog.Warn("remit: extensions: the card plugin did not start", "err", err)
	}
	root, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		slog.Warn("remit: extensions: no data directory", "err", err)
		return
	}
	found, err := ext.Discover(filepath.Join(root, "extensions"))
	if err != nil {
		slog.Warn("remit: extensions: discovery", "err", err)
	}
	byName := map[string]ext.Extension{}
	for _, e := range found {
		byName[e.Manifest.Name] = e
	}

	var runners []ext.Hook
	for _, name := range enabled {
		e, ok := byName[name]
		if !ok {
			slog.Warn("remit: extension enabled but not installed", "name", name)
			continue
		}
		r, err := ext.Open(e, host)
		if err != nil {
			slog.Warn("remit: extension did not start", "name", name, "err", err)
			continue
		}
		runners = append(runners, r)
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
// finds for the settings page, and the runner a role names. Runners are kept by
// manifest name, so reopening a library does not start a plugin twice.
type pluginService struct {
	root string
	mu   sync.Mutex
	open map[string]*ext.JSRunner
	host ext.Host
}

// newPluginService serves the plugins installed under root.
func newPluginService(root string) *pluginService {
	return &pluginService{root: root, open: map[string]*ext.JSRunner{}}
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

// Open starts the plugin a manifest name names, for the role that chose it.
func (s *pluginService) Open(name string) (*ext.JSRunner, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if runner, ok := s.open[name]; ok {
		return runner, nil
	}
	runner, err := s.start(name)
	if err != nil {
		return nil, err
	}
	if err := runner.Warm(); err != nil {
		slog.Warn("remit: a role's plugin did not warm", "name", name, "err", err)
	}
	s.open[name] = runner
	return runner, nil
}

// SetLibrary gives the plugins the library their callbacks act on. Runners
// opened before it are dropped: a plugin started without a host answers every
// capability call with an error, and a page that reads nothing is worse than
// one that asks again.
func (s *pluginService) SetLibrary(lib *store.Library) {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, err := newCardSource(lib)
	if err != nil {
		slog.Warn("remit: extensions: the card plugin did not start", "err", err)
	}
	s.host = host
	s.open = map[string]*ext.JSRunner{}
}

// start opens one plugin by manifest name. A role's hooks are pure — derive a
// card, answer a policy — and a contributed command or page reads the library,
// so every runner gets the host of the library open now.
func (s *pluginService) start(name string) (*ext.JSRunner, error) {
	switch name {
	case store.DefaultCardsPlugin:
		return builtin.Cards(s.host, ext.JSLimits{})
	case store.DefaultSchedulerPlugin:
		return builtin.Scheduler(s.host, ext.JSLimits{})
	}
	found, err := ext.Discover(s.root)
	if err != nil {
		slog.Warn("remit: plugin discovery", "err", err)
	}
	for _, candidate := range found {
		if candidate.Manifest.Name != name {
			continue
		}
		if ext.Runtime(candidate.Manifest.Runtime) != ext.RuntimeJS {
			return nil, fmt.Errorf("remit: %q is not a js plugin, and no role runs on another runtime", name)
		}
		return ext.OpenJS(candidate.Dir, s.host, ext.JSLimits{})
	}
	return nil, fmt.Errorf("remit: no plugin named %q", name)
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
