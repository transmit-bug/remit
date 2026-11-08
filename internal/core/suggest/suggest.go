// Package suggest applies what a plugin's importer answered to the library.
//
// An importer is a plugin the contract lets declare data formats
// (contributes.importers, #153). Its importFile hook reads one file and
// answers suggestions — notes with blocks, and cards on those blocks — and
// this package is the host's side of the contract: every suggestion is
// checked against the library before anything is written, a suggestion that
// does not hold up is dropped with its reason, and what survives is written
// the same way the app's own importers write it. The plugin never touches the
// library; output is a suggestion, never a write.
package suggest

import (
	"fmt"
	"log/slog"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Note is one note a plugin suggests: a title and its blocks' text, in order.
type Note struct {
	Title  string
	Blocks []string
}

// Card is one card a plugin suggests, on a block of the note at index Note:
// the block at Block of that note's blocks. Type, ordinal and fields are the
// derive contract's own shape, and the library's types decide what is real.
type Card struct {
	Note    int
	Block   int
	Type    string
	Ordinal int
	Fields  map[string]string
}

// Suggestions is what one importFile call answered.
type Suggestions struct {
	Notes    []Note
	Cards    []Card
	Warnings []string
}

// Report is what applying one file's suggestions did. Counts are what was
// written, so a second import of a file that suggests the same things reports
// what it wrote again — the plugin decides what that means, the way
// importmd's counts do.
type Report struct {
	Notes    int
	Cards    int
	Dropped  int
	Warnings []string
}

// Apply writes the suggestions that hold up into lib, stamped with producer.
// A note whose title and blocks are all empty is not a note anyone takes; a
// card on a note or block that does not exist, or a type the library does not
// hold, is dropped with a warning — the plugin's words, the host's rules.
func Apply(lib *store.Library, s Suggestions, producer core.Producer) (Report, error) {
	var report Report
	if lib == nil {
		return report, fmt.Errorf("suggest: no library")
	}
	types := map[core.ID]bool{}
	for _, typ := range lib.Types() {
		types[typ.ID] = true
	}

	// Written notes, in order: a card names its note and block by index, so
	// the indexes are of the suggestions as they arrived, not of what
	// survived — a dropped note carries its cards with it.
	blocks := make([][]core.ID, len(s.Notes))
	for i, note := range s.Notes {
		id, ok, warn := applyNote(lib, note)
		if !ok {
			if warn != "" {
				report.Warnings = append(report.Warnings, fmt.Sprintf("notes[%d]: %s", i, warn))
			}
			continue
		}
		report.Notes++
		stored, ok := lib.Note(id)
		if !ok {
			continue
		}
		for _, block := range stored.Blocks {
			blocks[i] = append(blocks[i], block.ID)
		}
	}

	for i, card := range s.Cards {
		if card.Note < 0 || card.Note >= len(blocks) || len(blocks[card.Note]) == 0 {
			report.Dropped++
			report.Warnings = append(report.Warnings, fmt.Sprintf("cards[%d]: notes[%d] was not written, so its cards are not either", i, card.Note))
			continue
		}
		if card.Block < 0 || card.Block >= len(blocks[card.Note]) {
			report.Dropped++
			report.Warnings = append(report.Warnings, fmt.Sprintf("cards[%d]: block %d is not one of notes[%d]'s blocks", i, card.Block, card.Note))
			continue
		}
		id := core.ID(card.Type)
		if !types[id] {
			report.Dropped++
			report.Warnings = append(report.Warnings, fmt.Sprintf("cards[%d]: type %q is not in the library", i, card.Type))
			continue
		}
		if _, err := lib.CreateCards(blocks[card.Note][card.Block], []core.Derived{{
			Type: id, Ordinal: card.Ordinal, Fields: card.Fields, Producer: producer,
		}}); err != nil {
			report.Dropped++
			report.Warnings = append(report.Warnings, fmt.Sprintf("cards[%d]: %v", i, err))
			continue
		}
		report.Cards++
	}
	for _, warning := range s.Warnings {
		slog.Warn("remit: importer", "warning", warning)
	}
	for _, warning := range report.Warnings {
		slog.Warn("remit: importer", "warning", warning)
	}
	return report, nil
}

// applyNote writes one suggested note, or says why it is not one: a title and
// blocks all empty is nothing, and a save that fails is a warning, not an
// error — the file's other notes still land.
func applyNote(lib *store.Library, note Note) (core.ID, bool, string) {
	blocks := make([]core.Block, 0, len(note.Blocks))
	for _, text := range note.Blocks {
		blocks = append(blocks, core.Block{Text: text})
	}
	if note.Title == "" && len(blocks) == 0 {
		return "", false, "a note needs a title or a block"
	}
	saved, err := lib.SaveNote(core.Note{Title: note.Title, Blocks: blocks})
	if err != nil {
		return "", false, err.Error()
	}
	return saved.ID, true, ""
}
