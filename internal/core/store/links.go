package store

import (
	"fmt"
	"sort"
	"strings"

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

// putLinks records the wiki links a note's blocks carry, one row per link. A
// link that names one block of another note — [[Note#^block_…]], the shape an
// excerpt writes — keeps that block in anchor; the excerpt tree and the
// notebook's excerpt filter are read from it rather than from block text again.
func putLinks(e execer, note core.ID, blocks []core.Block) error {
	for at, block := range blocks {
		for _, link := range core.Links(block.Text) {
			if _, err := e.Exec(
				`INSERT INTO links (note_id, block_id, title, paragraph, anchor) VALUES (?, ?, ?, ?, ?)`,
				string(note), string(block.ID), core.NormalizeTitle(link.Title), at+1, string(link.Block),
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

// NoteOfBlock names the note a block lives in, so a card, a link or a search
// hit can be followed back to the document it belongs to.
func (l *Library) NoteOfBlock(id core.ID) (core.ID, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return "", false
	}
	ref, ok := l.index.block(id)
	if !ok {
		return "", false
	}
	return ref.note, true
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

// ExcerptNode is one note in the excerpt tree: the note itself, how many cards
// it feeds, and the excerpts taken from it.
//
// An excerpt is a note whose block carries an anchored link back to one block of
// its source ([[Title#^block_…]], what core.Excerpt writes), so the tree is the
// links between notes rather than a second index of them. A note that is itself
// an excerpt nests under the note it was taken from and can carry children of
// its own: digging into a passage is the same act at a deeper level.
type ExcerptNode struct {
	Note core.ID
	// Title is the note's title as the index read it.
	Title string
	// Cards is how many cards the note's own blocks feed.
	Cards int
	// Children are the excerpts taken from this note, by title and then ID so
	// two calls agree.
	Children []ExcerptNode
}

// ExcerptTree returns the library's excerpts as a forest: one root per note that
// has excerpts taken from it, each holding its own in turn, deepest last.
//
// An excerpt whose source block is gone — the source note was deleted, or the
// paragraph it came from was edited away — is a root of its own: it is still a
// note with a history and cards, and leaving it out would hide work. A tree is
// what the tree screen shows, and a cycle (two notes excerpting each other,
// which only hand-edited links make) is broken by the same rule: every note
// appears once, at the shallowest place it is reachable from.
//
// It is one read of the links between notes, one of the note titles and one of
// the cards per note, so a library of a hundred thousand blocks opens its tree
// without reading a single note file.
func (l *Library) ExcerptTree() []ExcerptNode {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil
	}
	titles, cards, err := l.index.noteTitles()
	if err != nil {
		return nil
	}
	edges, orphans, err := l.index.excerptEdges(titles)
	if err != nil {
		return nil
	}

	// A child under each source, in one place, and every source found so the
	// roots are the notes nothing names.
	children := map[core.ID][]core.ID{}
	named := map[core.ID]bool{}
	for child, parents := range edges {
		for _, parent := range parents {
			children[parent] = append(children[parent], child)
			named[child] = true
		}
	}

	var tree []ExcerptNode
	seen := map[core.ID]bool{}
	var build func(core.ID) ExcerptNode
	build = func(note core.ID) ExcerptNode {
		seen[note] = true
		node := ExcerptNode{Note: note, Title: titles[note], Cards: cards[note]}
		ids := children[note]
		sort.SliceStable(ids, func(i, j int) bool { return lessTitle(titles, ids[i], ids[j]) })
		for _, child := range ids {
			if seen[child] {
				continue
			}
			node.Children = append(node.Children, build(child))
		}
		return node
	}

	roots := make([]core.ID, 0, len(children)+len(orphans))
	rooted := make(map[core.ID]bool, len(children)+len(orphans))
	add := func(note core.ID) {
		if !rooted[note] {
			rooted[note] = true
			roots = append(roots, note)
		}
	}
	for note := range children {
		if !named[note] {
			add(note)
		}
	}
	// An excerpt pointing at a block nothing holds any more is a root too: it is
	// a note with cards and history of its own, and the page lists it rather
	// than hiding work whose source was deleted.
	for _, note := range orphans {
		add(note)
	}
	sort.SliceStable(roots, func(i, j int) bool { return lessTitle(titles, roots[i], roots[j]) })
	for _, root := range roots {
		tree = append(tree, build(root))
	}
	// Anything left is in a cycle, or is an excerpt whose source is gone and
	// whose source note is in one: it is reachable from no root, so it stands as
	// one rather than vanishing.
	rest := make([]core.ID, 0, len(named))
	for note := range named {
		if !seen[note] {
			rest = append(rest, note)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool { return lessTitle(titles, rest[i], rest[j]) })
	for _, note := range rest {
		if !seen[note] {
			tree = append(tree, build(note))
		}
	}
	return tree
}

// lessTitle orders two notes by title, then by ID so ties stay in one order.
func lessTitle(titles map[core.ID]string, a, b core.ID) bool {
	x, y := strings.ToLower(titles[a]), strings.ToLower(titles[b])
	if x != y {
		return x < y
	}
	return a < b
}

// noteTitles reads every note's title and how many cards its blocks feed, which
// is what a tree node says besides its children.
func (x *index) noteTitles() (map[core.ID]string, map[core.ID]int, error) {
	rows, err := x.db.Query(`SELECT id, title FROM notes`)
	if err != nil {
		return nil, nil, fmt.Errorf("store: note titles query: %w", err)
	}
	titles := map[core.ID]string{}
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			rows.Close()
			return nil, nil, err
		}
		titles[core.ID(id)] = title
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// The cards a tree node carries, by the note each one resolved to: one walk
	// of the index, not a join of every block in the library.
	rows, err = x.db.Query(`SELECT note_id, COUNT(*) FROM cards WHERE note_id <> '' GROUP BY note_id`)
	if err != nil {
		return nil, nil, fmt.Errorf("store: note cards query: %w", err)
	}
	defer rows.Close()
	cards := map[core.ID]int{}
	for rows.Next() {
		var note string
		var count int
		if err := rows.Scan(&note, &count); err != nil {
			return nil, nil, err
		}
		cards[core.ID(note)] = count
	}
	return titles, cards, rows.Err()
}

// excerptEdges reads the excerpt links: one entry per excerpt note, naming the
// notes whose blocks it points into, and the excerpts whose anchor names no note
// at all — a source note deleted, or a paragraph edited away. Those are the
// orphans, and they are roots of the tree.
func (x *index) excerptEdges(titles map[core.ID]string) (map[core.ID][]core.ID, []core.ID, error) {
	rows, err := x.db.Query(`
		SELECT l.note_id, COALESCE((
			SELECT b.note_id FROM blocks b WHERE b.id = l.anchor
			ORDER BY b.note_id, b.position LIMIT 1), '')
		  FROM links l WHERE l.anchor != ''`)
	if err != nil {
		return nil, nil, fmt.Errorf("store: excerpt links query: %w", err)
	}
	defer rows.Close()
	edges := map[core.ID][]core.ID{}
	seen := map[[2]core.ID]bool{}
	orphan := map[core.ID]bool{}
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			return nil, nil, err
		}
		from, to := core.ID(child), core.ID(parent)
		// A note the index does not hold is not a node of the tree.
		if _, ok := titles[from]; !ok {
			continue
		}
		// A note pointing into itself is not its own excerpt, and a source the
		// index no longer holds leaves the excerpt an orphan.
		if to == "" || to == from {
			orphan[from] = true
			continue
		}
		if _, ok := titles[to]; !ok {
			orphan[from] = true
			continue
		}
		// A note with a source is not an orphan, whatever a second link of it
		// points at.
		delete(orphan, from)
		key := [2]core.ID{from, to}
		if seen[key] {
			continue
		}
		seen[key] = true
		edges[from] = append(edges[from], to)
	}
	orphans := make([]core.ID, 0, len(orphan))
	for note := range orphan {
		orphans = append(orphans, note)
	}
	return edges, orphans, rows.Err()
}
