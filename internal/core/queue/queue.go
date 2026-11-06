// Package queue turns a library's cards into today's review queue: what is
// due, what is new, and how many remain, in an order that does not change
// between two identical calls.
package queue

import (
	"fmt"
	"sort"
	"time"

	"github.com/transmit-bug/remit/internal/core"
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
	learning, due, fresh, err := partition(lib, now)
	if err != nil {
		return nil, err
	}
	fresh = introduce(lib, fresh, now, opts.NewPerDay)

	out := append(append(learning, due...), fresh...)
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

// CountsOf reports what Today would return, without building it.
func CountsOf(lib *store.Library, now time.Time) (Counts, error) {
	learning, due, fresh, err := partition(lib, now)
	if err != nil {
		return Counts{}, err
	}
	return Counts{
		Due:       len(due),
		Learning:  len(learning),
		New:       len(fresh),
		Remaining: len(learning) + len(due) + len(fresh),
	}, nil
}

// partition splits the queue into learning, due and new, each already ordered.
// A card that is suspended, reading, or lost from its source is in none: a
// queue is a promise that these cards can be reviewed now.
func partition(lib *store.Library, now time.Time) (learning, due, fresh []core.Card, err error) {
	last := lastRatings(lib)
	for _, card := range lib.Cards() {
		if !card.Status.Scheduled() {
			continue
		}
		if _, link := card.Ref.Text(lib); link == core.LinkLost {
			continue
		}
		state, ok := lib.CardState(card.ID)
		switch {
		case !ok || state.Reviews == 0:
			fresh = append(fresh, card)
		case state.Due.After(now):
			continue
		case last[card.ID] == "again":
			learning = append(learning, card)
		default:
			due = append(due, card)
		}
	}
	sortCards(learning, lib)
	sortCards(due, lib)
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].ID < fresh[j].ID })
	return learning, due, fresh, nil
}

func sortCards(cards []core.Card, lib *store.Library) {
	sort.Slice(cards, func(i, j int) bool {
		si, _ := lib.CardState(cards[i].ID)
		sj, _ := lib.CardState(cards[j].ID)
		if !si.Due.Equal(sj.Due) {
			return si.Due.Before(sj.Due)
		}
		return cards[i].ID < cards[j].ID
	})
}

// lastRatings is each card's last live rating, by name.
func lastRatings(lib *store.Library) map[core.ID]string {
	out := map[core.ID]string{}
	for _, ev := range lib.Events() {
		if ev.IsVoid() {
			continue
		}
		out[ev.Card] = ev.Rating.String()
	}
	return out
}

// introduce keeps at most NewPerDay cards that have never been reviewed, when
// that many were already started today. A card counts as started on the day of
// its first live review.
func introduce(lib *store.Library, fresh []core.Card, now time.Time, perDay int) []core.Card {
	if perDay <= 0 || len(fresh) == 0 {
		return fresh
	}
	started := map[core.ID]bool{}
	day := now.Format("2006-01-02")
	for _, ev := range lib.Events() {
		if ev.IsVoid() || ev.At.Format("2006-01-02") != day {
			continue
		}
		started[ev.Card] = true
	}
	room := perDay - len(started)
	if room <= 0 {
		return nil
	}
	if room >= len(fresh) {
		return fresh
	}
	return fresh[:room]
}

// String names a time for a log line.
func (c Counts) String() string {
	return fmt.Sprintf("%d due, %d learning, %d new", c.Due, c.Learning, c.New)
}
