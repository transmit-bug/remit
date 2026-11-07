package main

import (
	"errors"
	"log/slog"
	"time"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/settings"
	appui "github.com/transmit-bug/remit/internal/ui"
	"github.com/transmit-bug/remit/locales"
)

// reminderID is the notification's identity: showing today's reminder again
// replaces the one Notification Center still holds instead of stacking a
// second.
const reminderID = "remit-daily"

// reminder sends at most one reminder per calendar day, and only while the user
// wants them. Every input is a function, so a test drives the clock, the
// notifier and the choice without an operating-system notification.
type reminder struct {
	now     func() time.Time
	enabled func() bool
	last    func() string
	mark    func(day string) error
	due     func() (appui.Counts, error)
	notify  func(title, body string) error
}

// remind shows today's reminder when reminders are on, something is due — a
// card to review or a note to read — and none was shown today, and reports
// whether it showed one.
func (r *reminder) remind() (bool, error) {
	day := r.now().Format(time.DateOnly)
	if !r.enabled() || r.last() == day {
		return false, nil
	}
	counts, err := r.due()
	if err != nil {
		return false, err
	}
	if counts.Cards+counts.Reading <= 0 {
		return false, nil
	}
	if err := r.notify(appui.Name, locales.ReminderBody(counts.Cards, counts.Reading)); err != nil {
		return false, err
	}
	// The day is recorded after the notification was shown, so a refused
	// permission is retried at the next start instead of counting as today's.
	if err := r.mark(day); err != nil {
		return true, err
	}
	return true, nil
}

// remindAtStartup sends today's reminder once, through the given notifier, and
// writes nothing when the user turned reminders off.
func remindAtStartup(a *appui.App, notify func(title, body string) error) {
	r := &reminder{
		now:     time.Now,
		enabled: func() bool { return a.Settings().Notifications },
		last:    func() string { return a.Settings().RemindedOn },
		mark: func(day string) error {
			return a.UpdateSettings(func(s *settings.Settings) { s.RemindedOn = day })
		},
		due: func() (appui.Counts, error) {
			lib := a.Library()
			if lib == nil {
				return appui.Counts{}, nil
			}
			now := time.Now()
			cards, err := lib.Remaining(now)
			if err != nil {
				return appui.Counts{}, err
			}
			reading, err := queue.ReadingCount(lib, now)
			if err != nil {
				return appui.Counts{}, err
			}
			return appui.Counts{Cards: cards, Reading: reading}, nil
		},
		notify: notify,
	}
	shown, err := r.remind()
	if err != nil {
		slog.Warn("remit: reminder", "err", err)
	}
	if shown {
		slog.Info("remit: reminder sent")
	}
}

// systemNotify shows a desktop notification through MyGo. macOS shows them for
// a packaged app only, and says so rather than failing quietly.
func systemNotify(title, body string) error {
	if !mygo.NotificationsSupported() {
		return errors.New("notifications need the packaged app")
	}
	return mygo.NewNotification(mygo.NotificationOptions{ID: reminderID, Title: title, Body: body}).Show()
}
