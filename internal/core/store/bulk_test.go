package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// TestSuspendCardsTakesThemOutOfTheQueue is the batch suspend: each card file is
// rewritten with the status, the history stays where it was, and the card leaves
// the queue — which a reopened library and a rebuilt index agree on.
func TestSuspendCardsTakesThemOutOfTheQueue(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Studied", "a :: 1", "b :: 2", "c :: 3")
	var cards []core.Card
	for _, block := range note.Blocks {
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards = append(cards, card)
		clk.advance(time.Minute)
	}
	if err := lib.Review(cards[0].ID, schedule.Again, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}

	result, err := lib.SuspendCards([]core.ID{cards[0].ID, cards[1].ID}, clk.Now())
	if err != nil {
		t.Fatalf("SuspendCards: %v", err)
	}
	if result.Applied != 2 || result.Err() != nil {
		t.Fatalf("result = %+v (%v), want both cards written", result, result.Err())
	}
	for _, id := range []core.ID{cards[0].ID, cards[1].ID} {
		card, ok := lib.Card(id)
		if !ok {
			t.Fatalf("card %q is gone", id)
		}
		if card.Status != core.StatusSuspended {
			t.Errorf("card %q status = %s, want suspended", id, card.Status)
		}
	}
	// A suspension takes the card out of the queue and keeps its history.
	if state, ok := lib.CardState(cards[0].ID); !ok || state.Lapses != 1 || state.Reviews != 1 {
		t.Errorf("the suspended card's history = %+v, want the review and the lapse", state)
	}
	scheduled, err := lib.Scheduled()
	if err != nil {
		t.Fatalf("Scheduled: %v", err)
	}
	if len(scheduled) != 1 || scheduled[0].Card.ID != cards[2].ID {
		t.Errorf("queue = %+v, want the one card still in it", scheduled)
	}

	// The status is in the file, not only in memory.
	if body := string(read(t, cardPathOf(dir, cards[1]))); !strings.Contains(body, `"suspended"`) {
		t.Errorf("card file does not say suspended: %s", body)
	}

	t.Run("a reopened library and a rebuilt index agree", func(t *testing.T) {
		lib = reopen(t, lib, dir, clk)
		assertSuspended(t, lib, cards[0].ID, cards[1].ID)
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
		assertSuspended(t, lib, cards[0].ID, cards[1].ID)
	})
}

// assertSuspended checks the two suspended cards survived a reopen or a rebuild.
func assertSuspended(t *testing.T, lib *store.Library, ids ...core.ID) {
	t.Helper()
	for _, id := range ids {
		card, ok := lib.Card(id)
		if !ok {
			t.Fatalf("card %q is gone", id)
		}
		if card.Status != core.StatusSuspended {
			t.Errorf("card %q status = %s, want suspended", id, card.Status)
		}
	}
}

