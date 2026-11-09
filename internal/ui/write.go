package ui

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// searchLimit caps how many hits the search page shows per query: a search is a
// way in, not a place to read the library from.
const searchLimit = 50

// writePages is what the write area's pages read beyond the snapshot: the rows
// the store answered when the page loaded. No frame asks the store for them.
type writePages struct {
	// rows and total are the notes page's page of notes, and how many match.
	rows  []notes.Row
	total int
	// tags are the tags page's rows.
	tags []notes.TagRow
	// assets are the assets page's rows.
	assets []notes.AssetRow
	// hits and cardHits are the search page's two groups, and hitsTotal how
	// many results there are in all.
	hits, cardHits []notes.Hit
	hitsTotal      int
	// tree is the excerpt tree.
	tree []notes.TreeNode
}

// writeView draws the page of the write area the window is on: the notebook,
// or one of the six list pages its sub-navigation shows beside it.
func (a *App) writeView(c *ui.Context) {
	d := a.notesDeps()
	switch a.sub {
	case subNoteList:
		notes.ListView(c, &a.notes, d)
	case subSearch:
		notes.SearchView(c, &a.notes, d)
	case subTags:
		notes.TagsView(c, &a.notes, d)
	case subAssets:
		notes.AssetsView(c, &a.notes, d)
	case subTemplates:
		notes.TemplatesView(c, &a.notes, d)
	case subTree:
		notes.TreeView(c, &a.notes, d)
	default:
		notes.View(c, &a.notes, d)
	}
}

// loadWrite derives the rows the write page on screen reads: its own query of
// the store, run once per load so that no frame asks.
func (a *App) loadWrite() {
	if a.lib == nil {
		a.write = writePages{}
		return
	}
	switch a.sub {
	case subNoteList:
		a.loadNoteList()
	case subSearch:
		a.loadSearch()
	case subTags:
		a.loadTagRows()
	case subAssets:
		a.loadAssetRows()
	case subTree:
		a.loadExcerptTree()
	}
}

// loadNoteList asks the store for the notes page's rows: the query, the tag and
// the three filters the screen holds, in the order it chose.
func (a *App) loadNoteList() {
	a.write.rows, a.write.total = nil, 0
	rows, total, err := a.lib.QueryNoteRows(store.NoteRowQuery{
		Text:     strings.TrimSpace(a.notes.Query),
		Tag:      a.notes.TagFilter,
		Reading:  a.notes.Filter.Reading,
		HasCards: a.notes.Filter.HasCards,
		Excerpt:  a.notes.Filter.Excerpt,
		Sort:     noteSort(a.notes.Sort),
	})
	if err != nil {
		slog.Error("remit: notes list", "err", err)
		return
	}
	out := make([]notes.Row, 0, len(rows))
	for _, row := range rows {
		out = append(out, notes.Row{
			ID:            row.ID,
			Title:         row.Title,
			Summary:       row.Summary,
			Blocks:        row.Blocks,
			Cards:         row.Cards,
			ReadingBlocks: row.ReadingBlocks,
			Updated:       row.Updated,
			Excerpt:       row.Excerpt,
		})
	}
	a.write.rows, a.write.total = out, total
}

// noteSort is the order the screen chose as the store names it.
func noteSort(sort notes.Sort) store.NoteSort {
	switch sort {
	case notes.SortTitle:
		return store.NoteSortTitle
	case notes.SortCards:
		return store.NoteSortCards
	}
	return store.NoteSortUpdated
}

// loadSearch asks the store what the search page's query matched, and splits it
// into the two groups the page shows.
func (a *App) loadSearch() {
	a.write.hits, a.write.cardHits, a.write.hitsTotal = nil, nil, 0
	query := strings.TrimSpace(a.notes.SearchQuery)
	if query == "" {
		return
	}
	hits, total, err := a.lib.Search(query, "", searchLimit, 0)
	if err != nil {
		slog.Error("remit: search", "err", err)
		return
	}
	for _, hit := range hits {
		out := notes.Hit{
			Kind:  notes.HitNote,
			ID:    hit.ID,
			Note:  hit.Note,
			Title: hit.Title,
		}
		out.Before, out.Match, out.After = snippetParts(hit.Snippet)
		if hit.Kind == "card" {
			out.Kind = notes.HitCard
			a.write.cardHits = append(a.write.cardHits, out)
			continue
		}
		a.write.hits = append(a.write.hits, out)
	}
	a.write.hitsTotal = total
}

