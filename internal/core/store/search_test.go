package store_test

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// searchLibrary is a library whose notes make one hit of each shape: a title
// match, a body that says the word again and again, an exact phrase, and
// Chinese, which has no word boundary to search for. Two of the blocks carry a
// card, so a search has both kinds to find.
func searchLibrary(t *testing.T) (*store.Library, string, map[string]core.Note, map[core.ID]core.Card) {
	t.Helper()
	lib, dir, _ := newLibrary(t)
	notes := map[string]core.Note{}
	for _, note := range []core.Note{
		{Title: "Entropy", Blocks: []core.Block{{Text: "the second law"}}},
		{Title: "Body", Blocks: []core.Block{{Text: "entropy entropy entropy"}}},
		{Title: "Phrase", Blocks: []core.Block{{Text: "entropy always increases"}, {Text: "spacing beats cramming :: yes"}}},
		{Title: "Apart", Blocks: []core.Block{{Text: "always the entropy of a body"}}},
		{Title: "中文笔记", Blocks: []core.Block{{Text: "你好世界很重要"}}},
	} {
		saved, err := lib.SaveNote(note)
		if err != nil {
			t.Fatalf("SaveNote %q: %v", note.Title, err)
		}
		notes[note.Title] = saved
	}
	cards := map[core.ID]core.Card{}
	for _, block := range []core.Block{
		notes["Body"].Blocks[0],
		notes["Phrase"].Blocks[1],
	} {
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards[block.ID] = card
	}
	return lib, dir, notes, cards
}

// hitIDs is the set of what a search answered, so two queries can be compared
// for finding the same rows whatever their order.
func hitIDs(hits []store.SearchHit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.Kind+":"+string(hit.ID))
	}
	sort.Strings(out)
	return out
}

func hitTitles(hits []store.SearchHit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.Title)
	}
	return out
}

// TestSearchAnswersShortQueriesOverTheSameRows is the trigram's price: a query
// under three runes cannot be looked up in the index, so it is answered by a
// LIKE scan. The two paths must find the same rows, in Chinese as in English —
// a short query answered by a different corpus would make a search flicker as
// the user types the third character.
func TestSearchAnswersShortQueriesOverTheSameRows(t *testing.T) {
	lib, _, _, _ := searchLibrary(t)

	tests := []struct{ long, short string }{
		{"entropy", "ent"},
		{"entropy", "en"},
		{"你好世界", "你好"},
		{"你好世界", "你"},
	}
	for _, tt := range tests {
		t.Run(tt.short, func(t *testing.T) {
			long, _, err := lib.Search(tt.long, "", 0, 0)
			if err != nil {
				t.Fatalf("Search(%q): %v", tt.long, err)
			}
			short, total, err := lib.Search(tt.short, "", 0, 0)
			if err != nil {
				t.Fatalf("Search(%q): %v", tt.short, err)
			}
			if len(long) == 0 {
				t.Fatalf("Search(%q) found nothing", tt.long)
			}
			if total != len(short) {
				t.Errorf("total = %d, the page holds %d", total, len(short))
			}
			if got, want := hitIDs(short), hitIDs(long); !slices.Equal(got, want) {
				t.Errorf("Search(%q) found %v, Search(%q) found %v", tt.short, got, tt.long, want)
			}
		})
	}
}

