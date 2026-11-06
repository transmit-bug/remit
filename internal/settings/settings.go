// Package settings holds the app's own preferences — the ones that are not
// part of a library, and that must never travel with one (decision #23).
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the settings file inside the app's data directory.
const FileName = "settings.json"

// Settings are the user's preferences. The zero value is the shipped default:
// no update check, no library path chosen yet.
type Settings struct {
	// UpdateCheck is off by default: Remit sends nothing anywhere unless the
	// user turns this on.
	UpdateCheck bool `json:"update_check"`
	// LibraryDir is the library the app opens on launch; empty means the
	// default location.
	LibraryDir string `json:"library_dir,omitempty"`
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
