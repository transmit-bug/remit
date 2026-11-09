package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// The query layer answers what a list shows: which rows, in which order, one
// page at a time. It runs on the derived index in SQL, so a page costs the rows
// it returns rather than a walk over every note or card, and it never reads a
// source file. A caller that wants the text of the rows it got asks Note or
// Card for them.

// NoteSort names the order QueryNotes returns notes in. A value the index does
// not know falls back to the default.
type NoteSort string

const (
	// NoteSortUpdated lists newest save first, as the notebook does.
	NoteSortUpdated NoteSort = "updated"
	// NoteSortTitle lists by title, ignoring case.
	NoteSortTitle NoteSort = "title"
	// NoteSortCreated lists oldest save first. A note's source carries one
	// moment — its last save — so this is the order the notes were written in
	// until one of them is edited; the format has no creation stamp to order by.
	NoteSortCreated NoteSort = "created"
)

// NoteQuery is what a note list asks for: a filter, an order and a page.
type NoteQuery struct {
	// Text matches a note whose title or any block contains it, ignoring case.
	Text string
	// Tag matches a note carrying that tag, by its bare name.
	Tag  string
	Sort NoteSort
	// Limit is the page size. Zero or less means no limit.
	Limit int
	// Offset skips that many matches.
	Offset int
}

// NoteSummary is a note as a list shows it: no block text, so a page of a
// hundred notes costs no text read.
type NoteSummary struct {
	ID      core.ID
	Title   string
	Updated time.Time
}

// CardSort names the order QueryCards returns cards in. A value the index does
// not know falls back to the default.
type CardSort string

const (
	// CardSortCreated lists oldest card first, as Cards does.
	CardSortCreated CardSort = "created"
	// CardSortDue lists soonest due first: the replayed due moment, or the
	// moment of creation for a card no review has scheduled.
	CardSortDue CardSort = "due"
	// CardSortTitle lists by the title of the note the card's block lives in.
	CardSortTitle CardSort = "title"
)

// CardQuery is what a card list asks for: a filter, an order and a page.
type CardQuery struct {
	// Text matches a card whose block, or whose edited field values, contains
	// it, ignoring case.
	Text string
	// Tag matches a card whose note carries that tag, by its bare name — the
	// same notes NotesWithTag answers for.
	Tag string
	// Note matches a card whose block lives in that note.
	Note string
	// Status matches a card in that lifecycle state; nil matches any.
	Status *core.Status
	Sort   CardSort
	// Limit is the page size. Zero or less means no limit.
	Limit int
	// Offset skips that many matches.
	Offset int
}

// CardSummary is a card as a list shows it: no field values, so a page costs no
// rendering.
type CardSummary struct {
	ID      core.ID
	Block   core.ID
	Type    core.ID
	Status  core.Status
	Created time.Time
	Due     time.Time
}

// QueryNotes returns one page of the notes the query matches, and how many
// notes match in all, so a caller can page. An offset past the end is an empty
// page, not an error.
func (l *Library) QueryNotes(q NoteQuery) ([]NoteSummary, int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, 0, ErrClosed
	}
	where, args := noteWhere(q)
	total, err := l.index.count(`SELECT COUNT(*) FROM notes n`+where, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	page, pageArgs := pageLimit(q.Limit, q.Offset)
	rows, err := l.index.db.Query(
		`SELECT n.id, n.title, n.updated FROM notes n`+where+noteOrder(q.Sort)+page,
		append(args, pageArgs...)...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: notes query: %w", err)
	}
	defer rows.Close()

	var out []NoteSummary
	for rows.Next() {
		var (
			summary NoteSummary
			id      string
			updated int64
		)
		if err := rows.Scan(&id, &summary.Title, &updated); err != nil {
			return nil, 0, err
		}
		summary.ID = core.ID(id)
		summary.Updated = time.Unix(0, updated).UTC()
		out = append(out, summary)
	}
	return out, total, rows.Err()
}

// QueryCards returns one page of the cards the query matches, and how many cards
// match in all, so a caller can page. An offset past the end is an empty page,
// not an error.
func (l *Library) QueryCards(q CardQuery) ([]CardSummary, int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, 0, ErrClosed
	}
	where, args := cardWhere(q)
	total, err := l.index.count(`SELECT COUNT(*) FROM cards c`+where, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	page, pageArgs := pageLimit(q.Limit, q.Offset)
	rows, err := l.index.db.Query(
		`SELECT c.id, c.block_id, c.type, c.status, c.created, COALESCE(s.due, c.created)`+
			fromCards+where+cardOrder(q.Sort)+page, append(args, pageArgs...)...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: cards query: %w", err)
	}
	defer rows.Close()

	var out []CardSummary
	for rows.Next() {
		var (
			summary                CardSummary
			id, block, typ, status string
			created, due           int64
		)
		if err := rows.Scan(&id, &block, &typ, &status, &created, &due); err != nil {
			return nil, 0, err
		}
		summary.ID, summary.Block, summary.Type = core.ID(id), core.ID(block), core.ID(typ)
		summary.Status, _ = core.ParseStatus(status)
		summary.Created = time.Unix(0, created).UTC()
		summary.Due = time.Unix(0, due).UTC()
		out = append(out, summary)
	}
	return out, total, rows.Err()
}

