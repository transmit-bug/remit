package recall_test

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/ui/screens/recall"
)

// The strip covers half a year of calendar weeks: twenty-six columns of seven
// days ending with the week today falls in, each day's count where the log put
// it, and the days ahead marked as not due yet.
func TestHeatWeeksFoldsTheLogIntoCalendarColumns(t *testing.T) {
	today := time.Date(2026, time.November, 5, 15, 30, 0, 0, time.UTC) // a Thursday
	weeks, most := recall.HeatWeeks(today, []stats.DayCount{
		{Day: "2026-11-05", Count: 4},
		{Day: "2026-11-04", Count: 2},
		{Day: "2026-05-11", Count: 1},
	})

	if len(weeks) != 26 {
		t.Fatalf("the strip holds %d weeks, want 26", len(weeks))
	}
	for i, week := range weeks {
		if len(week) != 7 {
			t.Fatalf("week %d holds %d days, want 7", i, len(week))
		}
	}
	from, to := recall.HeatRange(today)
	if got := from.Format("2006-01-02"); got != "2026-05-11" {
		t.Errorf("the strip starts %s, want the Monday 26 weeks back", got)
	}
	if got := to.Format("2006-01-02"); got != "2026-11-09" {
		t.Errorf("the strip ends %s, want the Monday after today's week", got)
	}
	// A column runs Monday to Sunday, so today is the fourth row of the last one.
	last := weeks[len(weeks)-1]
	if got := last[3].Day.Format("2006-01-02"); got != "2026-11-05" {
		t.Errorf("the last column's Thursday is %s, want today", got)
	}
	if last[3].Count != 4 || last[3].Future {
		t.Errorf("today's square is %+v, want four reviews and not future", last[3])
	}
	if last[2].Future || !last[4].Future {
		t.Errorf("the days ahead are not marked: %+v %+v", last[2], last[4])
	}
	if most != 4 {
		t.Errorf("the strip's busiest day is %d reviews, want 4", most)
	}
}

// A saved filter reads back as the filter it names, and an id this build does not
// know is the whole queue rather than a page that hides the day's cards.
func TestFiltersRoundTrip(t *testing.T) {
	if got := recall.FilterOf(recall.FilterHard.Name()); got != recall.FilterHard {
		t.Errorf("the hard filter reads back as %v", got)
	}
	if got := recall.FilterOf("from-a-later-build"); got != recall.FilterAll {
		t.Errorf("an unknown filter reads back as %v, want every card", got)
	}
	got := recall.Filters([]string{"new", "hard", "new", "nonsense"})
	if len(got) != 3 || got[0] != recall.FilterNew || got[1] != recall.FilterHard || got[2] != recall.FilterAll {
		t.Errorf("saved filters read back as %v", got)
	}
	if got := recall.Filters(nil); len(got) != 0 {
		t.Errorf("no saved filters read back as %v", got)
	}
}

// A scope's count is the scope's own: the whole library, one note, or one tag,
// and zero for a scope nothing was derived for.
func TestScopeCountsAnswerEachScope(t *testing.T) {
	counts := recall.ScopeCounts{
		All:   9,
		Notes: map[core.ID]int{"note_a": 3},
		Tags:  map[string]int{"chapter": 2},
	}
	for _, tc := range []struct {
		name  string
		scope queue.Scope
		want  int
	}{
		{"library", queue.Scope{}, 9},
		{"note", queue.Scope{Kind: queue.ScopeNote, Note: "note_a"}, 3},
		{"tag", queue.Scope{Kind: queue.ScopeTag, Tag: "chapter"}, 2},
		{"a note with nothing due", queue.Scope{Kind: queue.ScopeNote, Note: "note_z"}, 0},
	} {
		if got := counts.Count(tc.scope); got != tc.want {
			t.Errorf("%s counts %d cards, want %d", tc.name, got, tc.want)
		}
	}
}
