package core

import "strings"

// Cloze turns a selection into a cloze blank: the first place the selected text
// appears becomes {{…}}, which is the shape the cloze type and the card
// templates read as one blank (ADR 0008).
//
// It is the editor's one rewrite for a selection, so it is pure and it is
// idempotent where it can be: a text that already holds that exact blank is
// returned unchanged, and a selection that is not in the text — or is only
// whitespace — leaves the text as it was rather than wrapping nothing.
//
// Offsets are byte offsets but every boundary Cloze inserts falls between
// characters: the selection it finds is matched whole, so a multi-byte
// character is never split, and a selection may itself be any script.
func Cloze(text, selected string) string {
	if strings.TrimSpace(selected) == "" {
		return text
	}
	if isBlank(selected) {
		// The selection is one blank already: the braces belong to it, so a
		// pass over them would end the blank where the next brace closes it.
		return text
	}
	clozed := "{{" + selected + "}}"
	if strings.Contains(text, clozed) {
		return text
	}
	at := strings.Index(text, selected)
	if at < 0 {
		return text
	}
	end := at + len(selected)
	// The blank is built in one write, so a selection that begins or ends with
	// a wide character keeps it whole.
	var out strings.Builder
	out.Grow(len(text) + 4)
	out.WriteString(text[:at])
	out.WriteString(clozed)
	out.WriteString(text[end:])
	return out.String()
}

// isBlank reports whether the text is one {{…}} blank and nothing else.
func isBlank(text string) bool {
	inner, ok := strings.CutPrefix(text, "{{")
	if !ok {
		return false
	}
	inner, ok = strings.CutSuffix(inner, "}}")
	return ok && inner != "" && !strings.Contains(inner, "{{") && !strings.Contains(inner, "}}")
}