// TestSearchRanksATitleAboveABody: a note called "Entropy" is about entropy
// before one that repeats the word in a paragraph, so the title match comes
// first. Without the column weight the repeated body would win.
func TestSearchRanksATitleAboveABody(t *testing.T) {
	lib, _, _, _ := searchLibrary(t)
	hits, total, err := lib.Search("entropy", "note", 0, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total != 4 || len(hits) != 4 {
		t.Fatalf("found %d of %d, want the four notes", len(hits), total)
	}
	if hits[0].Title != "Entropy" {
		t.Fatalf("order = %v, want the title match first", hitTitles(hits))
	}
	for i := 1; i < len(hits); i++ {
		if hits[i].Rank > hits[i-1].Rank {
			t.Fatalf("ranks went up at position %d: %v", i, hitTitles(hits))
		}
	}
}

// TestSearchMatchesAPhrase: a query is the user's words in sequence, not the
// words anywhere. A note that happens to hold both words apart is not an answer.
func TestSearchMatchesAPhrase(t *testing.T) {
	lib, _, _, _ := searchLibrary(t)

	hits, total, err := lib.Search("entropy always", "note", 0, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total != 1 || len(hits) != 1 || hits[0].Title != "Phrase" {
		t.Fatalf("found %v of %d, want only the note where the words meet", hitTitles(hits), total)
	}
	if !strings.Contains(hits[0].Snippet, "[") || !strings.Contains(hits[0].Snippet, "]") {
		t.Errorf("snippet %q does not mark the match", hits[0].Snippet)
	}

	if hits, total, err := lib.Search("always entropy", "note", 0, 0); err != nil || total != 0 || len(hits) != 0 {
		t.Fatalf("reversed phrase found %v of %d (%v), want nothing", hitTitles(hits), total, err)
	}
}

// TestSearchHitCarriesItsPlace: a hit says what it is and where it lives. A card
// hit names the card, the block it renders and the note that block is in; a note
// hit names no block. Getting this from the wrong row would send a reader to the
// wrong place.
func TestSearchHitCarriesItsPlace(t *testing.T) {
	lib, _, notes, made := searchLibrary(t)
	phrase := notes["Phrase"]
	block := phrase.Blocks[1]
	card := made[block.ID]

	hits, total, err := lib.Search("cramming", "card", 0, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total != 1 || len(hits) != 1 {
		t.Fatalf("found %d of %d card hits, want one", len(hits), total)
	}
	hit := hits[0]
	if hit.Kind != "card" || hit.ID != card.ID || hit.Block != block.ID || hit.Note != phrase.ID {
		t.Errorf("card hit = %+v, want the card %q of note %q at block %q", hit, card.ID, phrase.ID, block.ID)
	}
	if hit.Title != "Phrase" {
		t.Errorf("card hit title = %q, want the note's %q", hit.Title, "Phrase")
	}
	if !strings.Contains(hit.Snippet, "[cramm") {
		t.Errorf("snippet = %q, want the match marked", hit.Snippet)
	}
	if hit.Rank <= 0 {
		t.Errorf("rank = %v, want a positive score", hit.Rank)
	}

	notesHit, _, err := lib.Search("cramming", "note", 0, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(notesHit) != 1 || notesHit[0].Kind != "note" || notesHit[0].Note != phrase.ID {
		t.Fatalf("note hits = %+v, want the Phrase note", notesHit)
	}
	if notesHit[0].Block != "" {
		t.Errorf("a note hit names block %q, want none", notesHit[0].Block)
	}
}

// TestSearchKindsAndPages: kind narrows the answer to one table, "" is both, and
// a page reads as a page — the second one continues the first and the total does
// not move with the offset.
func TestSearchKindsAndPages(t *testing.T) {
	lib, _, _, _ := searchLibrary(t)

	for _, kind := range []string{"note", "card"} {
		hits, _, err := lib.Search("entropy", kind, 0, 0)
		if err != nil {
			t.Fatalf("Search(%q): %v", kind, err)
		}
		if len(hits) == 0 {
			t.Fatalf("Search(%q) found nothing", kind)
		}
		for _, hit := range hits {
			if hit.Kind != kind {
				t.Fatalf("Search(%q) answered a %q hit", kind, hit.Kind)
			}
		}
	}
	both, _, err := lib.Search("entropy", "", 0, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	var kinds []string
	for _, hit := range both {
		kinds = append(kinds, hit.Kind)
	}
	if !slices.Contains(kinds, "note") || !slices.Contains(kinds, "card") {
		t.Fatalf("kinds = %v, want both in one answer", kinds)
	}

	t.Run("pages", func(t *testing.T) {
		var paged []store.SearchHit
		total := -1
		for offset := 0; ; offset += 3 {
			page, count, err := lib.Search("entropy", "", 3, offset)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if total >= 0 && count != total {
				t.Fatalf("the total moved from %d to %d at offset %d", total, count, offset)
			}
			total = count
			if len(page) == 0 {
				break
			}
			paged = append(paged, page...)
		}
		if total != len(both) || len(paged) != len(both) {
			t.Fatalf("paged %d of %d, unpaged %d", len(paged), total, len(both))
		}
		for i := range both {
			if paged[i].Kind != both[i].Kind || paged[i].ID != both[i].ID {
				t.Fatalf("position %d: paged %v, unpaged %v", i, paged[i], both[i])
			}
		}
	})

	t.Run("past the end", func(t *testing.T) {
		hits, total, err := lib.Search("entropy", "", 5, 100)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(hits) != 0 {
			t.Errorf("page = %d hits, want none", len(hits))
		}
		if total < 1 {
			t.Errorf("total = %d, want every match regardless of the page", total)
		}
	})

	t.Run("a page wide enough to be read in one statement", func(t *testing.T) {
		// A page is read by its rowids until it is too wide for that to be
		// cheaper; asking for far more rows than the library holds takes the
		// other path, and it has to answer in the same order.
		wide, total, err := lib.Search("entropy", "", 5000, 0)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if total != len(both) || len(wide) != len(both) {
			t.Fatalf("wide page %d of %d, the paged answer %d", len(wide), total, len(both))
		}
		for i := range both {
			if wide[i].Kind != both[i].Kind || wide[i].ID != both[i].ID || wide[i].Snippet != both[i].Snippet {
				t.Fatalf("position %d: wide %+v, paged %+v", i, wide[i], both[i])
			}
		}
	})

	t.Run("an empty query", func(t *testing.T) {
		hits, total, err := lib.Search("   ", "", 10, 0)
		if err != nil || len(hits) != 0 || total != 0 {
			t.Errorf("Search(blank) = %d of %d (%v), want nothing and no error", len(hits), total, err)
		}
	})

	t.Run("a kind the search does not have", func(t *testing.T) {
		if _, _, err := lib.Search("entropy", "attachment", 10, 0); err == nil {
			t.Error("Search with an unknown kind answered, want an error")
		}
	})
}

// TestSearchFollowsEveryWrite: the search table is written in the same
// transaction as the rows it mirrors, so a note or a card that was just saved is
// found by what it now says and not by what it said before.
func TestSearchFollowsEveryWrite(t *testing.T) {
	lib, _, _ := newLibrary(t)
	count := func(t *testing.T, text string, want int) {
		t.Helper()
		hits, total, err := lib.Search(text, "", 0, 0)
		if err != nil {
			t.Fatalf("Search(%q): %v", text, err)
		}
		if len(hits) != want || total != want {
			t.Fatalf("Search(%q) = %d hits of %d, want %d", text, len(hits), total, want)
		}
	}

	note, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{{Text: "spacing beats cramming"}}})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	count(t, "spacing", 1)
	count(t, "interleaving", 0)

	replaced, err := lib.SaveNote(core.Note{ID: note.ID, Title: "Biology", Blocks: []core.Block{{Text: "interleaving wins"}}})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	count(t, "spacing", 0)
	count(t, "interleaving", 1)

	card, err := firstCard(lib, replaced.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	count(t, "interleaving", 2) // the note and the card that renders its block

	card.Fields = map[string]string{"Front": "rewritten prompt"}
	card.Edited = true
	if err := lib.SaveCard(card); err != nil {
		t.Fatalf("SaveCard: %v", err)
	}
	count(t, "rewritten", 1)

	card.Fields = nil
	card.Edited = false
	if err := lib.SaveCard(card); err != nil {
		t.Fatalf("SaveCard: %v", err)
	}
	count(t, "rewritten", 0)

	if err := lib.DeleteCard(card.ID); err != nil {
		t.Fatalf("DeleteCard: %v", err)
	}
	count(t, "interleaving", 1) // the note alone

	// A card made from a block after the note was saved finds the block's text.
	if _, err := lib.CreateCards(replaced.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}}); err != nil {
		t.Fatalf("CreateCards: %v", err)
	}
	count(t, "interleaving", 2)
}

// TestSearchSurvivesARebuiltIndex: the index is derived and may be thrown away,
// so a rebuild and a deleted .remit/ must answer exactly what the index did —
// same hits, same order, same snippets and scores. A search table that the
// rebuild did not fill would answer nothing, which is what this catches.
func TestSearchSurvivesARebuiltIndex(t *testing.T) {
	lib, dir, notes, cards := searchLibrary(t)
	edited := cards[notes["Phrase"].Blocks[1].ID]
	edited.Fields = map[string]string{"Front": "rewritten cramming"}
	edited.Edited = true
	if err := lib.SaveCard(edited); err != nil {
		t.Fatalf("SaveCard: %v", err)
	}

	queries := []string{"entropy", "entropy always", "cramming", "中文", "你好世界", "en"}
	before := make(map[string][]store.SearchHit, len(queries))
	for _, query := range queries {
		hits, total, err := lib.Search(query, "", 0, 0)
		if err != nil {
			t.Fatalf("Search(%q): %v", query, err)
		}
		if len(hits) != total || total == 0 {
			t.Fatalf("Search(%q) = %d hits of %d", query, len(hits), total)
		}
		before[query] = hits
	}

	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	for _, query := range queries {
		hits, _, err := lib.Search(query, "", 0, 0)
		if err != nil {
			t.Fatalf("Search(%q) after a rebuild: %v", query, err)
		}
		assertSameHits(t, query+" after a rebuild", hits, before[query])
	}

	// The index is a cache: deleting it is not a loss.
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".remit")); err != nil {
		t.Fatalf("remove index: %v", err)
	}
	again, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer again.Close()
	for _, query := range queries {
		hits, _, err := again.Search(query, "", 0, 0)
		if err != nil {
			t.Fatalf("Search(%q) after deleting the index: %v", query, err)
		}
		assertSameHits(t, query+" after deleting the index", hits, before[query])
	}
}

// assertSameHits compares two answers field by field: the same rows in the same
// order, with the same text and the same score.
func assertSameHits(t *testing.T, name string, got, want []store.SearchHit) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d hits, want %d", name, len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s at position %d:\n got %+v\nwant %+v", name, i, got[i], want[i])
		}
	}
}

// TestSearchReportsTheLibraryClosed: a closed handle has no index to search, and
// says so instead of answering nothing.
func TestSearchReportsTheLibraryClosed(t *testing.T) {
	lib, _, _ := newLibrary(t)
	if _, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "entropy"}}}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, _, err := lib.Search("entropy", "", 10, 0); err != store.ErrClosed {
		t.Errorf("Search on a closed library = %v, want %v", err, store.ErrClosed)
	}
	if _, _, err := lib.QueryNotes(store.NoteQuery{}); err != store.ErrClosed {
		t.Errorf("QueryNotes on a closed library = %v, want %v", err, store.ErrClosed)
	}
	if _, _, err := lib.QueryCards(store.CardQuery{}); err != store.ErrClosed {
		t.Errorf("QueryCards on a closed library = %v, want %v", err, store.ErrClosed)
	}
}
