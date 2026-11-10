package kit_test

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// boardNodes are the places a test's board holds: two on one row and one below,
// where the page's own space puts them.
func boardNodes() []kit.BoardNode {
	return []kit.BoardNode{
		{ID: "n1", Block: "b1", Title: "Memory", Text: "spacing", Count: "2 cards", X: 60, Y: 60, W: 250},
		{ID: "n2", Block: "b2", Title: "Sleep", Text: "rest", Count: "1 card", X: 560, Y: 60, W: 250},
		{ID: "n3", Block: "b3", Title: "Recall", Text: "testing", X: 60, Y: 400, W: 250},
	}
}

// boardOptions is the board a test draws: three places in a space of their own,
// two lines between them, and where a node goes.
func boardOptions(open *int) kit.BoardOptions {
	return kit.BoardOptions{
		Nodes:     boardNodes(),
		Edges:     [][2]string{{"n1", "n2"}, {"n1", "n3"}},
		Width:     1120,
		Height:    700,
		Label:     "Places",
		OpenLabel: "Back to this paragraph",
		Open:      func(i int) { *open = i },
	}
}

// boardView draws one board in the page's scroll, as a plugin page hands it over.
func boardView(o kit.BoardOptions) func(*ui.Context) {
	return func(c *ui.Context) {
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(design.SpaceXL).Children(func() {
				kit.Board(c, o)
			})
		})
	}
}

// boardShot draws a board once and returns the tester and its frame.
func boardShot(t *testing.T, o kit.BoardOptions, w, h int) (*ui.Tester, *image.RGBA) {
	t.Helper()
	tt := ui.NewTester(boardView(o), w, h)
	settleBoard(tt)
	return tt, tt.Image()
}

// settleBoard draws the first frame and lets its animations finish: the entrance
// and the highlight a located node runs once.
func settleBoard(tt *ui.Tester) {
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
}

// inked reports whether two frames differ anywhere in the pixels within radius
// of a point: a stroke one and a half DIPs wide lands on a pixel's edge as often
// as on its middle, and a line reaches a card's edge from just outside it.
func inked(a, b *image.RGBA, x, y float32, radius int) bool {
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			px, py := int(x)+dx, int(y)+dy
			if a.RGBAAt(px, py) != b.RGBAAt(px, py) {
				return true
			}
		}
	}
	return false
}

// A board places its nodes where the page's space put them, and paints one line
// per connection between them.
func TestBoardPlacesItsNodesWhereThePagePutThem(t *testing.T) {
	opened := -1
	tt, _ := boardShot(t, boardOptions(&opened), 1280, 800)

	memory, ok := tt.Find("Memory: spacing")
	if !ok {
		t.Fatalf("the first node has no box; texts %q", tt.Texts())
	}
	sleep, ok := tt.Find("Sleep: rest")
	if !ok {
		t.Fatalf("the second node has no box; texts %q", tt.Texts())
	}
	recall, ok := tt.Find("Recall: testing")
	if !ok {
		t.Fatalf("the third node has no box; texts %q", tt.Texts())
	}
	// Where the page put them: beside each other on the board's first row, and
	// the third under the first.
	if sleep.Y != memory.Y || sleep.X <= memory.X {
		t.Errorf("the two nodes of the first row are at %.1f,%.1f and %.1f,%.1f",
			memory.X, memory.Y, sleep.X, sleep.Y)
	}
	if recall.Y <= memory.Y || recall.X != memory.X {
		t.Errorf("the third node is at %.1f,%.1f, not under the first at %.1f,%.1f",
			recall.X, recall.Y, memory.X, memory.Y)
	}
	for _, box := range []ui.Rect{memory, sleep, recall} {
		if box.W <= 0 || box.H <= 0 || box.X+box.W > 1280 || box.Y+box.H > 800 {
			t.Errorf("a node's box is %.1f,%.1f %.1fx%.1f in 1280x800", box.X, box.Y, box.W, box.H)
		}
	}
	// A node is one control: it takes a press and says which place it stands for.
	if err := tt.Click("Sleep: rest"); err != nil {
		t.Fatalf("the node: %v", err)
	}
	if opened != 1 {
		t.Errorf("the node pressed opened place %d, want 1", opened)
	}
}

