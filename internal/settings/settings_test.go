package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsSendNothing(t *testing.T) {
	s, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load(missing): %v", err)
	}
	if s.UpdateCheck {
		t.Error("a fresh install checks for updates")
	}

	called := false
	if _, err := s.MaybeCheck(func() (string, error) {
		called = true
		return "1.0.0", nil
	}); err != nil {
		t.Fatalf("MaybeCheck: %v", err)
	}
	if called {
		t.Error("the update checker ran while the setting was off")
	}
}

func TestEnabledCheckDoesAsk(t *testing.T) {
	s := Settings{UpdateCheck: true}
	got, err := s.MaybeCheck(func() (string, error) { return "0.2.0", nil })
	if err != nil {
		t.Fatalf("MaybeCheck: %v", err)
	}
	if got != "0.2.0" {
		t.Errorf("version = %q, want 0.2.0", got)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Settings{UpdateCheck: true, LibraryDir: "/home/ada/Remit"}
	if err := Save(dir, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	info, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("settings mode = %o, want 600", perm)
	}
}

func TestCorruptSettingsAreReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("Load accepted a corrupt settings file")
	}
}
