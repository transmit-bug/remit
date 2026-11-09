package store_test

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// The production numbers are what the index counts, so they answer on a rebuilt
// index exactly as on a live one: no note file is read either way.
func TestVolumeCountsTextCardsAndExcerpts(t *testing.T) {
	lib, _, _ := newLibrary(t)
	source := saveNote(t, lib, "Source", "记忆", "beta")
	// A note that only mentions another note is not an excerpt of it.
	saveNote(t, lib, "Mention", "see [[Source]] for more")
	var cards []core.Card
	for _, block := range source.Blocks {
		cards = append(cards, makeCard(t, lib, block.ID))
	}
	child, err := lib.Excerpt(source.ID, source.Blocks[0].ID, "记忆", "摘自 Source")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}

	// The characters are the notes' block text, runes not bytes: "记忆" is two.
	want := store.Volume{Characters: 2 + 4 + len([]rune("see [[Source]] for more")) + len([]rune(child.Blocks[0].Text)),
		Notes: 1, Excerpts: 1, Cards: len(cards)}
	for _, check := range []struct {
		name string
		lib  *store.Library
	}{
		{"live index", lib},
		{"rebuilt index", rebuildIndex(t, lib)},
	} {
		got, err := check.lib.Volume()
		if err != nil {
			t.Fatalf("%s: Volume: %v", check.name, err)
		}
		if got != want {
			t.Errorf("%s: Volume = %+v, want %+v", check.name, got, want)
		}
	}

	// The mention note still carries no card, so it is not one of the notes the
	// cards come from; its text is counted all the same.
	if got, _ := lib.Volume(); got.Notes != 1 {
		t.Errorf("%+v counts the mention note as a source of cards", got)
	}

	// An anchor to the note's own block is not an excerpt of it.
	own := saveNote(t, lib, "Self", "own text")
	own.Blocks[0].Text = "own [[Self#^" + string(own.Blocks[0].ID) + "]]"
	if _, err := lib.SaveNote(own); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if got, _ := lib.Volume(); got.Excerpts != 1 {
		t.Errorf("%+v counts a self-reference as an excerpt", got)
	}
}

// The index keeps one stamp per note, its last save, so an excerpt is priced on
// the day it was last written — which is the day it was taken until it is edited.
func TestExcerptsBetweenUsesTheSaveStamp(t *testing.T) {
	lib, _, clk := newLibrary(t)
	source := saveNote(t, lib, "Source", "a paragraph")
	child, err := lib.Excerpt(source.ID, source.Blocks[0].ID, "a paragraph", "摘自 Source")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}

	clk.advance(24 * time.Hour)
	if _, err := lib.SaveNote(child); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	for _, tt := range []struct {
		name string
		from time.Time
		to   time.Time
		want int
	}{
		{"the day it was taken", t0, t0.Add(24 * time.Hour), 0},
		{"the day it was rewritten", t0.Add(24 * time.Hour), t0.Add(48 * time.Hour), 1},
		{"open bounds", time.Time{}, time.Time{}, 1},
		{"after the save", t0.Add(48 * time.Hour), time.Time{}, 0},
	} {
		got, err := lib.ExcerptsBetween(tt.from, tt.to)
		if err != nil {
			t.Fatalf("%s: ExcerptsBetween: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("%s: ExcerptsBetween = %d, want %d", tt.name, got, tt.want)
		}
	}
}

// makeCard makes one basic card for a block, which is all this test needs.
func makeCard(t *testing.T, lib *store.Library, blockID core.ID) core.Card {
	t.Helper()
	cards, err := lib.CreateCards(blockID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		t.Fatalf("CreateCards: %v", err)
	}
	return cards[0]
}

// rebuildIndex drops the derived index and builds it again from the text
// sources, which is what a caller's numbers must not depend on.
func rebuildIndex(t *testing.T, lib *store.Library) *store.Library {
	t.Helper()
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	return lib
}
