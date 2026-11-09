package notes

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// blockItems are the actions plugins contributed to a paragraph's menu and its
// inspector, in drawing order. A screen drawn without an app behind it has none.
func blockItems(d Deps, c *ui.Context) []BlockItem {
	if d.BlockItems == nil {
		return nil
	}
	return d.BlockItems(c)
}

// openInspector shows one paragraph of the note as three roles at once: the
// words themselves, the cards that read them, and the notes taken out of them
// (S20). The app derives what it shows once, as the dialog opens, so a frame
// never walks the library for it.
func (s *State) openInspector(c *ui.Context, d Deps, block core.ID) {
	if d.Inspector == nil {
		return
	}
	shown, ok := d.Inspector(block)
	if !ok {
		return
	}
	s.inspect, s.inspectOpen = &shown, true
}

// inspectorView draws the block inspector over the window. Each role is a
// section with its own rows: the paragraph read-only, the cards that read it —
// each opening its detail — and the excerpts taken from it. The footer goes back
// to the paragraph in the note, to its node on a board when one places it, and
// out.
func (s *State) inspectorView(c *ui.Context, tk design.Tokens, d Deps) {
	if !s.inspectOpen || s.inspect == nil {
		return
	}
	msgs := locales.In(c.Locale())
	shown := s.inspect
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(tk.SummaryWidth).Padding(tk.SpaceL).Gap(tk.SpaceM))

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
					ui.Text(c, msgs.BlockInspector()).FontSize(tk.TypeH2).Bold()
					ui.Text(c, msgs.ReadingParagraph(s.paragraphOf(shown.Block.ID))).
						FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
				})
				if shown.ReadingPosition {
					ui.Badge(c, msgs.InspectorReadingPosition()).
						Label(msgs.InspectorReadingPosition())
				}
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { s.inspectOpen = false })
			})

			// The paragraph itself: what the cards read, and what an excerpt
			// would take.
			ui.Text(c, msgs.InspectorText()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			ui.Column(c).FillWidth().MaxHeight(tk.Control * 4).Padding(tk.SpaceS).
				Background(tk.Sunken).Radius(tk.RadiusControl).Children(func() {
				ui.Scroll(c).MaxHeight(tk.Control * 4).Children(func() {
					markdown.View(c, shown.Block.Text, s.imageOptions())
				})
			})

			ui.Divider(c)
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, msgs.InspectorCards(len(shown.Cards))).FontSize(tk.TypeLabel).
					TextColor(tk.TextMuted).Grow(1)
				if d.OpenCardEditor != nil {
					ui.Button(c, msgs.MakeCard()).MinHeight(tk.Control).OnClick(func() {
						s.inspectOpen = false
						d.OpenCardEditor(c, shown.Block.ID)
					})
				}
			})
			if len(shown.Cards) == 0 {
				ui.Text(c, msgs.InspectorNoCards()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			} else {
				// A paragraph may feed many cards: the section scrolls rather
				// than growing the dialog past the window.
				ui.Scroll(c).MaxHeight(tk.Control * 3).Children(func() {
					ui.Column(c).Gap(tk.SpaceXS).Children(func() {
						for _, card := range shown.Cards {
							s.inspectorCardRow(c, tk, msgs, d, card)
						}
					})
				})
			}

			ui.Divider(c)
			ui.Text(c, msgs.InspectorExcerpts(len(shown.Excerpts))).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted)
			if len(shown.Excerpts) == 0 {
				ui.Text(c, msgs.InspectorNoExcerpts()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			} else {
				ui.Scroll(c).MaxHeight(tk.Control * 3).Children(func() {
					ui.Column(c).Gap(tk.SpaceXS).Children(func() {
						for _, excerpt := range shown.Excerpts {
							excerptRow(c, tk, excerpt, d)
						}
					})
				})
			}

			// A plugin's actions for a paragraph sit at the bottom, under the
			// host's, as they do in the block menu.
			inspectItems(c, tk, msgs, d)

			ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
				if d.LocateOnBoard != nil && shown.OnBoard {
					ui.Button(c, "").Label(msgs.InspectorLocateOnBoard()).MinHeight(tk.Control).
						Children(func() {
							tk.IconText(c, "layers", msgs.InspectorLocateOnBoard(), design.IconSmall)
						}).
						OnClick(func() {
							s.inspectOpen = false
							d.LocateOnBoard(c, shown.Block.ID)
						})
				}
				ui.Button(c, "").Label(msgs.InspectorLocateInNote()).MinHeight(tk.Control).
					Children(func() {
						tk.IconText(c, "arrow-left", msgs.InspectorLocateInNote(), design.IconSmall)
					}).
					OnClick(func() {
						s.inspectOpen = false
						s.jumpToBlock(c, shown.Block.ID)
					})
				ui.Spacer(c)
				ui.Button(c, msgs.Close()).MinHeight(tk.Control).
					OnClick(func() { s.inspectOpen = false })
			})
		})
	})
	// The dialog closes on Escape and on a press outside it; keeping the state
	// in step is what stops it reopening on the next frame.
	if !open {
		s.inspectOpen = false
	}
}

