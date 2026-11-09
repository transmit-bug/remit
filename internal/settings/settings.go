// Package settings holds the app's own preferences — the ones that are not
// part of a library, and that must never travel with one (decision #23).
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the settings file inside the app's data directory.
const FileName = "settings.json"

// DefaultHotkey is the review shortcut the app binds when the user has not
// chosen one: Cmd+Shift+R on macOS, Ctrl+Shift+R elsewhere.
const DefaultHotkey = "CmdOrCtrl+Shift+R"

// The appearances the user may pick: the desktop's own, or an override. They
// are the app's preference, not the library's, because the desktop a library is
// read on decides how it looks (decision #23).
const (
	AppearanceSystem = "system"
	AppearancePaper  = "paper"
	AppearanceInk    = "ink"
)

// The paper's measure: what a document's column may be set to, in DIPs, and
// what it is until the user says otherwise.
const (
	PaperWidthMin     = 620
	PaperWidthMax     = 860
	PaperWidthStep    = 20
	DefaultPaperWidth = 760
)

// RecentLimit is how many libraries the start view remembers: the ones a user
// moves between, without the list growing into a second library of its own.
const RecentLimit = 5

// Settings are the user's preferences. The zero value is the shipped default:
// no update check, no agent server, no reminders, the default shortcut, and no
// library path chosen yet.
type Settings struct {
	// UpdateCheck is off by default: Remit sends nothing anywhere unless the
	// user turns this on.
	UpdateCheck bool `json:"update_check"`
	// LibraryDir is the library the app opens on launch; empty means the
	// default location.
	LibraryDir string `json:"library_dir,omitempty"`
	// AgentServer serves the local agent API while the app runs. Off by
	// default: an open port is a decision the user makes, not one an update
	// makes for them.
	AgentServer bool `json:"agent_server,omitempty"`
	// Notifications sends at most one reminder a day while it is on. Off by
	// default: an operating-system notification is the user's to ask for.
	Notifications bool `json:"notifications,omitempty"`
	// Hotkey is the global accelerator that starts a review; empty means
	// DefaultHotkey.
	Hotkey string `json:"hotkey,omitempty"`
	// RemindedOn is the calendar day (2006-01-02) of the last reminder, so
	// that "once a day" holds across restarts; empty means none yet.
	RemindedOn string `json:"reminded_on,omitempty"`
	// Appearance is the look the user chose: AppearanceSystem (empty, the
	// default), AppearancePaper or AppearanceInk. It is an override of the
	// desktop's own appearance, which is the one a library-less first run keeps.
	Appearance string `json:"appearance,omitempty"`
	// PaperWidth is the document paper's measure in DIPs; zero means
	// DefaultPaperWidth.
	PaperWidth int `json:"paper_width,omitempty"`
	// SavedFilters are the recall queue's filters the user kept, by their stable
	// id: a chip on the queue page, and no card data, so it belongs to the app
	// rather than to a library.
	SavedFilters []string `json:"saved_filters,omitempty"`
	// RecentLibraries are the libraries the app opened before, newest first and
	// at most RecentLimit of them. The start view lists them so a second library
	// is one click away; only the path is kept, never anything it holds.
	RecentLibraries []string `json:"recent_libraries,omitempty"`
}

// WithRecent puts dir at the front of the libraries the app remembers, once
// each and at most RecentLimit of them: opening a library twice moves it to the
// front rather than listing it twice.
func (s Settings) WithRecent(dir string) Settings {
	if dir == "" {
		return s
	}
	recent := make([]string, 0, len(s.RecentLibraries)+1)
	recent = append(recent, dir)
	for _, path := range s.RecentLibraries {
		if path == dir || path == "" {
			continue
		}
		recent = append(recent, path)
		if len(recent) == RecentLimit {
			break
		}
	}
	s.RecentLibraries = recent
	return s
}

// ForgetRecent takes dir out of the libraries the app remembers: a place that
// will not open is dropped from the list it was offered in.
func (s Settings) ForgetRecent(dir string) Settings {
	recent := make([]string, 0, len(s.RecentLibraries))
	for _, path := range s.RecentLibraries {
		if path == dir {
			continue
		}
		recent = append(recent, path)
	}
	s.RecentLibraries = recent
	return s
}

// AppearanceName is the look the window draws: the one chosen, or the
// desktop's when none was.
func (s Settings) AppearanceName() string {
	switch s.Appearance {
	case AppearancePaper, AppearanceInk:
		return s.Appearance
	}
	return AppearanceSystem
}

// ThemeSource is that look as MyGo's theme source names it: the desktop's
// answer, or the forced light or dark one.
func (s Settings) ThemeSource() string {
	switch s.AppearanceName() {
	case AppearancePaper:
		return "light"
	case AppearanceInk:
		return "dark"
	}
	return "system"
}

// PaperMeasure is the document paper's width: what the user set, kept inside
// the range the slider offers, or the default while they have set none.
func (s Settings) PaperMeasure() int {
	if s.PaperWidth < PaperWidthMin || s.PaperWidth > PaperWidthMax {
		if s.PaperWidth == 0 {
			return DefaultPaperWidth
		}
		return min(max(s.PaperWidth, PaperWidthMin), PaperWidthMax)
	}
	return s.PaperWidth
}

// HotkeyAccelerator is the shortcut the app binds: the one the user chose, or
// DefaultHotkey when they have chosen none.
func (s Settings) HotkeyAccelerator() string {
	if acc := strings.TrimSpace(s.Hotkey); acc != "" {
		return acc
	}
	return DefaultHotkey
}

// Load reads the settings in dir, returning the defaults when the file does
// not exist yet.
func Load(dir string) (Settings, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("settings: read: %w", err)
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("settings: %s: %w", filepath.Join(dir, FileName), err)
	}
	return s, nil
}

// Save writes the settings, creating the directory when missing.
func Save(dir string, s Settings) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("settings: create %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("settings: encode: %w", err)
	}
	path := filepath.Join(dir, FileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("settings: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("settings: rename: %w", err)
	}
	return nil
}

// Checker looks for a newer release. It is a function so that a caller can
// prove the app asks nothing when the setting is off.
type Checker func() (version string, err error)

// MaybeCheck runs check only when the user enabled update checks. This is the
// one gate every update call goes through, so "off" means no traffic.
func (s Settings) MaybeCheck(check Checker) (string, error) {
	if !s.UpdateCheck || check == nil {
		return "", nil
	}
	return check()
}
