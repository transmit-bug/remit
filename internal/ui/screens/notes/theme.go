package notes

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// noteTheme is how a note is set on the paper: a body read at arm's length,
// three heading steps the outline mirrors, a mark-coloured quote rule and quiet
// code. The chrome around the paper keeps the window's 13px; only the document
// takes this scale.
func noteTheme(tk design.Tokens) *richtext.Theme {
	th := richtext.DefaultTheme(tk.Theme)
	th.Text = tk.Text
	th.Muted = tk.TextMuted
	th.Link = tk.Accent
	th.Caret = tk.Accent
	th.Selection = tk.Accent.Alpha(0.16)

	th.CodeFont = "monospace"
	th.CodeSize = 0.86
	th.Code = tk.Text
	th.CodeBackground = tk.Sunken
	th.CodeBlockBackground = tk.Sunken
	th.CodeBlockRadius = 0.3

	// The paper sets paragraphs apart instead of indenting their first line.
	th.ParagraphSpacing = 0.95
	th.FirstLineIndent = 0
	th.Indent = 1.6
	th.Marker = tk.TextMuted
	th.Checkbox = tk.Accent
	th.QuoteBar = tk.Accent
	th.QuoteBarWidth = 2
	th.QuoteText = tk.TextMuted

	// The editor sits inside the paper's own gutters, so it brings none.
	th.Padding = [2]float32{0, 0}

	headings := [6]richtext.HeadingStyle{
		{Size: 1.9, Weight: 700, Color: tk.Text, Top: 0, Bottom: 0.4},
		{Size: 1.5, Weight: 700, Color: tk.Text, Top: 1.4, Bottom: 0.3},
		{Size: 1.22, Weight: 600, Color: tk.Text, Top: 1.2, Bottom: 0.25},
		{Size: 1.08, Weight: 600, Color: tk.Text, Top: 1, Bottom: 0.2},
		{Size: 1, Weight: 600, Color: tk.TextMuted, Top: 0.9, Bottom: 0.2},
		{Size: 1, Weight: 600, Color: tk.TextMuted, Top: 0.9, Bottom: 0.2},
	}
	th.Headings = headings
	return th
}

// noteFont is the body the note is written in: the document's size at the
// desktop's text scale and the paper's line height, so a CJK paragraph has the
// leading it needs.
func noteFont(tk design.Tokens) ui.Font {
	return ui.Font{Size: tk.TypeBody, LineHeight: design.TextLineHeight}
}
