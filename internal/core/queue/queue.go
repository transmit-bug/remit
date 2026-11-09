// Package queue turns a library's cards and reading entries into today's
// queues: what is due, what is new, and how many remain, in an order that does
// not change between two identical calls.
package queue

import (
	"fmt"
	"sort"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Scope narrows a review session to part of the library. The zero scope is the
// whole library; a named note keeps the cards whose block lives in it, a named
// tag keeps the cards whose source block carries the tag. A scope never widens
// what a queue holds: a card the library already left out — suspended, reading,
// lost — stays out.
type Scope struct {
	Kind ScopeKind
	Note core.ID // KindScopeNote only
	Tag  string  // KindScopeTag only
}

// ScopeKind says which part of the library a Scope names.
type ScopeKind int

const (
	// ScopeAll is the whole library: every card a queue can hold.
	ScopeAll ScopeKind = iota
	// ScopeNote is one note's cards.
	ScopeNote
	// ScopeTag is every card whose source block carries the tag.
	ScopeTag
)

// Options limits a queue. Zero means no limit and no narrowing.
type Options struct {
	Limit     int
	NewPerDay int
	Scope     Scope
	// ReadingNewPerDay caps how many notes no session has read yet join today's
	// reading queue, when that many notes were already read today: it paces new
	// material, where NewPerDay paces new cards. A note being read again spends
	// none of it. Zero means no cap.
	ReadingNewPerDay int
}

// Limits answers the options a library's presets set: the daily allowance of
// new cards and of new reading. The limits travel with the library, so a queue
// takes them from it rather than from the caller's own settings; zero still
// means no cap.
func Limits(presets store.Presets) Options {
	return Options{
		NewPerDay:        presets.Review.NewPerDay,
		ReadingNewPerDay: presets.Reading.NewPerDay,
	}
}

// Counts is what the tray and the first screen show.
type Counts struct {
	Due       int
	Learning  int
	New       int
	Remaining int
}

// Cards returns the cards to review now, ordered learning, then due, then new,
// each by due time and then by ID so the order is stable. A scope in opts
// narrows the answer to that part of the library before the new-card allowance
// and the limit act.
//
// It is the review queue on its own. Today's mixed queue — reading, the excerpts
// owed, and these cards as one line — is Entries, which is what the first screen
// asks for.
func Cards(lib *store.Library, now time.Time, opts Options) ([]core.Card, error) {
	scheduled, err := lib.Scheduled()
	if err != nil {
		return nil, err
	}
	learning, due, fresh := partition(narrow(lib, scheduled, opts.Scope), now)
	fresh = introduce(lib, fresh, now, opts.NewPerDay)

	out := append(append(learning, due...), fresh...)
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

// CountsOf reports what Cards would return, without building it, under the
// same scope. The new-card allowance is not applied: a count is how much is
// waiting, not how much one sitting would take.
func CountsOf(lib *store.Library, now time.Time, opts Options) (Counts, error) {
	scheduled, err := lib.Scheduled()
	if err != nil {
		return Counts{}, err
	}
	learning, due, fresh := partition(narrow(lib, scheduled, opts.Scope), now)
	return Counts{
		Due:       len(due),
		Learning:  len(learning),
		New:       len(fresh),
		Remaining: len(learning) + len(due) + len(fresh),
	}, nil
}

// narrow keeps the scheduled cards a scope names, in the order they came. The
// note a card belongs to and the tags its block carries are read from the
// library's notes listing — one index read, no card file opened and no store
// query of its own. The whole-library scope changes nothing, which is what
// keeps today's answer what it has always been.
func narrow(lib *store.Library, scheduled []store.Scheduled, scope Scope) []store.Scheduled {
	switch scope.Kind {
	case ScopeNote, ScopeTag:
	default:
		return scheduled
	}
	in := map[core.ID]bool{}
	for _, note := range lib.Notes() {
		if scope.Kind == ScopeNote && note.ID != scope.Note {
			continue
		}
		for _, block := range note.Blocks {
			if scope.Kind == ScopeTag {
				for _, tag := range core.Tags(block.Text) {
					if tag == scope.Tag {
						in[block.ID] = true
						break
					}
				}
				continue
			}
			in[block.ID] = true
		}
		if scope.Kind == ScopeNote && len(in) > 0 {
			break
		}
	}
	out := make([]store.Scheduled, 0, len(scheduled))
	for _, entry := range scheduled {
		if in[entry.Card.Ref.Block] {
			out = append(out, entry)
		}
	}
	return out
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

// Reading returns the notes to read now, soonest due first and then by ID so the
// order is stable. A note that joined the queue and was never read is due at
// once; one that was read comes back on the schedule FSRS gave it, which is what
// makes reading incremental rather than a second to-do list.
//
// The day's introduction allowance caps the notes no session has read yet: the
// allowance limits new material, never the cadence a read note comes back on.
func Reading(lib *store.Library, now time.Time, opts Options) ([]store.Reading, error) {
	queued, err := lib.ReadingQueue()
	if err != nil {
		return nil, err
	}
	out := make([]store.Reading, 0, len(queued))
	for _, entry := range queued {
		if readingDue(entry, now) {
			out = append(out, entry)
		}
	}
	out = introduceReading(lib, out, now, opts.ReadingNewPerDay)
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

// introduceReading keeps today's reading queue within the day's allowance: a
// note no session has read yet is new material, and a day holds so much of it.
// A note being read again is not new — its cadence belongs to the schedule — so
// only never-read notes are held back, in the order the queue put them.
func introduceReading(lib *store.Library, due []store.Reading, now time.Time, perDay int) []store.Reading {
	if perDay <= 0 {
		return due
	}
	fresh := 0
	for _, entry := range due {
		if entry.State.Reviews == 0 {
			fresh++
		}
	}
	if fresh == 0 {
		return due
	}
	room := perDay - notesReadToday(lib, now)
	if room >= fresh {
		return due
	}
	out := make([]store.Reading, 0, len(due))
	for _, entry := range due {
		if entry.State.Reviews == 0 {
			if room <= 0 {
				continue
			}
			room--
		}
		out = append(out, entry)
	}
	return out
}

// notesReadToday is how many notes had a reading session today, which is the
// day's reading allowance already spent. A note read and then dropped from the
// queue the same day is not in the store's listing of the queue and so is not
// counted: the allowance paces what comes in, it is not an audit of the log.
func notesReadToday(lib *store.Library, now time.Time) int {
	queued, err := lib.ReadingQueue()
	if err != nil {
		return 0
	}
	day := startOfDay(now)
	end := day.AddDate(0, 0, 1)
	read := 0
	for _, entry := range queued {
		at := entry.State.LastReview
		if entry.State.Reviews > 0 && !at.Before(day) && at.Before(end) {
			read++
		}
	}
	return read
}

// ReadingCount is how many notes wait to be read now: the number the tray and
// the review screen report beside the cards' count, so reading is not drowned
// out by reviewing.
func ReadingCount(lib *store.Library, now time.Time) (int, error) {
	queued, err := lib.ReadingQueue()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range queued {
		if readingDue(entry, now) {
			count++
		}
	}
	return count, nil
}

// readingDue reports whether a reading entry comes up now. A note never read is
// due from the moment it joined the queue; one that was read waits for its
// schedule.
func readingDue(entry store.Reading, now time.Time) bool {
	due := entry.Due()
	return due.IsZero() || !due.After(now)
}

// String names a time for a log line.
func (c Counts) String() string {
	return fmt.Sprintf("%d due, %d learning, %d new", c.Due, c.Learning, c.New)
}
