// Package queue turns a library's cards into today's review queue: what is
// due, what is new, and how many remain, in an order that does not change
// between two identical calls.
package queue

import (
	"fmt"
	"sort"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Options limits a queue. Zero means no limit.
type Options struct {
	Limit     int
	NewPerDay int
}

// Counts is what the tray and the first screen show.
type Counts struct {
	Due       int
	Learning  int
	New       int
	Remaining int
}

// Today returns the cards to review now, ordered learning, then due, then new,
// each by due time and then by ID so the order is stable.
func Today(lib *store.Library, now time.Time, opts Options) ([]core.Card, error) {
	scheduled, err := lib.Scheduled()
	if err != nil {
		return nil, err
	}
	learning, due, fresh := partition(scheduled, now)
	fresh = introduce(lib, fresh, now, opts.NewPerDay)

	out := append(append(learning, due...), fresh...)
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

// CountsOf reports what Today would return, without building it.
func CountsOf(lib *store.Library, now time.Time) (Counts, error) {
	scheduled, err := lib.Scheduled()
	if err != nil {
		return Counts{}, err
	}
	learning, due, fresh := partition(scheduled, now)
	return Counts{
		Due:       len(due),
		Learning:  len(learning),
		New:       len(fresh),
		Remaining: len(learning) + len(due) + len(fresh),
	}, nil
}

// partition splits the queue into learning, due and new, each already ordered.
// A card that is suspended, reading, or lost from its source is in none: a
// queue is a promise that these cards can be reviewed now, and the store leaves
// those out of what it lists.
//
// The store hands the cards over in due order, so learning and due need no sort
// of their own: each keeps that order, and only the never-reviewed cards are
// put by ID.
func partition(scheduled []store.Scheduled, now time.Time) (learning, due, fresh []core.Card) {
	for _, card := range scheduled {
		switch {
		case card.State.Reviews == 0:
			fresh = append(fresh, card.Card)
		case card.State.Due.After(now):
			continue
		case card.State.LastRating == schedule.Again:
			learning = append(learning, card.Card)
		default:
			due = append(due, card.Card)
		}
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].ID < fresh[j].ID })
	return learning, due, fresh
}

// introduce keeps at most NewPerDay cards that have never been reviewed, when
// that many were already started today. A card counts as started on the day of
// its newest live review, whatever its status since.
func introduce(lib *store.Library, fresh []core.Card, now time.Time, perDay int) []core.Card {
	if perDay <= 0 || len(fresh) == 0 {
		return fresh
	}
	day := startOfDay(now)
	started, err := lib.CountReviewed(day, day.AddDate(0, 0, 1))
	if err != nil {
		return fresh
	}
	room := perDay - started
	if room <= 0 {
		return nil
	}
	if room >= len(fresh) {
		return fresh
	}
	return fresh[:room]
}

// startOfDay is midnight of the day now falls on, in now's own location.
func startOfDay(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

// String names a time for a log line.
func (c Counts) String() string {
	return fmt.Sprintf("%d due, %d learning, %d new", c.Due, c.Learning, c.New)
}
