package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// everyWhere is the fixture the receiver tests share: one item in every slot the
// window draws, each naming the context key its receiver is about, and a second
// item per slot whose when is false where that receiver is up. A receiver that
// forgot to evaluate an item's when draws a control the frame says is not there.
//
// It is apart from everySlot, which pins what each accessor answers: this one
// exists to be drawn.
const everyWhere = `{
  "commands": [
    {"id": "acme.report.toolbar", "title": "%toolbar%", "action": "command:toolbar"},
    {"id": "acme.report.toolbar.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.note", "title": "%note%", "action": "command:note"},
    {"id": "acme.report.note.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.block", "title": "%block%", "action": "command:block"},
    {"id": "acme.report.block.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.slash", "title": "%slash%", "action": "command:slash"},
    {"id": "acme.report.slash.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.card", "title": "%card%", "action": "command:card"},
    {"id": "acme.report.card.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.row", "title": "%row%", "action": "command:row"},
    {"id": "acme.report.row.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.side", "title": "%side%", "action": "command:side"},
    {"id": "acme.report.side.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.header", "title": "%header%", "action": "command:header"},
    {"id": "acme.report.header.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.today", "title": "%today%", "action": "command:today"},
    {"id": "acme.report.today.off", "title": "%off%", "action": "command:off"},
    {"id": "acme.report.stats", "title": "%stats%", "action": "command:stats"},
    {"id": "acme.report.stats.off", "title": "%off%", "action": "command:off"}
  ],
  "menus": {
    "toolbar.overflow": [
      {"command": "acme.report.toolbar", "group": "plugin", "order": 10, "when": "area == write"},
      {"command": "acme.report.toolbar.off", "group": "plugin", "order": 20, "when": "area == stats"}
    ],
    "note.overflow": [
      {"command": "acme.report.note", "group": "plugin", "order": 10, "when": "note.open && sub == notes"},
      {"command": "acme.report.note.off", "group": "plugin", "order": 20, "when": "sub == search"}
    ],
    "note.block": [
      {"command": "acme.report.block", "group": "plugin", "order": 10, "when": "note.selection == false"},
      {"command": "acme.report.block.off", "group": "plugin", "order": 20, "when": "note.selection"}
    ],
    "editor.slash": [
      {"command": "acme.report.slash", "group": "plugin", "order": 10, "when": "area == write"},
      {"command": "acme.report.slash.off", "group": "plugin", "order": 20, "when": "area == stats"}
    ],
    "review.card": [
      {"command": "acme.report.card", "group": "plugin", "order": 10, "when": "card.present && mode == review"},
      {"command": "acme.report.card.off", "group": "plugin", "order": 20, "when": "mode == browse"}
    ],
    "browse.row": [
      {"command": "acme.report.row", "group": "plugin", "order": 10, "when": "browse.selection"},
      {"command": "acme.report.row.off", "group": "plugin", "order": 20, "when": "mode == review"}
    ],
    "sidebar.note": [
      {"command": "acme.report.side", "group": "secondary", "order": 10, "when": "area == write"},
      {"command": "acme.report.side.off", "group": "secondary", "order": 20, "when": "area == stats"}
    ],
    "page.header": [
      {"command": "acme.report.header", "group": "secondary", "order": 10, "when": "page.id == report"},
      {"command": "acme.report.header.off", "group": "secondary", "order": 20, "when": "page.id == other"}
    ],
    "today.widget": [
      {"command": "acme.report.today", "group": "plugin", "order": 10, "when": "area == today"},
      {"command": "acme.report.today.off", "group": "plugin", "order": 20, "when": "area == stats"}
    ],
    "stats.block": [
      {"command": "acme.report.stats", "group": "plugin", "order": 10, "when": "area == stats && sub == overview"},
      {"command": "acme.report.stats.off", "group": "plugin", "order": 20, "when": "area == today"}
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
  "locales": {
    "en": {
      "toolbar": "Toolbar action",
      "note": "Note action",
      "block": "Block action",
      "slash": "Slash action",
      "card": "Card action",
      "row": "Row action",
      "side": "Sidebar action",
      "header": "Header action",
      "today": "Today's widget",
      "stats": "Stats block",
      "off": "Shadowed action",
      "page.title": "Report"
    },
    "zh-Hans": {
      "toolbar": "工具栏动作",
      "note": "笔记动作",
      "block": "块动作",
      "slash": "斜杠动作",
      "card": "卡片动作",
      "row": "行动作",
      "side": "侧栏动作",
      "header": "页首动作",
      "today": "今日小组件",
      "stats": "统计块",
      "off": "被遮住的动作",
      "page.title": "报告"
    }
  }
}`

