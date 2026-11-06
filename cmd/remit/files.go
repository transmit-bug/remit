package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/core/transfer"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// libraryExt is the file type the packaging associates with Remit.
const libraryExt = ".remitlib"

// installFileAssociations opens a library archive dropped on the app or
// double-clicked in the file manager.
func installFileAssociations(a *appui.App, root string) {
	mygo.App.OnOpenFile(func(path string) {
		if err := openArchive(a, root, path); err != nil {
			slog.Warn("remit: open file", "path", path, "err", err)
		}
	})
}

// openArchive opens the library an archive holds: the copy imported earlier
// when there is one, otherwise a fresh import into root, named after the file.
func openArchive(a *appui.App, root, path string) error {
	if !strings.EqualFold(filepath.Ext(path), libraryExt) {
		return fmt.Errorf("%s is not a %s file", filepath.Base(path), libraryExt)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if name == "" {
		return fmt.Errorf("%s has no library name", path)
	}
	target := filepath.Join(root, name)
	if _, err := os.Stat(filepath.Join(target, "remit.json")); err == nil {
		a.OpenLibrary(target)
		return nil
	}

	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	if err := transfer.Read(src, info.Size(), target); err != nil {
		return err
	}
	a.OpenLibrary(target)
	return nil
}
