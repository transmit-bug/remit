package about_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/screens/about"
)

// identity is the app as the About page is shown it.
var identity = about.State{Name: "Remit", Version: "0.1.0"}

// at draws the page at a size and lets it settle.
func at(t *testing.T, view func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(view, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// The About page names the app, shows its version and links its license, with
// nothing to ask of the app.
func TestAbout(t *testing.T) {
	view := func(c *ui.Context) { about.View(c, identity, about.Deps{}) }
	tt := at(t, view, about.Width, about.Height)

	for _, want := range []string{identity.Name, "Version " + identity.Version, "GNU AGPL-3.0"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("GNU AGPL-3.0"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 1 || got[0] != about.LicenseURL {
		t.Errorf("license link opened %q, want %q", got, about.LicenseURL)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