// statusOnly declares the status line's slot, which the contract reserves: this
// host validates it, keeps the item, draws nothing, and lists the declaration in
// the settings page's problems (ui.md §4).
const statusOnly = `{
  "commands": [
    {"id": "acme.report.status", "title": "%status%", "action": "command:status"}
  ],
  "menus": {
    "status.item": [{"command": "acme.report.status", "group": "plugin", "when": "library.open"}]
  },
  "locales": {"en": {"status": "Status action"}}
}`

// overflowMenu opens the window's ⋯ and answers the rows it drew.
func overflowMenu(t *testing.T, tt *ui.Tester) []string {
	t.Helper()
	if err := tt.Click(locales.In("en").More()); err != nil {
		t.Fatalf("the window's overflow menu: %v", err)
	}
	return tt.Menu()
}

// scrollPopoverTo brings one row of an open popover into view: the block and
// slash menus are longer than the smallest window allows. from is a row — or a
// control beside the popover — the pointer can reach, since a row below the
// fold takes no input to scroll.
func scrollPopoverTo(t *testing.T, tt *ui.Tester, from, label string) {
	t.Helper()
	anchor, ok := tt.Find(from)
	if !ok {
		t.Fatalf("the popover is not open: no %q; texts %q", from, tt.Texts())
	}
	x, y := anchor.X+anchor.W/2, anchor.Y+anchor.H/2
	for i := 0; i < 12; i++ {
		if r, ok := tt.Find(label); ok && r.W > 0 && r.H > 0 {
			return
		}
		tt.Scroll(x, y, 0, 60)
		tt.Frame()
	}
	t.Fatalf("%q never scrolled into view; texts %q", label, tt.Texts())
}

// textIndex is where one text sits in the frame's drawing order, -1 for absent.
func textIndex(tt *ui.Tester, text string) int {
	for i, shown := range tt.Texts() {
		if shown == text {
			return i
		}
	}
	return -1
}

// inOrder checks that a receiver drew one of its rows after another: a plugin's
// items close a menu, they do not open one.
func inOrder(t *testing.T, tt *ui.Tester, later, earlier string) {
	t.Helper()
	at, before := textIndex(tt, later), textIndex(tt, earlier)
	if at < 0 {
		t.Fatalf("%q is not drawn; texts %q", later, tt.Texts())
	}
	if before < 0 {
		t.Fatalf("%q is not drawn; texts %q", earlier, tt.Texts())
	}
	if at < before {
		t.Errorf("%q is drawn before %q, want after", later, earlier)
	}
}

// The toolbar's ⋯ is the receiver for both docked slots: an item whose when holds
// is there and runs its command, one whose when does not is not, and the note's
// slot follows the page inside the write area.
func TestOverflowMenuDrawsTheToolbarAndNoteSlots(t *testing.T) {
	runner := &fakeRunner{}
	a, tt := openContrib(t, "report", everyWhere, runner)

	// Today: neither item's when holds, so neither is drawn.
	tt.Frame()
	if menu := strings.Join(overflowMenu(t, tt), "\n"); strings.Contains(menu, "Toolbar action") {
		t.Fatalf("the toolbar's item drew on Today: %q", menu)
	}
	tt.CloseMenu()

	// The notebook, a note open: both items, after the window's own rows.
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	tt.Frame()
	menu := overflowMenu(t, tt)
	if !slices.Contains(menu, "Toolbar action") || !slices.Contains(menu, "Note action") {
		t.Fatalf("the docked slots are missing from %q", menu)
	}
	if strings.Contains(strings.Join(menu, "\n"), "Shadowed action") {
		t.Errorf("an item whose when is false was drawn: %q", menu)
	}
	if last := menu[len(menu)-2]; last != "Note action" && menu[len(menu)-1] != "Note action" {
		t.Errorf("the plugin's items do not close the menu: %q", menu)
	}
	if err := tt.ChooseMenuItem("Note action"); err != nil {
		t.Fatalf("the note's item: %v", err)
	}
	if !runner.called("note") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}

	// The write area's search page is not the notebook: the note's item goes and
	// the toolbar's stays, which is what sub answers.
	searchPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond)
	menu = overflowMenu(t, tt)
	if !slices.Contains(menu, "Toolbar action") {
		t.Errorf("the toolbar's item left with the note's: %q", menu)
	}
	if slices.Contains(menu, "Note action") {
		t.Errorf("the note's item drew off the notebook: %q", menu)
	}
	tt.CloseMenu()

	// Another area: the toolbar's item names the write area and nothing else.
	tt.Key(ui.Cmd, ui.Key5)
	tt.Advance(320 * time.Millisecond)
	if menu := strings.Join(overflowMenu(t, tt), "\n"); strings.Contains(menu, "Toolbar action") {
		t.Errorf("the toolbar's item drew in the statistics area: %q", menu)
	}
}

