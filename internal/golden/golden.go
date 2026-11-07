// Package golden freezes what today's implementation answers, so that the
// implementation that replaces it can be held to the same answers.
//
// It is the evidence #103 stands on: the Go card deriver and the Go scheduler are
// to be deleted, and what they knew lives here — the faces a paragraph gives,
// and what each of the four answers schedules from each kind of state. The set is
// built from the code under test and compared byte for byte with the frozen
// file; nothing is fuzzy.
package golden

import (
	"encoding/json"
	"math"
	"time"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// Note says what the file is and where it comes from.
const Note = "Frozen answers of the Go implementation (ADR 0005/0007): the faces a " +
	"paragraph gives, and the schedule each answer leaves. Regenerate with " +
	"go test ./internal/golden -update only when the answers are meant to change."

// Set is the whole frozen answer set.
type Set struct {
	Note     string    `json:"note"`
	Faces    []Face    `json:"faces"`
	Schedule []Step    `json:"schedule"`
	Excerpts []Excerpt `json:"excerpts"`
}

// Face is one card's sides, as the content model resolves them.
type Face struct {
	Proves  string `json:"proves"`
	Text    string `json:"text"`
	Kind    string `json:"kind"`
	Ordinal int    `json:"ordinal"`
	// What the card carries of its own, when it carries anything.
	OverridePrompt string `json:"override_prompt,omitempty"`
	OverrideAnswer string `json:"override_answer,omitempty"`
	FrozenText     string `json:"frozen_text,omitempty"`
	Lost           bool   `json:"lost,omitempty"`
	// What the model answers.
	Prompt string `json:"prompt"`
	Answer string `json:"answer"`
	State  string `json:"state"`
}

// Step is one review's answer: where the card stood, what was answered, and what
// the schedule became.
type Step struct {
	Proves     string  `json:"proves"`
	From       string  `json:"from"`
	At         string  `json:"at"`
	Rating     string  `json:"rating"`
	Due        string  `json:"due"`
	Reviews    int     `json:"reviews"`
	Stability  float64 `json:"stability"`
	Difficulty float64 `json:"difficulty"`
}

// Build derives the set from the implementation under test.
func Build() Set {
	return Set{
		Note:     Note,
		Faces:    faces(),
		Schedule: schedule_(),
		Excerpts: excerpts(),
	}
}

// Encode renders the set the way the frozen file holds it.
func Encode(set Set) ([]byte, error) {
	data, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// blockID is the block every face case points at, except the lost one.
const blockID = core.ID("block_00000000000000000000000000000001")

// faces is the seven shapes a card's sides take.
func faces() []Face {
	blocks := core.BlockSet{
		blockID: {ID: blockID, Text: "Spacing beats cramming :: reviewing just before you forget strengthens recall."},
		"block_00000000000000000000000000000002": {ID: "block_00000000000000000000000000000002",
			Text: "The {{first}} blank and the {{second}} one."},
		"block_00000000000000000000000000000003": {ID: "block_00000000000000000000000000000003",
			Text: "spacing ::: the gaps between reviews"},
		"block_00000000000000000000000000000004": {ID: "block_00000000000000000000000000000004",
			Text: "one block :: that divides :: twice"},
		"block_00000000000000000000000000000005": {ID: "block_00000000000000000000000000000005",
			Text: "a paragraph with no separator at all"},
	}

	cases := []struct {
		proves string
		card   core.Card
	}{
		{"a paragraph with no separator asks everything and answers nothing",
			core.Card{ID: "card_1", Type: core.TypeBasic, Ref: core.Reference{Block: "block_00000000000000000000000000000005"}}},
		{"the first separator divides, and later ones stay in the answer",
			core.Card{ID: "card_2", Type: core.TypeBasic, Ref: core.Reference{Block: "block_00000000000000000000000000000004"}}},
		{"a card's own prompt replaces the paragraph's",
			core.Card{ID: "card_3", Type: core.TypeBasic, Ref: core.Reference{Block: blockID},
				Fields: map[string]string{"Front": "asked another way"}, Edited: true}},
		{"a card's own answer replaces the paragraph's",
			core.Card{ID: "card_4", Type: core.TypeBasic, Ref: core.Reference{Block: blockID},
				Fields: map[string]string{"Back": "answered another way"}, Edited: true}},
		{"a lost card keeps only what it overrides",
			core.Card{ID: "card_5", Type: core.TypeBasic, Ref: core.Reference{Block: "block_ffffffffffffffffffffffffffffffff"},
				Fields: map[string]string{"Front": "still asked"}, Edited: true}},
		{"a frozen card asks the text it kept, not the paragraph's",
			core.Card{ID: "card_6", Type: core.TypeBasic, Ref: core.Reference{Block: blockID,
				Frozen: &core.Snapshot{Text: "old prompt :: old answer"}}}},
		{"the first blank of a cloze is the gap, the rest show",
			core.Card{ID: "card_7", Type: core.TypeCloze, Ref: core.Reference{Block: "block_00000000000000000000000000000002"},
				Ordinal: 0}},
		{"the second blank of a cloze is the gap, the first shows",
			core.Card{ID: "card_8", Type: core.TypeCloze, Ref: core.Reference{Block: "block_00000000000000000000000000000002"},
				Ordinal: 1}},
		{"a cloze whose blank is gone asks what it can",
			core.Card{ID: "card_9", Type: core.TypeCloze, Ref: core.Reference{Block: "block_00000000000000000000000000000002"},
				Ordinal: 5}},
		{"both ways asks the paragraph as written",
			core.Card{ID: "card_10", Type: core.TypeBidirectional, Ref: core.Reference{Block: "block_00000000000000000000000000000003"},
				Ordinal: 0}},
		{"both ways asks it reversed",
			core.Card{ID: "card_11", Type: core.TypeBidirectional, Ref: core.Reference{Block: "block_00000000000000000000000000000003"},
				Ordinal: 1}},
	}

	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		panic("golden: the card plugin did not start: " + err.Error())
	}
	defer runner.Close()
	resolver := cards.New(runner, core.BuiltinTypes())

	out := make([]Face, 0, len(cases))
	for _, c := range cases {
		got := resolver.Face(c.card, blocks)
		face := Face{
			Proves:         c.proves,
			Kind:           kindOf(c.card.Type),
			Ordinal:        c.card.Ordinal,
			OverridePrompt: c.card.Fields["Front"],
			OverrideAnswer: c.card.Fields["Back"],
			Lost:           got.State == core.LinkLost,
			Prompt:         got.Front,
			Answer:         got.Back,
			State:          stateName(got.State),
		}
		if block, ok := blocks[c.card.Ref.Block]; ok {
			face.Text = block.Text
		}
		if snapshot := c.card.Ref.Frozen; snapshot != nil {
			face.FrozenText = snapshot.Text
		}
		out = append(out, face)
	}
	return out
}

// kindOf names a card's type the way the frozen file writes it.
func kindOf(typ core.ID) string {
	switch typ {
	case core.TypeCloze:
		return "cloze"
	case core.TypeBidirectional:
		return "bidirectional"
	}
	return "basic"
}

// round trims floating-point noise to nine decimals. The same weights give the
// same answer on every platform to within the last bits — arm64 and amd64 fuse
// differently — and an answer a card's schedule turns on is far above 1e-9, so
// the frozen file holds the value without the noise rather than a platform's
// version of it.
func round(v float64) float64 { return math.Round(v*1e9) / 1e9 }

// stateName names a link state as the file writes it.
func stateName(state core.LinkState) string {
	switch state {
	case core.LinkLive:
		return "live"
	case core.LinkFrozen:
		return "frozen"
	case core.LinkLost:
		return "lost"
	}
	return "unknown"
}

// schedule_ is what each answer schedules from each kind of state, and the cap
// the interval never passes.
func schedule_() []Step {
	params := schedule.DefaultParams()
	at := time.Date(2026, time.November, 7, 9, 0, 0, 0, time.UTC)

	learned := params.Step(schedule.State{}, schedule.Event{Rating: schedule.Good, At: at.AddDate(0, 0, -2)})
	reviewed := params.Step(learned, schedule.Event{Rating: schedule.Good, At: at.AddDate(0, 0, -10)})
	// Stability beyond anything a real card reaches: the answer that follows is
	// the interval cap's, not the weight table's.
	capped := schedule.State{Memory: schedule.Memory{Stability: 1e7, Difficulty: 5}, Reviews: 3,
		LastReview: at.AddDate(0, 0, -30), Due: at}

	cases := []struct {
		proves string
		from   string
		state  schedule.State
	}{
		{"a card never reviewed, answered", "new", schedule.State{}},
		{"a card being learned, answered", "learned", learned},
		{"a card in long-term review, answered", "reviewed", reviewed},
		{"a card too stable to schedule further: the interval cap", "capped", capped},
	}

	var out []Step
	for _, c := range cases {
		for r := schedule.Again; r <= schedule.Easy; r++ {
			next := params.Step(c.state, schedule.Event{Rating: r, At: at})
			out = append(out, Step{
				Proves:     c.proves,
				From:       c.from,
				At:         at.Format(time.RFC3339),
				Rating:     r.String(),
				Due:        next.Due.UTC().Format(time.RFC3339),
				Reviews:    next.Reviews,
				Stability:  round(next.Memory.Stability),
				Difficulty: round(next.Memory.Difficulty),
			})
		}
	}
	return out
}

// Excerpt is one excerpt's frozen output: the block a child note is given for a
// selection, and the card that block makes, verbatim. It is the reading closure
// in one case: a passage, its way back, and the card it becomes.
type Excerpt struct {
	Proves string `json:"proves"`
	// What the reader selected, and the block of the note it came from.
	SourceTitle string `json:"source_title"`
	SourceBlock string `json:"source_block"`
	SourceText  string `json:"source_text"`
	Selected    string `json:"selected"`
	// The child note's first block, as the store would hold it.
	BlockText string `json:"block_text"`
	// The reference the block carries, as the index reads it.
	LinkTitle string `json:"link_title"`
	LinkBlock string `json:"link_block"`
	// The card the block makes, as its type's templates render it.
	CardKind  string `json:"card_kind"`
	Ordinal   int    `json:"ordinal"`
	CardFront string `json:"card_front"`
	CardBack  string `json:"card_back"`
}

// excerptSource is the note every excerpt case quotes from.
var excerptSource = core.Note{
	Title: "Memory and recall",
	Blocks: []core.Block{
		{ID: "block_00000000000000000000000000000011",
			Text: "Spacing beats cramming :: reviewing just before you forget strengthens recall."},
		{ID: "block_00000000000000000000000000000012",
			Text: "The {{first}} blank and the {{second}} one."},
	},
}

// excerpts is what a selection from a note makes: a child note's block, and the
// card that block asks for.
func excerpts() []Excerpt {
	cases := []struct {
		proves   string
		block    int
		selected string
		kind     core.ID
		ordinal  int
	}{
		{
			proves:   "an excerpt of a card-shaped paragraph is a card of its own",
			block:    0,
			selected: excerptSource.Blocks[0].Text,
			kind:     core.TypeBasic,
		},
		{
			proves:   "a selection of one sentence makes one card, and the reference stays in the answer",
			block:    0,
			selected: "reviewing just before you forget strengthens recall.",
			kind:     core.TypeBasic,
		},
		{
			proves:   "a selection of two lines keeps both, and the reference below them",
			block:    0,
			selected: "Spacing beats cramming.\nReviewing just before you forget strengthens recall.",
			kind:     core.TypeBasic,
		},
		{
			proves: "a cloze passage is a cloze card of the excerpt, asked at its first blank",
			block:  1, selected: excerptSource.Blocks[1].Text, kind: core.TypeCloze, ordinal: 0,
		},
	}

	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		panic("golden: the card plugin did not start: " + err.Error())
	}
	defer runner.Close()
	resolver := cards.New(runner, core.BuiltinTypes())

	out := make([]Excerpt, 0, len(cases))
	for _, c := range cases {
		source := excerptSource.Blocks[c.block]
		block := core.Excerpt(excerptSource, source, c.selected)
		// The child block's own ID is minted when the note is saved, and it is
		// never part of the text: a fixed one keeps the file comparable.
		block.ID = "block_00000000000000000000000000000021"
		card := core.Card{ID: "card_excerpt", Type: c.kind, Ordinal: c.ordinal,
			Ref: core.Reference{Block: block.ID}}
		face := resolver.Face(card, core.BlockSet{block.ID: block})
		link := core.Links(block.Text)
		excerpt := Excerpt{
			Proves:      c.proves,
			SourceTitle: excerptSource.Title,
			SourceBlock: string(source.ID),
			SourceText:  source.Text,
			Selected:    c.selected,
			BlockText:   block.Text,
			CardKind:    kindOf(c.kind),
			Ordinal:     c.ordinal,
			CardFront:   face.Front,
			CardBack:    face.Back,
		}
		if len(link) > 0 {
			excerpt.LinkTitle = link[0].Title
			excerpt.LinkBlock = string(link[0].Block)
		}
		out = append(out, excerpt)
	}
	return out
}
