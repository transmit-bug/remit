package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// boardContributes is the whiteboard page a test's plugin declares: one page, one
// action in the page header, and the plugin's own strings for both languages.
const boardContributes = `{
  "commands": [
    {
      "id": "acme.board.locate",
      "title": "%command.locate%",
      "category": "%category.board%",
      "action": "command:locate"
    }
  ],
  "menus": {
    "page.header": [
      {"command": "acme.board.locate", "group": "primary", "order": 10}
    ]
  },
  "pages": [
    {
      "id": "board",
      "title": "%page.title%",
      "icon": "layers",
      "nav": "plugins",
      "when": "library.open",
      "data": "command:data"
    }
  ],
  "locales": {
    "en": {
      "page.title": "Whiteboard",
      "command.locate": "Locate this block",
      "category.board": "Board",
      "section.board": "Places",
      "action.open": "Back to this block",
      "toast.located": "Located"
    },
    "zh-Hans": {
      "page.title": "白板",
      "command.locate": "定位这一段",
      "category.board": "白板",
      "section.board": "位置",
      "action.open": "回到这一段",
      "toast.located": "已定位"
    }
  }
}`

// boardPlace is one node a page's data puts on a board: the note and block it
// names, and the words the page said beside them.
type boardPlace struct {
	note, block core.ID
}

// placesOf lists one place per block of the notes, in the order the pages list
// them, with the words the block says.
func placesOf(notes ...core.Note) []boardPlace {
	var out []boardPlace
	for _, note := range notes {
		for _, block := range note.Blocks {
			out = append(out, boardPlace{note: note.ID, block: block.ID})
		}
	}
	return out
}

// boardTree is a whiteboard page's data as the contract allows it: a grid of
// cards, each naming the block it stands for through an action's value, with the
// note's title and the block's first line beside it. No coordinate, no node kind
// of its own (ui-redesign-v2 §6.4).
func boardTree(places []boardPlace) string {
	cards := make([]string, len(places))
	for i, place := range places {
		cards[i] = fmt.Sprintf(`{"kind":"card","children":[
			{"kind":"text","text":%s},
			{"kind":"markdown","text":%s},
			{"kind":"actions","items":[{"label":"%%action.open%%","value":%s,"command":"acme.board.locate"}]}
		]}`, quote(place.note), quote(place.block), quote(place.note+"/"+place.block))
	}
	return fmt.Sprintf(`{"kind":"page","children":[
		{"kind":"section","title":"%%section.board%%","children":[
			{"kind":"grid","columns":2,"children":[%s]}
		]}
	]}`, strings.Join(cards, ","))
}

