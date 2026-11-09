package ui

import (
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// The rail is 240 wide with its labels while there is room, and the icon strip
// at 64 once the window reaches BreakpointNarrow: at that width the labels are
// gone, the areas still name themselves to the pointer that rests on them, and
// the counts stay.
func TestRailCollapsesBelow980(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1200, 720)
	settle(tt)
	msgs := locales.In("en")

	rail := func() ui.Rect {
		t.Helper()
		info, ok := tt.Access(msgs.AreaNav())
		if !ok {
			t.Fatalf("the rail has no tree of its own; texts %q", tt.Texts())
		}
		return info.Bounds
	}

	if got := rail().W; got < design.RailWidth-2 || got > design.RailWidth {
		t.Fatalf("the open rail is %v wide, want %v", got, design.RailWidth)
	}
	wide := areaItem(t, tt, msgs.AreaRecall()).Bounds.W
	if wide < design.SpaceXL*4 {
		t.Fatalf("the open rail's item is %v wide, too narrow to hold its label", wide)
	}

	tt.SetSize(int(design.BreakpointNarrow), 720)
	tt.Frame()
	if got := rail().W; got < design.RailCollapsed-2 || got > design.RailCollapsed {
		t.Fatalf("the rail at %v is %v wide, want %v", design.BreakpointNarrow, got, design.RailCollapsed)
	}
	if got := areaItem(t, tt, msgs.AreaRecall()).Bounds.W; got >= wide {
		t.Fatalf("the folded item is %v wide, as wide as the labeled one: the label is still drawn", got)
	}

	// Each item keeps its name: assistive technology reads it, and a tooltip
	// says it where the pointer rests.
	item, ok := tt.Access(msgs.AreaRecall())
	if !ok {
		t.Fatalf("the folded item has no accessible name; texts %q", tt.Texts())
	}
	tt.Move(item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2)
	tt.Advance(time.Second)
	if !tt.HasText(msgs.AreaRecall()) {
		t.Errorf("resting on the folded item says nothing; texts %q", tt.Texts())
	}

	// The counts are still there, one digit wide.
	if !tt.HasText("9+") && !tt.HasText("4") {
		t.Errorf("the folded rail lost its counts; texts %q", tt.Texts())
	}

	// Opening the rail again is the control's own doing, and it comes back
	// with its labels.
	tt.ClickAt(item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2)
	tt.SetSize(1200, 720)
	tt.Frame()
	if got := rail().W; got < design.RailWidth-2 || got > design.RailWidth {
		t.Fatalf("the rail did not open again: %v wide, want %v", got, design.RailWidth)
	}
}

// areaItem is one area's own box on the rail.
func areaItem(t *testing.T, tt *ui.Tester, name string) ui.AccessInfo {
	t.Helper()
	info, ok := tt.Access(name)
	if !ok {
		t.Fatalf("the rail has no %q item; texts %q", name, tt.Texts())
	}
	return info
}

