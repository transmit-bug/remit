package stats

import (
	"math"
	"sort"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Kind is what a history event records: a session on a note, or an answer on a
// card. The timeline colours a node by kind, so the two activities stay apart
// although one live log holds them (ADR 0003).
type Kind uint8

const (
	// KindReading is a reading session on a note.
	KindReading Kind = iota
	// KindCard is a card answered.
	KindCard
)

// HistoryEvent is one review on the history timeline: when it happened, what it
// was on, the answer, and the interval it moved. A reading event names the note
// it read; a card event names the card it answered.
//
// IntervalFrom and IntervalTo are days. IntervalFrom is the interval the
// previous answer on the same entry scheduled, zero when this is the first;
// IntervalTo is the interval this answer scheduled, so the pair reads "from N
// days to M days".
type HistoryEvent struct {
	At           time.Time
	Kind         Kind
	NoteTitle    string
	CardID       core.ID
	Rating       schedule.Rating
	IntervalFrom int
	IntervalTo   int
}

// History is one range of the log as the history page shows it: the reviews in
// the order they happened, the four-grade split, and how many answers there were
// and how many of them were Again. A range with no review is the zero value.
type History struct {
	Events []HistoryEvent
	Grades [4]int
	Total  int
	Again  int
}

// HistoryOf answers the reviews in [from, to) — a zero bound is open — from the
// live log: a voided review and the review it cancels are both left out.
//
// The whole live log is replayed, not only the range, because an interval is a
// difference between neighbouring answers on one entry: a card answered before
// the range still sets the interval the first in-range answer moves from. Once a
// state is known, only that entry advances, so the cost follows the log, not the
// range.
//
// A change to the reading queue's membership is a record of intent, not of
// memory work, so it is not on the timeline — the same line isReview draws for
// the heatmap. An archived or dismissed note's sessions stay: what ended the
// reading is not a reason to forget that it happened.
func HistoryOf(lib *store.Library, from, to time.Time) History {
	events, err := lib.LiveReviews(time.Time{}, time.Time{})
	if err != nil {
		return History{} // a log that cannot be read has no history to show
	}

	applier := schedule.NewApplierFor(lib.ScheduleParams(), schedule.DefaultReadingParams())
	titles := map[core.ID]string{}
	var out History
	for _, ev := range events {
		if !isReview(ev) {
			continue
		}
		prev, had := applier.State(ev.Entry())
		if err := applier.Apply(ev); err != nil {
			continue // an event the schedule cannot read is not on the timeline
		}
		state, ok := applier.State(ev.Entry())
		if !ok {
			continue
		}
		if !inRange(ev.At, from, to) {
			continue
		}

		event := HistoryEvent{At: ev.At, Rating: ev.Rating, IntervalTo: intervalDays(ev.At, state.Due)}
		if had && prev.Reviews > 0 {
			event.IntervalFrom = intervalDays(prev.LastReview, prev.Due)
		}
		if ev.IsReading() {
			event.Kind = KindReading
			event.NoteTitle = titleOf(lib, titles, ev.Note)
		} else {
			event.Kind = KindCard
			event.CardID = ev.Card
		}
		out.Events = append(out.Events, event)

		out.Total++
		if ev.Rating.Valid() {
			out.Grades[ev.Rating-1]++
		}
		if ev.Rating == schedule.Again {
			out.Again++
		}
	}
	sort.SliceStable(out.Events, func(i, j int) bool { return out.Events[i].At.Before(out.Events[j].At) })
	return out
}

// titleOf is a note's title as the index holds it, remembered for the call: a
// day of reading names the same note more than once.
func titleOf(lib *store.Library, cache map[core.ID]string, note core.ID) string {
	if title, ok := cache[note]; ok {
		return title
	}
	title := ""
	if n, ok := lib.Note(note); ok {
		title = n.Title
	}
	cache[note] = title
	return title
}

// inRange reports whether t is in [from, to), a zero bound being open —
// LiveReviews's range, so a history and a heatmap of the same days agree.
func inRange(t, from, to time.Time) bool {
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && !t.Before(to) {
		return false
	}
	return true
}

// intervalDays is the interval between two moments in whole days, rounded the
// way the schedule writes it: a review's due date is a whole number of days
// after the review.
func intervalDays(from, to time.Time) int {
	days := to.Sub(from).Hours() / 24
	if days <= 0 {
		return 0
	}
	return int(math.Round(days))
}
