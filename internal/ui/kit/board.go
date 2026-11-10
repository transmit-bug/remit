package kit

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// BoardNode is one place a board lists: the block it stands for, what the page
// says about it, and where the page put it. The strings are already in the
// window's language — a board prints what it is given.
type BoardNode struct {
	// ID is the node's identity in the page, which is what an edge names.
	ID string
	// Block is the block the node stands for: what the window is told to locate,
	// and what a press opens. It is not unique — a page may point two nodes at
	// one paragraph — so it is not what an edge names.
	Block string
	// Title names the note the node points at; Text is the block's first line.
	Title, Text string
	// Count is how many cards the block feeds, as a sentence.
	Count string
	// Hint says the node's block is no longer in its note, "" when it is. The
	// node stays: a page's board may name a block a hand edit has moved on.
	Hint string
	// X, Y and W are where the page placed the node, in the board's own space.
	X, Y, W float32
}

// BoardOptions is one board as the page drew it: its places in the page's order,
// the connections between them, the space they were placed in, and the node the
// window was told to point at.
type BoardOptions struct {
	Nodes []BoardNode
	// Edges are the connections the page drew, as {from, to} keys of Nodes, in
	// the page's order.
	Edges [][2]string
	// Width and Height are the coordinate space the page placed its nodes in.
	// The host scales that space into the room it has rather than trusting a
	// pixel: a board names where a place is, not how wide the window is.
	Width, Height float32
	// Label names the board for assistive technology, and heads the list the
	// smallest window draws instead of the graph.
	Label string
	// Highlight is the block whose node the window was told to locate, "" for
	// none, and Run is how often that moved: a new run replays the pointer once.
	Highlight string
	Run       uint64
	// OpenLabel is the node's action in this frame's language: where pressing a
	// node goes.
	OpenLabel string
	// Open runs the node the user pressed.
	Open func(i int)
}

// Board is a read-only graph of the places a page lists: each node a card that
// names its note, the block's first line and how many cards the block feeds, its
// edges painted between them, and each node a button that goes to its block.
//
// The nodes are real controls — ui.ButtonBase, in the page's order, so the tab
// key, the focus ring and a screen reader reach every place — and only the edges
// are painted, in the canvas's Draw, which is the one thing no widget carries.
// A canvas of nodes (ui.NewCanvas, or a Draw that paints the cards too) would
// have to rebuild focus, keyboard activation and assistive technology inside a
// painter, which is what mygo-components forbids; ui.GridView places cells
// rather than nodes, and ui.List has no coordinates. Checked against MyGo
// v0.8.1 (ui-redesign-v2 §6.4).
func Board(c *ui.Context, o BoardOptions) ui.Element {
	// At the smallest window the graph becomes the list: see boardList.
	if w, _ := c.Size(); float32(w) <= design.BreakpointSmall {
		return boardList(c, o)
	}
	return boardGraph(c, o)
}

// boardGraph is the board on its canvas: the page's space, its nodes where it
// put them, and its edges under them.
//
// A node's place is read sideways as a share of the canvas, and its height as
// the page wrote it: the room a window loses is taken out of the gap between the
// columns, so a card keeps its size and its line, and a narrow window puts two
// cards closer together rather than on top of each other.
func boardGraph(c *ui.Context, o BoardOptions) ui.Element {
	t := c.Theme()
	p := design.PaletteOf(t.Dark)
	tk := design.NewTokens(t)
	// The canvas is the page's own space: a face one step under the page, so a
	// card stands on it, and as tall as the page said. Its edges are painted
	// before the nodes are built, which puts them under the cards.
	canvas := ui.Box(c).FillWidth().Height(o.Height).
		Background(p.Sunken).Radius(design.RadiusCard).
		Border(design.Hairline, t.Border)
	if o.Width > 0 && len(o.Edges) > 0 {
		ink := p.BorderStrong
		canvas = canvas.Draw(func(p *ui.Painter, r ui.Rect) { boardEdges(p, r, o, ink) })
	}
	// The pointer is replayed when the window is told to locate a node, not
	// every frame: an element told to come into view each pass would keep the
	// reader from scrolling it away.
	pointed := ui.Local(canvas, "board-run", func() uint64 { return 0 })
	canvas.Children(func() {
		for i, node := range o.Nodes {
			boardCard(c, tk, o, node, i, pointed)
		}
	})
	return canvas
}

// boardCard is one node of the graph: a card that is one button, placed where
// the page put it.
func boardCard(c *ui.Context, tk design.Tokens, o BoardOptions, node BoardNode, i int, pointed *uint64) {
	face := StyleCard(c, ui.ButtonBase(c).Justify(ui.Start).AlignItems(ui.Start)).
		Padding(tk.SpaceM).Gap(tk.SpaceXS).MinHeight(design.BoardNodeHeight).Absolute()
	// A node is placed by its share of the canvas rather than by a pixel, so the
	// board scales with the room it is given instead of measuring it.
	if o.Width > 0 {
		face.LeftPercent(node.X / o.Width * 100).WidthPercent(node.W / o.Width * 100)
	}
	face.Top(node.Y)
	face.Label(node.Title + ": " + node.Text)
	if o.Open != nil {
		at := i
		face.OnClick(func() { o.Open(at) })
	}
	boardHighlight(face, tk, node, o, pointed)
	face.Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).AlignItems(ui.Start).
			Gap(tk.SpaceXS).Children(func() { boardNodeBody(c, tk, o, node) })
	})
}

