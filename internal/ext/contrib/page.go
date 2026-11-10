package contrib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Kind is what one node of a page is: a layout node the host arranges, or a
// semantic leaf it renders with a widget of its own. A kind this host does not
// know keeps its name and its place, so the page shows where the newer plugin's
// node would be rather than losing it.
type Kind string

// The layout nodes: structure only, never pixels.
const (
	KindPage    Kind = "page"
	KindSection Kind = "section"
	KindStack   Kind = "stack"
	KindGrid    Kind = "grid"
	KindSplit   Kind = "split"
	KindTabs    Kind = "tabs"
	KindCard    Kind = "card"
	// KindBoard is the one node that names a coordinate: a connected graph of
	// places in the library, whose nodes the host places and whose edges the host
	// paints (ui-redesign-v2 §6.4). It names geometry, never a pixel: no colour,
	// no size of text, no style.
	KindBoard Kind = "board"
)

// The semantic leaves: what a page says, in the host's widgets.
const (
	KindMarkdown Kind = "markdown"
	KindText     Kind = "text"
	KindStat     Kind = "stat"
	KindTable    Kind = "table"
	KindList     Kind = "list"
	KindTree     Kind = "tree"
	KindForm     Kind = "form"
	KindChart    Kind = "chart"
	KindCallout  Kind = "callout"
	KindActions  Kind = "actions"
)

// Kinds are every node a page may name, in the order the contract lists them:
// the layout nodes, then the semantic leaves. This is the one list the host
// validates against and the one remit.d.ts is generated from.
var Kinds = []Kind{
	KindPage, KindSection, KindStack, KindGrid, KindSplit, KindTabs, KindCard,
	KindBoard,
	KindMarkdown, KindText, KindStat, KindTable, KindList, KindTree, KindForm,
	KindChart, KindCallout, KindActions,
}

// Gap is the only spacing a plugin names: a step of the host's scale, never a
// number of its own.
type Gap string

// The host's spacing steps.
const (
	GapXS Gap = "xs"
	GapS  Gap = "s"
	GapM  Gap = "m"
	GapL  Gap = "l"
)

// Gaps are every spacing step a page may name.
var Gaps = []Gap{GapXS, GapS, GapM, GapL}

// Tone is the only colour a plugin names: the meaning, which the theme answers.
type Tone string

// The theme's tones.
const (
	ToneNeutral Tone = "neutral"
	ToneInfo    Tone = "info"
	ToneWarn    Tone = "warn"
	ToneDanger  Tone = "danger"
	ToneSuccess Tone = "success"
)

// Tones are every tone a page may name.
var Tones = []Tone{ToneNeutral, ToneInfo, ToneWarn, ToneDanger, ToneSuccess}

// Direction is a stack's order.
type Direction string

// A stack lays its children out in a column (the default) or a row, which the
// host wraps when the window is too narrow for one line.
const (
	DirectionColumn Direction = "column"
	DirectionRow    Direction = "row"
)

// Directions are every order a stack may take.
var Directions = []Direction{DirectionColumn, DirectionRow}

// The page contract's limits. A page over any of them is a page error — the
// host says so and offers a retry — rather than a frame that pays for it.
const (
	MaxDepth      = 8
	MaxNodes      = 500
	MaxTableRows  = 500
	MaxPageBytes  = 256 << 10
	maxGridColumn = 4
	// MaxBoardNodes and MaxBoardEdges bound one board. A board is drawn as one
	// node of the tree whatever it holds, so it counts against the page's node
	// budget on its own: these are what keeps a graph a picture rather than a
	// canvas the host must reason about.
	MaxBoardNodes = 200
	MaxBoardEdges = 200
	// MaxBoardSize is the largest coordinate space a board may claim, in DIP.
	MaxBoardSize = 4000
)

// Node is one element of a page tree: the layout node or the leaf the plugin
// named, with the tokens it may name and nothing to style with.
type Node struct {
	Kind Kind
	// Source is the kind as the plugin wrote it; it differs from Kind only for
	// a kind this host does not know, which draws a placeholder naming it.
	Source string

	Title       string
	Description string
	Children    []Node

	Direction Direction
	Gap       Gap
	Tone      Tone
	// Columns is a grid's hint, 1 to 4; the host collapses it to one when the
	// window is narrow.
	Columns int
	// Ratio is a split's share: 1 for equal panes, 2 for one twice the other.
	Ratio int
	Tabs  []Tab

	Label string
	Value string
	Text  string
	// Headings and Rows are a table's: its column titles and its cells.
	Headings []string
	Rows     [][]string
	// Items are a list's, a tree's or an actions' rows.
	Items []Item
	// Fields are a form's rows: a label and the value beside it.
	Fields []Item
	// Points are a chart's values, one bar each.
	Points []Point
	// Width and Height are a board's coordinate space: the DIP rectangle the
	// page drew its nodes in. The host scales it to the room the window has.
	Width, Height float32
	// Nodes are a board's places, and Edges the connections between them by node
	// id: {from, to}, in the page's order.
	Nodes []BoardNode
	Edges [][2]string
}

