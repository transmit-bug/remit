package main

import (
	"io"
	"log/slog"

	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/plugin"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// startExtensions subscribes the plugins the library enables to its events, and
// the window too: a contributed page is what the library was when it loaded.
//
// The supervisor started them at Bind and holds one runner per name, so a plugin
// that is both enabled and a role hears events and answers hooks through the same
// pool, and its own store has one writer (docs/specs/plugin-system.md, P1). The
// runners reach the library through the provider Bind gave them, resolved per
// capability call, so one started before this library opened acts on it too.
func startExtensions(lib *store.Library, app *appui.App, plugins *plugin.Supervisor) {
	runners := plugins.Enabled()
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

// hostFor is the library the plugins act on, as the supervisor binds it: the
// same read and write layer the loopback agent API serves, deriving through the
// same card role, so a capability call and an API request see one library and
// one card plugin.
func hostFor(lib *store.Library, plugins *plugin.Supervisor) ext.Host {
	return newCardSource(lib, plugins)
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
