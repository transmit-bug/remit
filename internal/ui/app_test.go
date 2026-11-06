package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The view runs headless: no window, no GPU, as fast as a unit test.
func TestView(t *testing.T) {
	a := NewApp()
	tt := ui.NewTester(a.View, 480, 360)
	tt.Frame()

	for _, want := range []string{"Remit", "No library open"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