// A paragraph's menu and its inspector both carry the note.block slot at their
// foot, and the words a note has selected are what note.selection answers.
func TestBlockMenuDrawsTheNoteBlockSlot(t *testing.T) {
	runner := &fakeRunner{}
	a, tt := openContrib(t, "report", everyWhere, runner)
	msgs := locales.In("en")

	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	// The first paragraph's handle: a note opens where the reader left it, and
	// the gutter follows the caret.
	a.notes.Doc.SetSelection(0, 0)
	tt.Frame()

	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatalf("the paragraph has no menu: %v", err)
	}
	tt.Frame()
	inOrder(t, tt, "Block action", msgs.OpenInspector())
	if tt.HasText("Shadowed action") {
		t.Errorf("an item whose when is false was drawn; texts %q", tt.Texts())
	}
	scrollPopoverTo(t, tt, msgs.DeleteBlock(), "Block action")
	if err := tt.Click("Block action"); err != nil {
		t.Fatalf("the plugin's block item: %v", err)
	}
	if !runner.called("block") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}

	// The inspector is the slot's second receiver: the same items, under the
	// host's own.
	tt.Frame()
	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatalf("the paragraph has no menu: %v", err)
	}
	tt.Frame()
	scrollPopoverTo(t, tt, msgs.DeleteBlock(), msgs.OpenInspector())
	if err := tt.Click(msgs.OpenInspector()); err != nil {
		t.Fatalf("open the inspector: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Block action") {
		t.Fatalf("the inspector has no plugin section; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	// Words selected in the note flip note.selection: the item whose when names
	// it leaves, and the other arrives.
	a.notes.Doc.SetSelection(0, len([]rune("Spacing")))
	tt.Frame()
	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatalf("the paragraph has no menu with a selection: %v", err)
	}
	tt.Frame()
	if tt.HasText("Block action") {
		t.Errorf("the item whose when the selection rejects was drawn; texts %q", tt.Texts())
	}
	if !tt.HasText("Shadowed action") {
		t.Errorf("the item whose when the selection holds is missing; texts %q", tt.Texts())
	}
}

// The `/` menu closes with the editor.slash slot's items: the plugin's own
// commands come after the host's, and the query that filters those filters them.
func TestSlashMenuDrawsTheEditorSlashSlot(t *testing.T) {
	runner := &fakeRunner{}
	a, tt := openContrib(t, "report", everyWhere, runner)
	msgs := locales.In("en")

	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	tt.Frame()

	// A slash opens the menu at the start of a paragraph with nothing after it,
	// so the test adds one from the first paragraph's gutter and types there.
	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	tt.Move(body.X-44, body.Y+12)
	tt.Frame()
	if err := tt.Click(msgs.AddBlockBelow()); err != nil {
		t.Fatalf("the gutter has no add: %v", err)
	}
	tt.Frame()
	tt.Type("/")
	tt.Frame()

	if !tt.HasText("Slash action") {
		t.Fatalf("the `/` menu has no plugin item; texts %q", tt.Texts())
	}
	inOrder(t, tt, "Slash action", msgs.SlashTable())
	if tt.HasText("Shadowed action") {
		t.Errorf("an item whose when is false was drawn; texts %q", tt.Texts())
	}

	// The query narrows the plugin's items as it narrows the host's, and
	// choosing one runs it with the typed text gone.
	tt.Type("slash action")
	tt.Frame()
	if !tt.HasText("Slash action") {
		t.Fatalf("the query filtered the plugin's item out; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !runner.called("slash") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}
	if tt.HasText("/slash action") {
		t.Errorf("the typed query stayed in the note; texts %q", tt.Texts())
	}
}

// The review session's footer carries the review.card slot for the card on
// screen, and an item whose when names another screen is not drawn there.
func TestReviewFooterDrawsTheReviewCardSlot(t *testing.T) {
	runner := &fakeRunner{}
	a, _ := openContrib(t, "report", everyWhere, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	// The queue page is up with no card on screen: nothing to act on.
	recallPage(tt)
	if got := a.reviewCardItems(); len(got) != 0 {
		t.Fatalf("the slot answered the queue page with %d items, want none", len(got))
	}

	recallQueue(tt)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("Card action") {
		t.Fatalf("the review's footer has no plugin action; texts %q", tt.Texts())
	}
	if tt.HasText("Shadowed action") {
		t.Errorf("an item whose when is false was drawn; texts %q", tt.Texts())
	}
	if err := tt.Click("Card action"); err != nil {
		t.Fatalf("the plugin's review action: %v", err)
	}
	if !runner.called("card") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}
}

// The card table's action row carries the browse.row slot, and browse.selection
// is what its items wait for: no row chosen, no action.
func TestCardTableDrawsTheBrowseRowSlot(t *testing.T) {
	runner := &fakeRunner{}
	a, _ := openContrib(t, "report", everyWhere, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	msgs := locales.In("en")

	cardsPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(msgs.RecallCardsNoActions()) {
		t.Fatalf("the action row does not say it has no plugin actions; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.RecallCardsSelect("Spacing beats cramming")); err != nil {
		t.Fatalf("choose a card row: %v", err)
	}
	tt.Frame()
	if err := tt.Click(msgs.RecallCardsActions()); err != nil {
		t.Fatalf("the plugin actions button is not a control: %v", err)
	}
	menu := tt.Menu()
	if !slices.Contains(menu, "Row action") {
		t.Fatalf("the chosen row's actions are missing: %q", menu)
	}
	if strings.Contains(strings.Join(menu, "\n"), "Shadowed action") {
		t.Errorf("an item whose when is false was drawn: %q", menu)
	}
	if err := tt.ChooseMenuItem("Row action"); err != nil {
		t.Fatalf("the plugin's row action: %v", err)
	}
	if !runner.called("row") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}
}

// A note row's menu carries the sidebar.note slot under the host's own rows.
func TestNoteRowMenuDrawsTheSidebarNoteSlot(t *testing.T) {
	runner := &fakeRunner{}
	a, tt := openContrib(t, "report", everyWhere, runner)
	msgs := locales.In("en")

	notesMode(t, a, tt)
	noteListPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click(msgs.WriteRowActions() + " Memory and recall"); err != nil {
		t.Fatalf("the note row has no menu: %v", err)
	}
	menu := tt.Menu()
	if !slices.Contains(menu, "Sidebar action") {
		t.Fatalf("the note row's menu has no plugin item: %q", menu)
	}
	if strings.Contains(strings.Join(menu, "\n"), "Shadowed action") {
		t.Errorf("an item whose when is false was drawn: %q", menu)
	}
	if err := tt.ChooseMenuItem("Sidebar action"); err != nil {
		t.Fatalf("the plugin's row action: %v", err)
	}
	if !runner.called("side") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}
}

// A note row's menu offers what the host does with a note — read it, add it to
// the reading queue, rename it, delete it — with the sidebar.note slot under
// them, and every one of them runs: an item that only draws is a dead control.
func TestNoteRowMenuOffersReadingAndThePluginSlot(t *testing.T) {
	runner := &fakeRunner{}
	a, tt := openContrib(t, "report", everyWhere, runner)
	msgs := locales.In("en")

	notesMode(t, a, tt)
	noteListPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond)
	openRowMenu := func() {
		t.Helper()
		if err := tt.Click(msgs.WriteRowActions() + " Memory and recall"); err != nil {
			t.Fatalf("the note row has no menu: %v", err)
		}
	}
	openRowMenu()
	menu := tt.Menu()
	for _, want := range []string{
		msgs.WriteRowRead(), msgs.WriteRowQueue(), msgs.WriteRowRename(),
		msgs.WriteRowDelete(), "Sidebar action",
	} {
		if !slices.Contains(menu, want) {
			t.Fatalf("the note row's menu has no %q: %q", want, menu)
		}
	}

	// Adding it to the reading queue is the menu's second row, and it reaches
	// the store: the queue the read area lists holds the note.
	if err := tt.ChooseMenuItem(msgs.WriteRowQueue()); err != nil {
		t.Fatalf("the queue item: %v", err)
	}
	tt.Frame()
	queued, err := a.Library().ReadingQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 1 {
		t.Fatalf("the row's queue action added %d notes", len(queued))
	}
	if note, ok := a.Library().Note(queued[0].Note); !ok || note.Title != "Memory and recall" {
		t.Fatalf("the row's queue action added %q", note.Title)
	}

	// Reading opens the note read whole, not for editing.
	openRowMenu()
	if err := tt.ChooseMenuItem(msgs.WriteRowRead()); err != nil {
		t.Fatalf("the read item: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.notes.View != notes.Read {
		t.Errorf("the row's read action left the note in view %v", a.notes.View)
	}
}

// A contributed page's header carries the page.header slot for that page, and
// the items run their commands.
func TestPageHeaderDrawsThePageHeaderSlot(t *testing.T) {
	runner := &fakeRunner{}
	_, tt := openContrib(t, "report", everyWhere, runner)

	openPage(t, tt, "Report")
	tt.Frame()
	if !tt.HasText("Header action") {
		t.Fatalf("the page's header has no plugin action; texts %q", tt.Texts())
	}
	if tt.HasText("Shadowed action") {
		t.Errorf("an item whose page.id is another page was drawn; texts %q", tt.Texts())
	}
	if err := tt.Click("Header action"); err != nil {
		t.Fatalf("the page's header action: %v", err)
	}
	if !runner.called("header") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}
}

// Today draws the today.widget slot as a section of its own, one per plugin, and
// an item whose when names another area is not drawn.
func TestTodayDrawsTheTodayWidgetSlot(t *testing.T) {
	runner := &fakeRunner{}
	_, tt := openContrib(t, "report", everyWhere, runner)

	// The notebook is not Today: no section, no heading for one.
	if tt.HasText("Today's widget") {
		t.Fatalf("the widget drew outside Today; texts %q", tt.Texts())
	}
	tt.Key(ui.Cmd, ui.Key1)
	tt.Advance(320 * time.Millisecond)
	reveal(t, tt, 1024, 720, "Today's widget")
	if err := tt.Click("Today's widget"); err != nil {
		t.Fatalf("today's plugin item is not a control: %v", err)
	}
	if !runner.called("today") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}
	if tt.HasText("Shadowed action") {
		t.Errorf("an item whose when is false was drawn; texts %q", tt.Texts())
	}
}

// The status line's slot is reserved: the host keeps the declaration, draws
// nothing in the window, and lists it in the settings page's problems, which is
// where a reader learns why nothing appeared (ui.md §4).
func TestReservedStatusItemIsListedAndNeverDrawn(t *testing.T) {
	runner := &fakeRunner{}
	a, tt := openContrib(t, "report", statusOnly, runner)
	msgs := locales.In("en")

	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	tt.Frame()
	if tt.HasText("Status action") {
		t.Fatalf("the reserved slot drew in the window; texts %q", tt.Texts())
	}
	if got := runner.calls(); got != 0 {
		t.Errorf("drawing the window entered the plugin %d times", got)
	}

	plugins := a.pluginProblems()
	problems := strings.Join(plugins["report"], "\n")
	if !strings.Contains(problems, "status.item") {
		t.Fatalf("the problems do not name the reserved slot: %q", problems)
	}
	settingsPage(t, tt, a, subSettingsPlugins)
	reveal(t, tt, 1024, 720, msgs.PluginProblems())
	if !tt.HasText("status.item") {
		t.Errorf("the settings page does not list the reserved declaration; texts %q", tt.Texts())
	}
}

// noteListPage puts the window on the write area's notes list, where a note row
// has its menu.
func noteListPage(t *testing.T, tt *ui.Tester, loc string) {
	t.Helper()
	tt.Key(ui.Cmd, ui.Key2)
	tt.Frame()
	if err := tt.Click(locales.In(loc).ModeNotes()); err != nil {
		t.Fatalf("the write area has no notes list: %v", err)
	}
}

// The plugin surfaces hold the smallest window in Chinese: the block menu's and
// the slash menu's items are drawn, and one is still reachable — a receiver that
// only works at the default size and in English is not a receiver.
func TestPluginSurfacesAtTheMinimumSizeInChinese(t *testing.T) {
	runner := &fakeRunner{}
	a, _ := openContrib(t, "report", everyWhere, runner)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	settle(tt)
	zh := locales.In("zh-Hans")

	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	a.notes.Doc.SetSelection(0, 0)
	tt.Frame()

	// The block menu's item is drawn, and the menu scrolls to it.
	if err := tt.Click(zh.BlockActionsFor(1)); err != nil {
		t.Fatalf("the paragraph has no menu: %v", err)
	}
	tt.Frame()
	if !tt.HasText("块动作") {
		t.Fatalf("the block menu has no plugin item in Chinese; texts %q", tt.Texts())
	}
	scrollPopoverTo(t, tt, zh.DuplicateBlock(), "块动作")
	if err := tt.Click("块动作"); err != nil {
		t.Fatalf("the plugin's block item at the minimum size: %v", err)
	}
	if !runner.called("block") {
		t.Errorf("the item drew but ran nothing; the plugin saw %v", runner.calls())
	}

	// The `/` menu's item too, reached by the query that names it.
	body, ok := tt.Find(zh.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	tt.Move(body.X-44, body.Y+12)
	tt.Frame()
	if err := tt.Click(zh.AddBlockBelow()); err != nil {
		t.Fatalf("the gutter has no add: %v", err)
	}
	tt.Frame()
	tt.Type("/")
	tt.Frame()
	if !tt.HasText("斜杠动作") {
		t.Fatalf("the `/` menu has no plugin item in Chinese; texts %q", tt.Texts())
	}
	tt.Type("斜杠动作")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !runner.called("slash") {
		t.Errorf("the `/` item did not run at the minimum size; the plugin saw %v", runner.calls())
	}
	if tt.HasText("/斜杠动作") {
		t.Errorf("the typed query stayed in the note; texts %q", tt.Texts())
	}
}

// A contribution three failures took out leaves its receiver and stays in the
// settings page with what failed and the way back: hidden is not forgotten, and
// Re-enable is an informed action.
func TestHiddenContributionStaysInSettingsWithItsReason(t *testing.T) {
	runner := &fakeRunner{failed: map[string]error{"block": errThrown}}
	a, tt := openContrib(t, "report", everyWhere, runner)
	msgs := locales.In("en")

	notesMode(t, a, tt)
	for i := 0; i < contrib.FailureLimit; i++ {
		tt.Key(ui.Cmd, ui.KeyK)
		tt.Type("block action")
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
	}
	tt.Advance(6 * time.Second) // the last toast goes

	// Out of its receiver.
	if got := slotIDs(a.noteBlockItems()); len(got) != 0 {
		t.Fatalf("a hidden contribution is still in its slot: %v", got)
	}

	// In the settings page, with what failed and the one action that shows it
	// again.
	settingsPage(t, tt, a, subSettingsPlugins)
	reveal(t, tt, 1024, 720, msgs.PluginFailures(3))
	if !tt.HasText(msgs.PluginFailures(3)) {
		t.Fatalf("the settings page does not count the failures; texts %q", tt.Texts())
	}
	if !tt.HasText("acme.report.block: " + msgs.PluginCommandFailed("Block action", errThrown.Error())) {
		t.Fatalf("the settings page does not say which contribution failed and why; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.PluginReenable()); err != nil {
		t.Fatalf("the re-enable action: %v", err)
	}
	tt.Frame()
	if got := slotIDs(a.noteBlockItems()); len(got) != 1 {
		t.Fatalf("re-enabling left the slot with %v, want the contribution back", got)
	}
}

// The whole when context is what the window is drawing: one table, walked stop by
// stop through the real window, read the way a manifest reads it — through the
// contract's own evaluator — so a key that answers the wrong thing is red where a
// plugin would see it.
//
// A key a stop does not name is one the window keeps: a note stays open in the
// notebook after the user looks at the statistics, and the scope a session took
// stays the next session's until it is changed. Those are stated where they are
// asked for, not left out.
func TestContributionContextFollowsTheWindow(t *testing.T) {
	runner := &fakeRunner{}
	a, seed := contribApp(t, "report", everyWhere, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	en := locales.In("en")

	check := func(stop string, want ...string) {
		t.Helper()
		ctx := a.contributionContext()
		for _, expr := range append([]string{"library.open"}, want...) {
			if !contrib.When(expr, ctx) {
				t.Errorf("%s: %q does not hold in %+v", stop, expr, ctx)
			}
		}
	}

	// Today: the mixed queue, no legacy screen, no contributed page.
	tt.Key(ui.Cmd, ui.Key1)
	tt.Advance(320 * time.Millisecond)
	check("today", `area == today`, `sub == ""`, `mode == ""`, `page.id == ""`, `plugin.id == ""`, `!note.open`)

	// The notebook: the notes screen with a note in it and nothing selected.
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	tt.Frame()
	check("the notebook", `area == write`, `sub == notes`, `mode == notes`, `note.open`, `!note.selection`)

	// A selection in the note is note.selection.
	a.notes.Doc.SetSelection(0, len([]rune("Spacing")))
	tt.Frame()
	check("a selection", `area == write`, `note.open`, `note.selection`)
	a.notes.Doc.SetSelection(0, 0)
	tt.Frame()

	// The write area's search page is the browse screen, as it always has been.
	searchPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond)
	check("search", `area == write`, `sub == search`, `mode == browse`)

	// The recall queue: the review screen, no card on it yet.
	recallPage(tt)
	tt.Advance(320 * time.Millisecond)
	check("the recall queue", `area == recall`, `sub == queue`, `mode == review`, `!card.present`, `!scope.active`)

	// The scope dialog narrows the sitting, which is scope.active, and the
	// session's card is card.present with its status.
	if err := tt.Click(scopeButtonName(en, en.ReviewScope())); err != nil {
		t.Fatalf("the queue page has no scope button: %v", err)
	}
	tt.Frame()
	clickRole(t, tt, ui.RoleRadio, en.RecallScopeNote())
	clickRole(t, tt, ui.RoleRadio, seed.memory.Title)
	if err := tt.Click(en.RecallScopeStart()); err != nil {
		t.Fatalf("start the scoped session: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	check("a scoped session", `mode == review`, `scope.active`, `card.present`, `card.status != ""`)

	// The card table: browse, and the rows checked are browse.selection.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	cardsPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond)
	check("the card table", `area == recall`, `sub == cards`, `mode == browse`, `!browse.selection`)
	if err := tt.Click(en.RecallCardsSelect("Spacing beats cramming")); err != nil {
		t.Fatalf("choose a card row: %v", err)
	}
	tt.Frame()
	check("a checked row", `browse.selection`)

	// The statistics area's pages carry their sub id.
	tt.Key(ui.Cmd, ui.Key5)
	tt.Advance(320 * time.Millisecond)
	check("the statistics overview", `area == stats`, `sub == overview`, `mode == stats`)

	// A contributed page: no legacy screen, and the page and plugin it is.
	openPage(t, tt, "Report")
	tt.Frame()
	check("a contributed page", `area == plugin`, `mode == ""`, `page.id == report`, `plugin.id == acme.report`)

	// The settings area is the settings screen.
	settingsPage(t, tt, a, subSettingsReview)
	check("the settings area", `area == settings`, `sub == review`, `mode == settings`, `page.id == ""`, `plugin.id == ""`)
}
