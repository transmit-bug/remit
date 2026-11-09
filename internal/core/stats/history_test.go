package stats

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// A library nobody has read or reviewed has no history: no crash, no invented
// line, and the page's empty state is the whole answer.
func TestHistoryOfAnEmptyLibrary(t *testing.T) {
	lib := emptyLibrary(t)

	got := HistoryOf(lib, day0, day0.Add(24*time.Hour))
	if got.Total != 0 || got.Again != 0 || len(got.Events) != 0 {
		t.Fatalf("history of an empty library = %+v", got)
	}
	if got.Grades != [4]int{} {
		t.Errorf("grades of an empty library = %v", got.Grades)
	}
}

// One day of work holds both kinds of memory activity, and the timeline keeps
// them apart: a session shows the note it read, an answer shows the card.
func TestHistoryListsReadingAndCardEvents(t *testing.T) {
	lib, cards := library(t)
	note := writeNote(t, lib, "long read", "a paragraph")
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, day0.Add(time.Hour)); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if err := lib.Review(cards[0].ID, schedule.Good, day0.Add(2*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}

	got := HistoryOf(lib, day0, day0.Add(24*time.Hour))
	if len(got.Events) != 2 {
		t.Fatalf("history = %+v, want the reading and the review", got.Events)
	}
	// Oldest first, so the page lists the day as it happened.
	read, answered := got.Events[0], got.Events[1]
	if read.Kind != KindReading || read.NoteTitle != "long read" || read.CardID != "" {
		t.Errorf("first event = %+v, want the reading of \"long read\"", read)
	}
	if read.Rating != schedule.Good {
		t.Errorf("the session's rating = %v, want good", read.Rating)
	}
	if answered.Kind != KindCard || answered.CardID != cards[0].ID || answered.NoteTitle != "" {
		t.Errorf("second event = %+v, want the card answer", answered)
	}
	if got.Total != 2 || got.Again != 0 || got.Grades != [4]int{0, 0, 2, 0} {
		t.Errorf("history counted %+v, want two good answers", got)
	}

	// A card and a note nobody touched are not on the timeline.
	if history := HistoryOf(lib, day0.Add(-24*time.Hour), day0); len(history.Events) != 0 {
		t.Errorf("yesterday has %d events, want none", len(history.Events))
	}
}

// An undone answer was never given: the void takes the review with it, and the
// log stays as it was.
func TestHistoryExcludesVoidedReviews(t *testing.T) {
	lib, cards := library(t)
	note := writeNote(t, lib, "long read", "a paragraph")
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, day0); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if err := lib.UndoReading(note.ID, day0.Add(time.Hour)); err != nil {
		t.Fatalf("UndoReading: %v", err)
	}
	if err := lib.Review(cards[0].ID, schedule.Good, day0.Add(2*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.UndoLast(cards[0].ID); err != nil {
		t.Fatalf("undo: %v", err)
	}

	got := HistoryOf(lib, day0, day0.Add(24*time.Hour))
	if got.Total != 0 || len(got.Events) != 0 {
		t.Errorf("history = %+v, want the undone answers left out", got)
	}
}

// Archiving ends a note's reading, not the record of it: the page still shows
// the session that happened, and the archive event itself is not a review.
func TestHistoryKeepsAnArchivedNotesReading(t *testing.T) {
	lib, _ := library(t)
	note := writeNote(t, lib, "finished", "a paragraph")
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, day0); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	if err := lib.ArchiveReading(note.ID, day0.Add(time.Hour)); err != nil {
		t.Fatalf("ArchiveReading: %v", err)
	}

	got := HistoryOf(lib, day0, day0.Add(24*time.Hour))
	if len(got.Events) != 1 || got.Events[0].NoteTitle != "finished" || got.Total != 1 {
		t.Errorf("history = %+v, want the archived note's session alone", got)
	}
}

// The four numbers S34 shows: how many answers a range held, how many were
// Again, and how they split across the four grades.
func TestHistoryCountsGrades(t *testing.T) {
	for _, tt := range []struct {
		name    string
		ratings []schedule.Rating
		grades  [4]int
		again   int
	}{
		{"no answers", nil, [4]int{}, 0},
		{"one of each", []schedule.Rating{schedule.Again, schedule.Hard, schedule.Good, schedule.Easy}, [4]int{1, 1, 1, 1}, 1},
		{"two agains", []schedule.Rating{schedule.Again, schedule.Again}, [4]int{2, 0, 0, 0}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib, cards := library(t)
			for i, rating := range tt.ratings {
				if err := lib.Review(cards[0].ID, rating, day0.Add(time.Duration(i)*time.Hour)); err != nil {
					t.Fatalf("review: %v", err)
				}
			}
			got := HistoryOf(lib, day0, day0.Add(24*time.Hour))
			if got.Total != len(tt.ratings) || got.Grades != tt.grades || got.Again != tt.again {
				t.Errorf("history = %d answers %v grades, %d again; want %d %v %d",
					got.Total, got.Grades, got.Again, len(tt.ratings), tt.grades, tt.again)
			}
		})
	}
}

