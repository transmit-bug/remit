package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/importmd"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/core/transfer"
)

// runOffline performs the two library commands that need no window, and
// reports whether it did one: exporting a library and unpacking an archive.
func runOffline(dir, exportPath, importPath string) bool {
	if exportPath == "" && importPath == "" {
		return false
	}

	if importPath != "" {
		src, err := os.Open(importPath)
		if err != nil {
			log.Fatalf("open %s: %v", importPath, err)
		}
		defer src.Close()
		info, err := src.Stat()
		if err != nil {
			log.Fatalf("stat %s: %v", importPath, err)
		}
		if err := transfer.Read(src, info.Size(), dir); err != nil {
			log.Fatalf("import: %v", err)
		}
		fmt.Printf("imported %s into %s\n", importPath, dir)
		return true
	}

	if _, err := os.Stat(filepath.Join(dir, "remit.json")); err != nil {
		log.Fatalf("no library at %s: %v", dir, err)
	}
	dst, err := os.Create(exportPath)
	if err != nil {
		log.Fatalf("create %s: %v", exportPath, err)
	}
	if err := transfer.Write(dir, dst); err != nil {
		dst.Close()
		log.Fatalf("export: %v", err)
	}
	if err := dst.Close(); err != nil {
		log.Fatalf("close %s: %v", exportPath, err)
	}
	fmt.Printf("exported %s to %s\n", dir, exportPath)
	return true
}

// runImportAnki imports an Anki package into the library at dir, creating the
// library when it does not exist yet, and prints what it did.
func runImportAnki(dir, path string) bool {
	if path == "" {
		return false
	}
	lib, err := openOrCreate(dir)
	if err != nil {
		log.Fatalf("library %s: %v", dir, err)
	}
	defer lib.Close()

	report, err := importanki.Import(path, lib)
	if err != nil {
		log.Fatalf("import %s: %v", path, err)
	}
	fmt.Printf("imported %s: %d notes, %d cards, %d reviews, %d skipped rows, %d unreachable media\n",
		path, report.Notes, report.Cards, report.Reviews, report.SkippedRows, report.UnreachableMedia)
	for _, warning := range report.Warnings {
		fmt.Printf("  warning: %s\n", warning)
	}
	return true
}

// runMarkdown performs the two Markdown commands that need no window, and
// reports whether it did one: importing files as notes, exporting the
// library's notes as files.
func runMarkdown(dir, importPath, exportDir string) bool {
	if importPath == "" && exportDir == "" {
		return false
	}
	lib, err := openOrCreate(dir)
	if err != nil {
		log.Fatalf("library %s: %v", dir, err)
	}
	defer lib.Close()

	if importPath != "" {
		report, err := importmd.Import(importPath, lib, importmd.Options{})
		if err != nil {
			log.Fatalf("import %s: %v", importPath, err)
		}
		fmt.Printf("imported %s: %d notes, %d kept, %d skipped\n", importPath, report.Notes, report.Kept, report.Skipped)
		for _, warning := range report.Warnings {
			fmt.Printf("  warning: %s\n", warning)
		}
		return true
	}

	report, err := importmd.Export(exportDir, lib)
	if err != nil {
		log.Fatalf("export %s: %v", exportDir, err)
	}
	fmt.Printf("exported notes to %s: %d files, %d skipped\n", exportDir, report.Files, report.Skipped)
	return true
}

// openOrCreate opens the library at dir, creating it when it is absent.
func openOrCreate(dir string) (*store.Library, error) {
	lib, err := store.Open(dir, store.Options{})
	if errors.Is(err, fs.ErrNotExist) {
		return store.Create(dir, store.Options{})
	}
	return lib, err
}
