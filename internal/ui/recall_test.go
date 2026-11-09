package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/locales"
)

// recallSeed is the library the recall pages read: three cards whose lapse
// counts differ, one suspended card, and a card whose block is gone.
type recallSeed struct {
	alpha, beta, gamma core.Card
	suspended, lost    core.Card
}

// seedRecallLibrary writes a library with one card answered Again three times,
// one answered Again once, one never reviewed, one suspended and one lost.
func seedRecallLibrary(t *testing.T) (*App, recallSeed) {
	t.Helper()
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	note, err := lib.SaveNote(core.Note{Title: "Sorting", Blocks: []core.Block{
		{Text: "Alpha card :: first"},
		{Text: "Beta card :: second"},
		{Text: "Gamma card :: third"},
		{Text: "Suspended card :: fourth"},
		{Text: "Lost card :: fifth"},
	}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	card := func(i int) core.Card {
		c, err := firstCard(lib, note.Blocks[i].ID)
		if err != nil {
			t.Fatalf("create card: %v", err)
		}
		return c
	}
	alpha, beta, gamma := card(0), card(1), card(2)
	suspended, lost := card(3), card(4)
	for i := 0; i < 3; i++ {
		if err := lib.Review(alpha.ID, schedule.Again, now.Add(time.Duration(-72+i)*time.Hour)); err != nil {
			t.Fatalf("review alpha: %v", err)
		}
	}
	if err := lib.Review(beta.ID, schedule.Again, now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("review beta: %v", err)
	}
	suspended.Status = core.StatusSuspended
	if err := lib.SaveCard(suspended); err != nil {
		t.Fatalf("suspend card: %v", err)
	}
	note.Blocks = note.Blocks[:4]
	if _, err := lib.SaveNote(note); err != nil {
		t.Fatalf("delete block: %v", err)
	}
	return NewApp(Options{LibraryDir: dir}), recallSeed{
		alpha: alpha, beta: beta, gamma: gamma, suspended: suspended, lost: lost,
	}
}

// recallPage opens the window on one of the recall area's pages.
func recallPage(t *testing.T, tt *ui.Tester, loc, page string) {
	t.Helper()
	recallQueue(tt)
	tt.Frame()
	if err := tt.Click(locales.In(loc).CardsPage()); err != nil {
		// The queue is the area's first page; the tab row holds the rest.
		t.Fatalf("the recall area has no card page: %v", err)
	}
	tt.Frame()
	if page == locales.In(loc).CardsPage() {
		return
	}
	if err := tt.Click(page); err != nil {
		t.Fatalf("the recall area has no %q page: %v", page, err)
	}
	tt.Frame()
}

// The card table sorts by the column clicked: the lapses column puts the card
// answered Again most often first, and its second click reverses the order.
func TestCardTableSortsByColumn(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := seedRecallLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			recallPage(t, tt, "en", msgs.CardsPage())

			if err := tt.Click(msgs.RecallColumnLapses()); err != nil {
				t.Fatalf("sort by lapses: %v", err)
			}
			tt.Frame()
			alpha, ok := tt.Find("Alpha card")
			if !ok {
				t.Fatalf("the table lost Alpha; texts %q", tt.Texts())
			}
			gamma, ok := tt.Find("Gamma card")
			if !ok {
				t.Fatalf("the table lost Gamma; texts %q", tt.Texts())
			}
			if alpha.Y >= gamma.Y {
				t.Errorf("the lapses sort put Alpha at %.1f and Gamma at %.1f, want Alpha first", alpha.Y, gamma.Y)
			}

			if err := tt.Click(msgs.RecallColumnLapses()); err != nil {
				t.Fatalf("reverse the sort: %v", err)
			}
			tt.Frame()
			alpha, _ = tt.Find("Alpha card")
			gamma, _ = tt.Find("Gamma card")
			if gamma.Y >= alpha.Y {
				t.Errorf("the reversed sort put Gamma at %.1f and Alpha at %.1f, want Gamma first", gamma.Y, alpha.Y)
			}
		})
	}
}

