package store_test

import (
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// TestQueryNotesPagesWithoutRepeatsWhenKeysTie is the paging guarantee: notes
// whose sort key is the same still come out in one order, so page 2 begins
// exactly where page 1 ended and no note is shown twice or skipped. Every note
// here has the same title and the same save, so the ID is what orders them.
func TestQueryNotesPagesWithoutRepeatsWhenKeysTie(t *testing.T) {
	lib, _, _ := newLibrary(t)
	var want []string
	for range 7 {
		note, err := lib.SaveNote(core.Note{Title: "Same", Blocks: []core.Block{{Text: "body"}}})
		if err != nil {
			t.Fatalf("SaveNote: %v", err)
		}
		want = append(want, string(note.ID))
	}
	sort.Strings(want)

	for _, order := range []store.NoteSort{store.NoteSortUpdated, store.NoteSortTitle, store.NoteSortCreated} {
		t.Run(string(order), func(t *testing.T) {
			whole, total, err := lib.QueryNotes(store.NoteQuery{Sort: order})
			if err != nil {
				t.Fatalf("QueryNotes: %v", err)
			}
			if total != len(want) {
				t.Fatalf("total = %d, want %d", total, len(want))
			}
			if got := noteIDs(whole); !slices.Equal(got, want) {
				t.Fatalf("unpaged order = %v, want %v", got, want)
			}

			paged, total := pageNotes(t, lib, store.NoteQuery{Sort: order}, 3)
			if total != len(want) {
				t.Fatalf("paged total = %d, want %d", total, len(want))
			}
			if got := noteIDs(paged); !slices.Equal(got, want) {
				t.Fatalf("paged order = %v, want %v", got, want)
			}
		})
	}
}

// TestQueryCardsPagesWithoutRepeatsWhenKeysTie is the same guarantee for cards:
// every card was made in the same instant from a note with one title, so each of
// the three orders falls back to the ID.
func TestQueryCardsPagesWithoutRepeatsWhenKeysTie(t *testing.T) {
	lib, _, _ := newLibrary(t)
	note, err := lib.SaveNote(core.Note{Title: "Same", Blocks: []core.Block{
		{Text: "a :: 1"}, {Text: "b :: 2"}, {Text: "c :: 3"}, {Text: "d :: 4"}, {Text: "e :: 5"},
	}})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	var want []string
	for _, block := range note.Blocks {
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		want = append(want, string(card.ID))
	}
	sort.Strings(want)

	for _, order := range []store.CardSort{store.CardSortCreated, store.CardSortDue, store.CardSortTitle} {
		t.Run(string(order), func(t *testing.T) {
			whole, total, err := lib.QueryCards(store.CardQuery{Sort: order})
			if err != nil {
				t.Fatalf("QueryCards: %v", err)
			}
			if total != len(want) || !slices.Equal(cardIDs(whole), want) {
				t.Fatalf("unpaged = %v of %d, want %v", cardIDs(whole), total, want)
			}

			paged, total := pageCards(t, lib, store.CardQuery{Sort: order}, 2)
			if total != len(want) || !slices.Equal(cardIDs(paged), want) {
				t.Fatalf("paged = %v of %d, want %v", cardIDs(paged), total, want)
			}
		})
	}
}

// TestQueryNotesFilters says what each note filter means: a tag is the note's,
// the text is its title or any block, and a filter that matches nothing is an
// empty page rather than an error.
func TestQueryNotesFilters(t *testing.T) {
	lib, _, clk := newLibrary(t)
	notes := []core.Note{
		{Title: "Biology", Blocks: []core.Block{{Text: "spacing beats cramming #study"}}},
		{Title: "Physics", Blocks: []core.Block{{Text: "entropy grows #study"}, {Text: "heat flows #thermo"}}},
		{Title: "Chemistry", Blocks: []core.Block{{Text: "bonds break #chem"}}},
	}
	for _, note := range notes {
		if _, err := lib.SaveNote(note); err != nil {
			t.Fatalf("SaveNote %q: %v", note.Title, err)
		}
		clk.advance(time.Hour)
	}

	tests := []struct {
		name  string
		query store.NoteQuery
		want  []string
	}{
		{"no filter", store.NoteQuery{}, []string{"Chemistry", "Physics", "Biology"}},
		{"a word in a block", store.NoteQuery{Text: "entropy"}, []string{"Physics"}},
		{"a word in a title", store.NoteQuery{Text: "chemis"}, []string{"Chemistry"}},
		{"a word nothing carries", store.NoteQuery{Text: "quantum"}, nil},
		{"a tag", store.NoteQuery{Tag: "study"}, []string{"Physics", "Biology"}},
		{"a tag nothing carries", store.NoteQuery{Tag: "nope"}, nil},
		{"a tag and a word in the same note", store.NoteQuery{Tag: "study", Text: "heat"}, []string{"Physics"}},
		{"a tag and a word from another note", store.NoteQuery{Tag: "study", Text: "bonds"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total, err := lib.QueryNotes(tt.query)
			if err != nil {
				t.Fatalf("QueryNotes: %v", err)
			}
			if total != len(tt.want) {
				t.Errorf("total = %d, want %d", total, len(tt.want))
			}
			if !slices.Equal(titles(got), tt.want) {
				t.Errorf("titles = %v, want %v", titles(got), tt.want)
			}
		})
	}
}

// TestQueryNotesSorts pins each order: the default is the newest save first, the
// title ignores case, and "created" is the oldest save first, which is the one
// moment a note's source carries.
func TestQueryNotesSorts(t *testing.T) {
	lib, _, clk := newLibrary(t)
	first, err := lib.SaveNote(core.Note{Title: "alpha", Blocks: []core.Block{{Text: "one"}}})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	clk.advance(time.Hour)
	if _, err := lib.SaveNote(core.Note{Title: "Bravo", Blocks: []core.Block{{Text: "two"}}}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	clk.advance(time.Hour)
	if _, err := lib.SaveNote(core.Note{Title: "charlie", Blocks: []core.Block{{Text: "three"}}}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	// The oldest note is saved again, so its last save is now the newest: the
	// three orders have to disagree.
	clk.advance(time.Hour)
	if _, err := lib.SaveNote(core.Note{ID: first.ID, Title: "alpha", Blocks: []core.Block{{Text: "one, again"}}}); err != nil {
		t.Fatalf("SaveNote again: %v", err)
	}

	tests := []struct {
		name  string
		order store.NoteSort
		want  []string
	}{
		{"the newest save first", store.NoteSortUpdated, []string{"alpha", "charlie", "Bravo"}},
		{"by title, ignoring case", store.NoteSortTitle, []string{"alpha", "Bravo", "charlie"}},
		{"the oldest save first", store.NoteSortCreated, []string{"Bravo", "charlie", "alpha"}},
		{"an order the index does not know", "", []string{"alpha", "charlie", "Bravo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := lib.QueryNotes(store.NoteQuery{Sort: tt.order})
			if err != nil {
				t.Fatalf("QueryNotes: %v", err)
			}
			if !slices.Equal(titles(got), tt.want) {
				t.Errorf("order = %v, want %v", titles(got), tt.want)
			}
		})
	}
}

// TestQueryNotesPageBoundaries is what a pager leans on: no limit is every
// match, an offset past the end is an empty page and not an error, and the total
// counts the matches rather than the page.
func TestQueryNotesPageBoundaries(t *testing.T) {
	lib, _, clk := newLibrary(t)
	for _, title := range []string{"A", "B", "C"} {
		if _, err := lib.SaveNote(core.Note{Title: title, Blocks: []core.Block{{Text: "x"}}}); err != nil {
			t.Fatalf("SaveNote: %v", err)
		}
		clk.advance(time.Minute)
	}

	tests := []struct {
		name  string
		query store.NoteQuery
		want  int
	}{
		{"no limit", store.NoteQuery{}, 3},
		{"a first page", store.NoteQuery{Limit: 2}, 2},
		{"the last page", store.NoteQuery{Limit: 2, Offset: 2}, 1},
		{"past the end", store.NoteQuery{Limit: 2, Offset: 10}, 0},
		{"past the end with no limit", store.NoteQuery{Offset: 10}, 0},
		{"a negative offset", store.NoteQuery{Limit: 2, Offset: -5}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total, err := lib.QueryNotes(tt.query)
			if err != nil {
				t.Fatalf("QueryNotes: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("page = %d notes, want %d", len(got), tt.want)
			}
			if total != 3 {
				t.Errorf("total = %d, want 3: the count is every match, not the page", total)
			}
		})
	}
}

// TestQueryCardsFilters checks the card filters against what a card list shows:
// the status of a reviewed card is the one the log gives it and not the "new"
// its file still says, the tag is the note's, and due is the replayed schedule —
// the card's creation when no review has scheduled it.
func TestQueryCardsFilters(t *testing.T) {
	lib, _, clk := newLibrary(t)
	studied, err := lib.SaveNote(core.Note{Title: "Studied", Blocks: []core.Block{
		{Text: "spacing :: beats cramming #study"},
		{Text: "interleaving :: helps too #study"},
	}})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	other, err := lib.SaveNote(core.Note{Title: "Other", Blocks: []core.Block{{Text: "heat :: flows #thermo"}}})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	cards := map[core.ID]core.Card{}
	for _, block := range []core.Block{studied.Blocks[0], studied.Blocks[1], other.Blocks[0]} {
		clk.advance(time.Minute)
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards[block.ID] = card
	}
	reviewed := cards[studied.Blocks[0].ID]
	if err := lib.Review(reviewed.ID, schedule.Good, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}
	edited := cards[studied.Blocks[1].ID]
	edited.Fields = map[string]string{"Front": "rewritten prompt"}
	edited.Edited = true
	if err := lib.SaveCard(edited); err != nil {
		t.Fatalf("SaveCard: %v", err)
	}

	created := func(block core.ID) string { return string(block) }
	fresh, suspended := core.StatusNew, core.StatusSuspended
	tests := []struct {
		name  string
		query store.CardQuery
		want  []string
	}{
		{"no filter", store.CardQuery{}, []string{
			created(studied.Blocks[0].ID), created(studied.Blocks[1].ID), created(other.Blocks[0].ID),
		}},
		{"a reviewed card is not new", store.CardQuery{Status: &fresh},
			[]string{created(studied.Blocks[1].ID), created(other.Blocks[0].ID)}},
		{"a status nothing is in", store.CardQuery{Status: &suspended}, nil},
		{"one note", store.CardQuery{Note: string(studied.ID)},
			[]string{created(studied.Blocks[0].ID), created(studied.Blocks[1].ID)}},
		{"a tag", store.CardQuery{Tag: "study"},
			[]string{created(studied.Blocks[0].ID), created(studied.Blocks[1].ID)}},
		{"a tag nothing carries", store.CardQuery{Tag: "nope"}, nil},
		{"a word in a block", store.CardQuery{Text: "cramming"}, []string{created(studied.Blocks[0].ID)}},
		{"a word nothing carries", store.CardQuery{Text: "quantum"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total, err := lib.QueryCards(tt.query)
			if err != nil {
				t.Fatalf("QueryCards: %v", err)
			}
			if total != len(tt.want) {
				t.Errorf("total = %d, want %d", total, len(tt.want))
			}
			blocks := make([]string, 0, len(got))
			for _, card := range got {
				blocks = append(blocks, card.Block)
			}
			if !slices.Equal(blocks, tt.want) {
				t.Errorf("blocks = %v, want %v", blocks, tt.want)
			}
		})
	}

	t.Run("a word in an edited field", func(t *testing.T) {
		got, total, err := lib.QueryCards(store.CardQuery{Text: "rewritten"})
		if err != nil {
			t.Fatalf("QueryCards: %v", err)
		}
		if total != 1 || len(got) != 1 || got[0].Block != string(studied.Blocks[1].ID) {
			t.Fatalf("QueryCards = %+v of %d, want the edited card", got, total)
		}
	})

	t.Run("due and status are what the log derived", func(t *testing.T) {
		got, _, err := lib.QueryCards(store.CardQuery{})
		if err != nil {
			t.Fatalf("QueryCards: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d cards, want 3", len(got))
		}
		if got[0].Status != core.StatusReview {
			t.Errorf("the reviewed card's status = %s, want review", got[0].Status)
		}
		if got[0].Type != string(core.TypeBasic) {
			t.Errorf("type = %q, want %q", got[0].Type, core.TypeBasic)
		}
		for _, card := range got {
			want := card.Created
			if state, ok := lib.CardState(core.ID(card.ID)); ok {
				want = state.Due
			}
			if !card.Due.Equal(want) {
				t.Errorf("card %s due %s, want %s", card.Block, card.Due, want)
			}
		}
	})
}

// TestQueryCardsDueOrderMatchesTheQueue is the promise that the listing and the
// queue agree: sorted by due, the cards come out in the order the review queue
// would serve them, because both read the schedule the same replay derived.
func TestQueryCardsDueOrderMatchesTheQueue(t *testing.T) {
	lib, _, clk := newLibrary(t)
	var cards []core.Card
	for range 5 {
		note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}}})
		if err != nil {
			t.Fatalf("SaveNote: %v", err)
		}
		card, err := firstCard(lib, note.Blocks[0].ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards = append(cards, card)
		clk.advance(time.Hour)
	}
	for _, rating := range []schedule.Rating{schedule.Good, schedule.Again} {
		if err := lib.Review(cards[0].ID, rating, t0); err != nil {
			t.Fatalf("Review: %v", err)
		}
	}

	listing, total, err := lib.QueryCards(store.CardQuery{Sort: store.CardSortDue})
	if err != nil {
		t.Fatalf("QueryCards: %v", err)
	}
	scheduled, err := lib.Scheduled()
	if err != nil {
		t.Fatalf("Scheduled: %v", err)
	}
	if total != len(scheduled) || len(listing) != len(scheduled) {
		t.Fatalf("listing has %d of %d, the queue %d", len(listing), total, len(scheduled))
	}
	for i := range listing {
		if listing[i].ID != string(scheduled[i].Card.ID) {
			t.Fatalf("position %d: the listing has %s, the queue %s", i, listing[i].ID, scheduled[i].Card.ID)
		}
	}
}