// quote writes one string as JSON.
func quote(id core.ID) string {
	encoded, err := json.Marshal(string(id))
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// boardApp is the window on a library with a whiteboard page enabled, its data
// naming every block of two notes, and the plugin's answer for a command.
func boardApp(t *testing.T) (*App, testLibrary, *fakeRunner) {
	t.Helper()
	runner := &fakeRunner{reply: map[string]string{
		"locate": `{"result":"toast","message":"%toast.located%"}`,
	}}
	a, seed := contribApp(t, "board", boardContributes, runner)
	runner.reply["data"] = boardTree(placesOf(seed.memory, seed.writing))
	return a, seed, runner
}

// called reports whether the plugin was asked for one of its callbacks.
func (f *fakeRunner) called(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, entered := range f.names {
		if entered == name {
			return true
		}
	}
	return false
}

// pageTitle is the whiteboard page's name in one of the window's languages, and
// sectionTitle the heading its data puts over the nodes.
func pageTitle(locale string) string {
	if locale == "zh-Hans" {
		return "白板"
	}
	return "Whiteboard"
}

func sectionTitle(locale string) string {
	if locale == "zh-Hans" {
		return "位置"
	}
	return "Places"
}

// The page's own actions stand in its header beside the host's refresh, and one
// runs the command the slot item names. The actions come from the cache the
// library load left, so no frame asks the plugin for anything.
func TestPluginPageShowsItsHeaderActions(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			locale := "en"
			if size.name == "minimum" {
				locale = "zh-Hans"
			}
			msgs := locales.In(locale)
			action := "Locate this block"
			if locale == "zh-Hans" {
				action = "定位这一段"
			}
			runner := &fakeRunner{reply: map[string]string{
				"data":   `{"kind":"page","children":[{"kind":"section","title":"%section.board%","children":[{"kind":"text","text":"the page"}]}]}`,
				"locate": `{"result":"toast","message":"%toast.located%"}`,
			}}
			a, _ := contribApp(t, "board", boardContributes, runner)
			tt := ui.NewTester(a.View, size.w, size.h)
			tt.SetLocale(locale)
			settle(tt)
			openPage(t, tt, pageTitle(locale))
			tt.Frame()

			if !tt.HasText(action) {
				t.Fatalf("the page's header action is missing; texts %q", tt.Texts())
			}
			assertControl(t, tt, size.w, size.h, action)
			// The host's own refresh stays beside it, so the data can always be
			// asked for again.
			assertControl(t, tt, size.w, size.h, msgs.PluginPageRefresh())

			calls := runner.calls()
			if err := tt.Click(action); err != nil {
				t.Fatalf("the header action: %v", err)
			}
			tt.Frame()
			if !runner.called("locate") {
				t.Errorf("the header action did not run its command; calls %q", runner.names)
			}
			if !tt.HasText(map[string]string{"en": "Located", "zh-Hans": "已定位"}[locale]) {
				t.Errorf("the command's answer did not reach the window; texts %q", tt.Texts())
			}
			// One call for the command, none for the frames that drew it.
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			if runner.calls() != calls+1 {
				t.Errorf("the frames entered the plugin %d times, want the one command", runner.calls()-calls)
			}
		})
	}
}

// A board node is where a block lives: the note's title, the block's first line
// and what the block feeds, and clicking it shows the note read at that block.
// The narrow window stacks the board, so 720×520 never scrolls sideways.
func TestWhiteboardOpensTheBlock(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			locale := "en"
			if size.name == "minimum" {
				locale = "zh-Hans"
			}
			msgs := locales.In(locale)
			a, seed, _ := boardApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			tt.SetLocale(locale)
			settle(tt)
			openPage(t, tt, pageTitle(locale))
			tt.Frame()

			first := oneLine(seed.memory.Blocks[0].Text)
			second := oneLine(seed.memory.Blocks[1].Text)
			for _, want := range []string{"Memory and recall", first, msgs.BoardCards(1)} {
				if !tt.HasText(want) {
					t.Fatalf("the board is missing %q; texts %q", want, tt.Texts())
				}
			}
			// Six nodes in two columns, or one at the smallest window: either
			// way the second node is below the first once the columns collapse.
			top, _ := tt.Find(first)
			below, _ := tt.Find(second)
			if size.name == "minimum" && below.Y <= top.Y {
				t.Errorf("the narrow board kept its columns: %.1f, %.1f", top.Y, below.Y)
			}
			if size.name != "minimum" && below.Y > top.Y {
				t.Errorf("the wide board stacked its columns: %.1f, %.1f", top.Y, below.Y)
			}

			if err := tt.Click(first); err != nil {
				t.Fatalf("the board node: %v", err)
			}
			tt.Frame()
			if a.notes.Draft.ID != seed.memory.ID {
				t.Errorf("the node opened note %q, want %q", a.notes.Draft.ID, seed.memory.ID)
			}
			if a.notes.View != notes.Read {
				t.Errorf("the node opened the note in view %v, want read", a.notes.View)
			}
			if a.notes.At() != 1 {
				t.Errorf("the node placed the reader at %d, want the block's paragraph", a.notes.At())
			}
			if tt.HasText(sectionTitle(locale)) {
				t.Error("the board stayed up over the note it opened")
			}
		})
	}
}

