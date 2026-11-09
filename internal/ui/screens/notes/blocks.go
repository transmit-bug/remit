package notes

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

const (
	// blockGutter is the room left of the writing for its block controls.
	blockGutter float32 = 48
	// blockHandle is the height of one paragraph's handle, lined up with its
	// first line.
	blockHandle float32 = 24
)

// blockAction is a change a block handle asks for. It is applied after the
// frame is built, when the menu's Chosen has reported the choice.
type blockAction struct {
	kind  blockActionKind
	start int
	from  int
	to    int
	style richtext.ParagraphStyle
}

type blockActionKind uint8

const (
	blockActionNone blockActionKind = iota
	blockActionInsert
	blockActionDuplicate
	blockActionStyle
	blockActionMove
	blockActionDelete
)

// blockDrag is the block a gutter grip carries while the pointer drags it.
type blockDrag struct {
	From int
}

// activeBlock is the paragraph the page marks as current: the one under the
// pointer, or else the one holding the caret. A page draws its highlight from
// the boxes read before the editor, so the mark follows one frame behind the
// pointer.
func activeBlock(boxes []richtext.ParagraphBox, caret, hovered int) int {
	if hovered >= 0 {
		return hovered
	}
	active := -1
	for _, box := range boxes {
		if box.Start > caret {
			break
		}
		active = box.Index
	}
	return active
}

// blockHighlight draws the current paragraph's quiet paper tint and mark
// colour rule under the editor, so the text stays the top layer.
func (s *State) blockHighlight(c *ui.Context, tk design.Tokens, boxes []richtext.ParagraphBox) {
	active := activeBlock(boxes, s.caret(), s.blockHover)
	for _, box := range boxes {
		if box.Index != active {
			continue
		}
		top, height := box.Top, box.Height
		if top < 0 {
			height += top
			top = 0
		}
		if height <= 0 {
			return
		}
		ui.Box(c).Absolute().Left(0).Right(0).Top(top).Height(height).
			Background(tk.Text.Alpha(0.035)).
			Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 2, H: r.H}, tk.Accent.Alpha(0.8), 1)
			}).
			IgnoreInput(true).AccessHidden(true)
		return
	}
}

// blockHandles draws the controls beside the current paragraph: the one under
// the pointer from the last frame, or the caret's. The menu button stays while
// its menu is open, so moving the pointer into the menu does not make its
// anchor disappear.
func (s *State) blockHandles(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps, action *blockAction) {
	boxes := s.Doc.VisibleParagraphs()
	active := activeBlock(boxes, s.caret(), s.blockHover)
	hovered := -1
	for _, box := range boxes {
		row := ui.Row(c.Key(fmt.Sprintf("block-gutter-%d", box.Index))).
			Absolute().Left(-blockGutter).Top(max(0, box.Top)).
			Width(blockGutter).Height(blockHandle).
			AlignItems(ui.Center).Justify(ui.End).Gap(2)
		isHovered := row.Hovered()
		if isHovered {
			hovered = box.Index
		}
		open := s.blockMenus[box.Index]
		shown := box.Index == active || open
		row.Children(func() {
			if shown {
				plus := blockButton(c, tk, fmt.Sprintf("block-add-%d", box.Index), "plus", msgs.AddBlockBelow()).
					Tooltip(msgs.AddBlockBelow())
				if plus.Clicked() {
					*action = blockAction{kind: blockActionInsert, start: box.Start}
				}
			}

			grip := blockButton(c, tk, fmt.Sprintf("block-menu-%d", box.Index), "grip-vertical", msgs.BlockActionsFor(box.Index+1)).
				Tooltip(msgs.DragBlockFor(box.Index + 1)).Cursor(ui.CursorGrab)
			if !shown {
				grip.Opacity(0)
			}
			if grip.Clicked() {
				open = !open
			}
			if shown {
				s.blockMenu(c, tk, msgs, d, box, grip, &open, action)
			}
			if open {
				if s.blockMenus == nil {
					s.blockMenus = map[int]bool{}
				}
				s.blockMenus[box.Index] = true
			} else {
				delete(s.blockMenus, box.Index)
			}
		})
		row.Drag(blockDrag{From: box.Index})
		s.dropGutter(row, box.Index, tk, action)
	}
	s.blockHover = hovered
}

