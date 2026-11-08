package ui

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
)

// reportContributes is the manifest a test's plugin declares: one command in the
// palette and the window's overflow menu, one page, two settings and its own
// strings in the two languages the app speaks.
const reportContributes = `{
  "commands": [
    {
      "id": "acme.report.show",
      "title": "%command.show%",
      "category": "%category.report%",
      "icon": "chart-column",
      "when": "library.open",
      "action": "command:show"
    }
  ],
  "menus": {
    "toolbar.overflow": [
      {"command": "acme.report.show", "group": "plugin", "order": 10}
    ]
  },
  "pages": [
    {
      "id": "report",
      "title": "%page.title%",
      "icon": "chart-column",
      "nav": "plugins",
      "when": "library.open",
      "data": "command:data"
    }
  ],
  "settings": [
    {"key": "threshold", "type": "number", "title": "%setting.threshold%", "default": 8},
    {"key": "includeSuspended", "type": "bool", "title": "%setting.suspended%", "default": false}
  ],
  "locales": {
    "en": {
      "command.show": "Leech report",
      "category.report": "Reports",
      "page.title": "Leeches",
      "setting.threshold": "Lapses that make a leech",
      "setting.suspended": "Count suspended cards",
      "section.overview": "Overview",
      "stat.notes": "notes",
      "stat.cards": "cards",
      "tab.first": "Summary",
      "tab.second": "Rows",
      "action.run": "Run the report",
      "toast.done": "Report done",
      "table.note": "Note",
      "table.cards": "Cards",
      "markdown.body": "**Three** notes make four cards.",
      "empty": "Nothing here"
    },
    "zh-Hans": {
      "command.show": "水蛭报告",
      "category.report": "报告",
      "page.title": "水蛭",
      "setting.threshold": "水蛭阈值",
      "setting.suspended": "把已暂停的卡片也算进来",
      "section.overview": "概览",
      "stat.notes": "篇笔记",
      "stat.cards": "张卡片",
      "tab.first": "概要",
      "tab.second": "明细",
      "action.run": "运行报告",
      "toast.done": "报告已生成",
      "table.note": "笔记",
      "table.cards": "卡片",
      "markdown.body": "**三**篇笔记做出四张卡片。",
      "empty": "这里什么都没有"
    }
  }
}`

// reportTree is what the page's data command answers: every layout node and a
// spread of leaves, one node this host does not know, and a table.
const reportTree = `{
  "kind": "page",
  "children": [
    {
      "kind": "section",
      "title": "%section.overview%",
      "children": [
        {
          "kind": "stack",
          "direction": "row",
          "gap": "m",
          "children": [
            {"kind": "stat", "label": "%stat.notes%", "value": "3"},
            {"kind": "stat", "label": "%stat.cards%", "value": "4"}
          ]
        },
        {
          "kind": "grid",
          "columns": 2,
          "children": [
            {"kind": "text", "text": "left pane"},
            {"kind": "table", "columns": ["%table.note%", "%table.cards%"], "rows": [["Memory and recall", "4"]]}
          ]
        },
        {"kind": "markdown", "text": "%markdown.body%"},
        {
          "kind": "tabs",
          "tabs": [
            {"label": "%tab.first%", "children": [{"kind": "text", "text": "first tab"}]},
            {"label": "%tab.second%", "children": [{"kind": "list", "items": [{"label": "row one", "value": "1"}]}]}
          ]
        },
        {"kind": "actions", "items": [{"command": "acme.report.show", "label": "%action.run%"}]},
        {"kind": "sparkline"}
      ]
    }
  ]
}`

// fakeRunner answers what a plugin's commands and page data answer, and counts
// how often it was entered: a test asserts what a frame did not do.
type fakeRunner struct {
	mu     sync.Mutex
	names  []string
	reply  map[string]string // callback name -> its JSON answer
	failed map[string]error  // callback name -> the error it throws
	gate   chan struct{}     // when set, Call waits for it before answering
}