// The search field reads the card syntax: is:suspended narrows the table to the
// suspended card, and a term the syntax does not know searches as text.
func TestCardSearchSyntaxNarrows(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := seedRecallLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			recallPage(t, tt, "en", msgs.CardsPage())

			if err := tt.Click(msgs.RecallCardsSearch()); err != nil {
				t.Fatalf("focus the search field: %v", err)
			}
			tt.Type("is:suspended")
			tt.Frame()
			if !tt.HasText("Suspended card") {
				t.Fatalf("is:suspended lost its card; texts %q", tt.Texts())
			}
			for _, gone := range []string{"Alpha card", "Beta card", "Gamma card"} {
				if tt.HasText(gone) {
					t.Errorf("is:suspended kept %q", gone)
				}
			}

			tt.Type(" zzz")
			tt.Frame()
			if !tt.HasText(msgs.RecallCardsEmpty()) {
				t.Fatalf("a query that matches nothing kept rows; texts %q", tt.Texts())
			}
		})
	}
}

// The bulk bar suspends every row checked, and the store is what holds it: both
// cards leave the queue with their history.
func TestCardTableBulkSuspends(t *testing.T) {
	msgs := locales.In("en")
	a, seed := seedRecallLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(t, tt, "en", msgs.CardsPage())

	for _, prompt := range []string{"Alpha card", "Beta card"} {
		if err := tt.Click(msgs.RecallCardsSelect(prompt)); err != nil {
			t.Fatalf("check %q: %v", prompt, err)
		}
		tt.Frame()
	}
	if !tt.HasText(msgs.RecallCardsSelected(2)) {
		t.Fatalf("the bulk bar does not count the rows checked; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.Suspend()); err != nil {
		t.Fatalf("suspend the rows checked: %v", err)
	}
	tt.Frame()

	for _, id := range []core.ID{seed.alpha.ID, seed.beta.ID} {
		card, ok := a.lib.Card(id)
		if !ok {
			t.Fatalf("card %q is gone", id)
		}
		if card.Status != core.StatusSuspended {
			t.Errorf("card %q is %s, want suspended", id, card.Status)
		}
	}
	if got, err := a.lib.CardsWithStatus(core.StatusSuspended); err != nil || len(got) < 2 {
		t.Errorf("the library holds %d suspended cards (%v), want the two written", len(got), err)
	}
	tt.Advance(5 * time.Second) // the toast
}

// The presets page writes the library's rhythm into remit.json: one slider step
// is one file.
func TestPresetsWriteTheLibraryConfig(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := seedRecallLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			recallPage(t, tt, "en", msgs.PresetsPage())

			before := a.lib.Presets().Review.NewPerDay
			if err := tt.Click(msgs.RecallPresetsNewCards()); err != nil {
				t.Fatalf("focus the new-cards slider: %v", err)
			}
			tt.Key(0, ui.KeyRight)
			tt.Frame()

			after := a.lib.Presets().Review.NewPerDay
			if after == before {
				t.Fatalf("the slider wrote nothing: newPerDay is still %d", after)
			}
			config := configOf(t, a.dir)
			presets, _ := config["presets"].(map[string]any)
			review, _ := presets["review"].(map[string]any)
			if review["newPerDay"] != float64(after) {
				t.Errorf("remit.json review.newPerDay = %v, want the %d the page wrote", review["newPerDay"], after)
			}
		})
	}
}

