package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver: the shipped build stays cgo-free

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// schemaVersion is the shape of the index. The index is derived, so a version
// this build does not know is a rebuild, never a reason to refuse a library.
// Bump it whenever a table or a column changes.
const schemaVersion = "5"

// schema is the index. It holds nothing the text sources do not: every table
// here can be dropped and rebuilt from notes, cards and reviews (ADR 0003). It
// answers what a walk over the files cannot — what is due, how much is left,
// what lost its source, which text a search matches — and it is what an open
// reads when the sources have not changed.
const schema = `
DROP TABLE IF EXISTS meta;
DROP TABLE IF EXISTS notes;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS card_state;
CREATE TABLE meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE notes (
	id      TEXT PRIMARY KEY,
	slug    TEXT NOT NULL,
	title   TEXT NOT NULL,
	updated INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE blocks (
	note_id  TEXT NOT NULL,
	position INTEGER NOT NULL,
	id       TEXT NOT NULL,
	text     TEXT NOT NULL,
	PRIMARY KEY (note_id, position)
);
CREATE INDEX blocks_id ON blocks (id);
CREATE TABLE cards (
	id          TEXT PRIMARY KEY,
	block_id    TEXT NOT NULL,
	status      TEXT NOT NULL,
	stored      TEXT NOT NULL,
	created     INTEGER NOT NULL,
	prompt      TEXT NOT NULL,
	answer      TEXT NOT NULL,
	frozen      INTEGER NOT NULL,
	frozen_text TEXT NOT NULL,
	frozen_at   INTEGER NOT NULL,
	live        INTEGER NOT NULL,
	kind        TEXT NOT NULL DEFAULT '',
	ordinal     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX cards_block ON cards (block_id);
CREATE TABLE links (
	note_id   TEXT NOT NULL,
	block_id  TEXT NOT NULL,
	title     TEXT NOT NULL,
	paragraph INTEGER NOT NULL
);
CREATE INDEX links_title ON links (title);
CREATE TABLE card_state (
	card_id     TEXT PRIMARY KEY,
	stability   REAL NOT NULL,
	difficulty  REAL NOT NULL,
	due         INTEGER NOT NULL,
	last_review INTEGER NOT NULL,
	reviews     INTEGER NOT NULL,
	last_rating TEXT NOT NULL,
	last_seq    INTEGER NOT NULL
);
`

// execer is what both a database handle and a transaction can do, so the index
// is written the same way one row at a time or in a rebuild.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// index is the derived index.
type index struct {
	db *sql.DB
}

func openIndex(path string) (*index, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// The index is derived, so a lost commit only costs a replay: WAL with a
	// normal sync keeps a review write a WAL append rather than a file sync,
	// and survives a process crash whole. Power loss can lose the last
	// transactions, which leaves the watermark behind the log — exactly the
	// case the log tail replay at open is for.
	db, err := sql.Open("sqlite", "file:"+path+
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("store: open index: %w", err)
	}
	// One connection keeps the pragma and the writes in a single order.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: open index: %w", err)
	}
	return &index{db: db}, nil
}

// reset drops every table and creates the schema again.
func (x *index) reset() error {
	if _, err := x.db.Exec(schema); err != nil {
		return fmt.Errorf("store: rebuild index: %w", err)
	}
	return nil
}

// write runs fn in one transaction.
func (x *index) write(fn func(execer) error) error {
	tx, err := x.db.Begin()
	if err != nil {
		return fmt.Errorf("store: index: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: index: %w", err)
	}
	return nil
}

// checkpoint folds the write-ahead log back into the index file. It is done
// after a rebuild, where the cost belongs to the open, so the first review
// afterwards does not pay for the thousands of rows the rebuild wrote.
func (x *index) checkpoint() error {
	if _, err := x.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("store: index checkpoint: %w", err)
	}
	return nil
}

func (x *index) close() error {
	if x == nil {
		return nil
	}
	return x.db.Close()
}

// ---------------------------------------------------------------------------
// meta: what the index was built from, so an open knows whether it still holds.

const (
	metaSchema = "schema" // schemaVersion the tables were written with
	metaParams = "params" // digest of the scheduling parameters
	metaNotes  = "notes"  // notes/ directory fingerprint
	metaCards  = "cards"  // cards/ directory fingerprint
	metaLog    = "log"    // reviews.jsonl: bytes applied, its mtime, last sequence
)

