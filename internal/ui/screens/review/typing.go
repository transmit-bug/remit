package review

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// typedLimit caps what a typed answer may hold. It is a phrase, and the
// comparison is quadratic in its length.
const typedLimit = 512

// hintFold is the card's {{hint:}} as a fold: a nudge kept closed until asked
// for, so it never gives the answer away by being on screen.
func hintFold(c *ui.Context, s *State, card Card) {
	msgs := locales.In(c.Locale())
	open := s.hintOpen
	ui.Accordion(c, func() {
		ui.AccordionItem(c, msgs.Hint(), &open, func() {
			ui.Text(c, card.Hint).FontSize(design.TypeUI).TextColor(c.Theme().TextMuted)
		}).OnChange(s.ToggleHint)
	})
}

// typing is what a card with a {{type:}} tag asks before its answer: the input
// to write it in, and the one action that checks it. Enter in the input checks
// it too, so the answer is never a pointer away.
func typing(c *ui.Context, s *State, card Card) {
	msgs := locales.In(c.Locale())
	ui.TextInput(c, &s.typed).
		Label(msgs.TypedAnswer()).
		Placeholder(msgs.TypeAnswerPlaceholder()).
		AutoFocus().MaxLength(typedLimit).
		MinHeight(design.ControlHeight).
		FontSize(design.TypeBody).
		OnSubmit(func() { s.Submit(card.TypeAnswer) })
	ui.Row(c).Gap(design.SpaceM).AlignItems(ui.Center).Children(func() {
		ui.PrimaryButton(c, msgs.CheckAnswer()).
			MinHeight(design.ControlHeight).
			OnClick(func() { s.Submit(card.TypeAnswer) })
		ui.Text(c, msgs.CheckHint()).
			FontSize(design.TypeUI).TextColor(c.Theme().TextMuted)
	})
}

// comparison is what the typed answer came to: the verdict in the colour it
// carries, the two spellings aligned letter by letter, and how to read the
// difference when there is one.
func comparison(c *ui.Context, s *State) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	verdict := Of(s.verdict)
	colour, icon, word := t.Success, "check", msgs.AnswerCorrect()
	switch verdict {
	case Partial:
		colour, icon, word = t.Warning, "circle-alert", msgs.AnswerPartly()
	case Wrong:
		colour, icon, word = t.Danger, "x", msgs.AnswerWrong()
	}
	kit.StateBadge(c, icon, word, colour).AlignSelf(ui.Start)
	ui.Row(c).Gap(design.SpaceS).AlignItems(ui.Center).Children(func() {
		ui.Text(c, msgs.TypedAnswer()).
			FontSize(design.TypeUI).TextColor(t.TextMuted)
		ui.RichText(c, styledSpans(c, s.verdict)...).FontSize(design.TypeBody)
	})
	if verdict != Correct {
		ui.Text(c, msgs.CompareLegend()).
			FontSize(design.TypeUI).TextColor(design.PaletteOf(t.Dark).Faint)
	}
}

// styledSpans dresses a comparison for reading: what matched reads as the text
// does, what was typed and does not belong is struck through in the danger
// colour, and what was left out is underlined in the warning one.
func styledSpans(c *ui.Context, compare []Span) []ui.Span {
	t := c.Theme()
	out := make([]ui.Span, 0, len(compare))
	for _, span := range compare {
		switch span.Kind {
		case Extra:
			out = append(out, ui.Span{Text: span.Text, Strikethrough: true, Color: t.Danger})
		case Missing:
			out = append(out, ui.Span{Text: span.Text, Underline: true, Color: t.Warning})
		default:
			out = append(out, ui.Span{Text: span.Text, Color: t.Text})
		}
	}
	return out
}
