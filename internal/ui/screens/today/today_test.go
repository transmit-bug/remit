package today_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/screens/today"
	"github.com/transmit-bug/remit/locales"
)

// day is the day the page is shown for: a fixed date, so the tests say nothing
// about the machine's clock.
var day = time.Date(2026, 9, 18, 9, 30, 0, 0, time.UTC)

// state is a day that has loaded: two notes read and one line of cards, with
// three cards already recalled.
func state() today.State {
	return today.State{Day: day, Loaded: true, Recalled: 3}
}

// dayQueue is the queue those tests draw: a note read to its seventh paragraph,
// one read and not yet excerpted past the budget, and the cards as one line.
func dayQueue() queue.Today {
	return queue.Today{
		Entries: []queue.Entry{
			{Kind: queue.EntryRead, Note: "note_a", Title: "Memory and recall",
				Position: "blk_7", Blocks: 14, Read: 7, Minutes: 6},
			{Kind: queue.EntryExcerpt, Note: "note_b", Title: "A daily habit",
				Position: "blk_5", Blocks: 6, Read: 5, Minutes: 6, Over: true},
			{Kind: queue.EntryReview, Cards: 19, New: 4, Minutes: 11},
		},
		ReadMinutes: 12, ReviewMinutes: 11, TotalMinutes: 23, Budget: 30,
	}
}

// calls records what a control ran, by name and argument.
type calls struct{ ran []string }

func (c *calls) record(what string) { c.ran = append(c.ran, what) }

// deps is the page's actions as a test drives them: each records that it ran,
// and the plugin's messages resolve through the fake catalog below.
func deps(c *calls, day queue.Today, widgets []contrib.MenuRef) today.Deps {
	return today.Deps{
		Today:         func() queue.Today { return day },
		StartReview:   func() { c.record("review") },
		OpenReading:   func(note core.ID) { c.record("read " + string(note)) },
		OpenReadingAt: func(note, block core.ID) { c.record("read " + string(note) + " at " + string(block)) },
		OpenPalette:   func() { c.record("palette") },
		OpenDaily:     func() { c.record("daily") },
		NewNote:       func() { c.record("new note") },
		Daily:         func() (core.Note, bool) { return core.Note{}, false },
		Tomorrow:      func() int { return 9 },
		Widgets:       func() []contrib.MenuRef { return widgets },
		WidgetText:    widgetText,
		RunWidget:     func(ref contrib.CommandRef) { c.record("widget " + ref.Command.ID) },
		Reload:        func() { c.record("reload") },
	}
}

// widgetText is a plugin's catalog as the page reads it: the one message the
// fake plugin declares, and everything else as written.
func widgetText(_, raw string) string {
	if raw == "%command.report%" {
		return "Open the report"
	}
	return raw
}

