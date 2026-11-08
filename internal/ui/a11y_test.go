package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A grade changes the screen; it also has to be said, or a screen reader user
// never learns that the card left the queue.
func TestGradingAnnouncesWhatIsLeft(t *testing.T) {
	a, _, tt := openApp(t)

	tt.Key(ui.Cmd, ui.Key1)
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key3)

	got := tt.Announcements()
	if len(got) == 0 {
		t.Fatal("grading announced nothing")
	}
	if want := a.review.Remaining(); !containsSubstring(got, "left") {
		t.Errorf("announcements %q do not say what is left (%d cards)", got, want)
	}
}

func containsSubstring(list []string, want string) bool {
	for _, s := range list {
		if len(s) >= len(want) {
			for i := 0; i+len(want) <= len(s); i++ {
				if s[i:i+len(want)] == want {
					return true
				}
			}
		}
	}
	return false
}
