package schedule_test

import (
	"errors"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

var t0 = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

func TestRatingRoundTrips(t *testing.T) {
	tests := []struct {
		name   string
		rating schedule.Rating
		valid  bool
	}{
		{"again", schedule.Again, true},
		{"hard", schedule.Hard, true},
		{"good", schedule.Good, true},
		{"easy", schedule.Easy, true},
		{"zero", 0, false},
		{"past easy", schedule.Easy + 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rating.Valid(); got != tt.valid {
				t.Fatalf("%d.Valid() = %v, want %v", uint8(tt.rating), got, tt.valid)
			}
			if !tt.valid {
				if got := tt.rating.String(); got != "unknown" {
					t.Errorf("unknown rating String() = %q", got)
				}
				return
			}
			got, ok := schedule.ParseRating(tt.rating.String())
			if !ok || got != tt.rating {
				t.Errorf("ParseRating(%q) = (%v, %v), want %v",
					tt.rating.String(), got, ok, tt.rating)
			}
		})
	}
	if _, ok := schedule.ParseRating("Good"); ok {
		t.Error("ParseRating accepted a name it does not write")
	}
}

func TestParamsDefaults(t *testing.T) {
	zero := schedule.Params{}
	if !zero.IsZero() {
		t.Error("zero Params did not report IsZero")
	}
	got := zero.Normalized()
	want := schedule.DefaultParams()
	if got.Weights != want.Weights || got.DesiredRetention != want.DesiredRetention {
		t.Errorf("zero Params normalized to %+v, want the defaults", got)
	}
	if want.DesiredRetention != schedule.DefaultRetention {
		t.Errorf("DefaultParams retention = %v, want %v", want.DesiredRetention, schedule.DefaultRetention)
	}

	partial := schedule.Params{DesiredRetention: 0.8}.Normalized()
	if partial.Weights != want.Weights || partial.DesiredRetention != 0.8 {
		t.Errorf("partial Params normalized to %+v, want the default weights kept", partial)
	}

	kept := schedule.DefaultParams()
	kept.Weights[0] = 0.5
	if got := kept.Normalized(); got.Weights[0] != 0.5 {
		t.Errorf("Normalized replaced weights that were set: %v", got.Weights[0])
	}

	outOfRange := schedule.Params{DesiredRetention: 1.5}.Normalized()
	if outOfRange.DesiredRetention != schedule.DefaultRetention {
		t.Errorf("retention 1.5 normalized to %v, want the default", outOfRange.DesiredRetention)
	}
}

func review(seq int64, card core.ID, rating schedule.Rating, at time.Time) schedule.Event {
	return schedule.Event{Seq: seq, Card: card, Rating: rating, At: at}
}

func TestReplaySchedule(t *testing.T) {
	card := core.NewID("card")
	tests := []struct {
		name        string
		events      []schedule.Event
		wantReviews int
		wantDueAt   time.Time // zero: only check the due time moved past the review
	}{
		{
			name:        "first good",
			events:      []schedule.Event{review(1, card, schedule.Good, t0)},
			wantReviews: 1,
		},
		{
			name: "three reviews",
			events: []schedule.Event{
				review(1, card, schedule.Good, t0),
				review(2, card, schedule.Good, t0.AddDate(0, 0, 3)),
				review(3, card, schedule.Easy, t0.AddDate(0, 0, 12)),
			},
			wantReviews: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			states, err := schedule.Replay(tt.events, schedule.Params{})
			if err != nil {
				t.Fatalf("Replay: %v", err)
			}
			state, ok := states[card]
			if !ok {
				t.Fatalf("Replay returned no state for %q", card)
			}
			if state.Reviews != tt.wantReviews {
				t.Errorf("Reviews = %d, want %d", state.Reviews, tt.wantReviews)
			}
			last := tt.events[len(tt.events)-1]
			if !state.LastReview.Equal(last.At) {
				t.Errorf("LastReview = %v, want %v", state.LastReview, last.At)
			}
			if !state.Due.After(last.At) {
				t.Errorf("Due = %v, want later than the review at %v", state.Due, last.At)
			}
			if state.Memory.Stability <= 0 || state.Memory.Difficulty < 1 || state.Memory.Difficulty > 10 {
				t.Errorf("memory = %+v, want a stability above 0 and difficulty in [1,10]", state.Memory)
			}
			if again, err := schedule.Replay(tt.events, schedule.Params{}); err != nil {
				t.Fatalf("second Replay: %v", err)
			} else if again[card] != state {
				t.Errorf("Replay is not deterministic: %+v then %+v", state, again[card])
			}
		})
	}
}

