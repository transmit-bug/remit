package ui

import (
	"fmt"

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
			ui.Text(c, "No notes yet.").FontSize(tk.typeHeading).TextColor(tk.textMuted)
			ui.Button(c, "New note").MinHeight(tk.control).OnClick(a.newNote)
		})
		return
	}

	ui.Scroll(c).Grow(1).Padding(tk.spaceXL).Children(func() {
		ui.Column(c).Gap(tk.spaceL).Children(func() {
			ui.Text(c, note.Title).FontSize(tk.typeTitle).Bold()
			ui.Text(c, fmt.Sprintf("%s · %s", blockCount(len(note.Blocks)), cardCount(a.library.cardsInNote(note)))).
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
			Label(fmt.Sprintf("Block %d · %s", i+1, note.Title)).
			FontSize(tk.typeBody).Background(tk.surfaceSunken)
		if i == 0 {
			field.Bind(&a.editor.first)
		}

		ui.Column(c).Shrink(0).Gap(tk.spaceS).Children(func() {
			if cards > 0 {
				badge := ui.Badge(c, cardCount(cards)).
					Background(tk.linkFill(tk.accent)).TextColor(tk.linkInk(tk.accent))
				badge.Label(fmt.Sprintf("%s, %s", cardCount(cards), note.Title))
				return
			}
			ui.Button(c, "Make card").MinHeight(tk.control).
				OnClick(func() { a.makeCard(c, note, i) })
		})
	})
}

// makeCard makes a card from a block, splitting its text at the first :: the
// way the content model does.
func (a *App) makeCard(c *ui.Context, note core.Note, i int) {
	block := note.Blocks[i]
	if prompt, _ := core.Split(block.Text); prompt == "" {
		c.Toast("This block is empty.")
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
	c.Toast(fmt.Sprintf("Card made from block %d", i+1))
}

// blockCount names how many blocks a note has.
func blockCount(n int) string {
	if n == 1 {
		return "1 block"
	}
	return fmt.Sprintf("%d blocks", n)
}

// cardCount names how many cards point at a block or a note.
func cardCount(n int) string {
	if n == 1 {
		return "1 card"
	}
	return fmt.Sprintf("%d cards", n)
}
