package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
)

// editorState is editing mode's state: the note the list chose, the draft of
// it the fields edit, the handles the focus requests go through, and where the
// user last was in it. The draft is the app's own copy: a keystroke lands in
// it, and the store is written after the fields have taken what was typed.
type editorState struct {
	selected   string
	draft      core.Note
	loaded     string // the note the draft holds
	title      ui.Handle
	first      ui.Handle
	focusFirst bool            // a note just added waits for the frame its field exists in
	focusTitle bool            // the title takes the caret in the next pass
	focusBlock core.ID         // the block whose field takes the caret in the next pass
	active     int             // the block the palette's actions act on
	carets     map[core.ID]int // where each block's caret was, for split
	addAfter   int             // a block to add after, from a key; -1 for none
	addText    string          // that block's text as the key saw it
}

// blockAction is what a block's menu, or the palette, asks the editor to do.
// The view records it and editorView performs it after its fields are built,
// so no store write happens inside the build.
type blockAction uint8

const (
	actNone blockAction = iota
	actAdd
	actSplit
	actMoveUp
	actMoveDown
	actDelete
)

// follow points the draft at the note the list chose, before that note is
// built, so a choice made in this frame shows in it.
func (e *editorState) follow(d libraryData) {
	if e.loaded == e.selected && e.draft.ID != "" {
		return
	}
	e.draft = core.Note{}
	for _, note := range d.notes {
		if string(note.ID) == e.selected {
			e.hold(note)
			return
		}
	}
	// Nothing chosen, or the choice is gone: the first note stands in.
	if len(d.notes) == 0 {
		e.loaded, e.selected = "", ""
		e.draft = core.Note{}
		return
	}
	e.hold(d.notes[0])
}

// hold takes a copy of a note as the draft, blocks and all, so that editing
// never reaches into the snapshot the next reload replaces.
func (e *editorState) hold(note core.Note) {
	note.Blocks = append([]core.Block(nil), note.Blocks...)
	e.draft = note
	e.loaded = string(note.ID)
	e.selected = string(note.ID)
	e.active = 0
	clear(e.carets)
}

// ensureIDs mints an ID for a block the library has never seen. A block read
// from a hand-written note arrives without one; minting it here keeps the
// draft's blocks and the saved ones in the same order, and gives a card a
// source to point at.
func (e *editorState) ensureIDs() {
	for i := range e.draft.Blocks {
		if e.draft.Blocks[i].ID == "" {
			e.draft.Blocks[i].ID = core.NewID("block")
		}
	}
}

// caretOf is where the caret was in a block, as the split action needs it: the
// end of the text when the block has never been in a frame.
func (e *editorState) caretOf(id core.ID) int {
	if at, ok := e.carets[id]; ok {
		return at
	}
	if block, ok := e.draft.Block(id); ok {
		return len([]rune(block.Text))
	}
	return 0
}

// editorView draws the selected note: its title, its blocks — each one
// editable, each one able to become a card or to move — and the actions that
// change the note's shape.
func (a *App) editorView(c *ui.Context, tk tokens) {
	a.editor.follow(a.data)
	note := &a.editor.draft
	if note.ID == "" {
		ui.Column(c).Grow(1).Center().Gap(tk.spaceM).Children(func() {
			tk.icon(c, "file-text", iconLarge).TextColor(tk.TextMuted)
			ui.Text(c, m(c).NoNotes()).FontSize(tk.typeHeading).TextColor(tk.TextMuted)
			ui.Button(c, m(c).NewNote()).MinHeight(tk.control).OnClick(func() { a.newNote(c) })
		})
		return
	}

	edited := false
	act, actOn := actNone, 0
	carets := make([]blockCaret, 0, len(note.Blocks))
	focused := -1

	ui.Scroll(c).Grow(1).Padding(tk.spaceXL).Children(func() {
		ui.Column(c).Gap(tk.spaceL).Children(func() {
			title := ui.TextInput(c, &note.Title).FontSize(tk.typeTitle).Bold().
				MinHeight(tk.control).Bind(&a.editor.title).Label(m(c).NoteTitle())
			if title.Changed() {
				edited = true
			}
			if a.editor.focusTitle {
				title.Focus()
				a.editor.focusTitle = false
			}

			ui.Text(c, m(c).NoteSummary(blockCount(c, len(note.Blocks)), cardCount(c, a.data.cardsInNote(*note)))).
				FontSize(tk.typeSmall).TextColor(tk.TextMuted)
			ui.Divider(c)
			for i := range note.Blocks {
				out := a.blockRow(c, tk, note, i)
				edited = edited || out.changed
				if out.act != actNone {
					act, actOn = out.act, i
				}
				carets = append(carets, blockCaret{id: note.Blocks[i].ID, at: out.caret})
				if out.focused {
					focused = i
				}
				if i == 0 && a.editor.focusFirst {
					a.editor.first.Focus(c)
					a.editor.focusFirst = false
				}
			}
		})
	})

	// What this pass saw of the fields, kept for the actions that run in later
	// frames: the split needs the caret, and the palette needs the block the
	// user is in.
	a.recordCarets(carets, focused)

	// One write per pass, once the fields have taken what was typed, rather
	// than one per keystroke inside them. A key that asked for a block, or a
	// menu that asked for an edit, wins: it saves the draft itself.
	switch {
	case a.editor.addAfter >= 0:
		at := a.editor.addAfter
		text := a.editor.addText
		a.editor.addAfter, a.editor.addText = -1, ""
		if at >= 0 && at < len(note.Blocks) {
			note.Blocks[at].Text = text
		}
		a.addBlockAfter(c, at)
	case act != actNone:
		a.runBlockAction(c, act, actOn)
	case edited:
		a.saveNote(c)
	}
}

