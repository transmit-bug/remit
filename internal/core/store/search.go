package store

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/transmit-bug/remit/internal/core"
)

// Search is the ranked full-text path over the derived index. Its corpus is the
// FTS5 table `search`, which holds one row per note — the note's title and the
// text of its blocks — and one per card: the block it renders and the field
// values the user edited over it, which is the text SearchCards has always
// matched.
//
// The table is tokenized by trigrams, so a query is a substring match and needs
// no word boundary: "cramm" finds "cramming", and Chinese is answered without a
// segmenter. A trigram is three characters by construction, so a shorter query
// has nothing to look up: it falls back to the LIKE scan below, which reads the
// same rows and matches the same columns.

// SearchHit is one search result: what it is, where it is, and the text around
// the match.
type SearchHit struct {
	// Kind is "note" or "card".
	Kind string
	// ID is the note or the card that matched.
	ID core.ID
	// Note is the note the result belongs to; for a note hit, itself.
	Note core.ID
	// Block is the block a card renders; empty for a note.
	Block core.ID
	// Title is the note's title: the note's own, or the title of a card's note.
	Title string
	// Snippet is the text around the match, the match bracketed with "[" and
	// "]". Line breaks are FTS5's own escaped \n, so a snippet stays on one
	// line however the note was wrapped.
	Snippet string
	// Rank is how well the row matched; more is better. The LIKE path has no
	// score, so its hits all carry 0.
	Rank float64
}

const (
	// minTrigram is the shortest query the trigram tokenizer can look up.
	minTrigram = 3
	// snippetTokens is how many tokens of context a snippet keeps on each side.
	snippetTokens = 12
	// snippetRunes is how much of the line the LIKE path shows around a match.
	snippetRunes = 24
	// searchBatch is how many IDs one statement names, kept well inside
	// SQLite's limit on bound parameters.
	searchBatch = 400
	// searchPageRows is the biggest page read by its own rowids. Ranking scores
	// every matching row, and reading a page's text by rowid costs the match set
	// again, so a page much past this size is cheaper read in the scan that
	// ranks it: measured at a hundred thousand matching rows, a thousand-row
	// page by rowid takes 0.6 s and a four-thousand-row one 1.3 s, while the
	// scan reads the whole set in 1.05 s.
	searchPageRows = 1000
)

// searchRank scores a row with FTS5's bm25. Its weights are one per column, in
// the order the search table declares them: the six unindexed columns score
// nothing, and the last two carry the weight — a title match counts ten times a
// body match, so a note called "Entropy" is about entropy before one that
// merely mentions it. A column added to the table shifts them; the ranking test
// is what catches that.
const searchRank = `bm25(search, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 10.0, 1.0)`

// Search returns the notes and cards whose text contains query, best match
// first. kind narrows it to "note" or "card"; "" searches both. The count is how
// many results there are in all, so a caller can page.
func (l *Library) Search(query, kind string, limit, offset int) ([]SearchHit, int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, 0, ErrClosed
	}
	switch kind {
	case "", noteKind, cardKind:
	default:
		return nil, 0, fmt.Errorf("store: search kind %q is neither %q nor %q", kind, noteKind, cardKind)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, 0, nil
	}
	if utf8.RuneCountInString(query) < minTrigram {
		return l.index.searchLike(query, kind, limit, offset)
	}
	return l.index.searchMatch(query, kind, limit, offset)
}

// searchKindWhere narrows a search to one kind of result.
func searchKindWhere(kind string) (string, []any) {
	if kind == "" {
		return "", nil
	}
	return ` AND kind = ?`, []any{kind}
}

