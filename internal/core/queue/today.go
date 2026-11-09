package queue

import (
	"math"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// EntryKind says what a line of Today is: a note to read, a note read but not
// yet excerpted, or the day's cards as one line.
type EntryKind uint8

const (
	// EntryRead is a note the reading queue says is due now.
	EntryRead EntryKind = iota
	// EntryExcerpt is a note a session read and no excerpt was taken from.
	EntryExcerpt
	// EntryReview is every card due today, as one line.
	EntryReview
)

// MinutesPerReading and MinutesPerReview price a sitting: one reading session,
// one card. They are the queue's estimate, not a measurement — they exist so a
// day can be held against a budget before anything is opened.
const (
	MinutesPerReading = 6.0
	MinutesPerReview  = 0.6
)

// defaultExcerptSlots is how many read-but-not-excerpted notes Today offers when
// nothing asks otherwise: a sitting leaves a passage or two worth keeping, and
// offering more turns the day's queue into a list of debts.
const defaultExcerptSlots = 2

// Entry is one line of Today: what to open, how far into it the reader is, and
// what it costs. The fields a line has no use for stay zero.
type Entry struct {
	Kind     EntryKind
	Note     core.ID // EntryRead, EntryExcerpt
	Title    string
	Position core.ID // the block the last session stopped at
	Blocks   int     // blocks the note holds: the "of 14" of "6 of 14"
	Read     int     // the paragraph the reader reached, counted from one
	Cards    int     // EntryReview: how many cards are due
	New      int     // EntryReview: how many of them no review has touched
	Minutes  int
	Over     bool      // the line falls past the day's budget
	LastSeen time.Time // EntryExcerpt: when the note was last read
}

// Today is the day's queue and what it costs: the entries in the order they
// should be worked, and the minutes read and reviewing take against the budget.
type Today struct {
	Entries       []Entry
	ReadMinutes   int
	ReviewMinutes int
	TotalMinutes  int
	Budget        int
}

// TodayOptions is what a day's queue is built from.
type TodayOptions struct {
	Budget       time.Duration // 0 = nothing is past it
	ExcerptSlots int           // 0 = defaultExcerptSlots
	Reading      Options       // the reading queue's own limit, scope and pace
	Cards        Options       // the review line's scope
}

// Entries builds today's queue: the notes due to read, the notes read but not
// yet excerpted, and one line for the cards due, in that order, each priced
// against the day's budget.
//
// The order is frozen. Reading keeps the order Reading gives it — soonest due
// first, then by ID — so two calls agree. Excerpts follow, most recently read
// first, so the passage still fresh comes before the one half forgotten. The
// review line is last and appears only while cards remain: a day is reading
// before recall, and recall is what the rest of the day is left for.
//
// A note already listed as due to read is not listed again as owing an excerpt:
// one note is one sitting's business.
//
// The budget is the day's minutes. Reading and excerpt lines accumulate
// MinutesPerReading, and the first line the running total cannot pay for — and
// every line after it — is marked Over, because a line is past the budget for
// where it falls and not for how long it is; a line landing exactly on the
// budget is not over it. The review line is priced on its own and counts in
// TotalMinutes, but it is never Over: reviewing is what the day has left, not
// something the reading budget can postpone.
func Entries(lib *store.Library, now time.Time, opts TodayOptions) (Today, error) {
	today := Today{Budget: wholeMinutes(opts.Budget)}
	reading, err := Reading(lib, now, opts.Reading)
	if err != nil {
		return Today{}, err
	}
	listed := make(map[core.ID]bool, len(reading))
	for _, entry := range reading {
		today.Entries = append(today.Entries, readEntry(lib, entry))
		listed[entry.Note] = true
	}

	slots := opts.ExcerptSlots
	if slots <= 0 {
		slots = defaultExcerptSlots
	}
	// Ask for room to skip the notes already listed, so the slots are filled
	// with notes the queue does not hold rather than with duplicates.
	candidates, err := ExcerptCandidates(lib, now, slots+len(reading))
	if err != nil {
		return Today{}, err
	}
	excerpts := 0
	for _, candidate := range candidates {
		if excerpts == slots {
			break
		}
		if listed[candidate.Note] {
			continue
		}
		today.Entries = append(today.Entries, excerptEntry(lib, candidate))
		excerpts++
	}

	counts, err := CountsOf(lib, now, opts.Cards)
	if err != nil {
		return Today{}, err
	}
	if counts.Remaining > 0 {
		today.Entries = append(today.Entries, reviewEntry(counts))
	}

	running := 0
	for i := range today.Entries {
		entry := &today.Entries[i]
		switch entry.Kind {
		case EntryRead, EntryExcerpt:
			running += entry.Minutes
			today.ReadMinutes += entry.Minutes
			if opts.Budget > 0 && running > today.Budget {
				entry.Over = true
			}
		case EntryReview:
			today.ReviewMinutes += entry.Minutes
		}
	}
	today.TotalMinutes = today.ReadMinutes + today.ReviewMinutes
	return today, nil
}

// ExcerptCandidates returns the notes read but not yet excerpted, most recently
// read first, at most limit of them; a limit of zero or less is no limit.
//
// now is where the caller's clock stands, and the order is the history's: the
// answer is the same whatever it says.
func ExcerptCandidates(lib *store.Library, now time.Time, limit int) ([]store.Reading, error) {
	return lib.UnExcerptedNotes(limit)
}

// Estimate is how long a sitting of readings and cards takes, in the same
// numbers the budget is made of.
func Estimate(readings, cards int) time.Duration {
	minutes := float64(readings)*MinutesPerReading + float64(cards)*MinutesPerReview
	return time.Duration(minutes * float64(time.Minute))
}

// wholeMinutes is a duration as the minutes a screen prints. A card is six
// tenths of a minute, so nineteen of them read as eleven whole minutes rather
// than as a fraction nobody budgets in.
func wholeMinutes(d time.Duration) int {
	return int(math.Round(d.Minutes()))
}

// readEntry is a note due to read, with what the reader needs to decide whether
// to open it now.
func readEntry(lib *store.Library, reading store.Reading) Entry {
	entry := Entry{Kind: EntryRead, Note: reading.Note, Position: reading.Position,
		Minutes: wholeMinutes(Estimate(1, 0))}
	describe(lib, &entry)
	return entry
}

// excerptEntry is a note read but not yet excerpted: the block the reader
// stopped at is where the passage to keep is, and when it was read says how
// stale the debt is.
func excerptEntry(lib *store.Library, reading store.Reading) Entry {
	entry := Entry{Kind: EntryExcerpt, Note: reading.Note, Position: reading.Position,
		Minutes: wholeMinutes(Estimate(1, 0)), LastSeen: reading.State.LastReview}
	describe(lib, &entry)
	return entry
}

// reviewEntry is the day's cards as one line: how many, how many of them new,
// and what a sitting of them costs.
func reviewEntry(counts Counts) Entry {
	return Entry{Kind: EntryReview, Cards: counts.Remaining, New: counts.New,
		Minutes: wholeMinutes(Estimate(0, counts.Remaining))}
}

// describe fills in what a line says about its note: its title, how many blocks
// it holds, and the paragraph the reader reached. A reading entry outlives its
// note only when the note is gone from under the library, and then there is
// nothing to say about it.
func describe(lib *store.Library, entry *Entry) {
	note, ok := lib.Note(entry.Note)
	if !ok {
		return
	}
	entry.Title = note.Title
	entry.Blocks = len(note.Blocks)
	for i, block := range note.Blocks {
		if block.ID == entry.Position {
			entry.Read = i + 1
			break
		}
	}
}
