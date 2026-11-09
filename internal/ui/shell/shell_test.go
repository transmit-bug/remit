package shell_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/shell"
	"github.com/transmit-bug/remit/locales"
)

// frame is a window with the shell around an empty stage.
func frame(st *shell.State, d shell.Deps) func(*ui.Context) {
	return func(c *ui.Context) {
		ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
			shell.View(c, st, d, func() {})
		})
	}
}

// areas are the six rail items with the counts of the stores they lead to.
func areas() []shell.Item {
	return []shell.Item{
		{ID: shell.Today, Label: "Today", Icon: "inbox", Count: 6},
		{ID: shell.Write, Label: "Write", Icon: "notebook-pen", Count: 3},
		{ID: shell.Read, Label: "Read", Icon: "book-open"},
		{ID: shell.Recall, Label: "Recall", Icon: "layers", Count: 4},
		{ID: shell.Stats, Label: "Stats", Icon: "chart-column"},
		{ID: shell.Settings, Label: "Settings", Icon: "settings"},
	}
}

// A status line with nothing to say is not drawn: the stage gets the row back
// rather than an empty band. With a line, the segments are drawn with a rule
// between them.
func TestStatusLineOnlyWhenItSaysSomething(t *testing.T) {
	st := &shell.State{}
	quiet := ui.NewTester(frame(st, shell.Deps{Title: "Remit", Areas: areas(), Current: shell.Today}), 1024, 720)
	quiet.Frame()
	msgs := locales.In("en")
	if _, ok := quiet.Find(msgs.StatusLine()); ok {
		t.Fatalf("an empty status line is drawn; texts %q", quiet.Texts())
	}

	loud := ui.NewTester(frame(st, shell.Deps{
		Title: "Remit", Areas: areas(), Current: shell.Today,
		Status: []string{"1,204 words", "14 paragraphs", "Saved"},
	}), 1024, 720)
	loud.Frame()
	info, ok := loud.Access(msgs.StatusLine())
	if !ok {
		t.Fatalf("the status line is missing; texts %q", loud.Texts())
	}
	if info.Bounds.H != design.StatusbarHeight {
		t.Errorf("the status line is %v high, want %v", info.Bounds.H, design.StatusbarHeight)
	}
	for _, part := range []string{"1,204 words", "14 paragraphs", "Saved"} {
		if !loud.HasText(part) {
			t.Errorf("the status line lost %q; texts %q", part, loud.Texts())
		}
	}
}

// The trail is the path to where the user is: the last segment is the page, and
// choosing an earlier one goes back to that area.
func TestBreadcrumbGoesBackToAnEarlierSegment(t *testing.T) {
	st := &shell.State{}
	var back []int
	d := shell.Deps{
		Title: "Remit", Areas: areas(), Current: shell.Recall,
		Crumbs:  []string{"Remit", "Recall area", "Cards"},
		OnCrumb: func(i int) { back = append(back, i) },
	}
	tt := ui.NewTester(frame(st, d), 1024, 720)
	tt.Frame()

	// The last segment is where the user is: it is no link. The one before it
	// goes back to that area.
	if info, ok := tt.Access("Cards"); !ok || info.Role == ui.RoleLink {
		t.Errorf("the current page is a link: %+v", info)
	}
	if info, ok := tt.Access("Recall area"); !ok || info.Role != ui.RoleLink {
		t.Fatalf("the area is not a link: %+v", info)
	}
	if err := tt.Click("Recall area"); err != nil {
		t.Fatalf("choosing the area: %v", err)
	}
	if len(back) != 1 || back[0] != 1 {
		t.Fatalf("choosing the area reported %v, want 1", back)
	}
}

// An area without pages of its own has no sub-navigation: Today gets the row's
// height back instead of an empty one.
func TestSubnavHiddenForAnAreaWithoutPages(t *testing.T) {
	st := &shell.State{}
	none := ui.NewTester(frame(st, shell.Deps{Title: "Remit", Areas: areas(), Current: shell.Today}), 1024, 720)
	none.Frame()
	msgs := locales.In("en")
	if _, ok := none.Find(msgs.Subnav()); ok {
		t.Fatalf("an empty sub-navigation is drawn; texts %q", none.Texts())
	}

	// With pages, the row is there, the chosen one is marked, and a click
	// reports the page it names.
	var chosen []string
	d := shell.Deps{
		Title: "Remit", Areas: areas(), Current: shell.Recall,
		Subnav: []shell.Sub{
			{ID: "queue", Label: "Queue", Count: 4},
			{ID: "cards", Label: "Cards", Count: 12},
		},
		CurrentSub: "queue",
		OnSub:      func(id string) { chosen = append(chosen, id) },
	}
	tt := ui.NewTester(frame(st, d), 1024, 720)
	tt.Frame()
	info, ok := tt.Access(msgs.Subnav())
	if !ok {
		t.Fatalf("the sub-navigation is missing; texts %q", tt.Texts())
	}
	if info.Bounds.H != design.SubnavHeight-1 {
		t.Errorf("the sub-navigation is %v high, want about %v", info.Bounds.H, design.SubnavHeight)
	}
	if err := tt.Click("Cards"); err != nil {
		t.Fatalf("the page tab is not clickable: %v", err)
	}
	if len(chosen) != 1 || chosen[0] != "cards" {
		t.Fatalf("choosing a page reported %v, want cards", chosen)
	}
}

