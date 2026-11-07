package store_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

var t0 = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

// clock is the library's time, so a test reads the same on any machine.
type clock struct{ at time.Time }

func (c *clock) Now() time.Time { return c.at }

func (c *clock) advance(d time.Duration) { c.at = c.at.Add(d) }

func newLibrary(t *testing.T) (*store.Library, string, *clock) {
	t.Helper()
	dir := t.TempDir()
	clk := &clock{at: t0}
	lib, err := store.Create(dir, store.Options{Now: clk.Now})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib, dir, clk
}

// reopen closes a library and opens it again from the same directory.
func reopen(t *testing.T, lib *store.Library, dir string, clk *clock) *store.Library {
	t.Helper()
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	next, err := store.Open(dir, store.Options{Now: clk.Now})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { next.Close() })
	return next
}

func saveNote(t *testing.T, lib *store.Library, title string, texts ...string) core.Note {
	t.Helper()
	note, err := lib.CreateNote(title)
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	for _, text := range texts {
		note.Blocks = append(note.Blocks, core.Block{Text: text})
	}
	saved, err := lib.SaveNote(note)
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	return saved
}

func notePath(dir string, note core.Note) string {
	slug := strings.ToLower(strings.ReplaceAll(note.Title, " ", "-"))
	return filepath.Join(dir, "notes", slug+"-"+string(note.ID)+".md")
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func sameState(a, b schedule.State) bool {
	return a.Reviews == b.Reviews &&
		a.Memory == b.Memory &&
		a.Due.Equal(b.Due) &&
		a.LastReview.Equal(b.LastReview)
}

func ids(cards []core.Card) []core.ID {
	out := make([]core.ID, len(cards))
	for i, card := range cards {
		out[i] = card.ID
	}
	return out
}

func TestConfigIsWritten(t *testing.T) {
	_, dir, _ := newLibrary(t)

	var config struct {
		Format string `json:"format"`
		FSRS   struct {
			Weights          []float64 `json:"weights"`
			DesiredRetention float64   `json:"desired_retention"`
		} `json:"fsrs"`
	}
	if err := json.Unmarshal(read(t, filepath.Join(dir, "remit.json")), &config); err != nil {
		t.Fatalf("remit.json: %v", err)
	}
	if config.Format != "remit/2" {
		t.Errorf("format = %q, want remit/2", config.Format)
	}
	if len(config.FSRS.Weights) != 21 {
		t.Errorf("weights = %d, want 21", len(config.FSRS.Weights))
	}
	if config.FSRS.DesiredRetention != schedule.DefaultRetention {
		t.Errorf("desired retention = %v, want %v", config.FSRS.DesiredRetention, schedule.DefaultRetention)
	}
	for _, name := range []string{"notes", "cards", "types", ".remit"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("library has no %s directory: %v", name, err)
		}
	}
}

func TestNoteRoundTrip(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	note := saveNote(t, lib, "My First Note",
		"what is a block? :: a piece of a note",
		"an unmarked paragraph",
	)

	// The file is named after the slug minted at creation.
	path := notePath(dir, note)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("note file %s: %v", path, err)
	}

	// Every block is referencable and the marker is not part of its text.
	for _, block := range note.Blocks {
		if block.ID.Kind() != "block" || !block.ID.Valid() {
			t.Errorf("block ID %q is not a block ID", block.ID)
		}
		if strings.Contains(block.Text, "^block_") {
			t.Errorf("block text kept its marker: %q", block.Text)
		}
	}

	stored, ok := lib.Note(note.ID)
	if !ok {
		t.Fatalf("note %q was not read", note.ID)
	}
	if stored.Title != "My First Note" || len(stored.Blocks) != 2 {
		t.Fatalf("note read back as %+v", stored)
	}
	if stored.Blocks[0] != note.Blocks[0] || stored.Blocks[1] != note.Blocks[1] {
		t.Errorf("blocks read back as %+v, want %+v", stored.Blocks, note.Blocks)
	}
	block, ok := lib.Block(note.Blocks[1].ID)
	if !ok || block != note.Blocks[1] {
		t.Errorf("Block(%q) = (%+v, %v)", note.Blocks[1].ID, block, ok)
	}

	// A title change keeps the path and every block as it was.
	body := strings.SplitN(string(read(t, path)), "\n\n", 2)
	if len(body) != 2 || !strings.HasPrefix(body[0], "# ") {
		t.Fatalf("note file has no title line:\n%s", string(read(t, path)))
	}
	stored.Title = "Renamed"
	if _, err := lib.SaveNote(stored); err != nil {
		t.Fatalf("SaveNote after rename: %v", err)
	}
	after := strings.SplitN(string(read(t, path)), "\n\n", 2)
	if len(after) != 2 {
		t.Fatalf("renamed note lost its body:\n%s", string(read(t, path)))
	}
	if after[0] != "# Renamed" {
		t.Errorf("title line = %q, want %q", after[0], "# Renamed")
	}
	if after[1] != body[1] {
		t.Errorf("blocks changed on rename:\n%q\n%q", body[1], after[1])
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("rename moved the file: %v", err)
	}
}