func putMeta(e execer, key, value string) error {
	_, err := e.Exec(`INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`, key, value)
	return err
}

func deleteMeta(e execer, key string) error {
	_, err := e.Exec(`DELETE FROM meta WHERE key = ?`, key)
	return err
}

// readMeta reads the whole watermark in one query. A table that does not exist
// yet (a library opened by an older build) reads as no watermark at all.
func (x *index) readMeta() (map[string]string, error) {
	rows, err := x.db.Query(`SELECT key, value FROM meta`)
	if err != nil {
		return nil, nil // no meta table: the index is stale by definition
	}
	defer rows.Close()
	meta := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		meta[key] = value
	}
	return meta, rows.Err()
}

// ---------------------------------------------------------------------------
// notes and blocks: what a note says, so a caller can read one without the
// note file, and so a card can find the note its block lives in.

// noteRow is a note as the index holds it.
type noteRow struct {
	id      core.ID
	slug    string
	title   string
	updated time.Time
}

func putNote(e execer, row noteRow) error {
	_, err := e.Exec(`INSERT OR REPLACE INTO notes (id, slug, title, updated) VALUES (?, ?, ?, ?)`,
		string(row.id), row.slug, row.title, row.updated.UnixNano())
	return err
}

func (x *index) note(id core.ID) (noteRow, bool) {
	var row noteRow
	var idText string
	var updated int64
	err := x.db.QueryRow(`SELECT id, slug, title, updated FROM notes WHERE id = ?`, string(id)).
		Scan(&idText, &row.slug, &row.title, &updated)
	if err != nil {
		return noteRow{}, false
	}
	row.id = core.ID(idText)
	return row, true
}

func (x *index) notes() ([]noteRow, error) {
	rows, err := x.db.Query(`SELECT id, slug, title, updated FROM notes`)
	if err != nil {
		return nil, fmt.Errorf("store: notes query: %w", err)
	}
	defer rows.Close()
	var out []noteRow
	for rows.Next() {
		var row noteRow
		var idText string
		var updated int64
		if err := rows.Scan(&idText, &row.slug, &row.title, &updated); err != nil {
			return nil, err
		}
		row.id = core.ID(idText)
		row.updated = time.Unix(0, updated).UTC()
		out = append(out, row)
	}
	return out, rows.Err()
}

func deleteBlocksOfNote(e execer, note core.ID) error {
	_, err := e.Exec(`DELETE FROM blocks WHERE note_id = ?`, string(note))
	return err
}

func putBlock(e execer, note core.ID, position int, block core.Block) error {
	_, err := e.Exec(`INSERT INTO blocks (note_id, position, id, text) VALUES (?, ?, ?, ?)`,
		string(note), position, string(block.ID), block.Text)
	return err
}

// blockRef is where a referencable block was found: which note, the block
// itself, and its position in the note, which is where its marker would go.
type blockRef struct {
	note  core.ID
	block core.Block
	at    int
}

func (x *index) block(id core.ID) (blockRef, bool) {
	var ref blockRef
	var note, blockID, text string
	err := x.db.QueryRow(
		`SELECT note_id, position, id, text FROM blocks WHERE id = ? ORDER BY note_id, position LIMIT 1`,
		string(id)).Scan(&note, &ref.at, &blockID, &text)
	if err != nil {
		return blockRef{}, false
	}
	ref.note = core.ID(note)
	ref.block = core.Block{ID: core.ID(blockID), Text: text}
	return ref, true
}

// blocksOfNote lists a note's blocks in the order the file holds them.
func (x *index) blocksOfNote(note core.ID) ([]core.Block, error) {
	rows, err := x.db.Query(`SELECT id, text FROM blocks WHERE note_id = ? ORDER BY position`, string(note))
	if err != nil {
		return nil, fmt.Errorf("store: blocks query: %w", err)
	}
	defer rows.Close()
	return scanBlocks(rows)
}