func TestReplayRatingMovesTheDueDate(t *testing.T) {
	card := core.NewID("card")
	due := map[schedule.Rating]time.Time{}
	for _, rating := range []schedule.Rating{schedule.Again, schedule.Hard, schedule.Good, schedule.Easy} {
		states, err := schedule.Replay([]schedule.Event{review(1, card, rating, t0)}, schedule.Params{})
		if err != nil {
			t.Fatalf("Replay(%v): %v", rating, err)
		}
		due[rating] = states[card].Due
	}
	// Again and Hard both round to a single day on a first review.
	if due[schedule.Again].After(due[schedule.Hard]) ||
		due[schedule.Hard].After(due[schedule.Good]) ||
		!due[schedule.Good].Before(due[schedule.Easy]) {
		t.Errorf("due dates do not grow with the rating: %v", due)
	}
}

func TestVoidLeavesNoTrace(t *testing.T) {
	card := core.NewID("card")
	first := review(1, card, schedule.Good, t0)
	second := review(2, card, schedule.Again, t0.AddDate(0, 0, 4))
	undoSecond := schedule.Event{Seq: 3, Void: 2}
	undoFirst := schedule.Event{Seq: 4, Void: 1}

	before, err := schedule.Replay([]schedule.Event{first}, schedule.Params{})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	tests := []struct {
		name   string
		events []schedule.Event
		want   map[core.ID]schedule.State
	}{
		{
			name:   "void the last review",
			events: []schedule.Event{first, second, undoSecond},
			want:   before,
		},
		{
			name:   "void every review",
			events: []schedule.Event{first, second, undoSecond, undoFirst},
			want:   map[core.ID]schedule.State{},
		},
		{
			name:   "void an unknown review",
			events: []schedule.Event{first, {Seq: 2, Void: 99}},
			want:   before,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := schedule.Replay(tt.events, schedule.Params{})
			if err != nil {
				t.Fatalf("Replay: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Replay returned %d states, want %d", len(got), len(tt.want))
			}
			for id, want := range tt.want {
				if got[id] != want {
					t.Errorf("state of %q = %+v, want %+v", id, got[id], want)
				}
			}
		})
	}
}

func TestApplyIncrementallyMatchesReplay(t *testing.T) {
	a, b := core.NewID("card"), core.NewID("card")
	events := []schedule.Event{
		review(1, a, schedule.Good, t0),
		review(2, b, schedule.Again, t0),
		review(3, a, schedule.Hard, t0.AddDate(0, 0, 2)),
		review(4, b, schedule.Good, t0.AddDate(0, 0, 1)),
		{Seq: 5, Void: 2},
		review(6, a, schedule.Good, t0.AddDate(0, 0, 9)),
		{Seq: 7, Void: 4},
		review(8, b, schedule.Easy, t0.AddDate(0, 0, 2)),
	}

	whole, err := schedule.Replay(events, schedule.Params{})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	for _, split := range []int{1, 2, 3, 5, 6, 7, len(events) - 1} {
		applier := schedule.NewApplier(schedule.Params{})
		if err := applier.Apply(events[:split]...); err != nil {
			t.Fatalf("Apply(first %d): %v", split, err)
		}
		if err := applier.Apply(events[split:]...); err != nil {
			t.Fatalf("Apply(rest after %d): %v", split, err)
		}
		got := applier.States()
		if len(got) != len(whole) {
			t.Fatalf("split at %d: %d states, want %d", split, len(got), len(whole))
		}
		for id, want := range whole {
			if got[id] != want {
				t.Errorf("split at %d: state of %q = %+v, want %+v", split, id, got[id], want)
			}
		}
	}
}