func TestNoteFileFormat(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantTitle  string
		wantBlocks []core.Block
	}{
		{
			name:       "unmarked blocks have no ID",
			source:     "plain one\n\nplain two\n",
			wantBlocks: []core.Block{{Text: "plain one"}, {Text: "plain two"}},
		},
		{
			name:      "title and markers",
			source:    "# Today\n\nfirst ^block_11111111111111111111111111111111\n\nsecond ^block_22222222222222222222222222222222\n",
			wantTitle: "Today",
			wantBlocks: []core.Block{
				{ID: "block_11111111111111111111111111111111", Text: "first"},
				{ID: "block_22222222222222222222222222222222", Text: "second"},
			},
		},
		{
			name:       "marker on its own line",
			source:     "two lines\nof text\n^block_33333333333333333333333333333333\n",
			wantBlocks: []core.Block{{ID: "block_33333333333333333333333333333333", Text: "two lines\nof text"}},
		},
		{
			name:   "a marker ends a block without a blank line",
			source: "marked ^block_44444444444444444444444444444444\nafter\n",
			wantBlocks: []core.Block{
				{ID: "block_44444444444444444444444444444444", Text: "marked"},
				{Text: "after"},
			},
		},
		{
			name:       "a malformed marker is text",
			source:     "keep ^block_nothex\n",
			wantBlocks: []core.Block{{Text: "keep ^block_nothex"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			lib, err := store.Create(dir, store.Options{Now: func() time.Time { return t0 }})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			defer lib.Close()

			// A note written by hand, as a user would leave it.
			id := core.NewID("note")
			path := filepath.Join(dir, "notes", "hand-"+string(id)+".md")
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatalf("write note: %v", err)
			}
			if err := lib.Rebuild(); err != nil {
				t.Fatalf("Rebuild: %v", err)
			}

			note, ok := lib.Note(id)
			if !ok {
				t.Fatalf("note %q was not read", id)
			}
			if note.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", note.Title, tt.wantTitle)
			}
			if len(note.Blocks) != len(tt.wantBlocks) {
				t.Fatalf("blocks = %+v, want %+v", note.Blocks, tt.wantBlocks)
			}
			for i, want := range tt.wantBlocks {
				if note.Blocks[i] != want {
					t.Errorf("block %d = %+v, want %+v", i, note.Blocks[i], want)
				}
			}
		})
	}
}

