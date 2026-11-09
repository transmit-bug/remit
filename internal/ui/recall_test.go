package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/screens/browse"
	"github.com/transmit-bug/remit/internal/ui/screens/recall"
	"github.com/transmit-bug/remit/locales"
)

// The queue page groups the cards by the note they came from: one row per note,
// what the note holds, and how much of it is waiting — at both sizes the window
// holds, and in the language the window is showing.
func TestRecallQueueGroupsByNote(t *testing.T) {
	for _, tc := range []struct {
		name  string
		loc   string
		w, h  int
		notes []string
		meta  string
		due   string
	}{
		{
			name: "default", loc: "en", w: 1024, h: 720,
			notes: []string{"Memory and recall", "Writing cards"},
			meta:  locales.In("en").RecallRowMeta(3, 2, 0),
			due:   locales.In("en").RecallRowDue(2),
		},
		{
			name: "minimum", loc: "en", w: 720, h: 520,
			notes: []string{"Memory and recall", "Writing cards"},
			meta:  locales.In("en").RecallRowMeta(3, 2, 0),
			due:   locales.In("en").RecallRowDue(2),
		},
		{
			name: "chinese minimum", loc: "zh-Hans", w: 720, h: 520,
			notes: []string{"Memory and recall", "Writing cards"},
			meta:  locales.In("zh-Hans").RecallRowMeta(3, 2, 0),
			due:   locales.In("zh-Hans").RecallRowDue(2),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msgs := locales.In(tc.loc)
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, tc.w, tc.h)
			tt.SetLocale(tc.loc)
			settle(tt)
			recallPage(tt)
			tt.Advance(320 * time.Millisecond) // the page's entrance

			for _, want := range []string{
				msgs.RecallTitle(), msgs.RecallByNote(), msgs.RecallLoad(),
				msgs.RecallRatioLabel(1, 3, 0), msgs.RecallHeat(), tc.meta, tc.due,
			} {
				if !tt.HasText(want) {
					t.Fatalf("the page is missing %q; texts %q", want, tt.Texts())
				}
			}
			for _, note := range tc.notes {
				if !tt.HasText(note) {
					t.Fatalf("the rows are missing the note %q; texts %q", note, tt.Texts())
				}
			}
			// One row per note, not one per card: the library's four cards are two
			// rows of two, which is the number beside each title.
			if n := countTexts(tt.Texts(), tc.due); n != 2 {
				t.Errorf("%q is drawn %d times, want one row per note", tc.due, n)
			}
			// The sitting is priced on all four cards, whatever the rows group.
			if !tt.HasText(msgs.TodayMinutes(2)) {
				t.Errorf("the load card does not price the day; texts %q", tt.Texts())
			}
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// A filter narrows the rows the page lists and nothing else: the cards it
// counts, the sitting it prices and the session Start takes all stay the scope's.
func TestRecallFilterNarrowsTheQueue(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(tt)

	if a.recall.Filter != recall.FilterAll {
		t.Fatalf("the page opens on %v, want every card", a.recall.Filter)
	}
	all := a.recallCounts.Remaining

	// The never-reviewed card is Writing cards' second one: its note keeps a row
	// with one card in it, and the note with none loses its row.
	if err := tt.Click(msgs.RecallFilterNew()); err != nil {
		t.Fatalf("the filter row has no New filter: %v; texts %q", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText(msgs.RecallRowDue(1)) {
		t.Fatalf("the New filter does not count its card; texts %q", tt.Texts())
	}
	if tt.HasText("Memory and recall") {
		t.Fatalf("the New filter kept a note with no new card; texts %q", tt.Texts())
	}

	// Learning keeps the three cards the harness reviewed Again, in both notes.
	if err := tt.Click(msgs.RecallFilterLearning()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"Memory and recall", "Writing cards", msgs.RecallRowDue(2)} {
		if !tt.HasText(want) {
			t.Fatalf("the Learning filter is missing %q; texts %q", want, tt.Texts())
		}
	}

	// Nothing is hard yet: the page says so where the rows would be, rather than
	// claiming the day is empty.
	if err := tt.Click(msgs.RecallFilterHard()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(msgs.RecallNoMatch()) {
		t.Fatalf("a filter that keeps nothing has no line of its own; texts %q", tt.Texts())
	}
	if tt.HasText(msgs.RecallEmpty()) {
		t.Error("a filter that keeps nothing claims the day is empty")
	}

	// The filter is the page's, not the session's: the queue, its price and its
	// start hold every card still.
	if a.recallCounts.Remaining != all {
		t.Errorf("the filter changed the queue: %d cards, want %d", a.recallCounts.Remaining, all)
	}
	if !tt.HasText(msgs.TodayMinutes(2)) {
		t.Errorf("the filter repriced the sitting; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.RecallStart()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(msgs.Remaining(all)) {
		t.Fatalf("a filtered page started a narrowed session; texts %q", tt.Texts())
	}
}

// A filter the user keeps is a chip on the queue page after a restart, applies
// the filter it names, and leaves settings.json when the chip is taken away.
func TestSavedFilterSurvivesRestart(t *testing.T) {
	msgs := locales.In("en")
	data, dir := t.TempDir(), t.TempDir()
	seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir, DataDir: data})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(tt)

	if err := tt.Click(msgs.RecallFilterHard()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(msgs.RecallSaveFilter()); err != nil {
		t.Fatalf("the page has no save action: %v; texts %q", err, tt.Texts())
	}
	tt.Frame()
	saved, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.SavedFilters) != 1 || recall.FilterOf(saved.SavedFilters[0]) != recall.FilterHard {
		t.Fatalf("settings.json holds %q, want the hard filter", saved.SavedFilters)
	}
	if _, ok := tt.Find(msgs.RecallSaved()); !ok {
		t.Fatalf("the page does not show the kept filter; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.RecallFilterSaved()) {
		t.Fatalf("the page does not show the kept filter; texts %q", tt.Texts())
	}

	// The next start reads the chip back, and the chip is the filter it names.
	next := NewApp(Options{LibraryDir: dir, DataDir: data})
	tt = ui.NewTester(next.View, 1024, 720)
	settle(tt)
	recallPage(tt)
	if _, ok := tt.Find(msgs.RecallSaved()); !ok {
		t.Fatalf("the restarted page lost the kept filter; texts %q", tt.Texts())
	}
	clickRole(t, tt, ui.RoleButton, msgs.RecallFilterHard())
	if next.recall.Filter != recall.FilterHard {
		t.Fatalf("the chip applied %v, want the hard filter", next.recall.Filter)
	}

	// Taking the chip away is the only place the filter lives.
	if err := tt.Click(msgs.RecallRemoveFilter(msgs.RecallFilterHard())); err != nil {
		t.Fatalf("the chip has no remove action: %v; texts %q", err, tt.Texts())
	}
	tt.Frame()
	saved, err = settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.SavedFilters) != 0 {
		t.Fatalf("settings.json still holds %q", saved.SavedFilters)
	}
	if _, ok := tt.Find(msgs.RecallSaved()); ok {
		t.Fatalf("the chip is still drawn; texts %q", tt.Texts())
	}
}

// A card the session answered Again comes back at its end, grouped under the note
// it came from: the summary is a way back into the material, not only a count.
func TestReviewSummaryCarriesTheMissedCards(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallQueue(tt)

	// The first card is answered Again, the three after it Good, so the session
	// ends with one note missed.
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key1) // Again
	tt.Advance(320 * time.Millisecond)
	id, ok := a.review.Last()
	if !ok {
		t.Fatal("the Again grade was not recorded")
	}
	card, ok := a.Library().Card(id)
	if !ok {
		t.Fatal("the graded card is gone")
	}
	note, ok := a.data.noteOfBlock(card.Ref.Block)
	if !ok {
		t.Fatal("the graded card lost its note")
	}
	for range 3 {
		tt.Key(0, ui.KeySpace)
		tt.Key(0, ui.Key3) // Good
		tt.Advance(320 * time.Millisecond)
	}
	tt.Frame()

	if got := a.failedCardsOf(); len(got) != 1 {
		t.Fatalf("the session reports %d missed cards, want one", len(got))
	}
	if !tt.HasText(msgs.SummaryFailed()) {
		t.Fatalf("the summary lost the missed card; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.SummaryFailedOpen(a.data.titleOf(note))) {
		t.Fatalf("the summary does not name the note it came from; texts %q", tt.Texts())
	}
}

// countTexts is how many of the drawn texts are exactly s, which is how a test
// says one row per note rather than one per card.
func countTexts(texts []string, s string) int {
	n := 0
	for _, text := range texts {
		if text == s {
			n++
		}
	}
	return n
}

// The Lost filter counts the cards whose paragraph is gone and offers the one
// place they can be repaired: they are outside the queue, so they cannot be
// listed by note, and a chip that listed nothing would be a dead control.
func TestRecallLostFilterOffersTheRepair(t *testing.T) {
	msgs := locales.In("en")
	a, _ := seedBrowseLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(tt)

	if err := tt.Click(msgs.RecallFilterLost()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(msgs.RecallLost(1)) {
		t.Fatalf("the Lost filter does not count the card; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.RecallRelink()); err != nil {
		t.Fatalf("the Lost filter has no way to the repair: %v; texts %q", err, tt.Texts())
	}
	tt.Advance(320 * time.Millisecond)
	if a.area != areaRecall || a.sub != subRecallCards {
		t.Fatalf("the repair opened %v/%v, want the card table", a.area, a.sub)
	}
	if a.browse.Filter != browse.Lost {
		t.Fatalf("the card table is filtered to %v, want the lost cards", a.browse.Filter)
	}
	if !tt.HasText(lostBody) {
		t.Fatalf("the lost card is not listed; texts %q", tt.Texts())
	}
}