// Restoring the defaults asks first, and writes them back when the question is
// answered: the file the library keeps holds the defaults again.
func TestPresetsResetToDefaults(t *testing.T) {
	msgs := locales.In("en")
	a, _ := seedRecallLibrary(t)
	custom := store.DefaultPresets()
	custom.Review.NewPerDay = 33
	custom.Hard.Lapses = 7
	if err := a.lib.SetPresets(custom); err != nil {
		t.Fatalf("set presets: %v", err)
	}
	a.reload()

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(t, tt, "en", msgs.PresetsPage())

	reveal(t, tt, 1024, 720, msgs.RecallPresetsReset())
	if err := tt.Click(msgs.RecallPresetsReset()); err != nil {
		t.Fatalf("ask to restore the defaults: %v", err)
	}
	tt.Frame()
	if !tt.HasText(msgs.RecallPresetsResetTitle()) {
		t.Fatalf("the reset asks nothing; texts %q", tt.Texts())
	}
	// Cancelling keeps the library's rhythm.
	if err := tt.Click(msgs.Cancel()); err != nil {
		t.Fatalf("cancel the reset: %v", err)
	}
	tt.Frame()
	if got := a.lib.Presets().Review.NewPerDay; got != 33 {
		t.Fatalf("cancelling wrote newPerDay %d", got)
	}

	if err := tt.Click(msgs.RecallPresetsReset()); err != nil {
		t.Fatalf("ask again: %v", err)
	}
	tt.Frame()
	if err := tt.Click(msgs.RecallPresetsResetConfirm()); err != nil {
		t.Fatalf("confirm the reset: %v", err)
	}
	tt.Frame()

	if got := a.lib.Presets(); got != store.DefaultPresets() {
		t.Fatalf("the library's presets are %+v, want the defaults", got)
	}
	config := configOf(t, a.dir)
	presets, _ := config["presets"].(map[string]any)
	review, _ := presets["review"].(map[string]any)
	if review["newPerDay"] != float64(store.DefaultPresets().Review.NewPerDay) {
		t.Errorf("remit.json review.newPerDay = %v, want the default", review["newPerDay"])
	}
	tt.Advance(5 * time.Second) // the toast
}

// The history page lists what was answered today, with the note the card came
// from.
func TestHistoryListsTodaysReviews(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, seed := seedRecallLibrary(t)
			if err := a.lib.Review(seed.gamma.ID, schedule.Good, time.Now()); err != nil {
				t.Fatalf("review today: %v", err)
			}
			a.reload()
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			recallPage(t, tt, "en", msgs.HistoryPage())

			if !tt.HasText(msgs.RecallHistoryTimeline()) {
				t.Fatalf("the history has no timeline; texts %q", tt.Texts())
			}
			if !tt.HasText("Gamma card") {
				t.Fatalf("today's answer is not listed; texts %q", tt.Texts())
			}
			if !tt.HasText("Sorting") {
				t.Errorf("the answer does not name the note it came from; texts %q", tt.Texts())
			}
		})
	}
}

// The history's numbers are the log's: three answers with one Again read as 67%
// right, and the Again count is one.
func TestHistoryCountsGrades(t *testing.T) {
	msgs := locales.In("en")
	a, seed := seedRecallLibrary(t)
	now := time.Now()
	for _, card := range []core.Card{seed.alpha, seed.beta} {
		if err := a.lib.Review(card.ID, schedule.Good, now); err != nil {
			t.Fatalf("review good: %v", err)
		}
	}
	if err := a.lib.Review(seed.gamma.ID, schedule.Again, now); err != nil {
		t.Fatalf("review again: %v", err)
	}
	a.reload()
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(t, tt, "en", msgs.HistoryPage())

	for _, want := range []string{msgs.RecallHistoryGrades(), msgs.Again(), msgs.Good(), "67%"} {
		if !tt.HasText(want) {
			t.Fatalf("the history is missing %q; texts %q", want, tt.Texts())
		}
	}
}

// A lost card is pointed at a block from the broken-links page, and the store
// holds it: the card reads its new block, live again.
func TestRelinkReattachesALostCard(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, seed := seedRecallLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			recallPage(t, tt, "en", msgs.RelinkPage())

			if err := tt.Click(msgs.LostBody()); err != nil {
				t.Fatalf("choose the lost card: %v", err)
			}
			tt.Frame()
			if err := tt.Click("Beta card :: second"); err != nil {
				t.Fatalf("choose the block: %v", err)
			}
			tt.Frame()
			reveal(t, tt, size.w, size.h, msgs.RecallRelinkRelink())
			if err := tt.Click(msgs.RecallRelinkRelink()); err != nil {
				t.Fatalf("re-link the card: %v", err)
			}
			tt.Frame()

			card, ok := a.lib.Card(seed.lost.ID)
			if !ok {
				t.Fatal("the lost card is gone")
			}
			if _, _, state := a.faces(card); state != core.LinkLive {
				t.Errorf("the re-linked card is %s, want live", state)
			}
			before, _ := a.lib.Card(seed.beta.ID)
			if card.Ref.Block != before.Ref.Block {
				t.Errorf("the card reads %q, want Beta's block", card.Ref.Block)
			}
			tt.Advance(5 * time.Second) // the toast
		})
	}
}

