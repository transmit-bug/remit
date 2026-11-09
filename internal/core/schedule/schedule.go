// Package schedule turns a card's review history into its FSRS state.
//
// It is pure: no files, no clock, no goroutines. The library owns the log; this
// package only reads it. FSRS's next state depends on nothing but the previous
// memory state, the elapsed time and the rating, so replaying the log rebuilds
// a card's schedule exactly, with no per-card sidecar to keep in step (ADR
// 0003).
//
// The replay is memory-only on purpose. FSRS's learning steps would need a
// per-card step counter stored somewhere, and v1 keeps one authority for
// scheduling history, so intervals are whole days and a card answered Again
// comes back tomorrow rather than in ten minutes.
package schedule

import (
	"errors"
	"fmt"
	"math"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"

	"github.com/transmit-bug/remit/internal/core"
)

// DefaultRetention is the recall target used when a library sets none.
const DefaultRetention = 0.9

// Rating is the answer a reviewer gave a card.
type Rating uint8

const (
	// Again means the card was not recalled.
	Again Rating = iota + 1
	// Hard means it was recalled with difficulty.
	Hard
	// Good means it was recalled.
	Good
	// Easy means it was recalled effortlessly.
	Easy
)

// String names the rating, for logs and for the review log format.
func (r Rating) String() string {
	switch r {
	case Again:
		return "again"
	case Hard:
		return "hard"
	case Good:
		return "good"
	case Easy:
		return "easy"
	}
	return "unknown"
}

// ParseRating reads a rating back from the name String writes.
func ParseRating(s string) (Rating, bool) {
	switch s {
	case "again":
		return Again, true
	case "hard":
		return Hard, true
	case "good":
		return Good, true
	case "easy":
		return Easy, true
	}
	return 0, false
}

// Valid reports whether the rating is one a reviewer can give.
func (r Rating) Valid() bool { return r >= Again && r <= Easy }

func (r Rating) fsrs() (fsrs.Rating, error) {
	switch r {
	case Again:
		return fsrs.Again, nil
	case Hard:
		return fsrs.Hard, nil
	case Good:
		return fsrs.Good, nil
	case Easy:
		return fsrs.Easy, nil
	}
	return 0, fmt.Errorf("schedule: unknown rating %d", uint8(r))
}

// Kind is what a log event records: a review, or a change to the reading
// queue's membership.
type Kind uint8

const (
	// Reviewed is a review: a card answered, or a note read. The rating says
	// how it went, and a reading review names the block it stopped at.
	Reviewed Kind = iota
	// Queued puts a note in the reading queue, and takes it out of the archive.
	Queued
	// Dropped takes a note out of the reading queue. Its history stays.
	Dropped
	// Archived is a terminal state: the note is done reading, and leaves the
	// queue for good. Only a queued event brings it back.
	Archived
	// Dismissed is the other terminal state: the note is not worth reading, and
	// leaves the queue for good. Only a queued event brings it back.
	Dismissed
	// Postponed moves a note's due date without touching its membership or its
	// rating history: the reader means to come back to it later.
	Postponed
)

// String names the kind, for logs and for the review log format.
func (k Kind) String() string {
	switch k {
	case Queued:
		return "queued"
	case Dropped:
		return "dropped"
	case Archived:
		return "archived"
	case Dismissed:
		return "dismissed"
	case Postponed:
		return "postponed"
	}
	return "reviewed"
}

// ParseKind reads a kind back from the name String writes.
func ParseKind(s string) (Kind, bool) {
	switch s {
	case "reviewed", "":
		return Reviewed, true
	case "queued":
		return Queued, true
	case "dropped":
		return Dropped, true
	case "archived":
		return Archived, true
	case "dismissed":
		return Dismissed, true
	case "postponed":
		return Postponed, true
	}
	return Reviewed, false
}

// Terminal reports whether the kind ends a note's place in the reading queue.
// The two terminal states exclude each other, and a queued event clears either.
func (k Kind) Terminal() bool { return k == Archived || k == Dismissed }

// Event is one line of the review log: a review (of a card, or of a note
// read), a change to the reading queue, or a void naming the review it
// cancels. A void carries no entry, rating or time of its own.
//
// Cards and reading entries share the log on purpose (ADR 0003): one append-only
// history, one replay, one place to count what was remembered from.
type Event struct {
	Seq    int64
	Card   core.ID // the card reviewed, empty on a reading event
	Note   core.ID // the note a reading event is about, empty on a card review
	Kind   Kind
	Rating Rating
	// Position is where a reading session stopped: the block the reader was on.
	Position core.ID
	// Until is when a postponed note comes due. It is set on a postponed event
	// and nowhere else.
	Until time.Time
	At    time.Time
	Void  int64
	// Scheduler is what wrote this review: a family and its version, as the
	// plugin that scheduled it named itself. Empty means the review was written
	// before the log carried one.
	Scheduler string
}

