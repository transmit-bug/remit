// Package plugin is the plugin supervisor: the one owner of a plugin's life.
// Discovery, the enabled list, role resolution, one runner per manifest name,
// warm-up, the library a capability call reaches and close all live here, so
// the window asks for what it draws instead of deciding how a plugin starts,
// and every entry point asks the same owner (docs/specs/plugin-system.md, P1).
//
// It depends on internal/ext and internal/core, never on the store: a library
// hands it the names it enabled and the roles it chose, as data, so the
// supervisor needs no library, no index and no directory to be tested — a fake
// runner and a fake host are enough.
package plugin

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// Role is the runner that plays one of a library's roles: callable, and warm —
// a role's plugin pays the load cost at activation so the first review does not
// (ADR 0006). A role takes a js plugin only, so a Role is always one of those.
type Role interface {
	ext.Hook
	Warm() error
}

// Supervisor owns the plugins of one process. The window holds it, cmd holds
// it, and the library hands it names: nothing else starts a plugin.
type Supervisor struct {
	dir string

	// Construct turns a manifest name into a running plugin, and is nil for the
	// app's own: the two this build ships come from the binary, every other from
	// the plugin directory. A test hangs a fake runner here to drive the
	// supervisor without the engine — the one seam of the lift from cmd.
	Construct func(name string) (ext.Hook, error)

	// ProcessTimeout is how long one call to a process plugin may take; zero is
	// the app's, ext.Timeout. Set it before the first bind: a test raises it,
	// because its fake still has to be scheduled on a machine running the whole
	// suite.
	ProcessTimeout time.Duration

	mu sync.Mutex
	// open is the runner each manifest name resolves to, however it was asked
	// for: a role, an enabled plugin's events, or a contributed command. One
	// name, one runner, for the process's life, so a plugin's own store has one
	// writer.
	open map[string]ext.Hook
	// roles is the name playing each role, "" when nothing does, and enabled is
	// the order the bound library enables. Together they are what a bind keeps
	// alive. Guarded by mu.
	roles   map[string]string
	enabled []string
	closed  bool
	// broken is why each name that will not start did not, and fallbacks are the
	// roles a plugin lost to the one playing them instead: what Health answers for
	// a name that has no runner to ask. Guarded by mu.
	broken    map[string]string
	fallbacks map[string][]Fallback
	// host is the library a capability call reaches now, as a provider: a runner
	// starts before a library is open — a role's must, its policy is what the
	// library opens with — and asks on every call, so binding a library hands the
	// runner nothing to restart and the same runner answers from then on. An
	// atomic pointer rather than mu because a plugin may reach its host while a
	// lock here is held, loading a module that calls a capability at its top
	// level, and a lock there would deadlock with itself.
	host atomic.Pointer[ext.Host]
}

// New serves the plugins installed under dir, and the two this build ships.
func New(dir string) *Supervisor {
	return &Supervisor{
		dir:       dir,
		open:      map[string]ext.Hook{},
		roles:     map[string]string{},
		broken:    map[string]string{},
		fallbacks: map[string][]Fallback{},
	}
}

// Resolve resolves the plugin a library names for a role and warms it. It is
// called before the library opens, once per role, because what schedules has to
// be in place before the index replays (ADR 0007, store's layout).
//
// A role takes a js plugin only, and that rule is checked here, once, before
// anything starts (see jsOnly). fallback is what plays the role when the name
// cannot: the plugin this build ships for it. A role that can start nothing
// answers nil, and the caller runs without that plugin rather than not at all.
func (s *Supervisor) Resolve(role, name, fallback string) Role {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	hook, err := s.roleLocked(name)
	if err == nil {
		// A plugin three failures stopped plays nothing: its role takes the
		// built-in, the same road a plugin that will not start takes
		// (docs/specs/plugin-system.md P1: 停用之后回退内置 runner).
		err = s.disabledLocked(name)
	}
	if err != nil {
		var (
			notJS   roleRuntimeError
			stopped disabledError
		)
		switch {
		case errors.As(err, &notJS):
			slog.Warn("remit: a role takes a js plugin only",
				"role", role, "plugin", name, "fallback", fallback, "err", err)
		case errors.As(err, &stopped):
			slog.Warn("remit: a role's plugin is disabled, so the role falls back",
				"role", role, "plugin", name, "fallback", fallback, "err", err)
		default:
			// Missing, broken or not installed: the role still falls back, but the
			// reason is not the substrate rule and must not be reported as one.
			slog.Warn("remit: a role's plugin did not start",
				"role", role, "plugin", name, "fallback", fallback, "err", err)
		}
		if hook, err = s.roleLocked(fallback); err != nil {
			slog.Warn("remit: no plugin plays the role", "role", role, "plugin", fallback, "err", err)
			s.roles[role] = ""
			return nil
		}
		s.fellBackLocked(name, role, fallback)
	} else {
		s.playedLocked(name, role)
	}
	runner, ok := hook.(Role)
	if !ok {
		// Unreachable through roleLocked, which refuses anything that cannot
		// warm; kept so a Construct seam answering the wrong kind fails loudly
		// rather than in the window.
		s.roles[role] = ""
		return nil
	}
	s.roles[role] = hook.Name()
	if err := runner.Warm(); err != nil {
		slog.Warn("remit: a role's plugin did not warm", "role", role, "plugin", hook.Name(), "err", err)
	}
	return runner
}

