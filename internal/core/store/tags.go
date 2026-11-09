package store

import (
	"fmt"
	"sort"
	"strings"

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

// TagRow is one tag as the tag page shows it: how many notes carry it, and a
// couple of titles so the row says what the tag is about.
type TagRow struct {
	Tag   string
	Count int
	// Sample holds up to TagSampleTitles of the notes carrying the tag, by
	// title: enough to recognise the tag without opening it.
	Sample []string
}

// TagSampleTitles is how many example titles a tag row carries.
const TagSampleTitles = 2

// QueryTags returns the library's tags with how many notes carry each, the ones
// carried by the most notes first and by name after them, at most limit of them
// — zero or less is every tag.
//
// Two statements answer the whole page: the counts, then the examples of the
// tags it selected. A tag's examples are the titles of the notes that carry it,
// alphabetically and at most TagSampleTitles of each — by title rather than by
// save time so a rebuilt index shows the same examples, which is what keeps the
// page from changing under the reader for no edit of theirs.
func (l *Library) QueryTags(limit int) []TagRow {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil
	}
	page, args := pageLimit(limit, 0)
	rows, err := l.index.db.Query(
		`SELECT t.tag, COUNT(DISTINCT t.note_id) FROM tags t
		  GROUP BY t.tag ORDER BY COUNT(DISTINCT t.note_id) DESC, t.tag`+page, args...)
	if err != nil {
		return nil
	}
	var out []TagRow
	for rows.Next() {
		var row TagRow
		if err := rows.Scan(&row.Tag, &row.Count); err != nil {
			rows.Close()
			return nil
		}
		out = append(out, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil
	}
	if err := l.index.tagSamples(out); err != nil {
		return nil
	}
	return out
}

// tagSamples fills in the example titles of the tags it is given, one statement
// per batch of them.
func (x *index) tagSamples(tags []TagRow) error {
	if len(tags) == 0 {
		return nil
	}
	samples := make(map[string][]string, len(tags))
	for start := 0; start < len(tags); start += 400 {
		end := min(start+400, len(tags))
		args := make([]any, 0, end-start+1)
		for _, tag := range tags[start:end] {
			args = append(args, tag.Tag)
		}
		args = append(args, TagSampleTitles)
		rows, err := x.db.Query(`
			SELECT tag, title FROM (
				SELECT t.tag AS tag, n.title AS title,
				       ROW_NUMBER() OVER (PARTITION BY t.tag ORDER BY lower(n.title), n.id) AS rn
				  FROM tags t JOIN notes n ON n.id = t.note_id
				 WHERE t.tag IN (?`+strings.Repeat(", ?", end-start-1)+`) AND n.title <> ''
			) WHERE rn <= ? ORDER BY tag, rn`, args...)
		if err != nil {
			return fmt.Errorf("store: tag samples query: %w", err)
		}
		for rows.Next() {
			var tag, title string
			if err := rows.Scan(&tag, &title); err != nil {
				rows.Close()
				return err
			}
			samples[tag] = append(samples[tag], title)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	for i := range tags {
		tags[i].Sample = samples[tags[i].Tag]
	}
	return nil
}
