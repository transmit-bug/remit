package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/transmit-bug/remit/internal/core"
)

// typesDir holds a library's card types, one human-readable JSON file per
// type (spec §2.1: types are plain-text library data, like notes and cards).
const typesDir = "types"

// typeFile is the on-disk shape of a card type.
type typeFile struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Fields  []string `json:"fields"`
	Front   string   `json:"front"`
	Back    string   `json:"back"`
	Version int      `json:"version"`
}

// encodeType renders a type to its on-disk form: indented JSON, because types
// are edited by hand like every other library file.
func encodeType(typ core.Type) ([]byte, error) {
	data, err := json.MarshalIndent(typeFile{
		ID:      string(typ.ID),
		Name:    typ.Name,
		Fields:  typ.Fields,
		Front:   typ.Front,
		Back:    typ.Back,
		Version: typ.Version,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: encode type %q: %w", typ.ID, err)
	}
	return append(data, '\n'), nil
}

// DecodeType reads a type file. A missing version reads as 1; a type without
// an ID takes the file's name, so a hand-written types/front.json works. It is
// exported for a reader that must not open the library, so a checker can report
// a type the store would refuse instead of failing on it.
func DecodeType(file string, data []byte) (core.Type, error) {
	var tf typeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return core.Type{}, fmt.Errorf("store: decode type %q: %w", file, err)
	}
	if tf.ID == "" {
		base := filepath.Base(file)
		if cut := len(base) - len(filepath.Ext(base)); cut > 0 {
			tf.ID = base[:cut]
		}
	}
	if tf.ID == "" || tf.Name == "" || len(tf.Fields) == 0 {
		return core.Type{}, fmt.Errorf("store: type file %q is not a type", file)
	}
	if tf.Version == 0 {
		tf.Version = 1
	}
	return core.Type{
		ID:      core.ID(tf.ID),
		Name:    tf.Name,
		Fields:  tf.Fields,
		Front:   tf.Front,
		Back:    tf.Back,
		Version: tf.Version,
	}, nil
}

// loadTypes reads every type the library holds and, when seed is set, writes
// the built-ins a missing one leaves out (spec §2.2: the three built-ins always
// exist). It runs once per open, on three-plus small files, so it costs nothing
// on the index path. seed is false for a repair tool's rebuild, which must not
// change a source file.
func loadTypes(dir string, seed bool) ([]core.Type, error) {
	entries, err := os.ReadDir(filepath.Join(dir, typesDir))
	if err != nil {
		if os.IsNotExist(err) {
			entries = nil
		} else {
			return nil, err
		}
	}
	held := make(map[core.ID]bool, len(entries)+len(core.BuiltinTypes()))
	types := make([]core.Type, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, typesDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		typ, err := DecodeType(entry.Name(), data)
		if err != nil {
			return nil, err
		}
		held[typ.ID] = true
		types = append(types, typ)
	}
	for _, typ := range core.BuiltinTypes() {
		if held[typ.ID] {
			continue
		}
		if seed {
			if err := writeType(dir, typ); err != nil {
				return nil, err
			}
		}
		types = append(types, typ)
	}
	sort.Slice(types, func(i, j int) bool { return types[i].ID < types[j].ID })
	return types, nil
}

// writeType stores a type in the library's types directory.
func writeType(dir string, typ core.Type) error {
	if err := os.MkdirAll(filepath.Join(dir, typesDir), dirPerms); err != nil {
		return err
	}
	data, err := encodeType(typ)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, typesDir, string(typ.ID)+".json"), data)
}

// Types returns every card type the library holds, built-ins included, sorted
// by ID.
func (l *Library) Types() []core.Type {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]core.Type, len(l.types))
	copy(out, l.types)
	return out
}

// Type returns the card type with the given ID.
func (l *Library) Type(id core.ID) (core.Type, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, typ := range l.types {
		if typ.ID == id {
			return typ, true
		}
	}
	return core.Type{}, false
}
