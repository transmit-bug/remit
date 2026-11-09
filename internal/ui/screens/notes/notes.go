// Package notes is the write area: the notebook — the notes list, the note
// written in, read beside its own source or read whole, and the panel beside it
// — and the five pages beside the notebook that answer what the library holds:
// the notes list with its search, filters and order (S05), the search over notes
// and cards (S06), the tags (S07), the assets (S08), the templates (S09) and the
// excerpt tree (S10).
//
// Every one of them reads its rows through Deps and reports what the user chose;
// the app derives the rows from the store, so a frame asks it for nothing.
//
// The screen keeps the session: which note is open, its draft, the one document
// editor the note is written in, the split widths and the panel's tab. What the
// store owns — the notes, their cards, the backlinks — arrives through Deps, and
// every write goes back through it to the app, which owns the store, the reload
// and the card gestures (ADR 0004).
package notes

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

const (
	// defaultReadShare is the document's percentage when read beside its own
	// source: half of the note, so two places can be compared.
	defaultReadShare float32 = 50
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

// Backlink is a note that points at the note open now, the paragraph that
// names it, and what that paragraph holds beside its link.
type Backlink struct {
	Note      core.ID
	NoteTitle string
	Block     core.ID
	Paragraph int // counted from one, as the user counts paragraphs
	Text      string
	// Cards is how many cards the linking paragraph feeds, and Excerptable
	// whether it has no excerpt yet: what the row says of the note it names.
	Cards       int
	Excerptable bool
}

// Mark is what read mode says beside one paragraph of the note: the cards it
// feeds, whether something quotes it already, and whether a reading session
// stopped there (S12).
type Mark struct {
	Cards     int
	Excerpted bool
	LastStop  bool
}

// BadgeState is the shape a paragraph's card badge draws, as the prototype's
// four shapes read: a card due now, a card never answered, one that comes
// later, and one that stopped following the paragraph.
type BadgeState uint8

const (
	// BadgeDue is held by a paragraph at least one card of which is due now.
	BadgeDue BadgeState = iota
	// BadgeNew is held while every card of the paragraph is new.
	BadgeNew
	// BadgeLater is held while the paragraph's cards are all scheduled ahead.
	BadgeLater
	// BadgeLost is held when a card stopped following the paragraph, so the
	// words on the paper are no longer the words on the card.
	BadgeLost
)

// Badge is one paragraph's card badge: how many cards read it and the state
// its shape draws. It takes the place the badge strip above the document had.
type Badge struct {
	Paragraph int // counted from one, as the user counts paragraphs
	Cards     int
	State     BadgeState
}

// BlockItem is one action a plugin contributed to the block menu and the block
// inspector: what it says, already in the window's language, and what it runs.
type BlockItem struct {
	Label string
	Run   func(*ui.Context)
}

// Inspector is what the block inspector shows of one paragraph (S20): the
// paragraph itself, the cards that read it, the excerpts taken from it, whether
// a reading session stopped at it, and whether a board places it.
type Inspector struct {
	Block    core.Block
	Cards    []InspectorCard
	Excerpts []InspectorExcerpt
	// ReadingPosition says a reading session stopped at this paragraph.
	ReadingPosition bool
	// OnBoard says a contributed board places this paragraph, so the footer
	// offers the way to it.
	OnBoard bool
}

// InspectorCard is one card of the paragraph: the card, what it asks, and
// where its text comes from now — a card that stopped following the paragraph
// shows the words it was frozen at.
type InspectorCard struct {
	Card  core.Card
	Title string // the card's prompt on one line
	State core.LinkState
}

// InspectorExcerpt is one note taken out of the paragraph.
type InspectorExcerpt struct {
	Note  core.ID
	Title string
	Text  string // the excerpt's first line
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
	// Query is what the notes lists search for: the notebook's column, and the
	// notes page the sub-navigation shows.
	Query string
	// Filter is which notes the notes page keeps besides the query and the tag.
	Filter Filter
	// Sort is the order the notes page shows the notes in.
	Sort Sort
	// SearchQuery is what the search page looks for.
	SearchQuery string
	// Doc is the note as one document, markers and all.
	Doc *richtext.Document
	// PanelToggle is the top bar's panel button, which takes the focus back
	// when the panel closes.
	PanelToggle ui.Handle
	// readOptions is the reading views' Markdown options. Keeping one lets
	// their image cache survive a frame; the asset URLs are content-addressed.
	readOptions *markdown.Options
	// OutlineCurrent is the heading the caret is under, -1 for none.
	OutlineCurrent int

	// The list pages' rows, choices and scrolls: the state a reload must not
	// lose, which is each list's own.
	list         ui.ListState      // the notes page's rows
	tagList      ui.ListState      // the tags page's rows
	assetList    ui.ListState      // the assets page's rows
	templateList ui.ListState      // the templates page's rows
	searchNotes  ui.ListState      // the search page's note hits
	searchCards  ui.ListState      // the search page's card hits
	search       ui.Handle         // the search page's field, for a link or ⌘F
	treeOpen     map[core.ID]*bool // which excerpt-tree nodes are open

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
	saveFailed    bool    // the last write did not reach the store
	jumpTo        int     // a paragraph of the note to put the caret in, 1-based; 0 for none
	focus         bool    // the title takes the caret in the next pass
	readShare     float32 // the document's percentage when written beside its source
	panel         panelMode
	panelTab      PanelTab
	panelFocus    ui.Handle            // the panel itself, which takes the focus as it opens
	panelWant     bool                 // the panel was just asked for: it takes the focus next pass
	backlinks     ui.ListState         // the panel's backlink rows
	backlinkChose int                  // the backlink row chosen, -1 for none
	outline       ui.OutlineState[int] // the note's headings, as the panel lists them
	outlineChosen int                  // the row the user chose in the outline, -1 for none
	outlineFor    core.ID              // the note the outline was opened for
	outlineShape  int                  // how many headings that note had
	pendingJump   int                  // where a panel row asks the caret to go, -1 for none
	blockHover    int                  // the visible paragraph under the pointer, -1 for none
	selectOpen    bool                 // the selection's action bar is up
	selectRange   [2]int               // the selection the bar acts on
	selectBlock   core.ID              // the paragraph that selection is in
	selectSkipped [2]int               // the selection whose bar was dismissed
	inspect       *Inspector           // the paragraph the block inspector shows, nil while closed
	inspectOpen   bool                 // the block inspector dialog is up
	readBlock     int                  // the paragraph read mode is on, -1 before one is chosen
	blockMenus    map[int]bool         // the visible paragraphs whose block menu is open
	blockDelete   int                  // the paragraph a delete awaits confirmation for, -1 for none
	blockLost     int                  // the cards that delete would leave without a source
	renameNote    core.ID              // the note a rename is typed for, "" for none
	renameText    string               // the title the rename holds
	renameField   ui.Handle            // the field the rename types in
	deleteNote    core.ID              // the note a delete awaits confirmation for, "" for none
	deleteCards   int                  // the cards that delete would leave without a source
	slashOpen     bool                 // the block insertion menu is up
	slashSeen     bool                 // the current slash query already opened its menu
	slashQuery    string               // what filters the slash menu
	slashStart    int                  // where the slash query starts in the document
	slashEnd      int                  // where it ends, at the caret that chose a command
	slashChoice   int                  // the chosen slash command, -1 until one is taken
	slashTake     bool                 // Enter or a click took the chosen command
	slashMatches  []slashCommand       // the commands the query leaves
	tables        map[string]string    // table source by the attachment reference that stands for it
	tableWord     string               // the localized name a table placeholder shows
}

// FocusSearch puts the caret in the search page's field. A link from outside
// the window may ask before the field was ever built: the request waits for it.
func (s *State) FocusSearch() { s.search.Focus() }

// ClearFilters puts the whole notebook back on the notes page: no query, no
// tag, none of the three filters.
func (s *State) ClearFilters() { s.Query, s.TagFilter, s.Filter = "", "", Filter{} }

// New returns the notebook's state with the widths and rows the window opens
// with.
func New() State {
	return State{
		Width:          design.NotesColumnWidth,
		readShare:      defaultReadShare,
		outlineChosen:  -1,
		backlinkChose:  -1,
		OutlineCurrent: -1,
		pendingJump:    -1,
		blockHover:     -1,
		blockDelete:    -1,
		slashChoice:    -1,
		readBlock:      -1,
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
	// Cards lists the note's card-bearing paragraphs, as the block menu's
	// delete warning counts them.
	Cards func(core.Note) []CardGroup
	// Badges are the paragraph gutter's card badges: how many cards each
	// paragraph feeds and the state its shape draws.
	Badges func(core.Note) []Badge
	// Marks are what read mode says beside each paragraph of the note.
	Marks func(core.Note) map[core.ID]Mark
	// Inspector is what the block inspector shows of one paragraph, false when
	// the note no longer holds it.
	Inspector func(core.ID) (Inspector, bool)
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
	// ChooseImage asks the system for one picture file to insert, and answers
	// its path: empty when the user cancels. nil leaves the image command out.
	ChooseImage func() (string, error)
	// OpenCardEdit opens the field editor on a card.
	OpenCardEdit func(*ui.Context, core.ID)
	// OpenCardEditor opens the field editor on a block: the card it feeds, or
	// the empty fields a card of the chosen type starts with.
	OpenCardEditor func(*ui.Context, core.ID)
	// OpenCardDetail shows a card's source and its schedule history.
	OpenCardDetail func(*ui.Context, core.ID)
	// ClozeSelection rewrites the selected words of a paragraph into a cloze
	// blank and makes the cards the paragraph then asks for, answering whether
	// the note was written (S15).
	ClozeSelection func(*ui.Context, core.ID, core.ID, string) bool
	// Excerpt takes one paragraph out into a note of its own.
	Excerpt func(*ui.Context, core.ID, core.ID)
	// LocateOnBoard opens the board page on a paragraph's node, when a board
	// places it.
	LocateOnBoard func(*ui.Context, core.ID)
	// OnBoard reports whether a board the window holds places that paragraph. It
	// reads the pages' cached data, so the entry a paragraph's menu offers is one
	// that has somewhere to go.
	OnBoard func(core.ID) bool
	// BlockItems are the actions plugins contributed to the block menu and the
	// block inspector, in drawing order.
	BlockItems func(*ui.Context) []BlockItem
	// SlashItems are the actions plugins contributed to the `/` insert menu. They
	// follow the host's own commands at its foot, and the query filters them as
	// it filters those.
	SlashItems func(*ui.Context) []BlockItem
	// RowItems are the actions plugins contributed to a note row's menu.
	RowItems func(*ui.Context) []BlockItem
	// RenameNote writes a note's new title from its row, and answers whether it
	// was written.
	RenameNote func(*ui.Context, core.ID, string) bool
	// DeleteNote removes a note from the library, leaving the cards its
	// paragraphs fed lost, and answers whether it was removed.
	DeleteNote func(*ui.Context, core.ID) bool

	// Rows is the notes page's rows — the query, tag, filters and order the
	// state holds, answered by the store — and how many notes match in all.
	Rows func() (rows []Row, total int)
	// TagRows are the tags page's rows.
	TagRows func() []TagRow
	// AssetRows are the assets page's rows.
	AssetRows func() []AssetRow
	// ImportAssets asks the system for files and stores them in the library.
	ImportAssets func(*ui.Context)
	// SearchResults is the search page's answer: the notes and the cards the
	// state's query found, and how many results there are in all.
	SearchResults func() (notes, cards []Hit, total int)
	// Excerpts is the excerpt tree: the notes excerpts were taken from, and
	// the excerpts under each.
	Excerpts func() []TreeNode
	// Now is the clock the lists' "today" is measured against, so that a
	// frame reads no clock of its own.
	Now func() time.Time

	// OpenNote shows a note in the notebook.
	OpenNote func(core.ID)
	// OpenNoteRead shows a note read whole.
	OpenNoteRead func(core.ID)
	// QueueReading puts a note in the reading queue.
	QueueReading func(*ui.Context, core.ID)
	// OpenTag opens the notes page narrowed to one tag.
	OpenTag func(string)
	// OpenReadingQueue shows the reading queue.
	OpenReadingQueue func()
	// OpenCard shows a card's detail. Until that page is built it opens the
	// note the card renders from, read whole: a hit always goes somewhere that
	// shows the card's own text.
	OpenCard func(core.ID)
}

// View draws the notebook: the notes on the left, and on the right the note
// written in, read beside its own source, or read whole. The notes column is a
// split's pane, so its width is the user's and a double click puts it back;
// the panel is the note's second column, and a window too narrow for both
// shows it floating over the paper instead.
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
				// The note written beside the same note read: half the room each,
				// so two places can be compared.
				ui.SplitPercent(c, &s.readShare, func() {
					s.editorView(c, tk, d)
				}, func() {
					s.secondPane(c, tk, d)
				}).SplitReset(defaultReadShare).Fill()
			default:
				s.writeArea(c, tk, d, w)
			}
		})
	}).SplitReset(design.NotesColumnWidth).Fill()

	// A row chosen in the panel moves the caret after the pass: the editor above
	// keeps its own view state, and a jump made inside the build is written back
	// over by it in the next frame.
	if s.pendingJump >= 0 {
		s.jumpInDocument(c, s.pendingJump)
		s.pendingJump = -1
	}
	// The inspector is a dialog over everything else, so it is built last.
	s.inspectorView(c, tk, d)
}

