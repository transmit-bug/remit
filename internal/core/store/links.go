package store

import (
	"fmt"

	"github.com/transmit-bug/remit/internal/core"
)

// Backlink is one paragraph that points at a note: where it is, what it says,
// and which note it is in.
type Backlink struct {
	Note      core.ID
	NoteTitle string
	Block     core.ID
	Paragraph int // counted from one, as the user counts paragraphs
	Text      string
}

// putLinks records the wiki links a note's blocks carry, one row per link.
func putLinks(e execer, note core.ID, blocks []core.Block) error {
	for at, block := range blocks {
		for _, link := range core.Links(block.Text) {
			if _, err := e.Exec(
				`INSERT INTO links (note_id, block_id, title, paragraph) VALUES (?, ?, ?, ?)`,
				string(note), string(block.ID), core.NormalizeTitle(link.Title), at+1,
			); err != nil {
				return fmt.Errorf("store: record link %q: %w", link.Title, err)
			}
		}
	}
	return nil
}

// deleteLinksOfNote drops a note's links before its text is read again.
func deleteLinksOfNote(e execer, note core.ID) error {
	_, err := e.Exec(`DELETE FROM links WHERE note_id = ?`, string(note))
	return err
}

// Backlinks returns the paragraphs that name a note, by title, grouped by the
// note they are in and in document order. A note does not link to itself.
func (l *Library) Backlinks(note core.ID) []Backlink {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil
	}
	row, ok := l.index.note(note)
	if !ok {
		return nil
	}
	rows, err := l.index.db.Query(
		`SELECT l.note_id, n.title, l.block_id, l.paragraph, b.text
		   FROM links l
		   JOIN notes n ON n.id = l.note_id
		   JOIN blocks b ON b.id = l.block_id
		  WHERE l.title = ? AND l.note_id != ?
		  ORDER BY n.updated DESC, n.title, l.paragraph`,
		core.NormalizeTitle(row.title), string(note))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Backlink
	for rows.Next() {
		var link Backlink
		var noteID, blockID string
		if err := rows.Scan(&noteID, &link.NoteTitle, &blockID, &link.Paragraph, &link.Text); err != nil {
			return nil
		}
		link.Note = core.ID(noteID)
		link.Block = core.ID(blockID)
		out = append(out, link)
	}
	return out
}