// blockMenu is one block's menu in a popover. It is not a PopoverMenuButton
// because the grip is also the drag handle, and a menu button takes the
// pointer away from the drag. Its rows are ordinary buttons, so click, Enter
// and Tab work; the popover adds Escape and press-outside dismissal.
func (s *State) blockMenu(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps, box richtext.ParagraphBox, anchor ui.Element, open *bool, action *blockAction) {
	style := s.Doc.Text().ParagraphStyleAt(box.Start)
	row := func(key, label string, selected, disabled, danger bool) ui.Element {
		return blockMenuRow(c, tk, fmt.Sprintf("block-menu-%d-%s", box.Index, key), label, selected, disabled, danger)
	}
	ui.Popover(c, anchor, open, func() {
		ui.Scroll(c).MaxHeight(360).Children(func() {
			ui.Column(c).Role(ui.RoleMenu).MinWidth(220).Padding(tk.SpaceXS).Gap(1).Children(func() {
				if row("add", msgs.AddBlockBelow(), false, false, false).Clicked() {
					*action = blockAction{kind: blockActionInsert, start: box.Start}
					*open = false
				}
				if row("duplicate", msgs.DuplicateBlock(), false, false, false).Clicked() {
					*action = blockAction{kind: blockActionDuplicate, start: box.Start}
					*open = false
				}
				if row("move-up", msgs.MoveBlockUp(), false, box.Index == 0, false).Clicked() {
					*action = blockAction{kind: blockActionMove, from: box.Index, to: box.Index - 1}
					*open = false
				}
				if row("move-down", msgs.MoveBlockDown(), false, box.Index+1 >= len(s.Draft.Blocks), false).Clicked() {
					*action = blockAction{kind: blockActionMove, from: box.Index, to: box.Index + 1}
					*open = false
				}
				cards := blockCards(d, s.Draft, box.Index)
				if row("delete", msgs.DeleteBlock(), false, len(s.Draft.Blocks) <= 1, true).Clicked() {
					if cards > 0 {
						s.blockDelete, s.blockLost = box.Index, cards
					} else {
						*action = blockAction{kind: blockActionDelete, from: box.Index}
					}
					*open = false
				}
				ui.Divider(c)
				for _, choice := range []struct {
					key   string
					name  string
					style richtext.ParagraphStyle
				}{
					{"body", msgs.StyleBody(), richtext.ParagraphStyle{}},
					{"title", msgs.StyleTitle(), richtext.ParagraphStyle{Heading: 1}},
					{"heading", msgs.StyleHeading(), richtext.ParagraphStyle{Heading: 2}},
					{"subheading", msgs.StyleSubheading(), richtext.ParagraphStyle{Heading: 3}},
					{"bullets", msgs.FmtBullets(), richtext.ParagraphStyle{List: richtext.Bullets}},
					{"numbers", msgs.FmtNumbers(), richtext.ParagraphStyle{List: richtext.Numbers}},
					{"quote", msgs.FmtQuote(), richtext.ParagraphStyle{Quote: true}},
					{"code", msgs.FmtCodeBlock(), richtext.ParagraphStyle{CodeBlock: true}},
				} {
					if row("style-"+choice.key, choice.name, sameBlockStyle(style, choice.style), false, false).Clicked() {
						*action = blockAction{kind: blockActionStyle, start: box.Start, style: choice.style}
						*open = false
					}
				}
			})
		})
	})
}

// blockMenuRow is one line of a block menu. The selected style is tinted, and
// a destructive action is the theme's danger colour.
func blockMenuRow(c *ui.Context, tk design.Tokens, key, label string, selected, disabled, danger bool) ui.Element {
	colour := tk.Text
	if danger {
		colour = tk.Danger
	}
	if selected {
		colour = tk.Accent
	}
	row := ui.ButtonBase(c.Key("block-menu-"+key)).
		Label(label).Justify(ui.Start).FillWidth().
		Padding(tk.SpaceXS, tk.SpaceS).Radius(tk.RadiusControl).
		TextColor(colour).Disabled(disabled).Role(ui.RoleMenuItem)
	if selected {
		row.Background(tk.Accent.Alpha(0.1))
	} else if row.Hovered() && !disabled {
		row.Background(tk.SurfaceHover)
	}
	row.Children(func() {
		ui.Text(c, label).FontSize(tk.TypeUI).TextColor(colour)
	})
	return row
}

