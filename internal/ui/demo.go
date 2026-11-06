package ui

import (
	"time"

	"github.com/transmit-bug/remit/internal/core"
)

// demoLibrary is the in-memory library the prototype runs on. It is the only
// thing that has to go when the real store is wired in: the views read notes,
// blocks and cards, and nothing else.
type demoLibrary struct {
	notes []core.Note
	cards []core.Card
}

// newDemoLibrary returns a small library with the three link states in it: a
// live card, a frozen one whose block has moved on, and a lost one whose block
// is gone.
func newDemoLibrary() *demoLibrary {
	note := func(title string, texts ...string) core.Note {
		n := core.Note{ID: core.NewID("note"), Title: title}
		for _, text := range texts {
			n.Blocks = append(n.Blocks, core.Block{ID: core.NewID("block"), Text: text})
		}
		return n
	}

	memory := note("Memory and recall",
		"Spacing beats cramming :: reviewing just before you forget strengthens recall.",
		"The testing effect :: recalling an answer builds a stronger memory than rereading it.",
		"Interleaving :: mixing topics in one session trains you to tell them apart.",
	)
	writing := note("Writing cards",
		"A good card asks :: one question with one answer.",
		"A block divides at :: the first :: — prompt before, answer after.",
		"Cards point at blocks :: editing the note updates every card that follows it.",
	)
	habit := note("A daily habit",
		"When to review :: when the queue says a card is due, in one short sitting.",
		"What to do when stuck :: make the card smaller instead of repeating it.",
	)

	frozenAt := time.Date(2026, time.January, 8, 9, 0, 0, 0, time.UTC)
	return &demoLibrary{
		notes: []core.Note{memory, writing, habit},
		cards: []core.Card{
			{ID: core.NewID("card"), Ref: core.Reference{Block: memory.Blocks[0].ID}, Status: core.StatusReview},
			// The block is gone and no snapshot was kept.
			{ID: core.NewID("card"), Ref: core.Reference{Block: core.ID("block_2b6f0e5c7a1d4e8f9a0b3c5d7e9f1a2b")}, Status: core.StatusReview},
			// The block still exists, but the card stopped following it.
			{ID: core.NewID("card"), Ref: core.Reference{
				Block:  writing.Blocks[0].ID,
				Frozen: &core.Snapshot{Text: "A good card asks :: one question at a time.", At: frozenAt},
			}, Status: core.StatusReview},
			{ID: core.NewID("card"), Ref: core.Reference{Block: memory.Blocks[1].ID}, Status: core.StatusNew},
			{ID: core.NewID("card"), Ref: core.Reference{Block: writing.Blocks[1].ID}, Status: core.StatusLearning},
		},
	}
}

// blocks returns every block in the library, which is what cards resolve
// their text from.
func (l *demoLibrary) blocks() core.BlockSet {
	set := core.BlockSet{}
	for _, n := range l.notes {
		for _, b := range n.Blocks {
			set[b.ID] = b
		}
	}
	return set
}

// card returns the card with the given ID.
func (l *demoLibrary) card(id core.ID) (*core.Card, bool) {
	for i := range l.cards {
		if l.cards[i].ID == id {
			return &l.cards[i], true
		}
	}
	return nil, false
}

// cardsForBlock counts the cards that reference a block.
func (l *demoLibrary) cardsForBlock(id core.ID) int {
	n := 0
	for _, c := range l.cards {
		if c.Ref.Block == id {
			n++
		}
	}
	return n
}

// cardsInNote counts the cards that reference any block of a note.
func (l *demoLibrary) cardsInNote(note core.Note) int {
	n := 0
	for _, b := range note.Blocks {
		n += l.cardsForBlock(b.ID)
	}
	return n
}

// noteOfBlock finds the note that holds a block.
func (l *demoLibrary) noteOfBlock(id core.ID) (core.Note, bool) {
	for _, n := range l.notes {
		if _, ok := n.Block(id); ok {
			return n, true
		}
	}
	return core.Note{}, false
}