// An answer moves an interval, and the timeline reads the move off the schedule
// rather than remembering numbers of its own: the first answer of an entry has
// nothing to move from, and a range that starts in the middle still knows the
// interval the answer before it set.
func TestHistoryIntervalsComeFromNeighbouringAnswers(t *testing.T) {
	type step struct {
		after  time.Duration
		rating schedule.Rating
	}
	for _, tt := range []struct {
		name  string
		steps []step
		from  time.Duration // the range starts this far into day0
	}{
		{"one answer", []step{{0, schedule.Good}}, 0},
		{"two goods", []step{{0, schedule.Good}, {72 * time.Hour, schedule.Good}}, 0},
		{"again then good", []step{{0, schedule.Again}, {24 * time.Hour, schedule.Good}, {240 * time.Hour, schedule.Hard}}, 0},
		{"a range that starts in the middle", []step{{0, schedule.Good}, {72 * time.Hour, schedule.Good}, {240 * time.Hour, schedule.Easy}}, 120 * time.Hour},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib, cards := library(t)
			// The schedule's own answer for each step, so the test names no
			// interval of its own.
			var scheduled []int
			for _, s := range tt.steps {
				at := day0.Add(s.after)
				if err := lib.Review(cards[0].ID, s.rating, at); err != nil {
					t.Fatalf("review: %v", err)
				}
				state, ok := lib.CardState(cards[0].ID)
				if !ok {
					t.Fatal("no state after a review")
				}
				scheduled = append(scheduled, int(state.Due.Sub(at).Hours()/24))
			}

			first := 0
			for i, s := range tt.steps {
				if !day0.Add(s.after).Before(day0.Add(tt.from)) {
					first = i
					break
				}
			}
			got := HistoryOf(lib, day0.Add(tt.from), day0.Add(400*24*time.Hour))
			if len(got.Events) != len(tt.steps)-first {
				t.Fatalf("history = %+v, want %d events", got.Events, len(tt.steps)-first)
			}
			for i, event := range got.Events {
				wantFrom, wantTo := 0, scheduled[first+i]
				if first+i > 0 {
					wantFrom = scheduled[first+i-1]
				}
				if event.IntervalFrom != wantFrom || event.IntervalTo != wantTo {
					t.Errorf("event %d moved %d → %d days, want %d → %d",
						i, event.IntervalFrom, event.IntervalTo, wantFrom, wantTo)
				}
			}
		})
	}
}

// The hard-card filter W5 draws reads one number — the lapses replay counts —
// and compares it against a preset's threshold. B4 supplies the number, not
// the threshold: no preset is written and no second source is kept.
func TestLapsesAreAvailableForTheHardCardThreshold(t *testing.T) {
	lib, cards := library(t)
	for i, rating := range []schedule.Rating{schedule.Again, schedule.Again, schedule.Good} {
		if err := lib.Review(cards[0].ID, rating, day0.Add(time.Duration(i)*24*time.Hour)); err != nil {
			t.Fatalf("review: %v", err)
		}
	}

	states := map[core.ID]schedule.State{}
	scheduled, err := lib.Scheduled()
	if err != nil {
		t.Fatalf("Scheduled: %v", err)
	}
	for _, card := range scheduled {
		states[card.Card.ID] = card.State
	}
	if got := states[cards[0].ID].Lapses; got != 2 {
		t.Fatalf("the reviewed card has %d lapses, want 2", got)
	}

	for _, tt := range []struct {
		threshold int
		want      int
	}{{2, 1}, {3, 0}} {
		hard := 0
		for _, state := range states {
			if state.Lapses >= tt.threshold {
				hard++
			}
		}
		if hard != tt.want {
			t.Errorf("threshold %d selects %d hard cards, want %d", tt.threshold, hard, tt.want)
		}
	}

	// The timeline's Again count and the card's lapses are the same log read
	// twice, so the chip and the history cannot disagree.
	if got := HistoryOf(lib, day0, day0.Add(72*time.Hour)); got.Again != states[cards[0].ID].Lapses {
		t.Errorf("history counted %d agains, the schedule %d lapses", got.Again, states[cards[0].ID].Lapses)
	}
}
