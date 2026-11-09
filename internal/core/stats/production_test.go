package stats

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Production is what the index counts, so each library answers for what was
// written into it: characters are block text (runes, not bytes), and a note is
// one of the notes the cards come from, not every note there is.
func TestProductionCounts(t *testing.T) {
	for _, tt := range []struct {
		name  string
		build func(t *testing.T) (*store.Library, Production)
	}{
		{"an empty library", func(t *testing.T) (*store.Library, Production) {
			return emptyLibrary(t), Production{}
		}},
		{"reading only", func(t *testing.T) (*store.Library, Production) {
			lib := emptyLibrary(t)
			writeNote(t, lib, "read", "记忆")
			return lib, Production{Characters: 2} // two runes, six bytes
		}},
		{"cards only", func(t *testing.T) (*store.Library, Production) {
			lib := emptyLibrary(t)
			note := writeNote(t, lib, "n", "q :: a", "q2 :: a2")
			for _, block := range note.Blocks {
				if _, err := firstCard(lib, block.ID); err != nil {
					t.Fatalf("card: %v", err)
				}
			}
			return lib, Production{Characters: 6 + 8, Notes: 1, Cards: 2}
		}},
		{"an excerpt with a card", func(t *testing.T) (*store.Library, Production) {
			lib := emptyLibrary(t)
			source := writeNote(t, lib, "source", "alpha")
			if _, err := firstCard(lib, source.Blocks[0].ID); err != nil {
				t.Fatalf("card: %v", err)
			}
			child, err := lib.Excerpt(source.ID, source.Blocks[0].ID, "alpha", "摘自 source")
			if err != nil {
				t.Fatalf("Excerpt: %v", err)
			}
			if _, err := firstCard(lib, child.Blocks[0].ID); err != nil {
				t.Fatalf("card: %v", err)
			}
			return lib, Production{
				Characters:      5 + len([]rune(child.Blocks[0].Text)),
				Notes:           2, // the source and the excerpt both carry a card
				Excerpts:        1,
				Cards:           2,
				CardsPerExcerpt: 2,
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib, want := tt.build(t)
			if got := ProductionOf(lib); got != want {
				t.Errorf("Production = %+v, want %+v", got, want)
			}
		})
	}
}

// The time share is one bar over a range: reading, reviewing and excerpting are
// priced from one live log (the index's save stamp for the excerpt, which the
// log does not record), and an undone answer costs nothing.
func TestTimeShareOfSumsTheRange(t *testing.T) {
	lib := emptyLibrary(t)
	// Two sessions on one note: six minutes each.
	note := writeNote(t, lib, "long read", "a paragraph")
	for i := 0; i < 2; i++ {
		if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, day0.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("ReadNote: %v", err)
		}
	}
	// Being queued is intent, not a sitting: it costs no minutes.
	queued := writeNote(t, lib, "queued", "a paragraph")
	if err := lib.AddToReading(queued.ID, day0); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}

	// Four answers today, one of them undone, and one answer tomorrow.
	cardNote := writeNote(t, lib, "cards", "q :: a", "q2 :: a2")
	var cards []core.Card
	for _, block := range cardNote.Blocks {
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards = append(cards, card)
	}
	if err := lib.Review(cards[0].ID, schedule.Good, day0.Add(2*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.UndoLast(cards[0].ID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	for i, rating := range []schedule.Rating{schedule.Good, schedule.Hard, schedule.Again, schedule.Easy} {
		if err := lib.Review(cards[i%2].ID, rating, day0.Add(time.Duration(3+i)*time.Hour)); err != nil {
			t.Fatalf("review: %v", err)
		}
	}
	if err := lib.Review(cards[0].ID, schedule.Good, day0.Add(30*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}

	if _, err := lib.Excerpt(cardNote.ID, cardNote.Blocks[0].ID, "q", "摘自 cards"); err != nil {
		t.Fatalf("Excerpt: %v", err)
	}

	for _, tt := range []struct {
		name string
		from time.Time
		to   time.Time
		want TimeShare
	}{
		// 2 sessions × 6 min, 4 answers × 0.6 min, 1 excerpt × 2 min.
		{"today", day0, day0.Add(24 * time.Hour), TimeShare{Read: 12, Review: 2, Excerpt: 2}},
		{"tomorrow", day0.Add(24 * time.Hour), day0.Add(48 * time.Hour), TimeShare{Review: 1}},
		{"the whole log", time.Time{}, time.Time{}, TimeShare{Read: 12, Review: 3, Excerpt: 2}},
	} {
		got := TimeShareOf(lib, tt.from, tt.to)
		if got != tt.want {
			t.Errorf("%s: TimeShare = %+v, want %+v", tt.name, got, tt.want)
		}
		// The three segments are the bar's shares: no fourth line, no minute
		// twice.
		if sum := got.Read + got.Review + got.Excerpt; sum != tt.want.Read+tt.want.Review+tt.want.Excerpt {
			t.Errorf("%s: the shares add up to %d minutes, want %d", tt.name, sum,
				tt.want.Read+tt.want.Review+tt.want.Excerpt)
		}
	}
}

// A library with nothing in it costs nothing.
func TestTimeShareOfAnEmptyLibrary(t *testing.T) {
	if got := TimeShareOf(emptyLibrary(t), day0, day0.Add(24*time.Hour)); got != (TimeShare{}) {
		t.Errorf("TimeShare of an empty library = %+v", got)
	}
}

// emptyLibrary is a library with no notes, cards or reviews, on the test's clock
// so every save lands on day0.
func emptyLibrary(t *testing.T) *store.Library {
	t.Helper()
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return day0 }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return lib
}

// writeNote saves a note made of the given block texts.
func writeNote(t *testing.T, lib *store.Library, title string, texts ...string) core.Note {
	t.Helper()
	blocks := make([]core.Block, len(texts))
	for i, text := range texts {
		blocks[i] = core.Block{Text: text}
	}
	note, err := lib.SaveNote(core.Note{Title: title, Blocks: blocks})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	return note
}
