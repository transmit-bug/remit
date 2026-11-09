package kit_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// listRows is a list of n rows, each arriving behind the one above it.
func listRows(c *ui.Context, n int) {
	col := ui.Column(c).Gap(design.SpaceS)
	arrive := kit.Stagger(col, "rows")
	col.Children(func() {
		for i := range n {
			kit.Arrive(ui.Text(c, "row"), arrive(i)).FontSize(design.TypeUI)
		}
	})
}

// A list arrives behind itself and then rests: the rows ask for frames until
// the last of them is in place, and the frame after that asks for none.
func TestStaggerArrivesAndRests(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) { listRows(c, 8) }, 480, 320)
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("the rows arrived without moving")
	}
	// The last staggered row starts five steps in and takes Row's 180 ms.
	tt.Advance(design.Row.Spring.Duration + design.MaxStaggerRows*design.RowStagger)
	if tt.FramePending() {
		t.Fatal("the rows never came to rest")
	}
	// The list is drawn from the same state: a rebuild replays nothing.
	tt.Frame()
	if tt.FramePending() {
		t.Error("a rebuild plays the arrival again")
	}
}

// The arrival moves the rows' faces, not the list: what stands under the list
// is where the layout put it before the first row arrives and after the last
// has, and the rows keep their places in it.
func TestStaggerLeavesTheLayout(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Gap(design.SpaceS).Children(func() {
			listRows(c, 8)
			ui.Text(c, "under the list")
		})
	}, 480, 320)
	tt.Frame()
	start, ok := tt.Find("under the list")
	if !ok {
		t.Fatalf("the list has nothing under it; texts %q", tt.Texts())
	}
	tt.Advance(design.Row.Spring.Duration + design.MaxStaggerRows*design.RowStagger)
	end, ok := tt.Find("under the list")
	if !ok {
		t.Fatalf("what stands under the list went away; texts %q", tt.Texts())
	}
	if start != end {
		t.Errorf("the arrival moved what stands under the list from %+v to %+v", start, end)
	}
}

// A bar fills to its value once, when it mounts: a rebuild and a new value do
// not fill it again.
func TestProgressFillsOnce(t *testing.T) {
	value := 0.5
	view := func(c *ui.Context) {
		kit.Progress(c, kit.ProgressOptions{Value: value, Label: "half"})
	}
	tt := ui.NewTester(view, 320, 120)
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("the bar filled without growing")
	}
	tt.Advance(design.Progress.Duration + 20*time.Millisecond)
	if tt.FramePending() {
		t.Fatal("the bar never came to rest")
	}
	// A value that changes after the mount is drawn at once, and a rebuild
	// replays nothing.
	value = 0.8
	tt.Frame()
	if tt.FramePending() {
		t.Error("the bar fills again on a new value")
	}
}

// The moments move only what they were asked to: with the desktop asking for no
// motion, a list, a bar and a panel are where they end at once.
func TestMotionIsInstantWhenMotionIsReduced(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		listRows(c, 8)
		kit.Progress(c, kit.ProgressOptions{Value: 0.5, Label: "half"})
		kit.ArrivePanel(ui.Box(c).Size(120, 80).Background(c.Theme().Surface), 0)
		kit.Heatmap(c, heatWeeks(), 3, kit.HeatmapOptions{Cell: 12, Gap: design.HeatGap})
	}, 480, 320)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if tt.FramePending() {
		t.Error("the view still travels with reduced motion asked for")
	}
}

// A panel arrives over the page and rests: it asks for frames while it comes
// in, and a rebuild does not bring it in again.
func TestArrivePanelSettles(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		kit.ArrivePanel(ui.Box(c).Size(200, 120).Background(c.Theme().Surface), 0)
	}, 320, 240)
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("the panel arrived without moving")
	}
	tt.Advance(design.Panel.Spring.Duration + 20*time.Millisecond)
	if tt.FramePending() {
		t.Fatal("the panel never came to rest")
	}
	tt.Frame()
	if tt.FramePending() {
		t.Error("a rebuild brings the panel in again")
	}
}

// A face eases its colour as the pointer finds it, and asks for no frame to do
// it once the colour has arrived.
func TestHoverFaceRests(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		face := kit.HoverFace(c, ui.Box(c).Size(120, 40).Radius(design.RadiusControl))
		face.Label("face")
		if face.Hovered() {
			face.Background(c.Theme().SurfaceHover)
		}
		face.Background(c.Theme().Surface).Background(c.Theme().Surface)
	}, 200, 160)
	tt.Frame()
	box, ok := tt.Find("face")
	if !ok {
		t.Fatalf("no face; texts %q", tt.Texts())
	}
	tt.Move(box.X+box.W/2, box.Y+box.H/2)
	tt.Advance(design.Hover.Duration + 20*time.Millisecond)
	if tt.FramePending() {
		t.Error("the face still eases after its preset ran")
	}
}

// A row built after the arrival has read the list's playhead, not one of its
// own: a list that scrolls during the stagger shows what it scrolled to where
// the rest of the list is, and starts nothing again.
func TestStaggerCarriesRowsBuiltLater(t *testing.T) {
	rows := 2
	tt := ui.NewTester(func(c *ui.Context) {
		col := ui.Column(c)
		arrive := kit.Stagger(col, "rows")
		col.Children(func() {
			for i := range rows {
				kit.Arrive(ui.Text(c, "row"), arrive(i)).FontSize(design.TypeUI)
			}
		})
	}, 320, 240)
	tt.Frame()
	tt.Advance(design.Row.Spring.Duration + design.MaxStaggerRows*design.RowStagger + 20*time.Millisecond)
	// Rows eight and nine come into view once the list has arrived.
	rows = 9
	tt.Frame()
	if tt.FramePending() {
		t.Error("a row built after the arrival started the stagger again")
	}
}
