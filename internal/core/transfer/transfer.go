// Package transfer moves a library in and out: a .remitlib is a zip of the
// plaintext sources, never the derived index, so the archive is readable, and
// unpacking it produces a library that rebuilds itself (ADR 0003).
package transfer

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SourceFiles are the parts of a library that are its own data — the assets
// directory included, so a note's images travel in the archive (ADR 0003).
// The derived index under .remit is deliberately absent.
var SourceFiles = []string{"remit.json", "notes", "cards", "assets", "reviews.jsonl", "types", "plugins-data"}

// Write packages the library at dir into w.
func Write(dir string, w io.Writer) error {
	zw := zip.NewWriter(w)
	for _, name := range SourceFiles {
		root := filepath.Join(dir, name)
		info, err := os.Stat(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("transfer: %s: %w", name, err)
		}
		if !info.IsDir() {
			if err := addFile(zw, dir, name); err != nil {
				return err
			}
			continue
		}
		err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			return addFile(zw, dir, rel)
		})
		if err != nil {
			return fmt.Errorf("transfer: walk %s: %w", name, err)
		}
	}
	return zw.Close()
}

func addFile(zw *zip.Writer, dir, rel string) error {
	f, err := os.Open(filepath.Join(dir, rel))
	if err != nil {
		return fmt.Errorf("transfer: open %s: %w", rel, err)
	}
	defer f.Close()
	w, err := zw.Create(filepath.ToSlash(rel))
	if err != nil {
		return fmt.Errorf("transfer: zip %s: %w", rel, err)
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("transfer: copy %s: %w", rel, err)
	}
	return nil
}

// Read unpacks a .remitlib into dir, which must not already hold a library.
// Entries outside the archive's own tree are refused: an archive is not
// trusted to stay inside its directory.
func Read(r io.ReaderAt, size int64, dir string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("transfer: read archive: %w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "remit.json")); err == nil {
		return fmt.Errorf("transfer: %s already holds a library", dir)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("transfer: create %s: %w", dir, err)
	}
	for _, entry := range zr.File {
		name := filepath.Clean(entry.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) || name == "." {
			return fmt.Errorf("transfer: refusing entry %q", entry.Name)
		}
		target := filepath.Join(dir, name)
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
			return fmt.Errorf("transfer: refusing entry %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fmt.Errorf("transfer: create %s: %w", target, err)
		}
		if err := writeEntry(entry, target); err != nil {
			return err
		}
	}
	return nil
}

func writeEntry(entry *zip.File, target string) error {
	src, err := entry.Open()
	if err != nil {
		return fmt.Errorf("transfer: open entry %s: %w", entry.Name, err)
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("transfer: create %s: %w", target, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("transfer: write %s: %w", target, err)
	}
	return dst.Close()
}