// searchMatch answers a query the trigram index can look up. A page is read in
// two statements: the ranking, then the text of the page's rows, which is where a
// snippet comes from. Asking for a snippet beside the ranking builds one for
// every matching row, so a wide read is left to searchMatchAll, which does that
// and is what a caller asking for thousands of rows is really asking for.
func (x *index) searchMatch(query, kind string, limit, offset int) ([]SearchHit, int, error) {
	countSQL, countArgs := searchMatchCountSQL(query, kind)
	total, err := x.count(countSQL, countArgs...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	if limit <= 0 || limit > searchPageRows {
		return x.searchMatchAll(query, kind, limit, offset, total)
	}
	rankSQL, rankArgs := searchMatchRankSQL(query, kind, limit, offset)
	page, err := x.rankedRows(rankSQL, rankArgs...)
	if err != nil {
		return nil, 0, err
	}
	hits, err := x.readHits(query, kind, page)
	if err != nil {
		return nil, 0, err
	}
	return hits, total, nil
}

// searchMatchAll is a ranked search with no page size: one statement that ranks
// and reads every row as it goes.
func (x *index) searchMatchAll(query, kind string, limit, offset, total int) ([]SearchHit, int, error) {
	kindWhere, kindArgs := searchKindWhere(kind)
	page, pageArgs := pageLimit(limit, offset)
	args := append(append([]any{matchQuery(query)}, kindArgs...), pageArgs...)
	rows, err := x.db.Query(`
		SELECT search.kind, search.doc, search.note, search.block,
		       CASE search.kind WHEN 'note' THEN search.title ELSE COALESCE(n.title, '') END,
		       snippet(search, -1, '[', ']', '…', `+formatInt(snippetTokens)+`),
		       -`+searchRank+`
		  FROM search LEFT JOIN notes n ON n.id = search.note
		 WHERE search MATCH ?`+kindWhere+`
		 ORDER BY `+searchRank+`, search.kind, search.doc`+page, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: search: %w", err)
	}
	defer rows.Close()
	return scanHits(rows, total)
}

// searchMatchCountSQL is how many rows a ranked search pages over. It sits
// beside the ranking's statement, as a function, so the cost check can ask
// SQLite which plan it picks for each.
func searchMatchCountSQL(query, kind string) (string, []any) {
	kindWhere, kindArgs := searchKindWhere(kind)
	return `SELECT COUNT(*) FROM search WHERE search MATCH ?` + kindWhere,
		append([]any{matchQuery(query)}, kindArgs...)
}

// searchMatchRankSQL is the page's rows in rank order: best match first, ties in
// a fixed order so paging is stable.
func searchMatchRankSQL(query, kind string, limit, offset int) (string, []any) {
	kindWhere, kindArgs := searchKindWhere(kind)
	page, pageArgs := pageLimit(limit, offset)
	args := append(append([]any{matchQuery(query)}, kindArgs...), pageArgs...)
	return `SELECT rowid, -` + searchRank + ` FROM search WHERE search MATCH ?` + kindWhere +
		` ORDER BY ` + searchRank + `, kind, doc` + page, args
}

// rankedRow is one row of a ranked page: which row it is, and how well it
// matched, so the text is read for exactly these and in this order.
type rankedRow struct {
	rowid int64
	rank  float64
}

// readHits reads the text of the given rows, and answers them in the order the
// ranking put them. Bounded pages are small enough that the rowids drive the
// statement: a page costs a page, not the whole match set.
func (x *index) readHits(query, kind string, page []rankedRow) ([]SearchHit, error) {
	kindWhere, kindArgs := searchKindWhere(kind)
	found := make(map[int64]SearchHit, len(page))
	for start := 0; start < len(page); start += searchBatch {
		chunk := page[start:min(start+searchBatch, len(page))]
		args := append([]any{matchQuery(query)}, kindArgs...)
		for _, row := range chunk {
			args = append(args, row.rowid)
		}
		if err := x.readHitText(kindWhere, args, len(chunk), found); err != nil {
			return nil, err
		}
	}

	hits := make([]SearchHit, 0, len(page))
	for _, row := range page {
		hit, ok := found[row.rowid]
		if !ok {
			continue // the row went away between the two statements
		}
		hit.Rank = row.rank
		hits = append(hits, hit)
	}
	return hits, nil
}

// readHitText reads the text of the rows one statement names by rowid, into
// found.
func (x *index) readHitText(kindWhere string, args []any, want int, found map[int64]SearchHit) error {
	rows, err := x.db.Query(hydrateSQL(kindWhere, want), args...)
	if err != nil {
		return fmt.Errorf("store: search: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			rowid                  int64
			hit                    SearchHit
			kind, doc, note, block string
		)
		if err := rows.Scan(&rowid, &kind, &doc, &note, &block, &hit.Title, &hit.Snippet); err != nil {
			return err
		}
		hit.Kind, hit.ID, hit.Note, hit.Block = kind, core.ID(doc), core.ID(note), core.ID(block)
		found[rowid] = hit
	}
	return rows.Err()
}

// scanHits reads the ranked rows one statement returned.
func scanHits(rows *sql.Rows, total int) ([]SearchHit, int, error) {
	var hits []SearchHit
	for rows.Next() {
		var hit SearchHit
		var kind, doc, note, block string
		if err := rows.Scan(&kind, &doc, &note, &block, &hit.Title, &hit.Snippet, &hit.Rank); err != nil {
			return nil, 0, err
		}
		hit.Kind, hit.ID, hit.Note, hit.Block = kind, core.ID(doc), core.ID(note), core.ID(block)
		hits = append(hits, hit)
	}
	return hits, total, rows.Err()
}

// hydrateSQL reads a page's text: the rowids chosen by the ranking, matched
// again so that snippet() has the query it marks. It is a function of its own so
// the cost check can ask SQLite which plan it picks.
func hydrateSQL(kindWhere string, want int) string {
	return `
		SELECT search.rowid, search.kind, search.doc, search.note, search.block,
		       CASE search.kind WHEN 'note' THEN search.title ELSE COALESCE(n.title, '') END,
		       snippet(search, -1, '[', ']', '…', ` + formatInt(snippetTokens) + `)
		  FROM search LEFT JOIN notes n ON n.id = search.note
		 WHERE search MATCH ?` + kindWhere + `
		   AND search.rowid IN (` + placeholders(want) + `)`
}

// placeholders is n bound parameters, comma separated, for an IN list.
func placeholders(n int) string {
	return strings.TrimPrefix(strings.Repeat(", ?", n), ", ")
}

// rankedRows reads a ranked page: each row's id and its score.
func (x *index) rankedRows(query string, args ...any) ([]rankedRow, error) {
	rows, err := x.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: search: %w", err)
	}
	defer rows.Close()
	var out []rankedRow
	for rows.Next() {
		var row rankedRow
		if err := rows.Scan(&row.rowid, &row.rank); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// matchQuery turns a user's text into one quoted FTS5 phrase: the text is
// searched for literally, the way the LIKE path takes it, so "50%" is a question
// about a percentage and never a syntax error.
func matchQuery(query string) string {
	return `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
}

// searchLike answers a query too short for the trigram index. It reads the same
// rows and matches the same columns with a substring LIKE; what it cannot give
// is a score, so its hits are ordered the way the unranked listings are — notes
// newest save first, then cards oldest first — and all rank 0.
func (x *index) searchLike(query, kind string, limit, offset int) ([]SearchHit, int, error) {
	like := "%" + escapeLike(query) + "%"
	kindWhere, kindArgs := searchKindWhere(kind)
	where := ` WHERE (title LIKE ? ESCAPE '\' OR body LIKE ? ESCAPE '\')` + kindWhere
	args := append([]any{like, like}, kindArgs...)

	total, err := x.count(`SELECT COUNT(*) FROM search`+where, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	page, pageArgs := pageLimit(limit, offset)
	rows, err := x.db.Query(`
		SELECT kind, doc, note, block, title, body FROM search`+where+`
		 ORDER BY CASE kind WHEN 'note' THEN 0 ELSE 1 END,
		          CASE kind WHEN 'note' THEN -updated ELSE created END,
		          doc`+page, append(args, pageArgs...)...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: search: %w", err)
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var hit SearchHit
		var kind, doc, note, block, body string
		if err := rows.Scan(&kind, &doc, &note, &block, &hit.Title, &body); err != nil {
			return nil, 0, err
		}
		hit.Kind, hit.ID, hit.Note, hit.Block = kind, core.ID(doc), core.ID(note), core.ID(block)
		hit.Snippet = likeSnippet(hit.Title, body, query)
		hits = append(hits, hit)
	}
	return hits, total, rows.Err()
}

// ---------------------------------------------------------------------------
// the search table: written in the same transaction as the rows it mirrors.

// searchCardProjection derives a card's search row from what the cards and
// blocks tables hold, so the row is never a copy a writer could forget: the
// text of the block the card renders, then the field values it was given. A
// card whose block is gone keeps its own text, as SearchCards does.
const searchCardProjection = `
	SELECT '` + cardKind + `', c.id,
	       COALESCE((SELECT b.note_id FROM blocks b WHERE b.id = c.block_id
	                  ORDER BY b.note_id, b.position LIMIT 1), ''),
	       c.block_id, 0, c.created, '',
	       trim(COALESCE((SELECT b.text FROM blocks b WHERE b.id = c.block_id
	                       ORDER BY b.note_id, b.position LIMIT 1), '')
	            || char(10) || c.fields_text, char(10))
	  FROM cards c`

const searchCardInsert = `INSERT INTO search (kind, doc, note, block, updated, created, title, body)` + searchCardProjection

// putSearchNote writes a note's search row: its title, and its block text in
// file order, so the snippet reads the way the note does.
func putSearchNote(e execer, row noteRow, blocks []core.Block) error {
	if err := deleteSearchNote(e, row.id); err != nil {
		return err
	}
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			texts = append(texts, block.Text)
		}
	}
	_, err := e.Exec(
		`INSERT INTO search (kind, doc, note, block, updated, created, title, body)
		 VALUES (?, ?, ?, '', ?, 0, ?, ?)`,
		noteKind, string(row.id), string(row.id), row.updated.UnixNano(), row.title, strings.Join(texts, "\n"))
	return err
}

// putSearchCards writes the search rows of every card in the index, which is
// what a rebuild needs: the same rows, derived in one statement.
func putSearchCards(e execer) error {
	_, err := e.Exec(searchCardInsert)
	return err
}

// refreshCardSearch re-derives the search rows of the named cards, for a card
// that was just written.
func refreshCardSearch(e execer, cards []core.ID) error {
	return refreshCardSearchWhere(e, cards, `c.id`, `doc`)
}

// refreshCardSearchOfBlocks re-derives the search rows of the cards rendering
// any of the given blocks, which is what a note save changes.
func refreshCardSearchOfBlocks(e execer, blocks []core.ID) error {
	return refreshCardSearchWhere(e, blocks, `c.block_id`, `block`)
}

// refreshCardSearchWhere deletes and re-inserts the search rows of the cards one
// of whose columns is in ids. The two column names are the literals the callers
// above pass; no caller text reaches the statement.
func refreshCardSearchWhere(e execer, ids []core.ID, cardColumn, searchColumn string) error {
	for start := 0; start < len(ids); start += searchBatch {
		end := min(start+searchBatch, len(ids))
		marks := placeholders(end - start)
		args := make([]any, 0, end-start)
		for _, id := range ids[start:end] {
			args = append(args, string(id))
		}
		if _, err := e.Exec(`DELETE FROM search WHERE kind = ? AND `+searchColumn+` IN (`+marks+`)`,
			append([]any{cardKind}, args...)...); err != nil {
			return err
		}
		if _, err := e.Exec(searchCardInsert+` WHERE `+cardColumn+` IN (`+marks+`)`, args...); err != nil {
			return err
		}
	}
	return nil
}

// deleteSearchCard drops a card's search row with the card.
func deleteSearchCard(e execer, card core.ID) error {
	_, err := e.Exec(`DELETE FROM search WHERE kind = ? AND doc = ?`, cardKind, string(card))
	return err
}

// deleteSearchNote drops a note's search row with the note.
func deleteSearchNote(e execer, note core.ID) error {
	_, err := e.Exec(`DELETE FROM search WHERE kind = ? AND doc = ?`, noteKind, string(note))
	return err
}

// ---------------------------------------------------------------------------
// snippets for the path that has no FTS5 match to quote.

// likeSnippet brackets the query where the row matched it, with the words around
// it: the shape snippet() gives the ranked path, for a query too short for the
// trigram index.
func likeSnippet(title, body, query string) string {
	if out, ok := bracket(body, query); ok {
		return out
	}
	if out, ok := bracket(title, query); ok {
		return out
	}
	return clip(body)
}

// bracket wraps the first case-insensitive occurrence of query in text with the
// markers, keeping the words on either side and saying where it cut them off.
func bracket(text, query string) (string, bool) {
	runes, want := []rune(text), []rune(query)
	at := foldIndex(runes, want)
	if at < 0 {
		return "", false
	}
	from := max(at-snippetRunes, 0)
	to := min(at+len(want)+snippetRunes, len(runes))

	var out strings.Builder
	if before := fields(runes[from:at]); before != "" {
		if from > 0 {
			out.WriteString("…")
		}
		out.WriteString(before)
		out.WriteString(" ")
	}
	out.WriteString("[")
	out.WriteString(string(runes[at : at+len(want)]))
	out.WriteString("]")
	if after := fields(runes[at+len(want) : to]); after != "" {
		out.WriteString(" ")
		out.WriteString(after)
	}
	if to < len(runes) {
		out.WriteString("…")
	}
	return out.String(), true
}

// clip is the opening of the text, for a hit the markers did not point into.
func clip(text string) string {
	runes := []rune(text)
	if len(runes) <= 2*snippetRunes {
		return fields(runes)
	}
	return fields(runes[:2*snippetRunes]) + "…"
}

// fields joins the text's words with single spaces, so a snippet stays on one
// line whatever the note's line breaks were.
func fields(runes []rune) string {
	return strings.Join(strings.Fields(string(runes)), " ")
}

// foldIndex finds query in text, ignoring case, and answers where it starts.
func foldIndex(text, query []rune) int {
	if len(query) == 0 || len(query) > len(text) {
		return -1
	}
	for at := 0; at+len(query) <= len(text); at++ {
		found := true
		for i := range query {
			if unicode.ToLower(text[at+i]) != unicode.ToLower(query[i]) {
				found = false
				break
			}
		}
		if found {
			return at
		}
	}
	return -1
}
