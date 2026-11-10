package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// The rail is the frame's fixed column: the whole width while the window has
// room for its labels, the icon strip at the smallest window the app opens at.
// A rail that folded at the wrong width would take the stage's room without
// saying so.
func TestTheRailKeepsItsWidth(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)

			nav, ok := tt.Find(msgs.AreaNav())
			if !ok {
				t.Fatalf("the rail has no box; texts %q", tt.Texts())
			}
			want := design.RailWidth
			if size.w <= int(design.BreakpointNarrow) {
				want = design.RailCollapsed
			}
			// The sidebar insets its rows inside the rail's own column.
			if inset := nav.W - want; inset < -design.SpaceM || inset > 0 {
				t.Errorf("the rail is %v wide, want about %v", nav.W, want)
			}
			if nav.X != 0 {
				t.Errorf("the rail starts at %v, not at the window's edge", nav.X)
			}
		})
	}
}

// The paper keeps its measure however wide the window gets: a document is read
// in a column of its own, not spread across the stage, and its room beside the
// text is the design's gutter.
func TestThePaperKeepsItsMeasure(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1920, 1080)
	settle(tt)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")

	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the paper is missing; texts %q", tt.Texts())
	}
	if want := design.PaperWidth - 2*design.Gutter; body.W > want+1 || body.W < want-1 {
		t.Errorf("the paper's text is %v wide, want the measure's %v", body.W, want)
	}
	title, ok := tt.Find(msgs.NoteTitle())
	if !ok {
		t.Fatalf("the note's title is missing; texts %q", tt.Texts())
	}
	if title.X != body.X {
		t.Errorf("the title starts at %v and the text at %v: the paper's edge is not shared", title.X, body.X)
	}
}

// The status line is the window's last row, under the stage, at the window's
// own height: a document says where its words went there, not inside the paper.
func TestTheStatusLineIsTheWindowsLastRow(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			notesMode(t, a, tt)
			openNote(t, tt, "Memory and recall")

			line, ok := tt.Find(msgs.StatusLine())
			if !ok {
				t.Fatalf("the document has no status line; texts %q", tt.Texts())
			}
			if want := design.StatusbarHeight; line.H > want+1 {
				t.Errorf("the status line is %v tall, want at most %v", line.H, want)
			}
			if bottom := float32(size.h); line.Y+line.H > bottom+1 || line.Y+line.H < bottom-1 {
				t.Errorf("the status line ends at %v, want the window's %v", line.Y+line.H, bottom)
			}
			if _, ok := tt.Find(msgs.Saved()); !ok {
				t.Errorf("the saved state is not in the status line; texts %q", tt.Texts())
			}
		})
	}
}

// The line belongs to the document: every other area leaves it to the frame,
// where it is not drawn at all rather than drawn empty.
func TestOtherAreasShowNoStatusLine(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	for _, area := range []areaID{areaToday, areaRead, areaRecall, areaStats, areaSettings} {
		a.setArea(area, 0)
		settle(tt)
		if _, ok := tt.Find(msgs.StatusLine()); ok {
			t.Errorf("%v draws the document's status line", area)
		}
	}
}

// The card table is read in the room it has: a window too narrow for every
// column folds the secondary ones away rather than cutting the prompt to a
// word, and a wide window shows them again.
func TestTheCardTableFoldsItsColumns(t *testing.T) {
	msgs := locales.In("en")
	at := func(t *testing.T, w, h int) *ui.Tester {
		t.Helper()
		a, _ := seedRecallLibrary(t)
		tt := ui.NewTester(a.View, w, h)
		settle(tt)
		recallSubPage(t, tt, "en", msgs.CardsPage())
		tt.Advance(320 * time.Millisecond)
		return tt
	}

	wide := at(t, 1440, 900)
	for _, want := range []string{msgs.RecallColumnPrompt(), msgs.RecallColumnNote(), msgs.RecallColumnDue()} {
		if !wide.HasText(want) {
			t.Errorf("the wide table does not show %q; texts %q", want, wide.Texts())
		}
	}
	narrow := at(t, 720, 520)
	for _, want := range []string{msgs.RecallColumnPrompt(), msgs.RecallColumnDue()} {
		if !narrow.HasText(want) {
			t.Errorf("the narrow table does not show %q; texts %q", want, narrow.Texts())
		}
	}
	if narrow.HasText(msgs.RecallColumnNote()) {
		t.Errorf("the narrow table still shows %q, over a prompt cut to a word", msgs.RecallColumnNote())
	}
}

// A library with no review yet opens its history on the empty state: the page
// draws, with the sentence and the line that says what fills it.
func TestTheHistoryOfAnEmptyLibraryDraws(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{Now: time.Now})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	lib.Close()

	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	a.setArea(areaRecall, subRecallHistory)
	settle(tt)

	for _, want := range []string{msgs.RecallHistoryEmpty(), msgs.RecallHistoryEmptyBody()} {
		if !tt.HasText(want) {
			t.Errorf("the empty history does not draw %q; texts %q", want, tt.Texts())
		}
	}
}
