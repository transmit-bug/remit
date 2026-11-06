package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
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
	first, err := lib.CreateCard(note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	second, err := lib.CreateCard(note.Blocks[1].ID)
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
	tt.Type("stat")
	tt.Key(0, ui.KeyEnter)
	if tt.HasText("Search actions") {
		t.Fatalf("the palette stayed open; texts %q", tt.Texts())
	}
}

// The page replays the log into its three datasets: reviews per day, retention
// per predicted bucket, and the forecast.
func TestStatsPage(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a := seedStatsLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			tt.Frame()
			openStats(t, tt)

			for _, want := range []string{
				"Statistics",
				"Reviews per day",
				"Last 30 days · 3 reviews",
				"Retention by predicted chance",
				"100% remembered over 1 review",
				"Next 30 days",
				"2 cards",
			} {
				if !tt.HasText(want) {
					t.Fatalf("the statistics page is missing %q; texts %q", want, tt.Texts())
				}
			}
			for _, gone := range []string{
				"No reviews in the last 30 days.",
				"No reviews to compare yet.",
				"Nothing scheduled in the next 30 days.",
			} {
				if tt.HasText(gone) {
					t.Errorf("the page shows the empty state %q beside its data", gone)
				}
			}
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
			a := NewApp(Options{LibraryDir: t.TempDir()})
			tt := ui.NewTester(a.View, size.w, size.h)
			tt.Frame()
			openStats(t, tt)

			for _, want := range []string{
				"Statistics",
				"Reviews per day",
				"No reviews in the last 30 days.",
				"Retention by predicted chance",
				"No reviews to compare yet.",
				"Next 30 days",
				"Nothing scheduled in the next 30 days.",
			} {
				if !tt.HasText(want) {
					t.Fatalf("the empty statistics page is missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// Chinese labels do not clip at the smallest window: browse's controls and the
// statistics headings have a box of their own inside it.
func TestZhHansAtMinimum(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	for _, label := range []string{"笔记", "复习", "浏览", "统计", "新建笔记", "笔记本", "建卡"} {
		assertControl(t, tt, 720, 520, label)
	}

	tt.Key(ui.Cmd, ui.Key3)
	for _, label := range []string{"浏览卡片", "搜索卡片", "筛选卡片", "全部", "到期", "新卡", "已暂停", "已失联", "打开 (Enter)"} {
		assertControl(t, tt, 720, 520, label)
	}
	assertControl(t, tt, 720, 520, "Spacing beats cramming")

	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("统计")
	tt.Key(0, ui.KeyEnter)
	for _, label := range []string{"统计数据", "每日复习", "按预测记忆率看正确率"} {
		assertControl(t, tt, 720, 520, label)
	}
	if tt.HasText("Search actions") {
		t.Error("the palette stayed open after running its action")
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
