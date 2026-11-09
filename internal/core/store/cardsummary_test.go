package store_test

import (
	"slices"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// TestQueryCardsDescribesEachRow is what the card table shows per card beyond its
// text: the note it came from, where its text comes from, its lapses and the
// interval it is on.
func TestQueryCardsDescribesEachRow(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Studied", "a :: 1", "b :: 2", "c :: 3", "d :: 4")
	cards := map[core.ID]core.Card{}
	for _, block := range note.Blocks {
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards[block.ID] = card
		clk.advance(time.Minute)
	}
	// Yesterday's card answered twice "again": a hard card with two lapses.
	lapsed := cards[note.Blocks[0].ID]
	if err := lib.Review(lapsed.ID, schedule.Again, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if err := lib.Review(lapsed.ID, schedule.Again, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}
	// Another answered well: one review, a longer interval, no lapse.
	studied := cards[note.Blocks[1].ID]
	if err := lib.Review(studied.ID, schedule.Easy, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}
	// One frozen, one whose block is about to be edited away.
	if _, err := lib.FreezeCards([]core.ID{cards[note.Blocks[2].ID].ID}, clk.Now()); err != nil {
		t.Fatalf("FreezeCards: %v", err)
	}
	if _, err := lib.SaveNote(core.Note{ID: note.ID, Title: note.Title, Blocks: note.Blocks[:3]}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	rows, total, err := lib.QueryCards(store.CardQuery{Sort: store.CardSortCreated})
	if err != nil {
		t.Fatalf("QueryCards: %v", err)
	}
	if total != 4 || len(rows) != 4 {
		t.Fatalf("rows = %d of %d, want the four cards", len(rows), total)
	}
	byBlock := map[core.ID]store.CardSummary{}
	for _, row := range rows {
		byBlock[row.Block] = row
	}

	tests := []struct {
		name     string
		block    core.ID
		ref      core.LinkState
		lapses   int
		interval bool    // whether the card is on an interval at all
		note     core.ID // empty for a card whose block no note holds any more
	}{
		{"a card answered again twice", note.Blocks[0].ID, core.LinkLive, 2, true, note.ID},
		{"a card answered well", note.Blocks[1].ID, core.LinkLive, 0, true, note.ID},
		{"a frozen card", note.Blocks[2].ID, core.LinkFrozen, 0, false, note.ID},
		{"a card whose block was edited away", note.Blocks[3].ID, core.LinkLost, 0, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, ok := byBlock[tt.block]
			if !ok {
				t.Fatalf("no row for block %q", tt.block)
			}
			if row.Ref != tt.ref {
				t.Errorf("ref = %s, want %s", row.Ref, tt.ref)
			}
			if row.Lapses != tt.lapses {
				t.Errorf("lapses = %d, want %d", row.Lapses, tt.lapses)
			}
			if row.Note != tt.note {
				t.Errorf("note = %q, want %q", row.Note, tt.note)
			}
			if row.Type != core.TypeBasic {
				t.Errorf("type = %q, want %q", row.Type, core.TypeBasic)
			}
			if onInterval := row.IntervalDays > 0; onInterval != tt.interval {
				t.Errorf("interval = %d days, want an interval: %v", row.IntervalDays, tt.interval)
			}
			if tt.interval && !row.Due.After(row.Created) {
				t.Errorf("due %s is not after the review that scheduled it", row.Due)
			}
		})
	}

	if got := byBlock[note.Blocks[0].ID].IntervalDays; got > byBlock[note.Blocks[1].ID].IntervalDays {
		t.Errorf("the again-answered card is on %d days and the easy one on %d: want the easy one longer",
			got, byBlock[note.Blocks[1].ID].IntervalDays)
	}

	t.Run("a rebuilt index answers the same", func(t *testing.T) {
		before, _, err := lib.QueryCards(store.CardQuery{})
		if err != nil {
			t.Fatalf("QueryCards: %v", err)
		}
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
		after, _, err := lib.QueryCards(store.CardQuery{})
		if err != nil {
			t.Fatalf("QueryCards after a rebuild: %v", err)
		}
		if !slices.Equal(before, after) {
			t.Errorf("rebuilt rows = %+v, want %+v", after, before)
		}
	})
}

// TestQueryCardsFiltersTypeRefAndLapses says what the new filters keep: a type, a
// reference state, and the hard cards a preset's threshold names.
func TestQueryCardsFiltersTypeRefAndLapses(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Mixed", "cloze {{one}} text", "a :: 1", "b :: 2")
	cloze, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Type: core.TypeCloze}})
	if err != nil {
		t.Fatalf("CreateCards: %v", err)
	}
	basic, err := firstCard(lib, note.Blocks[1].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	clk.advance(time.Minute)
	other, err := firstCard(lib, note.Blocks[2].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	for range 3 {
		if err := lib.Review(other.ID, schedule.Again, t0); err != nil {
			t.Fatalf("Review: %v", err)
		}
	}
	if _, err := lib.FreezeCards([]core.ID{cloze[0].ID}, clk.Now()); err != nil {
		t.Fatalf("FreezeCards: %v", err)
	}

	live, frozen, lost := core.LinkLive, core.LinkFrozen, core.LinkLost
	tests := []struct {
		name  string
		query store.CardQuery
		want  []core.ID
	}{
		{"by type", store.CardQuery{Type: core.TypeCloze}, []core.ID{cloze[0].ID}},
		{"a type no card has", store.CardQuery{Type: core.TypeBidirectional}, nil},
		{"live sources", store.CardQuery{Ref: &live}, []core.ID{basic.ID, other.ID}},
		{"frozen sources", store.CardQuery{Ref: &frozen}, []core.ID{cloze[0].ID}},
		{"lost sources", store.CardQuery{Ref: &lost}, nil},
		{"one lapse is enough for hard", store.CardQuery{MinLapses: 1}, []core.ID{other.ID}},
		{"three lapses exclude the hard card", store.CardQuery{MinLapses: 4}, nil},
		{"a hard card of one type", store.CardQuery{MinLapses: 1, Type: core.TypeCloze}, nil},
		{"a hard rule beside a reference", store.CardQuery{MinLapses: 1, Ref: &live}, []core.ID{other.ID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, total, err := lib.QueryCards(tt.query)
			if err != nil {
				t.Fatalf("QueryCards: %v", err)
			}
			got := make([]core.ID, 0, len(rows))
			for _, row := range rows {
				got = append(got, row.ID)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("cards = %v, want %v", got, tt.want)
			}
			if total != len(tt.want) {
				t.Errorf("total = %d, want %d", total, len(tt.want))
			}
		})
	}

	t.Run("a note named by its title", func(t *testing.T) {
		rows, _, err := lib.QueryCards(store.CardQuery{Note: note.Title})
		if err != nil {
			t.Fatalf("QueryCards: %v", err)
		}
		if len(rows) != 3 {
			t.Errorf("cards = %d, want the note's three", len(rows))
		}
	})
}

// TestQueryCardsSortsIntervalAndLapses pins the two new orders: the longest
// interval first, and the cards answered "again" most often first.
func TestQueryCardsSortsIntervalAndLapses(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Studied", "a :: 1", "b :: 2", "c :: 3")
	cards := make(map[core.ID]core.Card, len(note.Blocks))
	for _, block := range note.Blocks {
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards[block.ID] = card
		clk.advance(time.Minute)
	}
	if err := lib.Review(cards[note.Blocks[0].ID].ID, schedule.Easy, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if err := lib.Review(cards[note.Blocks[1].ID].ID, schedule.Good, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}
	for range 2 {
		if err := lib.Review(cards[note.Blocks[2].ID].ID, schedule.Again, t0); err != nil {
			t.Fatalf("Review: %v", err)
		}
	}

	tests := []struct {
		name string
		sort store.CardSort
		want []core.ID // by block
	}{
		{
			name: "the longest interval first",
			sort: store.CardSortInterval,
			want: []core.ID{note.Blocks[0].ID, note.Blocks[1].ID, note.Blocks[2].ID},
		},
		{
			name: "the most lapses first",
			sort: store.CardSortLapses,
			want: []core.ID{note.Blocks[2].ID, note.Blocks[0].ID, note.Blocks[1].ID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, total, err := lib.QueryCards(store.CardQuery{Sort: tt.sort})
			if err != nil {
				t.Fatalf("QueryCards: %v", err)
			}
			if total != 3 || len(rows) != 3 {
				t.Fatalf("rows = %d of %d, want 3", len(rows), total)
			}
			if got := rows[0].Block; got != tt.want[0] {
				t.Errorf("first card is %q, want %q", got, tt.want[0])
			}
			if tt.sort == store.CardSortInterval && !slices.Equal(blockIDs(rows), tt.want) {
				t.Errorf("interval order = %v, want %v", blockIDs(rows), tt.want)
			}
			if tt.sort == store.CardSortLapses && rows[1].Lapses != 0 && rows[2].Lapses != 0 {
				t.Errorf("the cards without lapses are not last: %+v", rows)
			}
		})
	}
}

// blockIDs is the blocks a page of cards renders from, in order.
func blockIDs(cards []store.CardSummary) []core.ID {
	out := make([]core.ID, 0, len(cards))
	for _, card := range cards {
		out = append(out, card.Block)
	}
	return out
}
