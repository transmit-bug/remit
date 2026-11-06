package core

import "strings"

// LinkOpen and LinkClose wrap a wiki link in a block's text: [[Note title]]
// names another note, and the derived index holds the edge from the block that
// carries it to the note it names.
const (
	LinkOpen  = "[["
	LinkClose = "]]"
)

// Link is one wiki link: the title it names, as written.
type Link struct {
	Title string
}

// Links returns the wiki links a block's text carries, in the order they appear.
// A link that is not closed, or that names nothing, is text and not a link.
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
		title := strings.TrimSpace(rest[:end])
		rest = rest[end+len(LinkClose):]
		if title == "" || strings.Contains(title, LinkOpen) {
			continue
		}
		out = append(out, Link{Title: title})
	}
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
