package ui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// whiteboardExample runs the whiteboard the guide ships, for real: the same
// loader, capability check and JS runtime a downloaded plugin gets, over the test
// library through a host. Its page is the contract's stands-in for a canvas — a
// node tree over note and block references, drawn by the host — so the example
// cannot rot without this turning red.
func whiteboardExample(t *testing.T, locale string) (*App, testLibrary, *ui.Tester) {
	t.Helper()
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	enablePlugin(t, dir, "whiteboard")
	extDir := filepath.Join("..", "..", "examples", "plugins", "whiteboard")

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
	return a, seed, tt
}

// boardTitle is the whiteboard page's name in one of the window's languages.
func boardTitle(locale string) string {
	if locale == "zh-Hans" {
		return "白板"
	}
	return "Whiteboard"
}

// The example's board is the library it read through notes:read, drawn by the
// host as one node per paragraph, and a node opens the paragraph it names in the
// note, read.
func TestTheWhiteboardExampleOpensItsBlock(t *testing.T) {
	for _, locale := range []string{"en", "zh-Hans"} {
		t.Run(locale, func(t *testing.T) {
			a, seed, tt := whiteboardExample(t, locale)
			openPage(t, tt, boardTitle(locale))
			msgs := locales.In(locale)

			if !tt.HasText(seed.memory.Title) {
				t.Fatalf("the board draws no node for the note; texts %q", tt.Texts())
			}
			if !tt.HasText(msgs.BoardCards(1)) {
				t.Errorf("the node does not say how many cards its paragraph feeds; texts %q", tt.Texts())
			}

			// A node is one button: it shows the note read, at the paragraph it
			// names.
			reveal(t, tt, 1024, 720, seed.memory.Title)
			if err := tt.Click(seed.memory.Title); err != nil {
				t.Fatalf("the board's node: %v", err)
			}
			tt.Advance(320 * time.Millisecond)
			if a.area != areaWrite || a.sub != subNotes {
				t.Fatalf("the node went to %v/%v, want the notebook", a.area, a.sub)
			}
			if a.notes.Draft.ID != seed.memory.ID {
				t.Errorf("the node opened note %q, want %q", a.notes.Draft.ID, seed.memory.ID)
			}
			if a.notes.View != notes.Read || a.notes.At() != 1 {
				t.Errorf("the node opened the note in view %v at paragraph %d, want it read at 1",
					a.notes.View, a.notes.At())
			}
		})
	}
}

// The block's menu brings the reader back to the page and points at the node
// once: the round trip the board is for, and no entry for a paragraph no board
// places.
func TestTheWhiteboardExampleLocatesItsNode(t *testing.T) {
	a, seed, tt := whiteboardExample(t, "")
	msgs := locales.In("en")
	openPage(t, tt, boardTitle("en"))
	tt.Frame()

	if !a.OnBoard(seed.memory.Blocks[0].ID) {
		t.Fatal("the board the window holds does not place the paragraph it drew")
	}
	// The example lists every paragraph, so what it does not place is a block of
	// another library: an id nothing drew has nowhere to go.
	if a.OnBoard("block_nowhere") || a.OnBoard("") {
		t.Error("the entry is offered for a paragraph no node names")
	}

	// Back to the note, and from its menu to the node.
	if err := tt.Click(seed.memory.Title); err != nil {
		t.Fatalf("the board's node: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	// The editor is where a paragraph's menu lives, and its document is built by
	// the frame that draws it.
	a.notes.View = notes.Edit
	tt.Frame()
	if a.notes.Doc == nil {
		t.Fatal("the notebook has no document to put the caret in")
	}
	a.notes.Doc.SetSelection(0, 0)
	tt.Frame()
	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatalf("the paragraph has no menu: %v", err)
	}
	tt.Frame()
	scrollPopoverTo(t, tt, msgs.DeleteBlock(), msgs.BoardLocate())
	if err := tt.Click(msgs.BoardLocate()); err != nil {
		t.Fatalf("the block menu's board entry: %v", err)
	}
	tt.Frame()

	if a.area != areaPlugin || a.page.Page.ID != "whiteboard" {
		t.Fatalf("the entry went to %v/%q, want the whiteboard page", a.area, a.page.Page.ID)
	}
	state := a.pageState(a.page)
	if state.Highlight != string(seed.memory.Blocks[0].ID) || state.HighlightRun != 1 {
		t.Errorf("the entry points at %q (run %d), want the paragraph's node",
			state.Highlight, state.HighlightRun)
	}
	tt.Advance(design.ModeSwitch.Duration + design.ModeSwitch.Duration)
	tt.Frame()
	if tt.FramePending() {
		t.Error("the board keeps asking for frames after the pointer settled")
	}
}

// The example's own strings are the plugin's: its page and its header action
// answer in the window's language.
func TestTheWhiteboardExampleFollowsTheLocale(t *testing.T) {
	_, _, tt := whiteboardExample(t, "zh-Hans")
	openPage(t, tt, boardTitle("zh-Hans"))

	for _, want := range []string{"白板", "位置", "关于这块白板", "页面动作", "连线"} {
		if !tt.HasText(want) {
			t.Fatalf("the board is not in Chinese: missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("Whiteboard") || tt.HasText("Places") || tt.HasText("About this board") {
		t.Errorf("the page shows English strings in a Chinese window; texts %q", tt.Texts())
	}
}
