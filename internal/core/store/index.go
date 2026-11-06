package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver: the shipped build stays cgo-free

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// schema is the index. It holds nothing the text sources do not: every table
// here can be dropped and rebuilt from notes, cards and reviews (ADR 0003).
const schema = `
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS card_state;
CREATE TABLE blocks (
	id      TEXT PRIMARY KEY,
	note_id TEXT NOT NULL,
	text    TEXT NOT NULL
);
CREATE TABLE cards (
	id      TEXT PRIMARY KEY,
	block_id TEXT NOT NULL,
	status  TEXT NOT NULL,
	created INTEGER NOT NULL,
	frozen  INTEGER NOT NULL
);
CREATE TABLE card_state (
	card_id     TEXT PRIMARY KEY,
	stability   REAL NOT NULL,
	difficulty  REAL NOT NULL,
	due         INTEGER NOT NULL,
	last_review INTEGER NOT NULL,
	reviews     INTEGER NOT NULL
);
CREATE INDEX cards_block ON cards (block_id);
CREATE INDEX cards_status ON cards (status);
CREATE INDEX cards_backlog ON cards (created);
CREATE INDEX card_state_due ON card_state (due);
`

// execer is what both a database handle and a transaction can do, so the index
// is written the same way one row at a time or in a rebuild.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// index is the derived index. It answers the questions a walk over the files
// cannot: what is due, how much is left, what lost its source.
type index struct {
	db *sql.DB
}

func openIndex(path string) (*index, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
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

func (x *index) close() error {
	if x == nil {
		return nil
	}
	return x.db.Close()
}

func putBlock(e execer, note core.ID, block core.Block) error {
	_, err := e.Exec(`INSERT INTO blocks (id, note_id, text) VALUES (?, ?, ?)`,
		string(block.ID), string(note), block.Text)
	return err
}

func deleteBlocksOfNote(e execer, note core.ID) error {
	_, err := e.Exec(`DELETE FROM blocks WHERE note_id = ?`, string(note))
	return err
}

func putCard(e execer, card core.Card, created time.Time) error {
	frozen := 0
	if card.Ref.Frozen != nil {
		frozen = 1
	}
	_, err := e.Exec(
		`INSERT OR REPLACE INTO cards (id, block_id, status, created, frozen) VALUES (?, ?, ?, ?, ?)`,
		string(card.ID), string(card.Ref.Block), card.Status.String(), created.UnixNano(), frozen)
	return err
}

func deleteCard(e execer, card core.ID) error {
	if _, err := e.Exec(`DELETE FROM cards WHERE id = ?`, string(card)); err != nil {
		return err
	}
	_, err := e.Exec(`DELETE FROM card_state WHERE card_id = ?`, string(card))
	return err
}

func putState(e execer, card core.ID, state schedule.State) error {
	_, err := e.Exec(
		`INSERT OR REPLACE INTO card_state (card_id, stability, difficulty, due, last_review, reviews)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		string(card), state.Memory.Stability, state.Memory.Difficulty,
		state.Due.UnixNano(), state.LastReview.UnixNano(), state.Reviews)
	return err
}

func deleteState(e execer, card core.ID) error {
	_, err := e.Exec(`DELETE FROM card_state WHERE card_id = ?`, string(card))
	return err
}

// dueClause is the review queue: a scheduled card whose source still resolves,
// due by its own schedule or, never reviewed, by the moment it was created.
const dueClause = `
	FROM cards c
	LEFT JOIN card_state s ON s.card_id = c.id
	WHERE c.status IN ('new', 'learning', 'review')
	  AND (c.frozen = 1 OR c.block_id IN (SELECT id FROM blocks))
	  AND COALESCE(s.due, c.created) <= ?
`

func (x *index) due(now time.Time, limit int) ([]core.ID, error) {
	query := `SELECT c.id ` + dueClause + ` ORDER BY COALESCE(s.due, c.created), c.id`
	args := []any{now.UnixNano()}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := x.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: due query: %w", err)
	}
	defer rows.Close()
	return scanIDs(rows)
}

func (x *index) remaining(now time.Time) (int, error) {
	var count int
	err := x.db.QueryRow(`SELECT COUNT(*) `+dueClause, now.UnixNano()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: remaining query: %w", err)
	}
	return count, nil
}

// lost is a card whose block is gone and which kept no snapshot: it has text to
// show to nobody, but it keeps its history (ADR 0002).
func (x *index) lost() ([]core.ID, error) {
	rows, err := x.db.Query(`
		SELECT id FROM cards
		WHERE frozen = 0 AND block_id NOT IN (SELECT id FROM blocks)
		ORDER BY created, id`)
	if err != nil {
		return nil, fmt.Errorf("store: lost query: %w", err)
	}
	defer rows.Close()
	return scanIDs(rows)
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