// blockCaret is where one block's caret was in the pass that built it.
type blockCaret struct {
	id core.ID
	at int
}

// blockOutcome is what one block's row reports: whether its text changed, the
// action its menu chose, and where its caret is.
type blockOutcome struct {
	changed bool
	act     blockAction
	caret   int
	focused bool
}

// recordCarets keeps where the user is in the note. It only mirrors state the
// framework already holds, and the actions that read it run after the pass.
func (a *App) recordCarets(carets []blockCaret, focused int) {
	if a.editor.carets == nil {
		a.editor.carets = make(map[core.ID]int, len(carets))
	}
	for _, caret := range carets {
		a.editor.carets[caret.id] = caret.at
	}
	if focused >= 0 {
		a.editor.active = focused
	}
}

// blockRow draws one block — its text, the cards that already point at it, and
// the menu of changes to it — and reports what the pass saw. It is a wrapper,
// not a copied widget: see RECONCILE.md.
func (a *App) blockRow(c *ui.Context, tk tokens, note *core.Note, i int) blockOutcome {
	block := &note.Blocks[i]
	cards := a.data.cardsForBlock(block.ID)
	var out blockOutcome

	ui.Column(c).Key(string(block.ID)).Gap(tk.spaceS).Children(func() {
		field := ui.TextArea(c, &block.Text).Grow(1).
			Label(m(c).BlockLabel(i+1, note.Title)).
			FontSize(tk.typeBody).Background(tk.sunken)
		if i == 0 {
			field.Bind(&a.editor.first)
		}
		out.changed = field.Changed()
		out.focused = field.Focused()
		out.caret, _ = field.TextSelection()
		// A text area takes Enter itself, so the key is seen here before it
		// does: Cmd/Ctrl+Enter adds the block after this one and leaves the
		// text as typed for the pass that adds it.
		field.OnTextKey(func(k ui.TextKey) bool {
			if k.Key != ui.KeyEnter || k.Mods != ui.Cmd {
				return false
			}
			a.editor.addAfter, a.editor.addText = i, k.Text()
			return true
		})
		if block.ID == a.editor.focusBlock {
			field.Focus()
			a.editor.focusBlock = ""
		}

		ui.Row(c).Gap(tk.spaceS).AlignItems(ui.Center).Children(func() {
			if cards > 0 {
				badge := ui.Badge(c, cardCount(c, cards)).
					Background(tk.linkFill(tk.Accent)).TextColor(tk.linkInk(tk.Accent))
				badge.Label(m(c).CardBadge(cardCount(c, cards), note.Title))
			} else {
				ui.Button(c, m(c).MakeCard()).MinHeight(tk.control).
					OnClick(func() { a.makeCard(c, i) })
			}
			ui.Spacer(c)
			out.act = a.blockMenu(c, note, i)
		})
	})
	return out
}

