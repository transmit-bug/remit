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

// Event is one line of the review log: either a review, or a void naming the
// review it cancels. A void carries no card, rating or time of its own.
type Event struct {
	Seq    int64
	Card   core.ID
	Rating Rating
	At     time.Time
	Void   int64
}

// IsVoid reports whether the event cancels another review instead of recording
// one.
func (e Event) IsVoid() bool { return e.Void != 0 }

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
}

// Params configures the review schedule: the 21 FSRS weights and the recall
// target every interval is computed against.
type Params struct {
	Weights          [21]float64 `json:"weights"`
	DesiredRetention float64     `json:"desired_retention"`
}

// DefaultParams returns the FSRS defaults: the published weights and a 0.9
// recall target.
func DefaultParams() Params {
	return Params{Weights: defaultWeights(), DesiredRetention: DefaultRetention}
}

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

// Replay returns the state of every card that has at least one live review.
// Voided reviews leave no trace: a card whose reviews were all voided is absent
// from the result.
func Replay(events []Event, params Params) (map[core.ID]State, error) {
	a := NewApplier(params)
	if err := a.Apply(events...); err != nil {
		return nil, err
	}
	return a.States(), nil
}

// Applier replays a log incrementally. It keeps each card's live events, so a
// void costs a replay of that one card rather than of the whole log, and
// applying events in batches reaches the same state as replaying everything at
// once.
type Applier struct {
	fsrs   *fsrs.FSRS
	params Params

	states map[core.ID]State
	live   map[core.ID][]Event // per card, in sequence order
	card   map[int64]core.ID   // sequence -> the card it reviewed
	seen   map[int64]bool      // every sequence applied, reviews and voids
}

// NewApplier returns an applier with no history.
func NewApplier(params Params) *Applier {
	params = params.Normalized()
	return &Applier{
		fsrs:   fsrs.NewFSRS(params.parameters()),
		params: params,
		states: make(map[core.ID]State),
		live:   make(map[core.ID][]Event),
		card:   make(map[int64]core.ID),
		seen:   make(map[int64]bool),
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
		if _, err := ev.Rating.fsrs(); err != nil {
			return fmt.Errorf("schedule: event %d: %w", ev.Seq, err)
		}
		if ev.Card == "" {
			return fmt.Errorf("schedule: event %d: no card", ev.Seq)
		}

		a.card[ev.Seq] = ev.Card
		a.live[ev.Card] = append(a.live[ev.Card], ev)
		a.states[ev.Card] = a.step(a.states[ev.Card], ev)
	}
	return nil
}

// void cancels a review and replays the one card it belonged to, so the card
// reads exactly as if the review had never been written.
func (a *Applier) void(void Event) {
	cardID, ok := a.card[void.Void]
	if !ok {
		return
	}
	delete(a.card, void.Void)

	kept := a.live[cardID][:0]
	for _, ev := range a.live[cardID] {
		if ev.Seq != void.Void {
			kept = append(kept, ev)
		}
	}

	if len(kept) == 0 {
		delete(a.live, cardID)
		delete(a.states, cardID)
		return
	}

	a.live[cardID] = kept
	state := State{}
	for _, ev := range kept {
		state = a.step(state, ev)
	}
	a.states[cardID] = state
}

// step applies one review to a card's state.
func (a *Applier) step(state State, ev Event) State {
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
	return state
}

// State returns one card's state, and whether it has any live review.
func (a *Applier) State(card core.ID) (State, bool) {
	state, ok := a.states[card]
	return state, ok
}

// Last returns the newest live review of a card: the one an undo cancels.
func (a *Applier) Last(card core.ID) (Event, bool) {
	events := a.live[card]
	if len(events) == 0 {
		return Event{}, false
	}
	return events[len(events)-1], true
}

// States returns every card's state. The result is a copy: changing it does not
// change the applier.
func (a *Applier) States() map[core.ID]State {
	states := make(map[core.ID]State, len(a.states))
	for id, state := range a.states {
		states[id] = state
	}
	return states
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
