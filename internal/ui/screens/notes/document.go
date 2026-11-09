package notes

import (
	"strings"
	"unicode/utf8"

	"github.com/transmit-bug/remit/internal/core"
)

// DocumentText is the note as the editor holds it: its paragraphs, separated by
// a blank line, without the markers that carry their identity.
//
// The markers live in the file, not in the editor (ADR 0005's "the editor hides
// it"). They are kept out of the text the user types in because the editor's
// caret can come to rest inside one — at the end of a paragraph, or a few
// characters before the end — and then keystrokes split the id and the paragraph
// loses the card that pointed at it. Identity is re-attached on save, by place.
func DocumentText(note core.Note) string {
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
		out = append(out, core.Block{Text: body})
	}
	for at, id := range blockIDs(prev, out) {
		out[at].ID = id
	}
	for at := range out {
		id := out[at].ID
		if id == "" {
			id = core.NewID(core.BlockKind)
		}
		out[at].ID = id
	}
	return out
}

// blockIDs carries block identities from prev to the blocks an edit made.
// Unchanged blocks are anchors wherever they moved by insertion; the blocks
// between two anchors are paired in order, so typing inside one keeps its ID
// and the blocks after an insertion keep theirs. A block with no partner is
// new.
func blockIDs(prev, now []core.Block) []core.ID {
	ids := make([]core.ID, len(now))
	if len(prev) == 0 || len(now) == 0 {
		return ids
	}
	if len(prev) == len(now) {
		for at := range prev {
			ids[at] = prev[at].ID
		}
		return ids
	}
	if len(prev)*len(now) > 4_000_000 {
		for at := 0; at < len(prev) && at < len(now); at++ {
			ids[at] = prev[at].ID
		}
		return ids
	}

	// lcs[i][j] is the longest common run of block texts in prev[i:] and
	// now[j:].
	lcs := make([][]int, len(prev)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(now)+1)
	}
	for i := len(prev) - 1; i >= 0; i-- {
		for j := len(now) - 1; j >= 0; j-- {
			if prev[i].Text == now[j].Text {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}

	type anchor struct{ prev, now int }
	var anchors []anchor
	for i, j := 0, 0; i < len(prev) && j < len(now); {
		switch {
		case prev[i].Text == now[j].Text:
			anchors = append(anchors, anchor{i, j})
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			i++
		default:
			j++
		}
	}

	oldAt, newAt := 0, 0
	for _, a := range anchors {
		carryBlockIDs(ids, newAt, prev[oldAt:a.prev], now[newAt:a.now])
		ids[a.now] = prev[a.prev].ID
		oldAt, newAt = a.prev+1, a.now+1
	}
	carryBlockIDs(ids, newAt, prev[oldAt:], now[newAt:])
	return ids
}

// carryBlockIDs pairs the blocks of a gap between two unchanged anchors in
// their order, up to the shorter side.
func carryBlockIDs(ids []core.ID, at int, prev, now []core.Block) {
	for i := 0; i < len(prev) && i < len(now); i++ {
		ids[at+i] = prev[i].ID
	}
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

// ParagraphOffset is where a paragraph starts in the document's text, counted
// from one as the panel counts; -1 when the note has fewer paragraphs.
func ParagraphOffset(text string, paragraph int) int {
	if paragraph < 1 {
		return -1
	}
	at := 0
	for i, part := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		if i+1 == paragraph {
			return at
		}
		at += utf8.RuneCountInString(part) + 2
	}
	return -1
}
