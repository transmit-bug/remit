package main

import (
	"log/slog"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
)

// startExtensions runs the extensions this library enabled. A broken or
// missing extension is reported and skipped: enabling one must never keep the
// app from opening.
//
// The runners reach the library through the same source the loopback API uses:
// the store's read/write layer plus card derivation, so a plugin's cards.create
// makes what the app would.
func startExtensions(lib *store.Library) {
	enabled := lib.EnabledExtensions()
	if len(enabled) == 0 {
		return
	}
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
	if len(runners) == 0 {
		return
	}

	// Events are notifications, not vetoes: a failing extension is logged and
	// the review goes on.
	lib.SetEventHandler(func(ev store.Event) {
		for _, r := range runners {
			if err := r.Call("on_event", map[string]any{"kind": ev.Kind, "id": string(ev.ID)}, nil); err != nil {
				slog.Warn("remit: extension event failed", "extension", r.Name(), "err", err)
			}
		}
	})
}