// The rail chooses an area by pointer and by keyboard, and reports which: the
// shell never writes the app's area itself.
func TestRailChoosesAnArea(t *testing.T) {
	st := &shell.State{}
	var picked []shell.Area
	d := shell.Deps{
		Title: "Remit", Areas: areas(), Current: shell.Today,
		OnSelect: func(a shell.Area) { picked = append(picked, a) },
	}
	tt := ui.NewTester(frame(st, d), 1024, 720)
	tt.Frame()

	if err := tt.Click("Recall"); err != nil {
		t.Fatalf("the rail item is not clickable: %v", err)
	}
	if len(picked) != 1 || picked[0] != shell.Recall {
		t.Fatalf("the rail reported %v, want recall", picked)
	}

	// The sidebar is one tab stop, and the arrows choose within it.
	info, _ := tt.Access("Areas")
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if len(picked) < 2 {
		t.Fatalf("the arrow keys chose nothing: %v", picked)
	}
	if last := picked[len(picked)-1]; last == shell.Today {
		t.Errorf("the arrow key kept the same area")
	}
}

// Folding is the rail's own: the control and the top bar's switch both say what
// they will do, and the rail is the icon strip at BreakpointNarrow whatever the
// choice was.
func TestRailFoldsFromItsOwnControls(t *testing.T) {
	st := &shell.State{}
	msgs := locales.In("en")
	tt := ui.NewTester(frame(st, shell.Deps{Title: "Remit", Areas: areas(), Current: shell.Today}), 1100, 720)
	tt.Frame()

	// Open, the rail's foot says it will fold it.
	if !tt.HasText(msgs.RailFold()) {
		t.Fatalf("the rail offers no fold; texts %q", tt.Texts())
	}
	tt.Click(msgs.RailFold())
	tt.Frame()
	if !st.Folded {
		t.Fatal("the control did not fold the rail")
	}
	if !tt.HasText(msgs.RailUnfold()) {
		t.Fatalf("a folded rail offers no way back; texts %q", tt.Texts())
	}
	// Unfolding is the top bar's switch, which says the same.
	tt.ClickAt(0, 0) // let the pointer leave the rail's foot
	tt.Click(msgs.RailUnfold())
	tt.Frame()
	if st.Folded {
		t.Fatal("the bar's switch did not unfold the rail")
	}

	// Below the break the rail is folded whatever the choice says.
	tt.SetSize(int(design.BreakpointNarrow), 720)
	tt.Frame()
	info, ok := tt.Access("Areas")
	if !ok {
		t.Fatal("the rail is missing")
	}
	if info.Bounds.W > design.RailCollapsed {
		t.Errorf("the rail is %v wide at %v, want %v", info.Bounds.W, design.BreakpointNarrow, design.RailCollapsed)
	}
	tt.Advance(time.Second) // let anything the fold started settle
	if tt.FramePending() {
		t.Error("the frame still requests frames after it should settle")
	}
}

// A rail the user folded says so at its foot and in the bar: both controls name
// the way back, and either one takes it.
func TestFoldedRailOffersItsWayBack(t *testing.T) {
	st := &shell.State{Folded: true}
	msgs := locales.In("en")
	tt := ui.NewTester(frame(st, shell.Deps{Title: "Remit", Areas: areas(), Current: shell.Today}), 1100, 720)
	tt.Frame()

	if tt.HasText(msgs.RailFold()) {
		t.Fatalf("a folded rail still offers to fold; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.RailUnfold()) {
		t.Fatalf("a folded rail offers no way back; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.RailUnfold()); err != nil {
		t.Fatal(err)
	}
	if st.Folded {
		t.Fatal("the control did not unfold the rail")
	}
}
