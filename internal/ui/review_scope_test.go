package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/locales"
)

// scopeSeed gives the harness library a tagged note beside the seeded ones, so
// a scope has something to name that the rest of the library does not match.
func scopeSeed(t *testing.T, a *App) core.Note {
	t.Helper()
	if a.lib == nil {
		t.Fatal("no library open")
	}
	note, err := a.lib.SaveNote(core.Note{Title: "A chapter of its own", Blocks: []core.Block{
		{Text: "Chapter asks :: chapter answers #chapter"},
		{Text: "Second asks :: second answers #chapter"},
	}})
	if err != nil {
		t.Fatalf("save scoped note: %v", err)
	}
	a.reload()
	return note
}

// The scope picker sits beside the mode switch on the review side only, names
// the whole library until a scope is chosen, and lists the notes and tags the
// snapshot holds.
func TestReviewScopePickerSitsBesideTheSwitch(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			scopeSeed(t, a)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			tt.Key(ui.Cmd, ui.Key1)

			if err := tt.Click(msgs.ReviewScopeMenu()); err != nil {
				t.Fatalf("the scope menu did not open: %v; texts %q", err, tt.Texts())
			}
			if items := tt.Menu(); !contains(items, msgs.ReviewScope()) {
				t.Fatalf("the scope menu lost %q; items %q", msgs.ReviewScope(), items)
			}
			// The submenus list their entries once opened through the path.
			if err := tt.ChooseMenuItem(msgs.ReviewScopeNotes(), "A chapter of its own"); err != nil {
				t.Fatalf("the menu does not reach the note: %v; items %q", err, tt.Menu())
			}
			tt.Advance(320 * time.Millisecond)
			if a.reviewScope.Note == "" {
				t.Fatalf("choosing the note did not take: items %q", tt.Menu())
			}
		})
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

// Choosing a note narrows the queue to that note's cards, and back on the whole
// library everything returns: the choice lives in the session, not in the
// library.
func TestReviewScopeChoosesANoteAndGivesItBack(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	note := scopeSeed(t, a)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key1)

	whole, ok := tt.Find("4 cards left")
	if !ok {
		t.Fatalf("the whole-library queue is not four cards; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.ReviewScopeMenu()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(msgs.ReviewScopeNotes(), "A chapter of its own"); err != nil {
		t.Fatalf("the note was not chosen: %v; items %q", err, tt.Menu())
	}
	tt.Advance(320 * time.Millisecond)

	if _, ok := tt.Find(msgs.ReviewScopeEmpty()); !ok {
		t.Fatalf("the scoped queue did not say it is empty; texts %q", tt.Texts())
	}
	if tt.HasText("4 cards left") {
		t.Fatal("the scoped session still counts the whole library")
	}
	if whole.W <= 0 {
		t.Fatal("the whole-library count had no box to compare against")
	}
	if a.reviewScope.Kind != queue.ScopeNote || a.reviewScope.Note != note.ID {
		t.Fatalf("the app did not take the scope: %+v", a.reviewScope)
	}

	// Back on the whole library: no note is left out, because nothing was
	// stored — the scope lives in the session only.
	if err := tt.Click(msgs.ReviewScopeMenu()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(msgs.ReviewScope()); err != nil {
		t.Fatalf("the whole-library entry was not chosen: %v; items %q", err, tt.Menu())
	}
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText("4 cards left") {
		t.Fatalf("leaving the scope did not give the queue back; texts %q", tt.Texts())
	}
	if a.reviewScope.Kind != queue.ScopeAll {
		t.Fatalf("the scope did not reset: %+v", a.reviewScope)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after the scope returned")
	}
}

// A tag scope keeps only the cards whose blocks carry the tag, and the picker's
// label names it.
func TestReviewScopeChoosesATag(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	scopeSeed(t, a)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key1)

	if err := tt.Click(msgs.ReviewScopeMenu()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(msgs.ReviewScopeTags(), "chapter"); err != nil {
		t.Fatalf("the tag was not chosen: %v; items %q", err, tt.Menu())
	}
	tt.Advance(320 * time.Millisecond)

	if _, ok := tt.Find(msgs.ReviewScopeEmpty()); !ok {
		t.Fatalf("the tag scope did not narrow to its cards; texts %q", tt.Texts())
	}
	if a.reviewScope.Tag != "chapter" {
		t.Fatalf("the app did not take the tag scope: %+v", a.reviewScope)
	}
	if _, ok := tt.Find("#chapter"); !ok {
		t.Fatalf("the picker does not name the tag; texts %q", tt.Texts())
	}
}
