package browse_test

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/browse"
	"github.com/transmit-bug/remit/locales"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// fake is the app browse talks to: the rows it derived and the actions the
// screen asked for, with no store behind it.
type fake struct {
	rows    []browse.Row
	reloads int
	opened  []core.ID
	relinks []core.ID
}

func (f *fake) deps() browse.Deps {
	return browse.Deps{
		Rows:   func() []browse.Row { return f.rows },
		Reload: func() { f.reloads++ },
		Open:   func(id core.ID) { f.opened = append(f.opened, id) },
		Relink: func(id core.ID) { f.relinks = append(f.relinks, id) },
	}
}

// listed is what the app derived: a live card, a suspended one, a lost one and
// a frozen one, so each row's face is on screen at once.
var listed = []browse.Row{
	{ID: "card-1", Prompt: "Spacing beats cramming", Note: "Memory and recall", State: core.LinkLive},
	{ID: "card-2", Prompt: "A good card asks", Note: "Writing cards", State: core.LinkLive, Suspended: true},
	{ID: "card-3", State: core.LinkLost},
	{ID: "card-4", Prompt: "A block divides at", State: core.LinkFrozen},
}

// at draws the screen at a size, with nothing chosen and the given rows.
func at(t *testing.T, f *fake, w, h int) (*browse.State, *ui.Tester) {
	t.Helper()
	s := &browse.State{Selected: -1}
	d := f.deps()
	tt := ui.NewTester(func(c *ui.Context) { browse.View(c, s, d) }, w, h)
	tt.Frame()
	return s, tt
}

// The screen lists the rows the app derived at both the sizes the window opens
// at: the card's prompt, its note, and the state of its reference.
func TestRows(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{rows: listed}
	want := []string{
		msgs.BrowseTitle(), msgs.BrowseSearch(), msgs.BrowseOpen(),
		msgs.BrowseAll(), msgs.BrowseDue(), msgs.BrowseNew(), msgs.BrowseSuspended(), msgs.Lost(),
		"Spacing beats cramming", "Memory and recall", "A good card asks", "A block divides at",
		msgs.CardCount(4), msgs.LostBody(), msgs.Relink(), msgs.Frozen(),
	}
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			_, tt := at(t, f, size.w, size.h)
			for _, text := range want {
				if !tt.HasText(text) {
					t.Fatalf("missing %q; texts %q", text, tt.Texts())
				}
			}
			if tt.HasText(msgs.BrowseEmpty()) {
				t.Errorf("a list with rows says it is empty")
			}
			if subject, ok := tt.Find(msgs.BrowseOpen()); !ok || subject == (ui.Rect{}) {
				t.Errorf("the open button is not drawn")
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// A filter or a query is the app's to answer: the screen reports the change and
// holds what the user chose.
func TestFilterAndQueryAskForRows(t *testing.T) {
	f := &fake{rows: listed}
	s, tt := at(t, f, 1024, 720)
	msgs := locales.In("en")

	if err := tt.Click(msgs.Lost()); err != nil {
		t.Fatal(err)
	}
	if s.Filter != browse.Lost {
		t.Errorf("the filter is %v, want lost", s.Filter)
	}
	if f.reloads != 1 {
		t.Errorf("the screen asked for rows %d times, want 1", f.reloads)
	}

	tt.Frame()
	tt.Click(msgs.BrowseSearch())
	tt.Type("spacing")
	if s.Query != "spacing" {
		t.Fatalf("the search field holds %q, want %q", s.Query, "spacing")
	}
	if f.reloads != 2 {
		t.Errorf("typing asked for rows %d times, want 2", f.reloads)
	}
}

// With nothing to list the screen says so and leaves the open action alone.
func TestEmpty(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{}
	_, tt := at(t, f, 1024, 720)

	if !tt.HasText(msgs.BrowseEmpty()) {
		t.Fatalf("the empty state is missing %q; texts %q", msgs.BrowseEmpty(), tt.Texts())
	}
	if tt.HasText(msgs.LostBody()) {
		t.Error("the empty list shows a row")
	}
	if tt.HasText(msgs.CardCount(0)) != true {
		t.Errorf("the count does not say there are none; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.BrowseOpen()); err == nil {
		if len(f.opened) != 0 {
			t.Errorf("an empty list opened %v", f.opened)
		}
	}
}

// A row opens on Enter and on the Open button, and a lost row asks for the
// repair it offers.
func TestActions(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{rows: listed}
	s, tt := at(t, f, 1024, 720)

	if err := tt.Click("Spacing beats cramming"); err != nil {
		t.Fatal(err)
	}
	if s.Selected != 0 {
		t.Fatalf("the click chose %d, want 0", s.Selected)
	}
	tt.Key(0, ui.KeyEnter)
	if len(f.opened) != 1 || f.opened[0] != "card-1" {
		t.Fatalf("Enter opened %v, want card-1", f.opened)
	}

	if err := tt.Click("A block divides at"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(msgs.BrowseOpen()); err != nil {
		t.Fatal(err)
	}
	if len(f.opened) != 2 || f.opened[1] != "card-4" {
		t.Fatalf("Open opened %v, want card-4 second", f.opened)
	}

	if err := tt.Click(msgs.Relink()); err != nil {
		t.Fatal(err)
	}
	if len(f.relinks) != 1 || f.relinks[0] != "card-3" {
		t.Fatalf("Re-link asked for %v, want card-3", f.relinks)
	}
}

// Choosing keeps the keys inside the rows a reload leaves.
func TestKeep(t *testing.T) {
	s := &browse.State{Selected: 3}
	s.Keep(2)
	if s.Selected != 1 {
		t.Errorf("the choice is %d after two rows, want 1", s.Selected)
	}
	s.Keep(0)
	if s.Selected != -1 {
		t.Errorf("the choice is %d with no rows, want -1", s.Selected)
	}
}