// CardResolver answers what a library derives through: a resolver over the
// plugin playing the card role, with the library's own card types. Every path
// that makes a card takes its resolver from here — the window, the loopback
// agent API and a plugin's remit.cards.create — so which plugin derives is
// answered once and each card carries that plugin's stamp (docs/specs/
// plugin-system.md P1: derivation has one road).
//
// A card role that resolved to nothing answers the same resolver with no
// plugin: internal/cards then makes no card and a card reads its own
// paragraph's words. That is the one fallback, and it lives there — a second
// one here would be the copy that drifts.
func (s *Supervisor) CardResolver(types []core.Type) *cards.Resolver {
	s.mu.Lock()
	runner := s.cardRunnerLocked()
	s.mu.Unlock()
	if runner == nil {
		return cards.New(nil, core.Producer{}, types)
	}
	// Built outside the lock: the resolver compiles the library's templates, and
	// no other caller waits on that. What leaves the supervisor is the capability
	// and the stamp its cards carry, never the runner.
	return cards.New(ext.HooksOf(runner), ext.ProducerOf(runner), types)
}

// cardRunnerLocked is the runner the card role answers, nil when nothing plays
// it. Resolve warmed it, so a resolver built here never pays the load cost on
// the first review; a role runs on js only (see jsOnly), which is why it is the
// engine's own runner.
func (s *Supervisor) cardRunnerLocked() *ext.JSRunner {
	runner, _ := s.open[s.roles[core.RoleCardsDerive]].(*ext.JSRunner)
	return runner
}

// Bind makes host the library a capability call reaches and enabled the names
// that library approved. It is called after the library opens, on the same path
// as the open itself, and is repeatable: a switch swaps the provider and
// reconciles the runner set by name.
//
// A runner whose name is still wanted — enabled, or playing a role — is kept
// without a restart, so switching libraries never costs a plugin its module
// state nor gives its store a second writer. One that is no longer wanted is
// closed and dropped.
func (s *Supervisor) Bind(host ext.Host, enabled []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.host.Store(&host)
	s.enabled = append([]string(nil), enabled...)
	wanted := make(map[string]bool, len(enabled)+len(s.roles))
	for _, name := range enabled {
		wanted[name] = true
	}
	for _, name := range s.roles {
		// A role's runner outlives its name leaving the enabled list: the role
		// still answers with it, so closing it would leave the library without
		// what it chose.
		if name != "" {
			wanted[name] = true
		}
	}
	for name := range wanted {
		if _, ok := s.open[name]; ok {
			continue
		}
		if _, err := s.startLocked(name); err != nil {
			slog.Warn("remit: an enabled plugin did not start", "name", name, "err", err)
		}
	}
	for name, hook := range s.open {
		if wanted[name] {
			continue
		}
		if err := hook.Close(); err != nil {
			slog.Warn("remit: a plugin did not close", "name", name, "err", err)
		}
		delete(s.open, name)
	}
}

// Runner answers the runner a manifest name names, starting it on first use on
// the substrate its manifest names: what a contributed command or a page's data
// reaches. A name that is already open answers the same runner every time.
func (s *Supervisor) Runner(name string) (ext.Hook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errClosed
	}
	if hook, ok := s.open[name]; ok {
		return hook, nil
	}
	return s.startLocked(name)
}

