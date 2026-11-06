package ui

import (
	"errors"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// paragraphCards is one paragraph of the note that feeds cards: the paragraph's
// place in the document, the block it names, and the cards that point at it.
type paragraphCards struct {
	paragraph int // counted from one, as the user counts paragraphs
	block     core.ID
	cards     []core.Card
}

// cardsByParagraph lists the note's card-bearing paragraphs in document order.
func (a *App) cardsByParagraph(note core.Note) []paragraphCards {
	var out []paragraphCards
	for i, block := range note.Blocks {
		// Only a paragraph that asks something is a card's source, so a
		// paragraph whose separator or blanks went away has no badge to show.
		if len(core.Derive(block.Text)) == 0 {
			continue
		}
		var cards []core.Card
		for _, card := range a.data.cards {
			if card.Ref.Block == block.ID {
				cards = append(cards, card)
			}
		}
		if len(cards) == 0 {
			continue
		}
		out = append(out, paragraphCards{paragraph: i + 1, block: block.ID, cards: cards})
	}
	return out
}

// reconcileCards applies the note's shape to its cards after a save. A paragraph
// that asks something makes the card it asks for — one per blank, both ways, or
// one — unless a card already holds that place; a card whose place the paragraph
// no longer asks for is suspended, out of the queue with its history kept.
// Nothing is deleted.
func (a *App) reconcileCards(c *ui.Context) {
	if a.lib == nil || a.editor.draft.ID == "" {
		return
	}
	wrote := false
	for _, block := range a.editor.draft.Blocks {
		asked := core.Derive(block.Text)
		type place struct {
			kind    core.Kind
			ordinal int
		}
		wanted := make(map[place]bool, len(asked))
		for _, d := range asked {
			wanted[place{d.Kind, d.Ordinal}] = true
		}
		held := map[place]bool{}
		for _, card := range a.data.cards {
			if card.Ref.Block != block.ID || card.Status == core.StatusSuspended {
				continue
			}
			place := place{card.Kind, card.Ordinal}
			if !wanted[place] {
				// The paragraph stopped asking this: it leaves the queue.
				card.Status = core.StatusSuspended
				if err := a.lib.SaveCard(card); err != nil {
					a.report(c, "suspend card", err)
					return
				}
				wrote = true
				continue
			}
			held[place] = true
		}
		missing := 0
		for range wanted {
			missing++
		}
		if missing == len(held) && missing == len(wanted) {
			continue
		}
		if missing > 0 {
			// A card the note asks for and the library does not hold — a new
			// separator, a new blank — is made now.
			if _, err := a.lib.CreateCards(block.ID); err != nil && !errors.Is(err, store.ErrNothingToAsk) {
				a.report(c, "make card", err)
				return
			}
			wrote = true
		}
	}
	if wrote {
		a.reload()
	}
}

// cardsStrip shows the note's card paragraphs as badges: how many cards each
// feeds, and a button that edits the overrides of each. It sits above the
// document rather than in a gutter, so that it never covers the text it
// describes.
func (a *App) cardsStrip(c *ui.Context, tk tokens, note core.Note) {
	paras := a.cardsByParagraph(note)
	if len(paras) == 0 {
		return
	}
	ui.ScrollHorizontal(c).Children(func() {
		ui.Row(c).Gap(tk.spaceS).Padding(tk.spaceXS, 0).Children(func() {
			for _, para := range paras {
				for _, card := range para.cards {
					label := m(c).CardOnParagraph(para.paragraph, len(para.cards))
					badge := ui.ButtonBase(c).Key(string(card.ID)).
						Padding(tk.spaceXS, tk.spaceS).MinHeight(tk.control-tk.spaceS).
						Radius(tk.radiusControl).Background(tk.Surface).
						Border(tk.hairline, tk.ratingEdge(cardColour(tk, card))).
						Label(label).Tooltip(m(c).EditCard()).
						OnClick(func() { a.openCardEdit(c, card.ID) })
					badge.Children(func() {
						tk.icon(c, cardIcon(card), iconSmall).
							TextColor(cardColour(tk, card))
						ui.Text(c, label).FontSize(tk.typeSmall).TextColor(tk.Text)
					})
				}
			}
		})
	})
}

// cardIcon names a card's state in the strip: a card that is still fed by its
// paragraph, one whose snapshot stopped following it, one whose source is gone.
func cardIcon(card core.Card) string {
	switch {
	case card.Ref.Frozen != nil:
		return "snowflake"
	case card.Status == core.StatusSuspended:
		return "moon"
	default:
		return "sparkles"
	}
}

// cardColour is the badge's edge for the state the icon names.
func cardColour(tk tokens, card core.Card) ui.Color {
	switch {
	case card.Ref.Frozen != nil:
		return tk.Warning
	case card.Status == core.StatusSuspended:
		return tk.TextMuted
	default:
		return tk.Accent
	}
}

// cardEditState is the card whose prompt and answer overrides are being edited.
type cardEditState struct {
	open    bool
	card    core.ID
	prompt  string
	answer  string
	field   ui.Handle
	answerF ui.Handle
}

// openCardEdit opens the override editor on a card, filled with what the card
// says now: its own override, empty for a side it takes from its paragraph.
func (a *App) openCardEdit(c *ui.Context, id core.ID) {
	card, ok := a.data.card(id)
	if !ok {
		return
	}
	a.cardEdit.open = true
	a.cardEdit.card = id
	a.cardEdit.prompt = card.Prompt
	a.cardEdit.answer = card.Answer
	a.cardEdit.field.Focus(c)
}

// cardEditView draws the override editor over the window: a prompt and an answer
// that replace the paragraph's, and nothing else. An empty side takes the
// paragraph's, which is what a card is until someone overrides it.
func (a *App) cardEditView(c *ui.Context, tk tokens) {
	if !a.cardEdit.open {
		return
	}
	msgs := m(c)
	submit := false

	ui.DialogBase(c, &a.cardEdit.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.scrim)
		panel = kit.StylePanel(c, panel.Width(paletteWidth).Padding(tk.spaceL).Gap(tk.spaceM))
		panel.Label(msgs.EditCard())

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.spaceS).Children(func() {
				ui.Text(c, msgs.EditCard()).FontSize(tk.typeHeading).Bold().Grow(1)
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.icon(c, "x", iconSmall) }).
					OnClick(func() { a.cardEdit.open = false })
			})

			prompt := ui.TextInput(c, &a.cardEdit.prompt).
				Placeholder(msgs.CardPrompt()).Bind(&a.cardEdit.field).
				Label(msgs.CardPrompt()).MinHeight(tk.control)
			answer := ui.TextInput(c, &a.cardEdit.answer).
				Placeholder(msgs.CardAnswer()).Bind(&a.cardEdit.answerF).
				Label(msgs.CardAnswer()).MinHeight(tk.control)
			if prompt.Submitted() || answer.Submitted() {
				submit = true
			}

			ui.Row(c).Gap(tk.spaceS).Children(func() {
				// A handler runs after this pass, so it writes directly rather
				// than leaving a flag the view would read too early.
				ui.PrimaryButton(c, msgs.Save()).MinHeight(tk.control).
					OnClick(func() { a.writeCardEdit(c) })
				ui.Button(c, msgs.Cancel()).MinHeight(tk.control).
					OnClick(func() { a.cardEdit.open = false })
			})
		})
	})

	if submit {
		a.writeCardEdit(c)
	}
}

// writeCardEdit stores the card's overrides and closes the dialog.
func (a *App) writeCardEdit(c *ui.Context) {
	card, ok := a.data.card(a.cardEdit.card)
	if !ok {
		a.cardEdit.open = false
		return
	}
	card.Prompt = a.cardEdit.prompt
	card.Answer = a.cardEdit.answer
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "edit card", err)
		return
	}
	a.cardEdit.open = false
	a.reload()
}