func (f *fakeRunner) Call(method string, params, out any) error {
	input, _ := params.(map[string]any)
	name, _ := input["name"].(string)

	f.mu.Lock()
	f.names = append(f.names, name)
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failed[name]; err != nil {
		return err
	}
	// A contributed command's params name the callback; a hook the app calls
	// directly (importFile) has no such field, so its reply is keyed by the
	// method name.
	raw, ok := f.reply[name]
	if !ok {
		raw, ok = f.reply[method]
	}
	if !ok || out == nil {
		return nil
	}
	return json.Unmarshal([]byte(raw), out)
}

// setFailure makes a callback answer or throw, as the plugin's next run does.
func (f *fakeRunner) setFailure(name string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failed == nil {
		f.failed = map[string]error{}
	}
	if err == nil {
		delete(f.failed, name)
		return
	}
	f.failed[name] = err
}

// calls is how many times the plugin was entered.
func (f *fakeRunner) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.names)
}

// Identity and Revision are what a producer stamp reads off a runner, the
// same surface ext.JSRunner carries.
func (f *fakeRunner) Identity() string { return "acme.obsidian" }
func (f *fakeRunner) Revision() string { return "1" }

// enablePlugin approves a plugin in the library at dir, before the app opens it.
func enablePlugin(t *testing.T, dir, name string) {
	t.Helper()
	lib, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	defer lib.Close()
	if err := lib.SetEnabledExtensions([]string{name}); err != nil {
		t.Fatalf("enable %q: %v", name, err)
	}
}

// contribApp opens the window on a library with one plugin enabled that declares
// contributes, and hands the app a runner the test controls. The background is
// the test's own: a job and its answer run inline, where there is one thread.
func contribApp(t *testing.T, name, contributes string, runner *fakeRunner) (*App, testLibrary) {
	t.Helper()
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	enablePlugin(t, dir, name)
	manifest := ext.Manifest{
		ID:          "acme." + name,
		Name:        name,
		Version:     "1",
		APIVersion:  1,
		Runtime:     "js",
		Entry:       "main.js",
		Contributes: json.RawMessage(contributes),
	}
	if runner == nil {
		runner = &fakeRunner{}
	}
	if runner.reply == nil {
		runner.reply = map[string]string{}
	}
	if _, ok := runner.reply["data"]; !ok {
		runner.reply["data"] = reportTree
	}
	a := NewApp(Options{
		LibraryDir: dir,
		Manifests: func() ([]ext.Extension, error) {
			return []ext.Extension{{Manifest: manifest}}, nil
		},
		Discover: func() ([]Plugin, error) {
			return []Plugin{{Name: name, ID: manifest.ID, Version: "1"}}, nil
		},
		Contribution: func(string) (CommandRunner, error) { return runner, nil },
	})
	a.runBackground = func(c *ui.Context, job func() func(*ui.Context)) { job()(c) }
	return a, seed
}

// openReportPage shows the notes, then opens the page from the navigation: the
// way a reader reaches a contributed page, in the language the window speaks.
func openReportPage(t *testing.T, tt *ui.Tester) {
	t.Helper()
	openPage(t, tt, "Plugin pages", "Leeches")
}

// openPage reaches one contributed page from the window's navigation.
func openPage(t *testing.T, tt *ui.Tester, nav, page string) {
	t.Helper()
	tt.Key(ui.Cmd, ui.Key2) // notes: the toolbar is up outside review
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click(nav); err != nil {
		t.Fatalf("the plugin pages button: %v", err)
	}
	if err := tt.ChooseMenuItem(page); err != nil {
		t.Fatalf("the page's menu item: %v", err)
	}
}

