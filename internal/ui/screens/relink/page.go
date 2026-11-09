package relink

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// CardRow is one card the broken-links page lists: a lost card waiting for a
// block, or a frozen one that stopped following its block.
type CardRow struct {
	ID     core.ID
	Prompt string // what the card asks, or the snapshot it kept
	Note   string // the note it came from, empty when that is gone
}

// BlockRow is one block a lost card can be pointed at: where it sits, and the
// text it holds.
type BlockRow struct {
	Block     core.ID
	Note      core.ID
	NoteTitle string
	Paragraph int // its place in the note, from 1
	Text      string
}

// PageState is the broken-links page: the card and the block the user chose, the
// query narrowing the blocks, and the question the bulk repair asks.
type PageState struct {
	Query string
	// Card and Block are the chosen pair, empty until both rows are chosen.
	Card  core.ID
	Block core.ID
	// Lost and BlockRow are the rows the lists chose, -1 for none.
	Lost     int
	BlockRow int
	confirm  bool

	lostList, blockList ui.ListState
	field               ui.Handle
}

// NewPage returns the page on no chosen card.
func NewPage() PageState { return PageState{Lost: -1, BlockRow: -1} }

// PageDeps is what the page asks of the app: the three lists it derived, and
// the four writes a lost or frozen card carries. Each runs on the UI thread.
type PageDeps struct {
	// Lost are the cards whose block is gone and which kept no snapshot.
	Lost func() []CardRow
	// Frozen are the cards that stopped following their block.
	Frozen func() []CardRow
	// Blocks are the blocks the notes hold, narrowed by query.
	Blocks func(string) []BlockRow
	// Relink points one card at one block, live again.
	Relink func(card, block core.ID)
	// RelinkAll points every lost card at the chosen block.
	RelinkAll func(block core.ID)
	// Freeze points a card at the chosen block and keeps a snapshot of it, so
	// the card stops following the text it was repaired with.
	Freeze func(card, block core.ID)
	// Thaw makes a frozen card follow its block again.
	Thaw func(card core.ID)
}

// PageView draws the broken links: the lost cards on the left, the blocks they
// can be pointed at on the right, and the frozen cards below.
func PageView(c *ui.Context, s *PageState, d PageDeps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	lost, frozen := d.Lost(), d.Frozen()
	blocks := d.Blocks(s.Query)
	s.keep(lost, blocks)

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.BrowseWidth).Gap(tk.SpaceL).Children(func() {
				head(c, s, tk, msgs, lost, blocks)
				if len(lost) == 0 && len(frozen) == 0 {
					kit.Empty(c, "unlink", msgs.RecallRelinkEmpty(), func() {
						ui.Text(c, msgs.RecallRelinkEmptyBody()).FontSize(tk.TypeUI).
							TextColor(tk.TextMuted).MaxWidth(tk.SummaryWidth)
					})
					return
				}
				lists(c, s, d, tk, msgs, lost, blocks)
				actions(c, s, d, tk, msgs, lost, blocks)
				frozenSection(c, s, d, tk, msgs, frozen)
			})
		})
	})

	confirmDialog(c, s, d, msgs, lost, blocks)
}

// keep holds the choices inside the lists a reload left, so a query that drops
// rows does not leave the page pointing at one that is gone.
func (s *PageState) keep(lost []CardRow, blocks []BlockRow) {
	s.Lost = min(s.Lost, len(lost)-1)
	s.BlockRow = min(s.BlockRow, len(blocks)-1)
	s.Card, s.Block = "", ""
	if s.Lost >= 0 && s.Lost < len(lost) {
		s.Card = lost[s.Lost].ID
	}
	if s.BlockRow >= 0 && s.BlockRow < len(blocks) {
		s.Block = blocks[s.BlockRow].Block
	}
}