// pageNotes walks a note query a page at a time and answers what it saw with the
// total the index reported. The total is checked to be the same on every page: a
// total that moved with the offset would make a pager show rows that are not
// there.
func pageNotes(t *testing.T, lib *store.Library, q store.NoteQuery, size int) ([]store.NoteSummary, int) {
	t.Helper()
	var all []store.NoteSummary
	total := -1
	for offset := 0; ; offset += size {
		q.Limit, q.Offset = size, offset
		page, count, err := lib.QueryNotes(q)
		if err != nil {
			t.Fatalf("QueryNotes(%+v): %v", q, err)
		}
		if total >= 0 && count != total {
			t.Fatalf("the total moved from %d to %d at offset %d", total, count, offset)
		}
		total = count
		if len(page) == 0 {
			return all, total
		}
		all = append(all, page...)
	}
}

// pageCards is pageNotes for cards.
func pageCards(t *testing.T, lib *store.Library, q store.CardQuery, size int) ([]store.CardSummary, int) {
	t.Helper()
	var all []store.CardSummary
	total := -1
	for offset := 0; ; offset += size {
		q.Limit, q.Offset = size, offset
		page, count, err := lib.QueryCards(q)
		if err != nil {
			t.Fatalf("QueryCards(%+v): %v", q, err)
		}
		if total >= 0 && count != total {
			t.Fatalf("the total moved from %d to %d at offset %d", total, count, offset)
		}
		total = count
		if len(page) == 0 {
			return all, total
		}
		all = append(all, page...)
	}
}

func titles(notes []store.NoteSummary) []string {
	out := make([]string, 0, len(notes))
	for _, note := range notes {
		out = append(out, note.Title)
	}
	return out
}

func noteIDs(notes []store.NoteSummary) []string {
	out := make([]string, 0, len(notes))
	for _, note := range notes {
		out = append(out, string(note.ID))
	}
	return out
}

func cardIDs(cards []store.CardSummary) []string {
	out := make([]string, 0, len(cards))
	for _, card := range cards {
		out = append(out, card.ID)
	}
	return out
}
