package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Escape on a page the user reached from another page returns there: the search
// page was opened from the notes list, so leaving it lands on that list rather
// than on the document a mode names.
func TestEscapeFromSearchReturnsToTheNotesList(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	searchPage(t, tt, "en")

	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	if !tt.HasText("Writing cards") {
		t.Errorf("Escape left the search page for the document; texts %q", tt.Texts())
	}
}

// A page entered from outside the window has nothing behind it: a link lands
// where it names, and back stays there instead of guessing a page.
func TestOutsideEntryLeavesNothingBehind(t *testing.T) {
	a, _ := newTestApp(t)
	a.land(areaRead, subReadArchive)

	a.goBack()

	if a.area != areaRead || a.sub != subReadArchive {
		t.Errorf("back moved to %v/%v, want the archive the link named", a.area, a.sub)
	}
}

// A session is opened over the page it starts from, so leaving it returns there:
// a review started at the card table goes back to the card table, not to the
// queue a mode maps to.
func TestBackRemembersThePageASessionWasEnteredFrom(t *testing.T) {
	a, _ := newTestApp(t)
	a.goTo(areaRecall, subRecallCards)
	a.enterSession(sessionReview)

	a.goBack()

	if a.area != areaRecall || a.sub != subRecallCards || a.session != sessionNone {
		t.Errorf("leaving the session landed on %v/%v (session %v), want the card table",
			a.area, a.sub, a.session)
	}
}
