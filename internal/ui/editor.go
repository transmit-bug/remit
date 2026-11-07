package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// editorState is the notebook: which note is open, its draft, and the one
// document editor the note is written in. A note is a document whose paragraphs
// are addressable (ADR 0005); the per-block form is gone.
type editorState struct {
	selected    string
	draft       core.Note
	loaded      string // the note the draft holds
	title       ui.Handle
	body        ui.Handle
	findOpen    bool   // the in-document find bar is up
	findQuery   string // what it looks for
	findCurrent int    // which match is shown
	findJumped  string // the query the selection was last put on
	jumpTo      int    // a paragraph of the note to put the caret in, 1-based; 0 for none

	focus   bool               // the title takes the caret in the next pass
	doc     *richtext.Document // the note as one document, markers and all
	docNote core.ID            // the note the document holds
	docText string             // the text the document was last built from
}

// follow points the draft at the note the list chose, before that note is
// built, so a choice made in this frame shows in it.
func (e *editorState) follow(d libraryData) {
	if e.loaded == e.selected && e.draft.ID != "" {
		return
	}
	e.draft = core.Note{}
	for _, note := range d.notes {
		if string(note.ID) == e.selected {
			e.hold(note)
			return
		}
	}
	// Nothing chosen, or the choice is gone: the first note stands in.
	if len(d.notes) == 0 {
		e.loaded, e.selected = "", ""
		e.draft = core.Note{}
		return
	}
	e.hold(d.notes[0])
}

// hold takes a copy of a note as the draft, blocks and all, so that editing
// never reaches into the snapshot the next reload replaces.
func (e *editorState) hold(note core.Note) {
	note.Blocks = append([]core.Block(nil), note.Blocks...)
	e.draft = note
	e.loaded = string(note.ID)
	e.selected = string(note.ID)
	e.doc, e.docNote, e.docText = nil, "", ""
}

// ensureIDs mints an ID for a block the library has never seen. A block read
// from a hand-written note arrives without one; minting it here keeps the
// draft's blocks and the saved ones in the same order, and gives a card a
// source to point at.
func (e *editorState) ensureIDs() {
	for i := range e.draft.Blocks {
		if e.draft.Blocks[i].ID == "" {
			e.draft.Blocks[i].ID = core.NewID(core.BlockKind)
		}
	}
}

// ensureDocument gives the note its editor document: built when the note
// changes, kept otherwise, so typing never loses its caret to a rebuild.
func (e *editorState) ensureDocument() {
	if e.doc != nil && e.docNote == e.draft.ID {
		return
	}
	text := documentText(e.draft)
	e.doc = richtext.NewDocument(richtext.ParseMarkdown(text))
	e.docNote, e.docText = e.draft.ID, text
}

// editorView draws the selected note: its title, how many cards it feeds, and
// its text as one document — headings, lists, code, images and links included.
func (a *App) editorView(c *ui.Context, tk tokens) {
	a.editor.follow(a.data)
	note := &a.editor.draft
	if note.ID == "" {
		kit.Empty(c, "file-text", m(c).NoNotes(), func() {
			ui.Button(c, m(c).NewNote()).MinHeight(tk.control).OnClick(func() { a.newNote(c) })
		})
		return
	}
	a.editor.ensureDocument()
	if a.editor.findOpen {
		a.findBar(c)
	}
	if a.editor.jumpTo > 0 {
		// A backlink names a paragraph of this note: the caret goes there, which
		// scrolls it into view.
		if at := paragraphOffset(documentText(a.editor.draft), a.editor.jumpTo); at >= 0 {
			a.editor.doc.SetSelection(at, at)
			a.editor.body.Focus(c)
		}
		a.editor.jumpTo = 0
	}

	edited, retitled := false, false
	ui.Column(c).Grow(1).MinWidth(0).Padding(tk.spaceXL).Gap(tk.spaceM).Children(func() {
		title := ui.TextInput(c, &note.Title).FontSize(tk.typeTitle).Bold().
			Placeholder(m(c).NoteTitle()).Bind(&a.editor.title).
			Label(m(c).NoteTitle())
		if a.editor.focus {
			a.editor.title.Focus(c)
			a.editor.focus = false
		}
		if title.Changed() {
			retitled = true
		}

		ui.Row(c).AlignItems(ui.Center).Gap(tk.spaceS).Children(func() {
			ui.Text(c, m(c).EditorHint()).
				FontSize(tk.typeSmall).TextColor(tk.TextMuted).Grow(1)
			ui.Button(c, "").Label(m(c).PanelTitle()).Tooltip(m(c).PanelTitle()).
				Children(func() { tk.icon(c, "sparkles", iconSmall) }).
				OnClick(a.togglePanel)
		})

		if richtext.Editor(c, a.editor.doc).
			Grow(1).MinHeight(tk.control * 8).
			Label(m(c).NoteBody()).Bind(&a.editor.body).Changed() {
			edited = true
		}
	})
	switch {
	case edited:
		a.saveDocument(c)
	case retitled:
		a.saveNote(c)
	}
}

