package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// pluginsDataDir is where a plugin's own data lives: one readable JSON file per
// plugin, inside the library, so it travels with the archive (spec §6,
// store:own).
const pluginsDataDir = "plugins-data"

// ErrBadPluginID is a plugin id that cannot name a file: empty, or carrying a
// path in it.
var ErrBadPluginID = errors.New("store: plugin id cannot name a file")

// pluginDataFile is one plugin's stored value, as it lies on disk: a JSON
// object a person can read, not a wire format.
type pluginDataFile struct {
	Value string `json:"value"`
}

// PluginData returns the value a plugin stored under id, and whether it had
// one. A plugin's namespace is its own: one plugin reads no other's data.
func (l *Library) PluginData(id string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !validPluginID(id) {
		return "", false
	}
	data, err := os.ReadFile(l.pluginDataPath(id))
	if err != nil {
		return "", false
	}
	var file pluginDataFile
	if err := json.Unmarshal(data, &file); err != nil {
		return "", false
	}
	return file.Value, true
}

// SetPluginData stores a plugin's value in plugins-data/<id>.json. A value
// that is already what the file says is not rewritten: the file keeps its
// bytes until there is something new to say.
func (l *Library) SetPluginData(id, value string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if !validPluginID(id) {
		return ErrBadPluginID
	}
	data, err := json.MarshalIndent(pluginDataFile{Value: value}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := l.pluginDataPath(id)
	if current, err := os.ReadFile(path); err == nil {
		var file pluginDataFile
		if json.Unmarshal(current, &file) == nil && file.Value == value {
			return nil // the file already says this, byte for byte it may not
		}
	}
	if err := os.MkdirAll(filepath.Join(l.dir, pluginsDataDir), dirPerms); err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

// pluginDataPath is where a plugin's value lives.
func (l *Library) pluginDataPath(id string) string {
	return filepath.Join(l.dir, pluginsDataDir, id+".json")
}

// validPluginID reports whether id can name a file inside plugins-data: it
// must be a name, not a path — no separators, no dot-or-dotdot traversal.
func validPluginID(id string) bool {
	if id == "" || id == "." || id == ".." || strings.Contains(id, "..") {
		return false
	}
	return !strings.ContainsAny(id, `/\`) && id != string(filepath.Separator)
}