// blockMenu is one block's menu: add, split, move and delete. It reports the
// action chosen in this pass, which editorView performs once every field is
// built.
func (a *App) blockMenu(c *ui.Context, note *core.Note, i int) blockAction {
	msgs := m(c)
	act := actNone
	ui.PopoverMenuButton(c, msgs.BlockActions(), func(menu *ui.Menu) {
		if menu.Item(msgs.AddBlockBelow()).Shortcut(ui.Cmd, ui.KeyEnter).Chosen() {
			act = actAdd
		}
		if menu.Item(msgs.SplitAtCursor()).Chosen() {
			act = actSplit
		}
		if menu.Item(msgs.MoveBlockUp()).Disabled(i == 0).Chosen() {
			act = actMoveUp
		}
		if menu.Item(msgs.MoveBlockDown()).Disabled(i == len(note.Blocks)-1).Chosen() {
			act = actMoveDown
		}
		menu.Separator()
		if menu.Item(msgs.DeleteBlock()).Disabled(len(note.Blocks) == 1).Chosen() {
			act = actDelete
		}
	}).Label(msgs.BlockActionsFor(i + 1))
	return act
}

// runBlockAction performs one block action on the note in the draft.
func (a *App) runBlockAction(c *ui.Context, act blockAction, i int) {
	switch act {
	case actAdd:
		a.addBlockAfter(c, i)
	case actSplit:
		a.splitBlock(c, i, a.editor.caretOf(a.editor.draft.Blocks[i].ID))
	case actMoveUp:
		a.moveBlock(c, i, -1)
	case actMoveDown:
		a.moveBlock(c, i, 1)
	case actDelete:
		a.deleteBlock(c, i)
	}
}

// saveNote writes the draft through the store and reloads, and reports whether
// it did. The draft keeps what was typed — the store trims, and the caret must
// not move under the user's hands — while the blocks keep the IDs they were
// given.
func (a *App) saveNote(c *ui.Context) bool {
	if !a.writeNote(c) {
		return false
	}
	a.reload()
	return true
}

// writeNote saves the draft without reloading, for an action that writes more
// than the note and reloads once at the end.
func (a *App) writeNote(c *ui.Context) bool {
	a.editor.ensureIDs()
	if _, err := a.lib.SaveNote(a.editor.draft); err != nil {
		a.report(c, "save note", err)
		return false
	}
	return true
}

// addBlockAfter puts an empty block after block i, saves the note and puts the
// caret in it.
func (a *App) addBlockAfter(c *ui.Context, i int) {
	note := &a.editor.draft
	if note.ID == "" {
		return
	}
	at := clamp(i+1, 0, len(note.Blocks))
	block := core.Block{ID: core.NewID("block")}
	note.Blocks = insertBlock(note.Blocks, at, block)
	if !a.saveNote(c) {
		return
	}
	a.editor.focusBlock = block.ID
}

// splitBlock divides block i at the caret, minting the second block an ID of
// its own. References keep pointing at the original block, which keeps the
// text before the split — the side that still holds what a card read, as ADR
// 0002 asks. The one decidable case is a split at the start: the original
// block keeps no text at all, so every live reference moves to the new block,
// which then holds all of it. A frozen reference keeps its snapshot either way.
func (a *App) splitBlock(c *ui.Context, i, caret int) {
	note := &a.editor.draft
	if i < 0 || i >= len(note.Blocks) {
		return
	}
	block := &note.Blocks[i]
	runes := []rune(block.Text)
	caret = clamp(caret, 0, len(runes))
	original := block.ID
	block.Text = string(runes[:caret])
	next := core.Block{ID: core.NewID("block"), Text: string(runes[caret:])}
	note.Blocks = insertBlock(note.Blocks, i+1, next)

	if !a.writeNote(c) {
		return
	}
	if strings.TrimSpace(note.Blocks[i].Text) == "" {
		a.repointCards(c, original, next.ID)
	}
	a.reload()
	a.editor.focusBlock = next.ID
}

// repointCards moves every live reference from one block to another, as a
// split that left the original block empty does: the reference follows the
// text it read. A frozen reference shows its snapshot and is left alone.
func (a *App) repointCards(c *ui.Context, from, to core.ID) {
	for _, card := range a.data.cards {
		if card.Ref.Block != from || card.Ref.Frozen != nil {
			continue
		}
		card.Ref.Block = to
		if err := a.lib.SaveCard(card); err != nil {
			a.report(c, "move reference", err)
			return
		}
	}
}

