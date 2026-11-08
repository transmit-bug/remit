package notes

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

type slashKind uint8

const (
	slashStyle slashKind = iota
	slashInsert
	slashImage
	slashTable
)

// slashCommand is one way a slash menu can change the block it was typed in.
// A style changes the paragraph the command leaves behind; insert puts text
// at the command's place; image asks for a picture file.
type slashCommand struct {
	key      string
	label    string
	keywords string
	kind     slashKind
	style    richtext.ParagraphStyle
	insert   string
	caret    int
}

// slashCommands is the menu's whole set, in the order it lists them. Table is
// added by the table slice, after the document can preserve one.
func slashCommands(msgs locales.Messages, d Deps) []slashCommand {
	commands := []slashCommand{
		{key: "body", label: msgs.StyleBody(), keywords: "text body paragraph p", kind: slashStyle},
		{key: "title", label: msgs.StyleTitle(), keywords: "h1 heading 1 title", kind: slashStyle, style: richtext.ParagraphStyle{Heading: 1}},
		{key: "heading", label: msgs.StyleHeading(), keywords: "h2 heading 2", kind: slashStyle, style: richtext.ParagraphStyle{Heading: 2}},
		{key: "subheading", label: msgs.StyleSubheading(), keywords: "h3 heading 3", kind: slashStyle, style: richtext.ParagraphStyle{Heading: 3}},
		{key: "bullets", label: msgs.FmtBullets(), keywords: "bullet list unordered ul", kind: slashStyle, style: richtext.ParagraphStyle{List: richtext.Bullets}},
		{key: "numbers", label: msgs.FmtNumbers(), keywords: "number ordered list ol", kind: slashStyle, style: richtext.ParagraphStyle{List: richtext.Numbers}},
		{key: "quote", label: msgs.FmtQuote(), keywords: "quote blockquote", kind: slashStyle, style: richtext.ParagraphStyle{Quote: true}},
		{key: "code", label: msgs.FmtCodeBlock(), keywords: "code block fence", kind: slashStyle, style: richtext.ParagraphStyle{CodeBlock: true}},
		{key: "formula", label: msgs.SlashFormula(), keywords: "math formula equation display tex", kind: slashInsert, insert: "$$ $$", caret: 3},
		{key: "inline-formula", label: msgs.SlashInlineFormula(), keywords: "math formula equation inline tex", kind: slashInsert, insert: "$ $", caret: 2},
		{key: "table", label: msgs.SlashTable(), keywords: "table grid rows columns cells", kind: slashTable},
	}
	if d.ChooseImage != nil {
		commands = append(commands, slashCommand{
			key: "image", label: msgs.SlashImage(), keywords: "image picture photo asset file", kind: slashImage,
		})
	}
	return commands
}

// resetSlash closes the insertion menu and forgets the query it was opened
// for. A note change starts with no menu.
func (s *State) resetSlash() {
	s.slashOpen, s.slashSeen, s.slashTake = false, false, false
	s.slashQuery = ""
	s.slashStart, s.slashEnd = 0, 0
	s.slashChoice = -1
	s.slashMatches = nil
}

// syncSlash reads the paragraph at the caret as a slash query. A slash at the
// start of a paragraph opens the menu once; Escape closes it without deleting
// what was typed, and the same query stays closed until the slash is removed.
func (s *State) syncSlash(msgs locales.Messages, d Deps) {
	start, end, query, ok := s.slashAtCaret()
	if !ok {
		s.slashOpen, s.slashSeen, s.slashTake = false, false, false
		s.slashQuery, s.slashMatches = "", nil
		s.slashChoice = -1
		return
	}
	if query != s.slashQuery {
		s.slashQuery = query
		s.slashChoice = -1
	}
	s.slashStart, s.slashEnd = start, end
	s.slashMatches = filterSlash(slashCommands(msgs, d), query)
	if len(s.slashMatches) == 0 {
		s.slashChoice = -1
	} else if s.slashChoice < 0 || s.slashChoice >= len(s.slashMatches) {
		s.slashChoice = 0
	}
	if !s.slashOpen && !s.slashSeen {
		s.slashOpen, s.slashSeen = true, true
	}
}

