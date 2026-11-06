package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Chinese labels are wider than the English keys they replace; at the smallest
// window every one of them still has to sit inside the window.
func TestChineseFitsTheSmallestWindow(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	inside := func(label string) {
		t.Helper()
		box, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %q; texts %q", label, tt.Texts())
		}
		if box.X < 0 || box.Y < 0 || box.X+box.W > 720 || box.Y+box.H > 520 {
			t.Errorf("%q box %+v is outside the 720x520 window", label, box)
		}
	}
	for _, label := range []string{"复习", "新建笔记", "显示答案", "编辑笔记 (E)"} {
		inside(label)
	}
	tt.Advance(200 * time.Millisecond) // the card's entry

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(200 * time.Millisecond) // the card's exit
	for _, label := range []string{"笔记", "笔记本", "第 1 块 · A daily habit"} {
		inside(label)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after the locale switch")
	}
}

// A grade changes the screen; it also has to be said, or a screen reader user
// never learns that the card left the queue.
func TestGradingAnnouncesWhatIsLeft(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

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

// The whole review flow is reachable from the keyboard: enter, reveal, grade,
// undo, leave.
func TestReviewIsKeyboardOnly(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	tt.Key(ui.Cmd, ui.Key1)
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key3)
	tt.Key(0, ui.KeyU)
	tt.Key(0, ui.KeyEscape)
	if !tt.HasText("Notebook") {
		t.Errorf("Escape did not return to notes; texts %q", tt.Texts())
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