// The rail counts what each area holds: the notes to read and the cards due are
// the queues' own numbers, and an empty queue has no badge at all.
func TestRailCountsMatchTheQueues(t *testing.T) {
	a, seed := readingApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	msgs := locales.In("en")

	if got := railCount(t, tt, msgs.AreaRead()); got != "2" {
		t.Errorf("the rail says %q notes to read, want the queue's 2", got)
	}
	if got := railCount(t, tt, msgs.AreaRecall()); got != "4" {
		t.Errorf("the rail says %q cards due, want the queue's 4", got)
	}
	for _, area := range []string{msgs.AreaStats(), msgs.AreaSettings()} {
		if got := railCount(t, tt, area); got != "" {
			t.Errorf("%s holds no queue and shows the badge %q", area, got)
		}
	}

	// Grading a card moves the count with the queue.
	recallQueue(tt)
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key3)
	tt.Key(0, ui.KeyEscape) // out of the session, back to the queue page
	tt.Frame()
	if got := railCount(t, tt, msgs.AreaRecall()); got != "3" {
		t.Errorf("after one grade the rail says %q cards due, want 3", got)
	}

	// A queue that empties loses its badge rather than showing a zero.
	if err := a.Library().RemoveFromReading(seed.memory.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := a.Library().RemoveFromReading(seed.writing.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt.Frame()
	if got := railCount(t, tt, msgs.AreaRead()); got != "" {
		t.Errorf("an empty reading queue shows the badge %q, want none", got)
	}
}

// railCount is the number the rail shows beside an area. The rail draws the
// item's name and then, when it has one, its badge, so the badge is the first
// number among the labels right after the item's own.
func railCount(t *testing.T, tt *ui.Tester, label string) string {
	t.Helper()
	texts := tt.Texts()
	for i, text := range texts {
		if text != label {
			continue
		}
		for _, next := range texts[i+1 : min(i+4, len(texts))] {
			if _, err := strconv.Atoi(next); err == nil {
				return next
			}
		}
		return ""
	}
	t.Fatalf("the rail shows no %q area; texts %q", label, texts)
	return ""
}

// A page with pages of its own keeps the one chosen through a reload: a write
// from the page rederives the library, and the window stays where the reader
// left it.
func TestSubnavKeepsTheAreaOnReload(t *testing.T) {
	a, _ := readingApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	msgs := locales.In("en")

	// The read area's second page is kept through a reload: a write that
	// rederives the library leaves the window where the reader put it.
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(msgs.QueuePage()) {
		t.Fatalf("the read area has no queue page; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.ArchivePage()); err != nil {
		t.Fatalf("the read area has no archive page: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	a.reload()
	tt.Frame()
	if a.area != areaRead || a.sub != subReadArchive {
		t.Fatalf("the reload moved the window to %v/%v, want the archive", a.area, a.sub)
	}
	if !tt.HasText(msgs.ReadArchiveEmpty()) {
		t.Fatalf("the archive page is not up after the reload; texts %q", tt.Texts())
	}

	// The recall area's card table is not its first page: a reload keeps the
	// page chosen, not the area's default.
	cardsPage(t, tt, "en")
	tt.Advance(320 * time.Millisecond)
	if a.sub != subRecallCards {
		t.Fatalf("the card table did not open; on %v", a.sub)
	}
	a.reload()
	tt.Frame()
	if a.area != areaRecall || a.sub != subRecallCards {
		t.Fatalf("the reload moved the window to %v/%v, want the card table", a.area, a.sub)
	}
	if !tt.HasText(msgs.BrowseTitle()) {
		t.Fatalf("the card table is not up after the reload; texts %q", tt.Texts())
	}
	// The tabs say which one is chosen, so the page is not only in the state.
	if !tt.HasText(msgs.CardsPage()) {
		t.Errorf("the sub-navigation lost its pages; texts %q", tt.Texts())
	}
}

// The smallest window scrolls nothing sideways: the frame fits its width
// exactly — the folded rail, the bar's controls and the page tabs — and each
// page scrolls inside its own box.
func TestNoHorizontalScrollAt720(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)
	msgs := locales.In("en")

	for _, page := range []struct {
		key  ui.Key
		want string
	}{
		{ui.Key1, msgs.TodayBudget()},
		{ui.Key2, msgs.Notebook()},
		{ui.Key3, msgs.ReadQueueEmpty()},
		{ui.Key4, msgs.RecallStart()},
		{ui.Key5, msgs.StatsTitle()},
		{ui.Key6, msgs.SettingsAppearanceLead()},
	} {
		tt.Key(ui.Cmd, page.key)
		tt.Advance(320 * time.Millisecond)
		if !tt.HasText(page.want) {
			t.Fatalf("Cmd+%v did not show its page: missing %q; texts %q", page.key, page.want, tt.Texts())
		}
		// The frame's own controls are inside the window, and the bar reaches
		// its right edge: there is no room left over for the window to scroll
		// into.
		for _, label := range []string{msgs.AreaNav(), msgs.More(), msgs.SearchActions(), msgs.Path()} {
			info, ok := tt.Access(label)
			if !ok {
				if label == msgs.SearchActions() {
					continue // no library, no palette button
				}
				t.Fatalf("the frame has no %q; texts %q", label, tt.Texts())
			}
			if info.Bounds.X < 0 || info.Bounds.X+info.Bounds.W > 720.5 {
				t.Errorf("%q is outside the 720-wide window: %+v", label, info.Bounds)
			}
		}
		more, _ := tt.Access(msgs.More())
		if more.Bounds.X+more.Bounds.W < 720-3*design.SpaceS {
			t.Errorf("the bar ends at %v, short of the window's 720", more.Bounds.X+more.Bounds.W)
		}
	}
}