// slashAtCaret returns the slash query the caret sits in: a slash at the
// start of its paragraph and the text between it and the caret.
func (s *State) slashAtCaret() (start, end int, query string, ok bool) {
	if s.Doc == nil {
		return 0, 0, "", false
	}
	anchor, caret := s.Doc.Selection()
	if anchor != caret {
		return 0, 0, "", false
	}
	text := s.Doc.Text()
	start, end = text.ParagraphRange(caret)
	runes := []rune(text.String())
	if start < 0 || end > len(runes) || caret <= start || caret > end {
		return 0, 0, "", false
	}
	before := runes[start:caret]
	if len(before) == 0 || before[0] != '/' {
		return 0, 0, "", false
	}
	if strings.TrimSpace(string(runes[caret:end])) != "" {
		return 0, 0, "", false
	}
	return start, caret, strings.TrimSpace(string(before[1:])), true
}

// filterSlash keeps the commands whose label or search words hold query. An
// empty query keeps every command.
func filterSlash(commands []slashCommand, query string) []slashCommand {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return commands
	}
	out := make([]slashCommand, 0, len(commands))
	for _, command := range commands {
		if strings.Contains(strings.ToLower(command.label+" "+command.keywords), query) {
			out = append(out, command)
		}
	}
	return out
}

// slashShortcuts takes the keys the menu owns while it is open. The handlers
// sit around the editor, before the focused rich-text view, so Enter chooses
// instead of making a newline and arrows move the choice instead of the caret.
func (s *State) slashShortcuts(c *ui.Context, editor ui.Element) {
	if !s.slashOpen {
		return
	}
	editor.OnShortcut(0, ui.KeyEnter, func() {
		if len(s.slashMatches) == 0 {
			return
		}
		s.slashTake = true
		c.Invalidate()
	})
	editor.OnShortcut(0, ui.KeyUp, func() {
		s.slashStep(-1)
		c.Invalidate()
	})
	editor.OnShortcut(0, ui.KeyDown, func() {
		s.slashStep(1)
		c.Invalidate()
	})
	editor.OnShortcut(0, ui.KeyEscape, func() {
		s.slashOpen = false
		s.slashTake = false
		c.Invalidate()
	})
}

// slashStep moves the menu's highlight, wrapping at either end.
func (s *State) slashStep(delta int) {
	if len(s.slashMatches) == 0 {
		return
	}
	s.slashChoice = (s.slashChoice + delta + len(s.slashMatches)) % len(s.slashMatches)
}

// slashMenu draws the insertion menu under the paragraph that holds the slash.
// The rows are ordinary buttons, so a click and Enter take the same path.
func (s *State) slashMenu(c *ui.Context, tk design.Tokens, msgs locales.Messages, boxes []richtext.ParagraphBox) {
	if !s.slashOpen {
		return
	}
	top := float32(0)
	for _, box := range boxes {
		if box.Start <= s.slashStart {
			top = box.Top
			continue
		}
		break
	}
	anchor := ui.Box(c.Key("slash-anchor")).Absolute().Left(0).Top(top).Width(1).Height(1)
	ui.PopoverBase(c, anchor, &s.slashOpen, func(panel ui.Element) {
		panel.Width(280).Padding(tk.SpaceXS).Gap(2).Radius(tk.RadiusControl).
			Background(tk.Surface).Border(1, tk.Border).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
		panel.Children(func() {
			if len(s.slashMatches) == 0 {
				ui.Text(c, msgs.NoSlashMatches()).FontSize(tk.TypeSmall).
					TextColor(tk.TextMuted).Padding(tk.SpaceS)
				return
			}
			ui.Scroll(c).MaxHeight(300).Children(func() {
				ui.Column(c).Role(ui.RoleListBox).Gap(1).Children(func() {
					for i, command := range s.slashMatches {
						chosen := i == s.slashChoice
						colour := tk.Text
						if chosen {
							colour = tk.Accent
						}
						row := ui.ButtonBase(c.Key(fmt.Sprintf("slash-%s", command.key))).
							Label(command.label).Justify(ui.Start).FillWidth().
							Padding(tk.SpaceXS, tk.SpaceS).Radius(tk.RadiusControl).
							TextColor(colour).Role(ui.RoleOption)
						if chosen {
							row.Background(tk.SurfaceHover)
						}
						row.Children(func() {
							ui.Text(c, command.label).FontSize(tk.TypeBody).
								TextColor(colour)
						})
						if row.Hovered() {
							s.slashChoice = i
							row.Background(tk.SurfaceHover)
						}
						if row.Clicked() {
							s.slashChoice = i
							s.slashTake = true
						}
					}
				})
			})
		})
	})
}

