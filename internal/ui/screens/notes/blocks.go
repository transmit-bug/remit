package notes

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

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
	style richtext.ParagraphStyle
}

type blockActionKind uint8

const (
	blockActionNone blockActionKind = iota
	blockActionInsert
	blockActionDuplicate
	blockActionStyle
)

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
func (s *State) blockHandles(c *ui.Context, tk design.Tokens, msgs locales.Messages, action *blockAction) {
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
		if box.Index != active && !open {
			continue
		}
		row.Children(func() {
			style := s.Doc.Text().ParagraphStyleAt(box.Start)

			plus := blockButton(c, tk, fmt.Sprintf("block-add-%d", box.Index), "plus", msgs.AddBlockBelow()).
				Tooltip(msgs.AddBlockBelow())
			if plus.Clicked() {
				*action = blockAction{kind: blockActionInsert, start: box.Start}
			}

			more := blockButton(c, tk, fmt.Sprintf("block-menu-%d", box.Index), "ellipsis", msgs.BlockActionsFor(box.Index+1)).
				Tooltip(msgs.BlockActions())
			more.PopoverMenuButton(func(menu *ui.Menu) {
				if menu.Item(msgs.AddBlockBelow()).Chosen() {
					*action = blockAction{kind: blockActionInsert, start: box.Start}
				}
				if menu.Item(msgs.DuplicateBlock()).Chosen() {
					*action = blockAction{kind: blockActionDuplicate, start: box.Start}
				}
				menu.Separator()
				for _, choice := range []struct {
					name  string
					style richtext.ParagraphStyle
				}{
					{msgs.StyleBody(), richtext.ParagraphStyle{}},
					{msgs.StyleTitle(), richtext.ParagraphStyle{Heading: 1}},
					{msgs.StyleHeading(), richtext.ParagraphStyle{Heading: 2}},
					{msgs.StyleSubheading(), richtext.ParagraphStyle{Heading: 3}},
					{msgs.FmtBullets(), richtext.ParagraphStyle{List: richtext.Bullets}},
					{msgs.FmtNumbers(), richtext.ParagraphStyle{List: richtext.Numbers}},
					{msgs.FmtQuote(), richtext.ParagraphStyle{Quote: true}},
					{msgs.FmtCodeBlock(), richtext.ParagraphStyle{CodeBlock: true}},
				} {
					if menu.Item(choice.name).Checked(sameBlockStyle(style, choice.style)).Chosen() {
						*action = blockAction{kind: blockActionStyle, start: box.Start, style: choice.style}
					}
				}
			}, ui.PopoverMenuButtonOptions{Open: &open})
			if open {
				if s.blockMenus == nil {
					s.blockMenus = map[int]bool{}
				}
				s.blockMenus[box.Index] = true
			} else {
				delete(s.blockMenus, box.Index)
			}
		})
	}
	s.blockHover = hovered
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

// applyBlockAction changes the document as one undo step and leaves the caret
// in the paragraph that changed. The caller saves the document at the end of
// the frame.
func (s *State) applyBlockAction(c *ui.Context, action blockAction) bool {
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
	default:
		return false
	}
	s.blockHover, s.blockMenus = -1, nil
	s.body.Focus(c)
	return true
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