// The block search narrows as it is typed: a term the app's blocks do not hold
// takes them away, and clearing it brings them back.
func TestRelinkNarrowsAsYouType(t *testing.T) {
	msgs := locales.In("en")
	a, _ := seedRecallLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(t, tt, "en", msgs.RelinkPage())

	if err := tt.Click(msgs.RecallRelinkSearch()); err != nil {
		t.Fatalf("focus the block search: %v", err)
	}
	tt.Type("Beta")
	tt.Frame()
	if !tt.HasText("Beta card :: second") {
		t.Fatalf("the search lost the matching block; texts %q", tt.Texts())
	}
	for _, gone := range []string{"Alpha card :: first", "Gamma card :: third"} {
		if tt.HasText(gone) {
			t.Errorf("the search kept %q", gone)
		}
	}

	tt.Type(" zzz")
	tt.Frame()
	if !tt.HasText(msgs.RecallRelinkNoBlocks()) {
		t.Fatalf("a search with no match says nothing; texts %q", tt.Texts())
	}
}

// A frozen card follows its block again once it is thawed, and its reviews are
// untouched.
func TestRelinkThawsAFrozenCard(t *testing.T) {
	msgs := locales.In("en")
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(t, tt, "en", msgs.RelinkPage())

	const frozen = "A good card asks"
	reveal(t, tt, 1024, 720, msgs.RecallRelinkThawCard(frozen))
	if err := tt.Click(msgs.RecallRelinkThawCard(frozen)); err != nil {
		t.Fatalf("thaw the frozen card: %v", err)
	}
	tt.Frame()

	card, ok := a.lib.Card(seed.frozen.ID)
	if !ok {
		t.Fatal("the frozen card is gone")
	}
	if card.Ref.Frozen != nil {
		t.Error("the thawed card still holds a snapshot")
	}
	if _, _, state := a.faces(card); state != core.LinkLive {
		t.Errorf("the thawed card is %s, want live", state)
	}
	if tt.HasText(frozen) {
		t.Errorf("the frozen section still lists the card; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.RecallRelinkEmpty()) {
		t.Errorf("the page does not say there is nothing left to repair; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // the toast
}

// The recall area's pages hold the smallest window in Chinese: the table, the
// presets and the broken links each keep their controls inside it.
func TestRecallPagesAtMinimumInChinese(t *testing.T) {
	zh := locales.In("zh-Hans")
	a, _ := seedRecallLibrary(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	settle(tt)
	recallPage(t, tt, "zh-Hans", zh.CardsPage())

	for _, label := range []string{zh.RecallCardsTitle(), zh.RecallCardsSyntax(), zh.RecallCardsFilters(), zh.RecallColumnPrompt()} {
		assertControl(t, tt, 720, 520, label)
	}
	assertControl(t, tt, 720, 520, "Alpha card")

	recallPage(t, tt, "zh-Hans", zh.PresetsPage())
	for _, label := range []string{zh.RecallPresetsTitle(), zh.RecallPresetsSteps(), zh.RecallPresetsLimits()} {
		assertControl(t, tt, 720, 520, label)
	}

	recallPage(t, tt, "zh-Hans", zh.RelinkPage())
	for _, label := range []string{zh.RecallRelinkTitle(), zh.RecallRelinkLost(), zh.RecallRelinkBlocks(), zh.RecallRelinkSearch()} {
		assertControl(t, tt, 720, 520, label)
	}
	tt.Advance(500 * time.Millisecond)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
