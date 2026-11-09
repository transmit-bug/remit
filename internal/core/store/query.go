package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	// NoteSortCards lists by how many cards a note feeds, most first: the note
	// that carries the most of the library's material comes first.
	NoteSortCards NoteSort = "cards"
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

// NoteRowQuery is what the notebook's list asks for: a filter, an order and a
// page. The three toggles mirror the list's own, and no toggle leaves the whole
// notebook.
type NoteRowQuery struct {
	// Text matches a note whose title or any block contains it, ignoring case.
	Text string
	// Tag matches a note carrying that tag, by its bare name.
	Tag string
	// Reading keeps the notes the reading queue holds now.
	Reading bool
	// HasCards keeps the notes at least one card renders from.
	HasCards bool
	// Excerpt keeps the notes that are excerpts: a note pointing into one block
	// of another note.
	Excerpt bool
	Sort    NoteSort
	// Limit is the page size. Zero or less means no limit.
	Limit int
	// Offset skips that many matches.
	Offset int
}

// NoteRow is one line of the notebook's list: what the note is called, how much
// of it there is, and where the reader left it. It carries no block text beyond
// the first block's summary, so a page costs no note read.
type NoteRow struct {
	ID    core.ID
	Title string
	// Summary is the note's first block pressed into one line, at most
	// SummaryRunes of it.
	Summary string
	// Blocks is how many paragraphs the note holds.
	Blocks int
	// Cards is how many cards the note's blocks feed.
	Cards int
	// ReadingPos is the block the last reading session stopped at, empty for a
	// note no session has read. Whether the note is still in the queue is the
	// Reading filter's business, not this field's.
	ReadingPos core.ID
	// ReadingBlocks is the paragraph the reader reached, counted from one — the
	// "6" of "6 of 14" — and zero for a note no session has read.
	ReadingBlocks int
	Updated       time.Time
	// Excerpt says the note is an excerpt: one of its blocks carries an
	// anchored link back into another note.
	Excerpt bool
}

// SummaryRunes is how much of a note's first block a row carries. It is a list's
// one line, and a bound keeps a page of long paragraphs cheap to read.
const SummaryRunes = 240