// IsVoid reports whether the event cancels another review instead of recording
// one.
func (e Event) IsVoid() bool { return e.Void != 0 }

// Entry returns the ID this event schedules: the card it reviewed, or the note
// it read or queued.
func (e Event) Entry() core.ID {
	if e.Note != "" {
		return e.Note
	}
	return e.Card
}

// IsReading reports whether the event belongs to the reading queue rather than
// to a card's schedule.
func (e Event) IsReading() bool { return e.Note != "" }

// Memory is a card's FSRS memory state: how stable the memory is and how hard
// the card is. It is the whole of what one review hands to the next.
type Memory = fsrs.MemoryState

// State is a card's schedule after replaying its live reviews.
type State struct {
	Memory     Memory
	Due        time.Time
	LastReview time.Time
	Reviews    int
	// LastRating is the answer the newest live review gave, zero when the card
	// has none. Replay derives it with the rest of the state, so a caller that
	// holds one card's state knows whether it is still being learned without
	// reading the log.
	LastRating Rating
	// Lapses counts the live reviews answered Again: the hard card a preset
	// compares against, and a column of the card table.
	Lapses int
}

// Params configures the review schedule: the 21 FSRS weights and the recall
// target every interval is computed against.
type Params struct {
	Weights          [21]float64 `json:"weights"`
	DesiredRetention float64     `json:"desired_retention"`
	// Scheduler is the identity future reviews are stamped with: a family and
	// its version, as the scheduling plugin names them.
	Scheduler string `json:"scheduler,omitempty"`
}

// DefaultParams returns the FSRS defaults: the published weights and a 0.9
// recall target.
func DefaultParams() Params {
	return Params{Weights: defaultWeights(), DesiredRetention: DefaultRetention}
}

// DefaultReadingParams is what a reading entry is scheduled with: the same FSRS
// mathematics and the published defaults, independent of the library's card
// schedule. Reading cadence is the host's, so the scheduling plugin that shapes
// cards (ADR 0007) has no say over when a note comes back.
func DefaultReadingParams() Params { return DefaultParams() }

// IsZero reports whether nothing was configured, so the defaults apply.
func (p Params) IsZero() bool {
	return p.Weights == [21]float64{} && p.DesiredRetention == 0
}

// Normalized fills in a default for every field left at zero, so a library may
// store only what it changed.
func (p Params) Normalized() Params {
	if p.Weights == [21]float64{} {
		p.Weights = defaultWeights()
	}
	if !(p.DesiredRetention > 0 && p.DesiredRetention <= 1) {
		p.DesiredRetention = DefaultRetention
	}
	return p
}

func defaultWeights() [21]float64 {
	w := fsrs.DefaultWeights()
	return w
}

// parameters is the scheduler this package drives. Fuzz is off: replay must
// give the same answer twice.
func (p Params) parameters() fsrs.Parameters {
	p = p.Normalized()
	fp := fsrs.DefaultParam()
	fp.W = fsrs.Weights(p.Weights)
	fp.RequestRetention = p.DesiredRetention
	fp.EnableFuzz = false
	fp.EnableShortTerm = true
	return fp
}

// ErrDuplicateSeq reports a log that carries the same review twice.
var ErrDuplicateSeq = errors.New("schedule: duplicate event sequence")

// Replay returns the state of every entry that has at least one live review.
// Voided reviews leave no trace: an entry whose reviews were all voided is
// absent from the result.
func Replay(events []Event, params Params) (map[core.ID]State, error) {
	a := NewApplier(params)
	if err := a.Apply(events...); err != nil {
		return nil, err
	}
	return a.States(), nil
}

// Reading is a note's place in the reading queue, as replay derives it.
type Reading struct {
	// State is the note's schedule. Zero means it was queued and never read,
	// which makes it due at once.
	State State
	// Position is the block the newest live session stopped at.
	Position core.ID
	// Queued reports whether the note is still in the queue now.
	Queued bool
	// Archived and Dismissed are the two terminal states. They exclude each
	// other, and neither is set while the note is queued: a note leaves the
	// queue for good, and a queued event takes it back.
	Archived  bool
	Dismissed bool
	// Until is the time the newest live postpone asked for, as long as that
	// postpone still governs the due date. A session or a fresh queueing
	// schedules the note again and clears it.
	Until time.Time
}

