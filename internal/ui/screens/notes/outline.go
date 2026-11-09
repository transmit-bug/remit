package notes

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// outlineEntry is one heading of the note: what it says, the paragraph it is
// (counted from one, as the reader counts), how deep it sits, and the heading it
// is under.
type outlineEntry struct {
	label     string
	paragraph int
	level     int
	parent    int // the entry this one sits under, -1 at the top
}

// outlineEntries reads a note's blocks into its headings, in document order. The
// blocks carry the Markdown source — the editor keeps a heading's hashes as
// style, not text — and each entry names the paragraph it is, which is what the
// editor is asked to show. A heading is a child of the last heading above it
// that is shallower, which is the tree the panel shows.
func outlineEntries(blocks []core.Block) []outlineEntry {
	var out []outlineEntry
	last := [7]int{-1, -1, -1, -1, -1, -1, -1}
	for i, block := range blocks {
		if level, label := heading(strings.TrimSpace(block.Text)); level > 0 {
			parent := -1
			for l := level - 1; l >= 1; l-- {
				if last[l] >= 0 {
					parent = last[l]
					break
				}
			}
			out = append(out, outlineEntry{label: label, paragraph: i + 1, level: level, parent: parent})
			last[level] = len(out) - 1
			for l := level + 1; l < len(last); l++ {
				last[l] = -1
			}
		}
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
// outline's once a row has the focus. The rows arrive as a list, once, when the
// panel shows.
func (s *State) outlineList(c *ui.Context, tk design.Tokens, holder ui.Element) {
	msgs := locales.In(c.Locale())
	entries := outlineEntries(s.Draft.Blocks)
	if s.Doc == nil || len(entries) == 0 {
		ui.Row(c).AlignItems(ui.Start).Gap(tk.SpaceS).Padding(tk.SpaceS, 0).Children(func() {
			tk.Icon(c, "notebook-pen", design.IconSmall).TextColor(tk.TextFaint)
			ui.Column(c).Grow(1).MinWidth(0).Gap(tk.SpaceXS).Children(func() {
				ui.Text(c, msgs.OutlineEmpty()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
				ui.Text(c, msgs.OutlineEmptyHint()).FontSize(tk.TypeCaption).TextColor(tk.TextFaint)
			})
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
	s.OutlineCurrent = headingAt(entries, s.caretParagraph())
	s.outline.List.Selected = &s.outlineChosen
	s.outline.List.Label = func(row int) string { return entries[s.outline.Item(row)].label }
	arrive := kit.Stagger(holder, "outline")
	list := ui.Outline(c, &s.outline, roots, func(i int) []int { return children[i] }, func(i int) {
		text := kit.Arrive(ui.Text(c, entries[i].label).FontSize(tk.TypeUI).SingleLine(), arrive(i))
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
			s.pendingJump = entries[i].paragraph
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

// headingAt is the heading the paragraph is under, -1 before the first one.
func headingAt(entries []outlineEntry, paragraph int) int {
	at := -1
	for i, entry := range entries {
		if entry.paragraph > paragraph {
			break
		}
		at = i
	}
	return at
}

// caretParagraph is the paragraph the caret sits in, counted from one as the
// reader counts; zero before the editor has one.
func (s *State) caretParagraph() int {
	at, ok := s.ParagraphAtCaret()
	if !ok {
		return 0
	}
	return at + 1
}

// jumpInDocument puts the caret on a paragraph of the note open now, which the
// editor scrolls into view.
func (s *State) jumpInDocument(c *ui.Context, paragraph int) {
	if s.Doc == nil {
		return
	}
	at := s.OffsetOfParagraph(paragraph)
	if at < 0 {
		return
	}
	s.Doc.SetSelection(at, at)
	s.body.Focus(c)
}

// OffsetOfParagraph is where a paragraph starts in the editor's own text,
// counted from one as the reader counts; -1 when the note has fewer paragraphs.
func (s *State) OffsetOfParagraph(paragraph int) int {
	if s.Doc == nil || paragraph < 1 {
		return -1
	}
	at, index := 0, 1
	for _, held := range s.Doc.Text().Paragraphs() {
		if index == paragraph {
			return at
		}
		length := 0
		for _, run := range held.Runs {
			length += len([]rune(run.Text))
		}
		at += length + 1
		index++
	}
	return -1
}
