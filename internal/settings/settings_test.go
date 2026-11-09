package settings

import (
	"os"
	"path/filepath"
	"reflect"
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
	if s.AgentServer {
		t.Error("a fresh install opens an agent port")
	}
	if s.Notifications {
		t.Error("a fresh install sends notifications")
	}
	if s.RemindedOn != "" {
		t.Errorf("a fresh install reports a reminder on %q", s.RemindedOn)
	}
	if got := s.HotkeyAccelerator(); got != DefaultHotkey {
		t.Errorf("hotkey = %q, want the default %q", got, DefaultHotkey)
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
	want := Settings{
		UpdateCheck: true, LibraryDir: "/home/ada/Remit", AgentServer: true,
		Notifications: true, Hotkey: "CmdOrCtrl+Shift+J", RemindedOn: "2026-11-06",
		Appearance: AppearanceInk, PaperWidth: 700,
		SavedFilters:    []string{"new", "hard"},
		RecentLibraries: []string{"/home/ada/Remit", "/home/ada/Work"},
	}
	if err := Save(dir, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
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

// The look and the paper's measure are the app's preferences: the zero value
// follows the desktop at the default measure, and a stored one is read back
// inside the range the page offers.
func TestAppearanceAndPaperMeasure(t *testing.T) {
	if got := (Settings{}).AppearanceName(); got != AppearanceSystem {
		t.Errorf("a fresh install looks %q, want the desktop's", got)
	}
	if got := (Settings{}).ThemeSource(); got != "system" {
		t.Errorf("a fresh install asks MyGo for %q, want system", got)
	}
	if got := (Settings{}).PaperMeasure(); got != DefaultPaperWidth {
		t.Errorf("a fresh install's paper is %d, want %d", got, DefaultPaperWidth)
	}
	for _, c := range []struct {
		appearance, source string
	}{
		{AppearanceSystem, "system"},
		{AppearancePaper, "light"},
		{AppearanceInk, "dark"},
		{"green", "system"},
	} {
		s := Settings{Appearance: c.appearance}
		if got := s.ThemeSource(); got != c.source {
			t.Errorf("appearance %q asks for %q, want %q", c.appearance, got, c.source)
		}
	}
	for _, c := range []struct{ set, want int }{
		{0, DefaultPaperWidth}, {700, 700}, {PaperWidthMin - 40, PaperWidthMin}, {PaperWidthMax + 40, PaperWidthMax},
	} {
		if got := (Settings{PaperWidth: c.set}).PaperMeasure(); got != c.want {
			t.Errorf("paper width %d reads as %d, want %d", c.set, got, c.want)
		}
	}
}

// The shortcut the user typed is used as it stands, and clearing it puts the
// default back rather than binding nothing.
func TestHotkeyAccelerator(t *testing.T) {
	if got := (Settings{Hotkey: " Ctrl+Alt+R "}).HotkeyAccelerator(); got != "Ctrl+Alt+R" {
		t.Errorf("hotkey = %q, want it trimmed", got)
	}
	if got := (Settings{Hotkey: "   "}).HotkeyAccelerator(); got != DefaultHotkey {
		t.Errorf("blank hotkey = %q, want %q", got, DefaultHotkey)
	}
}

// Opening a library moves it to the front of the list the welcome screen
// offers, once: the sixth library is forgotten rather than the list growing,
// and a library opened twice is not offered twice.
func TestRecentLibrariesKeepTheLastFive(t *testing.T) {
	var s Settings
	for _, dir := range []string{"/a", "/b", "/c", "/d", "/e", "/f"} {
		s = s.WithRecent(dir)
	}
	want := []string{"/f", "/e", "/d", "/c", "/b"}
	if !reflect.DeepEqual(s.RecentLibraries, want) {
		t.Fatalf("recent = %q, want %q", s.RecentLibraries, want)
	}

	s = s.WithRecent("/d")
	want = []string{"/d", "/f", "/e", "/c", "/b"}
	if !reflect.DeepEqual(s.RecentLibraries, want) {
		t.Errorf("reopening a library gave %q, want %q", s.RecentLibraries, want)
	}

	s = s.WithRecent("")
	if !reflect.DeepEqual(s.RecentLibraries, want) {
		t.Errorf("an empty path changed the list to %q", s.RecentLibraries)
	}
}

// A library that is not there any more is taken out of the list it was offered
// in, and only that one.
func TestForgetRecentDropsOnePath(t *testing.T) {
	s := Settings{RecentLibraries: []string{"/a", "/b", "/c"}}
	got := s.ForgetRecent("/b").RecentLibraries
	if want := []string{"/a", "/c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("forgetting /b gave %q, want %q", got, want)
	}
	if len(s.RecentLibraries) != 3 {
		t.Errorf("ForgetRecent changed the value it was called on: %q", s.RecentLibraries)
	}
}
