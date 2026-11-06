package ui

import (
	"strings"

	"github.com/transmit-bug/remit/internal/core"
)

// documentText is the note as the editor holds it: its paragraphs separated by a
// blank line, each ending with the marker that carries its identity. The markers
// are the file's own convention (ADR 0005), so what the editor shows round-trips
// through Markdown without an identity table to keep.
func documentText(note core.Note) string {
	var out strings.Builder
	for i, block := range note.Blocks {
		if i > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(strings.TrimRight(block.Text, " \t"))
		if block.ID != "" {
			out.WriteString(" ")
			out.WriteString(core.BlockMarker(block.ID))
		}
	}
	return out.String()
}

// blocksOfDocument reads the paragraphs of an edited document back into blocks.
// A paragraph that ends with a marker keeps the identity it names; one without a
// marker is a paragraph the user just wrote, and gets a new one. Blank text is
// not a block. A marker whose kind is not a block's is left in the text rather
// than trusted.
func blocksOfDocument(text string, prev []core.Block) []core.Block {
	paragraphs := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	known := make(map[core.ID]bool, len(prev))
	for _, block := range prev {
		known[block.ID] = true
	}
	out := make([]core.Block, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		body := strings.TrimSpace(paragraph)
		if body == "" {
			continue
		}
		id, rest, ok := core.SplitBlockMarker(body)
		if ok && id.Kind() == core.BlockKind {
			body = strings.TrimSpace(rest)
		} else {
			id = ""
		}
		if body == "" {
			continue
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
