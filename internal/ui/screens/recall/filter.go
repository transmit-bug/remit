package recall

import (
	"strings"

	"github.com/transmit-bug/remit/locales"
)

// Filter is what the queue page keeps of the cards due: all of them, one kind of
// card, or one trouble. It narrows the rows the page lists; the session's scope
// is the scope button's, so a filter never changes what Start takes.
type Filter uint8

const (
	// FilterAll is every card the queue holds now.
	FilterAll Filter = iota
	// FilterDue is the cards a schedule put off once, and that are due again.
	FilterDue
	// FilterNew is the cards no review has touched.
	FilterNew
	// FilterLearning is the cards last answered Again, still being learned.
	FilterLearning
	// FilterLost is the cards whose paragraph is gone from the library.
	FilterLost
	// FilterHard is the cards that keep coming back wrong.
	FilterHard

	filterCount
)

// Names are the filters' stable ids, which a saved filter keeps in
// settings.json: a filter is a choice, and the file must read the same after the
// language changes.
var names = [...]string{"all", "due", "new", "learning", "lost", "hard"}

// Name is the filter's stable id.
func (f Filter) Name() string {
	if int(f) < len(names) {
		return names[f]
	}
	return names[FilterAll]
}

// FilterOf reads a saved filter's id. An id this build does not know is the
// whole queue: a filter saved by a later build must not hide the day's cards.
func FilterOf(name string) Filter {
	for i, candidate := range names {
		if strings.EqualFold(name, candidate) {
			return Filter(i)
		}
	}
	return FilterAll
}

// Filters reads saved filter ids, skipping the ones this build cannot apply and
// the repeats.
func Filters(ids []string) []Filter {
	seen := map[Filter]bool{}
	out := make([]Filter, 0, len(ids))
	for _, id := range ids {
		f := FilterOf(id)
		if seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// label names the filter in this frame's language.
func (f Filter) label(msgs locales.Messages) string {
	switch f {
	case FilterDue:
		return msgs.RecallFilterDue()
	case FilterNew:
		return msgs.RecallFilterNew()
	case FilterLearning:
		return msgs.RecallFilterLearning()
	case FilterLost:
		return msgs.RecallFilterLost()
	case FilterHard:
		return msgs.RecallFilterHard()
	}
	return msgs.RecallFilterAll()
}
