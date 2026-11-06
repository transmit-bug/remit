package ui

import (
	"strings"

	"github.com/transmit-bug/remit/internal/core"
)

// documentText is the note as the editor holds it: its paragraphs, separated by a
// blank line, without the markers that carry their identity.
//
// The markers live in the file, not in the editor (ADR 0005's "the editor hides
// it"). They are kept out of the text the user types in because the editor's
// caret can come to rest inside one — at the end of a paragraph, or a few
// characters before the end — and then keystrokes split the id and the paragraph
// loses the card that pointed at it. Identity is re-attached on save, by place.
func documentText(note core.Note) string {
	var out strings.Builder
	for i, block := range note.Blocks {
		if i > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(strings.TrimRight(block.Text, " \t"))
	}
	return out.String()
}

// blocksOfDocument reads the paragraphs of an edited document back into blocks.
// A paragraph keeps the place it had — the third paragraph is the third block —
// so identity survives typing inside a paragraph, adding one and deleting one,
// and is only lost when paragraphs are reordered. A paragraph that appears where
// there was none gets a new identity. Blank text is not a block.
func blocksOfDocument(text string, prev []core.Block) []core.Block {
	paragraphs := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	out := make([]core.Block, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		body := strings.TrimSpace(paragraph)
		if body == "" {
			continue
		}
		id := core.ID("")
		if at := len(out); at < len(prev) {
			id = prev[at].ID
		}
		if id == "" {
			id = core.NewID(core.BlockKind)
		}
		out = append(out, core.Block{ID: id, Text: body})
	}
	return out
}

// paragraphAt is the paragraph of text the offset falls in, as a range: blank
// lines separate paragraphs, as they separate blocks.
func paragraphAt(text string, offset int) (start, end int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	start = strings.LastIndex(text[:offset], "\n\n")
	if start < 0 {
		start = 0
	} else {
		start += 2
	}
	end = strings.Index(text[offset:], "\n\n")
	if end < 0 {
		return start, len(text)
	}
	return start, offset + end
}

// paragraphOffset is where a paragraph starts in the document's text, counted
// from one as the panel counts; -1 when the note has fewer paragraphs.
func paragraphOffset(text string, paragraph int) int {
	if paragraph < 1 {
		return -1
	}
	at := 0
	for i, part := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		if i+1 == paragraph {
			return at
		}
		at += len(part) + 2
	}
	return -1
}