// applySlash replaces the query with the chosen command. It reports whether
// the caller still has to save the document; an image picker saves what it
// inserted itself.
func (s *State) applySlash(c *ui.Context, d Deps) bool {
	if !s.slashTake {
		return false
	}
	s.slashTake = false
	if !s.slashOpen || s.slashChoice < 0 || s.slashChoice >= len(s.slashMatches) || s.Doc == nil {
		return false
	}
	command := s.slashMatches[s.slashChoice]
	start, end := s.slashStart, s.slashEnd
	s.slashOpen = false
	s.slashSeen = false

	if command.kind == slashImage {
		s.replaceSlash(c, start, end, "", 0, slashCommand{})
		path, err := d.ChooseImage()
		if err != nil {
			c.Toast(locales.In(c.Locale()).ChooseImageFailed(err.Error()))
			s.saveDocument(c, d)
			return false
		}
		if path != "" {
			if s.insertImageFile(c, d, path) {
				s.saveDocument(c, d)
			}
			return false
		}
		s.saveDocument(c, d)
		return false
	}
	if command.kind == slashTable {
		s.insertTable(c, d, start, end)
		return false
	}

	s.replaceSlash(c, start, end, command.insert, command.caret, command)
	s.body.Focus(c)
	return true
}

// insertTable puts a small GFM table where the command stood. The marker is
// only a bridge through the editor's Markdown; the draft gets the table source
// and the document is rebuilt with its read-only placeholder.
func (s *State) insertTable(c *ui.Context, d Deps, start, end int) {
	marker := "REMITTABLE" + strings.ReplaceAll(string(core.NewID(core.BlockKind)), "_", "")
	s.replaceSlash(c, start, end, marker, 0, slashCommand{})
	blocks := s.blocksFromLive(s.Doc.Text().Markdown())
	table := tableSkeleton(3, 2)
	at := -1
	for i := range blocks {
		if blocks[i].Text == marker {
			blocks[i].Text = table
			at = i
			break
		}
	}
	if at < 0 {
		s.saveDocument(c, d)
		return
	}
	s.Draft.Blocks = blocks
	s.rebuildDocument(c, at)
	if saved, ok := d.SaveDocument(c, s.Draft); ok {
		s.Keep(saved)
	}
}

// tableSkeleton is a new table with a header and rows data rows. Its cells are
// empty: the table's text is edited in the Markdown source until the table
// block model lands.
func tableSkeleton(cols, rows int) string {
	cols = max(1, cols)
	rows = max(0, rows)
	var b strings.Builder
	for i := 0; i < cols; i++ {
		b.WriteString("|   ")
	}
	b.WriteString("|\n")
	for i := 0; i < cols; i++ {
		b.WriteString("| --- ")
	}
	b.WriteString("|\n")
	for row := 0; row < rows; row++ {
		for i := 0; i < cols; i++ {
			b.WriteString("|     ")
		}
		b.WriteString("|\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// replaceSlash removes the query, inserts the command's text and applies the
// paragraph style it asks for. The command's caret is relative to the inserted
// text, which leaves formulas ready to type between their dollars.
func (s *State) replaceSlash(c *ui.Context, start, end int, insert string, caret int, command slashCommand) {
	attrs := s.Doc.Text().AttributesAt(start)
	s.Doc.Edit(func(e *richtext.Edit) {
		e.ReplaceString(start, end, insert, attrs)
		if command.kind == slashStyle {
			e.FormatParagraphs(start, start, func(p *richtext.ParagraphStyle) {
				id := p.ID
				*p = command.style
				p.ID = id
			})
		}
		at := start + caret
		e.Select(at, at)
	})
	s.body.Focus(c)
}
