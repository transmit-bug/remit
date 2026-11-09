package queue

import (
	"fmt"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

var now = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

func library(t *testing.T) *store.Library {
	t.Helper()
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return lib
}

func cards(t *testing.T, lib *store.Library, n int) []core.Card {
	t.Helper()
	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: blocksOf(n)})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	out := make([]core.Card, 0, n)
	for _, b := range note.Blocks {
		card, err := firstCard(lib, b.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		out = append(out, card)
	}
	return out
}

func blocksOf(n int) []core.Block {
	out := make([]core.Block, n)
	for i := range out {
		out[i] = core.Block{Text: "q :: a"}
	}
	return out
}

func TestNewCardsComeLastAndOnlySoMany(t *testing.T) {
	lib := library(t)
	all := cards(t, lib, 3)
	// Reviewed yesterday, so today's new-card allowance is untouched.
	if err := lib.Review(all[0].ID, schedule.Good, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}

	got, err := Cards(lib, now, Options{NewPerDay: 1})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	// The reviewed card is scheduled into the future, so the queue is the
	// new-card allowance alone.
	if len(got) != 1 {
		t.Fatalf("queue = %d cards, want the one allowed new card", len(got))
	}
	if got[0].ID == all[0].ID {
		t.Error("the reviewed card came back before it was due")
	}
	if state, _ := lib.CardState(got[0].ID); state.Reviews != 0 {
		t.Error("the allowed card is not new")
	}
}

