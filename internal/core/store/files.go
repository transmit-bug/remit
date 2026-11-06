package store

import (
	"os"
	"path/filepath"
)

// writeFileAtomic replaces path with data in one step: a reader either sees the
// old file or the new one, never a half-written note or card. The bytes are
// flushed before the rename so a crash leaves the old file intact.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // no-op once the rename has moved it

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, filePerms); err != nil {
		return err
	}
	return os.Rename(name, path)
}
