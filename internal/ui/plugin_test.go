package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/plugin"
	"github.com/transmit-bug/remit/locales"
)

// withPlugin opens a library with the card plugin given, on the library the
// other tests seed.
func withPlugin(t *testing.T, dir string, source string) (*App, testLibrary) {
	t.Helper()
	lib := t.TempDir()
	seed := seedLibrary(t, lib)
	extDir := t.TempDir()
	manifest := `{"id":"test.cards","name":"cards","version":"1","apiVersion":1,"runtime":"js","entry":"main.js","implements":{"deriveBlock":1}}`
	if err := os.WriteFile(filepath.Join(extDir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	runner, err := ext.OpenJS(extDir, nil, ext.JSLimits{Timeout: time.Second})
	if err != nil {
		t.Fatalf("open the test plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return NewApp(Options{LibraryDir: lib, Plugins: pluginsWith(t, t.TempDir(), map[string]ext.Hook{"cards": runner})}), seed
}

// pluginsWith is a supervisor that answers the runners given, by manifest name,
// and builds the app's own for everything else: a window driven by a fake plugin
// without a plugin directory or a second implementation.
func pluginsWith(t *testing.T, dir string, runners map[string]ext.Hook) *plugin.Supervisor {
	t.Helper()
	s := plugin.New(dir)
	s.Construct = func(name string) (ext.Hook, error) {
		if runner, ok := runners[name]; ok {
			return runner, nil
		}
		return s.Build(name)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// A plugin's derived fields are what the card reads, through its type's
// templates: the host draws the template's answer over the plugin's fields,
// not anything the host would have said itself.
func TestRenderedCardComesFromThePlugin(t *testing.T) {
	a, _ := withPlugin(t, "", `
export function deriveBlock(block) {
  const text = (block && block.text) || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{ typeId: "remit.basic", ordinal: 0,
    fields: { Front: "PLUGIN " + text.slice(0, at).trim(), Back: text.slice(at + 2).trim() } }];
}
`)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallQueue(tt)
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond) // the answer unfolding

	if !tt.HasText("PLUGIN Spacing beats cramming") {
		t.Fatalf("the card does not read the plugin's fields; texts %q", tt.Texts())
	}
}

// A plugin that throws, or loops past its budget, does not stop the review: the
// card reads its paragraph's own words.
func TestBrokenPluginFallsBackToTheParagraph(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"throws", `export function deriveBlock() { throw new Error("no"); }`},
		{"loops", `export function deriveBlock() { for (;;) {} }`},
		{"answers with nonsense", `export function deriveBlock() { return 42; }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := withPlugin(t, "", tc.source)
			tt := ui.NewTester(a.View, 1024, 720)
			settle(tt)
			recallQueue(tt)
			tt.Key(0, ui.KeySpace)
			tt.Advance(320 * time.Millisecond)

			// The card still reads its paragraph, and the queue still moves.
			if !tt.HasText("reviewing just before you forget strengthens recall.") {
				t.Fatalf("the card reads nothing; texts %q", tt.Texts())
			}
			tt.Key(0, ui.Key3) // Good
			tt.Advance(320 * time.Millisecond)
			if !tt.HasText("3 cards left") && !tt.HasText("2 cards left") {
				t.Errorf("grading did not move the queue on; texts %q", tt.Texts())
			}
			if a.cards == nil {
				t.Error("the plugin was dropped rather than reported")
			}
		})
	}
}

// Without a card plugin the library still works: a paragraph makes no new card,
// and the cards it has read their paragraphs.
func TestLibraryWithoutACardPlugin(t *testing.T) {
	dir := t.TempDir()
	_ = seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir, WithoutCards: true})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallQueue(tt)

	// The review works, reading the paragraph's own words.
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("reviewing just before you forget strengthens recall.") {
		t.Fatalf("the review lost its card; texts %q", tt.Texts())
	}

	// A new note with a separator makes no card, and says so nowhere in red.
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click("New note"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Note"); err != nil {
		t.Fatal(err)
	}
	tt.Type("what is spacing :: the gaps between reviews")
	tt.Advance(320 * time.Millisecond)
	for _, card := range a.Library().Cards() {
		if card.Ref.Block == a.notes.Draft.Blocks[0].ID {
			t.Fatalf("a card was made without a plugin: %+v", card)
		}
	}
	if !tt.HasText(locales.In("en").NoteTitle()) {
		t.Errorf("the notebook did not survive the missing plugin; texts %q", tt.Texts())
	}
}

// The app's own plugin is what derives cards: the one it ships answers a
// separator with a card whose sides are the model's.
func TestTheShippedPluginDerivesCards(t *testing.T) {
	a, _ := newTestApp(t)
	if a.cards == nil {
		t.Fatal("the app started without its card plugin")
	}
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	notesMode(t, a, tt)
	if err := tt.Click("New note"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Note"); err != nil {
		t.Fatal(err)
	}
	tt.Type("what is spacing :: the gaps between reviews")
	tt.Advance(320 * time.Millisecond)

	block := a.notes.Draft.Blocks[0].ID
	var made []core.Card
	for _, card := range a.Library().Cards() {
		if card.Ref.Block == block {
			made = append(made, card)
		}
	}
	if len(made) != 1 || made[0].Type != core.TypeBasic {
		t.Fatalf("the plugin made %+v, want one basic card", made)
	}
	prompt, answer, _ := a.faces(made[0])
	if prompt != "what is spacing" || answer != "the gaps between reviews" {
		t.Errorf("the card reads %q / %q", prompt, answer)
	}
}

// The scheduling plugin decides what a finished session says beyond its numbers:
// a line of its own, and whether it is quiet about it.
func TestSchedulerPluginWordTheSummary(t *testing.T) {
	dir := t.TempDir()
	_ = seedLibrary(t, dir)
	extDir := t.TempDir()
	manifest := `{"id":"test.scheduler","name":"scheduler","version":"1","apiVersion":1,"runtime":"js","entry":"main.js","implements":{"policy":1,"summary":1}}`
	if err := os.WriteFile(filepath.Join(extDir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `
export function policy() { return { family: "test", version: "9", retention: 0.9 }; }
export function summary(session) {
  return { tone: "celebrated", lines: ["you reviewed " + session.reviewed + " cards"] };
}
`
	if err := os.WriteFile(filepath.Join(extDir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	runner, err := ext.OpenJS(extDir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the test scheduler: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	a := NewApp(Options{LibraryDir: dir, Plugins: pluginsWith(t, t.TempDir(), map[string]ext.Hook{"scheduler": runner})})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallQueue(tt)

	// The session runs to its end: the summary is what the screen shows then.
	for range a.Library().Cards() {
		tt.Key(0, ui.KeySpace)
		tt.Key(0, ui.Key3) // Good
		tt.Advance(320 * time.Millisecond)
	}
	if !tt.HasText("you reviewed 4 cards") {
		t.Fatalf("the summary does not carry the plugin's line; texts %q", tt.Texts())
	}
	// The plugin names the scheduler every review it writes is stamped with.
	if got := a.Library().Scheduler(); got != "test-9" {
		t.Errorf("the library schedules with %q, want test-9", got)
	}
}

// fakeRole is a plugin a test hands the supervisor in place of the engine: the
// window asks a role for a policy, and nothing else in these tests needs a hook.
type fakeRole struct {
	name   string
	family string
	warmed int
}

func (f *fakeRole) Name() string { return f.name }
func (f *fakeRole) Warm() error  { f.warmed++; return nil }
func (f *fakeRole) Close() error { return nil }

func (f *fakeRole) Call(method string, _, out any) error {
	if method != "policy" {
		return fmt.Errorf("fake role %q has no %s", f.name, method)
	}
	return json.Unmarshal([]byte(fmt.Sprintf(`{"family":%q,"version":"9"}`, f.family)), out)
}

// The window builds no plugin of its own: with a supervisor that can start
// nothing it runs without the roles, rather than with a built-in it made itself
// (docs/specs/plugin-system.md, P1: one owner of a plugin's life). The
// regression this guards: a second construction path in the window, which is what
// let a library's plugin and the app's own drift apart.
func TestTheWindowBuildsNoPluginOfItsOwn(t *testing.T) {
	dir := t.TempDir()
	_ = seedLibrary(t, dir)
	s := plugin.New(t.TempDir())
	s.Construct = func(name string) (ext.Hook, error) {
		return nil, fmt.Errorf("nothing starts: %s", name)
	}

	a := NewApp(Options{LibraryDir: dir, Plugins: s})
	if a.cards != nil || a.scheduler != nil {
		t.Fatalf("the window holds roles its supervisor did not build: cards %v, scheduler %v", a.cards, a.scheduler)
	}
}

// The window's roles are the supervisor's runners, runner for runner: the policy
// a library opens with comes from the one plugin the module constructed and
// warmed, not from a copy the window started beside it.
func TestTheWindowResolvesAndWarmsRolesThroughTheSupervisor(t *testing.T) {
	dir := t.TempDir()
	_ = seedLibrary(t, dir)
	scheduler := &fakeRole{name: "scheduler", family: "test"}
	asked := []string{}
	s := plugin.New(t.TempDir())
	s.Construct = func(name string) (ext.Hook, error) {
		asked = append(asked, name)
		if name == "scheduler" {
			return scheduler, nil
		}
		return nil, fmt.Errorf("no plugin named %q", name)
	}

	a := NewApp(Options{LibraryDir: dir, Plugins: s})
	if got := a.Library().Scheduler(); got != "test-9" {
		t.Errorf("the library schedules with %q, want the runner the supervisor resolved", got)
	}
	if !slices.Contains(asked, "scheduler") {
		t.Errorf("the supervisor was asked for %v, want the scheduling role's plugin", asked)
	}
	if scheduler.warmed == 0 {
		t.Error("the role's plugin was not warmed before the library read")
	}
}