// at draws the page at a size and lets it settle.
func at(t *testing.T, s today.State, d today.Deps, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { today.View(c, &s, d) }, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// The page says what the day holds and what it costs: the budget, the three
// counts, each line's progress and minutes, the excerpts still owed and what
// tomorrow brings.
func TestTodayDrawsTheDay(t *testing.T) {
	msgs := locales.In("en")
	tt := at(t, state(), deps(&calls{}, dayQueue(), nil), 1024, 720)

	want := []string{
		msgs.TodayTitle(),
		msgs.TodayDay(day, 30),
		msgs.TodayMinutes(23),
		msgs.TodayBudgetCaption(12, 11),
		msgs.TodayDaily(day),
		msgs.TodayDailyNone(),
		"2", msgs.TodayStatReading(),
		"19", msgs.TodayStatRecall(),
		"3", msgs.TodayStatRecalled(),
		msgs.TodayQueue(),
		"Memory and recall", msgs.TodayParagraph(7, 14), msgs.TodayMinutesTail(6),
		"A daily habit", msgs.TodayExcerptMeta(5, 6),
		msgs.TodayReviewTitle(), msgs.TodayReviewMeta(19, 4), msgs.TodayMinutesTail(11),
		msgs.TodayExcerpts(),
		msgs.TodayTomorrowTitle(), msgs.TodayTomorrow(9),
	}
	for _, text := range want {
		if !tt.HasText(text) {
			t.Errorf("the page does not say %q; texts %q", text, tt.Texts())
		}
	}
	if tt.FramePending() {
		t.Error("the page still requests frames after it should settle")
	}
}

// An empty day is one sentence and the one action that fills it: no queue, and
// no excerpt reminder for notes that do not exist.
func TestTodayEmptyDayOffersANote(t *testing.T) {
	msgs := locales.In("en")
	ran := &calls{}
	empty := queue.Today{Budget: 30}
	tt := at(t, state(), deps(ran, empty, nil), 1024, 720)

	if !tt.HasText(msgs.TodayEmpty()) {
		t.Fatalf("an empty day says nothing; texts %q", tt.Texts())
	}
	if tt.HasText(msgs.TodayQueue()) {
		t.Errorf("an empty day draws a queue; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.TodayEmptyAction()); err != nil {
		t.Fatal(err)
	}
	if len(ran.ran) != 1 || ran.ran[0] != "new note" {
		t.Errorf("the empty day's action ran %v, want a new note", ran.ran)
	}
}

// A day whose queue could not be built says why and offers to load it again,
// rather than drawing an empty day as if the library had nothing in it.
func TestTodaySaysWhyItCouldNotLoad(t *testing.T) {
	msgs := locales.In("en")
	ran := &calls{}
	s := state()
	s.Loaded, s.Err = false, "store: closed"
	d := deps(ran, queue.Today{}, nil)
	tt := at(t, s, d, 1024, 720)

	if !tt.HasText(msgs.TodayLoadFailed("store: closed")) {
		t.Fatalf("a failed load says nothing; texts %q", tt.Texts())
	}
	if tt.HasText(msgs.TodayEmpty()) || tt.HasText(msgs.TodayStatReading()) {
		t.Errorf("a failed load draws a day; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.TodayRetry()); err != nil {
		t.Fatal(err)
	}
	if len(ran.ran) != 1 || ran.ran[0] != "reload" {
		t.Errorf("the retry ran %v, want the load again", ran.ran)
	}
}

// A day that has not loaded yet does not claim it is empty: the page draws its
// skeleton and says nothing about a queue it does not have.
func TestTodayDoesNotClaimAnUnloadedDayIsEmpty(t *testing.T) {
	msgs := locales.In("en")
	s := state()
	s.Loaded = false
	tt := at(t, s, deps(&calls{}, queue.Today{}, nil), 1024, 720)

	if tt.HasText(msgs.TodayEmpty()) || tt.HasText(msgs.TodayQueue()) {
		t.Errorf("an unloaded day draws its queue; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.TodayTitle()) {
		t.Errorf("an unloaded day does not even name itself; texts %q", tt.Texts())
	}
}

// Plugins contribute sections to the day: one per plugin, headed by its name, and
// a slot with no items draws neither a heading nor a button that does nothing.
func TestTodayDrawsAPluginSectionOnlyWhenItHasItems(t *testing.T) {
	item := contrib.MenuRef{
		CommandRef: contrib.CommandRef{
			Plugin: "acme.report", Name: "report",
			Command: contrib.Command{ID: "acme.report.today", Title: "%command.report%"},
		},
		Group: "plugin",
	}

	// Nothing contributed: nothing drawn.
	quiet := at(t, state(), deps(&calls{}, dayQueue(), nil), 1024, 720)
	if quiet.HasText("report") || quiet.HasText("Open the report") {
		t.Fatalf("a slot with no items drew a section; texts %q", quiet.Texts())
	}

	// A window tall enough to hold the whole day: the section comes after the
	// queue, and a click needs the button on screen.
	ran := &calls{}
	tt := at(t, state(), deps(ran, dayQueue(), []contrib.MenuRef{item}), 1024, 1200)
	if !tt.HasText("report") {
		t.Fatalf("the plugin's section is not drawn; texts %q", tt.Texts())
	}
	if err := tt.Click("Open the report"); err != nil {
		t.Fatal(err)
	}
	if len(ran.ran) != 1 || ran.ran[0] != "widget acme.report.today" {
		t.Errorf("the widget button ran %v, want its command", ran.ran)
	}
}

// The page holds the smallest window in Chinese: every control is on screen, and
// nothing reaches past the window's edge.
func TestTodayFitsTheSmallWindowInChinese(t *testing.T) {
	msgs := locales.In("zh-Hans")
	const w, h = 720, 520

	s := state()
	tt := at(t, s, deps(&calls{}, dayQueue(), nil), w, h)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	// The head, the budget, the daily row and the queue's own heading: the
	// list's rows scroll inside the paper at this size, so what must fit is the
	// paper itself, measured by the meter that spans it.
	for _, text := range []string{
		msgs.TodayTitle(),
		msgs.TodayDay(day, 30),
		msgs.TodayBudget(),
		msgs.TodayDaily(day),
		msgs.TodayStatRecalled(),
		msgs.TodayQueue(),
		msgs.TodayStart(),
	} {
		box, ok := tt.Find(text)
		if !ok {
			t.Fatalf("Chinese page has no %q; texts %q", text, tt.Texts())
		}
		if box.W <= 0 || box.H <= 0 || box.X < 0 || box.X+box.W > w {
			t.Errorf("%q is off the %d-wide window: %+v", text, w, box)
		}
	}
	if tt.FramePending() {
		t.Error("the page still requests frames after it should settle")
	}
}
