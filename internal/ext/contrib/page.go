package contrib

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	return n, nil
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
