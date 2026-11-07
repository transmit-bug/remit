package core

import (
	"strings"
)

// Excerpt is the block an excerpt makes: the text taken from a source block, and
// the reference back to the block it came from. Both live in one block because a
// block is the unit of reference (ADR 0002/0005): the reference travels with the
// passage, the derived index reads it as a link from the child note to the
// source, and a card made from the excerpt keeps the way back.
//
// The passage is not decorated: an excerpt that quotes itself would make cards
// whose prompt reads "> a question", and a card is what an excerpt is often for.
// A block may not hold a blank line — that is where one block ends in the file —
// so the reference sits on its own line under the passage.
func Excerpt(source Note, block Block, text string) Block {
	passage := strings.TrimSpace(text)
	var out strings.Builder
	out.WriteString(passage)
	out.WriteString("\n")
	out.WriteString(LinkOpen)
	out.WriteString(referenceTitle(source.Title))
	out.WriteString(LinkAnchor)
	out.WriteString(BlockMarker(block.ID))
	out.WriteString(LinkClose)
	return Block{Text: out.String()}
}

// referenceTitle is a source note's title as a link can carry it: a title that
// holds the brackets or the anchor would end the link early, so they are dropped
// rather than written broken.
func referenceTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.ReplaceAll(title, LinkClose, "")
	title = strings.ReplaceAll(title, LinkOpen, "")
	title = strings.ReplaceAll(title, LinkAnchor, "")
	if title == "" {
		return "note"
	}
	return title
}