// snippetParts splits a hit's line where the store marked the match: it brackets
// it with [ and ], and the page shows the match as a marked span instead.
func snippetParts(snippet string) (before, match, after string) {
	open := strings.Index(snippet, "[")
	if open < 0 {
		return snippet, "", ""
	}
	rest := snippet[open+1:]
	close := strings.Index(rest, "]")
	if close < 0 {
		return snippet, "", ""
	}
	return snippet[:open], rest[:close], rest[close+1:]
}

// loadTagRows asks the store for the tags page's rows.
func (a *App) loadTagRows() {
	a.write.tags = nil
	for _, row := range a.lib.QueryTags(0) {
		a.write.tags = append(a.write.tags, notes.TagRow{
			Tag:    row.Tag,
			Count:  row.Count,
			Sample: row.Sample,
		})
	}
}

// loadAssetRows asks the store for the files the library holds, with the notes
// that carry each. The store keeps the reference by note ID; the page names the
// notes it opens.
func (a *App) loadAssetRows() {
	a.write.assets = nil
	rows, err := a.lib.AssetRows()
	if err != nil {
		slog.Error("remit: assets", "err", err)
		return
	}
	for _, row := range rows {
		out := notes.AssetRow{
			Name:  row.Name,
			Ext:   row.Ext,
			Bytes: row.Bytes,
		}
		for _, note := range row.Notes {
			out.Notes = append(out.Notes, notes.AssetRef{ID: note, Title: a.data.titleOf(note)})
		}
		a.write.assets = append(a.write.assets, out)
	}
}

// loadExcerptTree asks the store for the excerpts and names the ones whose
// source is gone: the store answers the forest, and the page says which roots
// stand alone.
func (a *App) loadExcerptTree() {
	a.write.tree = nil
	for _, node := range a.lib.ExcerptTree() {
		a.write.tree = append(a.write.tree, a.treeNode(node))
	}
}

// treeNode converts one node of the store's forest, and every excerpt under it.
func (a *App) treeNode(node store.ExcerptNode) notes.TreeNode {
	out := notes.TreeNode{
		Note:   node.Note,
		Title:  node.Title,
		Cards:  node.Cards,
		Orphan: a.orphanExcerpt(node.Note),
	}
	for _, child := range node.Children {
		out.Children = append(out.Children, a.treeNode(child))
	}
	return out
}

// orphanExcerpt reports whether a note is an excerpt whose source is gone: it
// quotes one block of another note, and the library no longer holds that block.
// A note that quotes nothing is a root of the tree rather than an orphan, and a
// note quoting a live block sits under the note that holds it.
func (a *App) orphanExcerpt(note core.ID) bool {
	n, ok := a.data.note(note)
	if !ok {
		return false
	}
	for _, block := range n.Blocks {
		for _, link := range core.Links(block.Text) {
			if link.Block == "" {
				continue
			}
			_, live := a.data.blocks[link.Block]
			return !live
		}
	}
	return false
}

// importAssets asks the system for files and stores them in the library: an
// asset travels with the library, named by its content, so importing is copying
// it in and saying how many arrived.
func (a *App) importAssets(c *ui.Context) {
	if a.lib == nil {
		return
	}
	paths, err := pickAssetFiles()
	if err != nil {
		a.report(c, "import assets", err)
		return
	}
	imported := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			a.report(c, "import asset", err)
			continue
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		if _, err := a.lib.PutAsset(data, ext); err != nil {
			a.report(c, "import asset", err)
			continue
		}
		imported++
	}
	if imported == 0 {
		return
	}
	a.reload()
	c.Toast(m(c).WriteAssetsImported(imported))
}

// pickAssetFiles is the assets page's picker: any files, several at a time. It
// blocks until the user answers, which is why a handler calls it.
func pickAssetFiles() ([]string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       locales.WriteAssetsImport(),
		ButtonLabel: locales.WriteAssetsImport(),
		Multiple:    true,
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// openNoteRead shows a note read whole: the notebook, in its reading view.
func (a *App) openNoteRead(id core.ID) {
	if _, ok := a.data.note(id); !ok {
		return
	}
	a.notes.Selected = string(id)
	a.notes.View = notes.Read
	a.setArea(areaWrite, subNotes)
}

// openTag shows the notes page narrowed to one tag.
func (a *App) openTag(tag string) {
	a.notes.TagFilter = tag
	a.setArea(areaWrite, subNoteList)
}

// openCardSource shows what a card renders from. The card's own page is not
// built yet, so a hit on a card opens its note read whole — where the card's
// text is — and the screen reaches it through Deps.OpenCard, which this fills.
func (a *App) openCardSource(id core.ID) {
	card, ok := a.data.card(id)
	if !ok {
		return
	}
	note, ok := a.data.noteOfBlock(card.Ref.Block)
	if !ok {
		return
	}
	a.openNoteRead(note)
}