// writeArea is the note written in, with its panel beside it when the window
// has room for both. The panel is a column of its own at PanelWidth, giving
// way to the paper's floor rather than squeezing the writing out; a window too
// narrow for the two shows it floating over the paper, which is the same panel
// with the same rows (S21/S22).
func (s *State) writeArea(c *ui.Context, tk design.Tokens, d Deps, w float32) {
	if !s.panelShown(w) {
		s.editorView(c, tk, d)
		return
	}
	if w <= design.BreakpointNarrow {
		s.editorView(c, tk, d)
		s.panelOverlay(c, tk, d)
		return
	}
	// The panel is a column of its own at the design's width, so its rows — the
	// tabs and the close — fit at every window it shows in; the paper takes the
	// rest and the notes column, which is a split, is how a narrow window gives
	// the writing more room.
	ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
		s.editorView(c, tk, d)
		ui.Divider(c).Shrink(0).AlignSelf(ui.Stretch)
		s.notePanel(c, tk, d)
	})
}

// panelOverlay shows the note's panel over the paper on a window too narrow for
// a second column: a drawer at the window's edge, dismissed with Escape or a
// press beside it. The focus goes into it as it opens and back to the top bar's
// button as it closes.
func (s *State) panelOverlay(c *ui.Context, tk design.Tokens, d Deps) {
	msgs := locales.In(c.Locale())
	ui.Overlay(c, func() {
		back := ui.Row(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Justify(ui.End).Background(tk.Scrim).Modal()
		back.Children(func() {
			panel := ui.Column(c).Width(tk.PanelWidth).FillHeight().Shrink(0).
				Background(tk.Surface).Role(ui.RoleDialog).Label(msgs.PanelTitle())
			if panel.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				s.closePanel(c)
			}
			s.panelBody(c, tk, d)
		})
	})
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
	s.blockHover, s.blockMenus = -1, nil
	s.blockDelete, s.blockLost = -1, 0
	s.readBlock, s.selectOpen, s.inspectOpen = -1, false, false
	s.inspect = nil
	s.resetSlash()
}

