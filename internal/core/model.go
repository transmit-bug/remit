// Package core holds Remit's domain: notes, blocks, cards and the rules that
// connect them. It knows nothing about windows, and nothing about how a
// library is stored on disk.
package core

import (
	"strings"
	"time"
)

// Separator divides a block's text into the two sides of a card: what comes
// before the first one is the prompt, what comes after is the answer.
const Separator = "::"

// Note is a plaintext document made of ordered blocks.
type Note struct {
	ID     ID
	Title  string
	Blocks []Block
	// Updated is when the note was last saved, as the library reads it back:
	// the notebook lists notes newest first.
	Updated time.Time
}

// Block is a addressable piece of a note. Its ID is written inline in the
// plaintext source, so moving, splitting or reordering the text keeps every
// reference pointing at the same content.
type Block struct {
	ID   ID
	Text string
}

// Block returns the block with the given ID.
func (n Note) Block(id ID) (Block, bool) {
	for _, b := range n.Blocks {
		if b.ID == id {
			return b, true
		}
	}
	return Block{}, false
}

// Blocks looks blocks up by ID: a note, a library, or a test double.
type Blocks interface {
	Block(ID) (Block, bool)
}

// BlockSet is an in-memory Blocks, used by the index and by tests.
type BlockSet map[ID]Block

// Block returns the block with the given ID.
func (s BlockSet) Block(id ID) (Block, bool) {
	b, ok := s[id]
	return b, ok
}

// Snapshot is what a block said when a card stopped following it.
type Snapshot struct {
	Text string
	At   time.Time
}

// LinkState says where a card's text came from, and is what the UI shows when
// a card's source is not what it used to be.
type LinkState uint8

const (
	// LinkLive means the card renders the block as it is now.
	LinkLive LinkState = iota
	// LinkFrozen means the card shows the snapshot it was frozen at.
	LinkFrozen
	// LinkLost means the block is gone and no snapshot was kept.
	LinkLost
)

// String names the state, for logs and for the storage format.
func (s LinkState) String() string {
	switch s {
	case LinkLive:
		return "live"
	case LinkFrozen:
		return "frozen"
	case LinkLost:
		return "lost"
	}
	return "unknown"
}

// Reference links a card to the one block it renders from. A frozen reference
// keeps its snapshot and stops following the block; a lost one has neither.
type Reference struct {
	Block  ID
	Frozen *Snapshot // nil while the reference follows the block
}

// Text returns what the reference shows and where that came from. A frozen
// reference shows its snapshot even when the block is still there; a live one
// whose block is gone resolves to LinkLost rather than to empty content that
// looks real.
func (r Reference) Text(src Blocks) (string, LinkState) {
	if r.Frozen != nil {
		return r.Frozen.Text, LinkFrozen
	}
	b, ok := src.Block(r.Block)
	if !ok {
		return "", LinkLost
	}
	return b.Text, LinkLive
}

// Freeze stops the reference from following its block, keeping what the block
// says now.
func (r Reference) Freeze(src Blocks, at time.Time) Reference {
	text, _ := r.Text(src)
	r.Frozen = &Snapshot{Text: text, At: at}
	return r
}

// Thaw makes the reference follow its block again, discarding the snapshot.
func (r Reference) Thaw() Reference {
	r.Frozen = nil
	return r
}

// Status is where a card is in its life, independent of FSRS scheduling.
type Status uint8

const (
	// StatusNew is a card that has never been reviewed.
	StatusNew Status = iota
	// StatusLearning is a card still being learned.
	StatusLearning
	// StatusReview is a card in long-term review.
	StatusReview
	// StatusSuspended is a card taken out of the queue without losing history.
	StatusSuspended
	// StatusReading is reserved for incremental reading. v1 stores it and
	// filters on it, but has no reading queue yet.
	StatusReading
)

// String names the status, for logs and for the storage format.
func (s Status) String() string {
	switch s {
	case StatusNew:
		return "new"
	case StatusLearning:
		return "learning"
	case StatusReview:
		return "review"
	case StatusSuspended:
		return "suspended"
	case StatusReading:
		return "reading"
	}
	return "unknown"
}

// ParseStatus reads a status back from the name Status.String writes.
func ParseStatus(s string) (Status, bool) {
	switch s {
	case "new":
		return StatusNew, true
	case "learning":
		return StatusLearning, true
	case "review":
		return StatusReview, true
	case "suspended":
		return StatusSuspended, true
	case "reading":
		return StatusReading, true
	}
	return StatusNew, false
}

// Scheduled reports whether the card belongs in the review queue at all.
func (s Status) Scheduled() bool {
	switch s {
	case StatusNew, StatusLearning, StatusReview:
		return true
	}
	return false
}

// Card is an independent review entity: it references exactly one block,
// owns its scheduling history, and may replace the wording the block gives it.
type Card struct {
	ID     ID
	Ref    Reference
	Prompt string // replaces the block's prompt when set
	Answer string // replaces the block's answer when set
	Status Status
	// Kind and Ordinal say which of its paragraph's cards this is: one
	// paragraph can ask several things (each blank, both directions).
	Kind    Kind
	Ordinal int
}

// Sides returns the prompt and the answer the card shows, and where the text
// came from. Overrides win over the block, and either side alone may be
// overridden.
func (c Card) Sides(src Blocks) (prompt, answer string, state LinkState) {
	text, state := c.Ref.Text(src)
	prompt, answer = faces(c.Kind, c.Ordinal, text)
	if c.Prompt != "" {
		prompt = c.Prompt
	}
	if c.Answer != "" {
		answer = c.Answer
	}
	return prompt, answer, state
}

