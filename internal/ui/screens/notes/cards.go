package notes

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// gutterBadge is one paragraph's card badge in the gutter: how many cards read
// the paragraph, and the shape the state draws — a card due now, one never
// answered, one that comes later, and one that stopped following the paragraph.
// The words beside the number say the state, and the whole badge is the way into
// the paragraph's inspector (S11, S20).
func (s *State) gutterBadge(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps, badge Badge, block core.ID) {
	label := msgs.CardOnParagraph(badge.Cards, badge.Paragraph)
	count := itoa(badge.Cards)
	edge, ink := badgeEdge(tk, badge.State)
	if badge.State == BadgeLost {
		count = "!" + count
	}
	// The badge says how many cards with its number, and what state they are in
	// with its shape; the words for both are its description and its tip, so the
	// shape is never the only thing that carries the meaning.
	button := ui.ButtonBase(c.Key(fmt.Sprintf("block-badge-%d", badge.Paragraph))).
		Size(blockHandle-3, blockHandle-3).Radius(tk.RadiusControl).Justify(ui.Center).
		Background(tk.Surface).Border(tk.Hairline, edge).BorderStyle(badgeBorder(badge.State)).
		Description(badgeStateWord(msgs, badge.State)).
		Tooltip(label + " · " + badgeStateWord(msgs, badge.State)).
		Label(label).Cursor(ui.CursorPointer)
	if button.Hovered() {
		button.Background(tk.SurfaceHover)
	}
	button.Children(func() {
		ui.Text(c, count).FontSize(tk.TypeCaption).TextColor(ink)
	})
	if button.Clicked() {
		s.openInspector(c, d, block)
	}
}

// badgeEdge is the badge's border and number for the state its shape draws: a
// due card's edge is the mark colour, a new card's is dashed, a later one is
// faint, and one that stopped following its paragraph is the warning colour.
func badgeEdge(tk design.Tokens, state BadgeState) (edge, ink ui.Color) {
	switch state {
	case BadgeDue:
		return tk.Accent, tk.Accent
	case BadgeNew:
		return tk.BorderStrong, tk.Text
	case BadgeLost:
		return tk.Warning, tk.Warning
	}
	return tk.Border, tk.TextMuted
}

// badgeBorder is the badge's edge style for the state: only a new card's is
// dashed, so the shape alone says "never answered".
func badgeBorder(state BadgeState) ui.BorderStyle {
	if state == BadgeNew {
		return ui.BorderDashed
	}
	return ui.BorderSolid
}

// badgeStateWord names the state in words, so the shape and the colour never
// carry the meaning alone.
func badgeStateWord(msgs locales.Messages, state BadgeState) string {
	switch state {
	case BadgeDue:
		return msgs.BadgeDue()
	case BadgeNew:
		return msgs.BadgeNew()
	case BadgeLost:
		return msgs.BadgeLost()
	}
	return msgs.BadgeLater()
}

// blockBadges is the badges of the note's paragraphs by their place, which is
// how the gutter finds one for a paragraph box.
func blockBadges(d Deps, note core.Note) map[int]Badge {
	if d.Badges == nil {
		return nil
	}
	badges := d.Badges(note)
	if len(badges) == 0 {
		return nil
	}
	out := make(map[int]Badge, len(badges))
	for _, badge := range badges {
		out[badge.Paragraph] = badge
	}
	return out
}

// blockOfParagraph is the block one of the note's paragraphs holds, false when
// the draft is shorter than the document the editor shows.
func (s *State) blockOfParagraph(index int) (core.Block, bool) {
	if index < 0 || index >= len(s.Draft.Blocks) {
		return core.Block{}, false
	}
	return s.Draft.Blocks[index], true
}
