package core

import "strings"

// MarkerSigil starts the ID a referencable paragraph ends with. It is the app's
// convention inside the document, not the file format's: a paragraph of a note
// that a card points at carries it, in plain text, so that identity survives
// editing, reordering and any Markdown tool (ADR 0005).
const MarkerSigil = "^"

// BlockKind is the tag a block's identifier carries: the paragraphs of a note
// are the blocks a card can point at.
const BlockKind = "block"

// BlockMarker is a block's identity as it is written at the end of its text.
func BlockMarker(id ID) string { return MarkerSigil + string(id) }

// SplitBlockMarker reads the ID a line ends with. The marker is the line's last
// whitespace-separated token, the sigil and an ID; rest is the line without it.
// ok is false when the line does not end with a well-formed ID, and callers
// check the kind the ID carries against the kind they hold.
func SplitBlockMarker(line string) (id ID, rest string, ok bool) {
	trimmed := strings.TrimRight(line, " \t")
	cut := strings.LastIndexAny(trimmed, " \t")
	token := trimmed[cut+1:]
	raw, found := strings.CutPrefix(token, MarkerSigil)
	if !found {
		return "", line, false
	}
	// A Markdown serializer escapes the underscore in an id, so the text an
	// editor hands back may hold it escaped. The file never does: it writes
	// the marker itself.
	id = ID(strings.ReplaceAll(raw, `\_`, "_"))
	if !id.Valid() {
		return "", line, false
	}
	return id, strings.TrimRight(trimmed[:cut+1], " \t"), true
}

// CutBlockMarker finds the marker of a paragraph wherever it sits, and returns
// the paragraph without it. An editor puts the caret at the end of the document,
// which is past the last paragraph's marker, so what the user types next lands
// after it: the marker still names that paragraph, and cutting it puts the
// paragraph back in the shape the file holds.
func CutBlockMarker(text string) (id ID, rest string, ok bool) {
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] != MarkerSigil[0] {
			continue
		}
		// The id runs to the end of the token, and a serializer may have
		// escaped its underscore.
		end := i + 1
		for end < len(text) && text[end] != ' ' && text[end] != '\t' && text[end] != '\n' {
			end++
		}
		token := text[i+1 : end]
		for cut := len(token); cut > 0; cut-- {
			candidate := ID(strings.ReplaceAll(token[:cut], `\_`, "_"))
			if candidate.Kind() != BlockKind || !candidate.Valid() {
				continue
			}
			// What follows the id in the same token is text the user typed
			// after the marker.
			return candidate, strings.TrimSpace(text[:i] + token[cut:] + text[end:]), true
		}
	}
	return "", text, false
}