// noteWhere is the filter QueryNotes and its count share, so the total always
// describes the page.
func noteWhere(q NoteQuery) (string, []any) {
	var where []string
	var args []any
	if text := strings.TrimSpace(q.Text); text != "" {
		like := "%" + escapeLike(text) + "%"
		where = append(where, `(n.title LIKE ? ESCAPE '\' OR EXISTS (
			SELECT 1 FROM blocks b WHERE b.note_id = n.id AND b.text LIKE ? ESCAPE '\'))`)
		args = append(args, like, like)
	}
	if tag := strings.TrimSpace(q.Tag); tag != "" {
		where = append(where, `EXISTS (SELECT 1 FROM tags t WHERE t.note_id = n.id AND t.tag = ?)`)
		args = append(args, tag)
	}
	if len(where) == 0 {
		return "", nil
	}
	return ` WHERE ` + strings.Join(where, ` AND `), args
}

// noteOrder is a note list's ORDER BY. Every order ends on the ID, so two notes
// that tie stay in one order across pages.
func noteOrder(sort NoteSort) string {
	switch sort {
	case NoteSortTitle:
		return ` ORDER BY lower(n.title), n.id`
	case NoteSortCreated:
		return ` ORDER BY n.updated, n.id`
	}
	return ` ORDER BY n.updated DESC, n.id`
}

// cardWhere is the filter QueryCards and its count share, so the total always
// describes the page.
func cardWhere(q CardQuery) (string, []any) {
	var where []string
	var args []any
	if text := strings.TrimSpace(q.Text); text != "" {
		like := "%" + escapeLike(text) + "%"
		where = append(where, `(EXISTS (SELECT 1 FROM blocks b WHERE b.id = c.block_id AND b.text LIKE ? ESCAPE '\')
			OR c.fields_text LIKE ? ESCAPE '\')`)
		args = append(args, like, like)
	}
	if tag := strings.TrimSpace(q.Tag); tag != "" {
		where = append(where, `c.block_id IN (
			SELECT b.id FROM tags t JOIN blocks b ON b.note_id = t.note_id WHERE t.tag = ?)`)
		args = append(args, tag)
	}
	if note := strings.TrimSpace(q.Note); note != "" {
		where = append(where, `c.block_id IN (SELECT id FROM blocks WHERE note_id = ?)`)
		args = append(args, note)
	}
	if q.Status != nil {
		where = append(where, `c.status = ?`)
		args = append(args, q.Status.String())
	}
	if len(where) == 0 {
		return "", nil
	}
	return ` WHERE ` + strings.Join(where, ` AND `), args
}

// cardNoteTitle is the title of the note a card's block lives in, which is the
// title a card list sorts by: a card has none of its own.
const cardNoteTitle = `COALESCE((SELECT n.title FROM blocks b JOIN notes n ON n.id = b.note_id
	WHERE b.id = c.block_id ORDER BY b.note_id, b.position LIMIT 1), '')`

// cardOrder is a card list's ORDER BY. The due moment is the schedule the index
// cached, or the card's creation for one no review has scheduled, which is the
// order the queue puts it in. Every order ends on the ID, so two cards that tie
// stay in one order across pages.
func cardOrder(sort CardSort) string {
	switch sort {
	case CardSortDue:
		return ` ORDER BY COALESCE(s.due, c.created), c.id`
	case CardSortTitle:
		return ` ORDER BY lower(` + cardNoteTitle + `), c.id`
	}
	return ` ORDER BY c.created, c.id`
}

// pageLimit is the LIMIT/OFFSET clause of a page and its arguments. A limit of
// zero or less is no limit, which SQLite spells -1; an offset past the end is an
// empty page.
func pageLimit(limit, offset int) (string, []any) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		if offset == 0 {
			return "", nil
		}
		return ` LIMIT -1 OFFSET ?`, []any{offset}
	}
	return ` LIMIT ? OFFSET ?`, []any{limit, offset}
}

// count runs a SELECT COUNT(*) and answers how many rows it found.
func (x *index) count(query string, args ...any) (int, error) {
	var n int
	if err := x.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count query: %w", err)
	}
	return n, nil
}

