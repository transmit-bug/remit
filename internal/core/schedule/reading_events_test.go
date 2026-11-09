package schedule_test

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

func TestKindRoundTrips(t *testing.T) {
	tests := []struct {
		name     string
		kind     schedule.Kind
		valid    bool
		terminal bool
	}{
		{"reviewed", schedule.Reviewed, true, false},
		{"queued", schedule.Queued, true, false},
		{"dropped", schedule.Dropped, true, false},
		{"archived", schedule.Archived, true, true},
		{"dismissed", schedule.Dismissed, true, true},
		{"postponed", schedule.Postponed, true, false},
		{"past postponed", schedule.Postponed + 1, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.Terminal(); got != tt.terminal {
				t.Errorf("%d.Terminal() = %v, want %v", uint8(tt.kind), got, tt.terminal)
			}
			if !tt.valid {
				// A kind this build does not know writes as the one it falls back
				// to, so a log line never carries a name ParseKind refuses.
				if got := tt.kind.String(); got != "reviewed" {
					t.Errorf("unknown kind String() = %q, want reviewed", got)
				}
				return
			}
			got, ok := schedule.ParseKind(tt.kind.String())
			if !ok || got != tt.kind {
				t.Errorf("ParseKind(%q) = (%v, %v), want %v", tt.kind.String(), got, ok, tt.kind)
			}
		})
	}
	// A log written before kinds carried one is a review, and a kind is written
	// in the lowercase spelling the parser accepts.
	if got, ok := schedule.ParseKind(""); !ok || got != schedule.Reviewed {
		t.Errorf(`ParseKind("") = (%v, %v), want the review kind`, got, ok)
	}
	if _, ok := schedule.ParseKind("Archived"); ok {
		t.Error("ParseKind accepted a name it does not write")
	}
}

// postponed is one log line for a postpone: the time to come back and the moment
// the reader asked.
func postponed(seq int64, note core.ID, until, at time.Time) schedule.Event {
	return schedule.Event{Seq: seq, Note: note, Kind: schedule.Postponed, Until: until, At: at}
}

