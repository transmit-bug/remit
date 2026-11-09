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

// windowSizes are the sizes every mode has to hold at: the one the window
// opens at and the smallest it goes.
var windowSizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// testLibrary is the library a test starts from: three notes and four cards,
// written through the real store into a temporary directory, so what the
// window reads is what a later run would read.
type testLibrary struct {
	dir     string
	memory  core.Note // "Memory and recall"
	writing core.Note // "Writing cards"
	habit   core.Note // "A daily habit"

	// The queue's order: the three reviewed cards by due time, then the card
	// that has never been reviewed.
	spacing core.Card // memory's first block
	frozen  core.Card // writing's first block, stopped following it
	testing core.Card // memory's second block
	pending core.Card // writing's second block, new
}

// seedLibrary creates a library in dir.
func seedLibrary(t *testing.T, dir string) testLibrary {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	save := func(title string, texts ...string) core.Note {
		blocks := make([]core.Block, len(texts))
		for i, text := range texts {
			blocks[i] = core.Block{Text: text}
		}
		note, err := lib.SaveNote(core.Note{Title: title, Blocks: blocks})
		if err != nil {
			t.Fatalf("save note %q: %v", title, err)
		}
		return note
	}
	memory := save("Memory and recall",
		"Spacing beats cramming :: reviewing just before you forget strengthens recall.",
		"The testing effect :: recalling an answer builds a stronger memory than rereading it.",
		"Interleaving :: mixing topics in one session trains you to tell them apart.",
	)
	writing := save("Writing cards",
		"A good card asks :: one question with one answer.",
		"A block divides at :: the first :: — prompt before, answer after.",
		"Cards point at blocks :: editing the note updates every card that follows it.",
	)
	habit := save("A daily habit",
		"When to review :: when the queue says a card is due, in one short sitting.",
		"What to do when stuck :: make the card smaller instead of repeating it.",
	)

	card := func(block core.ID) core.Card {
		c, err := firstCard(lib, block)
		if err != nil {
			t.Fatalf("create card: %v", err)
		}
		return c
	}
	spacing := card(memory.Blocks[0].ID)
	frozen := card(writing.Blocks[0].ID)
	testing := card(memory.Blocks[1].ID)
	pending := card(writing.Blocks[1].ID)

	// A frozen card shows what its block said when it stopped following it,
	// which is no longer what the block says.
	frozen.Ref.Frozen = &core.Snapshot{Text: "A good card asks :: one question at a time.", At: now}
	if err := lib.SaveCard(frozen); err != nil {
		t.Fatalf("freeze card: %v", err)
	}

	// Learning cards are queued by when they are due, so reviews a day apart
	// put the queue in the order of this list.
	for i, c := range []core.Card{spacing, frozen, testing} {
		if err := lib.Review(c.ID, schedule.Again, now.Add(time.Duration(i-3)*24*time.Hour)); err != nil {
			t.Fatalf("seed review: %v", err)
		}
	}
	return testLibrary{
		dir: dir, memory: memory, writing: writing, habit: habit,
		spacing: spacing, frozen: frozen, testing: testing, pending: pending,
	}
}

// settle draws the window's first frame and lets its entrance finish, so that a
// test asserting the view is at rest starts from a window that is: the
// entrance's two beats, and the lists on the page arriving behind each other.
func settle(tt *ui.Tester) {
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
}

// settingsPage opens the settings area at one of its pages, the way the
// sub-navigation does, and lets the page settle.
func settingsPage(t *testing.T, tt *ui.Tester, a *App, sub subID) {
	t.Helper()
	a.setArea(areaSettings, sub)
	settle(tt)
}

// newTestApp opens a window on a freshly seeded library.
func newTestApp(t *testing.T) (*App, testLibrary) {
	t.Helper()
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	return NewApp(Options{LibraryDir: dir}), seed
}

// openApp is newTestApp with its window drawn: the app at the size it opens at,
// settled. The ones that need another size, locale or darkness keep building
// their own tester over newTestApp.
func openApp(t *testing.T) (*App, testLibrary, *ui.Tester) {
	t.Helper()
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	return a, seed, tt
}

// assertControl checks that s has a box of its own inside the window, which is
// what makes it visible and clickable rather than only present as text.
func assertControl(t *testing.T, tt *ui.Tester, w, h int, s string) {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("%q has no box; texts %q", s, tt.Texts())
	}
	if r.W <= 0 || r.H <= 0 {
		t.Errorf("%q has an empty box %.1fx%.1f", s, r.W, r.H)
	}
	if r.X < 0 || r.Y < 0 || r.X+r.W > float32(w) || r.Y+r.H > float32(h) {
		t.Errorf("%q is outside the window: %.1f,%.1f %.1fx%.1f in %dx%d", s, r.X, r.Y, r.W, r.H, w, h)
	}
}

// firstCard makes one basic card for a block, which is all this test needs: the
// shapes a paragraph makes are the card plugin's decision, and the store only
// stores what it is told to.
func firstCard(lib *store.Library, blockID core.ID) (core.Card, error) {
	cards, err := lib.CreateCards(blockID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return cards[0], nil
}

// recallQueue puts the window on the recall area's queue and enters the session
// when the queue has cards to review: the digit keys name the areas in rail
// order, so it is Cmd+4, and Start is the page's one way into a card. An empty
// queue leaves the page up, with its empty state.
func recallQueue(tt *ui.Tester) {
	recallPage(tt)
	for _, loc := range []string{"en", "zh-Hans"} {
		if err := tt.Click(locales.In(loc).RecallStart()); err == nil {
			tt.Frame()
			return
		}
	}
}

// recallPage puts the window on the recall area's queue page alone, without
// entering the session.
func recallPage(tt *ui.Tester) {
	tt.Key(ui.Cmd, ui.Key4)
	tt.Frame()
}

// cardsPage puts the window on the recall area's card table, where the browse
// screen lives, in the language the window is showing.
func cardsPage(t *testing.T, tt *ui.Tester, loc string) {
	t.Helper()
	recallPage(tt)
	if err := tt.Click(locales.In(loc).CardsPage()); err != nil {
		t.Fatalf("the recall area has no card page: %v", err)
	}
}

// searchPage puts the window on the write area's search page.
func searchPage(t *testing.T, tt *ui.Tester, loc string) {
	t.Helper()
	tt.Key(ui.Cmd, ui.Key2)
	tt.Frame()
	if err := tt.Click(locales.In(loc).SearchPage()); err != nil {
		t.Fatalf("the write area has no search page: %v", err)
	}
}