// Forget drops the note the notebook holds open, which deleting it from the
// list leaves behind: the next frame that draws the editor must not save a note
// that is gone back into the library.
func (s *State) Forget() { s.hold(core.Note{}) }

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
	s.Doc = richtext.NewDocument(s.noteDocument())
	s.docNote, s.docText = s.Draft.ID, text
	s.blockHover, s.blockMenus = -1, nil
	s.blockDelete, s.blockLost = -1, 0
	s.resetSlash()
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

// ParagraphAtCaret is the paragraph the caret is in, counted from zero as the
// note's blocks are; false when the notebook holds no note or the caret is past
// its last paragraph.
func (s *State) ParagraphAtCaret() (int, bool) {
	if s.Doc == nil || s.Draft.ID == "" {
		return 0, false
	}
	_, caret := s.Doc.Selection()
	at := s.paragraphAt(caret)
	return at, at >= 0 && at < len(s.Draft.Blocks)
}

// paragraphAt is the paragraph of the editor's own text an offset falls in,
// counted from zero as the note's blocks are. The paragraphs are the document's
// structure, not a split on characters: a blank line the user types is part of
// the paragraph it is in until the editor starts another, and each paragraph's
// newline belongs to it.
func (s *State) paragraphAt(offset int) int {
	if s.Doc == nil {
		return -1
	}
	index, at := 0, 0
	for _, paragraph := range s.Doc.Text().Paragraphs() {
		length := 0
		for _, run := range paragraph.Runs {
			length += len([]rune(run.Text))
		}
		if offset <= at+length {
			return index
		}
		at += length + 1
		index++
	}
	return max(0, index-1)
}