// The four ways a note leaves or stays in the reading queue, and the rules that
// hold between them: the two terminal states replace each other, a queued event
// ends either, and a postpone moves only the due date.
func TestReplayReadingStates(t *testing.T) {
	note := core.NewID("note")
	place := core.NewID("block")
	queued := reading(1, note, schedule.Queued, t0)
	session := read(2, note, schedule.Good, place, t0.Add(time.Hour))
	at := t0.AddDate(0, 0, 2)
	until := t0.AddDate(0, 0, 5)
	terminal := func(seq int64, kind schedule.Kind) schedule.Event { return reading(seq, note, kind, at) }

	tests := []struct {
		name       string
		events     []schedule.Event
		queued     bool
		archived   bool
		dismissed  bool
		reviews    int
		position   core.ID
		due        time.Time // zero: the schedule's own, checked elsewhere
		untilAsked time.Time
	}{
		{
			name:   "queued and not read",
			events: []schedule.Event{queued},
			queued: true, due: t0,
		},
		{
			name:     "archived",
			events:   []schedule.Event{queued, session, terminal(3, schedule.Archived)},
			archived: true, reviews: 1, position: place,
		},
		{
			name:      "dismissed",
			events:    []schedule.Event{queued, session, terminal(3, schedule.Dismissed)},
			dismissed: true, reviews: 1, position: place,
		},
		{
			name:      "dismissed after being archived",
			events:    []schedule.Event{queued, terminal(2, schedule.Archived), terminal(3, schedule.Dismissed)},
			dismissed: true,
		},
		{
			name:     "archived after being dismissed",
			events:   []schedule.Event{queued, terminal(2, schedule.Dismissed), terminal(3, schedule.Archived)},
			archived: true,
		},
		{
			name:   "queued after being archived",
			events: []schedule.Event{queued, session, terminal(3, schedule.Archived), reading(4, note, schedule.Queued, until)},
			queued: true, reviews: 1, position: place,
		},
		{
			name:   "queued after being dismissed",
			events: []schedule.Event{queued, terminal(2, schedule.Dismissed), reading(3, note, schedule.Queued, at)},
			queued: true,
		},
		{
			name:   "postponed before it was read",
			events: []schedule.Event{queued, postponed(2, note, until, at)},
			queued: true, due: until, untilAsked: until,
		},
		{
			name:   "postponed after a session",
			events: []schedule.Event{queued, session, postponed(3, note, until, at)},
			queued: true, reviews: 1, position: place, due: until, untilAsked: until,
		},
		{
			name:   "queued after a postpone schedules again",
			events: []schedule.Event{queued, postponed(2, note, until, at), reading(3, note, schedule.Queued, at.AddDate(0, 0, 1))},
			queued: true, due: at.AddDate(0, 0, 1),
		},
		{
			name:   "a session after a postpone takes the cadence back",
			events: []schedule.Event{queued, postponed(2, note, until, at), read(3, note, schedule.Good, place, at.Add(time.Hour))},
			queued: true, reviews: 1, position: place,
		},
		{
			name:   "a session ends a terminal state",
			events: []schedule.Event{queued, terminal(2, schedule.Archived), read(3, note, schedule.Good, place, at.Add(time.Hour))},
			queued: true, reviews: 1, position: place,
		},
		{
			name:     "dropped after being archived",
			events:   []schedule.Event{queued, terminal(2, schedule.Archived), terminal(3, schedule.Dropped)},
			archived: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applier := schedule.NewApplier(schedule.Params{})
			if err := applier.Apply(tt.events...); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			got, ok := applier.ReadingOf(note)
			if !ok {
				t.Fatalf("the log knows no reading entry for %q", note)
			}
			if got.Queued != tt.queued || got.Archived != tt.archived || got.Dismissed != tt.dismissed {
				t.Errorf("queued/archived/dismissed = %v/%v/%v, want %v/%v/%v",
					got.Queued, got.Archived, got.Dismissed, tt.queued, tt.archived, tt.dismissed)
			}
			if got.State.Reviews != tt.reviews {
				t.Errorf("reviews = %d, want %d", got.State.Reviews, tt.reviews)
			}
			if got.Position != tt.position {
				t.Errorf("position = %q, want %q", got.Position, tt.position)
			}
			if !tt.due.IsZero() && !got.State.Due.Equal(tt.due) {
				t.Errorf("due = %v, want %v", got.State.Due, tt.due)
			}
			if !got.Until.Equal(tt.untilAsked) {
				t.Errorf("until = %v, want %v", got.Until, tt.untilAsked)
			}
		})
	}
}

// A terminal state and a postpone leave the schedule alone: the note keeps the
// due date and the rating history it had, and only its place in the queue changes.
func TestTerminalEventLeavesTheScheduleAlone(t *testing.T) {
	note := core.NewID("note")
	place := core.NewID("block")
	queued := reading(1, note, schedule.Queued, t0)
	session := read(2, note, schedule.Hard, place, t0.Add(time.Hour))
	at := t0.AddDate(0, 0, 2)

	before, err := schedule.Replay([]schedule.Event{queued, session}, schedule.Params{})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	for _, kind := range []schedule.Kind{schedule.Archived, schedule.Dismissed} {
		t.Run(kind.String(), func(t *testing.T) {
			got, err := schedule.Replay([]schedule.Event{queued, session, reading(3, note, kind, at)}, schedule.Params{})
			if err != nil {
				t.Fatalf("Replay: %v", err)
			}
			if got[note] != before[note] {
				t.Errorf("%s changed the schedule: %+v, want %+v", kind, got[note], before[note])
			}
		})
	}
}

