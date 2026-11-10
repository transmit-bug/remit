package pluginpage

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// Target is one place a page's data puts on a board: the note and the block its
// node names. It is what the host reads of a node instead of trusting the words
// beside it, so a block whose text changed since the data was loaded still goes
// to the right place, and nothing the page says is a coordinate the host
// believes (ui-redesign-v2 §6.4).
type Target struct {
	Note  string
	Block string
}

// BoardTargets lists the places a page's data puts on a board, in the page's
// order: every node of every board the tree holds. A page with no board has no
// targets, and the block menu offers no way back to it.
func BoardTargets(tree contrib.Node) []Target {
	var out []Target
	walk(tree, func(n contrib.Node) {
		if n.Kind != contrib.KindBoard {
			return
		}
		for _, node := range n.Nodes {
			out = append(out, Target{Note: node.Note, Block: node.Block})
		}
	})
	return out
}

// walk visits every node of a page tree once, depth first.
func walk(n contrib.Node, fn func(contrib.Node)) {
	fn(n)
	for _, child := range n.Children {
		walk(child, fn)
	}
	for _, tab := range n.Tabs {
		for _, child := range tab.Children {
			walk(child, fn)
		}
	}
}

// hasBoard reports whether a page's tree draws a board, which is what asks the
// page for the wide measure a graph needs and a paragraph does not.
func hasBoard(tree contrib.Node) bool {
	found := false
	walk(tree, func(n contrib.Node) {
		if n.Kind == contrib.KindBoard {
			found = true
		}
	})
	return found
}

// drawBoard draws the places a page's board names: the host places the nodes in
// the space the page drew, resolves each against the library it holds now, paints
// the connections between them, and goes to a block itself when a node is
// pressed (§6.4).
func drawBoard(c *ui.Context, s *State, d Deps, n contrib.Node, tk design.Tokens) {
	msgs := locales.In(c.Locale())
	nodes := make([]kit.BoardNode, 0, len(n.Nodes))
	for _, place := range n.Nodes {
		node := kit.BoardNode{
			ID:    place.ID,
			Block: place.Block,
			Title: d.message(c, place.Title),
			Text:  d.message(c, place.Text),
			X:     place.X, Y: place.Y, W: place.W,
		}
		if d.Node == nil {
			// No library to read: the page's own words stand, and the count it
			// stated is the only one there is.
			if place.Cards > 0 {
				node.Count = msgs.BoardCards(place.Cards)
			}
		} else if info, ok := d.Node(place.Note, place.Block); ok {
			// The host's answer is the truth: the page's data is as old as the
			// last load, and a note may have been edited since.
			node.Title, node.Text, node.Count = info.Title, info.Text, msgs.BoardCards(info.Cards)
		} else {
			node.Hint = msgs.BoardBlockGone()
		}
		nodes = append(nodes, node)
	}
	var open func(int)
	if d.OpenBlock != nil {
		open = func(i int) {
			if i < 0 || i >= len(n.Nodes) {
				return
			}
			d.OpenBlock(c, n.Nodes[i].Note, n.Nodes[i].Block)
		}
	}
	kit.Board(c, kit.BoardOptions{
		Nodes:     nodes,
		Edges:     n.Edges,
		Width:     n.Width,
		Height:    n.Height,
		Label:     d.message(c, n.Title),
		Highlight: s.Highlight,
		Run:       s.HighlightRun,
		OpenLabel: msgs.BoardOpen(),
		Open:      open,
	})
}