// A page's data is older than the library: a block whose text changed after the
// load still names its place, and the board says what the note says now. A block
// the note no longer holds keeps its node and says so, rather than taking the
// whole page down.
func TestWhiteboardSurvivesAChangedBlock(t *testing.T) {
	a, seed, _ := boardApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	openPage(t, tt, pageTitle("en"))
	tt.Frame()

	was := oneLine(seed.memory.Blocks[0].Text)
	if !tt.HasText(was) {
		t.Fatalf("the board does not list the block; texts %q", tt.Texts())
	}

	// The paragraph is rewritten after the page's data was loaded.
	edited := seed.memory
	edited.Blocks = append([]core.Block(nil), seed.memory.Blocks...)
	edited.Blocks[0].Text = "Spacing beats cramming :: rewritten while the page was open."
	if _, err := a.Library().SaveNote(edited); err != nil {
		t.Fatalf("save the edited note: %v", err)
	}
	a.reload()
	tt.Frame()

	now := oneLine(edited.Blocks[0].Text)
	if tt.HasText(was) {
		t.Errorf("the board still prints what the data said before the edit; texts %q", tt.Texts())
	}
	if !tt.HasText(now) {
		t.Fatalf("the board lost the node when its block changed; texts %q", tt.Texts())
	}
	// It still goes to that block.
	if err := tt.Click(now); err != nil {
		t.Fatalf("the board node after the edit: %v", err)
	}
	tt.Frame()
	if a.notes.Draft.ID != seed.memory.ID || a.notes.At() != 1 {
		t.Errorf("the node opened note %q at %d, want %q at 1", a.notes.Draft.ID, a.notes.At(), seed.memory.ID)
	}

	// The paragraph goes altogether: the page still names the place, and the node
	// says the note no longer holds it rather than dropping out of the board.
	shortened := edited
	shortened.Blocks = edited.Blocks[1:]
	if _, err := a.Library().SaveNote(shortened); err != nil {
		t.Fatalf("save the note without the block: %v", err)
	}
	a.reload()
	openPage(t, tt, pageTitle("en"))
	tt.Frame()
	if !tt.HasText(locales.In("en").BoardBlockGone()) {
		t.Errorf("a node whose block is gone says nothing; texts %q", tt.Texts())
	}
}

// The block menu and the block inspector offer "locate on board" only for a
// block the board on screen actually places: the entry reads the page's cached
// data, and a block no node names has nowhere to go. The trip opens the board
// and points at the node once — twice, if the user asks twice.
func TestInspectorOffersTheWhiteboardWhenANodeExists(t *testing.T) {
	a, seed, _ := boardApp(t)
	// The inspector's entry runs from a handler; the test is the handler.
	trip := false
	tt := ui.NewTester(func(c *ui.Context) {
		if trip {
			trip = false
			a.LocateOnBoard(c, seed.memory.Blocks[1].ID)
		}
		a.View(c)
	}, 1024, 720)
	settle(tt)
	openPage(t, tt, pageTitle("en"))
	tt.Frame()

	placed, unplaced := seed.memory.Blocks[0].ID, seed.habit.Blocks[0].ID
	if !a.OnBoard(placed) {
		t.Fatal("the entry is not offered for a block the board places")
	}
	if a.OnBoard(unplaced) {
		t.Error("the entry is offered for a block no node names")
	}
	if a.OnBoard("") {
		t.Error("the entry is offered for no block at all")
	}

	// The entry's trip: the board opens and the node is pointed at.
	trip = true
	tt.Frame()
	tt.Frame()
	ref := a.page
	if a.area != areaPlugin || ref.Page.ID != "board" {
		t.Fatalf("the trip went to %v/%q, want the board page", a.area, ref.Page.ID)
	}
	state := a.pageState(ref)
	if state.Highlight != string(seed.memory.Blocks[1].ID) || state.HighlightRun != 1 {
		t.Errorf("the trip points at %q (run %d), want the block it was asked for",
			state.Highlight, state.HighlightRun)
	}
	tt.Advance(300 * time.Millisecond)
	if tt.FramePending() {
		t.Error("the board keeps asking for frames after the pointer settled")
	}
	// A second trip replays the pointer rather than staying still.
	trip = true
	tt.Frame()
	if state := a.pageState(ref); state.HighlightRun != 2 {
		t.Errorf("the second trip ran the pointer %d times, want 2", state.HighlightRun)
	}
}
