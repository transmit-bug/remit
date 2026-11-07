package review

// Kind is what a run of characters in a comparison is.
type Kind uint8

const (
	// Equal is a run the typed answer and the card's answer share.
	Equal Kind = iota
	// Extra is a run the user typed and the answer does not have.
	Extra
	// Missing is a run the answer has and the user did not type.
	Missing
)

// Span is a run of characters of one Kind, in reading order.
type Span struct {
	Text string
	Kind Kind
}

// Verdict is what a compared answer came to.
type Verdict uint8

const (
	// Correct is an answer whose every rune is the card's, in place.
	Correct Verdict = iota
	// Partial is one that matched somewhere and not everywhere.
	Partial
	// Wrong is one that matched nowhere.
	Wrong
)

// Compare aligns a typed answer with the card's, rune by rune, along their
// longest common subsequence. Every rune of either side lands in exactly one
// span: what both share is Equal, what only the typed side has is Extra, and
// what only the answer has is Missing.
//
// Comparison is literal: no case folding, no trimming, and a letter with a
// combining mark is the two runes it is. Nor is a transposition a match —
// "ab" typed for "ba" is a run of Equal around an Extra and a Missing, which Of
// reads as Partial, and that is the boundary between the three verdicts.
func Compare(want, got string) []Span {
	a, b := []rune(want), []rune(got)
	// common[i][j] is the length of the longest common subsequence of a[i:] and
	// b[j:]. The answers a card takes are short phrases; the table is at its
	// largest the typed input's length times the answer's.
	common := make([][]int32, len(a)+1)
	for i := range common {
		common[i] = make([]int32, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				common[i][j] = common[i+1][j+1] + 1
			case common[i+1][j] >= common[i][j+1]:
				common[i][j] = common[i+1][j]
			default:
				common[i][j] = common[i][j+1]
			}
		}
	}

	out := make([]Span, 0, len(a)+len(b))
	add := func(kind Kind, r rune) {
		if n := len(out); n > 0 && out[n-1].Kind == kind {
			out[n-1].Text += string(r)
			return
		}
		out = append(out, Span{Text: string(r), Kind: kind})
	}
	// Walk the two strings against the table, taking what they share and, where
	// they part, the side the table says loses less. A tie takes the typed run
	// first, so an inserted letter reads before the one it displaced.
	for i, j := 0, 0; i < len(a) || j < len(b); {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			add(Equal, a[i])
			i, j = i+1, j+1
		case j < len(b) && (i == len(a) || common[i][j+1] >= common[i+1][j]):
			add(Extra, b[j])
			j++
		default:
			add(Missing, a[i])
			i++
		}
	}
	return out
}

// Of reads a comparison as the verdict the screen says: Correct when nothing
// was left out or typed wrong, Wrong when nothing matched, and Partial in
// between — a typo, an extra run, a missing one, or two letters in each other's
// places.
func Of(spans []Span) Verdict {
	matched, differed := false, false
	for _, span := range spans {
		if span.Kind == Equal {
			matched = true
		} else {
			differed = true
		}
	}
	switch {
	case !differed:
		return Correct
	case matched:
		return Partial
	default:
		return Wrong
	}
}
