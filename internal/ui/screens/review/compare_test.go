package review_test

import (
	"reflect"
	"testing"

	"github.com/transmit-bug/remit/internal/ui/screens/review"
)

// span is one run of a comparison the table below expects.
func span(text string, kind review.Kind) review.Span {
	return review.Span{Text: text, Kind: kind}
}

// Compare aligns a typed answer with the card's along the letters they share,
// and Of reads that alignment as one of the three verdicts. The table pins the
// boundary between the verdicts: two letters in each other's places are Partial,
// not Correct, and the comparison is literal — case and combining marks count.
func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		name  string
		want  string
		got   string
		spans []review.Span
		of    review.Verdict
	}{
		{
			name:  "all correct",
			want:  "colour",
			got:   "colour",
			spans: []review.Span{span("colour", review.Equal)},
			of:    review.Correct,
		},
		{
			name:  "a typo drops a letter",
			want:  "colour",
			got:   "colur",
			spans: []review.Span{span("col", review.Equal), span("o", review.Missing), span("ur", review.Equal)},
			of:    review.Partial,
		},
		{
			name:  "a substitution",
			want:  "abc",
			got:   "axc",
			spans: []review.Span{span("a", review.Equal), span("x", review.Extra), span("b", review.Missing), span("c", review.Equal)},
			of:    review.Partial,
		},
		{
			name:  "extra characters",
			want:  "cat",
			got:   "cats",
			spans: []review.Span{span("cat", review.Equal), span("s", review.Extra)},
			of:    review.Partial,
		},
		{
			name:  "nothing typed",
			want:  "colour",
			got:   "",
			spans: []review.Span{span("colour", review.Missing)},
			of:    review.Wrong,
		},
		{
			name:  "everything wrong",
			want:  "cat",
			got:   "dog",
			spans: []review.Span{span("dog", review.Extra), span("cat", review.Missing)},
			of:    review.Wrong,
		},
		{
			// A transposition is not a match: the answer's letters are there,
			// both of them, but not where they belong.
			name:  "two neighbours swapped",
			want:  "ab",
			got:   "ba",
			spans: []review.Span{span("b", review.Extra), span("a", review.Equal), span("b", review.Missing)},
			of:    review.Partial,
		},
		{
			name:  "case is part of the answer",
			want:  "Paris",
			got:   "paris",
			spans: []review.Span{span("p", review.Extra), span("P", review.Missing), span("aris", review.Equal)},
			of:    review.Partial,
		},
		{
			name:  "CJK",
			want:  "日本語",
			got:   "日本後",
			spans: []review.Span{span("日本", review.Equal), span("後", review.Extra), span("語", review.Missing)},
			of:    review.Partial,
		},
		{
			// The composed é and the decomposed e + mark are different runes:
			// the comparison says so rather than guessing at normalisation.
			name:  "a combining mark is its own rune",
			want:  "café",
			got:   "cafe\u0301",
			spans: []review.Span{span("caf", review.Equal), span("e\u0301", review.Extra), span("é", review.Missing)},
			of:    review.Partial,
		},
		{
			name:  "nothing on either side",
			want:  "",
			got:   "",
			spans: []review.Span{},
			of:    review.Correct,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := review.Compare(tc.want, tc.got)
			if !reflect.DeepEqual(got, tc.spans) {
				t.Errorf("Compare(%q, %q) = %v, want %v", tc.want, tc.got, got, tc.spans)
			}
			if of := review.Of(got); of != tc.of {
				t.Errorf("Of(Compare(%q, %q)) = %v, want %v", tc.want, tc.got, of, tc.of)
			}
		})
	}
}
