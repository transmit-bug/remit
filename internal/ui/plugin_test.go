package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/plugin"
	"github.com/transmit-bug/remit/locales"
)

// withDeriver opens the seeded library with the window's card role played by a Go
// Deriver: what a derived field renders through its type's templates, with no
// JavaScript written and no plugin started at all — both roles are the test's, so
// the window resolves nothing and loads no engine. The role is the function, so a
// test that watches the window derive watches exactly this.
func withDeriver(t *testing.T, derive func(ext.DeriveIn) ([]ext.DerivedCard, error)) *App {
	t.Helper()
	dir := t.TempDir()
	seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir, WithoutCards: true, WithoutScheduler: true})
	a.cards = &cardPlugin{deriver: deriverFunc(derive), producer: core.Producer{ID: "test.cards", Version: "1"}}
	a.reload()
	return a
}

// deriverFunc is a card plugin with no engine behind it: deriving is the whole of
// what the window asks, so a function is the plugin.
type deriverFunc func(ext.DeriveIn) ([]ext.DerivedCard, error)

func (f deriverFunc) DeriveBlock(in ext.DeriveIn) ([]ext.DerivedCard, error) { return f(in) }

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
	a := withDeriver(t, func(in ext.DeriveIn) ([]ext.DerivedCard, error) {
		text := in.Block.Text
		at := strings.Index(text, "::")
		if at < 0 {
			return nil, nil
		}
		return []ext.DerivedCard{{
			TypeID: string(core.TypeBasic), Ordinal: 0,
			Fields: map[string]string{
				"Front": "PLUGIN " + strings.TrimSpace(text[:at]),
				"Back":  strings.TrimSpace(text[at+2:]),
			},
		}}, nil
	})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallQueue(tt)
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond) // the answer unfolding

	if !tt.HasText("PLUGIN Spacing beats cramming") {
		t.Fatalf("the card does not read the plugin's fields; texts %q", tt.Texts())
	}
}

// A derivation that fails does not stop the review: the error is the window's to
// swallow, so the card reads its paragraph's own words and the queue still moves.
// The regression this guards: a plugin's failure on screen — a blank card, or a
// review that will not go on.
func TestFailingDerivationFallsBackToTheParagraph(t *testing.T) {
	a := withDeriver(t, func(ext.DeriveIn) ([]ext.DerivedCard, error) {
		return nil, errors.New("the card plugin threw")
	})
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
// It answers the capability itself, which is how a stand-in with no wire behind it
// is asked (ext.HooksOf).
type fakeRole struct {
	name   string
	family string
	warmed int
}

func (f *fakeRole) Name() string { return f.name }
func (f *fakeRole) Warm() error  { f.warmed++; return nil }
func (f *fakeRole) Close() error { return nil }

func (f *fakeRole) Policy() (ext.Policy, error) {
	return ext.Policy{Family: f.family, Version: "9"}, nil
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

// askedTexts is a card plugin that counts what it was asked, by text: what the
// window asks it again after a write is the number this test reads.
func askedTexts(asks map[string]int) func(ext.DeriveIn) ([]ext.DerivedCard, error) {
	return func(in ext.DeriveIn) ([]ext.DerivedCard, error) {
		asks[in.Block.Text]++
		return nil, nil
	}
}

// A write — a rating is one — must not ask the card plugin again for a paragraph
// it already answered: the window keeps its resolver, and the answers in it,
// across the reload every write ends with. The regression this guards: a resolver
// built per write, which made the cache live exactly as long as the write before
// it (docs/specs/plugin-system.md P4).
func TestAWriteDoesNotAskThePluginAgain(t *testing.T) {
	asks := map[string]int{}
	a := withDeriver(t, askedTexts(asks))
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallQueue(tt)
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond) // the answer unfolding

	id, ok := a.review.Current()
	if !ok {
		t.Fatal("the review has no card on screen")
	}
	card, ok := a.data.byID[id]
	if !ok {
		t.Fatalf("the card %q is not in the snapshot", id)
	}
	text, _ := card.Ref.Text(a.data.blocks)
	if got := asks[text]; got != 1 {
		t.Fatalf("the card's text was asked %d times before the rating, want once", got)
	}

	tt.Key(0, ui.Key3) // Good: the write the rating makes
	tt.Advance(320 * time.Millisecond)
	a.face(card)
	if got := asks[text]; got != 1 {
		t.Errorf("rating asked the plugin again: %d asks for %q, want one", got, text)
	}
}

// A type's fields are part of what the plugin is asked, so editing them must ask
// again: the window keeps its resolver only while the type table is the one it was
// built for. The regression this guards: a kept resolver serving the answers of
// the old field table after a type is edited.
func TestAnEditedTypeAsksThePluginAgain(t *testing.T) {
	asks := map[string]int{}
	a := withDeriver(t, askedTexts(asks))
	card := a.data.cards[0]
	text, _ := card.Ref.Text(a.data.blocks)
	a.face(card)

	typ, ok := a.Library().Type(card.Type)
	if !ok {
		t.Fatalf("the card's type %q is not in the library", card.Type)
	}
	typ.Fields = append(typ.Fields, "Extra")
	if _, err := a.Library().SaveType(typ); err != nil {
		t.Fatalf("save the edited type: %v", err)
	}
	a.reload()
	a.face(card)
	if got := asks[text]; got != 2 {
		t.Errorf("an edited type was asked %d times, want 2: the old answer was served", got)
	}
}

// Switching the card plugin is a different plugin answering, so the window asks
// the new one rather than serving the old one's answers. The regression this
// guards: a kept resolver deriving through the plugin the window no longer holds.
func TestASwitchedCardPluginAsksThePluginAgain(t *testing.T) {
	asks := map[string]int{}
	a := withDeriver(t, askedTexts(asks))
	card := a.data.cards[0]
	text, _ := card.Ref.Text(a.data.blocks)
	a.face(card)

	a.cards = &cardPlugin{
		deriver:  deriverFunc(askedTexts(asks)),
		producer: core.Producer{ID: "test.cards.other", Version: "1"},
	}
	a.reload()
	a.face(card)
	if got := asks[text]; got != 2 {
		t.Errorf("a switched plugin was asked %d times, want 2: the old answer was served", got)
	}
}
