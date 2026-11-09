package kit

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// BoardNode is one place a board lists: the block it stands for, and what the
// page says about it. The strings are already in the window's language — a
// board prints what it is given.
type BoardNode struct {
	// Key is the node's identity in the page, which is what the highlight and
	// the focus stay on across a frame.
	Key string
	// Title names the note the node points at; Text is the block's first line.
	Title, Text string
	// Count is how many cards the block feeds, as a sentence.
	Count string
	// Hint says the node's block is no longer in its note, "" when it is. The
	// node stays: a page's board may name a block a hand edit has moved on.
	Hint string
}

// BoardOptions is one board as the page drew it: its nodes in the page's order,
// the columns the page asked for, and the node the window was told to point at.
type BoardOptions struct {
	Nodes []BoardNode
	// Columns is the grid's count; a window narrower than the page's layout
	// gives one, so a board never scrolls sideways.
	Columns int
	// Highlight is the node the window was told to locate, "" for none, and Run
	// is how often that moved: a new run replays the pointer once.
	Highlight string
	Run       uint64
	// Open runs the node the user clicked.
	Open func(i int)
}

// Board is a read-only grid of the places a page lists: each node a card that
// names its note, the block's first line and how many cards the block feeds,
// and each one a button that goes there.
//
// Composed rather than taken from a widget: ui.NewCanvas or a painter could
// draw a free canvas, but the nodes would lose the keyboard, focus, assistive
// technology and the 720×520 reflow that this grid gets from ui.Grid and
// ui.ButtonBase; ui.GridView scrolls its own viewport, while a page's board
// lives inside the page's scroll. Checked against MyGo v0.8.1 (spec §6.4, §4.14).
func Board(c *ui.Context, o BoardOptions) ui.Element {
	columns := o.Columns
	if columns < 1 {
		columns = 1
	}
	return ui.Grid(c).FillWidth().Columns(columns).
		GapX(design.SpaceL).GapY(design.SpaceL).Children(func() {
		for i, node := range o.Nodes {
			boardNode(c, o, node, i)
		}
	})
}

// boardNode is one node of a board: a card face that is one button, so the
// pointer, the tab key and a screen reader reach the same place.
func boardNode(c *ui.Context, o BoardOptions, node BoardNode, i int) {
	t := c.Theme()
	p := design.PaletteOf(t.Dark)
	// A node the page can be sent to is one button; a board nobody can open is
	// cards, so a read-only board has no control that does nothing.
	// A node takes the track it is given: a button's own row measures its whole
	// content otherwise, and pushes the grid to one column. The node's identity
	// is its place in the page's data — a keyed grid child, in MyGo v0.8.1, is
	// placed in a row of its own, which a board cannot have.
	// Exactly one element per node: a body built and then replaced would leave
	// the unused one in the grid as a cell of its own.
	var body ui.Element
	if o.Open != nil {
		body = ui.ButtonBase(c).FillWidth().MinWidth(0).
			Justify(ui.Start).AlignItems(ui.Start)
	} else {
		body = ui.Column(c).FillWidth().MinWidth(0)
	}
	face := StyleCard(c, body).Padding(design.SpaceL).Gap(design.SpaceXS)
	// What the round trip is for: the node the window was sent to is pointed at
	// once, and the run replays it when the same node is asked for again.
	if o.Highlight != "" && node.Key == o.Highlight {
		hit := face.Keyframes("board-hit", design.ModeFrames, ui.KeyframeOptions{Run: o.Run})
		face.Border(design.Hairline, t.Accent.Alpha(0.25+0.75*hit))
	}
	face.Label(node.Title + ": " + node.Text)
	if o.Open != nil {
		at := i
		face.OnClick(func() { o.Open(at) })
	}
	face.Children(func() {
		// Wrapped and clamped rather than kept on one line: a long block narrows
		// its node's min-content width, so a two-column board stays two columns.
		ui.Text(c, node.Title).FontSize(design.TypeUI).Bold().MaxLines(1)
		ui.Text(c, node.Text).FontSize(design.TypeUI).TextColor(t.TextMuted).MaxLines(2)
		if node.Count == "" && node.Hint == "" {
			return
		}
		ui.Row(c).AlignItems(ui.Center).Gap(design.SpaceXS).Children(func() {
			if node.Count != "" {
				ui.Text(c, node.Count).FontSize(design.TypeLabel).TextColor(t.TextMuted)
			}
			if node.Hint != "" {
				ui.Text(c, node.Hint).FontSize(design.TypeLabel).TextColor(p.Faint)
			}
		})
	})
}
