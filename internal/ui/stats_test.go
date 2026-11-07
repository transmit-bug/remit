package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/locales"
)

// seedStatsLibrary writes a library with reviews to replay: one card reviewed
// twice, so the retention plot has a prediction to compare with, and both
// cards scheduled ahead, so the forecast has something to show.
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
	return NewApp(Options{LibraryDir: dir})
}

// openStats enters the statistics page the way the user does: from the command
// palette.
func openStats(t *testing.T, tt *ui.Tester) {
	t.Helper()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	tt.Type("stat")
	tt.Key(0, ui.KeyEnter)
	if tt.HasText("Search actions") {
		t.Fatalf("the palette stayed open; texts %q", tt.Texts())
	}
}

// The page replays the log into its three datasets: the review heatmap,
// retention per predicted bucket, and the forecast.
func TestStatsPage(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a := seedStatsLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			tt.Advance(320 * time.Millisecond) // the opening card's entry
			openStats(t, tt)

			for _, want := range []string{
				"Statistics",
				"Review heatmap",
				"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun",
				"Less", "More",
				"Retention by predicted chance",
				"100% remembered over 1 review",
				"Next 30 days",
				"2 cards",
			} {
				if !tt.HasText(want) {
					t.Fatalf("the statistics page is missing %q; texts %q", want, tt.Texts())
				}
			}
			if !containsText(tt.Texts(), "3 reviews") {
				t.Errorf("the heatmap does not say how many reviews it holds; texts %q", tt.Texts())
			}
			// Every day the log holds a review on has its own square, named
			// with that day and its count.
			labelled, counted := 0, 0
			for _, week := range a.stats.Heat {
				for _, day := range week {
					if day.Count == 0 {
						continue
					}
					labelled++
					counted += day.Count
					if want := locales.In("en").StatsDayCount(day.Day, day.Count); !tt.HasText(want) {
						t.Errorf("the heatmap has no square for %q; texts %q", want, tt.Texts())
					}
				}
			}
			if counted != a.stats.HeatTotal || labelled == 0 {
				t.Errorf("%d labelled squares hold %d reviews, the log %d", labelled, counted, a.stats.HeatTotal)
			}
			for _, gone := range []string{
				"No reviews yet. Grade a card and the map fills in.",
				"No reviews to compare yet.",
				"Nothing scheduled in the next 30 days.",
			} {
				if tt.HasText(gone) {
					t.Errorf("the page shows the empty state %q beside its data", gone)
				}
			}
			tt.Advance(320 * time.Millisecond) // the screen's entrance
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// A library with no reviews has no rate to divide by: the page says so instead
// of plotting an axis with no values.
func TestStatsEmptyLibrary(t *testing.T) {
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
			tt.Advance(320 * time.Millisecond) // the empty queue settles
			openStats(t, tt)

			for _, want := range []string{
				"Statistics",
				"Review heatmap",
				"No reviews yet. Grade a card and the map fills in.",
				"Retention by predicted chance",
				"No reviews to compare yet.",
				"Next 30 days",
				"Nothing scheduled in the next 30 days.",
			} {
				if !tt.HasText(want) {
					t.Fatalf("the empty statistics page is missing %q; texts %q", want, tt.Texts())
				}
			}
			tt.Advance(320 * time.Millisecond) // the screen's entrance
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// containsText reports whether any of texts holds want.
func containsText(texts []string, want string) bool {
	for _, text := range texts {
		if strings.Contains(text, want) {
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

	for _, label := range []string{"显示答案", "撤销 (U)", "暂停 (S)", "编辑笔记 (E)"} {
		assertControl(t, tt, 720, 520, label)
	}
	tt.Advance(320 * time.Millisecond) // the card's entry

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	for _, label := range []string{"笔记", "浏览", "统计", "笔记本", "笔记标题"} {
		assertControl(t, tt, 720, 520, label)
	}

	tt.Key(ui.Cmd, ui.Key3)
	for _, label := range []string{"浏览卡片", "搜索卡片", "筛选卡片", "全部", "到期", "新卡", "已暂停", "已失联", "打开 (Enter)"} {
		assertControl(t, tt, 720, 520, label)
	}
	assertControl(t, tt, 720, 520, "Spacing beats cramming")

	tt.Key(ui.Cmd, ui.KeyK)
	tt.Advance(320 * time.Millisecond) // the card's exit
	tt.Type("统计")
	tt.Key(0, ui.KeyEnter)
	for _, label := range []string{"统计数据", "复习热力图", "按预测记忆率看正确率", "少", "多"} {
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