// Again counts a lapse, the count survives a void of the review that caused it,
// and a card answered anything else never adds one.
func TestReplayCountsLapses(t *testing.T) {
	card := core.NewID("card")
	good := review(1, card, schedule.Good, t0)
	again := review(2, card, schedule.Again, t0.AddDate(0, 0, 2))
	easy := review(3, card, schedule.Easy, t0.AddDate(0, 0, 6))

	tests := []struct {
		name   string
		events []schedule.Event
		lapses int
	}{
		{"never reviewed", nil, 0},
		{"answered Good", []schedule.Event{good}, 0},
		{"answered Again", []schedule.Event{good, again}, 1},
		{"answered Again twice", []schedule.Event{good, again, easy, review(4, card, schedule.Again, t0.AddDate(0, 0, 9))}, 2},
		{"the lapse was undone", []schedule.Event{good, again, {Seq: 3, Void: 2}}, 0},
		{"Easy after Again keeps the lapse", []schedule.Event{good, again, review(3, card, schedule.Easy, t0.AddDate(0, 0, 6))}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := schedule.Replay(tt.events, schedule.Params{})
			if err != nil {
				t.Fatalf("Replay: %v", err)
			}
			if got := state[card].Lapses; got != tt.lapses {
				t.Errorf("lapses = %d, want %d", got, tt.lapses)
			}
		})
	}
}

// An event the log format would not write is refused by replay too: a postpone
// names a time after the moment it was asked, and nothing else names one.
func TestApplyRefusesBadPostpones(t *testing.T) {
	note := core.NewID("note")
	tests := []struct {
		name  string
		event schedule.Event
	}{
		{"postponed without an until", reading(1, note, schedule.Postponed, t0)},
		{"postponed into the past", postponed(1, note, t0.Add(-time.Hour), t0)},
		{"queued with an until", schedule.Event{Seq: 1, Note: note, Kind: schedule.Queued, Until: t0, At: t0}},
		{"archived with an until", schedule.Event{Seq: 1, Note: note, Kind: schedule.Archived, Until: t0, At: t0}},
		{"a review with an until", schedule.Event{Seq: 1, Card: core.NewID("card"), Rating: schedule.Good, Until: t0, At: t0}},
		{"a reading change with a card", schedule.Event{Seq: 1, Card: core.NewID("card"), Kind: schedule.Archived, At: t0}},
		{"a reading change with a rating", schedule.Event{Seq: 1, Note: note, Kind: schedule.Archived, Rating: schedule.Good, At: t0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := schedule.NewApplier(schedule.Params{}).Apply(tt.event); err == nil {
				t.Fatalf("Apply accepted %+v", tt.event)
			}
		})
	}

	// Postponing a note to the moment it was asked is not a postponement into
	// the past, and the event is kept.
	if err := schedule.NewApplier(schedule.Params{}).Apply(postponed(1, note, t0, t0)); err != nil {
		t.Errorf("Apply refused a postpone to the moment it was asked: %v", err)
	}
}

// A note is replayed whole after a void, so events written after the voided
// session still count: undoing a session does not undo the postpone that
// followed it.
func TestVoidKeepsALaterPostpone(t *testing.T) {
	note := core.NewID("note")
	place := core.NewID("block")
	until := t0.AddDate(0, 0, 9)
	applier := schedule.NewApplier(schedule.Params{})
	if err := applier.Apply(
		reading(1, note, schedule.Queued, t0),
		read(2, note, schedule.Good, place, t0.Add(time.Hour)),
		postponed(3, note, until, t0.AddDate(0, 0, 1)),
	); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := applier.Apply(schedule.Event{Seq: 4, Void: 2}); err != nil {
		t.Fatalf("Apply void: %v", err)
	}
	got, ok := applier.ReadingOf(note)
	if !ok {
		t.Fatal("the void took the whole entry with it")
	}
	if !got.Queued {
		t.Error("the void took the note out of the reading queue")
	}
	if got.State.Reviews != 0 || got.Position != "" {
		t.Errorf("the voided session left reviews %d and position %q", got.State.Reviews, got.Position)
	}
	if !got.State.Due.Equal(until) || !got.Until.Equal(until) {
		t.Errorf("due/until = %v/%v, want the postpone's %v", got.State.Due, got.Until, until)
	}
}
