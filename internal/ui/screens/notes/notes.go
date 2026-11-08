// Package notes is the notebook: the notes list, the note written in, read
// beside its own source or read whole, and the panel beside it — the note's
// outline, the notes that point at it, and the cards its paragraphs feed.
//
// The screen keeps the session: which note is open, its draft, the one document
// editor the note is written in, the split widths and the panel's tab. What the
// store owns — the notes, their cards, the backlinks — arrives through Deps, and
// every write goes back through it to the app, which owns the store, the reload
// and the card gestures (ADR 0004).
package notes

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
)

const (
	// defaultNotesWidth is the notes column's width until the divider moves:
	// the user's width is kept across frames and window resizes, and the split
	// gives it back when the window has room again. On the smallest window the
	// column keeps its room and the note shrinks, which is the divider's job.
	defaultNotesWidth float32 = 260
	// defaultEditorShare is the document's percentage of the note's room: the
	// panel takes the rest, in proportion, so both follow the window.
	defaultEditorShare float32 = 72
	// defaultReadShare is the document's percentage when read beside its own
	// source: half of the note, so two places can be compared.
	defaultReadShare float32 = 50
	// narrowWindow is the width below which the notebook drops its panel.
	narrowWindow float32 = 900
)

// ViewMode is how the notebook shows the note: written in, written beside the
// same note as it reads, or read whole.
type ViewMode uint8

const (
	Edit ViewMode = iota
	Split
	Read
)

// Note is one note as the notebook lists it: the note itself, how many cards it
// feeds, and the tags it carries.
type Note struct {
	core.Note
	Cards int
	Tags  []string
}

// Backlink is a note that points at the note open now, and the paragraph that
// names it.
type Backlink struct {
	Note      core.ID
	NoteTitle string
	Block     core.ID
	Paragraph int // counted from one, as the user counts paragraphs
	Text      string
}

// CardGroup is one paragraph of the note that feeds cards: the paragraph's place
// in the document, and the cards that point at it.
type CardGroup struct {
	Paragraph int // counted from one, as the user counts paragraphs
	Cards     []core.Card
}

// PanelTab is which of the panel's two views is showing.
type PanelTab uint8

const (
	PanelOutline PanelTab = iota
	PanelBacklinks
)

// panelMode is what the user said about the note's panel, when they said
// anything: a narrow window closes it until they ask for it.
type panelMode uint8

const (
	panelAuto panelMode = iota
	panelOpen
	panelClosed
)

// State is the notebook's session: the note open now, its draft, and the one
// document editor the note is written in. A note is a document whose paragraphs
// are addressable (ADR 0005); the per-block form is gone.
type State struct {
	// Selected is the note the list chose.
	Selected string
	// Draft is the note as the editor holds it, blocks and all.
	Draft core.Note
	// Loaded is the note the draft holds.
	Loaded string
	// Width is the notes column, as the divider left it.
	Width float32
	// View is how the note shows: written in, beside its source, or read whole.
	View ViewMode
	// FindOpen is the in-document find bar.
	FindOpen bool
	// TagFilter is the tag the notes are filtered by, empty for none.
	TagFilter string
	// Query is what the sidebar's search looks for.
	Query string
	// Doc is the note as one document, markers and all.
	Doc *richtext.Document
	// readOptions is the reading views' Markdown options. Keeping one lets
	// their image cache survive a frame; the asset URLs are content-addressed.
	readOptions *markdown.Options
	// OutlineCurrent is the heading the caret is under, -1 for none.
	OutlineCurrent int

	docNote       core.ID // the note the document holds
	docText       string  // the text the document was last built from
	deps          *Deps   // the screen's seams, for work a build pass hands to the next
	title         ui.Handle
	body          ui.Handle
	linkOpen      bool    // the popover a link's address is typed in is up
	linkURL       string  // what that popover holds
	findQuery     string  // what the find bar looks for
	findCurrent   int     // which match is shown
	findJumped    string  // the query the selection was last put on
	jumpTo        int     // a paragraph of the note to put the caret in, 1-based; 0 for none
	focus         bool    // the title takes the caret in the next pass
	editorShare   float32 // the document's percentage of the note's room
	readShare     float32 // the document's percentage when written beside its source
	panel         panelMode
	panelTab      PanelTab
	outline       ui.OutlineState[int] // the note's headings, as the panel lists them
	outlineChosen int                  // the row the user chose in the outline, -1 for none
	outlineFor    core.ID              // the note the outline was opened for
	outlineShape  int                  // how many headings that note had
	pendingJump   int                  // where a panel row asks the caret to go, -1 for none
}

