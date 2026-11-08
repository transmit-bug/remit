package relink_test

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/relink"
	"github.com/transmit-bug/remit/locales"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// fake is the app the picker talks to: the blocks it derived for a query, and
// the block the user chose, with no store behind them.
type fake struct {
	rows   []relink.Row
	asked  []string
	chosen []core.ID
}

// candidates narrows the blocks the way the app does: by the note's title and
// the block's text.
func (f *fake) candidates(query string) []relink.Row {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []relink.Row
	for _, row := range f.rows {
		if q == "" || strings.Contains(strings.ToLower(row.Note+" "+row.Text), q) {
			out = append(out, row)
		}
	}
	return out
}

func (f *fake) deps() relink.Deps {
	return relink.Deps{
		Candidates: func(query string) []relink.Row {
			f.asked = append(f.asked, query)
			return f.candidates(query)
		},
		Choose: func(id core.ID) { f.chosen = append(f.chosen, id) },
	}
}

// blocks is what the app derived: the blocks the notes hold.
var blocks = []relink.Row{
	{Block: "block-1", Text: "Spacing beats cramming", Note: "Memory and recall"},
	{Block: "block-2", Text: "Fresh card :: answer", Note: "Filters"},
	{Block: "block-3", Text: "A good card asks", Note: "Writing cards"},
}

// shown draws the picker for a lost card, opened from a handler as the app
// opens it, at a size and in a language.
func shown(t *testing.T, f *fake, w, h int, locale string) *ui.Tester {
	t.Helper()
	s := &relink.State{}
	d := f.deps()
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		if open {
			open = false
			s.Open(c, "card-3", f.candidates(""))
		}
		relink.View(c, s, d)
	}, w, h)
	if locale != "" {
		tt.SetLocale(locale)
	}
	tt.Frame()
	tt.Advance(300 * time.Millisecond) // the picker's arrival
	return tt
}

// at draws the picker at a size, in the window's language.
func at(t *testing.T, f *fake, w, h int) *ui.Tester { return shown(t, f, w, h, "") }

// The picker lists the blocks the app derived at both the sizes the window
// opens at: the text the card would show, and the note it lives in.
func TestCandidates(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{rows: blocks}
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := at(t, f, size.w, size.h)
			want := []string{msgs.RelinkTitle(), msgs.RelinkSearch(), msgs.Close()}
			for _, row := range blocks {
				want = append(want, row.Text, row.Note)
			}
			for _, text := range want {
				if !tt.HasText(text) {
					t.Fatalf("missing %q; texts %q", text, tt.Texts())
				}
			}
			if tt.HasText(msgs.NoBlocks()) {
				t.Errorf("a picker with candidates says there are none")
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// A query is the app's to answer: the picker asks for the blocks it finds and
// draws what comes back, down to the empty state.
func TestQuery(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{rows: blocks}
	tt := at(t, f, 1024, 720)

	tt.Type("fresh")
	if len(f.asked) != 1 || f.asked[0] != "fresh" {
		t.Fatalf("the picker asked for %v, want one query for fresh", f.asked)
	}
	if !tt.HasText("Fresh card :: answer") {
		t.Fatalf("the matching block is gone; texts %q", tt.Texts())
	}
	for _, gone := range []string{"Spacing beats cramming", "A good card asks"} {
		if tt.HasText(gone) {
			t.Errorf("the query kept %q", gone)
		}
	}

	tt.Type("zzz")
	if !tt.HasText(msgs.NoBlocks()) {
		t.Fatalf("the empty state is missing %q; texts %q", msgs.NoBlocks(), tt.Texts())
	}
}

// A click chooses the block, and so does Enter on the row the keys reached.
func TestChoose(t *testing.T) {
	f := &fake{rows: blocks}
	tt := at(t, f, 1024, 720)

	if err := tt.Click("A good card asks"); err != nil {
		t.Fatal(err)
	}
	if len(f.chosen) != 1 || f.chosen[0] != "block-3" {
		t.Fatalf("the click chose %v, want block-3", f.chosen)
	}

	f = &fake{rows: blocks}
	tt = at(t, f, 1024, 720)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if len(f.chosen) != 1 || f.chosen[0] != "block-2" {
		t.Fatalf("Enter chose %v, want block-2", f.chosen)
	}

	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyEnter)
	if len(f.chosen) != 2 || f.chosen[1] != "block-1" {
		t.Fatalf("the keys chose %v, want block-1 second", f.chosen)
	}
}

// The close button and Escape both put the picker away, and choosing nothing
// writes nothing.
func TestClose(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{rows: blocks}
	tt := at(t, f, 1024, 720)

	if err := tt.Click(msgs.Close()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(msgs.RelinkTitle()) {
		t.Fatalf("the close button left the picker up; texts %q", tt.Texts())
	}

	tt = at(t, f, 1024, 720)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText(msgs.RelinkTitle()) {
		t.Fatalf("Escape left the picker up; texts %q", tt.Texts())
	}
	if len(f.chosen) != 0 {
		t.Errorf("closing chose %v", f.chosen)
	}
}

// An empty picker says so and Enter on it writes nothing.
func TestEmpty(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{}
	tt := at(t, f, 1024, 720)

	if !tt.HasText(msgs.NoBlocks()) {
		t.Fatalf("the empty state is missing %q; texts %q", msgs.NoBlocks(), tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	if len(f.chosen) != 0 {
		t.Errorf("an empty picker chose %v", f.chosen)
	}
}
