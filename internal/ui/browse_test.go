package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// lostBody is the sentence a lost card's row shows: its prompt has no text of
// its own, because the block it came from is gone.
const lostBody = "The note block this card came from is gone."

// Browse is a mode of the window: Cmd+3 shows the search field, the filters and
// the cards, and the frame settles.
func TestBrowseEnter(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			tt.Advance(200 * time.Millisecond) // the opening card's entry

			tt.Key(ui.Cmd, ui.Key3)
			tt.Advance(200 * time.Millisecond) // the review card's exit
			for _, want := range []string{"Browse cards", "Search cards", "All", "Due", "Spacing beats cramming", "Memory and recall", "4 cards"} {
				if !tt.HasText(want) {
					t.Fatalf("browse is missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.HasText("Notebook") {
				t.Error("browse still shows the notes list")
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// Cmd+F opens browse on its search field from anywhere, and what is typed
// narrows the rows by the cards' text: a prompt comes and goes.
func TestBrowseSearch(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Advance(200 * time.Millisecond) // the opening card's entry

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Advance(200 * time.Millisecond) // the review card's exit
	if !tt.HasText("Browse cards") {
		t.Fatalf("Cmd+F did not open browse; texts %q", tt.Texts())
	}
	tt.Type("testing")
	if !tt.HasText("The testing effect") {
		t.Fatalf("the search lost the matching card; texts %q", tt.Texts())
	}
	for _, gone := range []string{"Spacing beats cramming", "A good card asks", "A block divides at"} {
		if tt.HasText(gone) {
			t.Errorf("the search kept %q", gone)
		}
	}

	// Escape with text clears the query and the rows come back.
	tt.Key(0, ui.KeyEscape)
	for _, want := range []string{"Spacing beats cramming", "A good card asks", "A block divides at"} {
		if !tt.HasText(want) {
			t.Fatalf("clearing the query lost %q; texts %q", want, tt.Texts())
		}
	}

	// With nothing typed, Escape leaves browse.
	tt.Key(0, ui.KeyEscape)
	if !tt.HasText("Notebook") {
		t.Fatalf("Esc did not return to the notes; texts %q", tt.Texts())
	}
	tt.Advance(200 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// browseSeed is the library the filter test reads: exactly one card per
// filter, so each filter's rows are its own.
type browseSeed struct {
	due, fresh, suspended, lost core.Card
}

// seedBrowseLibrary writes a library with one due card, one never reviewed, one
// suspended and one whose block is gone.
func seedBrowseLibrary(t *testing.T) (*App, browseSeed) {
	t.Helper()
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	note, err := lib.SaveNote(core.Note{Title: "Filters", Blocks: []core.Block{
		{Text: "Due card :: answer"},
		{Text: "Fresh card :: answer"},
		{Text: "Suspended card :: answer"},
		{Text: "Lost card :: answer"},
	}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	card := func(i int) core.Card {
		c, err := lib.CreateCard(note.Blocks[i].ID)
		if err != nil {
			t.Fatalf("create card: %v", err)
		}
		return c
	}
	due, fresh, suspended, lost := card(0), card(1), card(2), card(3)

	// Again a day ago leaves the card due now, in learning.
	if err := lib.Review(due.ID, schedule.Again, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("review due card: %v", err)
	}
	suspended.Status = core.StatusSuspended
	if err := lib.SaveCard(suspended); err != nil {
		t.Fatalf("suspend card: %v", err)
	}
	// Reviewed a day ago and scheduled ahead: lost, but neither new nor due.
	if err := lib.Review(lost.ID, schedule.Good, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("review lost card: %v", err)
	}
	// Deleting the paragraph the card was made from loses its reference.
	note.Blocks = note.Blocks[:3]
	if _, err := lib.SaveNote(note); err != nil {
		t.Fatalf("delete block: %v", err)
	}

	return NewApp(Options{LibraryDir: dir}), browseSeed{due: due, fresh: fresh, suspended: suspended, lost: lost}
}

// Each filter is its own answer from the store, and All brings every card back.
func TestBrowseFilters(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := seedBrowseLibrary(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			tt.Advance(200 * time.Millisecond) // the opening card's entry
			tt.Key(ui.Cmd, ui.Key3)
			tt.Advance(200 * time.Millisecond) // the review card's exit

			for _, want := range []string{"Due card", "Fresh card", "Suspended card", lostBody} {
				if !tt.HasText(want) {
					t.Fatalf("All is missing %q; texts %q", want, tt.Texts())
				}
			}

			cases := []struct {
				filter string
				want   string
				gone   []string
			}{
				{"Due", "Due card", []string{"Fresh card", "Suspended card", lostBody}},
				{"New", "Fresh card", []string{"Due card", "Suspended card", lostBody}},
				{"Suspended", "Suspended card", []string{"Due card", "Fresh card", lostBody}},
				{"Lost", lostBody, []string{"Due card", "Fresh card", "Suspended card"}},
			}
			for _, tc := range cases {
				if err := tt.Click(tc.filter); err != nil {
					t.Fatalf("click %q: %v", tc.filter, err)
				}
				if !tt.HasText(tc.want) {
					t.Fatalf("%s filter lost %q; texts %q", tc.filter, tc.want, tt.Texts())
				}
				for _, gone := range tc.gone {
					if tt.HasText(gone) {
						t.Errorf("%s filter kept %q", tc.filter, gone)
					}
				}
				if tt.FramePending() {
					t.Errorf("%s filter did not settle", tc.filter)
				}
			}

			if err := tt.Click("All"); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Due card", "Fresh card", "Suspended card", lostBody} {
				if !tt.HasText(want) {
					t.Fatalf("All did not bring %q back; texts %q", want, tt.Texts())
				}
			}
		})
	}
}

// Choosing a card opens the note it came from, or the card in review when it
// has no note to open.
func TestBrowseOpens(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Advance(200 * time.Millisecond) // the opening card's entry
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(200 * time.Millisecond) // the review card's exit

	if err := tt.Click("Spacing beats cramming"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	for _, want := range []string{"Notebook", "Block 1 · Memory and recall"} {
		if !tt.HasText(want) {
			t.Fatalf("Enter did not open the card's note: missing %q; texts %q", want, tt.Texts())
		}
	}

	// The Open button does what Enter does.
	tt.Key(ui.Cmd, ui.Key3)
	if err := tt.Click("A good card asks"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Open (Enter)"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Block 1 · Writing cards") {
		t.Fatalf("Open did not show the card's note; texts %q", tt.Texts())
	}
	if tt.HasText("Block 1 · Memory and recall") {
		t.Error("the editor still shows the note opened before")
	}
	tt.Advance(200 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Opening a lost card lands in review, where it can still be suspended.
func TestBrowseOpensLostCard(t *testing.T) {
	a, _ := seedBrowseLibrary(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Advance(200 * time.Millisecond) // the opening card's entry
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(200 * time.Millisecond) // the review card's exit

	if err := tt.Click(lostBody); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	for _, want := range []string{"Lost", lostBody, "Suspend card"} {
		if !tt.HasText(want) {
			t.Fatalf("a lost card did not open in review: missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("Notebook") {
		t.Error("a lost card opened the editor")
	}
	tt.Advance(200 * time.Millisecond) // the card's entry
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A search link from outside the window opens browse on its query, its field
// ready for typing.
func TestSearchLink(t *testing.T) {
	a, _ := newTestApp(t)

	// The link may arrive before the window is drawn: the focus request waits
	// for the field's first build.
	a.Search("question")
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	for _, want := range []string{"Browse cards", "A good card asks"} {
		if !tt.HasText(want) {
			t.Fatalf("the search link is missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("Spacing beats cramming") {
		t.Error("the search link did not narrow the rows")
	}

	// Typing reaches the field the link focused.
	tt.Type("zzz")
	if !tt.HasText("No cards match.") {
		t.Fatalf("the search field did not take the text; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