// A contributed command is in the palette and in the menu slot it declared, and
// withdrawing the plugin takes it out of both: the declarations are the only
// door, and disabling removes them from every receiver.
func TestContributedCommandLivesInThePaletteAndItsSlot(t *testing.T) {
	a, _ := contribApp(t, "report", reportContributes, &fakeRunner{})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	if !tt.HasText("Leech report") {
		t.Fatalf("the palette does not list the plugin's command; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)

	if err := tt.Click("More"); err != nil {
		t.Fatalf("the window's overflow menu: %v", err)
	}
	menu := strings.Join(tt.Menu(), "\n")
	if !strings.Contains(menu, "Leech report") {
		t.Fatalf("the overflow menu does not hold the slot's item: %q", tt.Menu())
	}
	tt.CloseMenu()

	// Withdrawing the plugin is the settings page's switch, and the palette and
	// the menu lose the command with it.
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("Enable report") {
		t.Fatalf("the settings page does not list the plugin; texts %q", tt.Texts())
	}
	// The plugins section is below the fold at the window's default size.
	reveal(t, tt, 1024, 720, "Enable report")
	if err := tt.Click("Enable report"); err != nil {
		t.Fatalf("the plugin's switch: %v", err)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(320 * time.Millisecond)

	tt.Key(ui.Cmd, ui.KeyK)
	if tt.HasText("Leech report") {
		t.Fatal("the palette still lists a withdrawn plugin's command")
	}
	tt.Key(0, ui.KeyEscape)
	if err := tt.Click("More"); err != nil {
		t.Fatalf("the window's overflow menu: %v", err)
	}
	if strings.Contains(strings.Join(tt.Menu(), "\n"), "Leech report") {
		t.Error("the overflow menu still holds a withdrawn plugin's item")
	}
	tt.CloseMenu()
	if _, ok := tt.Find("Plugin pages"); ok {
		t.Error("the plugin's page is still in navigation after it was withdrawn")
	}
}

// Running a contributed command runs the plugin's callback off the frame, and
// the host applies what it answers: a toast here, a page there.
func TestContributedCommandAppliesItsAnswer(t *testing.T) {
	runner := &fakeRunner{reply: map[string]string{"show": `{"result":"toast","message":"%toast.done%"}`}}
	a, _ := contribApp(t, "report", reportContributes, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("leech")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("Report done") {
		t.Fatalf("the command's toast did not arrive; texts %q", tt.Texts())
	}

	// A navigate result opens the page it names.
	runner.reply["show"] = `{"result":"navigate","page":"report"}`
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("leech")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("Overview") || !tt.HasText("Leeches") {
		t.Fatalf("navigate did not open the page; texts %q", tt.Texts())
	}
}

// A command that fails toasts what went wrong and the window goes on; the third
// failure hides that command, and only that command.
func TestAFailingCommandToastsThenHidesItself(t *testing.T) {
	runner := &fakeRunner{failed: map[string]error{"show": errThrown}}
	a, _ := contribApp(t, "report", reportContributes, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	run := func() {
		tt.Key(ui.Cmd, ui.KeyK)
		tt.Type("leech")
		tt.Key(0, ui.KeyEnter)
	}
	run()
	if !tt.HasText("Leech report failed: the plugin threw") {
		t.Fatalf("a failing command said nothing; texts %q", tt.Texts())
	}
	// The window is still a window: its own commands are there and work.
	if !tt.HasText("New note") {
		t.Errorf("the failure took the window's own actions with it; texts %q", tt.Texts())
	}
	run()
	run()
	tt.Advance(6 * time.Second) // the last toast goes
	tt.Key(ui.Cmd, ui.KeyK)
	if tt.HasText("Leech report") {
		t.Errorf("the command is still listed after three failures; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)

	// Its page is untouched: one contribution's failures are its own.
	if _, ok := tt.Find("Plugin pages"); !ok {
		t.Error("a command's failures hid the plugin's page")
	}

	// The settings page counts the failures and offers the one action that
	// clears them, and the command is back.
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Advance(320 * time.Millisecond)
	reveal(t, tt, 1024, 720, "Re-enable")
	if !tt.HasText("3 contributed entries failed") {
		t.Fatalf("the settings page does not count the failures; texts %q", tt.Texts())
	}
	if err := tt.Click("Re-enable"); err != nil {
		t.Fatalf("the re-enable action: %v", err)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(320 * time.Millisecond)
	tt.Advance(6 * time.Second) // the re-enable toast goes
	tt.Key(ui.Cmd, ui.KeyK)
	if !tt.HasText("Leech report") {
		t.Errorf("re-enabling did not show the command again; texts %q", tt.Texts())
	}
}

// errThrown is a plugin that threw, as the runner reports it.
var errThrown = errorText("the plugin threw")

type errorText string

func (e errorText) Error() string { return string(e) }

// A page is the host's: the layout nodes lay out, the leaves are the host's
// widgets, and the grid collapses to one column at the smallest window.
func TestContributedPageDrawsAtBothSizes(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := contribApp(t, "report", reportContributes, &fakeRunner{})
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			openReportPage(t, tt)

			for _, want := range []string{"Leeches", "Overview", "3", "notes", "Note", "Memory and recall", "Summary", "Rows", "Run the report"} {
				if !tt.HasText(want) {
					t.Fatalf("the page is missing %q; texts %q", want, tt.Texts())
				}
			}
			// The markdown leaf is the host's renderer, not a pre-split text.
			if !tt.HasText("notes make four cards") {
				t.Errorf("the markdown leaf did not render; texts %q", tt.Texts())
			}
			// An unknown kind keeps its place and says what it was.
			if !tt.HasText("sparkline is not something this version of Remit draws.") {
				t.Errorf("the unknown node drew nothing; texts %q", tt.Texts())
			}

			left, ok := tt.Find("left pane")
			if !ok {
				t.Fatalf("the grid's first cell has no box; texts %q", tt.Texts())
			}
			note, ok := tt.Find("Note")
			if !ok {
				t.Fatalf("the table has no header; texts %q", tt.Texts())
			}
			// The page lays itself out for a wide window at its default size and
			// collapses at the smallest one.
			if size.name == windowSizes[0].name {
				if note.X <= left.X {
					t.Errorf("the grid did not lay out in columns: left %.1f, table %.1f", left.X, note.X)
				}
			} else if note.Y <= left.Y {
				t.Errorf("the narrow grid did not collapse to one column: left y %.1f, table y %.1f", left.Y, note.Y)
			}
		})
	}
}

// The tabs are the host's tab list: choosing one shows its content, and Tab
// leaves it for the next focusable thing in document order, with the focus
// where the reader can see it.
func TestContributedPageKeepsAKeyboardPath(t *testing.T) {
	a, _ := contribApp(t, "report", reportContributes, &fakeRunner{})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	openReportPage(t, tt)

	if !tt.HasText("first tab") {
		t.Fatalf("the first tab's content is not shown; texts %q", tt.Texts())
	}
	if err := tt.Click("Summary"); err != nil {
		t.Fatalf("the tab list: %v", err)
	}
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("Run the report") {
		t.Errorf("Tab did not leave the tabs for the page's action; texts %q", tt.Texts())
	}
	tt.Key(ui.Shift, ui.KeyTab)
	if !tt.Focused("Summary") {
		t.Errorf("Shift+Tab did not come back to the tab list")
	}
	if err := tt.Click("Rows"); err != nil {
		t.Fatalf("choosing the second tab: %v", err)
	}
	if !tt.HasText("row one") || tt.HasText("first tab") {
		t.Errorf("choosing a tab did not show its content; texts %q", tt.Texts())
	}
}

// A page follows the window's language: its own catalog answers in Chinese, and
// the host's chrome does too.
func TestContributedPageFollowsTheLocale(t *testing.T) {
	a, _ := contribApp(t, "report", reportContributes, &fakeRunner{})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.SetLocale("zh-Hans")
	settle(tt)
	openPage(t, tt, "插件页面", "水蛭")

	for _, want := range []string{"水蛭", "概览", "篇笔记", "笔记", "概要", "运行报告"} {
		if !tt.HasText(want) {
			t.Fatalf("the page is not in Chinese: missing %q; texts %q", want, tt.Texts())
		}
	}
	tt.Key(ui.Cmd, ui.KeyK)
	if !tt.HasText("水蛭报告") {
		t.Errorf("the palette lists the command untranslated; texts %q", tt.Texts())
	}
}

// A page whose data did not arrive says so and offers a retry — never a blank —
// and the third failure takes its navigation entry, and only its own.
func TestContributedPageFailsWithARetry(t *testing.T) {
	runner := &fakeRunner{failed: map[string]error{"data": errThrown}}
	a, _ := contribApp(t, "report", reportContributes, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	openReportPage(t, tt)

	if !tt.HasText("This page could not be loaded: the plugin threw") {
		t.Fatalf("the failed page says nothing; texts %q", tt.Texts())
	}
	if !tt.HasText("Retry") {
		t.Errorf("the failed page offers no retry; texts %q", tt.Texts())
	}

	// The plugin answers now, and the retry is the one action that asks again.
	runner.setFailure("data", nil)
	if err := tt.Click("Retry"); err != nil {
		t.Fatalf("the retry: %v", err)
	}
	if !tt.HasText("Overview") {
		t.Fatalf("the retry did not load the page; texts %q", tt.Texts())
	}

	// Three failures hide the page from navigation: the window keeps working
	// and the plugin's command is untouched.
	runner.setFailure("data", errThrown)
	for i := 0; i < contributionErrorLimit; i++ {
		if err := tt.Click("Refresh"); err != nil {
			t.Fatalf("refresh %d: %v", i+1, err)
		}
	}
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	if _, ok := tt.Find("Plugin pages"); ok {
		t.Error("the page is still in navigation after three failures")
	}
	tt.Key(ui.Cmd, ui.KeyK)
	if !tt.HasText("Leech report") {
		t.Errorf("a page's failures hid the plugin's command; texts %q", tt.Texts())
	}
}

// The settings a plugin declares are the host form's fields, and the value the
// reader changes is written into the plugin's own store, not through the plugin.
func TestContributedSettingsWriteThroughTheHostForm(t *testing.T) {
	a, _ := contribApp(t, "report", reportContributes, &fakeRunner{})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("Lapses that make a leech") || !tt.HasText("Count suspended cards") {
		t.Fatalf("the settings page does not draw the plugin's settings; texts %q", tt.Texts())
	}
	reveal(t, tt, 1024, 720, "Count suspended cards")
	reveal(t, tt, 1024, 720, "Increase")

	// A bool is a switch; a number is a number input.
	if err := tt.Click("Count suspended cards"); err != nil {
		t.Fatalf("the switch: %v", err)
	}
	if err := tt.Click("Increase"); err != nil {
		t.Fatalf("the number input: %v", err)
	}

	stored, _ := a.Library().PluginData("acme.report")
	var document struct {
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal([]byte(stored), &document); err != nil {
		t.Fatalf("the plugin's data is not JSON: %v (%q)", err, stored)
	}
	if document.Settings["includeSuspended"] != true {
		t.Errorf("the switch wrote %v, want true", document.Settings["includeSuspended"])
	}
	if document.Settings["threshold"] != float64(9) {
		t.Errorf("the number input wrote %v, want 9", document.Settings["threshold"])
	}

	// Withdrawing the plugin removes its settings from the form and leaves the
	// data where it was: trying a plugin costs nothing but the try.
	reveal(t, tt, 1024, 720, "Enable report")
	if err := tt.Click("Enable report"); err != nil {
		t.Fatalf("the plugin's switch: %v", err)
	}
	if tt.HasText("Lapses that make a leech") {
		t.Error("a withdrawn plugin's settings are still in the form")
	}
	kept, ok := a.Library().PluginData("acme.report")
	if !ok || !strings.Contains(kept, `"threshold":9`) {
		t.Errorf("withdrawing the plugin took its data: %q", kept)
	}
}

// A plugin that declares nothing in contributes grants nothing: no command in
// the palette, no page in navigation, and nothing the window ever asks it.
func TestUndeclaredContributionsGrantNothing(t *testing.T) {
	runner := &fakeRunner{}
	a, _ := contribApp(t, "report", `{}`, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	for _, text := range tt.Texts() {
		if strings.Contains(text, "Leech") {
			t.Errorf("an undeclared command is in the palette: %q", text)
		}
	}
	tt.Key(0, ui.KeyEscape)
	if _, ok := tt.Find("Plugin pages"); ok {
		t.Error("a plugin with no pages has a navigation entry")
	}
	if got := runner.calls(); got != 0 {
		t.Errorf("the plugin was entered %d times for contributions it did not declare", got)
	}
}

// One contribution that does not hold up costs only itself: its sibling still
// shows, and the settings page says what was dropped.
func TestOneBrokenContributionHidesOnlyItself(t *testing.T) {
	contributes := `{
	  "commands": [
	    {"id": "acme.report.good", "title": "%good%", "action": "command:show"},
	    {"id": "acme.report.bad", "title": "%bad%", "icon": "no-such-icon", "action": "command:show"}
	  ],
	  "locales": {"en": {"good": "Good command", "bad": "Bad command"}}
	}`
	runner := &fakeRunner{}
	a, _ := contribApp(t, "report", contributes, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	if !tt.HasText("Good command") {
		t.Fatalf("the legal command is not in the palette; texts %q", tt.Texts())
	}
	if tt.HasText("Bad command") {
		t.Error("a command with an icon this host has not is in the palette")
	}
	tt.Key(0, ui.KeyEscape)

	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(`icon "no-such-icon": the host has no such icon`) {
		t.Errorf("the settings page does not say what was dropped; texts %q", tt.Texts())
	}
}

// A frame never calls a plugin: the page shows its skeleton while a slow data
// command is still out, frames keep returning, and the tree arrives on the next
// frame after the answer does.
func TestFrameNeverCallsAPlugin(t *testing.T) {
	runner := &fakeRunner{gate: make(chan struct{})}
	a, _ := contribApp(t, "report", reportContributes, runner)
	a.runBackground = nil // the app's own: a goroutine, applied on a later frame

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	// Drawing the notes called nothing, and neither do more frames.
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if got := runner.calls(); got != 0 {
		t.Fatalf("a frame entered the plugin %d times", got)
	}

	// Opening the page starts the load off the frame and returns at once: the
	// page draws its skeleton while the plugin is still working.
	if err := tt.Click("Plugin pages"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Leeches"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Loading…") {
		t.Fatalf("the page did not show a skeleton while loading; texts %q", tt.Texts())
	}
	if got := runner.calls(); got != 1 {
		t.Fatalf("the load did not reach the plugin: %d calls", got)
	}

	// The frame that draws the loading page was not waiting on the plugin: the
	// gate is still closed and the frame returned.
	tt.Frame()

	close(runner.gate)
	// The answer is applied on the UI thread: wait for it, then draw.
	deadline := time.Now().Add(5 * time.Second)
	for {
		a.pending.mu.Lock()
		waiting := len(a.pending.jobs)
		a.pending.mu.Unlock()
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the plugin's answer never reached the window")
		}
		time.Sleep(time.Millisecond)
	}
	tt.Frame()
	if !tt.HasText("Overview") || tt.HasText("Loading…") {
		t.Fatalf("the page did not draw the answer; texts %q", tt.Texts())
	}
	if got := runner.calls(); got != 1 {
		t.Errorf("drawing the answer called the plugin again: %d calls", got)
	}
}
