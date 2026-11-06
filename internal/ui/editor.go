package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// editorState is the notebook: which note is open, its draft, and the one
// document editor the note is written in. A note is a document whose paragraphs
// are addressable (ADR 0005); the per-block form is gone.
type editorState struct {
	selected string
	draft    core.Note
	loaded   string // the note the draft holds
	title    ui.Handle

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

	edited, retitled := false, false
	ui.Scroll(c).Grow(1).Padding(tk.spaceXL).Children(func() {
		ui.Column(c).Gap(tk.spaceM).Children(func() {
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
				if n := a.data.cardsInNote(*note); n > 0 {
					ui.Badge(c, blockCount(c, n))
				}
				ui.Text(c, m(c).EditorHint()).
					FontSize(tk.typeSmall).TextColor(tk.TextMuted)
			})

			if richtext.Editor(c, a.editor.doc).
				Fill().MinHeight(tk.control * 8).
				Label(m(c).NoteBody()).Changed() {
				edited = true
			}
		})
	})
	switch {
	case edited:
		a.saveDocument(c)
	case retitled:
		a.saveNote(c)
	}
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
	if prompt, _ := core.Split(block.Text); prompt == "" {
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
	if _, err := a.lib.CreateCard(block.ID); err != nil {
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
	text := a.editor.doc.Text().Markdown()
	_, caret := a.editor.doc.Selection()
	start, end := paragraphAt(text, caret)
	id, _, ok := core.SplitBlockMarker(strings.TrimSpace(text[start:end]))
	if !ok || id.Kind() != core.BlockKind {
		return
	}
	for i, block := range a.editor.draft.Blocks {
		if block.ID == id {
			a.makeCard(c, i)
			return
		}
	}
}

// blockCount names how many blocks a note has.
func blockCount(c *ui.Context, n int) string { return m(c).BlockCount(n) }

// cardCount names how many cards point at a block or a note.
func cardCount(c *ui.Context, n int) string { return m(c).CardCount(n) }