// findBar shows the in-document find bar: it counts the query in the note open
// now, and choosing a match puts the selection on it, which scrolls it into view.
// It never looks at another note.
func (a *App) findBar(c *ui.Context) {
	matches, spans := findMatches(a.editor.doc.Text().String(), a.editor.findQuery)
	if a.editor.findCurrent >= matches {
		a.editor.findCurrent = 0
	}
	before := a.editor.findCurrent
	ui.FindBar(c, &a.editor.findOpen, &a.editor.findQuery, matches, &a.editor.findCurrent)
	if (a.editor.findCurrent != before || a.editor.findQuery != a.editor.findJumped) && matches > 0 {
		// The bar keeps the focus: Escape and Enter are its keys while it is
		// open, and the selection is enough to scroll the match into view.
		span := spans[a.editor.findCurrent]
		a.editor.doc.SetSelection(span[0], span[1])
		a.editor.findJumped = a.editor.findQuery
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
func (a *App) secondPane(c *ui.Context, tk tokens) {
	a.editor.follow(a.data)
	if a.editor.draft.ID == "" {
		return
	}
	ui.Scroll(c).Grow(1).Padding(tk.spaceXL).Children(func() {
		markdown.View(c, documentText(a.editor.draft), nil)
	})
}

// saveDocument writes what the editor holds: its paragraphs back into blocks,
// the markers keeping every identity the document still carries.
func (a *App) saveDocument(c *ui.Context) {
	live := a.editor.doc.Text().Markdown()
	if live == a.editor.docText {
		return
	}
	a.editor.draft.Blocks = blocksOfDocument(live, a.editor.draft.Blocks)
	a.editor.docText = live
	a.saveNote(c)
	// Typing :: is what makes a paragraph a card, and deleting it is what takes
	// the card out of the queue.
	a.reconcileCards(c)
}

// saveNote writes the draft and reloads what the window reads.
func (a *App) saveNote(c *ui.Context) bool {
	if !a.writeNote(c) {
		return false
	}
	a.reload()
	return true
}

// writeNote saves the draft without reloading, for an action that writes more
// than the note and reloads once at the end.
func (a *App) writeNote(c *ui.Context) bool {
	a.editor.ensureIDs()
	saved, err := a.lib.SaveNote(a.editor.draft)
	if err != nil {
		a.report(c, "save note", err)
		return false
	}
	a.editor.draft = saved
	return true
}

// makeCard makes a card from one of the note's paragraphs.
func (a *App) makeCard(c *ui.Context, i int) {
	if i >= len(a.editor.draft.Blocks) {
		return
	}
	block := a.editor.draft.Blocks[i]
	derived, ok := a.deriveCards(block.Text)
	if !ok || len(derived) == 0 || derived[0].Prompt == "" {
		c.Toast(m(c).BlockEmpty())
		return
	}
	if block.ID == "" {
		// A block read from a hand-written note has no ID until the note is
		// saved; saving now mints one that the card can point at.
		if !a.saveNote(c) {
			return
		}
		block = a.editor.draft.Blocks[i]
	}
	if _, err := a.lib.CreateCards(block.ID, derived); err != nil {
		a.report(c, "make card", err)
		return
	}
	// A card made now is due now, so reloading puts it at the end of the queue
	// and the card the user is on does not move.
	a.reload()
	c.Toast(m(c).CardMade(i + 1))
}

// makeCardAtCaret makes a card from the paragraph the caret is in, which is what
// the palette's "Make card" does while card creation is a command rather than a
// gesture.
func (a *App) makeCardAtCaret(c *ui.Context) {
	a.editor.follow(a.data)
	if a.editor.doc == nil || a.editor.draft.ID == "" {
		return
	}
	text := a.editor.doc.Text().String()
	_, caret := a.editor.doc.Selection()
	start, _ := paragraphAt(text, caret)
	// The paragraphs are the blocks, in order: the caret's place among the
	// blank lines is the block it is in.
	at := strings.Count(text[:start], "\n\n")
	if at >= 0 && at < len(a.editor.draft.Blocks) {
		a.makeCard(c, at)
	}
}

// blockCount names how many blocks a note has.
func blockCount(c *ui.Context, n int) string { return m(c).BlockCount(n) }

// cardCount names how many cards point at a block or a note.
func cardCount(c *ui.Context, n int) string { return m(c).CardCount(n) }
