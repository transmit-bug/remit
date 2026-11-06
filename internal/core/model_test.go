package core

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func TestSplit(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantPrompt string
		wantAnswer string
	}{
		{"both sides", "What is a block? :: A piece of a note.", "What is a block?", "A piece of a note."},
		{"first separator wins", "a :: b :: c", "a", "b :: c"},
		{"prompt only", "just a prompt", "just a prompt", ""},
		{"empty answer", "prompt ::", "prompt", ""},
		{"empty", "", "", ""},
		{"trims both sides", "  q  ::  a  ", "q", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, answer := Split(tt.text)
			if prompt != tt.wantPrompt || answer != tt.wantAnswer {
				t.Errorf("Split(%q) = (%q, %q), want (%q, %q)",
					tt.text, prompt, answer, tt.wantPrompt, tt.wantAnswer)
			}
		})
	}
}

func TestReferenceText(t *testing.T) {
	block := Block{ID: NewID("block"), Text: "live text"}
	src := BlockSet{block.ID: block}
	missing := BlockSet{}
	snapshot := &Snapshot{Text: "frozen text", At: t0}

	tests := []struct {
		name      string
		ref       Reference
		src       Blocks
		wantText  string
		wantState LinkState
	}{
		{"live, block present", Reference{Block: block.ID}, src, "live text", LinkLive},
		{"live, block gone", Reference{Block: block.ID}, missing, "", LinkLost},
		{"frozen, block present", Reference{Block: block.ID, Frozen: snapshot}, src, "frozen text", LinkFrozen},
		{"frozen, block gone", Reference{Block: block.ID, Frozen: snapshot}, missing, "frozen text", LinkFrozen},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, state := tt.ref.Text(tt.src)
			if text != tt.wantText || state != tt.wantState {
				t.Errorf("Text() = (%q, %v), want (%q, %v)", text, state, tt.wantText, tt.wantState)
			}
		})
	}
}

func TestFreezeKeepsTheTextItSaw(t *testing.T) {
	block := Block{ID: NewID("block"), Text: "as written"}
	src := BlockSet{block.ID: block}
	ref := Reference{Block: block.ID}

	frozen := ref.Freeze(src, t0)
	if frozen.Frozen == nil || frozen.Frozen.Text != "as written" {
		t.Fatalf("Freeze kept %+v, want the block's text", frozen.Frozen)
	}
	if !frozen.Frozen.At.Equal(t0) {
		t.Errorf("snapshot time %v, want %v", frozen.Frozen.At, t0)
	}

	// The block moves on; the frozen reference does not.
	src[block.ID] = Block{ID: block.ID, Text: "rewritten"}
	if text, state := frozen.Text(src); text != "as written" || state != LinkFrozen {
		t.Errorf("frozen Text() = (%q, %v), want the snapshot", text, state)
	}

	thawed := frozen.Thaw()
	if text, state := thawed.Text(src); text != "rewritten" || state != LinkLive {
		t.Errorf("thawed Text() = (%q, %v), want the block as it is now", text, state)
	}
}

func TestCardSides(t *testing.T) {
	block := Block{ID: NewID("block"), Text: "what :: that"}
	src := BlockSet{block.ID: block}
	snapshot := &Snapshot{Text: "frozen :: answer", At: t0}

	tests := []struct {
		name       string
		card       Card
		wantPrompt string
		wantAnswer string
		wantState  LinkState
	}{
		{
			name:       "from the block",
			card:       Card{Ref: Reference{Block: block.ID}},
			wantPrompt: "what",
			wantAnswer: "that",
			wantState:  LinkLive,
		},
		{
			name:       "prompt overridden",
			card:       Card{Ref: Reference{Block: block.ID}, Prompt: "why"},
			wantPrompt: "why",
			wantAnswer: "that",
			wantState:  LinkLive,
		},
		{
			name:       "answer overridden",
			card:       Card{Ref: Reference{Block: block.ID}, Answer: "because"},
			wantPrompt: "what",
			wantAnswer: "because",
			wantState:  LinkLive,
		},
		{
			name:       "frozen",
			card:       Card{Ref: Reference{Block: block.ID, Frozen: snapshot}},
			wantPrompt: "frozen",
			wantAnswer: "answer",
			wantState:  LinkFrozen,
		},
		{
			name:       "lost",
			card:       Card{Ref: Reference{Block: NewID("block")}},
			wantPrompt: "",
			wantAnswer: "",
			wantState:  LinkLost,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, answer, state := tt.card.Sides(src)
			if prompt != tt.wantPrompt || answer != tt.wantAnswer || state != tt.wantState {
				t.Errorf("Sides() = (%q, %q, %v), want (%q, %q, %v)",
					prompt, answer, state, tt.wantPrompt, tt.wantAnswer, tt.wantState)
			}
		})
	}
}