// Every connection the page draws is painted, and nothing where it draws none:
// the same board without its edges differs at the two lines' ends and nowhere
// else.
func TestBoardPaintsOneLinePerConnection(t *testing.T) {
	opened := -1
	o := boardOptions(&opened)
	tt, withEdges := boardShot(t, o, 1280, 800)
	memory, _ := tt.Find("Memory: spacing")
	recall, _ := tt.Find("Recall: testing")

	o.Edges = nil
	_, withoutEdges := boardShot(t, o, 1280, 800)

	anchor := design.BoardNodeHeight / 2
	// A line leaves the first node to the right, and the second leaves the first
	// node's right edge too and arrives at the third.
	if !inked(withEdges, withoutEdges, memory.X+memory.W+3, memory.Y+anchor, 1) {
		t.Error("the line leaving the first node was not painted")
	}
	if !inked(withEdges, withoutEdges, recall.X-3, recall.Y+anchor, 3) {
		t.Error("the line arriving at the third node was not painted")
	}
	// The third node's right side has no line, and neither has the space under the
	// second: the page drew two lines, not a lattice.
	if inked(withEdges, withoutEdges, recall.X+recall.W+3, recall.Y+anchor, 1) {
		t.Error("a line was painted where the page drew none")
	}
}

// The nodes are controls in the page's order: the tab key walks them as the data
// lists them, and the one it lands on is the one Enter opens.
func TestBoardNodesAreFocusableInDataOrder(t *testing.T) {
	opened := -1
	tt, _ := boardShot(t, boardOptions(&opened), 1280, 800)

	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if !tt.Focused("Memory: spacing") {
		t.Fatalf("the tab key did not reach the first node; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if !tt.Focused("Sleep: rest") {
		t.Errorf("the second node is not next in the tab order; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if opened != 1 {
		t.Errorf("enter opened place %d, want the node it was on", opened)
	}
}

// At the smallest window the board is a list of the same places, one row each,
// with the same action: a graph has no room at 720 DIPs, and a list asks for no
// sideways scroll.
func TestBoardAtTheSmallestWindowIsAList(t *testing.T) {
	opened := -1
	tt, _ := boardShot(t, boardOptions(&opened), 720, 520)

	var last float32
	for _, want := range []string{"Memory: spacing", "Sleep: rest", "Recall: testing"} {
		box, ok := tt.Find(want)
		if !ok {
			t.Fatalf("the list is missing %q; texts %q", want, tt.Texts())
		}
		if box.W <= 0 || box.H <= 0 || box.X+box.W > 720 || box.Y+box.H > 520 {
			t.Errorf("%q has the box %.1f,%.1f %.1fx%.1f in 720x520", want, box.X, box.Y, box.W, box.H)
		}
		if box.Y <= last {
			t.Errorf("%q is not under the row above it: %.1f after %.1f", want, box.Y, last)
		}
		last = box.Y
	}
	// The rows are controls there too: the tab key walks them in the page's
	// order, and Enter opens the one it lands on.
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if !tt.Focused("Sleep: rest") {
		t.Fatalf("the tab key does not reach the list's rows; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if opened != 1 {
		t.Errorf("enter opened place %d, want the row it was on", opened)
	}
	if err := tt.Click("Recall: testing"); err != nil {
		t.Fatalf("the third row: %v", err)
	}
	if opened != 2 {
		t.Errorf("the row pressed opened place %d, want 2", opened)
	}
}

// The node the window was told to locate is brought into view, which is what a
// trip back to the board is for: its lowest node is often below the fold.
func TestBoardBringsTheLocatedNodeIntoView(t *testing.T) {
	opened := -1
	o := boardOptions(&opened)
	o.Highlight = "b3"

	// The board as it opens: the third node is past the bottom of the window.
	tt, _ := boardShot(t, o, 1280, 400)
	if !tt.HasText("Recall: testing") {
		t.Fatalf("the board lost the node it was told to point at; texts %q", tt.Texts())
	}
	before, _ := tt.Find("Recall: testing")
	if before.Y < 400 {
		t.Fatalf("the third node is already in view at %.1f: the test proves nothing", before.Y)
	}

	o.Run = 1
	trip, _ := boardShot(t, o, 1280, 400)
	after, _ := trip.Find("Recall: testing")
	if after.Y < 0 || after.Y+after.H > 400 {
		t.Errorf("the located node is at %.1f..%.1f, still outside 0..400", after.Y, after.Y+after.H)
	}
	if trip.FramePending() {
		t.Error("the board keeps asking for frames after the pointer settled")
	}
}
