package ui

import (
	"log/slog"

	"github.com/transmit-bug/remit/internal/settings"
)

// loadSettings reads the app's settings from its data directory, so that a
// choice made in an earlier run is shown in this one. Settings that cannot be
// read leave the defaults and a log line: a broken settings file must not keep
// the window from opening.
func (a *App) loadSettings() {
	if a.settingsDir == "" {
		a.syncPrefs()
		return
	}
	s, err := settings.Load(a.settingsDir)
	if err != nil {
		slog.Warn("remit: read settings", "err", err)
		a.syncPrefs()
		return
	}
	a.settings = s
	if a.dir == "" {
		a.dir = s.LibraryDir
	}
	a.syncPrefs()
}

// Settings returns the app's preferences as the last read from or written to
// disk. It is a copy: change them with UpdateSettings, so the write, the page
// and the services that read one preference all see the change.
func (a *App) Settings() settings.Settings { return a.settings }

// UpdateSettings applies fn to the settings, writes them atomically and with
// owner-only permissions, reads them back and tells whoever registered
// OnSettings. What the app and its page show is then what the file holds; a
// write that fails leaves the settings as they were and returns the error.
func (a *App) UpdateSettings(fn func(*settings.Settings)) error {
	next := a.settings
	fn(&next)
	if a.settingsDir != "" {
		if err := settings.Save(a.settingsDir, next); err != nil {
			return err
		}
		saved, err := settings.Load(a.settingsDir)
		if err != nil {
			return err
		}
		next = saved
	}
	a.settings = next
	a.syncPrefs()
	if a.settingsChanged != nil {
		a.settingsChanged(next)
	}
	return nil
}

// OnSettings registers a function told about the settings whenever the app
// writes them, which is how the tray's pause item follows the page and the page
// follows the tray. Setting it again replaces the previous one.
func (a *App) OnSettings(fn func(settings.Settings)) {
	a.settingsChanged = fn
	if fn != nil {
		fn(a.settings)
	}
}

// remember writes the library the user chose into the settings, keeping every
// other preference, so the next start opens it without asking again.
func (a *App) remember(dir string) error {
	return a.UpdateSettings(func(s *settings.Settings) { s.LibraryDir = dir })
}
