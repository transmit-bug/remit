package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
)

// editorState is editing mode's state: the note the list chose, the draft of
// it the fields edit, and the handle on its first block. The draft is the
// app's own copy: a keystroke lands in it, and the store is written after the
// fields have taken what was typed.
type editorState struct {
	selected   string
	draft      core.Note
	loaded     string // the note the draft holds
	first      ui.Handle
	focusFirst bool // a note just added waits for the frame its field exists in
}

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

// editorView draws the selected note: its blocks, each one editable and each
// one able to become a card.
func (a *App) editorView(c *ui.Context, tk tokens) {
	a.editor.follow(a.data)
	note := &a.editor.draft
	if note.ID == "" {
		ui.Column(c).Grow(1).Center().Gap(tk.spaceM).Children(func() {
			ui.Text(c, m(c).NoNotes()).FontSize(tk.typeHeading).TextColor(tk.textMuted)
			ui.Button(c, m(c).NewNote()).MinHeight(tk.control).OnClick(func() { a.newNote(c) })
		})
		return
	}

	edited := false
	ui.Scroll(c).Grow(1).Padding(tk.spaceXL).Children(func() {
		ui.Column(c).Gap(tk.spaceL).Children(func() {
			ui.Text(c, note.Title).FontSize(tk.typeTitle).Bold()
			ui.Text(c, m(c).NoteSummary(blockCount(c, len(note.Blocks)), cardCount(c, a.data.cardsInNote(*note)))).
				FontSize(tk.typeSmall).TextColor(tk.textMuted)
			ui.Divider(c)
			for i := range note.Blocks {
				if a.blockRow(c, tk, note, i) {
					edited = true
				}
				if i == 0 && a.editor.focusFirst {
					a.editor.first.Focus(c)
					a.editor.focusFirst = false
				}
			}
		})
	})
	// One write per pass, once the fields have taken what was typed, rather
	// than one per keystroke inside them.
	if edited {
		a.saveNote(c)
	}
}

// blockRow draws one block — its text, the cards that already point at it, and
// the action that makes another — and reports whether its text changed in this
// pass. It is a wrapper, not a copied widget: see RECONCILE.md.
func (a *App) blockRow(c *ui.Context, tk tokens, note *core.Note, i int) bool {
	block := &note.Blocks[i]
	cards := a.data.cardsForBlock(block.ID)
	changed := false

	ui.Row(c).Key(string(block.ID)).Gap(tk.spaceM).AlignItems(ui.Start).Children(func() {
		field := ui.TextArea(c, &block.Text).Grow(1).
			Label(m(c).BlockLabel(i+1, note.Title)).
			FontSize(tk.typeBody).Background(tk.surfaceSunken)
		if i == 0 {
			field.Bind(&a.editor.first)
		}
		changed = changed || field.Changed()

		ui.Column(c).Shrink(0).Gap(tk.spaceS).Children(func() {
			if cards > 0 {
				badge := ui.Badge(c, cardCount(c, cards)).
					Background(tk.linkFill(tk.accent)).TextColor(tk.linkInk(tk.accent))
				badge.Label(m(c).CardBadge(cardCount(c, cards), note.Title))
				return
			}
			ui.Button(c, m(c).MakeCard()).MinHeight(tk.control).
				OnClick(func() { a.makeCard(c, i) })
		})
	})
	return changed
}

// saveNote writes the draft through the store and reports whether it did. The
// draft keeps what was typed — the store trims, and the caret must not move
// under the user's hands — while the blocks keep the IDs they were given.
func (a *App) saveNote(c *ui.Context) bool {
	a.editor.ensureIDs()
	if _, err := a.lib.SaveNote(a.editor.draft); err != nil {
		a.report(c, "save note", err)
		return false
	}
	a.reload()
	return true
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
