package store

import (
	"fmt"
	"sort"

	"github.com/transmit-bug/remit/internal/core"
)

// TagCount is one tag and how much of the library carries it: the notes whose
// text names it, and the cards whose source block does — a card's block and its
// note are two doors into the same demand for review.
type TagCount struct {
	Tag   string
	Notes int
	Cards int
}

// putTags records the tags a note's blocks carry, one row per tag per block.
func putTags(e execer, note core.ID, blocks []core.Block) error {
	for _, block := range blocks {
		for _, tag := range core.Tags(block.Text) {
			if _, err := e.Exec(
				`INSERT INTO tags (note_id, block_id, tag) VALUES (?, ?, ?)`,
				string(note), string(block.ID), tag,
			); err != nil {
				return fmt.Errorf("store: record tag %q: %w", tag, err)
			}
		}
	}
	return nil
}

// deleteTagsOfNote drops a note's tags before its text is read again.
func deleteTagsOfNote(e execer, note core.ID) error {
	_, err := e.Exec(`DELETE FROM tags WHERE note_id = ?`, string(note))
	return err
}

// Tags returns every tag in the library with how many notes and cards carry
// it, by name.
func (l *Library) Tags() []TagCount {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil
	}
	// One row per tag per block, joined to the cards that block feeds: the
	// distinct counts keep a tag on several blocks of one note, or on a block
	// with three cards, from being counted once per row.
	rows, err := l.index.db.Query(
		`SELECT t.tag, COUNT(DISTINCT t.note_id), COUNT(DISTINCT c.id)
		   FROM tags t
		   LEFT JOIN cards c ON c.block_id = t.block_id
		  GROUP BY t.tag ORDER BY t.tag`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []TagCount
	for rows.Next() {
		var count TagCount
		if err := rows.Scan(&count.Tag, &count.Notes, &count.Cards); err != nil {
			return nil
		}
		out = append(out, count)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out
}

// NotesWithTag returns the notes that carry a tag, newest save first.
func (l *Library) NotesWithTag(tag string) []core.ID {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil
	}
	rows, err := l.index.db.Query(
		`SELECT DISTINCT t.note_id FROM tags t
		   JOIN notes n ON n.id = t.note_id
		  WHERE t.tag = ? ORDER BY n.updated DESC`, tag)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []core.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil
		}
		out = append(out, core.ID(id))
	}
	return out
}