// Enabled answers the runners the bound library enables, in its order: what its
// events go to. A name that did not start is not here; Bind logged why.
func (s *Supervisor) Enabled() []ext.Hook {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ext.Hook, 0, len(s.enabled))
	for _, name := range s.enabled {
		if hook, ok := s.open[name]; ok {
			out = append(out, hook)
		}
	}
	return out
}

// Close closes every runner the supervisor holds. It is the end of the process,
// not of a library: a library switch keeps its runners.
func (s *Supervisor) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var problems []error
	for name, hook := range s.open {
		if err := hook.Close(); err != nil {
			problems = append(problems, err)
		}
		delete(s.open, name)
	}
	s.host.Store(nil)
	return errors.Join(problems...)
}

// roleLocked answers the runner a role's name gets, or why it gets none.
func (s *Supervisor) roleLocked(name string) (ext.Hook, error) {
	if hook, ok := s.open[name]; ok {
		if _, ok := hook.(Role); !ok {
			return nil, errRoleRuntime(name)
		}
		return hook, nil
	}
	if err := s.jsOnly(name); err != nil {
		return nil, err
	}
	return s.startLocked(name)
}

// jsOnly is the roles' rule, in one place and before anything starts: a role
// runs on js only, because every hook a role carries is a js one (ADR 0006, no
// exec plugin can answer them). The app's own are js and come from the binary;
// anything else must be installed and must say "js" in its manifest.
func (s *Supervisor) jsOnly(name string) error {
	if builtin.IsBuiltin(name) {
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

// roleRuntimeError is a role naming a plugin no role can run: the two runtimes
// are not interchangeable, and every hook a role carries is a js one (ADR 0006).
// It is a type so the refusal is reported as the substrate rule it is, rather
// than as "a role's plugin did not start".
type roleRuntimeError struct{ name string }

func (e roleRuntimeError) Error() string {
	return fmt.Sprintf("remit: %q is not a js plugin, and no role runs on another runtime", e.name)
}

func errRoleRuntime(name string) error { return roleRuntimeError{name: name} }

// errClosed is a supervisor asked for a plugin after the process closed it.
var errClosed = errors.New("remit: the plugin supervisor is closed")

// startLocked builds one plugin, gives a js runner the host provider, and holds
// it under its name. The provider is the supervisor's, not a snapshot: a role's
// runner is started before a library is and answers from whichever one is open
// when a capability is called.
func (s *Supervisor) startLocked(name string) (ext.Hook, error) {
	hook, err := s.build(name)
	if err != nil {
		// What will not start is remembered: Health reports it and Reenable is the
		// way back (docs/specs/plugin-system.md P1).
		s.broken[name] = err.Error()
		return nil, err
	}
	delete(s.broken, name)
	if runner, ok := hook.(*ext.JSRunner); ok {
		runner.UseHost(s.currentHost)
	}
	s.open[name] = hook
	return hook, nil
}

// build constructs a plugin through Construct, or through the app's own switch.
func (s *Supervisor) build(name string) (ext.Hook, error) {
	if s.Construct != nil {
		return s.Construct(name)
	}
	return s.Build(name)
}

// Build is the app's own construction, which Construct replaces: the two plugins
// this build ships come from the binary, every other from the plugin directory on
// the substrate its manifest names. It is the one place a built-in is constructed
// — neither the window nor the settings page makes one of its own — and it is
// exported so a test that replaces one plugin can delegate the rest here.
func (s *Supervisor) Build(name string) (ext.Hook, error) {
	switch name {
	case builtin.CardsName:
		return builtin.Cards(nil, ext.JSLimits{})
	case builtin.SchedulerName:
		return builtin.Scheduler(nil, ext.JSLimits{})
	}
	e, ok := s.find(name)
	if !ok {
		return nil, fmt.Errorf("remit: no plugin named %q", name)
	}
	return ext.Open(e, nil, ext.StartOptions{Timeout: s.ProcessTimeout})
}

// currentHost is the library a capability call acts on now; nil when no library
// is open yet. It is the provider every js runner is given.
func (s *Supervisor) currentHost() ext.Host {
	if host := s.host.Load(); host != nil {
		return *host
	}
	return nil
}
