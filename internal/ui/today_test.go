package ui

import (
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/locales"
)

// todayApp is a library whose day holds every kind of line: two notes due to be
// read, one note read with no excerpt taken from it, and the seeded cards due.
func todayApp(t *testing.T) (*App, testLibrary) {
	t.Helper()
	a, seed := readingApp(t)
	now := time.Now()
	if err := a.lib.AddToReading(seed.habit.ID, now); err != nil {
		t.Fatalf("add to reading: %v", err)
	}
	// Read, so it leaves today's reading queue and comes back owing an excerpt.
	if err := a.lib.ReadNote(seed.habit.ID, schedule.Good, seed.habit.Blocks[0].ID,
		now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("read note: %v", err)
	}
	a.reload()
	return a, seed
}

// textAt is the first place a text is drawn, counted in the page's draw order,
// or -1 when the page does not say it.
func textAt(tt *ui.Tester, text string) int {
	for i, drawn := range tt.Texts() {
		if drawn == text {
			return i
		}
	}
	return -1
}

// statValue is one of the page's three numbers: the count kit.Stat draws above
// its label.
func statValue(t *testing.T, tt *ui.Tester, label string) string {
	t.Helper()
	texts := tt.Texts()
	for i, text := range texts {
		if text != label {
			continue
		}
		for j := i - 1; j >= max(0, i-3); j-- {
			if _, err := strconv.Atoi(texts[j]); err == nil {
				return texts[j]
			}
		}
		t.Fatalf("the %q stat has no number above it; texts %q", label, texts)
	}
	t.Fatalf("the page shows no %q stat; texts %q", label, texts)
	return ""
}

// The day is worked in the order it is drawn: the notes to read, the notes read
// and not yet excerpted, and the cards last. A page that put recall first — or
// that let the queue's own order slip — turns this red.
func TestTodayOrdersReadingBeforeReview(t *testing.T) {
	a, seed := todayApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	msgs := locales.In("en")

	order := []string{seed.memory.Title, seed.writing.Title, seed.habit.Title, msgs.TodayReviewTitle()}
	last := -1
	for _, text := range order {
		at := textAt(tt, text)
		if at < 0 {
			t.Fatalf("the day does not list %q; texts %q", text, tt.Texts())
		}
		if at <= last {
			t.Fatalf("%q is drawn before the line that should come before it; texts %q",
				text, tt.Texts())
		}
		last = at
	}
}

// A line past the day's budget says it waits for tomorrow, and the lines the
// budget pays for do not: a page that dropped the line or marked a whole day as
// postponed turns this red.
func TestTodayMarksWhatIsOverBudget(t *testing.T) {
	a, seed := todayApp(t)
	presets := a.Library().Presets()
	presets.Budget.Minutes = 6 // one sitting: the day's first reading line
	if err := a.Library().SetPresets(presets); err != nil {
		t.Fatalf("set the budget: %v", err)
	}
	a.reload()

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	msgs := locales.In("en")

	first := textAt(tt, seed.memory.Title)
	second := textAt(tt, seed.writing.Title)
	if first < 0 || second < first {
		t.Fatalf("the day does not list the notes due to read; texts %q", tt.Texts())
	}
	marked := 0
	for i, text := range tt.Texts() {
		if text != msgs.TodayOverBudget() {
			continue
		}
		marked++
		if i < second {
			t.Errorf("the badge at %d marks a line the budget pays for; texts %q", i, tt.Texts())
		}
	}
	if marked == 0 {
		t.Fatalf("no line is marked as waiting for tomorrow; texts %q", tt.Texts())
	}
}

// The page's numbers are the queues' own: what the rail counts is what the three
// stats say, and grading a card moves both. A page that counted something else —
// the whole library, or yesterday — turns this red.
func TestTodayCountsMatchTheRail(t *testing.T) {
	a, _ := todayApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	msgs := locales.In("en")

	for _, stat := range []struct{ label, rail string }{
		{msgs.TodayStatReading(), msgs.AreaRead()},
		{msgs.TodayStatRecall(), msgs.AreaRecall()},
	} {
		if got, want := statValue(t, tt, stat.label), railCount(t, tt, stat.rail); got != want {
			t.Errorf("the page says %q %s, the rail %q", got, stat.label, want)
		}
	}

	// A card graded today is one recalled today, and one card fewer to recall.
	recallQueue(tt)
	tt.Advance(320 * time.Millisecond) // the queue's entrance
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key3)
	tt.Advance(320 * time.Millisecond) // the card's exit
	tt.Key(ui.Cmd, ui.Key1)
	tt.Advance(320 * time.Millisecond) // Today's entrance

	if got := statValue(t, tt, msgs.TodayStatRecalled()); got != "1" {
		t.Errorf("the page says %q recalled today, want the card just graded", got)
	}
	if got, want := statValue(t, tt, msgs.TodayStatRecall()), railCount(t, tt, msgs.AreaRecall()); got != want {
		t.Errorf("after one grade the page says %q to recall, the rail %q", got, want)
	}
}