// Applier replays a log incrementally. It keeps each entry's live events, so a
// void costs a replay of that one entry rather than of the whole log, and
// applying events in batches reaches the same state as replaying everything at
// once.
type Applier struct {
	fsrs    *fsrs.FSRS
	reading *fsrs.FSRS
	params  Params
	read    Params

	states map[core.ID]State
	live   map[core.ID][]Event // per entry, its live rating events, in sequence order
	entry  map[int64]core.ID   // sequence -> the entry it reviewed
	seen   map[int64]bool      // every sequence applied, reviews and voids
	notes  map[core.ID]*readingEntry
}

// readingEntry is a note's live events and the place in the queue they derive.
// A note carries few events, so a void replays them whole: which came first — a
// postpone or the session it followed — is what answers the due date.
type readingEntry struct {
	state     State
	position  core.ID
	queued    bool
	archived  bool
	dismissed bool
	until     time.Time
	events    []Event
}

// reading is the note's place in the queue as the entry derives it.
func (e *readingEntry) reading() Reading {
	return Reading{State: e.state, Position: e.position, Queued: e.queued,
		Archived: e.archived, Dismissed: e.dismissed, Until: e.until}
}

// NewApplier returns an applier with no history. Reading entries are scheduled
// with DefaultReadingParams; NewApplierFor names them.
func NewApplier(params Params) *Applier {
	return NewApplierFor(params, DefaultReadingParams())
}

// NewApplierFor returns an applier that schedules cards with params and reading
// entries with read. The two carry their own FSRS parameters: a library whose
// card weights a plugin changed keeps reading on the host's cadence.
func NewApplierFor(params, read Params) *Applier {
	params = params.Normalized()
	read = read.Normalized()
	return &Applier{
		fsrs:    fsrs.NewFSRS(params.parameters()),
		reading: fsrs.NewFSRS(read.parameters()),
		params:  params,
		read:    read,
		states:  make(map[core.ID]State),
		live:    make(map[core.ID][]Event),
		entry:   make(map[int64]core.ID),
		seen:    make(map[int64]bool),
		notes:   make(map[core.ID]*readingEntry),
	}
}

// Apply replays events in sequence order. A void naming a review this applier
// never saw is ignored: the log is append-only and hand-editable, and a library
// that cannot be opened is worse than a void with nothing to cancel.
func (a *Applier) Apply(events ...Event) error {
	ordered := make([]Event, len(events))
	copy(ordered, events)
	sortEvents(ordered)

	for _, ev := range ordered {
		if a.seen[ev.Seq] {
			return fmt.Errorf("%w: %d", ErrDuplicateSeq, ev.Seq)
		}
		a.seen[ev.Seq] = true

		if ev.IsVoid() {
			a.void(ev)
			continue
		}
		id := ev.Entry()
		if id == "" {
			return fmt.Errorf("schedule: event %d: no card and no note", ev.Seq)
		}
		if ev.Card != "" && ev.Note != "" {
			return fmt.Errorf("schedule: event %d: reviews a card and a note", ev.Seq)
		}
		if err := checkEvent(ev); err != nil {
			return err
		}

		switch ev.Kind {
		case Queued, Dropped, Archived, Dismissed, Postponed:
			a.change(ev)
			continue
		}

		if _, err := ev.Rating.fsrs(); err != nil {
			return fmt.Errorf("schedule: event %d: %w", ev.Seq, err)
		}
		a.entry[ev.Seq] = id
		a.live[id] = append(a.live[id], ev)
		if ev.IsReading() {
			// Reading a note puts it in the queue: it comes back on its own
			// cadence even if it was never added by hand.
			a.record(ev)
			continue
		}
		a.states[id] = a.step(a.states[id], ev)
	}
	return nil
}

// checkEvent refuses an event the log format would not write: a change to a
// note's place in the queue names the note and carries neither a card nor a
// rating, and only a postpone names an until.
func checkEvent(ev Event) error {
	if ev.Kind == Reviewed {
		if !ev.Until.IsZero() {
			return fmt.Errorf("schedule: event %d: a review carries an until", ev.Seq)
		}
		return nil
	}
	if ev.Card != "" || ev.Rating != 0 {
		return fmt.Errorf("schedule: event %d: %s carries a card or a rating", ev.Seq, ev.Kind)
	}
	if ev.Kind != Postponed {
		if !ev.Until.IsZero() {
			return fmt.Errorf("schedule: event %d: %s carries an until", ev.Seq, ev.Kind)
		}
		return nil
	}
	if ev.Until.IsZero() {
		return fmt.Errorf("schedule: event %d: postponed without an until", ev.Seq)
	}
	if ev.Until.Before(ev.At) {
		return fmt.Errorf("schedule: event %d: postponed until %s, before it was postponed at %s",
			ev.Seq, ev.Until.UTC().Format(time.RFC3339), ev.At.UTC().Format(time.RFC3339))
	}
	return nil
}

