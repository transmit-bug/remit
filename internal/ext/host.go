package ext

import (
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
// A host is optional. The extensions the app ships declare no capabilities and
// are opened before a library is; a call that needs a host a runner was not
// given is an error, never a panic.
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