// panelShown reports whether the note's panel shows this frame: a column on a
// window wide enough for the paper and the panel, and the same panel floating
// over the paper below that. A narrow window closes it until the user asks for
// it, so the writing starts with the room the whole window gives.
func (s *State) panelShown(w float32) bool {
	switch s.panel {
	case panelOpen:
		return true
	case panelClosed:
		return false
	}
	return w > design.BreakpointNarrow
}

// TogglePanel opens or closes the note's panel, whatever the window's width.
// The panel takes the focus as it opens; closing it gives the focus back to the
// top bar's button, which is what opened it from the keyboard.
func (s *State) TogglePanel(c *ui.Context) {
	w, _ := c.Size()
	if s.panelShown(w) {
		s.closePanel(c)
		return
	}
	s.ShowPanel(s.panelTab)
}

// closePanel puts the panel away and hands the focus back to the control that
// opens it, so the keyboard goes on from where it was.
func (s *State) closePanel(c *ui.Context) {
	s.panel, s.panelWant = panelClosed, false
	s.PanelToggle.Focus(c)
}

// ShowPanel opens the note's panel on one of its views, from the keyboard.
func (s *State) ShowPanel(tab PanelTab) { s.panel, s.panelTab, s.panelWant = panelOpen, tab, true }

// FocusTitle puts the caret in the note's title in the next pass, which is what
// a note just written wants.
func (s *State) FocusTitle() { s.focus = true }

// PanelVisible reports whether the note's panel shows in a window this wide, so
// the top bar's button can say so.
func (s *State) PanelVisible(w float32) bool { return s.panelShown(w) }
