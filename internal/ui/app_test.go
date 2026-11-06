package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The view runs headless: no window, no GPU, as fast as a unit test.
func TestView(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 480, 360)
	tt.Frame()

	for _, want := range []string{Name, "No library open"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// The About page names the app, shows its version and links its license at the
// size the window opens at.
func TestAbout(t *testing.T) {
	tt := ui.NewTester(About, aboutWidth, aboutHeight)
	tt.Frame()

	for _, want := range []string{Name, "Version " + Version, "GNU AGPL-3.0"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("GNU AGPL-3.0"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 1 || got[0] != licenseURL {
		t.Errorf("license link opened %q, want %q", got, licenseURL)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Packaging reads mygo.json's version and the About page shows Version: the
// two must not drift apart.
func TestVersionMatchesConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("mygo.json: %v", err)
	}
	if cfg.Version != Version {
		t.Errorf("mygo.json version %q, package Version %q", cfg.Version, Version)
	}
}
