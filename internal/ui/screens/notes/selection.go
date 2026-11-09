package notes

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// selectionBar is the action bar a selection raises (S15): it floats above the
// paragraph the selection is in, and offers the three things a passage is for —
// a cloze blank, a card, and a note of its own. The editor hides nothing of the
// selection; the bar names the paragraph it will act on, so the words on screen
// and the words written stay the same.
func (s *State) selectionBar(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps, boxes []richtext.ParagraphBox) {
	if s.Doc == nil || d.ClozeSelection == nil {
		return
	}
	start, end := s.Doc.Selection()
	if start > end {
		start, end = end, start
	}
	block, at, within := s.selectionBlock(start, end)
	selected := s.Doc.Text().Slice(start, end).String()
	want := within && len([]rune(selected)) >= 2

	// The bar belongs to the selection it was raised for: another selection
	// raises it again, and the same one that was dismissed stays dismissed
	// until the document lets go of it.
	if !want {
		s.selectOpen = false
		if start == end {
			s.selectSkipped = [2]int{}
		}
		return
	}
	if s.selectOpen && s.selectBlock != block.ID {
		s.selectOpen = false // a selection in another paragraph raises its own bar
	}
	if !s.selectOpen && s.selectSkipped != [2]int{start, end} {
		// The bar is raised for this selection and acts on it: a press on the
		// bar is not a press in the document, so what the editor does with its
		// caret meanwhile is not the passage the user chose.
		s.selectOpen, s.selectRange, s.selectBlock = true, [2]int{start, end}, block.ID
	}
	if !s.selectOpen {
		return
	}
	start, end = s.selectRange[0], s.selectRange[1]
	anchor := selectionAnchor(c, tk, boxes, at)
	if !anchor.Valid() {
		// The paragraph is scrolled out of view: a bar with nowhere to sit waits
		// for it to come back.
		return
	}

	open := s.selectOpen
	ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		panel = kit.StylePanel(c, panel.AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).
			Margin(6, 0, 0, 0).Padding(tk.SpaceXS).Radius(tk.RadiusPanel))
		panel.Role(ui.RoleToolbar).Label(msgs.SelectionActions())
		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(2).Children(func() {
				cloze := ui.Button(c, "").Label(msgs.SelectionCloze()).Tooltip(msgs.SelectionCloze()).
					MinHeight(tk.Control).KeepFocus()
				cloze.Children(func() {
					tk.IconText(c, "braces", msgs.SelectionCloze(), design.IconSmall)
				})
				if cloze.Clicked() {
					// The app rewrites the paragraph and makes the card that
					// reads the blank; the note is read again from it.
					if d.ClozeSelection(c, s.Draft.ID, block.ID, selected) {
						s.Loaded = ""
					}
					s.dismissSelection(start, end)
				}

				card := ui.Button(c, "").Label(msgs.MakeCard()).Tooltip(msgs.MakeCard()).
					MinHeight(tk.Control).KeepFocus()
				card.Children(func() { tk.IconText(c, "sparkles", msgs.MakeCard(), design.IconSmall) })
				if card.Clicked() && d.OpenCardEditor != nil {
					d.OpenCardEditor(c, block.ID)
					s.dismissSelection(start, end)
				}

				excerpt := ui.Button(c, "").Label(msgs.ExcerptWholeBlock()).
					Tooltip(msgs.ExcerptWholeBlock()).MinHeight(tk.Control).KeepFocus()
				excerpt.Children(func() {
					tk.IconText(c, "file-text", msgs.ExcerptWholeBlock(), design.IconSmall)
				})
				if excerpt.Clicked() && d.Excerpt != nil {
					d.Excerpt(c, s.Draft.ID, block.ID)
					s.dismissSelection(start, end)
				}
			})
		})
	})
	// Escape or a press beside the bar dismisses it for this selection; the
	// selection itself stays, so the writing is not lost by closing a bar.
	if !open {
		s.dismissSelection(start, end)
	}
}

// HasSelection reports whether the document has words selected, which is what
// the plugin contract's note.selection key answers. It reads the editor's
// selection, so a frame may ask it and no store is touched.
func (s *State) HasSelection() bool {
	if s.Doc == nil {
		return false
	}
	anchor, caret := s.Doc.Selection()
	return anchor != caret
}

// dismissSelection closes the bar for the range it was raised for, so the same
// selection does not raise it again the frame after.
func (s *State) dismissSelection(start, end int) {
	s.selectOpen, s.selectSkipped = false, [2]int{start, end}
}

// selectionBlock is the paragraph a selected range sits in: the block it
// belongs to and its paragraph index, and false when the range spans more than
// one paragraph, which is a selection the bar does not act on.
func (s *State) selectionBlock(start, end int) (core.Block, int, bool) {
	from, to := s.paragraphAt(start), s.paragraphAt(max(start, end-1))
	if from < 0 || from != to {
		return core.Block{}, 0, false
	}
	block, ok := s.blockOfParagraph(from)
	return block, from, ok
}

// selectionAnchor is where the bar sits: a line across the editor at the top of
// the paragraph the selection is in. It takes no input and is not read out: it
// is a place, not a control.
func selectionAnchor(c *ui.Context, tk design.Tokens, boxes []richtext.ParagraphBox, paragraph int) ui.Element {
	for _, box := range boxes {
		if box.Index != paragraph {
			continue
		}
		return ui.Box(c.Key("selection-anchor")).Absolute().Left(0).Right(0).
			Top(max(0, box.Top)).Height(tk.Hairline).
			IgnoreInput(true).AccessHidden(true)
	}
	return ui.Element{}
}