// dropGutter makes one block's gutter take another block dragged onto it.
// Dropping below the dragged block puts it after the target; dropping above
// puts it before, which is what the highlighted target says.
func (s *State) dropGutter(row ui.Element, at int, tk design.Tokens, action *blockAction) {
	drag, over := ui.DragOver[blockDrag](row)
	if over && drag.From != at {
		row.Background(tk.Accent.Alpha(0.08))
	}
	if drag, ok := ui.Drop[blockDrag](row); ok && drag.From != at {
		*action = blockAction{kind: blockActionMove, from: drag.From, to: at}
	}
}

// blockButton is a quiet 21-DIP control in the gutter, named for assistive
// technology and lit under the pointer.
func blockButton(c *ui.Context, tk design.Tokens, key, icon, label string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).
		Size(21, 21).Radius(tk.RadiusControl).Justify(ui.Center).
		Label(label).Cursor(ui.CursorPointer).TextColor(tk.TextMuted)
	if button.Hovered() {
		button.Background(tk.SurfaceHover).TextColor(tk.Text)
	}
	button.Children(func() { tk.Icon(c, icon, design.IconSmall) })
	return button
}

// sameBlockStyle reports whether two paragraph styles are the same shape,
// leaving the in-session id out.
func sameBlockStyle(a, b richtext.ParagraphStyle) bool {
	a.ID, b.ID = "", ""
	return a == b
}

// blockCards is how many live cards read the block at index. A suspended card
// is out of the queue, so deleting its block costs it nothing it had not
// already lost.
func blockCards(d Deps, note core.Note, index int) int {
	if d.Cards == nil || index < 0 {
		return 0
	}
	for _, group := range d.Cards(note) {
		if group.Paragraph == index+1 {
			cards := 0
			for _, card := range group.Cards {
				if card.Ref.Frozen == nil {
					cards++
				}
			}
			return cards
		}
	}
	return 0
}

// confirmBlockDelete asks before a deletion that would cost cards their
// source. The alert is built after the page so the modal sits over it, and the
// chosen delete joins the pass's one block action.
func (s *State) confirmBlockDelete(c *ui.Context, msgs locales.Messages, action *blockAction) {
	if s.blockDelete < 0 {
		return
	}
	open := true
	switch ui.AlertDialog(c, &open, msgs.DeleteBlockTitle(), msgs.DeleteBlockWarning(s.blockLost),
		msgs.Cancel(), msgs.DeleteBlock()) {
	case 0, ui.AlertDismissed:
		s.blockDelete, s.blockLost = -1, 0
	case 1:
		at := s.blockDelete
		s.blockDelete, s.blockLost = -1, 0
		*action = blockAction{kind: blockActionDelete, from: at}
	}
}

// applyBlockAction changes the document as one undo step and leaves the caret
// in the paragraph that changed. Actions that reorder or remove a whole block
// rebuild the document so the draft's block identities, not the paragraphs'
// old positions, survive; those actions save here and report false. The caller
// saves the document at the end of the frame for the actions that return true.
func (s *State) applyBlockAction(c *ui.Context, d Deps, action blockAction) bool {
	if s.Doc == nil || action.kind == blockActionNone {
		return false
	}
	text := s.Doc.Text()
	start, end := text.ParagraphRange(action.start)
	switch action.kind {
	case blockActionInsert:
		attrs := text.AttributesAt(end)
		s.Doc.Edit(func(e *richtext.Edit) {
			e.ReplaceString(end, end, "\n", attrs)
			e.FormatParagraphs(end+1, end+1, func(p *richtext.ParagraphStyle) {
				*p = richtext.ParagraphStyle{}
			})
			e.Select(end+1, end+1)
		})
	case blockActionDuplicate:
		paragraph, ok := paragraphStartingAt(text, action.start)
		if !ok {
			return false
		}
		after := paragraphAfter(paragraph)
		s.Doc.Edit(func(e *richtext.Edit) {
			e.Replace(end, end, after)
			at := end + 1 + end - start
			e.Select(at, at)
		})
	case blockActionStyle:
		s.Doc.Edit(func(e *richtext.Edit) {
			e.FormatParagraphs(start, end, func(p *richtext.ParagraphStyle) {
				id := p.ID
				*p = action.style
				p.ID = id
			})
		})
	case blockActionMove:
		if action.from == action.to {
			return false
		}
		s.blocksNow()
		if action.from < 0 || action.from >= len(s.Draft.Blocks) || action.to < 0 || action.to >= len(s.Draft.Blocks) {
			return false
		}
		s.moveBlock(c, d, action.from, action.to)
		return false
	case blockActionDelete:
		s.blocksNow()
		if len(s.Draft.Blocks) <= 1 || action.from < 0 || action.from >= len(s.Draft.Blocks) {
			return false
		}
		s.deleteBlock(c, d, action.from, blockCards(d, s.Draft, action.from))
		return false
	default:
		return false
	}
	s.blockHover, s.blockMenus = -1, nil
	s.body.Focus(c)
	return true
}

