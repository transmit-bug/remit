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
}

// Sides returns the prompt and the answer the card shows, and where the text
// came from. Overrides win over the block, and either side alone may be
// overridden.
func (c Card) Sides(src Blocks) (prompt, answer string, state LinkState) {
	text, state := c.Ref.Text(src)
	prompt, answer = Split(text)
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