// BoardNode is one place a board names: the block it stands for and where the
// page put it. X, Y and W are in the board's own space; the host turns them
// into the room it has, so a node keeps its place without the page knowing a
// pixel. Title, Text and Cards are what the page says the block is, which the
// host replaces with what the library holds now when it can.
//
// A board's node entries are the one place a page's data may carry a field this
// host does not know: the host reads the fields below and ignores the rest, so
// a newer plugin's extra datum does not take the page down. Nothing there is a
// style — the host draws the node with its own widget either way.
type BoardNode struct {
	ID          string
	Note, Block string
	X, Y, W     float32
	Title, Text string
	Cards       int
}

// Tab is one page of a tabs node: its label and the content it shows.
type Tab struct {
	Label    string
	Children []Node
}

// Item is one row of a list, a tree or a form, or one action of an actions
// node: a label, a value beside it, children below it, or the command a button
// runs.
type Item struct {
	Label   string
	Value   string
	Command string
	Items   []Item
}

// Point is one bar of a chart: what it is called, and how much.
type Point struct {
	Label string
	Value float64
}

// ParsePage reads a page data command's answer. The whole-page problems — the
// bytes over the budget, a node over a limit, a structural mistake, a key that
// would style rather than describe — are an error the page shows in place of a
// tree. An unknown node or leaf is not one: it stays in the tree, and the host
// draws a placeholder where it was.
func ParsePage(raw []byte) (Node, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Node{}, fmt.Errorf("page: the data command answered nothing")
	}
	if len(raw) > MaxPageBytes {
		return Node{}, fmt.Errorf("page: %d bytes, the contract allows %d", len(raw), MaxPageBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var w wire
	if err := dec.Decode(&w); err != nil {
		return Node{}, fmt.Errorf("page: %w", err)
	}
	root, err := w.node("page")
	if err != nil {
		return Node{}, err
	}
	if err := Validate(root); err != nil {
		return Node{}, err
	}
	return root, nil
}

// Validate walks a parsed page and reports the first structural problem, by its
// path in the tree: a depth or count over the contract, a split that is not two
// panes, a section or a tab without a label, a gap or tone the host has no
// token for.
func Validate(root Node) error {
	v := validator{}
	if err := v.walk(root, "page", 1); err != nil {
		return err
	}
	return nil
}

type validator struct{ nodes int }

func (v *validator) walk(n Node, path string, depth int) error {
	if depth > MaxDepth {
		return fmt.Errorf("%s: %d deep, the contract allows %d", path, depth, MaxDepth)
	}
	v.nodes++
	if v.nodes > MaxNodes {
		return fmt.Errorf("%s: more than %d nodes", path, MaxNodes)
	}
	if !knownKind(n.Kind) {
		// The host does not know it, so it has nothing to check; knowing the
		// children is enough to draw the placeholder around them.
		for i, child := range n.Children {
			if err := v.walk(child, fmt.Sprintf("%s.children[%d]", path, i), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if n.Gap != "" && !knownGap(n.Gap) {
		return fmt.Errorf("%s: gap %q, the host has xs, s, m and l", path, n.Gap)
	}
	if n.Tone != "" && !knownTone(n.Tone) {
		return fmt.Errorf("%s: tone %q, the host has neutral, info, warn, danger and success", path, n.Tone)
	}
	switch n.Kind {
	case KindSection:
		if strings.TrimSpace(n.Title) == "" {
			return fmt.Errorf("%s: a section needs a title", path)
		}
	case KindStack:
		if n.Direction != "" && n.Direction != DirectionRow && n.Direction != DirectionColumn {
			return fmt.Errorf("%s: direction %q, the host has row and column", path, n.Direction)
		}
	case KindGrid:
		if n.Columns < 1 || n.Columns > maxGridColumn {
			return fmt.Errorf("%s: columns %d, the host lays out 1 to %d", path, n.Columns, maxGridColumn)
		}
	case KindSplit:
		if len(n.Children) != 2 {
			return fmt.Errorf("%s: a split is two panes, not %d", path, len(n.Children))
		}
		if n.Ratio != 1 && n.Ratio != 2 {
			return fmt.Errorf("%s: ratio %d, a split is 1 or 2", path, n.Ratio)
		}
	case KindTabs:
		if len(n.Tabs) == 0 {
			return fmt.Errorf("%s: tabs has no tab", path)
		}
		for i, tab := range n.Tabs {
			if strings.TrimSpace(tab.Label) == "" {
				return fmt.Errorf("%s.tabs[%d]: a tab needs a label", path, i)
			}
		}
	case KindTable:
		if len(n.Headings) == 0 {
			return fmt.Errorf("%s: a table needs column headings", path)
		}
		if len(n.Rows) > MaxTableRows {
			return fmt.Errorf("%s: %d rows, the contract allows %d", path, len(n.Rows), MaxTableRows)
		}
		for i, row := range n.Rows {
			if len(row) != len(n.Headings) {
				return fmt.Errorf("%s.rows[%d]: %d cells for %d columns", path, i, len(row), len(n.Headings))
			}
		}
	case KindStat:
		if strings.TrimSpace(n.Label) == "" {
			return fmt.Errorf("%s: a stat needs a label", path)
		}
	case KindActions:
		for i, item := range n.Items {
			if strings.TrimSpace(item.Command) == "" {
				return fmt.Errorf("%s.items[%d]: an action needs a command", path, i)
			}
			if strings.TrimSpace(item.Label) == "" {
				return fmt.Errorf("%s.items[%d]: an action needs a label", path, i)
			}
		}
	case KindBoard:
		if err := validateBoard(n, path); err != nil {
			return err
		}
	}
	for i, child := range n.Children {
		if err := v.walk(child, fmt.Sprintf("%s.children[%d]", path, i), depth+1); err != nil {
			return err
		}
	}
	for i, tab := range n.Tabs {
		for j, child := range tab.Children {
			if err := v.walk(child, fmt.Sprintf("%s.tabs[%d].children[%d]", path, i, j), depth+1); err != nil {
				return err
			}
		}
	}
	for i, item := range n.Items {
		if err := v.walkItems(item.Items, fmt.Sprintf("%s.items[%d]", path, i), depth+1); err != nil {
			return err
		}
	}
	return nil
}

// walkItems checks the children of a tree item, which are rows rather than
// nodes: they add depth, and they cost the node budget, but they lay nothing
// out.
func (v *validator) walkItems(items []Item, path string, depth int) error {
	if len(items) == 0 {
		return nil
	}
	if depth > MaxDepth {
		return fmt.Errorf("%s: %d deep, the contract allows %d", path, depth, MaxDepth)
	}
	for i, item := range items {
		v.nodes++
		if v.nodes > MaxNodes {
			return fmt.Errorf("%s: more than %d nodes", path, MaxNodes)
		}
		if err := v.walkItems(item.Items, fmt.Sprintf("%s.items[%d]", path, i), depth+1); err != nil {
			return err
		}
	}
	return nil
}

// validateBoard checks one board: its own space, then each place and each
// connection, by its index: a space layout can use, nodes and edges inside the
// contract's count, an id no other node takes, a place inside the board, and an
// edge whose two ends are nodes the board names.
func validateBoard(n Node, path string) error {
	if n.Width <= 0 || n.Height <= 0 {
		return fmt.Errorf("%s: a board needs a width and a height, not %gx%g", path, n.Width, n.Height)
	}
	if n.Width > MaxBoardSize || n.Height > MaxBoardSize {
		return fmt.Errorf("%s: %gx%g, a board is at most %d DIP across", path, n.Width, n.Height, MaxBoardSize)
	}
	if len(n.Nodes) > MaxBoardNodes {
		return fmt.Errorf("%s: %d nodes, the contract allows %d", path, len(n.Nodes), MaxBoardNodes)
	}
	if len(n.Edges) > MaxBoardEdges {
		return fmt.Errorf("%s: %d edges, the contract allows %d", path, len(n.Edges), MaxBoardEdges)
	}
	ids := make(map[string]bool, len(n.Nodes))
	for i, node := range n.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("%s.nodes[%d]: a node needs an id", path, i)
		}
		if ids[node.ID] {
			return fmt.Errorf("%s.nodes[%d]: id %q is already a node of this board", path, i, node.ID)
		}
		ids[node.ID] = true
		if strings.TrimSpace(node.Block) == "" {
			return fmt.Errorf("%s.nodes[%d]: a node needs the block it stands for", path, i)
		}
		if node.X < 0 || node.Y < 0 {
			return fmt.Errorf("%s.nodes[%d]: %.0f,%.0f is off the board", path, i, node.X, node.Y)
		}
		if node.W <= 0 {
			return fmt.Errorf("%s.nodes[%d]: width %g, a node is wider than nothing", path, i, node.W)
		}
		if node.X+node.W > n.Width || node.Y > n.Height {
			return fmt.Errorf("%s.nodes[%d]: %.0f,%.0f %.0f wide is outside %gx%g",
				path, i, node.X, node.Y, node.W, n.Width, n.Height)
		}
	}
	for i, edge := range n.Edges {
		from, to := edge[0], edge[1]
		if !ids[from] || !ids[to] {
			return fmt.Errorf("%s.edges[%d]: %q to %q is not two nodes of this board", path, i, from, to)
		}
	}
	return nil
}

func knownKind(k Kind) bool {
	for _, known := range Kinds {
		if known == k {
			return true
		}
	}
	return false
}

func knownGap(g Gap) bool {
	for _, known := range Gaps {
		if known == g {
			return true
		}
	}
	return false
}

func knownTone(t Tone) bool {
	for _, known := range Tones {
		if known == t {
			return true
		}
	}
	return false
}

// wire is the JSON a page node arrives as. It is one struct for every kind so
// that a decoder with unknown fields refused answers the contract's ban on
// width, height, colour, font, margin, padding and position with the field's
// own name: a plugin cannot style what the host draws.
type wire struct {
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Children    []wire          `json:"children"`
	Direction   string          `json:"direction"`
	Gap         string          `json:"gap"`
	Tone        string          `json:"tone"`
	Columns     json.RawMessage `json:"columns"`
	Ratio       int             `json:"ratio"`
	Tabs        []tabWire       `json:"tabs"`
	Label       string          `json:"label"`
	Value       string          `json:"value"`
	Text        string          `json:"text"`
	Rows        [][]string      `json:"rows"`
	Items       []itemWire      `json:"items"`
	Fields      []itemWire      `json:"fields"`
	Points      []pointWire     `json:"points"`
	// The board's geometry, kept raw until the kind is known: a board is the one
	// node that may name a width, a height or a position, and the same field on
	// any other node is the styling the contract bans.
	Width  json.RawMessage `json:"width"`
	Height json.RawMessage `json:"height"`
	Nodes  json.RawMessage `json:"nodes"`
	Edges  json.RawMessage `json:"edges"`
}

type tabWire struct {
	Label    string `json:"label"`
	Children []wire `json:"children"`
}

type itemWire struct {
	Label   string     `json:"label"`
	Value   string     `json:"value"`
	Command string     `json:"command"`
	Items   []itemWire `json:"items"`
}

type pointWire struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// node turns one wire node into the tree, reading columns as the kind means it:
// a grid's count or a table's headings.
func (w wire) node(path string) (Node, error) {
	n := Node{
		Kind:        Kind(w.Kind),
		Source:      w.Kind,
		Title:       w.Title,
		Description: w.Description,
		Direction:   Direction(w.Direction),
		Gap:         Gap(w.Gap),
		Tone:        Tone(w.Tone),
		Ratio:       w.Ratio,
		Label:       w.Label,
		Value:       w.Value,
		Text:        w.Text,
		Rows:        w.Rows,
	}
	if n.Kind == "" {
		return Node{}, fmt.Errorf("%s: a node needs a kind", path)
	}
	if !knownKind(n.Kind) {
		// Unknown: keep the name and whatever children the host can still draw.
		n.Direction, n.Gap, n.Tone = "", "", ""
	}
	if n.Kind != KindBoard {
		// The board's geometry is the only coordinate a page may name, and a
		// node of any other kind naming one is the styling the contract bans.
		for _, field := range []struct {
			name string
			raw  json.RawMessage
		}{{"width", w.Width}, {"height", w.Height}, {"nodes", w.Nodes}} {
			if len(field.raw) > 0 {
				return Node{}, fmt.Errorf("%s: json: unknown field %q", path, field.name)
			}
		}
		if len(w.Edges) > 0 {
			return Node{}, fmt.Errorf("%s: json: unknown field %q", path, "edges")
		}
	}
	for i, child := range w.Children {
		node, err := child.node(fmt.Sprintf("%s.children[%d]", path, i))
		if err != nil {
			return Node{}, err
		}
		n.Children = append(n.Children, node)
	}
	for i, tab := range w.Tabs {
		out := Tab{Label: tab.Label}
		for j, child := range tab.Children {
			node, err := child.node(fmt.Sprintf("%s.tabs[%d].children[%d]", path, i, j))
			if err != nil {
				return Node{}, err
			}
			out.Children = append(out.Children, node)
		}
		n.Tabs = append(n.Tabs, out)
	}
	if len(w.Columns) > 0 {
		switch n.Kind {
		case KindTable:
			if err := json.Unmarshal(w.Columns, &n.Headings); err != nil {
				return Node{}, fmt.Errorf("%s: columns: %w", path, err)
			}
		default:
			if err := json.Unmarshal(w.Columns, &n.Columns); err != nil {
				return Node{}, fmt.Errorf("%s: columns: %w", path, err)
			}
		}
	}
	n.Items = items(w.Items)
	n.Fields = items(w.Fields)
	for _, point := range w.Points {
		n.Points = append(n.Points, Point{Label: point.Label, Value: point.Value})
	}
	if n.Kind == KindBoard {
		if err := n.readBoard(w, path); err != nil {
			return Node{}, err
		}
	}
	return n, nil
}

// readBoard reads a board's own space, its places and its connections.
func (n *Node) readBoard(w wire, path string) error {
	for _, field := range []struct {
		name string
		raw  json.RawMessage
		dst  *float32
	}{{"width", w.Width, &n.Width}, {"height", w.Height, &n.Height}} {
		if len(field.raw) == 0 {
			continue
		}
		value, err := finite(field.raw)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", path, field.name, err)
		}
		*field.dst = value
	}
	if len(w.Edges) > 0 {
		if err := json.Unmarshal(w.Edges, &n.Edges); err != nil {
			return fmt.Errorf("%s: edges: %w", path, err)
		}
	}
	if len(w.Nodes) == 0 {
		return nil
	}
	// A board's node entries are data, not page structure: unknown fields are
	// ignored here so a newer plugin's extra datum still draws.
	var nodes []boardWire
	if err := json.Unmarshal(w.Nodes, &nodes); err != nil {
		return fmt.Errorf("%s: nodes: %w", path, err)
	}
	for i, node := range nodes {
		x, err := usable(node.X)
		if err != nil {
			return fmt.Errorf("%s.nodes[%d]: x: %w", path, i, err)
		}
		y, err := usable(node.Y)
		if err != nil {
			return fmt.Errorf("%s.nodes[%d]: y: %w", path, i, err)
		}
		width, err := usable(node.W)
		if err != nil {
			return fmt.Errorf("%s.nodes[%d]: w: %w", path, i, err)
		}
		n.Nodes = append(n.Nodes, BoardNode{
			ID: node.ID, Note: node.Note, Block: node.Block,
			X: x, Y: y, W: width,
			Title: node.Title, Text: node.Text, Cards: node.Cards,
		})
	}
	return nil
}

// boardWire is one node of a board as the page wrote it.
type boardWire struct {
	ID    string  `json:"id"`
	Note  string  `json:"note"`
	Block string  `json:"block"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	Title string  `json:"title"`
	Text  string  `json:"text"`
	Cards int     `json:"cards"`
}

// finite is one coordinate as the host can use it: a JSON number that is
// neither NaN nor an infinity, and one float32 holds, so that a page cannot put
// a place where layout has no answer.
func finite(raw json.RawMessage) (float32, error) {
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, err
	}
	return usable(number)
}

// usable is one coordinate a page wrote, as the host can use it.
func usable(number float64) (float32, error) {
	value := float32(number)
	if math.IsNaN(number) || math.IsInf(number, 0) || math.IsInf(float64(value), 0) {
		return 0, fmt.Errorf("%v is past what a coordinate holds", number)
	}
	return value, nil
}

func items(in []itemWire) []Item {
	if len(in) == 0 {
		return nil
	}
	out := make([]Item, 0, len(in))
	for _, item := range in {
		out = append(out, Item{Label: item.Label, Value: item.Value, Command: item.Command, Items: items(item.Items)})
	}
	return out
}