// UnExcerptedNotes returns the notes that were read and have no excerpt yet:
// notes a reading session touched, with no child note pointing at one of their
// blocks. Most recently read first, then by ID so two calls agree, at most limit
// of them — zero or less is no limit.
//
// A child is a note whose block carries an anchored link to the source — the
// reference core.Excerpt writes, and the edge the excerpt tree is built from. A
// passing mention of a note by title is not an excerpt of it. A note the reading
// queue no longer holds is not a candidate either: an entry the user dropped is
// not a note to be reminded of, and the terminal states a note can be archived
// or dismissed into leave the queue the same way.
//
// The answer comes from the derived index — one query over the reading rows, one
// over the link edges — so it opens no note file and replays no log line.
func (l *Library) UnExcerptedNotes(limit int) ([]Reading, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	children, err := l.index.excerptChildren()
	if err != nil {
		return nil, err
	}
	rows, err := l.index.db.Query(`
		SELECT r.note, r.stability, r.difficulty, r.due, r.last_review, r.reviews,
		       r.last_rating, r.position, r.queued, n.title
		  FROM reading r JOIN notes n ON n.id = r.note
		 WHERE r.reviews > 0 AND r.queued = 1
		 ORDER BY r.last_review DESC, r.note`)
	if err != nil {
		return nil, fmt.Errorf("store: unexcerpted query: %w", err)
	}
	defer rows.Close()

	var out []Reading
	for rows.Next() {
		var (
			reading                        Reading
			note, lastRating, place, title string
			due, lastReview                int64
			queued                         int
		)
		if err := rows.Scan(&note, &reading.State.Memory.Stability, &reading.State.Memory.Difficulty,
			&due, &lastReview, &reading.State.Reviews, &lastRating, &place, &queued, &title); err != nil {
			return nil, err
		}
		// Every row here was read, so the schedule it carries is its own.
		reading.Note = core.ID(note)
		reading.Position = core.ID(place)
		reading.Queued = queued == 1
		reading.State.Due = time.Unix(0, due).UTC()
		reading.State.LastReview = time.Unix(0, lastReview).UTC()
		reading.State.LastRating, _ = schedule.ParseRating(lastRating)
		if hasExcerpt(children, reading.Note, title) {
			continue
		}
		out = append(out, reading)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out, rows.Err()
}

// excerptChildren answers which notes name a title through an excerpt: a link
// carrying a block anchor, written in another note. The index keeps the block a
// link is written in and not the block it names, so the carrying block's text is
// where the anchor is read. A mention without an anchor names the note as a
// whole and is not an excerpt of one of its blocks.
func (x *index) excerptChildren() (map[string][]core.ID, error) {
	rows, err := x.db.Query(`SELECT l.note_id, b.text FROM links l JOIN blocks b ON b.id = l.block_id`)
	if err != nil {
		return nil, fmt.Errorf("store: excerpts query: %w", err)
	}
	defer rows.Close()
	out := map[string][]core.ID{}
	for rows.Next() {
		var note, text string
		if err := rows.Scan(&note, &text); err != nil {
			return nil, err
		}
		for _, link := range core.Links(text) {
			if link.Block == "" {
				continue
			}
			title := core.NormalizeTitle(link.Title)
			out[title] = append(out[title], core.ID(note))
		}
	}
	return out, rows.Err()
}

// hasExcerpt reports whether a note of that title has a child: another note
// naming one of its blocks, which is what an excerpt writes. A note naming its
// own block is not its own child, as Backlinks agrees.
func hasExcerpt(children map[string][]core.ID, note core.ID, title string) bool {
	for _, child := range children[core.NormalizeTitle(title)] {
		if child != note {
			return true
		}
	}
	return false
}

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

// SearchCards returns the cards whose source block, or whose hand-edited field
// values, contain text, ignoring case. The search runs on the derived index, so
// it costs nothing on the files and needs no text in memory.
//
// It answers with the same cards the ranked Search does, in the oldest-first
// order a listing has: it is the unranked reading of the same corpus, kept for
// the callers that want every match rather than a page of the best ones.
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

// SearchNotes returns the notes whose text contains text, ignoring case.
//
// It searches a note's blocks and not its title, and leaves note templates out,
// which is what its callers — the notebook's search — have always received. A
// caller that wants the ranked answer over titles and bodies asks Search.
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
		note := core.Note{ID: id, Title: row.title, Blocks: blocks, Updated: row.updated}
		// A note template answers searches for the text it holds — it is a
		// note — but the notebook's search leaves it out, as the list does: a
		// template is a starting point, not a document to find (#154).
		if core.IsTemplate(note) {
			continue
		}
		notes = append(notes, note)
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

// searchCards matches the text a card renders from: the block it points at, and
// the field values the user edited over it. Field rows in a block body ("Field:
// value") are block text, so they match the first clause.
func (x *index) searchCards(text string) ([]core.ID, error) {
	like := "%" + escapeLike(text) + "%"
	rows, err := x.db.Query(`
		SELECT c.id FROM cards c
		WHERE EXISTS (SELECT 1 FROM blocks b WHERE b.id = c.block_id AND b.text LIKE ? ESCAPE '\')
		   OR c.fields_text LIKE ? ESCAPE '\'
		ORDER BY c.created, c.id`, like, like)
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
