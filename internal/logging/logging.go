// Package logging writes Remit's own diagnostics to a local file. Nothing it
// writes leaves the machine, and nothing here can: the only destination is the
// directory the app hands it.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// FileName is the log file inside the directory passed to Open.
const FileName = "remit.log"

// Open returns a logger that appends to <dir>/remit.log, and a function that
// closes it. The directory is created when missing. The file is readable only
// by its owner: a log may quote a note's text or a path.
func Open(dir string) (*slog.Logger, func() error, error) {
	if dir == "" {
		return nil, nil, fmt.Errorf("logging: no directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("logging: create %s: %w", dir, err)
	}
	path := filepath.Join(dir, FileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("logging: open %s: %w", path, err)
	}
	return slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})), f.Close, nil
}

// Discard is the logger for a context that must not write anywhere, such as a
// test or a dry run.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
