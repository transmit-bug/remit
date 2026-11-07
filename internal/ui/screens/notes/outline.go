package notes

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// outlineEntry is one heading of the note: what it says, where it starts in the
// document the editor holds, how deep it sits, and the heading it is under.
type outlineEntry struct {
	label  string
	offset int
	level  int
	parent int // the entry this one sits under, -1 at the top
}

// outlineEntries reads a note's blocks into its headings, in document order. The
// blocks carry the Markdown source — the editor keeps a heading's hashes as
// style, not text — and the offset is where the heading starts in the document
// the editor holds. A heading is a child of the last heading above it that is
// shallower, which is the tree the panel shows.
func outlineEntries(blocks []core.Block) []outlineEntry {
	var out []outlineEntry
	last := [7]int{-1, -1, -1, -1, -1, -1, -1}
	offset := 0
	for i, block := range blocks {
		if i > 0 {
			offset += 2 // the blank line between blocks
		}
		if level, label := heading(strings.TrimSpace(block.Text)); level > 0 {
			parent := -1
			for l := level - 1; l >= 1; l-- {
				if last[l] >= 0 {
					parent = last[l]
					break
				}
			}
			out = append(out, outlineEntry{label: label, offset: offset, level: level, parent: parent})
			last[level] = len(out) - 1
			for l := level + 1; l < len(last); l++ {
				last[l] = -1
			}
		}
		offset += len(block.Text)
	}
	return out
}

// heading reads a Markdown heading, returning its level and its text.
func heading(body string) (level int, label string) {
	for level < len(body) && body[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(body) || body[level] != ' ' {
		return 0, ""
	}
	// A heading is its first line: what follows a blank line is the paragraph
	// under it, not part of its name.
	label = body[level+1:]
	if end := strings.IndexByte(label, '\n'); end >= 0 {
		label = label[:end]
	}
	return level, strings.TrimSpace(label)
}

// outlineList shows the note's headings as an outline, the one under the caret
// marked; choosing one puts the caret on its paragraph, which is what scrolls it
// into view. A heading is the arrow's to open and close, and the keys are the
// outline's once a row has the focus.
func (s *State) outlineList(c *ui.Context, tk design.Tokens) {
	msgs := locales.In(c.Locale())
	entries := outlineEntries(s.Draft.Blocks)
	if s.Doc == nil || len(entries) == 0 {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Padding(tk.SpaceS, 0).Children(func() {
			tk.Icon(c, "notebook-pen", design.IconSmall).TextColor(tk.TextFaint)
			ui.Text(c, msgs.OutlineEmpty()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		})
		return
	}
	roots := make([]int, 0, len(entries))
	children := make(map[int][]int, len(entries))
	for i, entry := range entries {
		if entry.parent < 0 {
			roots = append(roots, i)
		} else {
			children[entry.parent] = append(children[entry.parent], i)
		}
	}
	s.openOutline(entries)
	// The caret marks the heading it is under; the choice stays the user's, so a
	// click on a row is always a choice and never just a remark about the caret.
	s.OutlineCurrent = headingAt(entries, s.caretOffset())
	s.outline.List.Selected = &s.outlineChosen
	s.outline.List.Label = func(row int) string { return entries[s.outline.Item(row)].label }
	list := ui.Outline(c, &s.outline, roots, func(i int) []int { return children[i] }, func(i int) {
		text := ui.Text(c, entries[i].label).FontSize(tk.TypeSmall).SingleLine()
		if i == s.OutlineCurrent {
			text.TextColor(tk.Accent)
			return
		}
		text.TextColor(tk.Text)
	}).Grow(1).Label(msgs.Outline())
	// A choice jumps to the heading; Enter or a double click on the row chosen
	// jumps again, for a reader who scrolled away from it.
	if list.Changed() || list.Submitted() {
		if i := s.outline.Item(s.outlineChosen); i >= 0 && i < len(entries) {
			s.pendingJump = entries[i].offset
		}
	}
}

// openOutline opens every heading a note's outline lists the first time it is
// built, and again when the note or its shape changes, so a note reads whole;
// the user may close what they like after that.
func (s *State) openOutline(entries []outlineEntry) {
	if s.outlineFor == s.Draft.ID && s.outlineShape == len(entries) {
		return
	}
	if s.outlineFor != s.Draft.ID {
		// Another note's choice means nothing here.
		s.outlineChosen = -1
	}
	s.outline.Open.Clear()
	for i := range entries {
		s.outline.Open.Add(i)
	}
	s.outlineFor, s.outlineShape = s.Draft.ID, len(entries)
}

// headingAt is the heading the offset falls under, -1 before the first one.
func headingAt(entries []outlineEntry, offset int) int {
	at := -1
	for i, entry := range entries {
		if entry.offset > offset {
			break
		}
		at = i
	}
	return at
}

// caretOffset is where the caret sits in the document the editor holds.
func (s *State) caretOffset() int {
	if s.Doc == nil {
		return 0
	}
	_, caret := s.Doc.Selection()
	return caret
}

// jumpInDocument puts the caret on a paragraph of the note open now, which the
// editor scrolls into view.
func (s *State) jumpInDocument(c *ui.Context, offset int) {
	if s.Doc == nil {
		return
	}
	s.Doc.SetSelection(offset, offset)
	s.body.Focus(c)
}
