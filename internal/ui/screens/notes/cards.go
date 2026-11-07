package notes

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// cardsStrip shows the note's card paragraphs as badges: how many cards each
// feeds, and a button that edits the overrides of each. It sits above the
// document rather than in a gutter, so that it never covers the text it
// describes.
func (s *State) cardsStrip(c *ui.Context, tk design.Tokens, d Deps) {
	paras := d.Cards(s.Draft)
	if len(paras) == 0 {
		return
	}
	msgs := locales.In(c.Locale())
	ui.ScrollHorizontal(c).Children(func() {
		ui.Row(c).Gap(tk.SpaceS).Padding(tk.SpaceXS, 0).Children(func() {
			for _, para := range paras {
				for _, card := range para.Cards {
					label := msgs.CardOnParagraph(para.Paragraph, len(para.Cards))
					badge := ui.ButtonBase(c.Key(string(card.ID))).
						Padding(tk.SpaceXS, tk.SpaceS).MinHeight(tk.Control-tk.SpaceS).
						Radius(tk.RadiusControl).Background(tk.Surface).
						Border(tk.Hairline, tk.RatingEdge(cardColour(tk, card))).
						Label(label).Tooltip(msgs.EditCard()).
						OnClick(func() { d.OpenCardEdit(c, card.ID) })
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
