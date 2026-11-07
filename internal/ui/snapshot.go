package ui

import (
	"github.com/transmit-bug/remit/internal/core"
)

// libraryData is the library as the views read it: a snapshot the app retakes
// after every write, so that no frame has to touch the store. Everything the
// notes list, the editor, review and browse show is derived here once, per
// write.
type libraryData struct {
	notes  []core.Note
	cards  []core.Card
	byID   map[core.ID]core.Card
	blocks core.BlockSet

	// titles name a note, so a card can say which note it came from.
	titles map[core.ID]string
	// perBlock and perNote are the card counts the editor's badge and the
	// note's summary show.
	perBlock map[core.ID]int
	perNote  map[core.ID]int
	// noteOf is the note a block belongs to, so a card can open its source.
	noteOf map[core.ID]core.ID
	// tags counts how many notes carry each tag, and noteTags is what each note
	// carries: the sidebar's filter reads them without asking the store.
	tags     map[string]int
	noteTags map[core.ID][]string
}

// newLibraryData derives the snapshot from the store's notes and cards.
func newLibraryData(notes []core.Note, cards []core.Card) libraryData {
	d := libraryData{
		notes:    notes,
		tags:     map[string]int{},
		noteTags: map[core.ID][]string{},
		cards:    cards,
		byID:     make(map[core.ID]core.Card, len(cards)),
		blocks:   make(core.BlockSet),
		titles:   make(map[core.ID]string, len(notes)),
		perBlock: make(map[core.ID]int, len(cards)),
		perNote:  make(map[core.ID]int, len(notes)),
		noteOf:   make(map[core.ID]core.ID, len(notes)),
	}
	for _, note := range notes {
		d.titles[note.ID] = note.Title
		seen := map[string]bool{}
		for _, block := range note.Blocks {
			if block.ID == "" {
				continue
			}
			d.blocks[block.ID] = block
			d.noteOf[block.ID] = note.ID
			for _, tag := range core.Tags(block.Text) {
				if seen[tag] {
					continue
				}
				seen[tag] = true
				d.noteTags[note.ID] = append(d.noteTags[note.ID], tag)
				d.tags[tag]++
			}
		}
	}
	for _, card := range cards {
		d.byID[card.ID] = card
		d.perBlock[card.Ref.Block]++
		if note, ok := d.noteOf[card.Ref.Block]; ok {
			d.perNote[note]++
		}
	}
	return d
}

// card is a card as the library has it now.
func (d libraryData) card(id core.ID) (core.Card, bool) {
	card, ok := d.byID[id]
	return card, ok
}

// cardsForBlock counts the cards that reference a block.
func (d libraryData) cardsForBlock(id core.ID) int { return d.perBlock[id] }

// cardsInNote counts the cards that reference any block of a note.
func (d libraryData) cardsInNote(note core.Note) int { return d.perNote[note.ID] }

// noteOfBlock finds the note that holds a block.
func (d libraryData) noteOfBlock(id core.ID) (core.ID, bool) {
	note, ok := d.noteOf[id]
	return note, ok
}

// titleOf names a note.
func (d libraryData) titleOf(note core.ID) string { return d.titles[note] }