// New returns the notebook's state with the widths and rows the window opens
// with.
func New() State {
	return State{
		Width:          defaultNotesWidth,
		editorShare:    defaultEditorShare,
		readShare:      defaultReadShare,
		outlineChosen:  -1,
		OutlineCurrent: -1,
		pendingJump:    -1,
	}
}

// Deps is what the notebook asks of the app: the notes and cards it reads, and
// one function per thing the user can do. Each runs on the UI thread; the app
// derives the rows, writes the store and reloads.
type Deps struct {
	// Notes are the library's notes, newest first, as the list shows them.
	Notes func() []Note
	// Tags counts how many notes carry each tag: the filter row's chips.
	Tags func() map[string]int
	// Backlinks are the notes that point at the note open now.
	Backlinks func() []Backlink
	// Cards lists the note's card-bearing paragraphs, as the badge strip shows
	// them.
	Cards func(core.Note) []CardGroup
	// Due is how many cards today's queue holds.
	Due func() int

	// Reload retakes the snapshot, so the note open now is read again.
	Reload func()
	// OpenReview shows the queue.
	OpenReview func()
	// NewNote writes an empty note and opens it.
	NewNote func(*ui.Context)
	// Templates are the library's note templates, newest first; the list
	// hides them, and a new note can start from one. Empty when the library
	// holds none, which is when the entry is not shown.
	Templates func() []core.Note
	// NewFromTemplate starts a note from the template named.
	NewFromTemplate func(*ui.Context, core.ID)
	// SaveNote stores the note and returns it as the store holds it.
	SaveNote func(*ui.Context, core.Note) (core.Note, bool)
	// SaveDocument stores the note and reconciles the cards its paragraphs ask
	// for.
	SaveDocument func(*ui.Context, core.Note) (core.Note, bool)
	// PutAsset stores a picture in the library and answers the relative
	// path a note references it by.
	PutAsset func(data []byte, ext string) (string, error)
	// GetAsset reads a stored asset's bytes, false when the library holds
	// no such file.
	GetAsset func(rel string) ([]byte, bool)
	// OpenCardEdit opens the field editor on a card.
	OpenCardEdit func(*ui.Context, core.ID)
}

// View draws the notebook: the notes on the left, and on the right the note
// written in, read beside its own source, or read whole. Each edge is a pane the
// user drags — a split remembers the width it was given — and a narrow window
// drops the note's panel so that the document keeps room to be typed.
func View(c *ui.Context, s *State, d Deps) {
	tk := design.NewTokens(c.Theme())
	s.deps = &d
	w, _ := c.Size()
	ui.Split(c, &s.Width, func() {
		s.notesList(c, tk, d)
	}, func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			switch s.View {
			case Read:
				s.readView(c, tk, d)
			case Split:
				ui.SplitPercent(c, &s.readShare, func() {
					s.editorView(c, tk, d)
				}, func() {
					s.secondPane(c, tk, d)
				}).SplitReset(defaultReadShare).Fill()
			default:
				if !s.panelShown(w) {
					s.editorView(c, tk, d)
					return
				}
				ui.SplitPercent(c, &s.editorShare, func() {
					s.editorView(c, tk, d)
				}, func() {
					s.notePanel(c, tk, d)
				}).SplitReset(defaultEditorShare).Fill()
			}
		})
	}).SplitReset(defaultNotesWidth).Fill()

	// A row chosen in the panel moves the caret after the pass: the editor above
	// keeps its own view state, and a jump made inside the build is written back
	// over by it in the next frame.
	if s.pendingJump >= 0 {
		s.jumpInDocument(c, s.pendingJump)
		s.pendingJump = -1
	}
}

// Follow points the draft at the note the list chose, before that note is
// built, so a choice made in this frame shows in it.
func (s *State) Follow(library []Note) {
	if s.Loaded == s.Selected && s.Draft.ID != "" {
		return
	}
	s.Draft = core.Note{}
	for _, note := range library {
		if string(note.ID) == s.Selected {
			s.hold(note.Note)
			return
		}
	}
	// Nothing chosen, or the choice is gone: the first note stands in.
	if len(library) == 0 {
		s.Loaded, s.Selected = "", ""
		s.Draft = core.Note{}
		return
	}
	s.hold(library[0].Note)
}