// head is the page's title, its explanation and the bulk repair.
func head(c *ui.Context, s *PageState, tk design.Tokens, msgs locales.Messages, lost []CardRow, blocks []BlockRow) {
	ui.Row(c).FillWidth().AlignItems(ui.End).Gap(tk.SpaceM).Children(func() {
		ui.Column(c).Gap(tk.SpaceXS).Grow(1).Children(func() {
			ui.Text(c, msgs.RecallRelinkTitle()).FontSize(tk.TypeH1).Bold()
			ui.Text(c, msgs.RecallRelinkBody()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		})
		ui.Button(c, msgs.RecallRelinkAll()).MinHeight(tk.Control).
			Disabled(len(lost) == 0 || s.Block == "").
			OnClick(func() { s.confirm = true })
		if len(lost) == 0 {
			ui.Text(c, msgs.RecallRelinkNoLost()).FontSize(tk.TypeCaption).
				TextColor(tk.TextFaint)
		} else if s.Block == "" {
			ui.Text(c, msgs.RecallRelinkChooseBlock()).FontSize(tk.TypeCaption).
				TextColor(tk.TextFaint)
		}
	})
}

// lists is the two-column choice: the lost cards beside the blocks, the block
// list narrowed by its autocomplete.
func lists(c *ui.Context, s *PageState, d PageDeps, tk design.Tokens, msgs locales.Messages, lost []CardRow, blocks []BlockRow) {
	ui.Row(c).FillWidth().AlignItems(ui.Stretch).Gap(tk.SpaceL).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(tk.SpaceS).Children(func() {
			sectionTitle(c, tk, msgs.RecallRelinkLost())
			s.lostList.Selected = &s.Lost
			s.lostList.Key = func(row int) any { return lost[row].ID }
			s.lostList.Label = func(row int) string { return lost[row].Prompt }
			ui.List(c, &s.lostList, len(lost), func(i int) {
				cardRow(c, tk, msgs, lost[i])
			}).Label(msgs.RecallRelinkLost())
			if len(lost) == 0 {
				ui.Text(c, msgs.RecallRelinkNoLost()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(tk.SpaceS).Children(func() {
			sectionTitle(c, tk, msgs.RecallRelinkBlocks())
			suggestions := make([]string, 0, len(blocks))
			for _, block := range blocks {
				suggestions = append(suggestions, block.Text)
			}
			ui.Autocomplete(c, &s.Query, suggestions).
				Placeholder(msgs.RecallRelinkSearch()).Label(msgs.RecallRelinkSearch()).
				MinHeight(tk.Control).Bind(&s.field)
			s.blockList.Selected = &s.BlockRow
			s.blockList.Key = func(row int) any { return blocks[row].Block }
			s.blockList.Label = func(row int) string { return blocks[row].Text }
			ui.List(c, &s.blockList, len(blocks), func(i int) {
				blockRow(c, tk, msgs, blocks[i])
			}).Label(msgs.RecallRelinkBlocks())
			if len(blocks) == 0 {
				ui.Text(c, msgs.RecallRelinkNoBlocks()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			}
		})
	})
}

// cardRow is one lost card: what it asks and the note it came from.
func cardRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, card CardRow) {
	ui.Column(c).FillWidth().Gap(tk.SpaceXS).Padding(tk.SpaceXS, tk.SpaceS).Children(func() {
		text := card.Prompt
		if text == "" {
			text = msgs.LostBody()
		}
		ui.Text(c, text).FontSize(tk.TypeUI).SingleLine()
		if card.Note != "" {
			ui.Text(c, card.Note).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
		}
	})
}

// blockRow is one block a card can be pointed at: its paragraph and the first
// line of what it says.
func blockRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, block BlockRow) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
		Padding(tk.SpaceXS, tk.SpaceS).Children(func() {
		ui.Text(c, msgs.RecallRelinkParagraph(block.Paragraph)).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted).Width(tk.CountColumnWidth)
		ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, block.Text).FontSize(tk.TypeUI).SingleLine()
			ui.Text(c, block.NoteTitle).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
		})
	})
}

// actions are the two repairs of the chosen pair: point the card at the block,
// or point it there and keep a snapshot so it stops following the text.
func actions(c *ui.Context, s *PageState, d PageDeps, tk design.Tokens, msgs locales.Messages, lost []CardRow, blocks []BlockRow) {
	ready := s.Lost >= 0 && s.Lost < len(lost) && s.BlockRow >= 0 && s.BlockRow < len(blocks)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		ui.PrimaryButton(c, msgs.RecallRelinkRelink()).MinHeight(tk.Control).
			Disabled(!ready).OnClick(func() { d.Relink(s.Card, s.Block) })
		ui.Button(c, msgs.RecallRelinkFreeze()).MinHeight(tk.Control).
			Disabled(!ready).OnClick(func() { d.Freeze(s.Card, s.Block) })
		if !ready {
			ui.Text(c, msgs.RecallRelinkChooseBoth()).FontSize(tk.TypeCaption).
				TextColor(tk.TextFaint)
		}
	})
}

// frozenSection is the cards that stopped following their block: what each
// kept, and the one action that makes it follow the block again.
func frozenSection(c *ui.Context, s *PageState, d PageDeps, tk design.Tokens, msgs locales.Messages, frozen []CardRow) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionTitle(c, tk, msgs.RecallRelinkFrozen())
		if len(frozen) == 0 {
			ui.Text(c, msgs.RecallRelinkFrozenEmpty()).FontSize(tk.TypeUI).
				TextColor(tk.TextMuted)
			return
		}
		ui.Column(c).Gap(tk.SpaceXS).Children(func() {
			for _, card := range frozen {
				card := card
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
					Padding(tk.SpaceXS, tk.SpaceS).Children(func() {
					ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
						ui.Text(c, card.Prompt).FontSize(tk.TypeUI).SingleLine()
						if card.Note != "" {
							ui.Text(c, card.Note).FontSize(tk.TypeLabel).
								TextColor(tk.TextMuted).SingleLine()
						}
					})
					ui.Button(c, msgs.RecallRelinkThaw()).MinHeight(tk.Control).
						Label(msgs.RecallRelinkThawCard(card.Prompt)).
						OnClick(func() { d.Thaw(card.ID) })
				})
			}
		})
	})
}

// confirmDialog asks before every lost card is pointed at one block, since a
// bulk repair rewrites every one of them and the library keeps no undo for it.
func confirmDialog(c *ui.Context, s *PageState, d PageDeps, msgs locales.Messages, lost []CardRow, blocks []BlockRow) {
	if !s.confirm {
		return
	}
	switch ui.AlertDialog(c, &s.confirm, msgs.RecallRelinkAllTitle(),
		msgs.RecallRelinkAllBody(len(lost)), msgs.Cancel(), msgs.RecallRelinkAllConfirm(len(lost))) {
	case 0, ui.AlertDismissed:
	case 1:
		if s.Block != "" {
			d.RelinkAll(s.Block)
		}
	}
}

// sectionTitle is one section's heading on the page.
func sectionTitle(c *ui.Context, tk design.Tokens, title string) {
	kit.SectionHead(c, tk, title, "", nil)
}