// moveBlock swaps a block with its neighbour, so the note reads in another
// order. References are by ID, so they follow their block.
func (a *App) moveBlock(c *ui.Context, i, delta int) {
	note := &a.editor.draft
	j := i + delta
	if i < 0 || i >= len(note.Blocks) || j < 0 || j >= len(note.Blocks) {
		return
	}
	note.Blocks[i], note.Blocks[j] = note.Blocks[j], note.Blocks[i]
	if !a.saveNote(c) {
		return
	}
	a.editor.focusBlock = note.Blocks[j].ID
}

// deleteBlock removes a block from the note. A card that referenced it becomes
// lost rather than losing its history: the card file and the review log are
// untouched, and the card can be re-linked to another block.
func (a *App) deleteBlock(c *ui.Context, i int) {
	note := &a.editor.draft
	if len(note.Blocks) <= 1 || i < 0 || i >= len(note.Blocks) {
		return
	}
	cards := a.data.cardsForBlock(note.Blocks[i].ID)
	note.Blocks = append(note.Blocks[:i], note.Blocks[i+1:]...)
	if !a.saveNote(c) {
		return
	}
	next := clamp(i, 0, len(note.Blocks)-1)
	a.editor.focusBlock = note.Blocks[next].ID
	c.Toast(m(c).BlockDeleted(cards))
	c.Announce(m(c).BlockDeleted(cards))
}

// focusTitle shows notes and puts the caret in the title of the note open.
func (a *App) focusTitle() {
	a.editor.follow(a.data)
	if a.editor.draft.ID == "" {
		return
	}
	a.setMode(modeNotes)
	a.editor.focusTitle = true
}

// onActiveBlock loads the note the editor is on and runs fn on the block the
// user was in. The palette can open from any mode, so it may have to load the
// note first.
func (a *App) onActiveBlock(c *ui.Context, fn func(*ui.Context, int)) {
	a.editor.follow(a.data)
	n := len(a.editor.draft.Blocks)
	if a.editor.draft.ID == "" || n == 0 {
		return
	}
	fn(c, clamp(a.editor.active, 0, n-1))
}

// blockFromPalette runs a block action from the command palette and shows the
// note it changed, so the outcome is where the action was.
func (a *App) blockFromPalette(c *ui.Context, fn func(*ui.Context, int)) {
	a.onActiveBlock(c, func(c *ui.Context, i int) {
		fn(c, i)
		a.setMode(modeNotes)
	})
}

// splitActiveBlock splits the block the caret was last in, from the palette,
// and shows the note the split happened in.
func (a *App) splitActiveBlock(c *ui.Context) {
	a.blockFromPalette(c, func(c *ui.Context, i int) {
		a.splitBlock(c, i, a.editor.caretOf(a.editor.draft.Blocks[i].ID))
	})
}

// insertBlock puts block at at, growing the slice.
func insertBlock(blocks []core.Block, at int, block core.Block) []core.Block {
	blocks = append(blocks, core.Block{})
	copy(blocks[at+1:], blocks[at:])
	blocks[at] = block
	return blocks
}

// makeCard makes a card from a block. The store mints the block's marker in
// the note source and splits the text at the first :: when the card renders.
func (a *App) makeCard(c *ui.Context, i int) {
	if i >= len(a.editor.draft.Blocks) {
		return
	}
	block := a.editor.draft.Blocks[i]
	if prompt, _ := core.Split(block.Text); prompt == "" {
		c.Toast(m(c).BlockEmpty())
		return
	}
	if block.ID == "" {
		// A block read from a hand-written note has no ID until the note is
		// saved; saving now mints one that the card can point at.
		if !a.saveNote(c) {
			return
		}
		block = a.editor.draft.Blocks[i]
	}
	if _, err := a.lib.CreateCard(block.ID); err != nil {
		a.report(c, "make card", err)
		return
	}
	// A card made now is due now, so reloading puts it at the end of the
	// queue and the card the user is on does not move.
	a.reload()
	c.Toast(m(c).CardMade(i + 1))
}

// blockCount names how many blocks a note has.
func blockCount(c *ui.Context, n int) string { return m(c).BlockCount(n) }

// cardCount names how many cards point at a block or a note.
func cardCount(c *ui.Context, n int) string { return m(c).CardCount(n) }