// inspectorCardRow is one card that reads the paragraph: what it asks, where its
// text comes from now, and the way to its own detail.
func (s *State) inspectorCardRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps, card InspectorCard) {
	row := ui.ButtonBase(c.Key("card-"+string(card.Card.ID))).FillWidth().
		Justify(ui.Start).Padding(tk.SpaceS, tk.SpaceS).MinHeight(tk.Control).
		Radius(tk.RadiusControl).Label(card.Title)
	if row.Hovered() {
		row.Background(tk.SurfaceHover)
	}
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
			ui.Text(c, card.Title).FontSize(tk.TypeUI).TextColor(tk.Text).Grow(1).SingleLine()
			linkState := linkStateBadge(card.State, msgs)
			ui.Badge(c, linkState).Label(linkState)
		})
	})
	if row.Clicked() && d.OpenCardDetail != nil {
		s.inspectOpen = false
		d.OpenCardDetail(c, card.Card.ID)
	}
}

// excerptRow is one note taken out of the paragraph: what it is called, the line
// that opens, and the way back to it.
func excerptRow(c *ui.Context, tk design.Tokens, excerpt InspectorExcerpt, d Deps) {
	row := ui.ButtonBase(c.Key("excerpt-"+string(excerpt.Note))).FillWidth().
		Justify(ui.Start).Padding(tk.SpaceS, tk.SpaceS).MinHeight(tk.Control).
		Radius(tk.RadiusControl).Label(excerpt.Title)
	if row.Hovered() {
		row.Background(tk.SurfaceHover)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
			ui.Text(c, excerpt.Title).FontSize(tk.TypeUI).TextColor(tk.Text).SingleLine()
			ui.Text(c, excerpt.Text).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
		})
	})
	if row.Clicked() && d.OpenNote != nil {
		d.OpenNote(excerpt.Note)
	}
}

// inspectItems is a paragraph's plugin actions in the inspector's footer.
func inspectItems(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps) {
	items := blockItems(d, c)
	if len(items) == 0 {
		return
	}
	ui.Divider(c)
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceXS).Children(func() {
		ui.Text(c, msgs.BlockActions()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).Grow(1)
	})
	for _, item := range items {
		if ui.Button(c, item.Label).MinHeight(tk.Control).Justify(ui.Start).Clicked() {
			item.Run(c)
		}
	}
}

// galleryOf is the paragraph of a block in the note open now, counted from one
// as the reader counts; zero when the note no longer holds it.
func (s *State) paragraphOf(block core.ID) int {
	for i, held := range s.Draft.Blocks {
		if held.ID == block {
			return i + 1
		}
	}
	return 0
}

// jumpToBlock goes back to a paragraph of the note: read mode marks it as the
// one on screen, and the writing view puts the caret in it, which is what
// scrolls it into view.
func (s *State) jumpToBlock(c *ui.Context, block core.ID) {
	paragraph := s.paragraphOf(block)
	if paragraph == 0 {
		return
	}
	if s.View == Read {
		s.readBlock = paragraph - 1
		return
	}
	s.jumpTo = paragraph
}

// linkStateBadge names where a card's text comes from now, in the words the
// rest of the window uses.
func linkStateBadge(state core.LinkState, msgs locales.Messages) string {
	switch state {
	case core.LinkFrozen:
		return msgs.Frozen()
	case core.LinkLost:
		return msgs.Lost()
	}
	return msgs.CardLinkLive()
}
