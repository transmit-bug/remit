package ext

import (
	"errors"
	"time"

	"github.com/transmit-bug/remit/internal/agentapi"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// Host is what a plugin reaches outside its own runtime: the read and write
// layer the loopback agent API also serves — one declaration, agentapi.Library,
// not a second one here — plus the library's clock and FSRS parameters, and the
// plugin's own store.
//
// A host is optional, and a runner may be given one after it started. The
// extensions the app ships declare no capabilities and are opened before a
// library is; a call that needs a host the runner has none of is ErrNoLibrary —
// never a panic, and never a state the runner cannot leave.
type Host interface {
	agentapi.Library

	// Now is the library's clock: the moment "due now" is measured from.
	Now() time.Time
	// ScheduleParams is the schedule this library reviews with, so that a
	// plugin advancing a state computes what the app would.
	ScheduleParams() schedule.Params
	// DeleteCard removes a card's definition and its schedule. The HTTP API
	// deliberately has no delete; a plugin that declared cards:write does.
	DeleteCard(core.ID) error

	// PluginData is the value the plugin stored under id; SetPluginData stores
	// one. A runner only ever passes its own id, so a plugin reaches its own
	// namespace and no other.
	PluginData(id string) (string, bool)
	SetPluginData(id, value string) error
}

// HostFunc answers with the library a capability call acts on, at the moment
// the call is made; nil is no library yet. It is the seam a runner holds instead
// of a host when the service that owns the library outlives it: binding a
// library then hands nothing to the runner, the provider answers differently
// from the next call on.
type HostFunc func() Host

// HostError is a failure the host raised while answering a capability call: a
// write the library refused, a disk that is full. Its message is the host's own,
// unchanged — a plugin reads what went wrong, not a label — and the runner
// recognises it on its way back out of the runtime, so the app's failure is
// logged instead of counted among the plugin's three
// (docs/specs/plugin-system.md P1: 失败要分归属).
type HostError struct{ Err error }

// Error is the host's own message, unchanged.
func (e HostError) Error() string { return e.Err.Error() }

// Unwrap is the host's own error, for errors.Is and errors.As.
func (e HostError) Unwrap() error { return e.Err }

// IsHostError reports whether err is the host's own failure. Uncaught, it comes
// back from the runtime as the thrown value's error, which unwraps to this.
func IsHostError(err error) bool {
	var failure HostError
	return errors.As(err, &failure)
}

// guardedHost is the host one capability call gets, and the one place a host
// failure is marked: everything it fails at leaves as a HostError. A plugin's own
// mistakes — an id it made up, an argument of the wrong shape — are answered
// above this, in the capability function, and stay the plugin's.
type guardedHost struct{ Host }

func (h guardedHost) SearchNotes(text string) ([]core.Note, error) {
	notes, err := h.Host.SearchNotes(text)
	return notes, hostError(err)
}

func (h guardedHost) CreateCard(block core.ID) (core.Card, error) {
	card, err := h.Host.CreateCard(block)
	return card, hostError(err)
}

func (h guardedHost) SaveCard(card core.Card) error { return hostError(h.Host.SaveCard(card)) }

func (h guardedHost) DeleteCard(id core.ID) error { return hostError(h.Host.DeleteCard(id)) }

func (h guardedHost) History(id core.ID) ([]schedule.Event, error) {
	events, err := h.Host.History(id)
	return events, hostError(err)
}

func (h guardedHost) Remaining(at time.Time) (int, error) {
	due, err := h.Host.Remaining(at)
	return due, hostError(err)
}

func (h guardedHost) Review(id core.ID, rating schedule.Rating, at time.Time) error {
	return hostError(h.Host.Review(id, rating, at))
}

func (h guardedHost) SetPluginData(id, value string) error {
	return hostError(h.Host.SetPluginData(id, value))
}

// hostError marks one host failure, leaving the host's message as it stands.
func hostError(err error) error {
	if err == nil {
		return nil
	}
	return HostError{Err: err}
}

// UseHost makes every later capability call resolve its host through provide,
// replacing the host the runner was opened with. The runner is not restarted:
// a plugin started before a library is open answers ErrNoLibrary until one is,
// then answers from it — the same runner, with its module state and its pool
// kept. A library switch is seen the same way, one call later. A nil provide
// leaves the runner reaching nothing again.
func (r *JSRunner) UseHost(provide HostFunc) {
	r.hostMu.Lock()
	defer r.hostMu.Unlock()
	r.host = provide
}

// hostProvider is the provider a capability call asks, copied out under the
// lock and asked without it: a provider that takes a lock of its own cannot
// deadlock a runtime that is being loaded, and one call sees one provider.
func (r *JSRunner) hostProvider() HostFunc {
	r.hostMu.RLock()
	defer r.hostMu.RUnlock()
	return r.host
}

// fixedHost is the provider a runner opened with one host uses.
func fixedHost(host Host) HostFunc {
	if host == nil {
		return nil
	}
	return func() Host { return host }
}