// boardHighlight points at the node the window was sent to, once per trip: the
// border runs the mode switch's own highlight, and the node comes into view when
// the guard admits the trip — a board's lowest node is often below the fold.
func boardHighlight(face ui.Element, tk design.Tokens, node BoardNode, o BoardOptions, pointed *uint64) {
	if o.Highlight == "" || node.Block != o.Highlight {
		return
	}
	hit := face.Keyframes("board-hit", design.ModeFrames, ui.KeyframeOptions{Run: o.Run})
	face.Border(design.Hairline, tk.Accent.Alpha(0.25+0.75*hit))
	if pointed != nil && *pointed != o.Run {
		face.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollCenter})
	}
}

// boardNodeBody is what a node says, on the graph and in the list: the note it
// belongs to, the block's first line, what the block feeds, and where pressing
// it goes.
func boardNodeBody(c *ui.Context, tk design.Tokens, o BoardOptions, node BoardNode) {
	// Wrapped and clamped rather than kept on one line: a long block narrows its
	// node's min-content width, and a node has the width the page gave it.
	ui.Text(c, node.Title).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).MaxLines(1)
	ui.Text(c, node.Text).FontSize(tk.TypeUI).MaxLines(2)
	if node.Count == "" && node.Hint == "" && o.OpenLabel == "" {
		return
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(tk.SpaceXS).Children(func() {
		switch {
		case node.Hint != "":
			ui.Text(c, node.Hint).FontSize(tk.TypeCaption).TextColor(tk.TextFaint)
		case node.Count != "":
			ui.Badge(c, node.Count)
		}
		if o.OpenLabel != "" {
			ui.Text(c, o.OpenLabel).FontSize(tk.TypeCaption).TextColor(tk.Accent).SingleLine()
		}
	})
}

// boardList is the board at the smallest window: the same places in the page's
// order, one row each, with the same action, and no sideways scroll.
//
// The edges are not drawn here. A line between two rows of a list would cross the
// words it connects and say nothing the order had not already said: what the
// graph shows is which place leads to which, and a list has room for the places
// alone, so the connections stay on the graph and the smallest window keeps every
// place reachable.
func boardList(c *ui.Context, o BoardOptions) ui.Element {
	tk := design.NewTokens(c.Theme())
	list := ui.List(c, nil, len(o.Nodes)).Label(o.Label)
	// The pointer is replayed once per trip, as on the graph: the row the window
	// was sent to comes into view rather than staying below the fold.
	pointed := ui.Local(list, "board-run", func() uint64 { return 0 })
	list.Rows(func(ui.ListRow) {
		// Every row is one button, so the tab key, the focus ring and a screen
		// reader reach the place the row stands for.
		for i, node := range o.Nodes {
			face := ui.ButtonBase(c).FillWidth().Justify(ui.Start).AlignItems(ui.Start).
				Padding(tk.SpaceM, tk.SpaceS).
				BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border)
			face.Label(node.Title + ": " + node.Text)
			if o.Open != nil {
				at := i
				face.OnClick(func() { o.Open(at) })
			}
			boardHighlight(face, tk, node, o, pointed)
			face.Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Gap(tk.SpaceXS).Children(func() {
					boardNodeBody(c, tk, o, node)
				})
			})
		}
	})
	if pointed != nil {
		*pointed = o.Run
	}
	return list
}

// boardEdges paints one connection per edge the page drew: a cubic leaving the
// side of a node that faces the one it arrives at, its handles at the middle of
// the gap, so the line reads as going from a place to another. Both ends are the
// page's own coordinates scaled into the canvas.
//
// BoardNodeHeight is where a connection meets its card: the same token is the
// card's floor, so the line arrives at the card rather than beside it, and one
// whose words made it taller hides the last of the line behind its face.
func boardEdges(p *ui.Painter, r ui.Rect, o BoardOptions, ink ui.Color) {
	at := make(map[string]int, len(o.Nodes))
	for i, node := range o.Nodes {
		at[node.ID] = i
	}
	// The canvas reads the page's width sideways and its height as written, so
	// x is a share of the room and y is the page's own.
	x := func(v float32) float32 { return r.X + v/o.Width*r.W }
	for _, edge := range o.Edges {
		from, ok := at[edge[0]]
		if !ok {
			continue
		}
		to, ok := at[edge[1]]
		if !ok {
			continue
		}
		a, b := o.Nodes[from], o.Nodes[to]
		y1 := r.Y + a.Y + design.BoardNodeHeight/2
		y2 := r.Y + b.Y + design.BoardNodeHeight/2
		x1, x2 := x(a.X+a.W), x(b.X)
		if b.X < a.X {
			x1, x2 = x(a.X), x(b.X+b.W)
		}
		mid := (x1 + x2) / 2
		var path ui.Path
		path.MoveTo(x1, y1)
		path.CubeTo(mid, y1, mid, y2, x2, y2)
		p.StrokePath(&path, design.BoardEdge, ink)
	}
}
