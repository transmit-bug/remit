package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
)

// editorState is editing mode's state: the note the list chose, the handle on
// its first block, and whether a note just added waits for the caret.
type editorState struct {
	selected   string
	first      ui.Handle
	focusFirst bool
}

// note is the note the notes list chose, or the first one while the list has
// not chosen yet.
func (e editorState) note(lib *demoLibrary) (core.Note, bool) {
	for _, n := range lib.notes {
		if string(n.ID) == e.selected {
			return n, true
		}
	}
	if len(lib.notes) > 0 {
		return lib.notes[0], true
	}
	return core.Note{}, false
}

// editorView draws the selected note: its blocks, each one editable and each
// one able to become a card.
func (a *App) editorView(c *ui.Context, tk tokens) {
	note, ok := a.editor.note(a.library)
	if !ok {
		ui.Column(c).Grow(1).Center().Gap(tk.spaceM).Children(func() {
			ui.Text(c, m(c).NoNotes()).FontSize(tk.typeHeading).TextColor(tk.textMuted)
			ui.Button(c, m(c).NewNote()).MinHeight(tk.control).OnClick(a.newNote)
		})
		return
	}

	ui.Scroll(c).Grow(1).Padding(tk.spaceXL).Children(func() {
		ui.Column(c).Gap(tk.spaceL).Children(func() {
			ui.Text(c, note.Title).FontSize(tk.typeTitle).Bold()
			ui.Text(c, m(c).NoteSummary(blockCount(c, len(note.Blocks)), cardCount(c, a.library.cardsInNote(note)))).
				FontSize(tk.typeSmall).TextColor(tk.textMuted)
			ui.Divider(c)
			for i := range note.Blocks {
				a.blockRow(c, tk, note, i)
				// A note just added waits for the frame its first field exists in.
				if i == 0 && a.editor.focusFirst {
					a.editor.first.Focus(c)
					a.editor.focusFirst = false
				}
			}
		})
	})
}

// blockRow draws one block: its text, the cards that already point at it, and
// the action that makes another. It is a wrapper, not a copied widget: see
// RECONCILE.md.
func (a *App) blockRow(c *ui.Context, tk tokens, note core.Note, i int) {
	// The note is a copy of the library's, sharing its blocks' array: the field
	// edits the stored block.
	block := &note.Blocks[i]
	cards := a.library.cardsForBlock(block.ID)

	ui.Row(c).Key(string(block.ID)).Gap(tk.spaceM).AlignItems(ui.Start).Children(func() {
		field := ui.TextArea(c, &block.Text).Grow(1).
			Label(m(c).BlockLabel(i+1, note.Title)).
			FontSize(tk.typeBody).Background(tk.surfaceSunken)
		if i == 0 {
			field.Bind(&a.editor.first)
		}

		ui.Column(c).Shrink(0).Gap(tk.spaceS).Children(func() {
			if cards > 0 {
				badge := ui.Badge(c, cardCount(c, cards)).
					Background(tk.linkFill(tk.accent)).TextColor(tk.linkInk(tk.accent))
				badge.Label(m(c).CardBadge(cardCount(c, cards), note.Title))
				return
			}
			ui.Button(c, m(c).MakeCard()).MinHeight(tk.control).
				OnClick(func() { a.makeCard(c, note, i) })
		})
	})
}

// makeCard makes a card from a block, splitting its text at the first :: the
// way the content model does.
func (a *App) makeCard(c *ui.Context, note core.Note, i int) {
	block := note.Blocks[i]
	if prompt, _ := core.Split(block.Text); prompt == "" {
		c.Toast(m(c).BlockEmpty())
		return
	}
	card := core.Card{
		ID:     core.NewID("card"),
		Ref:    core.Reference{Block: block.ID},
		Status: core.StatusNew,
	}
	a.library.cards = append(a.library.cards, card)
	if a.review.started {
		a.review.queue = append(a.review.queue, card.ID)
	}
	c.Toast(m(c).CardMade(i + 1))
}

// blockCount names how many blocks a note has.
func blockCount(c *ui.Context, n int) string { return m(c).BlockCount(n) }

// cardCount names how many cards point at a block or a note.
func cardCount(c *ui.Context, n int) string { return m(c).CardCount(n) }