// blocksNow reads the live document back into the draft before a structural
// action. Ordinary typing saved on earlier passes, but a block menu can be
// chosen in the same pass as an edit, so the draft is brought up to date here.
func (s *State) blocksNow() {
	live := s.Doc.Text().Markdown()
	s.Draft.Blocks = s.blocksFromLive(live)
	s.docText = live
}

// moveBlock puts one block at another block's place, keeping every identity:
// the cards that point at the moved block follow it rather than the position.
func (s *State) moveBlock(c *ui.Context, d Deps, from, to int) {
	blocks := s.Draft.Blocks
	block := blocks[from]
	if from < to {
		copy(blocks[from:to], blocks[from+1:to+1])
	} else {
		copy(blocks[to+1:from+1], blocks[to:from])
	}
	blocks[to] = block
	s.blockHover, s.blockMenus = -1, nil
	s.blockDelete, s.blockLost = -1, 0
	s.rebuildDocument(c, to)
	if saved, ok := d.SaveDocument(c, s.Draft); ok {
		s.Keep(saved)
	}
	c.Announce(locales.In(c.Locale()).BlockMoved(to + 1))
}

// deleteBlock removes one block and says how many cards lost their source.
// The cards themselves are kept by the store: they become lost, with their
// schedule and review history intact, and can be re-linked.
func (s *State) deleteBlock(c *ui.Context, d Deps, at, cards int) {
	msgs := locales.In(c.Locale())
	blocks := append(s.Draft.Blocks[:at], s.Draft.Blocks[at+1:]...)
	s.Draft.Blocks = blocks
	s.blockHover, s.blockMenus = -1, nil
	s.blockDelete, s.blockLost = -1, 0
	next := min(at, len(blocks)-1)
	s.rebuildDocument(c, next)
	if saved, ok := d.SaveDocument(c, s.Draft); ok {
		s.Keep(saved)
		notice := msgs.BlockDeleted(cards)
		c.Toast(notice)
		c.Announce(notice)
	}
}

// rebuildDocument replaces the editor's text from the draft after a structural
// change. The document is new because a move is a reorder, not one edit of the
// text; the caret goes to the block the user acted on, so the next keystroke
// lands where the block now is.
func (s *State) rebuildDocument(c *ui.Context, focus int) {
	text := DocumentText(s.Draft)
	if s.Doc == nil {
		s.Doc = richtext.NewDocument(s.noteDocument())
	} else {
		s.Doc.SetText(s.noteDocument())
	}
	s.docText = text
	s.rehydrate(s.deps)
	if focus >= 0 {
		at := ParagraphOffset(text, focus+1)
		s.Doc.SetSelection(at, at)
	}
	s.body.Focus(c)
}

// paragraphStartingAt returns the paragraph starting at start.
func paragraphStartingAt(text richtext.Text, start int) (richtext.Paragraph, bool) {
	for at, paragraph := range text.Paragraphs() {
		if at == start {
			return paragraph, true
		}
	}
	return richtext.Paragraph{}, false
}

// paragraphAfter is the text that, inserted at a paragraph's end, puts a copy
// of it after it: the newline that ends the original, then the copy's runs and
// style.
func paragraphAfter(paragraph richtext.Paragraph) richtext.Text {
	runs := make([]richtext.Run, 0, len(paragraph.Runs)+1)
	var attrs richtext.Attributes
	if len(paragraph.Runs) > 0 {
		attrs = paragraph.Runs[0].Attributes
	}
	runs = append(runs, richtext.Run{Text: "\n", Attributes: attrs})
	runs = append(runs, paragraph.Runs...)
	return richtext.TextOf(richtext.Paragraph{Style: paragraph.Style, Runs: runs})
}

// caret is the editor's caret, or 0 before a document has one.
func (s *State) caret() int {
	if s.Doc == nil {
		return 0
	}
	_, caret := s.Doc.Selection()
	return caret
}
