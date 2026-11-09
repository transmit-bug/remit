// Package core holds Remit's domain: notes, blocks, cards and the rules that
// connect them. It knows nothing about windows, and nothing about how a
// library is stored on disk.
package core

import (
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

// DueOverride is a due date the user set on a card by hand, and the moment they
// set it: the one departure from the schedule the review log replays, and it
// says both so it can stop standing. A review written after the card was moved
// answers the card again, and an answer newer than the hand-set date takes the
// card back — anything sooner would leave a card the user answered sitting on a
// date they chose last week.
//
// It is a schedule fact and not a field of the card: it changes when the card
// comes up and nothing about what the card says.
type DueOverride struct {
	// At is the moment the card is due.
	At time.Time
	// Set is when the override was written. Zero means unknown, and an
	// override with no set moment stands until the card is reviewed.
	Set time.Time
}

// Stands reports whether the override still governs a card whose newest review
// was at lastReview (zero when no review has touched it).
func (d *DueOverride) Stands(lastReview time.Time) bool {
	if d == nil || d.At.IsZero() {
		return false
	}
	return d.Set.IsZero() || lastReview.IsZero() || !lastReview.After(d.Set)
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

// Producer names what derived a card: the plugin that made it and the version
// that made it, so a card stays readable and explainable after its plugin is
// unloaded or updated.
type Producer struct {
	ID      string
	Version string
}

// Derived is one card a derive pass answered, before it has an identity or a
// schedule: which type it is, the field values it was given, and which of the
// paragraph's cards it is. The shapes are the plugin's; this is only how they
// travel.
type Derived struct {
	Type     ID
	Ordinal  int
	Fields   map[string]string
	Producer Producer
}

// Card is an independent review entity: it references exactly one block and
// owns its scheduling history.
//
// A card persists only type, ordinal and producer. Its field values are
// resolved at render time — live from the block, frozen from the snapshot —
// so the store never derives fields, and a hand edit of the block reaches
// every card that follows it. Fields is written only for values the user
// hand-edited (Edited says there are any), which is the same semantics the
// prompt/answer overrides used to carry.
type Card struct {
	ID     ID
	Ref    Reference
	Type   ID
	Fields map[string]string
	Edited bool
	Status Status
	// Ordinal says which of its paragraph's cards this is: one paragraph can
	// ask several things (each blank, both directions), counted from 0.
	Ordinal  int
	Producer Producer
	// Due is the due date the user set by hand, nil while the card keeps the
	// date its reviews derive. It is written to the card file, so it survives a
	// rebuilt index; a review written after it was set takes the card back.
	Due *DueOverride
}
