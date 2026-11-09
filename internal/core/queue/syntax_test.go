package queue

import (
	"reflect"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

func TestParseCardQueryReadsKeys(t *testing.T) {
	status := func(s core.Status) *core.Status { return &s }
	ref := func(r core.LinkState) *core.LinkState { return &r }

	tests := []struct {
		name string
		raw  string
		want store.CardQuery
	}{
		{"nothing", "", store.CardQuery{}},
		{"only space", "   \t ", store.CardQuery{}},
		{"one word", "spacing", store.CardQuery{Text: "spacing"}},
		{"two words are one phrase", "spacing beats", store.CardQuery{Text: "spacing beats"}},
		{"a status", "is:review", store.CardQuery{Status: status(core.StatusReview)}},
		{"a status in another case", "IS:New", store.CardQuery{Status: status(core.StatusNew)}},
		{"suspended", "is:suspended", store.CardQuery{Status: status(core.StatusSuspended)}},
		{"a live source", "is:live", store.CardQuery{Ref: ref(core.LinkLive)}},
		{"a frozen source", "is:frozen", store.CardQuery{Ref: ref(core.LinkFrozen)}},
		{"a lost source", "is:lost", store.CardQuery{Ref: ref(core.LinkLost)}},
		{"a hard card", "is:hard", store.CardQuery{MinLapses: 1}},
		{"an is value nothing knows", "is:due", store.CardQuery{Text: "is:due"}},
		{"a tag", "tag:study", store.CardQuery{Tag: "study"}},
		{"a tag written with its hash", "tag:#study", store.CardQuery{Tag: "study"}},
		{"a type", "type:remit.cloze", store.CardQuery{Type: core.TypeCloze}},
		{"a type this library minted", "type:type_0123456789abcdef0123456789abcdef",
			store.CardQuery{Type: core.ID("type_0123456789abcdef0123456789abcdef")}},
		{"a type nothing names", "type:cloze", store.CardQuery{Text: "type:cloze"}},
		{"a note by id", "note:note_0123456789abcdef0123456789abcdef",
			store.CardQuery{Note: "note_0123456789abcdef0123456789abcdef"}},
		{"a note by title with a space", `note:"Memory and recall"`, store.CardQuery{Note: "Memory and recall"}},
		{"a quote left open runs to the end", `note:"Memory and`, store.CardQuery{Note: "Memory and"}},
		{"a key with no value is text", "is:", store.CardQuery{Text: "is:"}},
		{"an unknown key is text", "due:tomorrow", store.CardQuery{Text: "due:tomorrow"}},
		{"terms are anded", "tag:study is:review entropy",
			store.CardQuery{Tag: "study", Status: status(core.StatusReview), Text: "entropy"}},
		{"two keys and a phrase", `is:frozen note:"Memory and recall" spacing beats`,
			store.CardQuery{Ref: ref(core.LinkFrozen), Note: "Memory and recall", Text: "spacing beats"}},
		{"the last status wins", "is:new is:review", store.CardQuery{Status: status(core.StatusReview)}},
		{"a colon inside a value", `note:a:b`, store.CardQuery{Note: "a:b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCardQuery(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseCardQuery(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

// TestParseCardQueryRunsOnTheStore is the parser and the query together: what the
// box says is what the table shows.
func TestParseCardQueryRunsOnTheStore(t *testing.T) {
	lib, cards := syntaxLibrary(t)

	tests := []struct {
		name string
		raw  string
		want []core.ID // by block
	}{
		{"a tag", "tag:study", []core.ID{cards["studied"].Block, cards["studied2"].Block}},
		{"a status", "is:suspended", []core.ID{cards["suspended"].Block}},
		{"a type", "type:remit.cloze", []core.ID{cards["cloze"].Block}},
		{"a hard card", "is:hard", []core.ID{cards["hard"].Block}},
		{"a note title", "note:Studied", []core.ID{cards["studied"].Block, cards["studied2"].Block}},
		{"a tag and a status of another card", "tag:study is:suspended", nil},
		{"text and a tag", "tag:study cramming", []core.ID{cards["studied"].Block}},
		{"text", "entropy", []core.ID{cards["other"].Block}},
		{"an unknown key is text", "due:tomorrow", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, total, err := lib.QueryCards(ParseCardQuery(tt.raw))
			if err != nil {
				t.Fatalf("QueryCards: %v", err)
			}
			got := make([]core.ID, 0, len(rows))
			for _, row := range rows {
				got = append(got, row.Block)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("cards = %v, want %v", got, tt.want)
			}
			for _, want := range tt.want {
				found := false
				for _, block := range got {
					if block == want {
						found = true
					}
				}
				if !found {
					t.Errorf("cards = %v, want %q among them", got, want)
				}
			}
			if total != len(tt.want) {
				t.Errorf("total = %d, want %d", total, len(tt.want))
			}
		})
	}
}

// syntaxLibrary builds the cards the cases above name, keyed by what they are.
func syntaxLibrary(t *testing.T) (*store.Library, map[string]store.CardSummary) {
	t.Helper()
	lib := library(t)
	studied := syntaxNote(t, lib, "Studied", "spacing :: beats cramming #study", "recall :: helps #study")
	other := syntaxNote(t, lib, "Other", "entropy :: grows")
	clozed := syntaxNote(t, lib, "Clozed", "the {{first}} blank")
	hard := syntaxNote(t, lib, "Hard", "again :: and again")

	byBlock := map[string]core.ID{}
	makeCard := func(key, block string, typ core.ID) {
		cards, err := lib.CreateCards(core.ID(block), []core.Derived{{Type: typ}})
		if err != nil {
			t.Fatalf("CreateCards: %v", err)
		}
		byBlock[key] = cards[0].ID
	}
	makeCard("studied", string(studied.Blocks[0].ID), core.TypeBasic)
	makeCard("studied2", string(studied.Blocks[1].ID), core.TypeBasic)
	makeCard("other", string(other.Blocks[0].ID), core.TypeBasic)
	makeCard("cloze", string(clozed.Blocks[0].ID), core.TypeCloze)
	makeCard("hard", string(hard.Blocks[0].ID), core.TypeBasic)

	if _, err := lib.SuspendCards([]core.ID{byBlock["other"]}, now); err != nil {
		t.Fatalf("SuspendCards: %v", err)
	}
	for range 2 {
		if err := lib.Review(byBlock["hard"], schedule.Again, now); err != nil {
			t.Fatalf("Review: %v", err)
		}
	}

	rows, _, err := lib.QueryCards(store.CardQuery{})
	if err != nil {
		t.Fatalf("QueryCards: %v", err)
	}
	out := map[string]store.CardSummary{}
	for key, id := range byBlock {
		for _, row := range rows {
			if row.ID == id {
				out[key] = row
			}
		}
	}
	out["suspended"] = out["other"]
	return lib, out
}

// syntaxNote saves a note with one block per text, so a case can name a block of
// it.
func syntaxNote(t *testing.T, lib *store.Library, title string, texts ...string) core.Note {
	t.Helper()
	blocks := make([]core.Block, 0, len(texts))
	for _, text := range texts {
		blocks = append(blocks, core.Block{Text: text})
	}
	note, err := lib.SaveNote(core.Note{Title: title, Blocks: blocks})
	if err != nil {
		t.Fatalf("SaveNote %q: %v", title, err)
	}
	return note
}