// allBlocks lists every note's blocks, grouped by note, in file order.
func (x *index) allBlocks() (map[core.ID][]core.Block, error) {
	rows, err := x.db.Query(`SELECT note_id, id, text FROM blocks ORDER BY note_id, position`)
	if err != nil {
		return nil, fmt.Errorf("store: blocks query: %w", err)
	}
	defer rows.Close()
	out := map[core.ID][]core.Block{}
	for rows.Next() {
		var note, id, text string
		if err := rows.Scan(&note, &id, &text); err != nil {
			return nil, err
		}
		out[core.ID(note)] = append(out[core.ID(note)], core.Block{ID: core.ID(id), Text: text})
	}
	return out, rows.Err()
}

func scanBlocks(rows *sql.Rows) ([]core.Block, error) {
	var out []core.Block
	for rows.Next() {
		var id, text string
		if err := rows.Scan(&id, &text); err != nil {
			return nil, err
		}
		out = append(out, core.Block{ID: core.ID(id), Text: text})
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// cards: the file's definition plus what replay and the notes say about it, so
// a listing, a search or the queue needs no file read.

// cardRow is a card as the index holds it.
type cardRow struct {
	card    core.Card
	stored  core.Status // the status the card file carries
	created time.Time
	live    bool
	state   schedule.State
	lastSeq int64
}

// cardColumns is the projection a card row is read through, in scan order.
const cardColumns = `
	c.id, c.block_id, c.status, c.stored, c.created, c.prompt, c.answer,
	c.frozen, c.frozen_text, c.frozen_at, c.live, c.kind, c.ordinal,
	COALESCE(s.stability, 0), COALESCE(s.difficulty, 0), COALESCE(s.due, 0),
	COALESCE(s.last_review, 0), COALESCE(s.reviews, 0), COALESCE(s.last_rating, ''),
	COALESCE(s.last_seq, 0)`

// fromCards joins the schedule a card's log gives it, if it has one.
const fromCards = ` FROM cards c LEFT JOIN card_state s ON s.card_id = c.id`

func putCard(e execer, row cardRow) error {
	frozen, frozenText, frozenAt := 0, "", int64(0)
	if row.card.Ref.Frozen != nil {
		frozen = 1
		frozenText = row.card.Ref.Frozen.Text
		frozenAt = row.card.Ref.Frozen.At.UnixNano()
	}
	live := 0
	if row.live {
		live = 1
	}
	_, err := e.Exec(
		`INSERT OR REPLACE INTO cards
		 (id, block_id, status, stored, created, prompt, answer, frozen, frozen_text, frozen_at, live, kind, ordinal)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(row.card.ID), string(row.card.Ref.Block), row.card.Status.String(), row.stored.String(),
		row.created.UnixNano(), row.card.Prompt, row.card.Answer, frozen, frozenText, frozenAt, live,
		row.card.Kind.String(), row.card.Ordinal)
	return err
}

func deleteCard(e execer, card core.ID) error {
	if _, err := e.Exec(`DELETE FROM cards WHERE id = ?`, string(card)); err != nil {
		return err
	}
	_, err := e.Exec(`DELETE FROM card_state WHERE card_id = ?`, string(card))
	return err
}

// resolveBlocks re-derives whether the cards rendering from the given blocks
// still have a source: a note save can add, move or delete them all at once.
func resolveBlocks(e execer, blocks []core.ID) error {
	if len(blocks) == 0 {
		return nil
	}
	for _, block := range blocks {
		_, err := e.Exec(`
			UPDATE cards
			SET live = CASE
				WHEN frozen = 1 THEN 1
				WHEN EXISTS (SELECT 1 FROM blocks b WHERE b.id = cards.block_id) THEN 1
				ELSE 0 END
			WHERE block_id = ?`, string(block))
		if err != nil {
			return err
		}
	}
	return nil
}

func setCardStatus(e execer, card core.ID, status core.Status) error {
	_, err := e.Exec(`UPDATE cards SET status = ? WHERE id = ?`, status.String(), string(card))
	return err
}

// cardsOfBlocks lists the cards that render from any of the given blocks.
func (x *index) cardsOfBlocks(blocks []core.ID) ([]core.ID, error) {
	var out []core.ID
	for start := 0; start < len(blocks); start += 500 {
		end := min(start+500, len(blocks))
		args := make([]any, 0, end-start)
		for _, id := range blocks[start:end] {
			args = append(args, string(id))
		}
		rows, err := x.db.Query(
			`SELECT id FROM cards WHERE block_id IN (?`+strings.Repeat(", ?", len(args)-1)+`)`,
			args...)
		if err != nil {
			return nil, fmt.Errorf("store: note cards query: %w", err)
		}
		page, err := scanIDs(rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
	}
	return out, nil
}

// cardRows reads every card, oldest first: the whole projection, in the order
// the listing wants it.
func (x *index) cardRows() ([]cardRow, error) {
	rows, err := x.db.Query(`SELECT` + cardColumns + fromCards + ` ORDER BY c.created, c.id`)
	if err != nil {
		return nil, fmt.Errorf("store: cards query: %w", err)
	}
	defer rows.Close()
	return scanCardRows(rows)
}

// cardRowsByID reads the given cards, skipping the ones the index does not have.
func (x *index) cardRowsByID(ids []core.ID) ([]cardRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []cardRow
	for start := 0; start < len(ids); start += 500 {
		end := min(start+500, len(ids))
		args := make([]any, 0, end-start)
		for _, id := range ids[start:end] {
			args = append(args, string(id))
		}
		rows, err := x.db.Query(
			`SELECT`+cardColumns+fromCards+` WHERE c.id IN (?`+strings.Repeat(", ?", len(args)-1)+`)`,
			args...)
		if err != nil {
			return nil, fmt.Errorf("store: cards query: %w", err)
		}
		page, err := scanCardRows(rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
	}
	return out, nil
}

func (x *index) cardRow(id core.ID) (cardRow, bool) {
	rows, err := x.db.Query(`SELECT`+cardColumns+fromCards+` WHERE c.id = ?`, string(id))
	if err != nil {
		return cardRow{}, false
	}
	defer rows.Close()
	found, err := scanCardRows(rows)
	if err != nil || len(found) == 0 {
		return cardRow{}, false
	}
	return found[0], true
}

func scanCardRows(rows *sql.Rows) ([]cardRow, error) {
	var out []cardRow
	for rows.Next() {
		var (
			row                                       cardRow
			id, block, status, stored, prompt, answer string
			frozenText, lastRating, kind              string
			created, frozenAt, due, lastReview        int64
			frozen, live, ordinal                     int
			stability, difficulty                     float64
			reviews                                   int
		)
		err := rows.Scan(&id, &block, &status, &stored, &created, &prompt, &answer,
			&frozen, &frozenText, &frozenAt, &live, &kind, &ordinal,
			&stability, &difficulty, &due, &lastReview, &reviews, &lastRating, &row.lastSeq)
		if err != nil {
			return nil, err
		}
		row.card.ID = core.ID(id)
		row.card.Ref.Block = core.ID(block)
		row.card.Prompt = prompt
		row.card.Answer = answer
		row.card.Status, _ = core.ParseStatus(status)
		if kind != "" {
			row.card.Kind, _ = core.ParseKind(kind)
		}
		row.card.Ordinal = ordinal
		row.stored, _ = core.ParseStatus(stored)
		if frozen == 1 {
			row.card.Ref.Frozen = &core.Snapshot{Text: frozenText, At: time.Unix(0, frozenAt).UTC()}
		}
		row.created = time.Unix(0, created).UTC()
		row.live = live == 1 || frozen == 1
		if reviews > 0 {
			row.state = schedule.State{
				Memory:     schedule.Memory{Stability: stability, Difficulty: difficulty},
				Due:        time.Unix(0, due).UTC(),
				LastReview: time.Unix(0, lastReview).UTC(),
				Reviews:    reviews,
			}
			row.state.LastRating, _ = schedule.ParseRating(lastRating)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// card_state: what replaying the log derived for one card.

func putState(e execer, card core.ID, state schedule.State, lastSeq int64) error {
	_, err := e.Exec(
		`INSERT OR REPLACE INTO card_state
		 (card_id, stability, difficulty, due, last_review, reviews, last_rating, last_seq)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		string(card), state.Memory.Stability, state.Memory.Difficulty,
		state.Due.UnixNano(), state.LastReview.UnixNano(), state.Reviews,
		state.LastRating.String(), lastSeq)
	return err
}

func deleteState(e execer, card core.ID) error {
	_, err := e.Exec(`DELETE FROM card_state WHERE card_id = ?`, string(card))
	return err
}

func scanIDs(rows *sql.Rows) ([]core.ID, error) {
	var ids []core.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, core.ID(id))
	}
	return ids, rows.Err()
}
