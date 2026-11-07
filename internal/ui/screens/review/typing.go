package review

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// hintFold is the card's {{hint:}} as a fold: a nudge kept closed until asked
// for, so it never gives the answer away by being on screen.
func hintFold(c *ui.Context, s *State, card Card) {
	msgs := locales.In(c.Locale())
	open := s.hintOpen
	ui.Accordion(c, func() {
		ui.AccordionItem(c, msgs.Hint(), &open, func() {
			ui.Text(c, card.Hint).FontSize(design.TypeBody).TextColor(c.Theme().TextMuted)
		}).OnChange(s.ToggleHint)
	})
}