func TestApplyRejectsDuplicatesAndBadRatings(t *testing.T) {
	card := core.NewID("card")
	applier := schedule.NewApplier(schedule.Params{})
	if err := applier.Apply(review(1, card, schedule.Good, t0)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := applier.Apply(review(1, card, schedule.Good, t0)); !errors.Is(err, schedule.ErrDuplicateSeq) {
		t.Errorf("duplicate Apply error = %v, want ErrDuplicateSeq", err)
	}
	if err := schedule.NewApplier(schedule.Params{}).Apply(review(1, card, 0, t0)); err == nil {
		t.Error("Apply accepted rating 0")
	}
	if err := schedule.NewApplier(schedule.Params{}).Apply(review(1, "", schedule.Good, t0)); err == nil {
		t.Error("Apply accepted an event with no card")
	}
}

func TestLastIsTheReviewAnUndoCancels(t *testing.T) {
	card := core.NewID("card")
	applier := schedule.NewApplier(schedule.Params{})
	if _, ok := applier.Last(card); ok {
		t.Error("Last found a review in an empty log")
	}
	for _, ev := range []schedule.Event{
		review(1, card, schedule.Good, t0),
		review(2, card, schedule.Good, t0.AddDate(0, 0, 1)),
		{Seq: 3, Void: 2},
	} {
		if err := applier.Apply(ev); err != nil {
			t.Fatalf("Apply(%+v): %v", ev, err)
		}
	}
	last, ok := applier.Last(card)
	if !ok || last.Seq != 1 {
		t.Errorf("Last = (%+v, %v), want the first review", last, ok)
	}
	if state, ok := applier.State(card); !ok || state.Reviews != 1 {
		t.Errorf("State = (%+v, %v), want one live review", state, ok)
	}
}

// The preview on the grade keys is the schedule's own answer: each outlook is
// the state Step would leave, and asking changes nothing about the card.
func TestPreviewMatchesStep(t *testing.T) {
	at := time.Date(2026, time.March, 2, 9, 0, 0, 0, time.UTC)
	params := schedule.DefaultParams()

	learned := params.Step(schedule.State{}, schedule.Event{Rating: schedule.Good, At: at.Add(-48 * time.Hour)})
	cases := []struct {
		name  string
		state schedule.State
		now   time.Time
	}{
		{"a card never reviewed", schedule.State{}, at},
		{"a card being learned", learned, at},
		{"a card learned a week ago", learned, at.Add(7 * 24 * time.Hour)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := c.state
			got := params.Preview(c.state, c.now)
			if c.state != before {
				t.Error("Preview changed the state it was given")
			}
			if len(got) != 4 {
				t.Fatalf("preview has %d outlooks, want 4", len(got))
			}
			for i, outlook := range got {
				r := schedule.Rating(i) + schedule.Again
				want := params.Step(before, schedule.Event{Rating: r, At: c.now})
				if !outlook.Due.Equal(want.Due) {
					t.Errorf("%v: preview due %v, step due %v", r, outlook.Due, want.Due)
				}
				if outlook.Rating != r {
					t.Errorf("outlook %d is labelled %v, want %v", i, outlook.Rating, r)
				}
				if outlook.Wait <= 0 {
					t.Errorf("%v: preview wait %v, want a positive interval", r, outlook.Wait)
				}
			}
			// The four answers differ: a preview showing one interval for every
			// key would be no preview at all.
			if got[0].Wait >= got[3].Wait {
				t.Errorf("Again waits %v, not less than Easy's %v", got[0].Wait, got[3].Wait)
			}
		})
	}
}