// change records a change to a note's place in the queue and derives the note
// again from every live event it has.
func (a *Applier) change(ev Event) {
	entry := a.notes[ev.Note]
	if entry == nil {
		if ev.Kind != Queued {
			// A membership event for a note the log never put in the queue:
			// dropping, archiving or postponing nothing.
			return
		}
		entry = &readingEntry{}
		a.notes[ev.Note] = entry
	}
	a.record(ev)
}

// record appends one live reading event to its note and derives the note again.
func (a *Applier) record(ev Event) {
	entry := a.notes[ev.Note]
	if entry == nil {
		entry = &readingEntry{}
		a.notes[ev.Note] = entry
	}
	entry.events = append(entry.events, ev)
	a.derive(ev.Note)
}

// derive replays one note's live events in the order they were written. A
// session brings the note into the queue and sets the place it stopped at; the
// two terminal kinds take it out and replace each other; a postpone moves the
// due date and leaves membership and the rating history alone.
func (a *Applier) derive(note core.ID) {
	entry := a.notes[note]
	if entry == nil {
		return
	}
	if len(entry.events) == 0 {
		// Every event the note had is void: the live log does not know it.
		delete(a.notes, note)
		delete(a.states, note)
		return
	}
	next := readingEntry{}
	for _, ev := range entry.events {
		switch ev.Kind {
		case Queued:
			next.queued, next.archived, next.dismissed = true, false, false
			if next.state.Reviews == 0 {
				// A note never read is due when it joined the queue, which is
				// what fills the queue in the order notes were added to it.
				next.state.Due = ev.At.UTC()
				next.until = time.Time{}
			}
		case Dropped:
			next.queued = false
		case Archived:
			next.queued, next.archived, next.dismissed = false, true, false
		case Dismissed:
			next.queued, next.archived, next.dismissed = false, false, true
		case Postponed:
			next.state.Due = ev.Until.UTC()
			next.until = ev.Until.UTC()
		default:
			next.state = a.step(next.state, ev)
			if ev.Position != "" {
				next.position = ev.Position
			}
			next.queued, next.archived, next.dismissed = true, false, false
			next.until = time.Time{}
		}
	}
	entry.state, entry.position = next.state, next.position
	entry.queued, entry.archived, entry.dismissed = next.queued, next.archived, next.dismissed
	entry.until = next.until
	a.states[note] = next.state
}

// void cancels a review and replays the one entry it belonged to, so the entry
// reads exactly as if the review had never been written. Membership and the
// terminal states are not touched: undoing an answer takes back the answer, not
// the reading.
func (a *Applier) void(void Event) {
	id, ok := a.entry[void.Void]
	if !ok {
		return
	}
	delete(a.entry, void.Void)
	a.live[id] = withoutSeq(a.live[id], void.Void)

	if entry, ok := a.notes[id]; ok {
		// A note is replayed whole, so a postpone written after the voided
		// session still governs the due date.
		entry.events = withoutSeq(entry.events, void.Void)
		a.derive(id)
		return
	}

	kept := a.live[id]
	if len(kept) == 0 {
		delete(a.live, id)
		delete(a.states, id)
		return
	}

	state := State{}
	for _, ev := range kept {
		state = a.step(state, ev)
	}
	a.states[id] = state
}

// withoutSeq copies events without one sequence: what a void leaves behind.
func withoutSeq(events []Event, seq int64) []Event {
	kept := make([]Event, 0, len(events))
	for _, ev := range events {
		if ev.Seq != seq {
			kept = append(kept, ev)
		}
	}
	return kept
}

// step applies one review to an entry's state, with the parameters that entry
// is scheduled by: a card follows the library's schedule, a reading entry the
// host's own.
func (a *Applier) step(state State, ev Event) State {
	if ev.IsReading() {
		return a.read.step(a.reading.Parameters, state, ev)
	}
	return a.params.step(a.fsrs.Parameters, state, ev)
}

