package core

import "strings"

// LinkOpen and LinkClose wrap a wiki link in a block's text: [[Note title]]
// names another note, and the derived index holds the edge from the block that
// carries it to the note it names. A link may name one block of that note —
// [[Note title#^block_…]] — which is what an excerpt writes: it points at the
// paragraph it was taken from, not only at the note.
const (
	LinkOpen  = "[["
	LinkClose = "]]"
	// LinkAnchor separates a note's title from the block it names.
	LinkAnchor = "#"
)

// Link is one wiki link: the title it names, as written, and the block it points
// into when it names one.
type Link struct {
	Title string
	Block ID
}

// Links returns the wiki links a block's text carries, in the order they appear.
// A link that is not closed, or that names nothing, is text and not a link; an
// anchor that is not a well-formed block ID is part of the title.
func Links(text string) []Link {
	var out []Link
	for rest := text; ; {
		open := strings.Index(rest, LinkOpen)
		if open < 0 {
			return out
		}
		rest = rest[open+len(LinkOpen):]
		end := strings.Index(rest, LinkClose)
		if end < 0 {
			return out
		}
		link := Link{Title: strings.TrimSpace(rest[:end])}
		rest = rest[end+len(LinkClose):]
		if name, anchor, found := strings.Cut(link.Title, LinkAnchor); found {
			if id, ok := blockAnchor(anchor); ok {
				link.Title, link.Block = strings.TrimSpace(name), id
			}
		}
		if link.Title == "" || strings.Contains(link.Title, LinkOpen) {
			continue
		}
		out = append(out, link)
	}
}

// blockAnchor reads the block a link's anchor names: the sigil and an ID.
func blockAnchor(anchor string) (ID, bool) {
	raw, found := strings.CutPrefix(strings.TrimSpace(anchor), MarkerSigil)
	if !found {
		return "", false
	}
	id := ID(strings.ReplaceAll(raw, `\_`, "_"))
	if id.Kind() != BlockKind || !id.Valid() {
		return "", false
	}
	return id, true
}

// NormalizeTitle is how two titles are held to be the same note: trimmed, with
// runs of space collapsed and the case folded.
func NormalizeTitle(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// Paragraphs splits a note's body into its paragraphs, as the editor's
// documents are: blank lines separate them.
func Paragraphs(text string) []string {
	parts := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if body := strings.TrimSpace(part); body != "" {
			out = append(out, body)
		}
	}
	return out
}
