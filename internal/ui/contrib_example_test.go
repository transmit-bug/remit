package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
)

// pluginHost is the store as a plugin's host: what a contributed page reads
// through notes:read. Nothing in these tests derives a card through it.
type pluginHost struct{ *store.Library }

func (h pluginHost) CreateCard(blockID core.ID) (core.Card, error) {
	created, err := h.Library.CreateCards(blockID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return created[0], nil
}

func (h pluginHost) Faces(core.Card) (string, string, string) { return "", "", core.LinkLive.String() }

// exampleContributionApp runs the example plugin the guide ships for real: the
// same loader, capability check and limits a downloaded plugin gets, over the
// test library through a host. It is the contract's fixture, so it cannot rot.
func exampleContributionApp(t *testing.T, locale string) *ui.Tester {
	t.Helper()
	_, tt := exampleContribution(t, locale)
	return tt
}

// exampleContribution is the app behind exampleContributionApp, for a test that
// writes to the library and expects the window to hear about it.
func exampleContribution(t *testing.T, locale string) (*App, *ui.Tester) {
	t.Helper()
	dir := t.TempDir()
	seedLibrary(t, dir)
	enablePlugin(t, dir, "leeches")
	extDir := filepath.Join("..", "..", "examples", "plugins", "leeches")

	a := NewApp(Options{
		LibraryDir: dir,
		Manifests: func() ([]ext.Extension, error) {
			found, err := ext.Load(extDir)
			return []ext.Extension{found}, err
		},
	})
	runner, err := ext.OpenJS(extDir, pluginHost{a.Library()}, ext.JSLimits{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("load the example plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	a.contribution = func(string) (CommandRunner, error) { return runner, nil }
	a.runBackground = func(c *ui.Context, job func() func(*ui.Context)) { job()(c) }

	tt := ui.NewTester(a.View, 1024, 720)
	if locale != "" {
		tt.SetLocale(locale)
	}
	settle(tt)
	return a, tt
}

// The example plugin's page is the library it read through its capability,
// drawn by the host: a stat row, a callout and a table of the notes.
func TestTheExamplePluginContributesItsPage(t *testing.T) {
	tt := exampleContributionApp(t, "")
	openPage(t, tt, "Leeches")

	for _, want := range []string{
		"Leeches",
		"Overview",
		"notes",
		"A leech is a card you keep forgetting",
		"Note",
		"Cards",
		"Memory and recall",
		"Writing cards",
		"A daily habit",
	} {
		if !tt.HasText(want) {
			t.Fatalf("the example's page is missing %q; texts %q", want, tt.Texts())
		}
	}
	if !tt.HasText("3") {
		t.Errorf("the page's note count is not the library's; texts %q", tt.Texts())
	}
}

// The example's own strings answer in Chinese when the window speaks it: its
// catalog is the plugin's, not the app's.
func TestTheExamplePluginFollowsTheLocale(t *testing.T) {
	tt := exampleContributionApp(t, "zh-Hans")
	openPage(t, tt, "水蛭")

	for _, want := range []string{"水蛭", "概览", "篇笔记", "水蛭是总也记不住的卡片", "笔记"} {
		if !tt.HasText(want) {
			t.Fatalf("the example's page is not in Chinese: missing %q; texts %q", want, tt.Texts())
		}
	}
}

// The example's two commands are in the palette and its menu slot, and running
// one runs its callback in the plugin's own runtime: a toast comes back, and the
// other navigates to the page.
func TestTheExamplePluginsCommandsRun(t *testing.T) {
	tt := exampleContributionApp(t, "")
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)

	if err := tt.Click("More"); err != nil {
		t.Fatalf("the window's overflow menu: %v", err)
	}
	if err := tt.ChooseMenuItem("Pause and tag"); err != nil {
		t.Fatalf("the plugin's menu item: %v", err)
	}
	if !tt.HasText("Card paused and tagged") {
		t.Fatalf("the command's toast did not arrive; texts %q", tt.Texts())
	}

	// The command that navigates opens the plugin's page. Its when shows it in
	// review and in browse, not in the notes, so the review is where it runs.
	tt.Advance(6 * time.Second)
	recallQueue(tt)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("leech")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("Overview") {
		t.Fatalf("the command did not open its page; texts %q", tt.Texts())
	}
}

// A write lands as an event: the page on screen asks its plugin again instead
// of showing the library it was loaded against (ui.md §9).
func TestAnEventReloadsTheOpenPage(t *testing.T) {
	a, tt := exampleContribution(t, "")
	openPage(t, tt, "Leeches")
	if !tt.HasText("Overview") {
		t.Fatalf("the page did not load; texts %q", tt.Texts())
	}
	if tt.HasText("A fourth note") {
		t.Fatalf("the page shows a note that is not in the library yet; texts %q", tt.Texts())
	}

	if _, err := a.Library().SaveNote(core.Note{
		Title:  "A fourth note",
		Blocks: []core.Block{{Text: "Question :: answer"}},
	}); err != nil {
		t.Fatalf("write a note: %v", err)
	}
	a.LibraryEvent()
	settle(tt)

	if !tt.HasText("A fourth note") {
		t.Fatalf("an event did not reload the open page; texts %q", tt.Texts())
	}
}

// An item's own when decides where it shows: the paused-and-tag command's
// note.overflow item names note.open, so the merged menu offers it in the notes
// and not in browse, even though the command itself carries no when.
func TestAMenuItemsWhenDecidesWhereItShows(t *testing.T) {
	tt := exampleContributionApp(t, "")

	cardsPage(t, tt, "en") // the card table: the toolbar is up, no note is open
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click("More"); err != nil {
		t.Fatalf("the window's overflow menu: %v", err)
	}
	menu := strings.Join(tt.Menu(), "\n")
	if strings.Contains(menu, "Pause and tag") {
		t.Fatalf("the note slot's item showed with no note open: %q", menu)
	}
	if !strings.Contains(menu, "Leech report") {
		t.Fatalf("the toolbar slot's item did not show: %q", menu)
	}
	tt.CloseMenu()

	tt.Key(ui.Cmd, ui.Key2) // notes: a note is open, so its slot item shows
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click("More"); err != nil {
		t.Fatalf("the window's overflow menu: %v", err)
	}
	if menu := strings.Join(tt.Menu(), "\n"); !strings.Contains(menu, "Pause and tag") {
		t.Fatalf("the note slot's item did not show with a note open: %q", menu)
	}
}