// Today's note is on the page before it is written, waiting to be opened; opening
// it writes the day's note, and the row follows what the library holds. A row
// that offered nothing, or that never noticed the note it made, turns this red.
func TestTodayOffersTheDailyNote(t *testing.T) {
	a, _ := todayApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	msgs := locales.In("en")
	title := msgs.TodayDaily(time.Now())

	if !tt.HasText(title) || !tt.HasText(msgs.TodayDailyNone()) {
		t.Fatalf("the day offers no note of its own; texts %q", tt.Texts())
	}
	if err := tt.Click(title); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond) // the notebook's entrance
	if a.area != areaWrite {
		t.Fatalf("opening today's note left the window on %v", a.area)
	}
	if _, ok := a.Library().Note(core.ID(a.notes.Loaded)); !ok {
		t.Fatalf("opening today's note wrote nothing: the editor holds %q", a.notes.Loaded)
	}

	// The day's note is a note like any other, and the row says so.
	tt.Key(ui.Cmd, ui.Key1)
	tt.Advance(320 * time.Millisecond)
	if tt.HasText(msgs.TodayDailyNone()) {
		t.Errorf("the row still says the day has no note; texts %q", tt.Texts())
	}
	if !tt.HasText(title) || !tt.HasText(msgs.TodayDailyEmpty()) {
		t.Errorf("the row does not name the day's note; texts %q", tt.Texts())
	}
}

// A row of the day opens the note it names in the reading session, at the
// paragraph the last session stopped at: a row that opened the first paragraph,
// or the wrong note, turns this red.
func TestTodayOpensAReadingLineAtItsParagraph(t *testing.T) {
	a, seed := newTestApp(t)
	now := time.Now()
	// A note due before it, so the session's own head is not the row clicked.
	if err := a.lib.AddToReading(seed.memory.ID, now.Add(-5*24*time.Hour)); err != nil {
		t.Fatalf("add to reading: %v", err)
	}
	if err := a.lib.AddToReading(seed.habit.ID, now.Add(-4*24*time.Hour)); err != nil {
		t.Fatalf("add to reading: %v", err)
	}
	// Read three days ago, so it is due again and the day knows where it stopped.
	if err := a.lib.ReadNote(seed.habit.ID, schedule.Again, seed.habit.Blocks[1].ID,
		now.Add(-3*24*time.Hour)); err != nil {
		t.Fatalf("read note: %v", err)
	}
	a.reload()

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	msgs := locales.In("en")
	if !tt.HasText(msgs.TodayParagraph(2, 2)) {
		t.Fatalf("the line does not say where the reader stopped; texts %q", tt.Texts())
	}
	if err := tt.Click(seed.habit.Title); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond) // the reading session's entrance

	if a.area != areaRead {
		t.Fatalf("the row left the window on %v, want the reading session", a.area)
	}
	if id, ok := a.reading.Current(); !ok || id != seed.habit.ID {
		t.Fatalf("the session opened on %q, want %q", id, seed.habit.ID)
	}
	if got := a.reading.Cursor(a.readingNoteOrEmpty(seed.habit.ID)); got != seed.habit.Blocks[1].ID {
		t.Errorf("the session opened at %q, want the paragraph it stopped at", got)
	}
}

// The page's controls go where they say: Start takes the first line, the review
// line opens the recall queue, and the command button opens the palette.
func TestTodayControlsGoWhereTheySay(t *testing.T) {
	a, _ := todayApp(t)
	// A window tall enough to show the whole day: the review line comes last,
	// and a click needs its row on screen.
	tt := ui.NewTester(a.View, 1024, 960)
	settle(tt)
	msgs := locales.In("en")

	if err := tt.Click(msgs.TodayStart()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.area != areaRead {
		t.Fatalf("Start left the window on %v, want the day's first line", a.area)
	}

	tt.Key(ui.Cmd, ui.Key1)
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click(msgs.TodayReviewTitle()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.area != areaRecall || a.sub != subRecallQueue {
		t.Fatalf("the review line opened %v/%v, want the recall queue", a.area, a.sub)
	}

	tt.Key(ui.Cmd, ui.Key1)
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click(msgs.TodayCommands()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond)
	if !a.palette.open || !tt.HasText(msgs.SearchActions()) {
		t.Error("the command button did not open the palette")
	}
}

// The day holds the smallest window in Chinese: its own controls are on screen
// and clickable there, and the page has not been squeezed into a blank.
func TestTodayInChineseAtTheMinimumWindow(t *testing.T) {
	a, _ := todayApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	settle(tt)
	msgs := locales.In("zh-Hans")

	for _, label := range []string{
		msgs.TodayTitle(), msgs.TodayDay(time.Now(), 30), msgs.TodayCommands(),
		msgs.TodayBudget(), msgs.TodayDaily(time.Now()), msgs.TodayStatReading(),
		msgs.TodayQueue(), msgs.TodayStart(),
	} {
		assertControl(t, tt, 720, 520, label)
	}
}
