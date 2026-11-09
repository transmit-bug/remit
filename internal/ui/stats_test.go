package ui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	corestats "github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
	screenstats "github.com/transmit-bug/remit/internal/ui/screens/stats"
	"github.com/transmit-bug/remit/locales"
)

// seedStatsLibrary writes a library with a week of work to replay: two cards,
// one of them reviewed twice so the retention has a prediction to compare with,
// a reading session so the calendar and the time share hold both kinds of
// activity, and the excerpt that session produced. Both cards are scheduled
// ahead, and so is the note, so the forecast merges them.
func seedStatsLibrary(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	note, err := lib.SaveNote(core.Note{Title: "Progress", Blocks: []core.Block{
		{Text: "First card :: answer"},
		{Text: "Second card :: answer"},
	}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	first, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	second, err := firstCard(lib, note.Blocks[1].ID)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	// A day apart, so the second review has a prediction to compare with.
	for _, review := range []struct {
		card core.ID
		at   time.Time
	}{
		{first.ID, now.Add(-26 * time.Hour)},
		{first.ID, now.Add(-2 * time.Hour)},
		{second.ID, now.Add(-time.Hour)},
	} {
		if err := lib.Review(review.card, schedule.Good, review.at); err != nil {
			t.Fatalf("review: %v", err)
		}
	}
	// Reading and the excerpt it left: a memory gets made both ways, and the
	// page has to price both.
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("read note: %v", err)
	}
	if _, err := lib.Excerpt(note.ID, note.Blocks[0].ID, "First card", "First card, taken out"); err != nil {
		t.Fatalf("excerpt: %v", err)
	}
	return NewApp(Options{LibraryDir: dir})
}

// openStats enters the statistics area the way the user does: from the command
// palette, which lands on the area's first page.
func openStats(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	tt.Type("stat")
	tt.Key(0, ui.KeyEnter)
	if a.palette.open {
		t.Fatalf("the palette stayed open; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
}

// openStatsArea enters the statistics area by its area key, which works in any
// language.
func openStatsArea(tt *ui.Tester) {
	tt.Key(ui.Cmd, ui.Key5)
	tt.Advance(320 * time.Millisecond) // the page's entrance
}

// openStatsPage enters the area and then one of its pages, by the name the
// sub-navigation gives it.
func openStatsPage(t *testing.T, a *App, tt *ui.Tester, page string) {
	t.Helper()
	openStats(t, a, tt)
	if err := tt.Click(page); err != nil {
		t.Fatalf("the statistics area has no %q page: %v", page, err)
	}
	tt.Advance(320 * time.Millisecond) // the page's entrance
}

// The overview says what the library has produced and how its time divided:
// both are loaded from the index and the log, and both are drawn with their
// numbers rather than as a proportion alone.
func TestStatsShowsProductionAndRatio(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a := seedStatsLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			openStats(t, a, tt)

			// The store's own answer, as the overview drew it. A page that read
			// something else — cards without excerpts, a stale count — is red
			// here.
			production, share := a.stats.Production, a.stats.TimeShare
			if want := corestats.ProductionOf(a.lib); production != want {
				t.Fatalf("the page drew the production %+v, the library has %+v", production, want)
			}
			if share.Read == 0 || share.Review == 0 || share.Excerpt == 0 {
				t.Fatalf("the seeded week prices no reading, reviewing and excerpting: %+v", share)
			}
			want := []string{
				msgs.StatsProduction(),
				msgs.StatsNumber(production.Characters), msgs.StatsCharacters(),
				msgs.StatsNumber(production.Excerpts), msgs.StatsExcerpts(),
				msgs.StatsCardsPerExcerpt(strconv.FormatFloat(production.CardsPerExcerpt, 'f', -1, 64)),
				msgs.StatsNumber(production.Cards), msgs.StatsCards(),
				msgs.StatsCardsFromNotes(production.Notes),
				msgs.StatsTimeShare(),
				msgs.StatsShareMinutes(msgs.StatsShareRead(), share.Read),
				msgs.StatsShareMinutes(msgs.StatsShareReview(), share.Review),
				msgs.StatsShareMinutes(msgs.StatsShareExcerpt(), share.Excerpt),
				msgs.StatsTimeShareNote(),
			}
			if missing := missingText(tt, want...); missing != "" {
				t.Fatalf("the overview is missing %q; texts %q", missing, tt.Texts())
			}
			for _, empty := range []string{msgs.StatsProductionEmpty(), msgs.StatsTimeShareEmpty()} {
				if textSays(tt, empty) {
					t.Errorf("the overview shows the empty state %q beside its data", empty)
				}
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// The stats.block slot is drawn by the host: one section per plugin, its items
// as buttons, and a button runs the command it names — the frame never asked
// the plugin for any of it.
func TestStatsBlocksComeFromPlugins(t *testing.T) {
	runner := &fakeRunner{}
	a, _ := contribApp(t, "report", everySlot, runner)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	tt.Key(ui.Cmd, ui.Key5) // the statistics area
	tt.Advance(320 * time.Millisecond)
	msgs := locales.In("en")
	if !tt.HasText(msgs.StatsTitle()) {
		t.Fatalf("Cmd+5 did not open the statistics area; texts %q", tt.Texts())
	}
	// The headings are the plugins' names, and the item is its own button.
	if got := len(a.statsBlockItems()); got != 1 {
		t.Fatalf("the slot holds %d items in the stats area, want 1", got)
	}
	reveal(t, tt, 1024, 720, "Stats block")
	if !tt.HasText("report") {
		t.Errorf("the plugin's section has no heading; texts %q", tt.Texts())
	}
	if err := tt.Click("Stats block"); err != nil {
		t.Fatalf("the plugin's item is not a control: %v", err)
	}
	if got := runner.calls(); got != 1 {
		t.Fatalf("the button entered the plugin %d times, want once", got)
	}

	// The item's when names the overview: on the calendar page the slot answers
	// nothing, and the section is not drawn with an empty heading.
	if err := tt.Click(msgs.StatsHeat()); err != nil {
		t.Fatalf("the calendar page: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if got := len(a.statsBlockItems()); got != 0 {
		t.Errorf("stats.block answered on the calendar page: %d items", got)
	}
	if tt.HasText("Stats block") {
		t.Errorf("the calendar page drew the section anyway; texts %q", tt.Texts())
	}
	if got := runner.calls(); got != 1 {
		t.Errorf("drawing the pages entered the plugin again: %d calls", got)
	}
}

// The calendar page says where its squares come from: the plaintext log, not
// the index — so deleting the index and rebuilding it changes nothing there.
func TestHeatmapExplainsItsSource(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a := seedStatsLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			openStatsPage(t, a, tt, msgs.StatsHeat())

			if missing := missingText(tt,
				msgs.StatsHeatSub(), msgs.StatsHeatSource(),
				msgs.StatsWeeks(len(a.stats.Heat)), msgs.StatsRange(),
				msgs.StatsRangeSpanTotal(a.stats.HeatFrom, a.stats.HeatTo, a.stats.HeatTotal),
				msgs.StatsLongestStreak(), msgs.StatsDays(a.stats.Streak),
			); missing != "" {
				t.Fatalf("the calendar page is missing %q; texts %q", missing, tt.Texts())
			}
			// Every day the log holds a review on has a square, named with that
			// day and its count: the squares are the log's, not a summary's.
			labelled := 0
			for _, week := range a.stats.Heat {
				for _, day := range week {
					if day.Count == 0 {
						continue
					}
					labelled++
					if want := msgs.StatsDayCount(day.Day, day.Count); !tt.HasText(want) {
						t.Errorf("the calendar has no square for %q; texts %q", want, tt.Texts())
					}
				}
			}
			if labelled == 0 {
				t.Error("the populated calendar drew no day at all")
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// One forecast holds the cards and the reading notes, because one schedule does:
// the page's days are both kinds added up, and it says so.
func TestForecastMergesReadingAndCards(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a := seedStatsLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			openStatsPage(t, a, tt, msgs.StatsForecastTitle())

			now := time.Now()
			cards := corestats.Forecast(a.lib, now, screenstats.ForecastDays)
			notes := corestats.ReadingForecast(a.lib, now, screenstats.ForecastDays)
			if len(cards) == 0 || len(notes) == 0 {
				t.Fatalf("the fixture put %d card days and %d reading days ahead, want both", len(cards), len(notes))
			}
			merged := map[string]int{}
			for _, day := range cards {
				merged[day.Day] += day.Count
			}
			for _, day := range notes {
				merged[day.Day] += day.Count
			}
			for _, day := range a.stats.Forecast {
				if want := merged[day.Day.Format("2006-01-02")]; want != day.Count {
					t.Fatalf("%s is drawn with %d, the log has %d", day.Day.Format("2006-01-02"), day.Count, want)
				}
			}

			peak, ok := a.stats.Peak()
			if !ok {
				t.Fatalf("the forecast has no peak: %+v", a.stats.Forecast)
			}
			if missing := missingText(tt,
				msgs.StatsForecastSub(), msgs.StatsForecastMerged(),
				msgs.StatsToday(), msgs.StatsNumber(a.stats.Forecast[0].Count),
				msgs.StatsPeak(), msgs.StatsNumber(peak.Count), msgs.StatsPeakDay(peak.Day),
				msgs.StatsTomorrow(), msgs.StatsNumber(a.stats.Forecast[1].Count),
			); missing != "" {
				t.Fatalf("the forecast page is missing %q; texts %q", missing, tt.Texts())
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// A library with no reviews has no rate to divide by: each page says what
// would fill it instead of plotting an axis with no values.
func TestStatsEmptyLibrary(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			dir := t.TempDir()
			lib, err := store.Create(dir, store.Options{})
			if err != nil {
				t.Fatalf("create library: %v", err)
			}
			lib.Close()
			a := NewApp(Options{LibraryDir: dir})
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			openStats(t, a, tt)

			if missing := missingText(tt,
				msgs.StatsTitle(),
				msgs.StatsHeatEmpty(), msgs.StatsProductionEmpty(),
				msgs.StatsTimeShareEmpty(), msgs.StatsRetentionEmpty(),
				msgs.StatsForecastEmpty(screenstats.OverviewForecastDays),
			); missing != "" {
				t.Fatalf("the empty overview is missing %q; texts %q", missing, tt.Texts())
			}

			if err := tt.Click(msgs.StatsHeat()); err != nil {
				t.Fatalf("the calendar page: %v", err)
			}
			tt.Advance(320 * time.Millisecond)
			if missing := missingText(tt,
				msgs.StatsHeatEmpty(), msgs.StatsHeatSource(),
				msgs.StatsRangeSpanTotal(a.stats.HeatFrom, a.stats.HeatTo, 0),
			); missing != "" {
				t.Fatalf("the empty calendar is missing %q; texts %q", missing, tt.Texts())
			}

			if err := tt.Click(msgs.StatsForecastTitle()); err != nil {
				t.Fatalf("the forecast page: %v", err)
			}
			tt.Advance(320 * time.Millisecond)
			if missing := missingText(tt,
				msgs.StatsForecastEmpty(screenstats.ForecastDays), msgs.StatsForecastMerged(),
			); missing != "" {
				t.Fatalf("the empty forecast is missing %q; texts %q", missing, tt.Texts())
			}
			if tt.FramePending() {
				t.Error("an empty page still requests frames after it should settle")
			}
		})
	}
}

// The three pages say their own words in Chinese at the smallest window, with
// the numbers each page is for inside it.
func TestStatsPagesZhHansAtMinimum(t *testing.T) {
	msgs := locales.In("zh-Hans")
	a := seedStatsLibrary(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	settle(tt)
	openStatsArea(tt)

	for _, label := range []string{
		msgs.StatsTitle(), msgs.StatsOverviewSub(), msgs.StatsProduction(),
		msgs.StatsCharacters(), msgs.StatsNumber(a.stats.Production.Characters),
	} {
		assertControl(t, tt, 720, 520, label)
	}

	if err := tt.Click(msgs.StatsHeat()); err != nil {
		t.Fatalf("the statistics area has no calendar page: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	for _, label := range []string{
		msgs.StatsHeat(), msgs.StatsHeatSub(), msgs.StatsRange(),
		msgs.StatsWeeks(len(a.stats.Heat)), msgs.StatsLongestStreak(), msgs.StatsDays(a.stats.Streak),
	} {
		assertControl(t, tt, 720, 520, label)
	}
	if !tt.HasText(msgs.StatsHeatSource()) {
		t.Errorf("the calendar page lost its source note; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.StatsForecastTitle()); err != nil {
		t.Fatalf("the statistics area has no forecast page: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	peak, _ := a.stats.Peak()
	for _, label := range []string{
		msgs.StatsForecastTitle(), msgs.StatsForecastSub(), msgs.StatsToday(),
		msgs.StatsPeak(), msgs.StatsPeakDay(peak.Day), msgs.StatsTomorrow(),
	} {
		assertControl(t, tt, 720, 520, label)
	}
	if !tt.HasText(msgs.StatsForecastMerged()) {
		t.Errorf("the forecast page lost why the two kinds merge; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("the page still requests frames after it should settle")
	}
}

// missingText reports the first of wants the window does not say.
func missingText(tt *ui.Tester, wants ...string) string {
	texts := strings.Join(tt.Texts(), "\n")
	for _, want := range wants {
		if !strings.Contains(texts, want) {
			return want
		}
	}
	return ""
}

// textSays reports whether the window says want as its own text.
func textSays(tt *ui.Tester, want string) bool {
	for _, text := range tt.Texts() {
		if text == want {
			return true
		}
	}
	return false
}

// Chinese labels do not clip at the smallest window: browse's controls and the
// statistics headings have a box of their own inside it.
func TestZhHansAtMinimum(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	// The frame speaks the window's language too: the rail, the path and the
	// controls around the stage.
	for _, label := range []string{"区域", "今日", "写", "读", "忆", "统计", "设置", "路径", "搜索操作", "更多"} {
		assertControl(t, tt, 720, 520, label)
	}
	recallQueue(tt)

	for _, label := range []string{"显示答案", "撤销 (U)", "暂停 (S)", "编辑笔记 (E)"} {
		assertControl(t, tt, 720, 520, label)
	}
	tt.Advance(320 * time.Millisecond) // the card's entry

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	for _, label := range []string{"笔记", "搜索", "笔记本", "笔记标题"} {
		assertControl(t, tt, 720, 520, label)
	}

	cardsPage(t, tt, "zh-Hans")
	for _, label := range []string{"浏览卡片", "搜索卡片", "筛选卡片", "全部", "到期", "新卡", "已暂停", "已失联", "打开 (Enter)"} {
		assertControl(t, tt, 720, 520, label)
	}
	assertControl(t, tt, 720, 520, "Spacing beats cramming")

	msgs := locales.In("zh-Hans")
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Advance(320 * time.Millisecond) // the card's exit
	tt.Type("统计")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(320 * time.Millisecond) // the screen's entrance

	// The area opens in Chinese: its heading and the production it counts have
	// a box of their own inside the window.
	for _, label := range []string{
		msgs.StatsTitle(), msgs.StatsOverviewSub(), msgs.StatsProduction(),
		msgs.StatsCharacters(), msgs.StatsCards(),
		msgs.OverviewPage(), msgs.StatsHeat(), msgs.StatsForecastTitle(),
	} {
		assertControl(t, tt, 720, 520, label)
	}
	if tt.HasText("Search actions") {
		t.Error("the palette stayed open after running its action")
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
