package main

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/settings"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// reminderFixture is a reminder whose clock, notifier and choices a test owns:
// no operating-system notification is shown and no day is real.
type reminderFixture struct {
	r        *reminder
	now      time.Time
	day      string
	due      int
	on       bool
	sent     []string
	sendFail error
	markFail error
}

func newReminderFixture() *reminderFixture {
	f := &reminderFixture{now: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC), due: 3, on: true}
	f.r = &reminder{
		now:     func() time.Time { return f.now },
		enabled: func() bool { return f.on },
		last:    func() string { return f.day },
		mark: func(day string) error {
			if f.markFail != nil {
				return f.markFail
			}
			f.day = day
			return nil
		},
		due: func() (int, error) { return f.due, nil },
		notify: func(title, body string) error {
			if f.sendFail != nil {
				return f.sendFail
			}
			f.sent = append(f.sent, body)
			return nil
		},
	}
	return f
}

// A reminder goes out once per calendar day, at most: due cards and reminders
// on do not mean a second notification the same afternoon, and the next day
// gets its own.
func TestReminderOncePerDay(t *testing.T) {
	f := newReminderFixture()

	shown, err := f.r.remind()
	if err != nil || !shown {
		t.Fatalf("remind = %v, %v; want the day's reminder", shown, err)
	}
	if len(f.sent) != 1 {
		t.Fatalf("sent %d reminders, want 1", len(f.sent))
	}
	if got, want := f.sent[0], "3 cards are due today."; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if f.day != "2026-11-06" {
		t.Errorf("recorded the reminder on %q", f.day)
	}

	// The same day, hours later: nothing more.
	f.now = f.now.Add(7 * time.Hour)
	if shown, err := f.r.remind(); shown || err != nil {
		t.Fatalf("a second reminder on the same day: %v, %v", shown, err)
	}
	if len(f.sent) != 1 {
		t.Fatalf("sent %d reminders on one day, want 1", len(f.sent))
	}

	// Midnight passes: the new day gets one.
	f.now = f.now.Add(8 * time.Hour)
	if shown, _ := f.r.remind(); !shown {
		t.Fatal("the next day sent no reminder")
	}
	if len(f.sent) != 2 {
		t.Fatalf("sent %d reminders over two days, want 2", len(f.sent))
	}
}

// Off sends nothing at all, whether or not cards are due and whether or not a
// reminder already went out today.
func TestReminderOffSendsNothing(t *testing.T) {
	f := newReminderFixture()
	f.on = false
	if shown, err := f.r.remind(); shown || err != nil {
		t.Fatalf("remind with reminders off = %v, %v", shown, err)
	}
	if len(f.sent) != 0 {
		t.Fatalf("reminders off sent %d notifications", len(f.sent))
	}
	if f.day != "" {
		t.Errorf("reminders off recorded the day as %q", f.day)
	}

	// Pausing after a reminder went out stops the next day's too, until it is
	// turned back on.
	f.on = true
	f.r.remind()
	f.on = false
	f.now = f.now.Add(24 * time.Hour)
	if shown, _ := f.r.remind(); shown {
		t.Error("a paused reminder was sent the next day")
	}
	if len(f.sent) != 1 {
		t.Errorf("sent %d reminders while paused, want 1", len(f.sent))
	}
}

// Nothing due, or a notification that could not be shown, sends nothing and
// records nothing, so the next start tries again.
func TestReminderNeedsDueCardsAndADelivery(t *testing.T) {
	f := newReminderFixture()
	f.due = 0
	if shown, _ := f.r.remind(); shown {
		t.Error("a reminder was sent with nothing due")
	}
	if len(f.sent) != 0 {
		t.Fatalf("sent %d reminders with nothing due", len(f.sent))
	}

	f.due = 2
	f.sendFail = errors.New("notifications are not allowed")
	if shown, err := f.r.remind(); shown || err == nil {
		t.Fatalf("remind with a refused notification = %v, %v", shown, err)
	}
	if f.day != "" {
		t.Errorf("a refused notification recorded %q as reminded", f.day)
	}

	f.sendFail = nil
	if shown, _ := f.r.remind(); !shown {
		t.Error("the reminder did not retry after the refusal")
	}
}

// Through the app, the once-a-day rule is the settings file: a reminder records
// its day, a second start the same day says nothing, and a fresh start with the
// setting off says nothing either.
func TestReminderAtStartup(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "library")
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return time.Now() }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "Memory", Blocks: []core.Block{{Text: "spacing :: beats cramming"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	if _, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}}); err != nil {
		t.Fatalf("create card: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close library: %v", err)
	}

	data := t.TempDir()
	a := appui.NewApp(appui.Options{LibraryDir: dir, DataDir: data})
	if err := a.UpdateSettings(func(s *settings.Settings) { s.Notifications = true }); err != nil {
		t.Fatalf("enable reminders: %v", err)
	}

	var sent []string
	notify := func(title, body string) error {
		sent = append(sent, title+" — "+body)
		return nil
	}
	remindAtStartup(a, notify)
	if len(sent) != 1 {
		t.Fatalf("the first start sent %d reminders, want 1: %v", len(sent), sent)
	}

	// A second start the same day: the setting remembers the day.
	remindAtStartup(a, notify)
	if len(sent) != 1 {
		t.Fatalf("a second start sent %d reminders, want 1: %v", len(sent), sent)
	}
	saved, err := settings.Load(data)
	if err != nil {
		t.Fatalf("settings.Load: %v", err)
	}
	if saved.RemindedOn != time.Now().Format(time.DateOnly) {
		t.Errorf("settings recorded the reminder on %q", saved.RemindedOn)
	}

	// Off sends nothing, even on a day with no reminder yet.
	if err := a.UpdateSettings(func(s *settings.Settings) {
		s.Notifications = false
		s.RemindedOn = ""
	}); err != nil {
		t.Fatalf("turn reminders off: %v", err)
	}
	remindAtStartup(a, notify)
	if len(sent) != 1 {
		t.Fatalf("reminders off sent %d more, want none: %v", len(sent), sent)
	}
}