func TestExcludedCardsAreNotInTheQueue(t *testing.T) {
	lib := library(t)
	all := cards(t, lib, 3)

	suspended := all[0]
	suspended.Status = core.StatusSuspended
	if err := lib.SaveCard(suspended); err != nil {
		t.Fatalf("save: %v", err)
	}
	reading := all[1]
	reading.Status = core.StatusReading
	if err := lib.SaveCard(reading); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Cards(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	for _, card := range got {
		if card.ID == suspended.ID || card.ID == reading.ID {
			t.Errorf("queue holds a %s card", card.Status)
		}
	}
	if len(got) != 1 {
		t.Errorf("queue = %d cards, want only the scheduled one", len(got))
	}
}

func TestOrderIsStable(t *testing.T) {
	lib := library(t)
	_ = cards(t, lib, 4)

	first, err := Cards(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	second, err := Cards(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("queue length changed between calls: %d then %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("queue order changed at %d: %s then %s", i, first[i].ID, second[i].ID)
		}
	}
}

// scopedFixture is two notes whose paragraphs carry cards: alpha's blocks are
// tagged, beta's are not, so a note scope and a tag scope each have a distinct
// answer, and the untagged note is the complement of the tag scope.
func scopedFixture(t *testing.T, lib *store.Library) (alpha, beta core.Note, all []core.Card) {
	t.Helper()
	alpha, err := lib.SaveNote(core.Note{Title: "alpha", Blocks: []core.Block{
		{Text: "one :: 1 #ch1"},
		{Text: "two :: 2 #ch1"},
	}})
	if err != nil {
		t.Fatalf("save alpha: %v", err)
	}
	beta, err = lib.SaveNote(core.Note{Title: "beta", Blocks: []core.Block{
		{Text: "three :: 3"},
		{Text: "four :: 4"},
	}})
	if err != nil {
		t.Fatalf("save beta: %v", err)
	}
	for _, block := range append(append([]core.Block{}, alpha.Blocks...), beta.Blocks...) {
		card, err := firstCard(lib, block.ID)
		if err != nil {
			t.Fatalf("card: %v", err)
		}
		all = append(all, card)
	}
	return alpha, beta, all
}

func ids(cards []core.Card) []core.ID {
	out := make([]core.ID, len(cards))
	for i, card := range cards {
		out[i] = card.ID
	}
	return out
}

func sameIDs(got, want []core.ID) bool {
	if len(got) != len(want) {
		return false
	}
	set := map[core.ID]int{}
	for _, id := range got {
		set[id]++
	}
	for _, id := range want {
		set[id]--
		if set[id] < 0 {
			return false
		}
	}
	return true
}

// The zero scope is the whole library: today's answer is exactly what it was
// before scopes existed.
func TestZeroScopeIsTheWholeLibrary(t *testing.T) {
	lib := library(t)
	_, _, all := scopedFixture(t, lib)

	got, err := Cards(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if !sameIDs(ids(got), ids(all)) {
		t.Fatalf("zero scope = %v, want the whole library %v", ids(got), ids(all))
	}
}

// A note scope keeps only the cards whose blocks live in that note; a tag scope
// keeps only the cards whose source block carries the tag, whatever note holds
// them.
func TestNoteAndTagScopesFilterTheQueue(t *testing.T) {
	lib := library(t)
	alpha, beta, all := scopedFixture(t, lib)

	byNote, err := Cards(lib, now, Options{Scope: Scope{Kind: ScopeNote, Note: alpha.ID}})
	if err != nil {
		t.Fatalf("Today by note: %v", err)
	}
	wantNote := ids(all)[:2]
	if !sameIDs(ids(byNote), wantNote) {
		t.Fatalf("note scope = %v, want alpha's cards %v", ids(byNote), wantNote)
	}

	byTag, err := Cards(lib, now, Options{Scope: Scope{Kind: ScopeTag, Tag: "ch1"}})
	if err != nil {
		t.Fatalf("Today by tag: %v", err)
	}
	if !sameIDs(ids(byTag), wantNote) {
		t.Fatalf("tag scope = %v, want the tagged cards %v", ids(byTag), wantNote)
	}

	// beta's cards are the complement: reachable under no scope here, so the
	// filters genuinely exclude rather than return everything.
	byBeta, err := Cards(lib, now, Options{Scope: Scope{Kind: ScopeNote, Note: beta.ID}})
	if err != nil {
		t.Fatalf("Today by beta: %v", err)
	}
	if sameIDs(ids(byBeta), ids(byNote)) {
		t.Fatal("beta's scope returned alpha's cards")
	}
	if len(byBeta) == len(all) {
		t.Fatal("a scoped queue returned the whole library")
	}
}

// CountsOf answers under the same scope Today filters by, so a screen can show
// how much a scope holds without building its queue.
func TestCountsOfFollowsTheScope(t *testing.T) {
	lib := library(t)
	alpha, _, all := scopedFixture(t, lib)

	counts, err := CountsOf(lib, now, Options{Scope: Scope{Kind: ScopeNote, Note: alpha.ID}})
	if err != nil {
		t.Fatalf("CountsOf: %v", err)
	}
	if counts.Remaining != 2 || counts.New != 2 {
		t.Fatalf("scoped counts = %+v, want two new cards", counts)
	}
	allCounts, err := CountsOf(lib, now, Options{})
	if err != nil {
		t.Fatalf("CountsOf all: %v", err)
	}
	if allCounts.Remaining != len(all) {
		t.Fatalf("whole-library counts = %d, want %d", allCounts.Remaining, len(all))
	}
}

// The allowance and the limit act on the scoped set: a scope of two cards with
// a one-card limit hands back one card from the scope, and the scoped queue
// never grows to reach the library's answer.
func TestAllowanceAndLimitActAfterTheScope(t *testing.T) {
	lib := library(t)
	alpha, beta, all := scopedFixture(t, lib)

	got, err := Cards(lib, now, Options{Limit: 1, Scope: Scope{Kind: ScopeNote, Note: alpha.ID}})
	if err != nil {
		t.Fatalf("Today scoped and limited: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("scoped queue with limit = %v, want one of alpha's cards", ids(got))
	}
	if got[0].ID != ids(all)[0] && got[0].ID != ids(all)[1] {
		t.Fatalf("limit pulled a card outside the scope: %v", ids(got))
	}

	// The scope comes first: with a limit of one, a scoped queue is one card,
	// while the whole library under the same limit is one card too — but of a
	// different, larger set. The scoped answer can never hold beta's cards.
	for _, card := range got {
		if card.ID == beta.Blocks[0].ID || card.ID == beta.Blocks[1].ID {
			t.Errorf("beta's block leaked into alpha's scope: %v", card.ID)
		}
	}

	// A new-card allowance cuts the scoped set, not the library: alpha holds
	// two new cards, an allowance of one admits one of them, and beta's review
	// yesterday spent none of today's allowance.
	if err := lib.Review(all[2].ID, schedule.Again, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}
	scoped, err := Cards(lib, now, Options{NewPerDay: 1, Scope: Scope{Kind: ScopeNote, Note: alpha.ID}})
	if err != nil {
		t.Fatalf("Today scoped: %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("scoped queue = %v, want one new card within the day's allowance", ids(scoped))
	}
	if scoped[0].ID != ids(all)[0] && scoped[0].ID != ids(all)[1] {
		t.Fatalf("the allowance pulled a card outside the scope: %v", ids(scoped))
	}
}

// A scope nothing answers — a note with no cards, a tag no block carries —
// leaves the queue empty rather than falling back to the library.
func TestEmptyScopeStaysEmpty(t *testing.T) {
	lib := library(t)
	alpha, _, _ := scopedFixture(t, lib)

	empty, err := lib.SaveNote(core.Note{Title: "prose", Blocks: []core.Block{{Text: "no cards here"}}})
	if err != nil {
		t.Fatalf("save prose: %v", err)
	}

	got, err := Cards(lib, now, Options{Scope: Scope{Kind: ScopeNote, Note: empty.ID}})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("cardless note scope = %v, want an empty queue", ids(got))
	}

	got, err = Cards(lib, now, Options{Scope: Scope{Kind: ScopeTag, Tag: "nothing-has-this"}})
	if err != nil {
		t.Fatalf("Today by missing tag: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("missing tag scope = %v, want an empty queue", ids(got))
	}

	counts, err := CountsOf(lib, now, Options{Scope: Scope{Kind: ScopeTag, Tag: "nothing-has-this"}})
	if err != nil {
		t.Fatalf("CountsOf: %v", err)
	}
	if counts.Remaining != 0 {
		t.Fatalf("missing tag count = %d, want 0", counts.Remaining)
	}
	_ = alpha
}

func TestCountsMatchTheQueue(t *testing.T) {
	lib := library(t)
	all := cards(t, lib, 3)
	if err := lib.Review(all[0].ID, schedule.Again, now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("review: %v", err)
	}

	counts, err := CountsOf(lib, now, Options{})
	if err != nil {
		t.Fatalf("CountsOf: %v", err)
	}
	cardsNow, err := Cards(lib, now, Options{})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if counts.Remaining != len(cardsNow) {
		t.Errorf("remaining %d, queue %d", counts.Remaining, len(cardsNow))
	}
	if counts.Remaining != counts.Due+counts.Learning+counts.New {
		t.Errorf("counts do not add up: %+v", counts)
	}
}

func TestLimitCapsTheQueue(t *testing.T) {
	lib := library(t)
	_ = cards(t, lib, 5)

	got, err := Cards(lib, now, Options{Limit: 2})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("queue = %d cards, want the limit of 2", len(got))
	}
}

func TestNewPerDayCountsWhatStartedToday(t *testing.T) {
	lib := library(t)
	all := cards(t, lib, 3)
	// Two cards were started today, so a per-day allowance of two leaves no
	// room for another new card.
	for _, card := range all[:2] {
		if err := lib.Review(card.ID, schedule.Again, now.Add(-time.Hour)); err != nil {
			t.Fatalf("review: %v", err)
		}
	}

	got, err := Cards(lib, now, Options{NewPerDay: 2})
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	for _, card := range got {
		if state, ok := lib.CardState(card.ID); !ok || state.Reviews == 0 {
			if card.ID == all[2].ID {
				t.Error("a new card joined the queue after the daily allowance was spent")
			}
		}
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

// readingNotes queues notes at one moment and answers with them in the order
// they were made, so a test names the order the queue should hold.
func readingNotes(t *testing.T, lib *store.Library, at time.Time, titles ...string) []core.Note {
	t.Helper()
	out := make([]core.Note, 0, len(titles))
	for _, title := range titles {
		note, err := lib.SaveNote(core.Note{Title: title, Blocks: []core.Block{{Text: "a paragraph"}}})
		if err != nil {
			t.Fatalf("save note: %v", err)
		}
		if err := lib.AddToReading(note.ID, at); err != nil {
			t.Fatalf("add to reading: %v", err)
		}
		out = append(out, note)
	}
	return out
}

// The reading queue is the notes due now, soonest first: a note never read is
// due when it joined, and a note read comes back on the cadence reading gave
// it. Neither the order nor the counts turn on the store being the same object.
func TestReadingQueueOrder(t *testing.T) {
	lib := library(t)
	notes := readingNotes(t, lib, now.AddDate(0, 0, -10), "alpha", "beta", "gamma")

	// alpha was read four days ago and is due again; beta was read just now and
	// is scheduled ahead; gamma was never read, so it has been due since it
	// joined.
	for _, read := range []struct {
		note   core.Note
		at     time.Time
		rating schedule.Rating
	}{
		{notes[0], now.AddDate(0, 0, -4), schedule.Good},
		{notes[1], now, schedule.Easy},
	} {
		if err := lib.ReadNote(read.note.ID, read.rating, read.note.Blocks[0].ID, read.at); err != nil {
			t.Fatalf("ReadNote: %v", err)
		}
	}

	got, err := Reading(lib, now, Options{})
	if err != nil {
		t.Fatalf("Reading: %v", err)
	}
	order := make([]core.ID, len(got))
	for i, entry := range got {
		order[i] = entry.Note
	}
	if len(order) != 2 || order[0] != notes[2].ID || order[1] != notes[0].ID {
		t.Fatalf("reading queue = %v, want the unread note then the one read four days ago", order)
	}

	count, err := ReadingCount(lib, now)
	if err != nil {
		t.Fatalf("ReadingCount: %v", err)
	}
	if count != len(got) {
		t.Errorf("count = %d, queue = %d", count, len(got))
	}

	limited, err := Reading(lib, now, Options{Limit: 1})
	if err != nil {
		t.Fatalf("Reading with a limit: %v", err)
	}
	if len(limited) != 1 || limited[0].Note != order[0] {
		t.Errorf("limited queue = %+v, want the first entry alone", limited)
	}
}

// A note read today and scheduled for later is not in today's reading queue,
// and not counted either: the queue is a promise of what can be read now.
func TestReadingQueueLeavesWhatIsNotDue(t *testing.T) {
	lib := library(t)
	notes := readingNotes(t, lib, now, "alpha")
	if err := lib.ReadNote(notes[0].ID, schedule.Easy, notes[0].Blocks[0].ID, now); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	got, err := Reading(lib, now.Add(time.Minute), Options{})
	if err != nil {
		t.Fatalf("Reading: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("reading queue = %v, want nothing due a minute after it was read", got)
	}
	count, err := ReadingCount(lib, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ReadingCount: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// When the schedule says it is due, it comes back where it stopped.
	entry, ok := lib.ReadingFor(notes[0].ID)
	if !ok {
		t.Fatal("the read note left the reading queue for good")
	}
	later := entry.State.Due.Add(time.Minute)
	got, err = Reading(lib, later, Options{})
	if err != nil {
		t.Fatalf("Reading when it is due: %v", err)
	}
	if len(got) != 1 || got[0].Note != notes[0].ID {
		t.Fatalf("reading queue = %+v, want the note back", got)
	}
	if got[0].Position != notes[0].Blocks[0].ID {
		t.Errorf("position = %q, want the block it stopped at", got[0].Position)
	}
}

// A note template is a note: it joins the reading queue like any other note,
// which is what makes incremental reading able to walk it (#154).
func TestTemplateJoinsTheReadingQueue(t *testing.T) {
	lib := library(t)
	template, err := lib.SaveNote(core.Note{Title: "Reading excerpt", Blocks: []core.Block{
		{Text: "#template"},
		{Text: "Source :: answer"},
	}})
	if err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := lib.AddToReading(template.ID, now); err != nil {
		t.Fatalf("add to reading: %v", err)
	}
	due, err := Reading(lib, now.Add(time.Minute), Options{})
	if err != nil {
		t.Fatalf("reading queue: %v", err)
	}
	if len(due) != 1 || due[0].Note != template.ID {
		t.Fatalf("reading queue = %v, want the template", due)
	}
}

// The day's reading allowance paces new material: a note no session has read
// yet is new, and only so many of them come in a day. A count is what is
// waiting, not what one sitting takes, so it is not capped.
func TestReadingIntroducesSoManyNewNotesPerDay(t *testing.T) {
	for _, tt := range []struct {
		name string
		cap  int
		want int
	}{
		{"no cap", 0, 3},
		{"three a day, two already spent", 3, 1},
		{"one a day, two already spent", 1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib := library(t)
			for i := range 3 {
				queueNote(t, lib, todayNote(t, lib, fmt.Sprintf("new%d", i), 1), now.AddDate(0, 0, -2))
			}
			for i := range 2 {
				stopReading(t, lib, todayNote(t, lib, fmt.Sprintf("read%d", i), 1), 1, schedule.Easy,
					now.Add(-time.Duration(i+1)*time.Hour))
			}

			due, err := Reading(lib, now, Options{ReadingNewPerDay: tt.cap})
			if err != nil {
				t.Fatalf("Reading: %v", err)
			}
			fresh := 0
			for _, entry := range due {
				if entry.State.Reviews == 0 {
					fresh++
				}
			}
			if fresh != tt.want {
				t.Fatalf("reading queue introduces %d unread notes, want %d", fresh, tt.want)
			}
			count, err := ReadingCount(lib, now)
			if err != nil {
				t.Fatalf("ReadingCount: %v", err)
			}
			if count != 3 {
				t.Errorf("count = %d, want the three notes waiting", count)
			}
		})
	}
}

// The day's allowance is spent by the reading and not by the queue's
// membership: a note read today and then dropped, archived or dismissed still
// holds the day's next note back, so leaving the queue cannot buy a second one.
func TestReadingAllowanceCountsANoteThatLeftTheQueue(t *testing.T) {
	for _, tt := range []struct {
		name  string
		leave func(t *testing.T, lib *store.Library, note core.Note, at time.Time)
	}{
		{"dropped", func(t *testing.T, lib *store.Library, note core.Note, at time.Time) {
			if err := lib.RemoveFromReading(note.ID, at); err != nil {
				t.Fatalf("RemoveFromReading: %v", err)
			}
		}},
		{"archived", func(t *testing.T, lib *store.Library, note core.Note, at time.Time) {
			if err := lib.ArchiveReading(note.ID, at); err != nil {
				t.Fatalf("ArchiveReading: %v", err)
			}
		}},
		{"dismissed", func(t *testing.T, lib *store.Library, note core.Note, at time.Time) {
			if err := lib.DismissReading(note.ID, at); err != nil {
				t.Fatalf("DismissReading: %v", err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib := library(t)
			read := todayNote(t, lib, "read today", 1)
			stopReading(t, lib, read, 1, schedule.Easy, now.Add(-time.Hour))
			tt.leave(t, lib, read, now.Add(-30*time.Minute))
			queueNote(t, lib, todayNote(t, lib, "unread", 1), now.AddDate(0, 0, -3))

			due, err := Reading(lib, now, Options{ReadingNewPerDay: 1})
			if err != nil {
				t.Fatalf("Reading: %v", err)
			}
			if len(due) != 0 {
				t.Fatalf("reading queue = %+v, want the unread note held back by the day's spent allowance", due)
			}
		})
	}
}

// A note coming back for a second reading is not new material: the allowance is
// spent, and the cadence the schedule set still brings it back. The unread note
// it outranks in the queue is the one held back.
func TestReadingAllowanceLeavesTheCadenceAlone(t *testing.T) {
	lib := library(t)
	again := todayNote(t, lib, "again", 2)
	stopReading(t, lib, again, 1, schedule.Again, now.AddDate(0, 0, -2))
	// One note read today, so a one-a-day allowance has nothing left.
	stopReading(t, lib, todayNote(t, lib, "read today", 1), 1, schedule.Easy, now.Add(-time.Hour))
	queueNote(t, lib, todayNote(t, lib, "unread", 1), now.AddDate(0, 0, -3))

	due, err := Reading(lib, now, Options{ReadingNewPerDay: 1})
	if err != nil {
		t.Fatalf("Reading: %v", err)
	}
	if len(due) != 1 || due[0].Note != again.ID {
		t.Fatalf("reading queue = %+v, want the note coming back and not the unread one", due)
	}
}