func TestCreateCard(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	note := saveNote(t, lib, "Card Making", "what :: that")
	block := note.Blocks[0]

	card, err := firstCard(lib, block.ID)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if card.Status != core.StatusNew {
		t.Errorf("a fresh card is %v, want new", card.Status)
	}
	if card.Ref.Block != block.ID {
		t.Errorf("card references %q, want %q", card.Ref.Block, block.ID)
	}

	if card.Type != core.TypeBasic || card.Ordinal != 0 {
		t.Errorf("the card is %v/%d, want the paragraph's first", card.Type, card.Ordinal)
	}

	// The marker is in the note source, and the card file exists.
	if !strings.Contains(string(read(t, notePath(dir, note))), "^"+string(block.ID)) {
		t.Errorf("note source does not carry the block marker")
	}
	cardData := read(t, filepath.Join(dir, "cards", string(card.ID)+".json"))
	var file map[string]any
	if err := json.Unmarshal(cardData, &file); err != nil {
		t.Fatalf("card file: %v", err)
	}
	for _, key := range []string{"block", "type", "status", "created"} {
		if _, ok := file[key]; !ok {
			t.Errorf("card file has no %q", key)
		}
	}
	if file["block"] != string(block.ID) || file["type"] != string(core.TypeBasic) || file["status"] != "new" {
		t.Errorf("card file = %s", cardData)
	}

	// Reading it back gives the same card.
	reread := reopen(t, lib, dir, &clock{at: t0})
	got, ok := reread.Card(card.ID)
	if !ok || got.Ref.Block != block.ID || got.Status != core.StatusNew {
		t.Fatalf("Card after reopen = (%+v, %v)", got, ok)
	}

	// The marker is appended to the block and nowhere else: drop it by hand,
	// let CreateCard put it back, and the file comes out byte for byte.
	path := notePath(dir, note)
	source := read(t, path)
	unmarked := bytes.Replace(source, []byte(" ^"+string(block.ID)), nil, 1)
	if bytes.Equal(unmarked, source) {
		t.Fatal("test could not remove the marker")
	}
	if err := os.WriteFile(path, unmarked, 0o644); err != nil {
		t.Fatalf("write unmarked note: %v", err)
	}
	if _, err := firstCard(reread, block.ID); err != nil {
		t.Fatalf("making a card without a marker: %v", err)
	}
	if got := read(t, path); !bytes.Equal(got, source) {
		t.Errorf("marking rewrote more than the marker:\n%q\nwant\n%q", got, source)
	}
}

func TestReviewAndReopen(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Review Me", "q :: a")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	due, err := lib.Due(clk.Now(), 0)
	if len(due) != 1 || due[0].ID != card.ID {
		t.Fatalf("a new card is not due: %v (%v)", ids(due), err)
	}
	if _, ok := lib.CardState(card.ID); ok {
		t.Error("a new card already has a schedule")
	}

	if err := lib.Review(card.ID, schedule.Good, clk.Now()); err != nil {
		t.Fatalf("Review: %v", err)
	}
	reviewed, ok := lib.CardState(card.ID)
	if !ok {
		t.Fatal("no state after a review")
	}
	if reviewed.Reviews != 1 || !reviewed.LastReview.Equal(t0) || !reviewed.Due.After(t0) {
		t.Errorf("state after one review = %+v", reviewed)
	}
	if due, _ := lib.Due(clk.Now(), 0); len(due) != 0 {
		t.Errorf("a card reviewed for tomorrow is still due: %v", ids(due))
	}
	if got, ok := lib.Card(card.ID); !ok || got.Status != core.StatusReview {
		t.Errorf("card status after a Good review = %v, want review", got.Status)
	}

	// The review survives a close: the log is the whole of the schedule.
	clk.advance(48 * time.Hour)
	reopened := reopen(t, lib, dir, clk)
	state, ok := reopened.CardState(card.ID)
	if !ok || !sameState(state, reviewed) {
		t.Errorf("replayed state = (%+v, %v), want %+v", state, ok, reviewed)
	}
	remaining, err := reopened.Remaining(clk.Now())
	if err != nil || remaining != 1 {
		t.Errorf("Remaining = (%d, %v), want 1", remaining, err)
	}
	due, err = reopened.Due(clk.Now(), 0)
	if err != nil || len(due) != 1 || due[0].ID != card.ID {
		t.Fatalf("Due after the interval = (%v, %v)", ids(due), err)
	}
}

