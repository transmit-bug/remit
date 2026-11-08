package notes

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// editorView draws the selected note: its title, how many cards it feeds, and
// its text as one document — headings, lists, code, images and links included.
func (s *State) editorView(c *ui.Context, tk design.Tokens, d Deps) {
	library := d.Notes()
	s.Follow(library)
	msgs := locales.In(c.Locale())
	note := &s.Draft
	if note.ID == "" {
		kit.Empty(c, "file-text", msgs.NoNotes(), func() {
			ui.Button(c, msgs.NewNote()).MinHeight(tk.Control).OnClick(func() { d.NewNote(c) })
		})
		return
	}
	s.ensureDocument()
	if s.FindOpen {
		s.findBar(c)
	}
	if s.jumpTo > 0 {
		// A backlink names a paragraph of this note: the caret goes there, which
		// scrolls it into view.
		if at := ParagraphOffset(DocumentText(s.Draft), s.jumpTo); at >= 0 {
			s.Doc.SetSelection(at, at)
			s.body.Focus(c)
		}
		s.jumpTo = 0
	}

	edited, retitled := false, false
	// The band under the frame: the format bar, then the paper itself, then the
	// status line. The paper is one centred measure; nothing else is boxed.
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		s.formatBar(c, tk)
		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Justify(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MaxWidth(tk.PaperWidth).
				Padding(tk.SpaceXXL, tk.Gutter).Children(func() {
				title := ui.TextInput(c, &note.Title).FontSize(tk.TypeDisplay).Bold().
					Placeholder(msgs.NoteTitle()).Bind(&s.title).
					Label(msgs.NoteTitle()).
					Background(ui.RGBA(0, 0, 0, 0)).Border(0, tk.Border).
					Padding(tk.SpaceS, 0)
				if s.focus {
					s.title.Focus(c)
					s.focus = false
				}
				if title.Changed() {
					retitled = true
				}

				ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Padding(0, 0, tk.SpaceM, 0).Children(func() {
					ui.Text(c, msgs.EditorHint()).
						FontSize(tk.TypeCaption).TextColor(tk.TextFaint).Grow(1).SingleLine()
					if n := len(outlineEntries(note.Blocks)); n > 0 {
						ui.Text(c, msgs.HeadingCount(n)).
							FontSize(tk.TypeCaption).TextColor(tk.TextFaint)
					}
				})

				// EditorBase, not Editor: the paper is the surface, so the editor
				// brings no box of its own. The wrapping row takes the paste
				// shortcut and dropped files: a picture pasted or dragged in
				// is stored in the library and inserted at the caret (#152).
				editor := ui.Row(c)
				drops := editor.DroppedFiles()
				over := editor.FileDragOver()
				editor.AlignItems(ui.Stretch).Grow(1).MinWidth(0)
				if over {
					editor.Border(1, tk.Accent).Radius(tk.RadiusControl)
				}
				editor.Children(func() {
					if richtext.EditorBase(c, s.Doc, richtext.EditorOptions{
						Label: msgs.NoteBody(),
						Font:  noteFont(tk),
						Theme: noteTheme(tk),
					}).Grow(1).MinHeight(tk.Control * 12).Bind(&s.body).Changed() {
						edited = true
					}
				})
				s.pasteShortcut(editor, c, d)
				if len(drops) > 0 || pasted != nil {
					s.takeImages(c, d, drops)
				}
			})
		})
		s.statusLine(c, tk, cardCount(s.Draft, library))
	})
	switch {
	case edited:
		s.saveDocument(c, d)
	case retitled:
		if saved, ok := d.SaveNote(c, s.Draft); ok {
			s.Keep(saved)
		}
	}
}

// cardCount is how many cards a note feeds, as the app counted them for the
// list.
func cardCount(note core.Note, library []Note) int {
	for _, row := range library {
		if row.ID == note.ID {
			return row.Cards
		}
	}
	return 0
}