// TestFreezeCardsKeepsTheWords is the batch freeze: a card stops following its
// block and keeps what the block said when it was frozen, so a rewrite of the
// note cannot change a card the user has already learned from.
func TestFreezeCardsKeepsTheWords(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Studied", "a :: 1")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}

	result, err := lib.FreezeCards([]core.ID{card.ID}, clk.Now())
	if err != nil {
		t.Fatalf("FreezeCards: %v", err)
	}
	if result.Applied != 1 || result.Err() != nil {
		t.Fatalf("result = %+v (%v), want the card written", result, result.Err())
	}
	frozen, ok := lib.Card(card.ID)
	if !ok || frozen.Ref.Frozen == nil {
		t.Fatalf("card = %+v, want a snapshot", frozen)
	}
	if frozen.Ref.Frozen.Text != "a :: 1" {
		t.Errorf("snapshot = %q, want the block's text", frozen.Ref.Frozen.Text)
	}
	if !frozen.Ref.Frozen.At.Equal(clk.Now()) {
		t.Errorf("snapshot taken at %s, want %s", frozen.Ref.Frozen.At, clk.Now())
	}

	// The note is rewritten under the card: the card still says what it learned.
	clk.advance(time.Hour)
	if _, err := lib.SaveNote(core.Note{ID: note.ID, Title: note.Title,
		Blocks: []core.Block{{ID: note.Blocks[0].ID, Text: "a :: rewritten"}}}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if text, state := func() (string, core.LinkState) {
		card, _ := lib.Card(card.ID)
		return card.Ref.Text(lib)
	}(); state != core.LinkFrozen || text != "a :: 1" {
		t.Errorf("the frozen card shows %q as %s, want the snapshot", text, state)
	}

	t.Run("freezing again keeps the first snapshot", func(t *testing.T) {
		clk.advance(time.Hour)
		again, err := lib.FreezeCards([]core.ID{card.ID}, clk.Now())
		if err != nil || again.Applied != 1 {
			t.Fatalf("FreezeCards = %+v (%v), want it done", again, err)
		}
		card, _ := lib.Card(card.ID)
		if card.Ref.Frozen.Text != "a :: 1" || !card.Ref.Frozen.At.Equal(frozen.Ref.Frozen.At) {
			t.Errorf("snapshot = %+v, want the first one kept", card.Ref.Frozen)
		}
	})

	t.Run("a card whose block is gone is reported, not frozen", func(t *testing.T) {
		lost := saveNote(t, lib, "Gone", "x :: y")
		marked, err := firstCard(lib, lost.Blocks[0].ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		if _, err := lib.SaveNote(core.Note{ID: lost.ID, Title: lost.Title,
			Blocks: []core.Block{{Text: "x :: y, edited away"}}}); err != nil {
			t.Fatalf("SaveNote: %v", err)
		}
		result, err := lib.FreezeCards([]core.ID{marked.ID}, clk.Now())
		if err != nil {
			t.Fatalf("FreezeCards: %v", err)
		}
		if result.Applied != 0 || len(result.Failed) != 1 || result.Failed[0].ID != marked.ID {
			t.Fatalf("result = %+v, want the lost card reported", result)
		}
	})

	t.Run("a reopened library and a rebuilt index agree", func(t *testing.T) {
		lib = reopen(t, lib, dir, clk)
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
		card, ok := lib.Card(card.ID)
		if !ok || card.Ref.Frozen == nil || card.Ref.Frozen.Text != "a :: 1" {
			t.Errorf("the rebuilt card = %+v, want the frozen snapshot", card)
		}
	})
}

// TestSetDueMovesTheCardAndAReviewTakesItBack is the batch due date: the date the
// user chose is what the card table and the queue say, it survives a reopen and a
// rebuild, and answering the card puts it back on the schedule its answer derives.
func TestSetDueMovesTheCardAndAReviewTakesItBack(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Studied", "a :: 1", "b :: 2")
	reviewed, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	fresh, err := firstCard(lib, note.Blocks[1].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	if err := lib.Review(reviewed.ID, schedule.Good, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}
	scheduled := dueOf(t, lib, reviewed.ID)
	moved := scheduled.AddDate(0, 0, 30)

	result, err := lib.SetDue([]core.ID{reviewed.ID, fresh.ID}, moved)
	if err != nil {
		t.Fatalf("SetDue: %v", err)
	}
	if result.Applied != 2 || result.Err() != nil {
		t.Fatalf("result = %+v (%v), want both cards moved", result, result.Err())
	}
	if got := dueOf(t, lib, reviewed.ID); !got.Equal(moved) {
		t.Errorf("the reviewed card is due %s, want %s", got, moved)
	}
	if got := dueOf(t, lib, fresh.ID); !got.Equal(moved) {
		t.Errorf("the new card is due %s, want %s", got, moved)
	}

	t.Run("the queue follows the reviewed card", func(t *testing.T) {
		scheduled, err := lib.Scheduled()
		if err != nil {
			t.Fatalf("Scheduled: %v", err)
		}
		for _, entry := range scheduled {
			if entry.Card.ID == reviewed.ID && !entry.State.Due.Equal(moved) {
				t.Errorf("the queue has it due %s, want %s", entry.State.Due, moved)
			}
		}
	})

	// The reopen is here and not in a subtest: it hands back the library the
	// rest of this test reads, and a subtest's cleanup would close it.
	lib = reopen(t, lib, dir, clk)
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got := dueOf(t, lib, reviewed.ID); !got.Equal(moved) {
		t.Errorf("the rebuilt card is due %s, want the date the user set %s", got, moved)
	}

	t.Run("answering the card takes the date back", func(t *testing.T) {
		clk.advance(time.Hour)
		if err := lib.Review(reviewed.ID, schedule.Good, clk.Now()); err != nil {
			t.Fatalf("Review: %v", err)
		}
		got := dueOf(t, lib, reviewed.ID)
		if got.Equal(moved) {
			t.Fatalf("the card is still due %s after a review", moved)
		}
		if !got.After(clk.Now()) {
			t.Errorf("the answered card is due %s, want a date after the answer %s", got, clk.Now())
		}
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
		if rebuilt := dueOf(t, lib, reviewed.ID); !rebuilt.Equal(got) {
			t.Errorf("the rebuilt card is due %s, want %s", rebuilt, got)
		}
	})
}

// dueOf is when a card comes up, as the card table reports it.
func dueOf(t *testing.T, lib *store.Library, id core.ID) time.Time {
	t.Helper()
	rows, _, err := lib.QueryCards(store.CardQuery{})
	if err != nil {
		t.Fatalf("QueryCards: %v", err)
	}
	for _, row := range rows {
		if row.ID == id {
			return row.Due
		}
	}
	t.Fatalf("card %q is not in the listing", id)
	return time.Time{}
}

// TestAddTagToNotesWritesTheTag is the batch tag: the tag becomes one block of
// each note, the note's own text and its cards are untouched, and tagging a note
// that carries the tag already writes nothing.
func TestAddTagToNotesWritesTheTag(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	first := saveNote(t, lib, "First", "spacing :: beats cramming")
	second := saveNote(t, lib, "Second", "entropy :: grows")
	carded, err := firstCard(lib, first.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}

	result, err := lib.AddTagToNotes([]core.ID{first.ID, second.ID}, "#study")
	if err != nil {
		t.Fatalf("AddTagToNotes: %v", err)
	}
	if result.Applied != 2 || result.Err() != nil {
		t.Fatalf("result = %+v (%v), want both notes tagged", result, result.Err())
	}
	tagged, ok := lib.Note(first.ID)
	if !ok || len(tagged.Blocks) != 2 || tagged.Blocks[1].Text != "#study" {
		t.Fatalf("note = %+v, want the tag as its own block", tagged)
	}
	if tagged.Blocks[0].Text != "spacing :: beats cramming" {
		t.Errorf("the first block = %q, want the note's text untouched", tagged.Blocks[0].Text)
	}
	if got := lib.NotesWithTag("study"); !slices.Equal(sortedIDs(got...), sortedIDs(first.ID, second.ID)) {
		t.Errorf("notes carrying the tag = %v, want both", got)
	}
	// The tag is in the plaintext, where a person edits it.
	if body := string(read(t, notePath(dir, first))); !strings.Contains(body, "\n\n#study ^") {
		t.Errorf("note file does not carry the tag as its own paragraph: %q", body)
	}

	t.Run("tagging a tagged note writes nothing", func(t *testing.T) {
		clk.advance(time.Hour)
		before := read(t, notePath(dir, first))
		result, err := lib.AddTagToNotes([]core.ID{first.ID}, "study")
		if err != nil || result.Applied != 1 || result.Err() != nil {
			t.Fatalf("AddTagToNotes = %+v (%v), want it done and unchanged", result, err)
		}
		if after := read(t, notePath(dir, first)); string(after) != string(before) {
			t.Errorf("the note file changed: %q → %q", before, after)
		}
	})

	t.Run("a card's text is not tagged", func(t *testing.T) {
		card, ok := lib.Card(carded.ID)
		if !ok {
			t.Fatalf("card %q is gone", carded.ID)
		}
		text, state := card.Ref.Text(lib)
		if state != core.LinkLive || strings.Contains(text, "study") {
			t.Errorf("the card shows %q as %s, want the paragraph without the tag", text, state)
		}
	})

	t.Run("a rebuilt index agrees", func(t *testing.T) {
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
		if got := lib.NotesWithTag("study"); !slices.Equal(sortedIDs(got...), sortedIDs(first.ID, second.ID)) {
			t.Errorf("rebuilt tag listing = %v, want both notes", got)
		}
	})
}

// TestBulkActionsReportEveryFailure is the batch's contract: an id that names
// nothing, or whose file cannot be written, is reported with the rest still
// written — a batch is not one write, and it is not a silent partial success.
func TestBulkActionsReportEveryFailure(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Studied", "a :: 1", "b :: 2")
	first, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	second, err := firstCard(lib, note.Blocks[1].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	unknown := core.ID("card_00000000000000000000000000000000")

	t.Run("an id that names nothing", func(t *testing.T) {
		result, err := lib.SuspendCards([]core.ID{unknown, first.ID}, clk.Now())
		if err != nil {
			t.Fatalf("SuspendCards: %v", err)
		}
		if result.Applied != 1 || len(result.Failed) != 1 || result.Failed[0].ID != unknown {
			t.Fatalf("result = %+v, want one written and one reported", result)
		}
		if result.Err() == nil {
			t.Fatal("Err() = nil, want the failure summarized")
		}
	})

	t.Run("a card file that cannot be written", func(t *testing.T) {
		// The card's file is a directory, so the atomic rename has nowhere to go.
		path := cardPathOf(dir, second)
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove %s: %v", path, err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		result, err := lib.SuspendCards([]core.ID{second.ID, first.ID}, clk.Now())
		if err != nil {
			t.Fatalf("SuspendCards: %v", err)
		}
		if result.Applied != 1 || len(result.Failed) != 1 || result.Failed[0].ID != second.ID {
			t.Fatalf("result = %+v, want the unwritable card reported", result)
		}
		// The card that could be written really was: a failure is not a rollback
		// of the others, and it is not a claim they failed either.
		if card, _ := lib.Card(first.ID); card.Status != core.StatusSuspended {
			t.Errorf("the writable card = %+v, want it suspended", card)
		}
	})
}

// TestAddTagToNotesRejectsWhatIsNotATag is the batch's one fatal refusal: a tag
// that is not one tag is nobody's write, so nothing is written.
func TestAddTagToNotesRejectsWhatIsNotATag(t *testing.T) {
	lib, _, _ := newLibrary(t)
	note := saveNote(t, lib, "Plain", "a paragraph")
	for _, tag := range []string{"", "#", "  ", "a b", "study!", "two#tags"} {
		if _, err := lib.AddTagToNotes([]core.ID{note.ID}, tag); err == nil {
			t.Errorf("AddTagToNotes(%q) = nil error, want a refusal", tag)
		}
	}
	if got, _ := lib.Note(note.ID); len(got.Blocks) != 1 {
		t.Errorf("the note holds %d blocks, want the one it started with", len(got.Blocks))
	}
}

// TestBulkTagsAndNotesReportPerID is the tag batch's report: a note id and a card
// id in one call, one written and the other named.
func TestBulkTagsAndNotesReportPerID(t *testing.T) {
	lib, _, _ := newLibrary(t)
	note := saveNote(t, lib, "Plain", "a paragraph")
	card, err := firstCard(lib, saveNote(t, lib, "Carded", "x :: y").Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}

	result, err := lib.AddTagToNotes([]core.ID{note.ID, card.ID}, "study")
	if err != nil {
		t.Fatalf("AddTagToNotes: %v", err)
	}
	if result.Applied != 1 || len(result.Failed) != 1 || result.Failed[0].ID != card.ID {
		t.Fatalf("result = %+v, want the note tagged and the card id reported", result)
	}
	if got := lib.NotesWithTag("study"); !slices.Equal(got, []core.ID{note.ID}) {
		t.Errorf("notes carrying the tag = %v, want only the note", got)
	}
}

// cardPathOf is where a card's definition lives in the library.
func cardPathOf(dir string, card core.Card) string {
	return filepath.Join(dir, "cards", string(card.ID)+".json")
}

// TestChangeCardTypesKeepsTheSharedFields is the batch type change: the cards
// render through the type chosen, the values the new schema still names come with
// them, and the ones it does not are dropped — a card never carries a field its
// type cannot show.
func TestChangeCardTypesKeepsTheSharedFields(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Studied", "a :: 1", "b :: 2")
	edited, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	plain, err := firstCard(lib, note.Blocks[1].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	// What the user typed into the two fields, kept the way a card keeps them.
	edited.Fields = map[string]string{"Front": "question", "Back": "answer"}
	edited.Edited = true
	if err := lib.SaveCard(edited); err != nil {
		t.Fatalf("SaveCard: %v", err)
	}

	// Basic and bidirectional share their two names, so both values stay.
	result, err := lib.ChangeCardTypes([]core.ID{edited.ID, plain.ID}, core.TypeBidirectional)
	if err != nil {
		t.Fatalf("ChangeCardTypes: %v", err)
	}
	if result.Applied != 2 || result.Err() != nil {
		t.Fatalf("result = %+v (%v), want both cards written", result, result.Err())
	}
	moved, ok := lib.Card(edited.ID)
	if !ok {
		t.Fatal("the card is gone")
	}
	if moved.Type != core.TypeBidirectional {
		t.Errorf("type = %s, want the type chosen", moved.Type)
	}
	if moved.Fields["Front"] != "question" || moved.Fields["Back"] != "answer" || !moved.Edited {
		t.Errorf("fields = %+v (edited %v), want the values the new type names", moved.Fields, moved.Edited)
	}

	t.Run("a field the new type does not name is dropped", func(t *testing.T) {
		// The cloze type asks one field, so the two the card held have nowhere
		// to go and the card resolves its text from the block again.
		result, err := lib.ChangeCardTypes([]core.ID{edited.ID}, core.TypeCloze)
		if err != nil || result.Applied != 1 {
			t.Fatalf("ChangeCardTypes = %+v (%v), want it done", result, err)
		}
		card, _ := lib.Card(edited.ID)
		if card.Type != core.TypeCloze {
			t.Errorf("type = %s, want cloze", card.Type)
		}
		if len(card.Fields) != 0 || card.Edited {
			t.Errorf("fields = %+v (edited %v), want none of the old schema", card.Fields, card.Edited)
		}
	})

	t.Run("a type the library does not hold fails every id", func(t *testing.T) {
		result, err := lib.ChangeCardTypes([]core.ID{edited.ID, plain.ID}, "remit.missing")
		if err != nil {
			t.Fatalf("ChangeCardTypes: %v", err)
		}
		if result.Applied != 0 || len(result.Failed) != 2 {
			t.Fatalf("result = %+v, want both ids reported", result)
		}
		if !errors.Is(result.Err(), store.ErrNoType) {
			t.Errorf("err = %v, want it to name the missing type", result.Err())
		}
	})

	t.Run("the type is in the file, not only in memory", func(t *testing.T) {
		if body := string(read(t, cardPathOf(dir, plain))); !strings.Contains(body, string(core.TypeBidirectional)) {
			t.Errorf("card file does not name the type: %s", body)
		}
	})

	// The reopen is here and not in a subtest: a subtest's cleanup would close
	// the library the rest of this test reads.
	lib = reopen(t, lib, dir, clk)
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	card, ok := lib.Card(edited.ID)
	if !ok || card.Type != core.TypeCloze || len(card.Fields) != 0 {
		t.Errorf("the rebuilt card = %+v, want the cloze type and no fields", card)
	}
}

// TestBuryCardsSetsTheSiblingsDueTomorrow is the batch bury: the other cards of
// the paragraphs chosen come back tomorrow, the chosen ones keep the date they
// had, and no review is written — burying is a due date, not a second history.
func TestBuryCardsSetsTheSiblingsDueTomorrow(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Blanks", "one {{a}} and {{b}}", "elsewhere :: 1")
	// Two cards on one paragraph, as a cloze makes: the sibling pair a bury is
	// about.
	created, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{
		{Type: core.TypeCloze, Ordinal: 0},
		{Type: core.TypeCloze, Ordinal: 1},
	})
	if err != nil {
		t.Fatalf("CreateCards: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("the paragraph made %d cards, want 2", len(created))
	}
	chosen, sibling := created[0], created[1]
	other, err := firstCard(lib, note.Blocks[1].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	if err := lib.Review(chosen.ID, schedule.Good, t0.Add(-24*time.Hour)); err != nil {
		t.Fatalf("Review: %v", err)
	}
	// The sibling is due now, which is what a bury is for: it is in the queue and
	// the user is about to meet the same paragraph's other card.
	if err := lib.Review(sibling.ID, schedule.Again, t0.Add(-24*time.Hour)); err != nil {
		t.Fatalf("Review: %v", err)
	}
	before := dueOf(t, lib, chosen.ID)
	events := len(lib.Events())
	siblingState, ok := lib.CardState(sibling.ID)
	if !ok || siblingState.Reviews == 0 {
		t.Fatalf("the sibling has no schedule: %+v", siblingState)
	}
	siblingReviews := siblingState.Reviews

	until := t0.AddDate(0, 0, 1)
	result, err := lib.BuryCards([]core.ID{chosen.ID}, until)
	if err != nil {
		t.Fatalf("BuryCards: %v", err)
	}
	if result.Applied != 1 || result.Err() != nil {
		t.Fatalf("result = %+v (%v), want the one sibling written", result, result.Err())
	}
	if got := dueOf(t, lib, sibling.ID); !got.Equal(until) {
		t.Errorf("the sibling is due %s, want %s", got, until)
	}
	if got := dueOf(t, lib, chosen.ID); !got.Equal(before) {
		t.Errorf("the chosen card is due %s, want the date it had %s", got, before)
	}
	if got := dueOf(t, lib, other.ID); got.Equal(until) {
		t.Errorf("a card of another paragraph was buried: due %s", got)
	}
	// A due date is not a review: the log holds what it held.
	if got := len(lib.Events()); got != events {
		t.Errorf("the log holds %d events, want the %d it had", got, events)
	}
	// The sibling's own history is untouched, and it leaves today's queue.
	if state, ok := lib.CardState(sibling.ID); !ok || state.Reviews != siblingReviews {
		t.Errorf("the buried sibling has %+v, want the %d reviews it had", state, siblingReviews)
	}
	if due, err := lib.Due(t0, 0); err != nil {
		t.Fatalf("Due: %v", err)
	} else {
		for _, card := range due {
			if card.ID == sibling.ID {
				t.Error("the buried sibling is still in today's queue")
			}
		}
	}

	// The reopen is here and not in a subtest: it hands back the library the
	// rest of this test reads, and a subtest's cleanup would close it.
	lib = reopen(t, lib, dir, clk)
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got := dueOf(t, lib, sibling.ID); !got.Equal(until) {
		t.Errorf("the rebuilt sibling is due %s, want %s", got, until)
	}

	t.Run("a sibling already due after the bury is left alone", func(t *testing.T) {
		clk.advance(48 * time.Hour)
		if err := lib.Review(sibling.ID, schedule.Good, clk.Now()); err != nil {
			t.Fatalf("Review: %v", err)
		}
		later := dueOf(t, lib, sibling.ID)
		if !later.After(until) {
			t.Fatalf("the reviewed sibling is due %s, want a date past %s", later, until)
		}
		result, err := lib.BuryCards([]core.ID{chosen.ID}, until)
		if err != nil {
			t.Fatalf("BuryCards: %v", err)
		}
		if result.Applied != 0 {
			t.Errorf("result = %+v, want nothing written over a later date", result)
		}
		if got := dueOf(t, lib, sibling.ID); !got.Equal(later) {
			t.Errorf("the sibling is due %s, want its own date %s kept", got, later)
		}
	})

	t.Run("a card id that names nothing is reported", func(t *testing.T) {
		result, err := lib.BuryCards([]core.ID{"card_00000000000000000000000000000000"}, until)
		if err != nil {
			t.Fatalf("BuryCards: %v", err)
		}
		if result.Applied != 0 || len(result.Failed) != 1 {
			t.Fatalf("result = %+v, want the unknown id reported", result)
		}
	})
}

// TestDeleteCardsKeepsTheReviews is the batch delete: the card files go, the
// review log stays, and an id that names nothing is reported instead of taking
// the rest of the batch with it.
func TestDeleteCardsKeepsTheReviews(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Studied", "a :: 1", "b :: 2", "c :: 3")
	var cards []core.Card
	for _, block := range note.Blocks {
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		cards = append(cards, card)
	}
	if err := lib.Review(cards[0].ID, schedule.Again, t0); err != nil {
		t.Fatalf("Review: %v", err)
	}
	events := len(lib.Events())
	unknown := core.ID("card_00000000000000000000000000000000")

	result, err := lib.DeleteCards([]core.ID{cards[0].ID, unknown, cards[1].ID})
	if err != nil {
		t.Fatalf("DeleteCards: %v", err)
	}
	if result.Applied != 2 || len(result.Failed) != 1 || result.Failed[0].ID != unknown {
		t.Fatalf("result = %+v, want the two cards deleted and the unknown id reported", result)
	}
	for _, id := range []core.ID{cards[0].ID, cards[1].ID} {
		if _, ok := lib.Card(id); ok {
			t.Errorf("card %q is still readable", id)
		}
		if _, ok := lib.CardState(id); ok {
			t.Errorf("card %q still has a schedule", id)
		}
	}
	if _, ok := lib.Card(cards[2].ID); !ok {
		t.Error("the card nobody chose went with them")
	}
	history, err := lib.History(cards[0].ID)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 1 || history[0].Rating != schedule.Again {
		t.Errorf("history = %+v, want the review the log keeps", history)
	}
	if got := len(lib.Events()); got != events {
		t.Errorf("the log holds %d events, want the %d it had", got, events)
	}
	// The card file is the truth: with it gone the card stays gone.
	if _, err := os.Stat(cardPathOf(dir, cards[0])); !os.IsNotExist(err) {
		t.Errorf("the deleted card's file is still there: %v", err)
	}

	// The reopen is here and not in a subtest: a subtest's cleanup would close
	// the library the rest of this test reads.
	lib = reopen(t, lib, dir, clk)
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if _, ok := lib.Card(cards[0].ID); ok {
		t.Error("the deleted card came back with the rebuilt index")
	}
	if all := lib.Cards(); len(all) != 1 || all[0].ID != cards[2].ID {
		t.Errorf("the library holds %+v, want the one card left", all)
	}
	if history, err := lib.History(cards[0].ID); err != nil || len(history) != 1 {
		t.Errorf("the rebuilt history = %+v (%v), want the review kept", history, err)
	}
}
