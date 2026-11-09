package notes

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// readView is the note as it reads (S12): its title, then its paragraphs one
// under another, each numbered and marked with what it carries — the cards it
// feeds, whether something already quotes it, and whether a reading session
// stopped there. The paragraph on screen is marked, and a click moves the mark:
// the paragraph is what an excerpt takes, so it is what the reader chooses. No
// gutter here: a reading view holds no writing tools.
func (s *State) readView(c *ui.Context, tk design.Tokens, d Deps) {
	msgs := locales.In(c.Locale())
	s.Follow(d.Notes())
	if s.Draft.ID == "" {
		kit.Empty(c, "file-text", msgs.NoNotes(), func() {
			ui.Button(c, msgs.NewNote()).MinHeight(tk.Control).OnClick(func() { d.NewNote(c) })
		})
		return
	}
	// A place asked for from elsewhere — a board node, a card's source — is the
	// paragraph the reader lands on.
	if s.jumpTo > 0 {
		s.readBlock = s.jumpTo - 1
		s.jumpTo = 0
	}
	if s.readBlock < 0 || s.readBlock >= len(s.Draft.Blocks) {
		s.readBlock = 0
	}
	w, _ := c.Size()
	narrow := w <= design.BreakpointSmall
	marks := readMarks(d, s.Draft)

	ui.Scroll(c).Grow(1).MinWidth(0).Padding(tk.SpaceXL, 0).Children(func() {
		ui.Column(c).Grow(1).MaxWidth(tk.PaperWidth).AlignSelf(ui.Center).
			Padding(0, paperGutter(tk, w)).Gap(tk.SpaceS).Children(func() {
			ui.Text(c, s.Draft.Title).FontSize(displaySize(tk, w)).Bold().
				Padding(0, tk.SpaceS, tk.SpaceM, tk.SpaceS)
			for i, block := range s.Draft.Blocks {
				s.readBlockRow(c, tk, msgs, block, i, marks[block.ID], narrow)
			}
		})
	})
}

// readBlockRow is one paragraph as it reads: its number in the margin, the
// paragraph itself set as Markdown, and what it carries beside it. The row is
// one control, so the paragraph the reader picks is reachable from the keyboard
// and says which one it is.
func (s *State) readBlockRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, block core.Block, index int, mark Mark, narrow bool) {
	current := index == s.readBlock
	// The paragraph is one control: a press anywhere on it chooses it, and the
	// keyboard reaches it as a row.
	row := ui.ButtonBase(c.Key(fmt.Sprintf("read-block-%d", index))).FillWidth().
		AlignItems(ui.Start).Gap(tk.SpaceM).Padding(tk.SpaceS, tk.SpaceS).
		Radius(tk.RadiusControl).Justify(ui.Start).Cursor(ui.CursorPointer).
		Role(ui.RoleListItem)
	if current {
		row.Background(tk.Accent.Alpha(0.10)).
			Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 2, H: r.H}, tk.Accent, 1)
			})
	} else if row.Hovered() {
		row.Background(tk.Text.Alpha(0.02))
	}
	if row.Clicked() {
		s.readBlock = index
	}
	row.Children(func() {
		// The number names the row: it is the paragraph the reader counts by.
		ui.Text(c, itoa(index+1)).FontSize(tk.TypeCaption).TextColor(tk.TextFaint).
			Width(tk.SpaceXL).TextAlign(ui.End).Padding(tk.SpaceXS, 0).
			Label(msgs.ReadingParagraph(index + 1))
		ui.Column(c).Grow(1).MinWidth(0).Gap(tk.SpaceXS).Children(func() {
			markdown.View(c, block.Text, s.imageOptions())
			if narrow {
				readMarksRow(c, tk, msgs, mark)
			}
		})
		if !narrow {
			readMarksColumn(c, tk, msgs, mark)
		}
	})
}

// readMarksColumn is what a paragraph carries, in the reader's right margin: how
// many cards it feeds, whether it was excerpted, and whether a session stopped
// here.
func readMarksColumn(c *ui.Context, tk design.Tokens, msgs locales.Messages, mark Mark) {
	if emptyMark(mark) {
		return
	}
	ui.Column(c).Width(tk.SpaceXXL * 4).Shrink(0).AlignItems(ui.End).Gap(tk.SpaceXS).Children(func() {
		readMarkItems(c, tk, msgs, mark)
	})
}

// readMarksRow is the same marks under the paragraph, for a window too narrow to
// keep a margin: the reader still learns what the paragraph carries.
func readMarksRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, mark Mark) {
	if emptyMark(mark) {
		return
	}
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		readMarkItems(c, tk, msgs, mark)
	})
}

// readMarkItems are the three marks themselves, in the order the margin shows
// them.
func readMarkItems(c *ui.Context, tk design.Tokens, msgs locales.Messages, mark Mark) {
	if mark.Cards > 0 {
		ui.Badge(c, itoa(mark.Cards)).Label(msgs.CardCount(mark.Cards))
	}
	if mark.Excerpted {
		ui.Badge(c, msgs.ReadExcerptedMark()).Label(msgs.ReadExcerptedMark())
	}
	if mark.LastStop {
		ui.Text(c, msgs.ReadLastStop()).FontSize(tk.TypeCaption).TextColor(tk.Accent)
	}
}

// emptyMark says a paragraph carries nothing worth a mark.
func emptyMark(mark Mark) bool { return mark.Cards == 0 && !mark.Excerpted && !mark.LastStop }

// readMarks is what read mode says beside each paragraph of the note; a screen
// drawn without an app behind it marks nothing.
func readMarks(d Deps, note core.Note) map[core.ID]Mark {
	if d.Marks == nil {
		return nil
	}
	return d.Marks(note)
}

// CurrentReadBlock is the paragraph read mode is on, counted from zero, and the
// block it is; false when the notebook holds no note or the paragraph is gone.
func (s *State) CurrentReadBlock() (int, core.Block, bool) {
	if s.Doc == nil || s.Draft.ID == "" || s.readBlock < 0 || s.readBlock >= len(s.Draft.Blocks) {
		return 0, core.Block{}, false
	}
	return s.readBlock, s.Draft.Blocks[s.readBlock], true
}

// ReadParagraph is the paragraph read mode is on, counted from one as the reader
// counts; zero when there is none.
func (s *State) ReadParagraph() int {
	if _, _, ok := s.CurrentReadBlock(); !ok {
		return 0
	}
	return s.readBlock + 1
}

// displaySize is the paper's title size: the display step, one step down on a
// small window, so the title does not crowd the paragraphs.
func displaySize(tk design.Tokens, w float32) float32 {
	if w <= design.BreakpointSmall {
		return tk.DisplayNarrow
	}
	return tk.TypeDisplay
}

// paperGutter is the room beside the paper's text: the paper's own gutter, the
// narrow one on a small window, where the notes column has already taken its
// share and a 48 DIP margin would leave the paragraph a handful of words. Only
// reading tightens it: the writing mode hangs the block handles in its margin.
func paperGutter(tk design.Tokens, w float32) float32 {
	if w <= design.BreakpointSmall {
		return tk.GutterNarrow
	}
	return tk.Gutter
}