func TestStatusRoundTrips(t *testing.T) {
	all := []Status{StatusNew, StatusLearning, StatusReview, StatusSuspended, StatusReading}
	for _, s := range all {
		got, ok := ParseStatus(s.String())
		if !ok || got != s {
			t.Errorf("ParseStatus(%q) = (%v, %v), want %v", s.String(), got, ok, s)
		}
	}
	if _, ok := ParseStatus("archived"); ok {
		t.Error("ParseStatus accepted an unknown name")
	}
	if got := Status(99).String(); got != "unknown" {
		t.Errorf("unknown status String() = %q", got)
	}
}

func TestStatusScheduled(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusNew, true},
		{StatusLearning, true},
		{StatusReview, true},
		{StatusSuspended, false},
		{StatusReading, false},
	}
	for _, tt := range tests {
		if got := tt.status.Scheduled(); got != tt.want {
			t.Errorf("%v.Scheduled() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestNoteBlock(t *testing.T) {
	a := Block{ID: NewID("block"), Text: "a"}
	b := Block{ID: NewID("block"), Text: "b"}
	note := Note{ID: NewID("note"), Blocks: []Block{a, b}}

	if got, ok := note.Block(b.ID); !ok || got.Text != "b" {
		t.Errorf("Block(%q) = (%+v, %v), want the second block", b.ID, got, ok)
	}
	if _, ok := note.Block(NewID("block")); ok {
		t.Error("Block found a block that is not in the note")
	}
}

func TestBlockSet(t *testing.T) {
	b := Block{ID: NewID("block"), Text: "text"}
	src := BlockSet{b.ID: b}

	got, ok := src.Block(b.ID)
	if !ok || got.Text != "text" {
		t.Errorf("Block(%q) = (%+v, %v)", b.ID, got, ok)
	}
	if _, ok := src.Block(NewID("block")); ok {
		t.Error("Block found a block that is not in the set")
	}
}

// A paragraph derives as many cards as it asks: each blank of a cloze, both
// directions of a both-ways paragraph, one for a single separator, none for
// plain text.
func TestDerive(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []Derived
	}{
		{
			name: "plain text asks nothing",
			text: "A paragraph of a note.",
			want: nil,
		},
		{
			name: "one separator asks one",
			text: "Spacing beats cramming :: reviewing just before you forget",
			want: []Derived{{Kind: KindBasic, Ordinal: 0,
				Prompt: "Spacing beats cramming", Answer: "reviewing just before you forget"}},
		},
		{
			name: "several separators still ask one, split at the first",
			text: "A block divides at :: the first :: prompt before, answer after.",
			want: []Derived{{Kind: KindBasic, Ordinal: 0,
				Prompt: "A block divides at", Answer: "the first :: prompt before, answer after."}},
		},
		{
			name: "two blanks ask two, each hiding its own",
			text: "The {{first}} blank and the {{second}} one.",
			want: []Derived{
				{Kind: KindCloze, Ordinal: 0, Prompt: "The […] blank and the second one.", Answer: "first"},
				{Kind: KindCloze, Ordinal: 1, Prompt: "The first blank and the […] one.", Answer: "second"},
			},
		},
		{
			name: "both ways asks two, reversed",
			text: "spacing ::: the gaps between reviews",
			want: []Derived{
				{Kind: KindBidirectional, Ordinal: 0, Prompt: "spacing", Answer: "the gaps between reviews"},
				{Kind: KindBidirectional, Ordinal: 1, Prompt: "the gaps between reviews", Answer: "spacing"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Derive(c.text)
			if len(got) != len(c.want) {
				t.Fatalf("derived %d cards, want %d: %+v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("card %d is %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// A card's sides follow its paragraph while the reference is live, and the
// paragraph's own shape picks which of its cards this is.
func TestClozeAndBothWaySides(t *testing.T) {
	src := BlockSet{"block_x": {ID: "block_x", Text: "The {{first}} blank asks {{second}}."}}
	cases := []struct {
		name  string
		card  Card
		wantP string
		wantA string
	}{
		{"live cloze, first blank", Card{Ref: Reference{Block: "block_x"}, Kind: KindCloze, Ordinal: 0},
			"The […] blank asks second.", "first"},
		{"live cloze, second blank", Card{Ref: Reference{Block: "block_x"}, Kind: KindCloze, Ordinal: 1},
			"The first blank asks […].", "second"},
		{"live cloze, blank gone", Card{Ref: Reference{Block: "block_x"}, Kind: KindCloze, Ordinal: 5},
			"The first blank asks second.", ""},
		{"override wins on the prompt", Card{Ref: Reference{Block: "block_x"}, Kind: KindCloze, Ordinal: 0, Prompt: "asked another way"},
			"asked another way", "first"},
	}

	both := BlockSet{"block_x": {ID: "block_x", Text: "spacing ::: the gaps between reviews"}}
	cases = append(cases,
		struct {
			name  string
			card  Card
			wantP string
			wantA string
		}{"both ways, as written", Card{Ref: Reference{Block: "block_x"}, Kind: KindBidirectional}, "spacing", "the gaps between reviews"},
		struct {
			name  string
			card  Card
			wantP string
			wantA string
		}{"both ways, reversed", Card{Ref: Reference{Block: "block_x"}, Kind: KindBidirectional, Ordinal: 1}, "the gaps between reviews", "spacing"},
	)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			blocks := src
			if c.card.Kind == KindBidirectional {
				blocks = both
			}
			prompt, answer, state := c.card.Sides(blocks)
			if state != LinkLive {
				t.Errorf("state is %s, want live", state)
			}
			if prompt != c.wantP || answer != c.wantA {
				t.Errorf("sides are %q / %q, want %q / %q", prompt, answer, c.wantP, c.wantA)
			}
		})
	}
}

// A paragraph that holds both separators divides at the both-ways one: the
// earlier :: belongs to the prompt.
func TestBothWaysWithAnEarlierSeparator(t *testing.T) {
	blocks := BlockSet{"block_x": {ID: "block_x", Text: "a :: b ::: c"}}
	derived := Derive("a :: b ::: c")
	if len(derived) != 2 {
		t.Fatalf("derived %d cards, want 2", len(derived))
	}
	for i, want := range []struct{ probe, answer string }{{"a :: b", "c"}, {"c", "a :: b"}} {
		card := Card{Ref: Reference{Block: "block_x"}, Kind: KindBidirectional, Ordinal: i}
		prompt, answer, _ := card.Sides(blocks)
		if prompt != want.probe || answer != want.answer {
			t.Errorf("card %d asks %q / %q, want %q / %q", i, prompt, answer, want.probe, want.answer)
		}
		if derived[i].Prompt != prompt || derived[i].Answer != answer {
			t.Errorf("card %d derives %q / %q but reads %q / %q", i, derived[i].Prompt, derived[i].Answer, prompt, answer)
		}
	}
}

// A frozen card asks the text it kept, and a lost one asks its overrides or
// nothing: the paragraph's changes do not reach either.
func TestFrozenAndLostSides(t *testing.T) {
	src := BlockSet{"block_x": {ID: "block_x", Text: "spacing :: the gaps between reviews"}}
	frozen := Card{
		Ref:  Reference{Block: "block_x", Frozen: &Snapshot{Text: "old :: snapshot"}},
		Kind: KindBasic,
	}
	prompt, answer, state := frozen.Sides(src)
	if state != LinkFrozen || prompt != "old" || answer != "snapshot" {
		t.Errorf("frozen card asks %q / %q (%s)", prompt, answer, state)
	}

	lost := Card{Ref: Reference{Block: "gone"}, Kind: KindBasic, Prompt: "override"}
	prompt, answer, state = lost.Sides(src)
	if state != LinkLost || prompt != "override" || answer != "" {
		t.Errorf("lost card asks %q / %q (%s)", prompt, answer, state)
	}
}
