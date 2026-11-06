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
	id = ID(raw)
	if !id.Valid() {
		return "", line, false
	}
	return id, strings.TrimRight(trimmed[:cut+1], " \t"), true
}
