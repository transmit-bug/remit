package queue

import (
	"strings"
	"unicode"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// ParseCardQuery reads the card table's one search field into the store query
// behind it.
//
// The syntax is what a person types there, a term at a time:
//
//	is:new  is:learning  is:review  is:suspended   the card's lifecycle status
//	is:live  is:frozen  is:lost                    where the card's text comes from
//	is:hard                                        a card answered "again" at least once
//	type:remit.cloze                               the card type, by ID
//	tag:study                                       a tag the card's note carries
//	note:Memory and recall                          the note the card's block is in
//
// Space separates terms and terms are ANDed, so "tag:study is:review" asks for
// the reviewed cards of the study notes. A value with a space in it is quoted:
// note:"Memory and recall". Everything a key does not claim is text, and so is a
// key the parser does not know — a search for "is:" or "due:tomorrow" searches
// for those words rather than quietly matching everything.
//
// is:hard is at least one lapse: the parser has no library to ask for a preset's
// hard-card threshold, so a caller that knows it raises MinLapses to that
// threshold before running the query.
func ParseCardQuery(raw string) store.CardQuery {
	var query store.CardQuery
	var text []string
	for _, term := range splitTerms(raw) {
		key, value, hasKey := strings.Cut(term, ":")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !hasKey || value == "" {
			text = append(text, term)
			continue
		}
		switch key {
		case "is":
			if !applyIs(&query, strings.ToLower(value)) {
				text = append(text, term)
			}
		case "type":
			if !isTypeID(core.ID(value)) {
				text = append(text, term)
				continue
			}
			query.Type = core.ID(value)
		case "tag":
			tag := strings.TrimPrefix(value, "#")
			if tag == "" {
				text = append(text, term)
				continue
			}
			query.Tag = tag
		case "note":
			query.Note = value
		default:
			text = append(text, term)
		}
	}
	query.Text = strings.Join(text, " ")
	return query
}

// applyIs reads an is: value into the query, answering false for one it does not
// know so the caller keeps it as text.
func applyIs(query *store.CardQuery, value string) bool {
	switch value {
	case "new", "learning", "review", "suspended":
		status, ok := core.ParseStatus(value)
		if !ok {
			return false
		}
		query.Status = &status
		return true
	case "live", "frozen", "lost":
		ref := core.LinkLive
		switch value {
		case "frozen":
			ref = core.LinkFrozen
		case "lost":
			ref = core.LinkLost
		}
		query.Ref = &ref
		return true
	case "hard":
		// One lapse is the floor of "hard"; a preset's threshold is the
		// caller's to apply.
		query.MinLapses = 1
		return true
	}
	return false
}

// isTypeID reports whether the text names a card type: a built-in ("remit.basic",
// "remit.cloze", …) or a type this library minted ("type_…"). Anything else stays
// text, so a typo keeps searching instead of emptying the table.
func isTypeID(id core.ID) bool {
	return strings.HasPrefix(string(id), "remit.") || id.Kind() == "type"
}

// splitTerms cuts a query into its terms, on space outside double quotes. The
// quotes are dropped and what they held stays together, so note:"Memory and
// recall" is one term; a quote that is never closed runs to the end of the line,
// which is what a half-typed query looks like.
func splitTerms(raw string) []string {
	var (
		terms  []string
		term   strings.Builder
		quoted bool
	)
	flush := func() {
		if term.Len() > 0 {
			terms = append(terms, term.String())
			term.Reset()
		}
	}
	for _, r := range raw {
		switch {
		case r == '"':
			quoted = !quoted
		case !quoted && unicode.IsSpace(r):
			flush()
		default:
			term.WriteRune(r)
		}
	}
	flush()
	return terms
}
