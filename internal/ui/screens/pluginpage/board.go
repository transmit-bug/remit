package pluginpage

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// Target is one place a page's data puts on a board: the note and the block its
// action names. It is what the host reads of a node instead of trusting the
// words beside it, so a block whose text changed since the data was loaded
// still goes to the right place, and nothing the page says is a coordinate
// (ui-redesign-v2 §6.4).
type Target struct {
	Note  string
	Block string
}

// BoardTargets lists the places a page's data puts on a board, in the page's
// order: every card of a grid whose cards each name a block through an action.
// A grid that is not such a board has no targets, and draws as a grid.
func BoardTargets(tree contrib.Node) []Target {
	var out []Target
	walk(tree, func(n contrib.Node) {
		if n.Kind != contrib.KindGrid || !isBoard(n.Children) {
			return
		}
		for _, card := range n.Children {
			target, _ := boardTarget(card)
			out = append(out, target)
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

// isBoard says whether a grid's children are a board: each one a card that
// names a block through an action. A grid of anything else is a layout node.
func isBoard(children []contrib.Node) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if _, ok := boardTarget(child); !ok {
			return false
		}
	}
	return true
}

// boardTarget reads one card of a board: the place its action names, and the
// words the page put beside it. The value is "<note>/<block>", the two the
// action acts on, or a lone block id the host resolves through the library.
func boardTarget(card contrib.Node) (Target, bool) {
	if card.Kind != contrib.KindCard {
		return Target{}, false
	}
	target := Target{}
	found := false
	for _, child := range card.Children {
		if child.Kind != contrib.KindActions {
			continue
		}
		for _, item := range child.Items {
			note, block, ok := boardValue(item.Value)
			if !ok {
				continue
			}
			target, found = Target{Note: note, Block: block}, true
			break
		}
	}
	return target, found && target.Block != ""
}

// boardValue reads a board action's value: the note and the block it stands
// for, or the block alone.
func boardValue(value string) (note, block string, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}
	note, block, split := strings.Cut(value, "/")
	if !split {
		return "", value, true
	}
	if note == "" || block == "" {
		return "", "", false
	}
	return note, block, true
}

// boardText is what a board node said when the page's data was loaded: the
// card's text is the note's title, and its markdown is the block's first line.
// The host resolves both against the library it holds now and keeps these only
// for a block the note no longer has.
func boardText(card contrib.Node) (title, text string) {
	for _, child := range card.Children {
		switch child.Kind {
		case contrib.KindText:
			if title == "" {
				title = child.Text
			}
		case contrib.KindMarkdown:
			if text == "" {
				text = child.Text
			}
		}
	}
	if title == "" {
		title = card.Title
	}
	return title, text
}

// drawBoard draws a grid of board cards as the host's board: the page names the
// places, and the host draws them with its own widgets and goes there itself.
func drawBoard(c *ui.Context, s *State, d Deps, children []contrib.Node, columns int, tk design.Tokens) {
	msgs := locales.In(c.Locale())
	nodes := make([]kit.BoardNode, 0, len(children))
	for _, card := range children {
		target, _ := boardTarget(card)
		title, text := boardText(card)
		node := kit.BoardNode{Key: d.message(c, target.Block), Title: d.message(c, title), Text: d.message(c, text)}
		if d.Node != nil {
			// The host's answer is the truth: the page's data is as old as the
			// last load, and a note may have been edited since.
			if info, ok := d.Node(target.Note, target.Block); ok {
				node.Title, node.Text, node.Count = info.Title, info.Text, msgs.BoardCards(info.Cards)
			} else {
				node.Hint = msgs.BoardBlockGone()
			}
		}
		nodes = append(nodes, node)
	}
	var open func(int)
	if d.OpenBlock != nil {
		open = func(i int) {
			if i < 0 || i >= len(children) {
				return
			}
			target, ok := boardTarget(children[i])
			if !ok {
				return
			}
			d.OpenBlock(c, target.Note, target.Block)
		}
	}
	kit.Board(c, kit.BoardOptions{
		Nodes:     nodes,
		Columns:   columns,
		Highlight: s.Highlight,
		Run:       s.HighlightRun,
		Open:      open,
	})
}