// Step applies one review to a card's schedule. It is the same step an applier
// performs, exposed so a caller holding one card's state — the index, which
// caches what replay derived — can advance it without the whole log.
func (p Params) Step(state State, ev Event) State {
	p = p.Normalized()
	return p.step(p.parameters(), state, ev)
}

func (p Params) step(fsrsParameters fsrs.Parameters, state State, ev Event) State {
	rating, err := ev.Rating.fsrs()
	if err != nil {
		return state // Apply rejects it before it gets here
	}

	var current *Memory
	if state.Reviews > 0 {
		memory := state.Memory
		current = &memory
	}

	item := fsrsParameters.NextState(
		current,
		p.DesiredRetention,
		elapsedDays(state.LastReview, ev.At),
		rating,
	)

	days := intervalDays(item.Interval, fsrsParameters.MaximumInterval)
	state.Memory = item.Memory
	state.LastReview = ev.At
	state.Due = ev.At.Add(time.Duration(days * 24 * float64(time.Hour)))
	state.Reviews++
	state.LastRating = ev.Rating
	if ev.Rating == Again {
		state.Lapses++
	}
	return state
}

// State returns one entry's state, and whether it has any live review.
func (a *Applier) State(entry core.ID) (State, bool) {
	state, ok := a.states[entry]
	return state, ok
}

// Last returns the newest live review of an entry: the one an undo cancels.
func (a *Applier) Last(entry core.ID) (Event, bool) {
	events := a.live[entry]
	if len(events) == 0 {
		return Event{}, false
	}
	return events[len(events)-1], true
}

// States returns every entry's state. The result is a copy: changing it does not
// change the applier.
func (a *Applier) States() map[core.ID]State {
	states := make(map[core.ID]State, len(a.states))
	for id, state := range a.states {
		states[id] = state
	}
	return states
}

// Reading returns every note the log knows, with where it stands in the reading
// queue. The result is a copy: changing it does not change the applier.
func (a *Applier) Reading() map[core.ID]Reading {
	out := make(map[core.ID]Reading, len(a.notes))
	for id, entry := range a.notes {
		out[id] = entry.reading()
	}
	return out
}

// ReadingOf returns one note's place in the reading queue, and whether the log
// has any reading event for it at all: a note the log only dropped was never in
// the queue, and is not queued now.
func (a *Applier) ReadingOf(note core.ID) (Reading, bool) {
	entry, ok := a.notes[note]
	if !ok {
		return Reading{}, false
	}
	return entry.reading(), true
}

// Outlook is what one answer would do to a card, for the interval a grade key
// shows: the due time it would leave behind, and how long that is from now.
type Outlook struct {
	Rating Rating
	Due    time.Time
	Wait   time.Duration
}

// Preview is what each of the four answers would schedule for a card in state,
// answered at now. It changes nothing: Step computes the same thing, and a test
// holds the two to each other.
//
// A card with no live review has no state, and Schedule.Preview takes a zero
// State as a card the applier has never seen — which is what a new card is.
//
// The result has one outlook per answer, in the order Again, Hard, Good, Easy.
func (p Params) Preview(state State, now time.Time) []Outlook {
	out := make([]Outlook, 0, 4)
	for r := Again; r <= Easy; r++ {
		next := p.Step(state, Event{Rating: r, At: now})
		out = append(out, Outlook{Rating: r, Due: next.Due, Wait: next.Due.Sub(now)})
	}
	return out
}

// elapsedDays is the calendar-day distance FSRS measures a review over. Both
// days are taken in UTC, which is how the reference implementation counts.
func elapsedDays(last, now time.Time) uint64 {
	if last.IsZero() {
		return 0
	}
	from := midnight(last)
	to := midnight(now)
	days := int64(to.Sub(from) / (24 * time.Hour))
	if days < 0 {
		return 0
	}
	return uint64(days)
}

func midnight(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// intervalDays is the whole-day interval FSRS schedules next: rounded, never
// under a day, never past the maximum.
func intervalDays(interval, max float64) float64 {
	return math.Max(math.Min(math.Round(interval), max), 1)
}

func sortEvents(events []Event) {
	// Insertion sort keeps the common case (already ordered) cheap.
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j].Seq < events[j-1].Seq; j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}
}

// PredictedRetention is the chance the schedule expected the card to be
// recalled after elapsedDays, given its stability at the previous review. It
// is what a retention chart compares a review's outcome against.
func (p Params) PredictedRetention(elapsedDays, stability float64) float64 {
	params := p.parameters()
	return params.ForgettingCurve(elapsedDays, stability)
}