// statusLine is the paper's quiet foot: what the note holds. 11px, pencil, on
// the same line as the writing, never a toolbar.
func (s *State) statusLine(c *ui.Context, tk design.Tokens, cards int) {
	msgs := locales.In(c.Locale())
	text := s.Doc.Text().String()
	words := len(strings.Fields(text))
	paragraphs := s.Doc.Text().ParagraphCount()
	ui.Row(c).Justify(ui.Center).Children(func() {
		ui.Row(c).Grow(1).MaxWidth(tk.PaperWidth).AlignItems(ui.Center).Gap(tk.SpaceS).
			Padding(tk.SpaceS, tk.Gutter).Children(func() {
			ui.Text(c, msgs.WordCount(words)).
				FontSize(tk.TypeCaption).TextColor(tk.TextMuted)
			ui.Text(c, "·").FontSize(tk.TypeCaption).TextColor(tk.TextFaint)
			ui.Text(c, msgs.ParagraphCount(paragraphs)).
				FontSize(tk.TypeCaption).TextColor(tk.TextMuted)
			if cards > 0 {
				ui.Text(c, "·").FontSize(tk.TypeCaption).TextColor(tk.TextFaint)
				ui.Text(c, msgs.CardCount(cards)).
					FontSize(tk.TypeCaption).TextColor(tk.TextMuted)
			}
			ui.Spacer(c)
			ui.Text(c, msgs.Saved()).
				FontSize(tk.TypeCaption).TextColor(tk.TextFaint)
		})
	})
}

// findBar shows the in-document find bar: it counts the query in the note open
// now, and choosing a match puts the selection on it, which scrolls it into view.
// It never looks at another note.
func (s *State) findBar(c *ui.Context) {
	matches, spans := findMatches(s.Doc.Text().String(), s.findQuery)
	if s.findCurrent >= matches {
		s.findCurrent = 0
	}
	before := s.findCurrent
	ui.FindBar(c, &s.FindOpen, &s.findQuery, matches, &s.findCurrent)
	if (s.findCurrent != before || s.findQuery != s.findJumped) && matches > 0 {
		// The bar keeps the focus: Escape and Enter are its keys while it is
		// open, and the selection is enough to scroll the match into view.
		span := spans[s.findCurrent]
		s.Doc.SetSelection(span[0], span[1])
		s.findJumped = s.findQuery
	}
}

// findMatches is every non-overlapping, case-insensitive match of query in text,
// with where each one is.
func findMatches(text, query string) (int, [][2]int) {
	if query == "" {
		return 0, nil
	}
	lower, needle := strings.ToLower(text), strings.ToLower(query)
	var spans [][2]int
	for from := 0; ; {
		at := strings.Index(lower[from:], needle)
		if at < 0 {
			break
		}
		start := from + at
		spans = append(spans, [2]int{start, start + len(needle)})
		from = start + len(needle)
	}
	return len(spans), spans
}

// secondPane is the right-hand side of a split: the note as it reads, in its own
// scroll, so that two places can be compared while the left side is edited.
//
// It is a reading view because v0.8.1's richtext editor keeps one view per
// document: two editors over the same Document do not both report their edits
// (checked: richtext.Editor, EditorBase, Document.SetText, Document.Selection —
// the view state that notices a change is per editor, and the second one never
// sees the first's). Editing the right side needs a second document and a merge,
// which is a design of its own.
func (s *State) secondPane(c *ui.Context, tk design.Tokens, d Deps) {
	s.Follow(d.Notes())
	if s.Draft.ID == "" {
		return
	}
	ui.Scroll(c).Grow(1).MinWidth(0).Padding(tk.SpaceXL).Children(func() {
		markdown.View(c, DocumentText(s.Draft), s.imageOptions())
	})
}

