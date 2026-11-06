package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenWritesLocally(t *testing.T) {
	dir := t.TempDir()

	logger, closeLog, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	logger.Info("opened a library", "path", "/tmp/library")
	if err := closeLog(); err != nil {
		t.Fatalf("close: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "opened a library") {
		t.Errorf("log does not contain the message: %q", data)
	}
	if !strings.Contains(string(data), "level=INFO") {
		t.Errorf("log line has no level: %q", data)
	}
}

func TestOpenAppends(t *testing.T) {
	dir := t.TempDir()

	for _, msg := range []string{"first", "second"} {
		logger, closeLog, err := Open(dir)
		if err != nil {
			t.Fatalf("Open() = %v", err)
		}
		logger.Info(msg)
		if err := closeLog(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "first") || !strings.Contains(string(data), "second") {
		t.Errorf("log lost an earlier run: %q", data)
	}
}

func TestOpenRejectsEmptyDir(t *testing.T) {
	if _, _, err := Open(""); err == nil {
		t.Error("Open(\"\") succeeded, want an error")
	}
}

func TestLogFileIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	_, closeLog, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if err := closeLog(); err != nil {
		t.Fatalf("close: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("log file mode = %o, want 600", perm)
	}
}
