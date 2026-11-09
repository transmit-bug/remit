package ui

import (
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/screens/pluginpage"
)

// The board is a page a plugin contributed and the host draws: the page names the
// places, and the window resolves them against the library it holds, so a node
// goes to a block that a hand edit has moved on and the words beside it are the
// ones the library has now (ui-redesign-v2 §6.4).

// boardPage answers the contributed page that places a block on its board: the
// page whose data, as the last load left it, names that block. It reads the
// cached trees, so a frame or a handler may ask it and no plugin is called.
func (a *App) boardPage(block core.ID) (contrib.PageRef, bool) {
	if block == "" {
		return contrib.PageRef{}, false
	}
	keys := make([]string, 0, len(a.ui.pages))
	for key := range a.ui.pages {
		keys = append(keys, key)
	}
	// One answer for every frame: two pages may place the same block, and the
	// first by id is the one the window offers.
	sort.Strings(keys)
	for _, key := range keys {
		state := a.ui.pages[key]
		if state == nil || state.Tree.Kind == "" {
			continue
		}
		ref, ok := a.pageOf(key)
		if !ok {
			continue
		}
		for _, target := range pluginpage.BoardTargets(state.Tree) {
			if target.Block == string(block) {
				return ref, true
			}
		}
	}
	return contrib.PageRef{}, false
}

// pageOf resolves a page's cache key back to the page the library declared.
func (a *App) pageOf(key string) (contrib.PageRef, bool) {
	plugin, id, ok := strings.Cut(key, "/")
	if !ok {
		return contrib.PageRef{}, false
	}
	return a.ui.index.Page(plugin, id)
}

// OnBoard reports whether the board the window holds places this block. It is
// what the block menu and the block inspector ask before they offer "locate on
// board": an entry for a block no board draws would be a control that does
// nothing. W3b's inspector takes it through notes.Deps.
func (a *App) OnBoard(block core.ID) bool {
	_, ok := a.boardPage(block)
	return ok
}

// LocateOnBoard opens the board page on the block's node and points at it once:
// the round trip the block menu's entry makes. A block no board places leaves
// the window where it is.
func (a *App) LocateOnBoard(c *ui.Context, block core.ID) {
	ref, ok := a.boardPage(block)
	if !ok {
		return
	}
	a.openPage(c, ref)
	state := a.pageState(ref)
	state.Highlight, state.HighlightRun = string(block), state.HighlightRun+1
}

// openBoardBlock is what a board node's click runs: the note read at the block
// the node names. A node that named a block but not its note gets the note the
// block lives in.
func (a *App) openBoardBlock(c *ui.Context, note, block string) {
	id := core.ID(note)
	if id == "" {
		if held, ok := a.data.noteOfBlock(core.ID(block)); ok {
			id = held
		}
	}
	a.openNoteAt(c, id, core.ID(block))
}

// openNoteAt shows a note read at one of its blocks: where a board node and a
// card's source both land. Read mode draws no per-block anchor yet (S12 is
// W3b's), so the block is held in the reader's place — the paragraph the note is
// shown at — and the note named is shown now. A note the library no longer
// holds leaves the window where it is: a link from outside is a request.
func (a *App) openNoteAt(c *ui.Context, note, block core.ID) {
	held, ok := a.data.note(note)
	if !ok {
		return
	}
	paragraph := 0
	for i, candidate := range held.Blocks {
		if candidate.ID == block {
			paragraph = i + 1
			break
		}
	}
	a.notes.Selected = string(note)
	a.setArea(areaWrite, subNotes)
	a.notes.ShowAt(paragraph)
}

// boardNodeInfo answers what a board node says now: the note's title, the block's
// first line and how many cards the block feeds. False when the note no longer
// holds the block, which leaves the page's own words standing — a node outlives
// an edit of the paragraph it names.
func (a *App) boardNodeInfo(note, block string) (pluginpage.NodeInfo, bool) {
	id := core.ID(note)
	if id == "" {
		if held, ok := a.data.noteOfBlock(core.ID(block)); ok {
			id = held
		}
	}
	held, ok := a.data.note(id)
	if !ok {
		return pluginpage.NodeInfo{}, false
	}
	for _, text := range held.Blocks {
		if text.ID != core.ID(block) {
			continue
		}
		return pluginpage.NodeInfo{
			Title: held.Title,
			Text:  oneLine(text.Text),
			Cards: a.data.cardsForBlock(text.ID),
		}, true
	}
	return pluginpage.NodeInfo{}, false
}

// cardOrigin is the note and the block a card reads.
func (a *App) cardOrigin(card core.Card) (note, block core.ID, ok bool) {
	note, ok = a.data.noteOfBlock(card.Ref.Block)
	return note, card.Ref.Block, ok
}

// sourceText names the note a card came from and the paragraph it reads, as the
// library holds them now; the text is empty when the block is gone.
func (a *App) sourceText(note, block core.ID) (title, text string) {
	held, ok := a.data.note(note)
	if !ok {
		return "", ""
	}
	for _, candidate := range held.Blocks {
		if candidate.ID == block {
			return held.Title, oneLine(candidate.Text)
		}
	}
	return held.Title, ""
}

// oneLine is a block's text on one line, which is what a board node and a source
// preview show of it.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
