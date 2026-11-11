package plugin

import (
	"fmt"

	"github.com/transmit-bug/remit/internal/ext"
)

// HealthState is how a plugin stands at the plugin level. It is not the
// contribution level: three failed calls stop the plugin whole, while three
// failures of one contributed command or page hide that one entry
// (docs/specs/plugin-system.md P1: 两层失败不合并).
type HealthState string

const (
	// HealthOK is a plugin that is running, or that nothing has asked anything of
	// yet.
	HealthOK HealthState = "ok"
	// HealthFailing is a streak of one or two failed calls: one more stops it.
	HealthFailing HealthState = "failing"
	// HealthDisabled is a plugin three failed calls in a row stopped. Reenable is
	// the way back.
	HealthDisabled HealthState = "disabled"
	// HealthFailed is a plugin that will not start: a file it cannot read, a
	// module that does not compile, a runner it cannot open.
	HealthFailed HealthState = "failed"
)

// Fallback is a role a plugin could not play and the plugin that plays it
// instead: the one this build ships for that role. It is the fallback a reader
// sees; the log line is the other (docs/specs/plugin-system.md P1: 回退写进
// Health，不是静默换人).
type Fallback struct {
	Role   string // the role, one of core.Role*
	Player string // the manifest name playing it instead
}

// Health is what the supervisor knows about one plugin now: the state, the count
// behind it, the raw reason it is in it, and the roles it lost. The reason is the
// plugin's or the host's own words — never translated over, never timestamped
// (docs/specs/plugin-system.md P1: Health 是状态加原始原因).
type Health struct {
	State    HealthState
	Failures int
	Reason   string
	Fallback []Fallback
}

// Health answers how one plugin stands now, without starting it: a runner that is
// open is asked for its streak, a plugin that would not start is remembered, and
// a name nothing has asked anything of is fine. It is the plugin level alone —
// what three failed contributions hid is the window's own count.
func (s *Supervisor) Health(name string) Health {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.healthLocked(name)
}

// healthLocked is Health's answer under the lock the caller already holds.
func (s *Supervisor) healthLocked(name string) Health {
	out := Health{State: HealthOK, Fallback: append([]Fallback(nil), s.fallbacks[name]...)}
	if reason, broken := s.broken[name]; broken {
		out.State, out.Reason = HealthFailed, reason
		return out
	}
	runner, ok := s.open[name].(*ext.JSRunner)
	if !ok {
		return out
	}
	failures, disabled, reason := runner.Streak()
	out.Failures, out.Reason = failures, reason
	switch {
	case disabled:
		out.State = HealthDisabled
	case failures > 0:
		out.State = HealthFailing
	}
	return out
}

// Reenable gives a plugin another try: the plugin-level count and the reason go,
// the runner is opened again — its pooled runtimes dropped, so the next call runs
// where the failure left nothing behind — and a plugin that would not start is
// started. A role it lost falls back to it again on the next Resolve.
//
// It is one half of the settings page's one action; the contribution layer's
// counts and hidden set are the window's own and are cleared beside it
// (docs/specs/plugin-system.md P1: 重新启用是一个用户动作、两个内部计数).
func (s *Supervisor) Reenable(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errClosed
	}
	_, broken := s.broken[name]
	delete(s.broken, name)
	delete(s.fallbacks, name)
	if runner, ok := s.open[name].(*ext.JSRunner); ok {
		runner.Reset()
		return nil
	}
	if !broken {
		// Nothing to open: a plugin that is not running is started when something
		// asks it something, not by being re-enabled.
		return nil
	}
	_, err := s.startLocked(name)
	return err
}

// disabledError is a role's plugin stopped by three failed calls: the fallback
// road, reported as itself rather than as a plugin that did not start.
type disabledError struct {
	name     string
	failures int
	reason   string
}

func (e disabledError) Error() string {
	return fmt.Sprintf("remit: %q is disabled after %d failed calls: %s", e.name, e.failures, e.reason)
}

// disabledLocked is why a role cannot use name: it is open and three failures
// stopped it. A stopped runner plays nothing — every call answers ErrDisabled —
// so its role takes the built-in, the same road a plugin that will not start
// takes (docs/specs/plugin-system.md P1: 停用之后回退内置 runner).
func (s *Supervisor) disabledLocked(name string) error {
	runner, ok := s.open[name].(*ext.JSRunner)
	if !ok {
		return nil
	}
	failures, disabled, reason := runner.Streak()
	if !disabled {
		return nil
	}
	return disabledError{name: name, failures: failures, reason: reason}
}

// fellBackLocked records that a plugin lost a role to the one playing it instead,
// so the settings page says so where the plugin is listed.
func (s *Supervisor) fellBackLocked(name, role, player string) {
	if name == "" || player == "" || name == player {
		return
	}
	s.fallbacks[name] = setFallback(s.fallbacks[name], Fallback{Role: role, Player: player})
}

// playedLocked forgets a role a plugin lost, once this library resolves it to a
// plugin that plays: a fallback is a state, not a history.
func (s *Supervisor) playedLocked(name, role string) {
	kept := s.fallbacks[name][:0]
	for _, fallback := range s.fallbacks[name] {
		if fallback.Role != role {
			kept = append(kept, fallback)
		}
	}
	if len(kept) == 0 {
		delete(s.fallbacks, name)
		return
	}
	s.fallbacks[name] = kept
}

// setFallback replaces one role's entry: a plugin lists a role once, however many
// times a library resolved it.
func setFallback(fallbacks []Fallback, entry Fallback) []Fallback {
	kept := fallbacks[:0]
	for _, fallback := range fallbacks {
		if fallback.Role != entry.Role {
			kept = append(kept, fallback)
		}
	}
	return append(kept, entry)
}