// hold takes a copy of a note as the draft, blocks and all, so that editing
// never reaches into the snapshot the next reload replaces.
func (s *State) hold(note core.Note) {
	note.Blocks = append([]core.Block(nil), note.Blocks...)
	s.Draft = note
	s.Loaded = string(note.ID)
	s.Selected = string(note.ID)
	s.Doc, s.docNote, s.docText = nil, "", ""
}

// Keep takes the note the store returned as the draft: the ids it minted are
// what the next save names. The document is kept, so the caret stays where it
// was typed.
func (s *State) Keep(note core.Note) { s.Draft = note }

// ensureDocument gives the note its editor document: built when the note
// changes, kept otherwise, so typing never loses its caret to a rebuild.
func (s *State) ensureDocument() {
	if s.Doc != nil && s.docNote == s.Draft.ID {
		return
	}
	text := DocumentText(s.Draft)
	s.Doc = richtext.NewDocument(richtext.ParseMarkdown(text))
	s.docNote, s.docText = s.Draft.ID, text
	s.rehydrate(s.deps)
}

// rehydrate decodes the pictures a stored note carries: ParseMarkdown keeps
// an image as its reference — the text is the truth, and the pixels are a
// note's to show — so a document is built with name boxes where the pictures
// go, and this puts the bytes of the library's assets behind them. A picture
// that cannot be read or decoded stays its reference: the note file still
// says exactly what it is. deps carries the reader; nil leaves the boxes.
func (s *State) rehydrate(deps *Deps) {
	if s.Doc == nil || deps == nil || deps.GetAsset == nil {
		return
	}
	text := s.Doc.Text()
	// Each attachment is one object-replacement character; its offset is
	// where the picture's attributes live.
	type pending struct {
		at  int
		rel string
	}
	var todo []pending
	for start, p := range text.Paragraphs() {
		off := start
		for _, run := range p.Runs {
			if at := run.Attributes.Attachment; at != nil && isAssetRef(at.URL) && at.Image == nil {
				todo = append(todo, pending{at: off, rel: at.URL})
			}
			off += len([]rune(run.Text))
		}
	}
	if len(todo) == 0 {
		return
	}
	s.Doc.Edit(func(e *richtext.Edit) {
		for _, item := range todo {
			data, ok := deps.GetAsset(item.rel)
			if !ok {
				continue
			}
			img, err := decodeAssetImage(item.rel, data)
			if err != nil {
				continue
			}
			e.Format(item.at, item.at+1, func(a *richtext.Attributes) {
				if at := a.Attachment; at != nil && at.URL == item.rel {
					at.Image = img
				}
			})
		}
	})
}

// isAssetRef says whether a reference is the library's own assets form.
func isAssetRef(url string) bool {
	return strings.HasPrefix(url, "assets/") && !strings.Contains(url, "..")
}

// compact drops adjacent duplicates of a sorted list.
func compact(refs []string) []string {
	out := refs[:0]
	for i, ref := range refs {
		if i == 0 || ref != refs[i-1] {
			out = append(out, ref)
		}
	}
	return out
}

// ParagraphAtCaret is the paragraph the caret is in, counted from zero as the
// note's blocks are; false when the notebook holds no note or the caret is past
// its last paragraph.
func (s *State) ParagraphAtCaret() (int, bool) {
	if s.Doc == nil || s.Draft.ID == "" {
		return 0, false
	}
	text := s.Doc.Text().String()
	_, caret := s.Doc.Selection()
	start, _ := paragraphAt(text, caret)
	// The paragraphs are the blocks, in order: the caret's place among the
	// blank lines is the block it is in.
	at := strings.Count(text[:start], "\n\n")
	return at, at >= 0 && at < len(s.Draft.Blocks)
}

// panelShown reports whether the note's panel takes a column this frame.
func (s *State) panelShown(w float32) bool {
	switch s.panel {
	case panelOpen:
		return true
	case panelClosed:
		return false
	}
	return w >= narrowWindow
}

// TogglePanel opens or closes the note's panel, whatever the window's width.
func (s *State) TogglePanel(c *ui.Context) {
	w, _ := c.Size()
	if s.panelShown(w) {
		s.panel = panelClosed
		return
	}
	s.panel = panelOpen
}

// ShowPanel opens the note's panel on one of its views, from the keyboard.
func (s *State) ShowPanel(tab PanelTab) { s.panel, s.panelTab = panelOpen, tab }

// FocusTitle puts the caret in the note's title in the next pass, which is what
// a note just written wants.
func (s *State) FocusTitle() { s.focus = true }
