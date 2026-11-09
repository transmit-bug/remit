package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// The six areas are one key each, in rail order, and each key lands on that
// area's page: its own first page, with its own name in the path.
func TestAreaShortcutsSwitchAreas(t *testing.T) {
	msgs := locales.In("en")
	areas := []struct {
		key  ui.Key
		area string
		page string
		// want is what the area's first page shows.
		want string
	}{
		{ui.Key1, msgs.AreaToday(), "", msgs.TodayBudget()},
		{ui.Key2, msgs.AreaWrite(), msgs.ModeNotes(), msgs.Notebook()},
		{ui.Key3, msgs.AreaRead(), msgs.QueuePage(), msgs.ReadQueueEmpty()},
		{ui.Key4, msgs.AreaRecall(), msgs.QueuePage(), "Spacing beats cramming"},
		{ui.Key5, msgs.AreaStats(), msgs.OverviewPage(), msgs.StatsTitle()},
		{ui.Key6, msgs.AreaSettings(), msgs.AppearancePage(), msgs.SettingsAppearanceLead()},
	}
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)

			// A library opens on today, which is a page of its own.
			if !tt.HasText(msgs.TodayBudget()) {
				t.Fatalf("the window did not open on today; texts %q", tt.Texts())
			}

			for _, area := range areas {
				tt.Key(ui.Cmd, area.key)
				tt.Advance(320 * time.Millisecond) // the page's entrance
				if !tt.HasText(area.want) {
					t.Fatalf("Cmd+%v did not show %s: missing %q; texts %q", area.key, area.area, area.want, tt.Texts())
				}
				if got := a.railChoice(); string(got) != snake(area.area) && area.area != msgs.AreaToday() {
					t.Fatalf("the rail is on %q after Cmd+%v, want %q", got, area.key, area.area)
				}
				if area.page != "" && !tt.HasText(area.page) {
					t.Errorf("%s did not open on its %q page; texts %q", area.area, area.page, tt.Texts())
				}
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// snake is the id the rail holds for an area, from the label it shows: the
// mapping is the app's, and this is the test's own reading of it.
func snake(label string) string {
	return map[string]string{
		"Today": "today", "Write": "write", "Read": "read",
		"Recall": "recall", "Stats": "stats", "Settings": "settings",
	}[label]
}

// At the smallest window every control is on screen and clickable, not only
// present as text.
func TestMinimumSizeControls(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)

	// The frame fits: the rail's areas, the path and the page's own controls.
	for _, label := range []string{"Today", "Write", "Read", "Recall", "Stats", "Settings", "Search actions", "More"} {
		assertControl(t, tt, 720, 520, label)
	}

	// The recall queue the window opens on its key is usable at the smallest
	// size.
	recallQueue(tt)
	tt.Advance(320 * time.Millisecond)
	assertControl(t, tt, 720, 520, "Show answer")

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	for _, label := range []string{"New note", "Notebook", "A daily habit", "Note title"} {
		assertControl(t, tt, 720, 520, label)
	}

	// Making a card is a command now, reachable at the smallest window too.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("make card")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("Card made from block 1") {
		t.Fatalf("Make card did not run at the minimum; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // let the toast go

	recallQueue(tt)
	assertControl(t, tt, 720, 520, "Show answer")
	if err := tt.Click("Show answer"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Again", "Hard", "Good", "Easy", "Undo (U)", "Suspend (S)", "Edit note (E)"} {
		assertControl(t, tt, 720, 520, label)
	}
	if err := tt.Click("Good"); err != nil {
		t.Fatal(err)
	}
	assertControl(t, tt, 720, 520, "Show answer")

	tt.Key(ui.Cmd, ui.KeyK)
	assertControl(t, tt, 720, 520, "Search actions")
	assertControl(t, tt, 720, 520, "Edit notes")
	tt.Advance(320 * time.Millisecond) // the palette's arrival
	if err := tt.Click("Edit notes"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond) // the screen's change
	if tt.HasText("Review cards") {
		t.Error("the palette stayed open after its row was clicked")
	}
	if !tt.HasText("Notebook") {
		t.Error("the palette's action did not run")
	}
	tt.Advance(320 * time.Millisecond) // the card's transition

	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// The frame fits the smallest window: the rail's areas, the bar's controls and
// the page's, and the folded rail still switches areas by pointer.
func TestAppBarAtMinimumSize(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)

	msgs := locales.In("en")
	for _, label := range []string{
		Name, msgs.AreaToday(), msgs.AreaWrite(), msgs.AreaRead(), msgs.AreaRecall(),
		msgs.AreaStats(), msgs.AreaSettings(), msgs.More(), msgs.Path(),
	} {
		assertControl(t, tt, 720, 520, label)
	}

	// Folded, an area shows no label: the pointer finds it by its accessible
	// name, and choosing it still switches the area.
	for _, area := range []struct {
		name string
		want string
	}{
		{msgs.AreaRecall(), "Spacing beats cramming"},
		{msgs.AreaStats(), msgs.StatsTitle()},
		{msgs.AreaSettings(), msgs.SettingsAppearanceLead()},
		{msgs.AreaWrite(), msgs.Notebook()},
	} {
		info, ok := tt.Access(area.name)
		if !ok {
			t.Fatalf("%q is not on the rail; texts %q", area.name, tt.Texts())
		}
		tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
		tt.Advance(320 * time.Millisecond) // the page's entrance
		if !tt.HasText(area.want) {
			t.Fatalf("the rail did not switch to %s: missing %q; texts %q", area.name, area.want, tt.Texts())
		}
		if info.Bounds.X+info.Bounds.W > 720 || info.Bounds.Y+info.Bounds.H > 520 {
			t.Errorf("%q is outside the window: %+v", area.name, info.Bounds)
		}
	}

	// The document's own controls are in the bar when the notebook is up.
	for _, label := range []string{msgs.ViewEdit(), msgs.ViewSplit(), msgs.ViewRead(), msgs.PanelTitle(), msgs.More()} {
		assertControl(t, tt, 720, 520, label)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// One page in the dark: the same views draw from the dark theme.
func TestDark(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	tt.SetDark(true)
	tt.Frame()
	recallQueue(tt)
	if !tt.HasText("Show answer") {
		t.Fatalf("dark review mode is incomplete; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	if !tt.HasText("Notebook") || !tt.HasText("Note title") {
		t.Fatalf("dark notes mode is incomplete; texts %q", tt.Texts())
	}

	recallQueue(tt)
	tt.Key(0, ui.KeySpace)
	if !tt.HasText("reviewing just before you forget strengthens recall.") {
		t.Fatalf("dark review mode lost its content; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A library that cannot be opened is one line and one action, not a crash;
// the action opens it once the problem is gone.
func TestCorruptLibrary(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "remit.json")
	if err := os.WriteFile(config, []byte(`{"format": "remit/9"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	msgs := locales.In("en")
	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	if !tt.HasText("Remit could not open its library") || !tt.HasText("remit/9") {
		t.Fatalf("the error state does not say what happened; texts %q", tt.Texts())
	}
	if tt.HasText("Notebook") {
		t.Error("the window drew a library it could not open")
	}
	assertControl(t, tt, 1024, 720, "Try again")

	if err := os.WriteFile(config, []byte(`{"format": "remit/2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Try again"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(msgs.TodayBudget()) || tt.HasText("Remit could not open its library") {
		t.Fatalf("trying again did not open the repaired library; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
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

// Every message comes from a catalog, so switching the language switches what
// the view says, without a restart and without a second code path.
func TestLocaleSwitchesTheLanguage(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)

	tt.SetLocale("zh-Hans")
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key4)
	tt.Advance(320 * time.Millisecond) // the page's entrance
	for _, want := range []string{"显示答案", "撤销 (U)"} {
		if !tt.HasText(want) {
			t.Errorf("missing Chinese text %q; texts %q", want, tt.Texts())
		}
	}

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	for _, want := range []string{"写", "笔记本"} {
		if !tt.HasText(want) {
			t.Errorf("missing Chinese text %q in the notes; texts %q", want, tt.Texts())
		}
	}

	tt.SetLocale("en")
	tt.Frame()
	for _, want := range []string{"Write", "Notebook", "New note"} {
		if !tt.HasText(want) {
			t.Errorf("missing English text %q after switching back; texts %q", want, tt.Texts())
		}
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after a locale switch")
	}
}
