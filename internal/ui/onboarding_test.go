package ui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/locales"
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
	settle(tt)

	if !tt.HasText("Open library…") {
		t.Fatalf("the start view offers no way to open one; texts %q", tt.Texts())
	}
	if err := tt.Click("Open library…"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("the picker ran %d times", calls)
	}
	if !tt.HasText(locales.In("en").TodayBudget()) {
		t.Fatalf("the picked library did not open on today; texts %q", tt.Texts())
	}
	// The queue it found is on the rail, one key away.
	recallQueue(tt)
	for _, want := range []string{"Show answer", "Spacing beats cramming", "4 cards left"} {
		if !tt.HasText(want) {
			t.Fatalf("the recall queue lost %q; texts %q", want, tt.Texts())
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
	settle(tt)
	if !tt.HasText(locales.In("en").TodayBudget()) || tt.HasText("Start your library") {
		t.Fatalf("the remembered library did not open; texts %q", tt.Texts())
	}
	if calls != 0 {
		t.Error("the app asked for a folder although it remembered one")
	}
	tt.Advance(320 * time.Millisecond) // the opening card's entry
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
	settle(tt)

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

// With no settings and no library named, the app still names a place for a
// library and hands it to the start view: Remit's own folder in Documents.
func TestStartViewWithNoSetting(t *testing.T) {
	a := NewApp(Options{DataDir: t.TempDir()})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	if a.startDir() == "" {
		t.Fatal("the app names no place for a library")
	}
	if !tt.HasText(a.startDir()) {
		t.Errorf("the window does not show %q; texts %q", a.startDir(), tt.Texts())
	}
}

// A first run has no library: the window asks whether to make one or open one,
// and only makes the one the user chose. The choice is remembered, so the next
// start opens the library without asking again.
func TestFirstRun(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Remit")
	data := t.TempDir()
	a := NewApp(Options{LibraryDir: dir, DataDir: data})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	if _, err := os.Stat(filepath.Join(dir, "remit.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the window made a library before it was asked: %v", err)
	}
	if tt.HasText("Notebook") {
		t.Error("the start view drew a library that is not there")
	}

	if err := tt.Click("Create library"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "remit.json")); err != nil {
		t.Fatalf("Create library did not make one: %v", err)
	}
	if !tt.HasText(locales.In("en").TodayBudget()) {
		t.Fatalf("a new library did not open on today; texts %q", tt.Texts())
	}
	// Its queue is empty, and the queue page says so, with what tomorrow holds.
	recallQueue(tt)
	for _, want := range []string{locales.In("en").RecallEmpty(), locales.In("en").RecallStartEmpty()} {
		if !tt.HasText(want) {
			t.Fatalf("a new library's queue is not empty and quiet: missing %q; texts %q", want, tt.Texts())
		}
	}

	// The choice is in the settings for the next start.
	s, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.LibraryDir != dir {
		t.Fatalf("the settings name %q, want %q", s.LibraryDir, dir)
	}

	// The next start opens the remembered library, not the start view.
	next := NewApp(Options{DataDir: data})
	tt = ui.NewTester(next.View, 1024, 720)
	tt.Frame()
	if tt.HasText("Start your library") {
		t.Fatalf("the remembered library was not opened; texts %q", tt.Texts())
	}
	if !tt.HasText(locales.In("en").TodayBudget()) {
		t.Fatalf("the remembered library did not open on today; texts %q", tt.Texts())
	}

	// The notes page says it is empty.
	tt.Key(ui.Cmd, ui.Key2)
	tt.Frame()
	if !tt.HasText("No notes yet.") {
		t.Fatalf("the empty library does not say it is empty; texts %q", tt.Texts())
	}
	if err := tt.Click("New note"); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused("Note title") {
		t.Fatalf("the first note did not open for typing; texts %q", tt.Texts())
	}

	third := NewApp(Options{DataDir: data})
	tt = ui.NewTester(third.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	if !tt.HasText("Untitled note") {
		t.Fatalf("the new note did not survive a restart; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Opening a library remembers it: the welcome screen offers it again, and one
// whose folder is gone is dropped from that list with the reason inside the
// card rather than a click that appears to do nothing.
func TestRecentLibraryThatIsGoneSaysWhy(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	data := t.TempDir()
	empty := filepath.Join(t.TempDir(), "Remit")

	first := NewApp(Options{
		LibraryDir:    empty,
		DataDir:       data,
		ChooseLibrary: func() (string, error) { return dir, nil },
	})
	tt := ui.NewTester(first.View, 1024, 720)
	settle(tt)
	if err := tt.Click("Open library…"); err != nil {
		t.Fatal(err)
	}
	s, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.RecentLibraries) != 1 || s.RecentLibraries[0] != dir {
		t.Fatalf("opening a library remembered %q, want %q", s.RecentLibraries, dir)
	}

	// The folder goes away between runs: the next start has no library, and the
	// list it offers says the one it remembers is not there.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	second := NewApp(Options{DataDir: data})
	tt = ui.NewTester(second.View, 1024, 720)
	settle(tt)
	if !tt.HasText(msgs.StartRecent()) {
		t.Fatalf("the welcome screen does not offer the library it opened; texts %q", tt.Texts())
	}
	if !tt.HasText(dir) || !tt.HasText(msgs.StartRecentMissing()) {
		t.Fatalf("a gone library is not marked as gone; texts %q", tt.Texts())
	}

	// Opening it fails, and the card says why: the path, the reason and the way
	// out. The path is dropped from the list it was offered in.
	if err := tt.Click(msgs.StartRecentOpen(dir)); err != nil {
		t.Fatalf("the recent row is not a control: %v", err)
	}
	tt.Frame()
	said := false
	for _, text := range tt.Texts() {
		said = said || strings.HasPrefix(text, "Remit could not open "+dir)
	}
	if !said {
		t.Fatalf("the card does not say why %q will not open; texts %q", dir, tt.Texts())
	}
	if !tt.HasText(msgs.StartChooseAnother()) {
		t.Errorf("the failure offers no way out; texts %q", tt.Texts())
	}
	if tt.HasText(msgs.StartRecentMissing()) {
		t.Errorf("the path that will not open is still offered; texts %q", tt.Texts())
	}
	s, err = settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.RecentLibraries) != 0 {
		t.Errorf("a library that is gone stayed in the list: %q", s.RecentLibraries)
	}

	// The next start offers no recent section at all.
	third := NewApp(Options{DataDir: data})
	tt = ui.NewTester(third.View, 1024, 720)
	settle(tt)
	if _, ok := tt.Find(msgs.StartRecent()); ok {
		t.Errorf("an empty list still has a heading; texts %q", tt.Texts())
	}
}
