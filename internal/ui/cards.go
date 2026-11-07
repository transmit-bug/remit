package ui

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/design"
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
		var cards []core.Card
		for _, card := range a.data.cards {
			// A suspended card feeds nothing: the badge counts what the
			// paragraph still puts in the queue.
			if card.Ref.Block == block.ID && card.Status != core.StatusSuspended {
				cards = append(cards, card)
			}
		}
		if len(cards) == 0 {
			// A paragraph that feeds nothing has no badge to show.
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
		asked, fromPlugin := a.deriveCards(block.Text)
		if !fromPlugin {
			// Without a plugin a paragraph makes no new card; the ones it
			// already has keep reading it.
			continue
		}
		type place struct {
			typ     core.ID
			ordinal int
		}
		wanted := make(map[place]bool, len(asked))
		for _, d := range asked {
			wanted[place{d.Type, d.Ordinal}] = true
		}
		held := map[place]bool{}
		for _, card := range a.data.cards {
			if card.Ref.Block != block.ID || card.Status == core.StatusSuspended {
				continue
			}
			place := place{card.Type, card.Ordinal}
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
			if _, err := a.lib.CreateCards(block.ID, asked); err != nil && !errors.Is(err, store.ErrNothingToAsk) {
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
func (a *App) cardsStrip(c *ui.Context, tk design.Tokens, note core.Note) {
	paras := a.cardsByParagraph(note)
	if len(paras) == 0 {
		return
	}
	ui.ScrollHorizontal(c).Children(func() {
		ui.Row(c).Gap(tk.SpaceS).Padding(tk.SpaceXS, 0).Children(func() {
			for _, para := range paras {
				for _, card := range para.cards {
					label := m(c).CardOnParagraph(para.paragraph, len(para.cards))
					badge := ui.ButtonBase(c.Key(string(card.ID))).
						Padding(tk.SpaceXS, tk.SpaceS).MinHeight(tk.Control-tk.SpaceS).
						Radius(tk.RadiusControl).Background(tk.Surface).
						Border(tk.Hairline, tk.RatingEdge(cardColour(tk, card))).
						Label(label).Tooltip(m(c).EditCard()).
						OnClick(func() { a.openCardEdit(c, card.ID) })
					badge.Children(func() {
						tk.Icon(c, cardIcon(card), design.IconSmall).
							TextColor(cardColour(tk, card))
						ui.Text(c, label).FontSize(tk.TypeSmall).TextColor(tk.Text)
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
func cardColour(tk design.Tokens, card core.Card) ui.Color {
	switch {
	case card.Ref.Frozen != nil:
		return tk.Warning
	case card.Status == core.StatusSuspended:
		return tk.TextMuted
	default:
		return tk.Accent
	}
}

// cardEditState is the card whose fields are being edited: one value per
// field of its type, in the type's order.
type cardEditState struct {
	open   bool
	card   core.ID
	typ    core.ID
	fields []string
	focus  []ui.Handle
}

// openCardEdit opens the field editor on a card, filled with what the card
// says now: its own edited value for a field, empty for one it takes from its
// paragraph.
func (a *App) openCardEdit(c *ui.Context, id core.ID) {
	card, ok := a.data.card(id)
	if !ok {
		return
	}
	typ, ok := a.lib.Type(card.Type)
	if !ok {
		return
	}
	edit := cardEditState{open: true, card: id, typ: typ.ID,
		fields: make([]string, len(typ.Fields)), focus: make([]ui.Handle, len(typ.Fields))}
	for i, name := range typ.Fields {
		edit.fields[i] = card.Fields[name]
	}
	a.cardEdit = edit
	if len(edit.focus) > 0 {
		edit.focus[0].Focus(c)
	}
}

// cardEditView draws the field editor over the window: one input per field of
// the card's type. An empty field takes the paragraph's value, which is what
// a card is until someone edits it.
func (a *App) cardEditView(c *ui.Context, tk design.Tokens) {
	if !a.cardEdit.open {
		return
	}
	msgs := m(c)
	submit := false

	ui.DialogBase(c, &a.cardEdit.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(paletteWidth).Padding(tk.SpaceL).Gap(tk.SpaceM))

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, msgs.EditCard()).FontSize(tk.TypeHeading).Bold().Grow(1)
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { a.cardEdit.open = false })
			})

			for i, value := range a.cardEdit.fields {
				name := a.fieldName(i)
				input := ui.TextInput(c, &a.cardEdit.fields[i]).
					Placeholder(name).Bind(&a.cardEdit.focus[i]).
					Label(name).MinHeight(tk.Control)
				if input.Submitted() {
					submit = true
				}
				_ = value
			}

			ui.Row(c).Gap(tk.SpaceS).Children(func() {
				// A handler runs after this pass, so it writes directly rather
				// than leaving a flag the view would read too early.
				ui.PrimaryButton(c, msgs.Save()).MinHeight(tk.Control).
					OnClick(func() { a.writeCardEdit(c) })
				ui.Button(c, msgs.Cancel()).MinHeight(tk.Control).
					OnClick(func() { a.cardEdit.open = false })
			})
		})
	})

	if submit {
		a.writeCardEdit(c)
	}
}

// fieldName names the i-th field of the card being edited.
func (a *App) fieldName(i int) string {
	typ, ok := a.lib.Type(a.cardEdit.typ)
	if !ok || i >= len(typ.Fields) {
		return ""
	}
	return typ.Fields[i]
}

// writeCardEdit stores the card's edited fields — only the non-empty ones, so
// an emptied field goes back to following its paragraph — and closes the
// dialog.
func (a *App) writeCardEdit(c *ui.Context) {
	card, ok := a.data.card(a.cardEdit.card)
	if !ok {
		a.cardEdit.open = false
		return
	}
	typ, ok := a.lib.Type(card.Type)
	if !ok {
		a.cardEdit.open = false
		return
	}
	fields := map[string]string{}
	for i, name := range typ.Fields {
		if i < len(a.cardEdit.fields) && strings.TrimSpace(a.cardEdit.fields[i]) != "" {
			fields[name] = a.cardEdit.fields[i]
		}
	}
	card.Fields = fields
	card.Edited = len(fields) > 0
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "edit card", err)
		return
	}
	a.cardEdit.open = false
	a.reload()
}
