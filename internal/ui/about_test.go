package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The About page names the app, shows its version and links its license at the
// size the window opens at.
func TestAbout(t *testing.T) {
	tt := ui.NewTester(About, aboutWidth, aboutHeight)
	settle(tt)

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
