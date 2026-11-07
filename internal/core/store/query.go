package store

import (
	"fmt"
	"strings"

	"github.com/transmit-bug/remit/internal/core"
)

// CardsWithStatus returns the cards whose stored status is s, oldest first.
//
// Two statuses are derived rather than stored: a card leaves "new" when its
// first review is logged, and "review" follows from that, so asking for those
// returns what the files say, not what the schedule implies. Filters over the
// schedule belong to the queue and stats packages, which replay it.
func (l *Library) CardsWithStatus(s core.Status) ([]core.Card, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	l.ensureCreated()
	var cards []core.Card
	for _, id := range l.byCreated {
		if meta, ok := l.cards[id]; ok && meta.card.Status == s {
			cards = append(cards, meta.card)
		}
	}
	return cards, nil
}

// SearchCards returns the cards whose source block contains text, ignoring
// case. The search runs on the derived index, so it costs nothing on the files
// and needs no block text in memory.
func (l *Library) SearchCards(text string) ([]core.Card, error) {
	query := strings.TrimSpace(text)
	if query == "" {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	ids, err := l.index.searchCards(query)
	if err != nil {
		return nil, err
	}
	cards := make([]core.Card, 0, len(ids))
	for _, id := range ids {
		if meta, ok := l.cards[id]; ok {
			cards = append(cards, meta.card)
		}
	}
	return cards, nil
}

// SearchNotes returns the notes whose text contains text, ignoring case. Like
// SearchCards it runs on the derived index, one SQL shape for both: a block's
// text matched with LIKE, the user's text escaped to a literal pattern.
func (l *Library) SearchNotes(text string) ([]core.Note, error) {
	query := strings.TrimSpace(text)
	if query == "" {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	ids, err := l.index.searchNotes(query)
	if err != nil {
		return nil, err
	}
	notes := make([]core.Note, 0, len(ids))
	for _, id := range ids {
		row, ok := l.index.note(id)
		if !ok {
			continue
		}
		blocks, err := l.index.blocksOfNote(id)
		if err != nil {
			return nil, err
		}
		notes = append(notes, core.Note{ID: id, Title: row.title, Blocks: blocks, Updated: row.updated})
	}
	return notes, nil
}

// escapeLike makes a user's text a literal pattern: a search for "50%" should
// not match everything.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

// searchCards matches the block text a card renders from.
func (x *index) searchCards(text string) ([]core.ID, error) {
	rows, err := x.db.Query(`
		SELECT c.id FROM cards c
		WHERE EXISTS (SELECT 1 FROM blocks b WHERE b.id = c.block_id AND b.text LIKE ? ESCAPE '\')
		ORDER BY c.created, c.id`, "%"+escapeLike(text)+"%")
	if err != nil {
		return nil, fmt.Errorf("store: search query: %w", err)
	}
	defer rows.Close()
	return scanIDs(rows)
}

// searchNotes matches the block text a note holds, newest save first, as the
// notebook lists notes.
func (x *index) searchNotes(text string) ([]core.ID, error) {
	rows, err := x.db.Query(`
		SELECT n.id FROM notes n
		WHERE EXISTS (SELECT 1 FROM blocks b WHERE b.note_id = n.id AND b.text LIKE ? ESCAPE '\')
		ORDER BY n.updated DESC, n.id`, "%"+escapeLike(text)+"%")
	if err != nil {
		return nil, fmt.Errorf("store: search query: %w", err)
	}
	defer rows.Close()
	return scanIDs(rows)
}
