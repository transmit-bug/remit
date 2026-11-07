package start_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/screens/start"
	"github.com/transmit-bug/remit/locales"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// fake is the app the screen talks to: the place it names and the calls the
// screen made, with no library and no dialog behind it.
type fake struct {
	dir     string
	created int
	chosen  int
}

func (f *fake) deps() start.Deps {
	return start.Deps{
		Dir:    func() string { return f.dir },
		Create: func(*ui.Context) { f.created++ },
		Choose: func(*ui.Context) { f.chosen++ },
	}
}

// at draws the screen once at a size and lets it settle.
func at(t *testing.T, view func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(view, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// shown is the screen with a place to name and the two actions.
func shown(t *testing.T, state start.State) (*fake, *ui.Tester) {
	t.Helper()
	f := &fake{dir: "/home/you/Documents/Remit"}
	deps := f.deps()
	return f, at(t, func(c *ui.Context) { start.View(c, state, deps) }, 1024, 720)
}

// With no library open the screen asks where to put one, at both the sizes the
// window opens at, and it names the folder a library would go in.
func TestNoLibraryYet(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f := &fake{dir: "/home/you/Documents/Remit"}
			deps := f.deps()
			tt := at(t, func(c *ui.Context) { start.View(c, start.State{Dir: f.dir}, deps) }, size.w, size.h)

			want := []string{msgs.StartTitle(), msgs.StartBody(), msgs.CreateLibrary(), msgs.OpenLibrary(), f.dir}
			for _, text := range want {
				if !tt.HasText(text) {
					t.Fatalf("missing %q; texts %q", text, tt.Texts())
				}
			}
			for _, label := range []string{msgs.CreateLibrary(), msgs.OpenLibrary()} {
				box, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%q has no box; texts %q", label, tt.Texts())
				}
				if box.W <= 0 || box.H <= 0 {
					t.Errorf("%q has an empty box %.1fx%.1f", label, box.W, box.H)
				}
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// The place the frame shows is the app's; when the frame names none, the screen
// asks the app for one rather than drawing a folder it made up.
func TestPlaceComesFromTheApp(t *testing.T) {
	msgs := locales.In("en")
	_, tt := shown(t, start.State{Dir: "/home/you/Notes"})
	if !tt.HasText("/home/you/Notes") {
		t.Fatalf("the app's place is not on screen; texts %q", tt.Texts())
	}
	if tt.HasText("/home/you/Documents/Remit") {
		t.Errorf("the screen named a second place; texts %q", tt.Texts())
	}

	f := &fake{dir: "/home/you/Documents/Remit"}
	deps := f.deps()
	tt = at(t, func(c *ui.Context) { start.View(c, start.State{}, deps) }, 1024, 720)
	if !tt.HasText(f.dir) {
		t.Fatalf("the screen did not ask the app where a library would go; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.CreateLibrary()) {
		t.Errorf("a place asked for cost the screen its actions; texts %q", tt.Texts())
	}
}

// Nothing is created by looking: the screen reports the click and the app does
// the writing.
func TestActionsGoToTheApp(t *testing.T) {
	msgs := locales.In("en")
	f, tt := shown(t, start.State{Dir: "/home/you/Documents/Remit"})

	if err := tt.Click(msgs.CreateLibrary()); err != nil {
		t.Fatal(err)
	}
	if f.created != 1 || f.chosen != 0 {
		t.Fatalf("Create library reached the app %d times, %d for open", f.created, f.chosen)
	}
	if err := tt.Click(msgs.OpenLibrary()); err != nil {
		t.Fatal(err)
	}
	if f.chosen != 1 {
		t.Errorf("Open library… reached the app %d times, want 1", f.chosen)
	}
}

// In Chinese the screen holds at the smallest window: the title, the folder and
// both actions keep a box of their own.
func TestAtMinimumInChinese(t *testing.T) {
	msgs := locales.In("zh-Hans")
	f := &fake{dir: "/home/you/Documents/Remit"}
	deps := f.deps()
	tt := ui.NewTester(func(c *ui.Context) {
		start.View(c, start.State{Dir: f.dir}, deps)
	}, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	for _, text := range []string{msgs.StartTitle(), msgs.CreateLibrary(), msgs.OpenLibrary(), f.dir} {
		box, ok := tt.Find(text)
		if !ok {
			t.Fatalf("%q has no box; texts %q", text, tt.Texts())
		}
		if box.X < 0 || box.Y < 0 || box.X+box.W > 720 || box.Y+box.H > 520 {
			t.Errorf("%q is outside the window: %.1f,%.1f %.1fx%.1f", text, box.X, box.Y, box.W, box.H)
		}
	}
	tt.Advance(300 * time.Millisecond)
	if tt.FramePending() {
		t.Error("view still requests frames after the locale switch")
	}
}
