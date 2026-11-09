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
	imports int
	opened  []string
}

func (f *fake) deps() start.Deps {
	return start.Deps{
		Dir:    func() string { return f.dir },
		Create: func(*ui.Context) { f.created++ },
		Choose: func(*ui.Context) { f.chosen++ },
		Import: func(*ui.Context) { f.imports++ },
		OpenAt: func(_ *ui.Context, dir string) { f.opened = append(f.opened, dir) },
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

// shown is the screen with a place to name and the ways in.
func shown(t *testing.T, state start.State) (*fake, *ui.Tester) {
	t.Helper()
	f := &fake{dir: "/home/you/Documents/Remit"}
	deps := f.deps()
	return f, at(t, func(c *ui.Context) { start.View(c, &state, deps) }, 1024, 720)
}

// With no library open the screen asks where to put one, at both the sizes the
// window opens at, and it names the folder a library would go in.
func TestNoLibraryYet(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f := &fake{dir: "/home/you/Documents/Remit"}
			deps := f.deps()
			var state start.State
			tt := at(t, func(c *ui.Context) { start.View(c, &state, deps) }, size.w, size.h)

			want := []string{msgs.StartTitle(), msgs.StartBody(), msgs.CreateLibrary(), msgs.OpenLibrary(), f.dir}
			for _, text := range want {
				if !tt.HasText(text) {
					t.Fatalf("missing %q; texts %q", text, tt.Texts())
				}
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// The place the frame shows is the app's: the screen asks the app for it rather
// than drawing a folder it made up.
func TestPlaceComesFromTheApp(t *testing.T) {
	f := &fake{dir: "/home/you/Documents/Remit"}
	deps := f.deps()
	var state start.State
	tt := at(t, func(c *ui.Context) { start.View(c, &state, deps) }, 1024, 720)
	if !tt.HasText(f.dir) {
		t.Fatalf("the screen did not ask the app where a library would go; texts %q", tt.Texts())
	}
	if !tt.HasText(locales.In("en").CreateLibrary()) {
		t.Errorf("a place asked for cost the screen its actions; texts %q", tt.Texts())
	}
}

// Nothing is created by looking: the screen reports the click and the app does
// the writing.
func TestActionsGoToTheApp(t *testing.T) {
	msgs := locales.In("en")
	f, tt := shown(t, start.State{})

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

// The welcome offers three doors — a new library, one already on disk, and an
// import — and each one reaches the app. A missing door is a way in the user
// does not have.
func TestWelcomeOffersThreeDoors(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f := &fake{dir: "/home/you/Documents/Remit"}
			deps := f.deps()
			var state start.State
			tt := at(t, func(c *ui.Context) { start.View(c, &state, deps) }, size.w, size.h)

			doors := []struct {
				label string
				hint  string
				times func() int
			}{
				{msgs.CreateLibrary(), msgs.StartCreateHint(), func() int { return f.created }},
				{msgs.OpenLibrary(), msgs.StartOpenHint(), func() int { return f.chosen }},
				{msgs.StartImport(), msgs.StartImportHint(), func() int { return f.imports }},
			}
			for _, door := range doors {
				box, ok := tt.Find(door.label)
				if !ok || box.W <= 0 || box.H <= 0 {
					t.Fatalf("the %q door has no box; texts %q", door.label, tt.Texts())
				}
				if !tt.HasText(door.hint) {
					t.Errorf("the %q door does not say where it leads; texts %q", door.label, tt.Texts())
				}
				if err := tt.Click(door.label); err != nil {
					t.Fatalf("click %q: %v", door.label, err)
				}
				if got := door.times(); got != 1 {
					t.Errorf("the %q door reached the app %d times, want 1", door.label, got)
				}
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// A library that will not open says why inside the card — the path, the reason
// and the door out — instead of a click that appears to have done nothing.
func TestWelcomeShowsWhyALibraryFailed(t *testing.T) {
	msgs := locales.In("en")
	dir := "/home/you/Documents/Gone"
	state := start.State{Failed: &start.Failure{Dir: dir, Err: "no such file or directory"}}
	f, tt := shown(t, state)

	want := msgs.StartFailed(dir, "no such file or directory")
	if !tt.HasText(want) {
		t.Fatalf("the card does not say why %q will not open; texts %q", dir, tt.Texts())
	}
	if !tt.HasText(dir) {
		t.Errorf("the callout does not name the path; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.StartChooseAnother()) {
		t.Fatalf("the callout offers no way out; texts %q", tt.Texts())
	}
	// The three doors stay where they were: a failed library is a callout, not
	// a page that took the ways in away.
	for _, label := range []string{msgs.CreateLibrary(), msgs.OpenLibrary(), msgs.StartImport()} {
		if !tt.HasText(label) {
			t.Errorf("a failure removed the %q door; texts %q", label, tt.Texts())
		}
	}
	if err := tt.Click(msgs.StartChooseAnother()); err != nil {
		t.Fatal(err)
	}
	if f.chosen != 1 {
		t.Errorf("Choose another reached the picker %d times, want 1", f.chosen)
	}
}

// The libraries the app opened before are one click each, and a folder that is
// gone says so on its row rather than looking like any other.
func TestRecentLibrariesAreOneClick(t *testing.T) {
	msgs := locales.In("en")
	state := start.State{Recent: []start.Recent{
		{Dir: "/home/you/Notes"},
		{Dir: "/home/you/Old", Missing: true},
	}}
	f, tt := shown(t, state)

	if !tt.HasText(msgs.StartRecent()) {
		t.Fatalf("the card has no recent section; texts %q", tt.Texts())
	}
	for _, dir := range []string{"/home/you/Notes", "/home/you/Old"} {
		if !tt.HasText(dir) {
			t.Errorf("the recent list is missing %q; texts %q", dir, tt.Texts())
		}
	}
	if !tt.HasText(msgs.StartRecentMissing()) {
		t.Errorf("a missing folder is not marked; texts %q", tt.Texts())
	}
	if err := tt.Click("/home/you/Notes"); err != nil {
		t.Fatalf("the recent row is not a control: %v", err)
	}
	if len(f.opened) != 1 || f.opened[0] != "/home/you/Notes" {
		t.Errorf("the recent row opened %q, want the library it names", f.opened)
	}
}

// A screen with nothing to remember draws no recent section at all, rather than
// an empty heading.
func TestNoRecentDrawsNoSection(t *testing.T) {
	msgs := locales.In("en")
	_, tt := shown(t, start.State{})
	if tt.HasText(msgs.StartRecent()) {
		t.Errorf("the recent heading is drawn over an empty list; texts %q", tt.Texts())
	}
}

// The smallest window in Chinese still shows the three doors, the list of
// libraries opened before and the reason one will not open: a screen that only
// works at one size and in one language is not a screen.
func TestWelcomeInChineseAtTheMinimumSize(t *testing.T) {
	msgs := locales.In("zh-Hans")
	f := &fake{dir: "/home/you/Documents/Remit"}
	deps := f.deps()
	state := start.State{
		Recent: []start.Recent{{Dir: "/home/you/Notes", Missing: true}},
		Failed: &start.Failure{Dir: "/home/you/Gone", Err: "no such file or directory"},
	}
	tt := ui.NewTester(func(c *ui.Context) { start.View(c, &state, deps) }, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)

	for _, want := range []string{
		msgs.CreateLibrary(), msgs.OpenLibrary(), msgs.StartImport(),
		msgs.StartRecent(), msgs.StartRecentMissing(), msgs.StartChooseAnother(),
	} {
		if !tt.HasText(want) {
			t.Errorf("the Chinese welcome is missing %q; texts %q", want, tt.Texts())
		}
	}
	for _, label := range []string{msgs.CreateLibrary(), msgs.OpenLibrary(), msgs.StartImport()} {
		if err := tt.Click(label); err != nil {
			t.Errorf("the %q door is not reachable at 720×520 in Chinese: %v", label, err)
		}
	}
	if f.created != 1 || f.chosen != 1 || f.imports != 1 {
		t.Errorf("the doors reached the app %d/%d/%d times", f.created, f.chosen, f.imports)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