// QueryNoteRows returns one page of the notebook's rows and how many notes match
// in all, so a caller can page. Note templates are not in the notebook's list —
// a template is a starting point, offered where a note is started (#154). An
// offset past the end is an empty page, not an error.
func (l *Library) QueryNoteRows(q NoteRowQuery) ([]NoteRow, int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, 0, ErrClosed
	}
	where, args := noteRowWhere(q)
	total, err := l.index.count(`SELECT COUNT(*) FROM notes n`+where, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	page, pageArgs := pageLimit(q.Limit, q.Offset)
	rows, err := l.index.db.Query(
		`SELECT `+noteRowColumns+` FROM notes n`+where+noteOrder(q.Sort)+page,
		append(args, pageArgs...)...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: note rows query: %w", err)
	}
	defer rows.Close()

	var out []NoteRow
	for rows.Next() {
		var (
			row          NoteRow
			id, position string
			first        string
			updated      int64
			excerpt      int
		)
		if err := rows.Scan(&id, &row.Title, &first, &row.Blocks, &row.Cards, &position,
			&row.ReadingBlocks, &updated, &excerpt); err != nil {
			return nil, 0, err
		}
		row.ID = core.ID(id)
		row.Summary = oneLine(first, SummaryRunes)
		row.ReadingPos = core.ID(position)
		row.Updated = time.Unix(0, updated).UTC()
		row.Excerpt = excerpt == 1
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// noteRowColumns is a notebook row's projection, in scan order.
const noteRowColumns = `n.id, n.title,
	COALESCE((SELECT b.text FROM blocks b WHERE b.note_id = n.id ORDER BY b.position LIMIT 1), ''),
	(SELECT COUNT(*) FROM blocks b WHERE b.note_id = n.id),
	` + noteCardCount + `,
	COALESCE((SELECT r.position FROM reading r WHERE r.note = n.id), ''),
	COALESCE((SELECT b.position + 1 FROM blocks b WHERE b.note_id = n.id
		AND b.id = (SELECT r.position FROM reading r WHERE r.note = n.id) LIMIT 1), 0),
	n.updated,
	EXISTS (SELECT 1 FROM links l WHERE l.note_id = n.id AND l.anchor <> '')`

// noteCardCount is how many cards a note's blocks feed, which a row and the
// order by it both read.
const noteCardCount = `(SELECT COUNT(*) FROM cards c JOIN blocks b ON b.id = c.block_id WHERE b.note_id = n.id)`

// noteRowWhere is the notebook's filter, shared by the count and the page so the
// total always describes the page.
func noteRowWhere(q NoteRowQuery) (string, []any) {
	// A template is a note, but not a document the notebook lists.
	where, args := noteFilters(q.Text, q.Tag)
	where = append([]string{`NOT EXISTS (SELECT 1 FROM tags t WHERE t.note_id = n.id AND t.tag = ?)`}, where...)
	args = append([]any{core.TemplateTag}, args...)
	if q.Reading {
		where = append(where, `EXISTS (SELECT 1 FROM reading r WHERE r.note = n.id AND r.queued = 1)`)
	}
	if q.HasCards {
		where = append(where, `EXISTS (SELECT 1 FROM cards c JOIN blocks b ON b.id = c.block_id WHERE b.note_id = n.id)`)
	}
	if q.Excerpt {
		where = append(where, `EXISTS (SELECT 1 FROM links l WHERE l.note_id = n.id AND l.anchor <> '')`)
	}
	return ` WHERE ` + strings.Join(where, ` AND `), args
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
	// CardSortInterval lists the longest interval first, from the card's newest
	// review to its due moment. A card no review has touched has no interval and
	// comes last.
	CardSortInterval CardSort = "interval"
	// CardSortLapses lists the cards answered "again" most often first: the hard
	// cards a preset's threshold names.
	CardSortLapses CardSort = "lapses"
)

// CardQuery is what a card list asks for: a filter, an order and a page.
type CardQuery struct {
	// Text matches a card whose block, or whose edited field values, contains
	// it, ignoring case.
	Text string
	// Tag matches a card whose note carries that tag, by its bare name — the
	// same notes NotesWithTag answers for.
	Tag string
	// Note matches a card whose block lives in that note, named by its ID or by
	// its title.
	Note string
	// Type matches a card of that type, by its ID.
	Type core.ID
	// Ref matches a card whose source is live, frozen or lost; nil matches any.
	Ref *core.LinkState
	// Status matches a card in that lifecycle state; nil matches any.
	Status *core.Status
	// MinLapses keeps the cards answered "again" at least that many times, which
	// is what a preset's hard-card threshold compares (queue is:hard).
	MinLapses int
	Sort      CardSort
	// Limit is the page size. Zero or less means no limit.
	Limit int
	// Offset skips that many matches.
	Offset int
}

// CardSummary is a card as a list shows it: no field values, so a page costs no
// rendering.
type CardSummary struct {
	ID    core.ID
	Block core.ID
	// Note is the note the card's block lives in, empty for a block no note
	// holds any more.
	Note    core.ID
	Type    core.ID
	Ref     core.LinkState
	Status  core.Status
	Created time.Time
	Due     time.Time
	// Lapses is how many live reviews answered "again": the hard card a preset
	// compares against, and a column of the card table.
	Lapses int
	// IntervalDays is the interval the card is on, in whole days, from its
	// newest review to its due moment; zero for a card no review has touched.
	IntervalDays int
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
	// The count joins the schedule as the page does: a filter over lapses is a
	// column of it.
	total, err := l.index.count(`SELECT COUNT(*)`+fromCards+where, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	page, pageArgs := pageLimit(q.Limit, q.Offset)
	rows, err := l.index.db.Query(
		`SELECT c.id, c.block_id, c.type, c.status, c.created, `+cardDueNS+`, `+cardNoteID+`,
		        c.frozen, c.live, COALESCE(s.lapses, 0), `+cardIntervalDays+
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
			note                   string
			created, due           int64
			frozen, live           int
		)
		if err := rows.Scan(&id, &block, &typ, &status, &created, &due, &note,
			&frozen, &live, &summary.Lapses, &summary.IntervalDays); err != nil {
			return nil, 0, err
		}
		summary.ID, summary.Block, summary.Type = core.ID(id), core.ID(block), core.ID(typ)
		summary.Note = core.ID(note)
		summary.Status, _ = core.ParseStatus(status)
		summary.Created = time.Unix(0, created).UTC()
		summary.Due = moment(due)
		summary.Ref = core.LinkLive
		if frozen == 1 {
			summary.Ref = core.LinkFrozen
		} else if live != 1 {
			summary.Ref = core.LinkLost
		}
		out = append(out, summary)
	}
	return out, total, rows.Err()
}

// noteWhere is the filter QueryNotes and its count share, so the total always
// describes the page.
func noteWhere(q NoteQuery) (string, []any) {
	where, args := noteFilters(q.Text, q.Tag)
	if len(where) == 0 {
		return "", nil
	}
	return ` WHERE ` + strings.Join(where, ` AND `), args
}

// noteFilters is what a text and a tag ask of a note, as a clause per demand:
// the list's own filters are added to these.
func noteFilters(text, tag string) ([]string, []any) {
	var where []string
	var args []any
	if text = strings.TrimSpace(text); text != "" {
		like := "%" + escapeLike(text) + "%"
		where = append(where, `(n.title LIKE ? ESCAPE '\' OR EXISTS (
			SELECT 1 FROM blocks b WHERE b.note_id = n.id AND b.text LIKE ? ESCAPE '\'))`)
		args = append(args, like, like)
	}
	if tag = strings.TrimSpace(tag); tag != "" {
		where = append(where, `EXISTS (SELECT 1 FROM tags t WHERE t.note_id = n.id AND t.tag = ?)`)
		args = append(args, tag)
	}
	return where, args
}

// noteOrder is a note list's ORDER BY. Every order ends on the ID, so two notes
// that tie stay in one order across pages.
func noteOrder(sort NoteSort) string {
	switch sort {
	case NoteSortTitle:
		return ` ORDER BY lower(n.title), n.id`
	case NoteSortCreated:
		return ` ORDER BY n.updated, n.id`
	case NoteSortCards:
		return ` ORDER BY ` + noteCardCount + ` DESC, n.updated DESC, n.id`
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
		// A note is named by its ID or by its title: the search syntax's note:
		// takes what a person types, and an ID is what a caller already holds.
		where = append(where, `c.block_id IN (SELECT b.id FROM blocks b JOIN notes n ON n.id = b.note_id
			WHERE n.id = ? OR lower(n.title) LIKE ? ESCAPE '\')`)
		args = append(args, note, "%"+escapeLike(strings.ToLower(note))+"%")
	}
	if typ := strings.TrimSpace(string(q.Type)); typ != "" {
		where = append(where, `c.type = ?`)
		args = append(args, typ)
	}
	if q.Ref != nil {
		switch *q.Ref {
		case core.LinkLive:
			where = append(where, `c.frozen = 0 AND c.live = 1`)
		case core.LinkFrozen:
			where = append(where, `c.frozen = 1`)
		case core.LinkLost:
			where = append(where, `c.frozen = 0 AND c.live = 0`)
		}
	}
	if q.Status != nil {
		where = append(where, `c.status = ?`)
		args = append(args, q.Status.String())
	}
	if q.MinLapses > 0 {
		where = append(where, `COALESCE(s.lapses, 0) >= ?`)
		args = append(args, q.MinLapses)
	}
	if len(where) == 0 {
		return "", nil
	}
	return ` WHERE ` + strings.Join(where, ` AND `), args
}

// cardNoteID is the note a card's block lives in. A block ID can appear in two
// notes — a copied file — so the tie-break names one, as NoteOfBlock does.
const cardNoteID = `COALESCE((SELECT b.note_id FROM blocks b WHERE b.id = c.block_id
	ORDER BY b.note_id, b.position LIMIT 1), '')`

// cardNoteTitle is the title of the note a card's block lives in, which is the
// title a card list sorts by: a card has none of its own.
const cardNoteTitle = `COALESCE((SELECT n.title FROM notes n WHERE n.id = ` + cardNoteID + `), '')`

// cardOrder is a card list's ORDER BY. The due moment is the schedule the index
// cached, the date the user set by hand, or the card's creation for one no review
// has scheduled, which is the order the queue puts it in. Every order ends on the
// ID, so two cards that tie stay in one order across pages.
func cardOrder(sort CardSort) string {
	switch sort {
	case CardSortDue:
		return ` ORDER BY ` + cardDueNS + `, c.id`
	case CardSortTitle:
		return ` ORDER BY lower(` + cardNoteTitle + `), c.id`
	case CardSortInterval:
		return ` ORDER BY ` + cardIntervalDays + ` DESC, c.created, c.id`
	case CardSortLapses:
		return ` ORDER BY COALESCE(s.lapses, 0) DESC, c.created, c.id`
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
// carrying a block anchor, written in another note. A mention without an anchor
// names the note as a whole and is not an excerpt of one of its blocks.
func (x *index) excerptChildren() (map[string][]core.ID, error) {
	rows, err := x.db.Query(`SELECT note_id, title FROM links WHERE anchor <> ''`)
	if err != nil {
		return nil, fmt.Errorf("store: excerpts query: %w", err)
	}
	defer rows.Close()
	out := map[string][]core.ID{}
	for rows.Next() {
		var note, title string
		if err := rows.Scan(&note, &title); err != nil {
			return nil, err
		}
		out[title] = append(out[title], core.ID(note))
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

// AssetRow is one file in the library's assets directory: its name, its type and
// size, and the notes that carry it by relative path.
type AssetRow struct {
	// Name is the file's name inside assets/ — its SHA-256 and extension. A
	// note refers to it as "assets/<Name>", which is what Asset reads.
	Name string
	// Ext is the extension without the dot, lower-cased: the file's format.
	Ext string
	// Bytes is the file's size, which the listing shows and a copy checks.
	Bytes int64
	// Notes are the notes that reference the file, by ID in order. A file
	// nothing references is still a row: it is in the library, and the page
	// says so rather than hiding it.
	Notes []core.ID
}

// metaAssets is what the assets table was derived from, so an open reads the
// files' listing without scanning the notes again.
const metaAssets = "assets"

// AssetRows returns every file in the library's assets directory, by name, with
// the notes that reference it.
//
// The table behind it is derived from two sources and rebuilt when either moves:
// the files themselves, and the notes' text, where a reference is a relative
// path (assets/<sha>.<ext>, as a markdown image or link writes it). A note saved
// since the table was written moves the notes' stamp, and a file added or
// removed moves the directory's; either way the next read re-derives it, so a
// rebuilt index answers the same rows.
func (l *Library) AssetRows() ([]AssetRow, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	if err := l.index.syncAssets(l.dir); err != nil {
		return nil, err
	}
	rows, err := l.index.db.Query(`SELECT name, ext, bytes, notes FROM assets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: assets query: %w", err)
	}
	defer rows.Close()
	var out []AssetRow
	for rows.Next() {
		var (
			row      AssetRow
			notesRaw string
		)
		if err := rows.Scan(&row.Name, &row.Ext, &row.Bytes, &notesRaw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(notesRaw), &row.Notes); err != nil {
			return nil, fmt.Errorf("store: asset %q: notes: %w", row.Name, err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// assetRow is one asset as the table holds it.
type assetRow struct {
	name  string
	ext   string
	bytes int64
	notes []core.ID
}

// syncAssets derives the assets table when its sources have moved since it was
// written. The lock is held: it reads the files and the notes' text, then
// replaces the table in one transaction.
func (x *index) syncAssets(dir string) error {
	meta, err := x.readMeta()
	if err != nil {
		return err
	}
	stamp, err := x.assetStamp(dir)
	if err != nil {
		return err
	}
	if meta[metaAssets] == stamp {
		return nil
	}

	entries, err := os.ReadDir(filepath.Join(dir, assetsDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	rows := make(map[string]*assetRow, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		// Only own files count as assets, and only the ones Asset can read back
		// by their reference: a stray directory, a dotfile or a name with a
		// slash in it is not one.
		if _, ok := assetFileName(assetPrefix + name); !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			continue
		}
		rows[name] = &assetRow{
			name:  name,
			ext:   strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."),
			bytes: info.Size(),
		}
	}

	if len(rows) > 0 {
		// The references are one pass over the block text, not one query per
		// asset: the notes are the other side of the relation, and a library
		// with a thousand assets still reads its notes once.
		blocks, err := x.db.Query(`SELECT note_id, text FROM blocks ORDER BY note_id, position`)
		if err != nil {
			return fmt.Errorf("store: assets query: %w", err)
		}
		for blocks.Next() {
			var note, text string
			if err := blocks.Scan(&note, &text); err != nil {
				return err
			}
			for _, name := range assetRefs(text) {
				row, ok := rows[name]
				if !ok {
					continue
				}
				if n := len(row.notes); n > 0 && row.notes[n-1] == core.ID(note) {
					continue
				}
				row.notes = append(row.notes, core.ID(note))
			}
		}
		// The rows are read before the transaction: the index has one connection,
		// and a write inside the read would wait for itself.
		err = blocks.Err()
		blocks.Close()
		if err != nil {
			return err
		}
	}

	ordered := make([]*assetRow, 0, len(rows))
	for _, row := range rows {
		sort.SliceStable(row.notes, func(i, j int) bool { return row.notes[i] < row.notes[j] })
		ordered = append(ordered, row)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].name < ordered[j].name })

	return x.write(func(e execer) error {
		if _, err := e.Exec(`DELETE FROM assets`); err != nil {
			return err
		}
		for _, row := range ordered {
			notes := row.notes
			if notes == nil {
				notes = []core.ID{}
			}
			encoded, err := json.Marshal(notes)
			if err != nil {
				return fmt.Errorf("store: asset %q: notes: %w", row.name, err)
			}
			if _, err := e.Exec(`INSERT INTO assets (name, ext, bytes, notes) VALUES (?, ?, ?, ?)`,
				row.name, row.ext, row.bytes, string(encoded)); err != nil {
				return err
			}
		}
		return putMeta(e, metaAssets, stamp)
	})
}

// assetStamp is what the assets table was derived from: how many notes the
// library holds, when they were last saved (a save may add or drop a reference)
// and the mark of the assets directory (a file added or removed).
//
// The saves are summed modulo a prime: a stored moment is nanoseconds, and a
// plain sum of two of them overflows the integer SQLite adds in. The modulus
// keeps every term small, the newest moment guards the arithmetic against a
// wrap, and a save moves at least one of the three whatever the note.
func (x *index) assetStamp(dir string) (string, error) {
	var (
		count   int
		updated int64
		newest  int64
	)
	if err := x.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(updated % 1000000007), 0), COALESCE(MAX(updated), 0) FROM notes`).
		Scan(&count, &updated, &newest); err != nil {
		return "", fmt.Errorf("store: assets stamp: %w", err)
	}
	mark, err := dirMark(filepath.Join(dir, assetsDir))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d/%d/%d/%d", count, updated, newest, mark), nil
}
