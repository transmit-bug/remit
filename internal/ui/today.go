package ui

import (
	"log/slog"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/screens/today"
)

// todayView draws the Today screen: the page itself lives in its own package,
// and this hands it the day the app loaded and the app's own actions (ADR 0004).
func (a *App) todayView(c *ui.Context) {
	today.View(c, &a.today, a.todayDeps(c))
}

// todayDeps is what the screen asks of the app: the day's queue as the last load
// left it, and one function per thing the user can do, each run here on the UI
// thread.
func (a *App) todayDeps(c *ui.Context) today.Deps {
	return today.Deps{
		Today:       func() queue.Today { return a.todayDay },
		StartReview: func() { a.setArea(areaRecall, subRecallQueue) },
		OpenReading: func(note core.ID) { a.openReading(c, note, "") },
		OpenReadingAt: func(note, block core.ID) {
			a.openReading(c, note, block)
		},
		OpenPalette: func() { a.openPalette(c) },
		OpenDaily:   func() { a.openDailyNote(c) },
		NewNote:     func() { a.newNote(c) },
		Daily:       a.dailyNote,
		Tomorrow:    func() int { return a.todayTomorrow },
		Widgets:     a.todayWidgetItems,
		WidgetText:  func(plugin, raw string) string { return a.pluginText(c, plugin, raw) },
		RunWidget:   func(ref contrib.CommandRef) { a.runContribution(c, ref) },
		Reload: func() {
			a.loadToday()
			c.Invalidate()
		},
	}
}

// loadToday builds the day's queue: the notes due to read, the notes read but
// not yet excerpted and the cards due, priced against the library's budget. It is
// a load path — the screen reads what it leaves, so no frame asks the store —
// and it runs on entering Today and after every write.
//
// The budget and the excerpt slots are the library's own presets: what a day
// holds follows the material, and the settings page writes the same numbers
// (ADR 0011). The reading limits pace new material as the library asks.
func (a *App) loadToday() {
	a.today.Loaded, a.today.Err = false, ""
	a.todayDay, a.todayTomorrow = queue.Today{}, 0
	if a.lib == nil {
		return
	}
	presets := a.lib.Presets()
	now := time.Now()
	day, err := queue.Entries(a.lib, now, queue.TodayOptions{
		Budget:       time.Duration(presets.Budget.Minutes) * time.Minute,
		ExcerptSlots: presets.Budget.ExcerptSlots,
		Reading:      queue.Limits(presets),
		Cards:        queue.Limits(presets),
	})
	if err != nil {
		a.today.Err = err.Error()
		slog.Error("remit: today's queue", "err", err)
		return
	}
	a.todayDay = day
	a.today.Day, a.today.Loaded = now, true
	a.today.Recalled = a.recalledToday(now)
	a.todayTomorrow = a.tomorrowCount(now)
}

// recalledToday is how many cards were reviewed today, which the store counts
// by the day a card's last review falls on.
func (a *App) recalledToday(now time.Time) int {
	start := dayStart(now)
	count, err := a.lib.CountReviewed(start, start.AddDate(0, 0, 1))
	if err != nil {
		slog.Error("remit: today's reviews", "err", err)
		return 0
	}
	return count
}

// tomorrowCount is what comes due on the day after today, from the same forecast
// the statistics page plots: cards and reading notes on one series.
func (a *App) tomorrowCount(now time.Time) int {
	total := 0
	for _, day := range stats.ForecastAll(a.lib, dayStart(now).AddDate(0, 0, 1), 1) {
		total += day.Count
	}
	return total
}

// dayStart is midnight of the day t falls on, where a day's counts start.
func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// dailyNote is today's note, when the library holds it: the note the day is
// written in, found by the title the date gives it.
func (a *App) dailyNote() (core.Note, bool) {
	title := dailyTitle(time.Now())
	for _, note := range a.data.notes {
		if note.Title == title {
			return note, true
		}
	}
	return core.Note{}, false
}

// openReading opens the reading session at a note, with the day's due notes
// behind it: the note the user chose comes first, so the sitting starts there,
// and the rest of the queue follows rather than being thrown away.
func (a *App) openReading(c *ui.Context, note, block core.ID) {
	if a.lib == nil {
		return
	}
	due, err := queue.Reading(a.lib, time.Now(), queue.Limits(a.lib.Presets()))
	if err != nil {
		a.report(c, "reading queue", err)
		return
	}
	ids := make([]core.ID, 0, len(due)+1)
	ids = append(ids, note)
	for _, entry := range due {
		if entry.Note != note {
			ids = append(ids, entry.Note)
		}
	}
	a.reading.Reset()
	a.reading.Sync(ids)
	if block != "" {
		a.reading.Choose(block)
	}
	a.setArea(areaRead, subReadQueue)
}
