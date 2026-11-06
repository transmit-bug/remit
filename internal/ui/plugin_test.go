package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/locales"
)

// withPlugin opens a library with the card plugin given, on the library the
// other tests seed.
func withPlugin(t *testing.T, dir string, source string) (*App, testLibrary) {
	t.Helper()
	lib := t.TempDir()
	seed := seedLibrary(t, lib)
	extDir := t.TempDir()
	manifest := `{"name":"cards","version":"1","protocol":1,"runtime":"js","entry":"main.js"}`
	if err := os.WriteFile(filepath.Join(extDir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	runner, err := ext.OpenJS(extDir, ext.JSLimits{Timeout: time.Second})
	if err != nil {
		t.Fatalf("open the test plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return NewApp(Options{LibraryDir: lib, Cards: runner}), seed
}

// A plugin's render is what the card reads: the host draws what the plugin
// answered, not what it would have said itself.
func TestRenderedCardComesFromThePlugin(t *testing.T) {
	a, _ := withPlugin(t, "", `
export function derive(block) {
  const text = (block && block.text) || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{ kind: "basic", ordinal: 0, front: text.slice(0, at).trim(), back: text.slice(at + 2).trim() }];
}
export function render(card) {
  return { front: "PLUGIN " + card.front, back: "PLUGIN " + card.back };
}
`)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(0, ui.KeySpace)
	tt.Advance(320 * time.Millisecond) // the answer unfolding

	if !tt.HasText("PLUGIN Spacing beats cramming") {
		t.Fatalf("the card does not read what the plugin answered; texts %q", tt.Texts())
	}
}

// A plugin that throws, or loops past its budget, does not stop the review: the
// card reads its paragraph's own words.
func TestBrokenPluginFallsBackToTheParagraph(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"throws", `export function derive() { throw new Error("no"); }
export function render() { throw new Error("no"); }`},
		{"loops", `export function derive() { for (;;) {} }
export function render() { for (;;) {} }`},
		{"answers with nonsense", `export function derive() { return "nonsense"; }
export function render() { return 42; }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := withPlugin(t, "", tc.source)
			tt := ui.NewTester(a.View, 1024, 720)
			settle(tt)
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
		if card.Ref.Block == a.editor.draft.Blocks[0].ID {
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

	block := a.editor.draft.Blocks[0].ID
	var made []core.Card
	for _, card := range a.Library().Cards() {
		if card.Ref.Block == block {
			made = append(made, card)
		}
	}
	if len(made) != 1 || made[0].Kind != core.KindBasic {
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
	manifest := `{"name":"scheduler","version":"1","protocol":1,"runtime":"js","entry":"main.js"}`
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
	runner, err := ext.OpenJS(extDir, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the test scheduler: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	a := NewApp(Options{LibraryDir: dir, Scheduler: runner})
	// A library with nothing due: the summary is what the screen shows.
	for _, card := range a.Library().Cards() {
		card.Status = core.StatusSuspended
		if err := a.Library().SaveCard(card); err != nil {
			t.Fatal(err)
		}
	}
	a.reload()
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	if !tt.HasText("you reviewed 0 cards") {
		t.Fatalf("the summary does not carry the plugin's line; texts %q", tt.Texts())
	}
	// The plugin names the scheduler every review it writes is stamped with.
	if got := a.Library().Scheduler(); got != "test-9" {
		t.Errorf("the library schedules with %q, want test-9", got)
	}
}
