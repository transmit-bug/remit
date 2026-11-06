package ui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/settings"
)

// The start view's other action opens a library already on disk, through the
// folder picker: the picked library becomes the one open, and the one the next
// start opens.
func TestOpenLibraryPicker(t *testing.T) {
	picked := t.TempDir()
	seedLibrary(t, picked)
	data := t.TempDir()
	empty := filepath.Join(t.TempDir(), "Remit")

	calls := 0
	a := NewApp(Options{
		LibraryDir: empty,
		DataDir:    data,
		ChooseLibrary: func() (string, error) {
			calls++
			return picked, nil
		},
	})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()

	if !tt.HasText("Open library…") {
		t.Fatalf("the start view offers no way to open one; texts %q", tt.Texts())
	}
	if err := tt.Click("Open library…"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("the picker ran %d times", calls)
	}
	for _, want := range []string{"Show answer", "Spacing beats cramming", "4 cards left"} {
		if !tt.HasText(want) {
			t.Fatalf("the picked library did not open on its queue: missing %q; texts %q", want, tt.Texts())
		}
	}
	// Opening another library is not a reason to make the default one too.
	if _, err := os.Stat(filepath.Join(empty, "remit.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the start view's place became a library anyway: %v", err)
	}
	s, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.LibraryDir != picked {
		t.Fatalf("the settings name %q, want %q", s.LibraryDir, picked)
	}

	// The next start opens the remembered library without asking.
	calls = 0
	next := NewApp(Options{
		DataDir: data,
		ChooseLibrary: func() (string, error) {
			calls++
			return "", nil
		},
	})
	tt = ui.NewTester(next.View, 1024, 720)
	tt.Frame()
	if !tt.HasText("4 cards left") || tt.HasText("Start your library") {
		t.Fatalf("the remembered library did not open; texts %q", tt.Texts())
	}
	if calls != 0 {
		t.Error("the app asked for a folder although it remembered one")
	}
	tt.Advance(200 * time.Millisecond) // the opening card's entry
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A canceled folder picker changes nothing: the window stays where it was.
func TestOpenLibraryPickerCancelled(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "Remit")
	a := NewApp(Options{
		LibraryDir:    empty,
		ChooseLibrary: func() (string, error) { return "", nil },
	})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()

	if err := tt.Click("Open library…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Start your library") {
		t.Fatalf("a canceled picker left the start view; texts %q", tt.Texts())
	}
	if _, err := os.Stat(filepath.Join(empty, "remit.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a canceled picker made a library: %v", err)
	}
}

// With no settings and no library named, the window still asks rather than
// guessing: the start view names Remit's own place in Documents.
func TestStartViewWithNoSetting(t *testing.T) {
	a := NewApp(Options{DataDir: t.TempDir()})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()

	if !tt.HasText("Start your library") {
		t.Fatalf("no library and no setting: texts %q", tt.Texts())
	}
	if a.startDir() == "" {
		t.Error("the start view names no place for a library")
	}
	if !tt.HasText(a.startDir()) {
		t.Errorf("the start view does not show %q; texts %q", a.startDir(), tt.Texts())
	}
}

// The start view in Chinese holds at the smallest window.
func TestStartViewAtMinimumInChinese(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Remit")
	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	for _, label := range []string{"从笔记库开始", "新建笔记库", "打开笔记库…"} {
		assertControl(t, tt, 720, 520, label)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after the locale switch")
	}
}
