package store

import (
	"fmt"
	"time"

	"github.com/transmit-bug/remit/internal/core"
)

// The statistics read what a library holds without opening a source file. Two
// answers are needed for that: how much was produced, and how many excerpt
// notes were last written in a range. Both come from the derived index in SQL.
// The excerpt edge is the one thing the index does not spell — the links table
// keeps the title a link names, not whether it carries a block anchor — so the
// carrying block's text is parsed here, exactly as UnExcerptedNotes parses it.

// Volume is what a library holds as the index counts it: the characters of note
// text, the notes that still carry at least one card, the notes that are
// excerpts of another note, and the cards themselves.
type Volume struct {
	Characters int
	Notes      int
	Excerpts   int
	Cards      int
}

// Volume counts a library's production from the derived index: block text and
// the card count in SQL, and the excerpt edge from the links table plus the
// block text an anchor is written in. No note file is opened and no log line
// replayed, so the cost follows the index, not the library.
func (l *Library) Volume() (Volume, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return Volume{}, ErrClosed
	}

	var out Volume
	err := l.index.db.QueryRow(`SELECT COALESCE(SUM(LENGTH(text)), 0) FROM blocks`).
		Scan(&out.Characters)
	if err != nil {
		return Volume{}, fmt.Errorf("store: text volume: %w", err)
	}
	out.Notes, err = l.index.count(`SELECT COUNT(DISTINCT b.note_id) FROM cards c JOIN blocks b ON b.id = c.block_id`)
	if err != nil {
		return Volume{}, err
	}
	out.Cards, err = l.index.count(`SELECT COUNT(*) FROM cards`)
	if err != nil {
		return Volume{}, err
	}
	saves, err := l.index.excerptSaves()
	if err != nil {
		return Volume{}, err
	}
	out.Excerpts = len(saves)
	return out, nil
}

// ExcerptsBetween counts the excerpt notes whose newest save is in [from, to).
// A zero bound is open. The index keeps one stamp per note — its last save — so
// an excerpt counts on the day it was last written; the library keeps no
// creation stamp to count it on the day it was taken.
func (l *Library) ExcerptsBetween(from, to time.Time) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return 0, ErrClosed
	}
	saves, err := l.index.excerptSaves()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, updated := range saves {
		if !from.IsZero() && updated.Before(from) {
			continue
		}
		if !to.IsZero() && !updated.Before(to) {
			continue
		}
		count++
	}
	return count, nil
}

// excerptSaves maps each excerpt note to its newest save. An excerpt is a note
// whose block carries a link anchored to a block of another note — the
// reference core.Excerpt writes, and the edge the excerpt tree is built from. A
// note naming its own block is not its own excerpt, as Backlinks agrees.
func (x *index) excerptSaves() (map[core.ID]time.Time, error) {
	rows, err := x.db.Query(`
		SELECT DISTINCT l.note_id, n.title, n.updated, b.text
		  FROM links l
		  JOIN notes n ON n.id = l.note_id
		  JOIN blocks b ON b.id = l.block_id`)
	if err != nil {
		return nil, fmt.Errorf("store: excerpts query: %w", err)
	}
	defer rows.Close()

	out := map[core.ID]time.Time{}
	for rows.Next() {
		var (
			note, title, text string
			updated           int64
		)
		if err := rows.Scan(&note, &title, &updated, &text); err != nil {
			return nil, err
		}
		own := core.NormalizeTitle(title)
		for _, link := range core.Links(text) {
			if link.Block == "" || core.NormalizeTitle(link.Title) == own {
				continue
			}
			id := core.ID(note)
			if at := time.Unix(0, updated).UTC(); at.After(out[id]) {
				out[id] = at
			}
			break
		}
	}
	return out, rows.Err()
}