// readView is the note as it reads, whole: its title, then the document with its
// headings, lists and links, and no control over it.
func (s *State) readView(c *ui.Context, tk design.Tokens, d Deps) {
	msgs := locales.In(c.Locale())
	s.Follow(d.Notes())
	if s.Draft.ID == "" {
		kit.Empty(c, "file-text", msgs.NoNotes(), func() {
			ui.Button(c, msgs.NewNote()).MinHeight(tk.Control).OnClick(func() { d.NewNote(c) })
		})
		return
	}
	ui.Scroll(c).Grow(1).MinWidth(0).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).MaxWidth(readWidth).Gap(tk.SpaceM).Children(func() {
			ui.Text(c, s.Draft.Title).FontSize(tk.TypeTitle).Bold()
			markdown.View(c, DocumentText(s.Draft), s.imageOptions())
		})
	})
}

// readWidth caps the measure of a note read whole, as the stats page caps its
// own: a paragraph wider than this is hard to follow.
const readWidth float32 = 760

// imageOptions is the reading views' Markdown options: the pictures the
// library stores are loaded from it, by the relative reference a note
// carries. A reference that is not the library's own assets form — a web
// URL, a path outside the library — shows as its alt text, never fetched:
// the library reads its own files and nothing else.
func (s *State) imageOptions() *markdown.Options {
	var read func(string) ([]byte, bool)
	if s.deps != nil && s.deps.GetAsset != nil {
		read = s.deps.GetAsset
	}
	return &markdown.Options{LoadImage: func(url string) (ui.ImageSource, error) {
		rel, ok := assetRefOf(url)
		if !ok {
			return nil, fmt.Errorf("not a library asset: %s", url)
		}
		if read == nil {
			return nil, fs.ErrNotExist
		}
		data, ok := read(rel)
		if !ok {
			return nil, fs.ErrNotExist
		}
		return decodeAssetImage(rel, data)
	}}
}

// assetRefOf says whether an image destination is the library's own assets
// form and answers the path inside the library.
func assetRefOf(url string) (string, bool) {
	if url == "" || strings.Contains(url, "..") || strings.HasPrefix(url, "assets/.") {
		return "", false
	}
	if !strings.HasPrefix(url, "assets/") {
		return "", false
	}
	return url, true
}

