package store_test

import (
	"slices"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core/store"
)

// TestQueryTagsCountsAndSamples says what the tag page shows per tag: how many
// notes carry it, and a couple of their titles so the tag is recognisable. Most
// used comes first; a tag carried by one note has one example.
func TestQueryTagsCountsAndSamples(t *testing.T) {
	lib, _, clk := newLibrary(t)
	saveNote(t, lib, "Biology", "spacing beats cramming #study")
	clk.advance(time.Hour)
	saveNote(t, lib, "Chemistry", "bonds break #study #chem")
	clk.advance(time.Hour)
	saveNote(t, lib, "Physics", "entropy grows #study")
	clk.advance(time.Hour)
	saveNote(t, lib, "Maths", "proofs follow #maths")

	tests := []struct {
		name   string
		limit  int
		want   []string // tags in order
		counts map[string]int
		first  map[string]string // the newest note carrying the tag
	}{
		{
			name:   "most used first",
			want:   []string{"study", "chem", "maths"},
			counts: map[string]int{"study": 3, "chem": 1, "maths": 1},
			first:  map[string]string{"study": "Biology"},
		},
		{
			name:   "the page keeps the most used",
			limit:  2,
			want:   []string{"study", "chem"},
			counts: map[string]int{"study": 3, "chem": 1},
			first:  map[string]string{"study": "Biology"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := lib.QueryTags(tt.limit)
			got := make([]string, 0, len(rows))
			for _, row := range rows {
				got = append(got, row.Tag)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("tags = %v, want %v", got, tt.want)
			}
			for _, row := range rows {
				if row.Count != tt.counts[row.Tag] {
					t.Errorf("%s counts %d notes, want %d", row.Tag, row.Count, tt.counts[row.Tag])
				}
				if len(row.Sample) == 0 || len(row.Sample) > store.TagSampleTitles {
					t.Fatalf("%s samples %v, want one or two titles", row.Tag, row.Sample)
				}
				if want := tt.first[row.Tag]; want != "" && row.Sample[0] != want {
					t.Errorf("%s's first example = %q, want %q", row.Tag, row.Sample[0], want)
				}
			}
		})
	}

	t.Run("a tag on several blocks of one note counts once", func(t *testing.T) {
		note := saveNote(t, lib, "Twice", "one #twice", "two #twice")
		if note.Title != "Twice" {
			t.Fatalf("saved %q", note.Title)
		}
		for _, row := range lib.QueryTags(0) {
			if row.Tag == "twice" && row.Count != 1 {
				t.Errorf("twice counts %d notes, want 1", row.Count)
			}
		}
	})

	t.Run("a library with no tags", func(t *testing.T) {
		empty, _, _ := newLibrary(t)
		saveNote(t, empty, "Plain", "no tags here")
		if rows := empty.QueryTags(0); len(rows) != 0 {
			t.Errorf("tags = %+v, want none", rows)
		}
	})

	t.Run("a rebuilt index answers the same", func(t *testing.T) {
		before := lib.QueryTags(0)
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
		after := lib.QueryTags(0)
		if len(after) != len(before) {
			t.Fatalf("rebuilt tags = %+v, want %d of them", after, len(before))
		}
		for i := range before {
			if after[i].Tag != before[i].Tag || after[i].Count != before[i].Count ||
				!slices.Equal(after[i].Sample, before[i].Sample) {
				t.Errorf("rebuilt tag %d = %+v, want %+v", i, after[i], before[i])
			}
		}
	})
}