func TestUndoRestoresTheState(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Undo", "q :: a")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if err := lib.UndoLast(card.ID); !errors.Is(err, store.ErrNoReview) {
		t.Fatalf("UndoLast on a new card = %v, want ErrNoReview", err)
	}

	if err := lib.Review(card.ID, schedule.Good, clk.Now()); err != nil {
		t.Fatalf("Review: %v", err)
	}
	afterFirst, _ := lib.CardState(card.ID)

	clk.advance(72 * time.Hour)
	if err := lib.Review(card.ID, schedule.Again, clk.Now()); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if state, _ := lib.CardState(card.ID); sameState(state, afterFirst) {
		t.Fatal("the second review changed nothing")
	}

	if err := lib.UndoLast(card.ID); err != nil {
		t.Fatalf("UndoLast: %v", err)
	}
	if state, _ := lib.CardState(card.ID); !sameState(state, afterFirst) {
		t.Errorf("state after undo = %+v, want %+v", state, afterFirst)
	}
	if got, ok := lib.Card(card.ID); !ok || got.Status != core.StatusReview {
		t.Errorf("status after undo = %v, want review", got.Status)
	}

	// The log is append-only: the review is still there, named by a void.
	lines := strings.Split(strings.TrimSpace(string(read(t, filepath.Join(dir, "reviews.jsonl")))), "\n")
	if len(lines) != 3 {
		t.Fatalf("log has %d lines, want 3:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	var void struct {
		Seq  int64 `json:"seq"`
		Void int64 `json:"void"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &void); err != nil {
		t.Fatalf("void line: %v", err)
	}
	if void.Void != 2 || void.Seq != 3 {
		t.Errorf("void = %+v, want seq 3 cancelling 2", void)
	}

	// Undoing the last review leaves the card as new again.
	if err := lib.UndoLast(card.ID); err != nil {
		t.Fatalf("second UndoLast: %v", err)
	}
	if _, ok := lib.CardState(card.ID); ok {
		t.Error("a card with every review voided still has a schedule")
	}
	if got, ok := lib.Card(card.ID); !ok || got.Status != core.StatusNew {
		t.Errorf("status = %v, want new", got.Status)
	}
	if err := lib.UndoLast(card.ID); !errors.Is(err, store.ErrNoReview) {
		t.Errorf("UndoLast with nothing left = %v, want ErrNoReview", err)
	}
}

func TestLostCardKeepsItsSchedule(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Orphans", "keep me :: for now")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if err := lib.Review(card.ID, schedule.Good, clk.Now()); err != nil {
		t.Fatalf("Review: %v", err)
	}
	before, _ := lib.CardState(card.ID)

	// The user deletes the paragraph the card was made from.
	note.Blocks = nil
	if _, err := lib.SaveNote(note); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	if _, ok := lib.Block(card.Ref.Block); ok {
		t.Error("the deleted block is still addressable")
	}
	lost, err := lib.LostCards()
	if err != nil || len(lost) != 1 || lost[0].ID != card.ID {
		t.Fatalf("LostCards = (%v, %v), want the orphan", ids(lost), err)
	}
	if _, state := lost[0].Ref.Text(lib); state != core.LinkLost {
		t.Errorf("a lost card renders as %v, want lost", state)
	}
	if state, _ := lib.CardState(card.ID); !sameState(state, before) {
		t.Errorf("losing its block changed the schedule: %+v, want %+v", state, before)
	}
	if due, _ := lib.Due(clk.Now().Add(24*time.Hour), 0); len(due) != 0 {
		t.Errorf("a lost card is in the review queue: %v", ids(due))
	}

	// None of this is in the index alone: a rebuild finds the same orphan.
	reopened := reopen(t, lib, dir, clk)
	if err := reopened.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	lost, err = reopened.LostCards()
	if err != nil || len(lost) != 1 || lost[0].ID != card.ID {
		t.Fatalf("LostCards after rebuild = (%v, %v)", ids(lost), err)
	}
	if state, _ := reopened.CardState(card.ID); !sameState(state, before) {
		t.Errorf("state after rebuild = %+v, want %+v", state, before)
	}
}

func TestDerivedIndexRebuilds(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	var cards []core.Card
	for i, rating := range []schedule.Rating{schedule.Good, schedule.Again, schedule.Easy} {
		note := saveNote(t, lib, "Indexed", "q :: a")
		card, err := firstCard(lib, note.Blocks[0].ID)
		if err != nil {
			t.Fatalf("CreateCard: %v", err)
		}
		clk.advance(time.Duration(i) * time.Hour)
		if err := lib.Review(card.ID, rating, clk.Now()); err != nil {
			t.Fatalf("Review: %v", err)
		}
		cards = append(cards, card)
	}

	future := t0.AddDate(1, 0, 0)
	wantDue, err := lib.Due(future, 0)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(wantDue) != len(cards) {
		t.Fatalf("Due = %v, want every card", ids(wantDue))
	}
	wantRemaining, err := lib.Remaining(future)
	if err != nil || wantRemaining != len(cards) {
		t.Fatalf("Remaining = (%d, %v)", wantRemaining, err)
	}
	wantStates := map[core.ID]schedule.State{}
	for _, card := range cards {
		wantStates[card.ID], _ = lib.CardState(card.ID)
	}
	for i := 1; i < len(wantDue); i++ {
		previous, _ := wantStates[wantDue[i-1].ID]
		next, _ := wantStates[wantDue[i].ID]
		if next.Due.Before(previous.Due) {
			t.Fatalf("queue is not soonest first: %v then %v", previous.Due, next.Due)
		}
	}

	// The index is derived: throw it away and the library comes back the same.
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".remit")); err != nil {
		t.Fatalf("remove index: %v", err)
	}
	opened, err := store.Open(dir, store.Options{Now: clk.Now})
	if err != nil {
		t.Fatalf("Open without an index: %v", err)
	}
	defer opened.Close()

	gotDue, err := opened.Due(future, 0)
	if err != nil {
		t.Fatalf("Due after reopen: %v", err)
	}
	if len(gotDue) != len(wantDue) {
		t.Fatalf("Due after reopen = %v, want %v", ids(gotDue), ids(wantDue))
	}
	for i := range wantDue {
		if gotDue[i].ID != wantDue[i].ID {
			t.Errorf("due order changed at %d: %v, want %v", i, ids(gotDue), ids(wantDue))
			break
		}
	}
	for id, want := range wantStates {
		if got, _ := opened.CardState(id); !sameState(got, want) {
			t.Errorf("state of %q = %+v, want %+v", id, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".remit", "index.sqlite")); err != nil {
		t.Errorf("reopening did not rebuild the index: %v", err)
	}

	// The same holds through an explicit rebuild on the open library.
	if err := opened.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	again, err := opened.Due(future, 0)
	if err != nil || len(ids(again)) != len(wantDue) {
		t.Fatalf("Due after Rebuild = (%v, %v)", ids(again), err)
	}
	for i := range wantDue {
		if again[i].ID != wantDue[i].ID {
			t.Errorf("due order after Rebuild = %v, want %v", ids(again), ids(wantDue))
			break
		}
	}
	if remaining, err := opened.Remaining(future); err != nil || remaining != wantRemaining {
		t.Errorf("Remaining after Rebuild = (%d, %v), want %d", remaining, err, wantRemaining)
	}
}

func TestReviewLeavesTheNoteAlone(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Hands Off", "q :: a", "another paragraph")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	path := notePath(dir, note)
	before := read(t, path)

	for _, rating := range []schedule.Rating{schedule.Good, schedule.Hard} {
		clk.advance(24 * time.Hour)
		if err := lib.Review(card.ID, rating, clk.Now()); err != nil {
			t.Fatalf("Review: %v", err)
		}
	}
	if err := lib.UndoLast(card.ID); err != nil {
		t.Fatalf("UndoLast: %v", err)
	}
	if after := read(t, path); !bytes.Equal(after, before) {
		t.Errorf("reviewing rewrote the note:\n%q\nwant\n%q", after, before)
	}
}

func TestCreateRefusesAnExistingLibrary(t *testing.T) {
	_, dir, clk := newLibrary(t)
	if _, err := store.Create(dir, store.Options{Now: clk.Now}); !errors.Is(err, store.ErrExists) {
		t.Errorf("Create over an existing library = %v, want ErrExists", err)
	}
}

func TestSaveCard(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	note := saveNote(t, lib, "Overrides", "the question :: the answer")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	card.Fields = map[string]string{"Front": "sharpened question"}
	card.Edited = true
	card.Status = core.StatusSuspended
	if err := lib.SaveCard(card); err != nil {
		t.Fatalf("SaveCard: %v", err)
	}
	stored, ok := lib.Card(card.ID)
	if !ok || stored.Fields["Front"] != "sharpened question" || stored.Status != core.StatusSuspended {
		t.Fatalf("Card = (%+v, %v)", stored, ok)
	}
	// The field values a card asks are resolved at render time; the store
	// keeps only the edit it was given.
	if !stored.Edited || stored.Fields["Back"] != "" {
		t.Errorf("the stored edit is %+v", stored.Fields)
	}
	if due, _ := lib.Due(clk.Now(), 0); len(due) != 0 {
		t.Errorf("a suspended card is in the queue: %v", ids(due))
	}

	// A frozen reference keeps the text it saw, and survives losing its block.
	frozen := card
	frozen.Status = core.StatusNew
	frozen.Ref = frozen.Ref.Freeze(lib, t0)
	if err := lib.SaveCard(frozen); err != nil {
		t.Fatalf("SaveCard frozen: %v", err)
	}
	stored, _ = lib.Card(card.ID)
	if _, state := stored.Ref.Text(lib); state != core.LinkFrozen {
		t.Errorf("a frozen card renders as %v, want frozen", state)
	}

	clk.advance(time.Hour)
	reopened := reopen(t, lib, dir, clk)
	stored, ok = reopened.Card(card.ID)
	if !ok {
		t.Fatalf("frozen card did not survive a reopen")
	}
	if _, state := stored.Ref.Text(reopened); state != core.LinkFrozen {
		t.Errorf("after reopen the reference is %v, want frozen", state)
	}
	if stored.Ref.Frozen == nil || stored.Ref.Frozen.Text != "the question :: the answer" {
		t.Fatalf("snapshot = %+v, want the block as it was", stored.Ref.Frozen)
	}
	if !stored.Ref.Frozen.At.Equal(t0) {
		t.Errorf("snapshot time = %v, want %v", stored.Ref.Frozen.At, t0)
	}

	note.Blocks = nil
	if _, err := reopened.SaveNote(note); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if lost, err := reopened.LostCards(); err != nil || len(lost) != 0 {
		t.Errorf("a frozen card is reported lost: %v (%v)", ids(lost), err)
	}
	if due, err := reopened.Due(clk.Now(), 0); err != nil || len(due) != 1 {
		t.Errorf("a frozen card left the queue: %v (%v)", ids(due), err)
	}

	cards := reopened.Cards()
	if len(cards) != 1 || cards[0].ID != card.ID {
		t.Errorf("Cards() = %v, want the one card", ids(cards))
	}
	if limited, err := reopened.Due(clk.Now(), 0); err != nil || len(limited) != 1 {
		t.Errorf("Due without a limit = (%v, %v)", ids(limited), err)
	}
	if limited, err := reopened.Due(clk.Now(), 1); err != nil || len(limited) != 1 {
		t.Errorf("Due with a limit of one = (%v, %v)", ids(limited), err)
	}
}

func TestConcurrentReadersSeeWholeValues(t *testing.T) {
	lib, _, clk := newLibrary(t)
	note := saveNote(t, lib, "Shared", "q :: a")
	card, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 20 {
			clk.advance(time.Hour)
			if err := lib.Review(card.ID, schedule.Good, clk.Now()); err != nil {
				t.Errorf("Review: %v", err)
				return
			}
		}
	}()
	for range 20 {
		if _, err := lib.Due(t0, 0); err != nil {
			t.Errorf("Due: %v", err)
		}
		if _, err := lib.Remaining(t0); err != nil {
			t.Errorf("Remaining: %v", err)
		}
		if len(lib.Notes()) != 1 || len(lib.Cards()) != 1 {
			t.Error("a reader saw a half-written library")
		}
		lib.CardState(card.ID)
	}
	<-done
}

func TestNoteSourceRoundTrips(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		blocks []string
	}{
		{name: "no title", blocks: []string{"plain"}},
		{name: "title", title: "Notes", blocks: []string{"one", "two"}},
		{name: "block that opens like a title", blocks: []string{"# heading in prose", "next"}},
		{name: "indented lines", blocks: []string{"- one\n- two", "  spaced  "}},
		{name: "empty block", blocks: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib, dir, clk := newLibrary(t)
			note := saveNote(t, lib, tt.title, tt.blocks...)

			reopened := reopen(t, lib, dir, clk)
			got, ok := reopened.Note(note.ID)
			if !ok {
				t.Fatalf("note did not survive the write")
			}
			if got.Title != tt.title {
				t.Errorf("title = %q, want %q", got.Title, tt.title)
			}
			if len(got.Blocks) != len(note.Blocks) {
				t.Fatalf("blocks = %+v, want %d", got.Blocks, len(note.Blocks))
			}
			for i := range note.Blocks {
				if got.Blocks[i] != note.Blocks[i] {
					t.Errorf("block %d = %+v, want %+v", i, got.Blocks[i], note.Blocks[i])
				}
			}
		})
	}
}

func TestLibraryInAPathWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My Notes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer lib.Close()
	if _, err := lib.CreateNote("hello"); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if _, err := lib.Remaining(t0); err != nil {
		t.Fatalf("Remaining: %v", err)
	}
}