// formatBar is the row of formatting controls above the document: what the
// selection holds, and the one click that changes it. The document is the
// richtext editor's; the controls are MyGo's, so the keys, the focus and the
// names assistive technology reads come with them.
func (s *State) formatBar(c *ui.Context, tk design.Tokens) {
	doc := s.Doc
	if doc == nil {
		return
	}
	msgs := locales.In(c.Locale())
	f := doc.Formatting()
	style := msgs.StyleBody()
	switch f.Paragraph.Heading {
	case 1:
		style = msgs.StyleTitle()
	case 2:
		style = msgs.StyleHeading()
	case 3:
		style = msgs.StyleSubheading()
	}
	if f.ParagraphsMixed {
		style = ""
	}

	ui.Toolbar(c, func() {
		ui.MenuButton(c, style, func(menu *ui.Menu) {
			for _, choice := range []struct {
				name  string
				level int
			}{
				{msgs.StyleBody(), 0},
				{msgs.StyleTitle(), 1},
				{msgs.StyleHeading(), 2},
				{msgs.StyleSubheading(), 3},
			} {
				if menu.Item(choice.name).Checked(f.Paragraph.Heading == choice.level).Chosen() {
					doc.SetHeading(choice.level)
				}
			}
		}).Label(msgs.ParagraphStyle())
		formatRule(c, tk)
		formatToggle(c, tk, "bold", msgs.FmtBold(), f.Bold == richtext.FormatOn, doc.ToggleBold)
		formatToggle(c, tk, "italic", msgs.FmtItalic(), f.Italic == richtext.FormatOn, doc.ToggleItalic)
		formatToggle(c, tk, "underline", msgs.FmtUnderline(), f.Underline == richtext.FormatOn, doc.ToggleUnderline)
		formatToggle(c, tk, "strikethrough", msgs.FmtStrikethrough(), f.Strikethrough == richtext.FormatOn, doc.ToggleStrikethrough)
		formatToggle(c, tk, "code", msgs.FmtCode(), f.Code == richtext.FormatOn, doc.ToggleCode)
		formatRule(c, tk)
		formatToggle(c, tk, "list", msgs.FmtBullets(), f.Paragraph.List == richtext.Bullets, func() { doc.ToggleList(richtext.Bullets) })
		formatToggle(c, tk, "list-ordered", msgs.FmtNumbers(), f.Paragraph.List == richtext.Numbers, func() { doc.ToggleList(richtext.Numbers) })
		formatToggle(c, tk, "square-check", msgs.FmtTasks(), f.Paragraph.List == richtext.Tasks, func() { doc.ToggleList(richtext.Tasks) })
		formatToggle(c, tk, "quote", msgs.FmtQuote(), f.Paragraph.Quote, doc.ToggleQuote)
		formatToggle(c, tk, "braces", msgs.FmtCodeBlock(), f.Paragraph.CodeBlock, doc.ToggleCodeBlock)
		formatRule(c, tk)
		link := ui.Button(c, "").Label(msgs.Link()).Tooltip(msgs.Link()).KeepFocus().
			MinWidth(tk.Control + 2).MinHeight(tk.Control + 2).Justify(ui.Center).
			Children(func() { tk.Icon(c, "link-2", design.IconSmall) })
		if f.Link != "" {
			link.Background(tk.Surface.Mix(tk.Accent, 0.16)).TextColor(tk.Accent)
		}
		if link.Clicked() {
			s.linkURL, s.linkOpen = f.Link, true
		}
		ui.Popover(c, link, &s.linkOpen, func() {
			ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Padding(tk.SpaceS).Children(func() {
				field := ui.TextInput(c, &s.linkURL).
					Placeholder(msgs.LinkURL()).Width(220).AutoFocus().Label(msgs.LinkURL())
				set := func() {
					doc.SetLink(strings.TrimSpace(s.linkURL))
					s.linkOpen = false
				}
				if field.Submitted() {
					set()
				}
				if ui.PrimaryButton(c, msgs.Link()).Clicked() {
					set()
				}
				if f.Link != "" && ui.Button(c, msgs.LinkRemove()).Clicked() {
					doc.SetLink("")
					s.linkOpen = false
				}
			})
		})
	}).Label(msgs.FormatBar()).
		MinHeight(tk.BarHeight).
		Padding(tk.SpaceXS, tk.SpaceL).
		Background(tk.Surface).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border)
}

// formatRule is the hairline between two groups of the format bar: a line, not
// a gap, so the groups read as groups.
func formatRule(c *ui.Context, tk design.Tokens) {
	ui.Divider(c).Height(tk.TypeBody).AlignSelf(ui.Center)
}

// formatToggle is one control of the format bar: the glyph that names the
// format, a label for assistive technology and a tooltip, and the face it wears
// while the selection holds the format. The focus stays on the bar, so a second
// click applies a second format.
func formatToggle(c *ui.Context, tk design.Tokens, icon, label string, on bool, run func()) ui.Element {
	button := ui.Button(c, "").Label(label).Tooltip(label).KeepFocus().
		Children(func() { tk.Icon(c, icon, design.IconSmall) }).
		MinWidth(tk.Control + 2).MinHeight(tk.Control + 2).Justify(ui.Center).
		Radius(tk.RadiusControl).TextColor(tk.TextMuted)
	if on {
		button.Background(tk.Surface.Mix(tk.Accent, 0.14)).TextColor(tk.Accent)
	}
	if button.Clicked() {
		run()
	}
	return button
}

// saveDocument writes what the editor holds: its paragraphs back into blocks,
// the markers keeping every identity the document still carries. The app stores
// the note and reconciles the cards the new shape asks for.
func (s *State) saveDocument(c *ui.Context, d Deps) {
	live := s.Doc.Text().Markdown()
	if live == s.docText {
		return
	}
	s.Draft.Blocks = blocksOfDocument(live, s.Draft.Blocks)
	s.docText = live
	if saved, ok := d.SaveDocument(c, s.Draft); ok {
		s.Keep(saved)
	}
}