// Split divides block text into prompt and answer at the first Separator. A
// block without one is all prompt, and a block that only separates has an
// empty answer rather than an error.
func Split(text string) (prompt, answer string) {
	before, after, ok := strings.Cut(text, Separator)
	if !ok {
		return strings.TrimSpace(text), ""
	}
	return strings.TrimSpace(before), strings.TrimSpace(after)
}

// The shapes a paragraph can take as a card. Which one it is comes from how it
// is written, so a note carries its own card definitions.
const (
	// ClozeOpen and ClozeClose wrap a blank of a cloze paragraph: every blank
	// is one card, its own blank hidden and the others shown.
	ClozeOpen, ClozeClose = "{{", "}}"
	// BothSeparator asks a paragraph both ways: the sides as written, then
	// reversed. It is checked before Separator, which it begins with.
	BothSeparator = Separator + ":"
)

// Kind is how a paragraph produced a card.
type Kind uint8

const (
	// KindBasic is one paragraph, prompt then answer.
	KindBasic Kind = iota
	// KindCloze is one blank of a paragraph's many.
	KindCloze
	// KindBidirectional is one of the two directions of a paragraph asked both
	// ways: ordinal 0 is as written, 1 is reversed.
	KindBidirectional
)

// String names the kind, as a card file and a form show it.
func (k Kind) String() string {
	switch k {
	case KindCloze:
		return "cloze"
	case KindBidirectional:
		return "bidirectional"
	}
	return "basic"
}

// ParseKind reads a kind back from the name Kind.String writes.
func ParseKind(s string) (Kind, bool) {
	switch s {
	case "basic":
		return KindBasic, true
	case "cloze":
		return KindCloze, true
	case "bidirectional":
		return KindBidirectional, true
	}
	return KindBasic, false
}

// Derived is one card a paragraph makes, before it has an identity or a
// schedule: what to ask, what to accept, and which of the paragraph's cards it
// is.
type Derived struct {
	Kind    Kind
	Ordinal int
	Prompt  string
	Answer  string
}

// Derive returns the cards a paragraph's text makes, in the order they are
// numbered. A paragraph with blanks is a cloze, one card per blank; one that
// separates both ways makes two cards, each the other's reverse; one with a
// single separator makes one; one with none makes none — text alone is a note,
// not a card.
func Derive(text string) []Derived {
	if blanks := clozeBlanks(text); len(blanks) > 0 {
		out := make([]Derived, 0, len(blanks))
		for i, blank := range blanks {
			out = append(out, Derived{
				Kind:    KindCloze,
				Ordinal: i,
				Prompt:  clozePrompt(text, blanks, i),
				Answer:  blank,
			})
		}
		return out
	}
	if before, after, ok := strings.Cut(text, BothSeparator); ok {
		prompt, answer := strings.TrimSpace(before), strings.TrimSpace(after)
		return []Derived{
			{Kind: KindBidirectional, Ordinal: 0, Prompt: prompt, Answer: answer},
			{Kind: KindBidirectional, Ordinal: 1, Prompt: answer, Answer: prompt},
		}
	}
	if prompt, answer := Split(text); strings.Contains(text, Separator) {
		return []Derived{{Kind: KindBasic, Prompt: prompt, Answer: answer}}
	}
	return nil
}

// clozeBlanks is the text of each blank, in the order they appear.
func clozeBlanks(text string) []string {
	var out []string
	for rest := text; ; {
		open := strings.Index(rest, ClozeOpen)
		if open < 0 {
			return out
		}
		rest = rest[open+len(ClozeOpen):]
		end := strings.Index(rest, ClozeClose)
		if end < 0 {
			return out
		}
		out = append(out, strings.TrimSpace(rest[:end]))
		rest = rest[end+len(ClozeClose):]
	}
}

// clozePrompt is the paragraph with every blank shown but the one asked: that
// one is a gap, so the card asks for it and nothing else.
func clozePrompt(text string, blanks []string, ask int) string {
	var out strings.Builder
	rest := text
	for i := 0; ; i++ {
		open := strings.Index(rest, ClozeOpen)
		if open < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:open])
		rest = rest[open+len(ClozeOpen):]
		end := strings.Index(rest, ClozeClose)
		if end < 0 {
			out.WriteString(ClozeOpen + rest)
			break
		}
		blank := strings.TrimSpace(rest[:end])
		rest = rest[end+len(ClozeClose):]
		if i == ask {
			out.WriteString("[…]")
			continue
		}
		out.WriteString(blank)
	}
	return strings.TrimSpace(out.String())
}

// faces is the pair a card asks, from the paragraph's text and how the card was
// derived.
func faces(kind Kind, ordinal int, text string) (prompt, answer string) {
	switch kind {
	case KindCloze:
		blanks := clozeBlanks(text)
		if ordinal >= 0 && ordinal < len(blanks) {
			return clozePrompt(text, blanks, ordinal), blanks[ordinal]
		}
		return clozePrompt(text, blanks, -1), ""
	case KindBidirectional:
		// The both-ways separator is what divided the paragraph, so a card's
		// sides come from it — not from replacing it and splitting again, which
		// would divide at an earlier :: when the paragraph holds both.
		if before, after, ok := strings.Cut(text, BothSeparator); ok {
			prompt, answer = strings.TrimSpace(before), strings.TrimSpace(after)
		} else {
			prompt, answer = Split(text)
		}
		if ordinal == 1 {
			return answer, prompt
		}
		return prompt, answer
	default:
		return Split(text)
	}
}
