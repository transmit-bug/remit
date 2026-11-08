package notes_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// fake is the app the notebook talks to: the snapshot it derived and the
// actions the screen asked for, with no store behind them.
type fake struct {
	library   []notes.Note
	tags      map[string]int
	backlinks []notes.Backlink
	groups    []notes.CardGroup
	due       int

	reloads   int
	reviews   int
	newNotes  int
	saved     []core.Note
	documents []core.Note
	edits     []core.ID

	chooseImage func() (string, error)
	putAsset    func([]byte, string) (string, error)
	getAsset    func(string) ([]byte, bool)
}

func (f *fake) deps() notes.Deps {
	d := notes.Deps{
		Notes:      func() []notes.Note { return f.library },
		Tags:       func() map[string]int { return f.tags },
		Backlinks:  func() []notes.Backlink { return f.backlinks },
		Cards:      func(core.Note) []notes.CardGroup { return f.groups },
		Due:        func() int { return f.due },
		Reload:     func() { f.reloads++ },
		OpenReview: func() { f.reviews++ },
		NewNote:    func(*ui.Context) { f.newNotes++ },
		SaveNote:   func(_ *ui.Context, n core.Note) (core.Note, bool) { f.saved = append(f.saved, n); return n, true },
		SaveDocument: func(_ *ui.Context, n core.Note) (core.Note, bool) {
			f.documents = append(f.documents, n)
			return n, true
		},
		OpenCardEdit: func(_ *ui.Context, id core.ID) { f.edits = append(f.edits, id) },
	}
	if f.chooseImage != nil {
		d.ChooseImage = f.chooseImage
	}
	if f.putAsset != nil {
		d.PutAsset = f.putAsset
	}
	if f.getAsset != nil {
		d.GetAsset = f.getAsset
	}
	return d
}

// notebook is the library the screen is handed: a note with headings and cards,
// and a second one the list can switch to.
var notebook = []notes.Note{
	{Note: core.Note{ID: "note-1", Title: "Memory and recall", Blocks: []core.Block{
		{ID: "block-1", Text: "# The first part"},
		{ID: "block-2", Text: "spacing :: the gaps"},
		{ID: "block-3", Text: "## The second part"},
		{ID: "block-4", Text: "testing :: recall"},
	}}, Cards: 2, Tags: []string{"memory"}},
	{Note: core.Note{ID: "note-2", Title: "Writing cards", Blocks: []core.Block{
		{ID: "block-5", Text: "A good card asks :: one question with one answer."},
		{ID: "block-6", Text: "Cards point at blocks."},
	}}},
}

// newFake is the notebook's app: the seeded library, two tags, one card group
// and four cards due.
func newFake() *fake {
	return &fake{
		library: notebook,
		tags:    map[string]int{"memory": 1, "writing": 1},
		groups:  []notes.CardGroup{{Paragraph: 2, Cards: []core.Card{{ID: "card-1"}}}},
		due:     4,
	}
}

// at draws the notebook at a size, with the first note chosen.
func at(t *testing.T, f *fake, w, h int) (*notes.State, *ui.Tester) {
	t.Helper()
	s := notes.New()
	s.Selected = "note-1"
	d := f.deps()
	tt := ui.NewTester(func(c *ui.Context) { notes.View(c, &s, d) }, w, h)
	tt.Frame()
	return &s, tt
}

// The notebook draws its panes at both the sizes the window opens at: the notes
// list, the note written in, and — when the window has room — the panel with the
// outline and the cards the note feeds. The smallest window drops the panel so
// that the document keeps room to be typed.
func TestPanes(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			_, tt := at(t, f, size.w, size.h)
			for _, text := range []string{
				msgs.Notebook(), msgs.Today(), "4",
				"Memory and recall", "Writing cards",
				msgs.NoteTitle(), msgs.NoteBody(),
			} {
				if !tt.HasText(text) {
					t.Fatalf("missing %q; texts %q", text, tt.Texts())
				}
			}
			// The panel and the cards it lists take a column only when the
			// window has room for them.
			_, panel := tt.Find(msgs.Close())
			if size.w < 900 {
				if panel {
					t.Error("the panel is open on the smallest window")
				}
				return
			}
			if !panel {
				t.Fatalf("the panel is missing; texts %q", tt.Texts())
			}
			for _, text := range []string{"The first part", "The second part", msgs.CardOnParagraph(2, 1)} {
				if !tt.HasText(text) {
					t.Fatalf("missing %q; texts %q", text, tt.Texts())
				}
			}
			if !tt.HasText(msgs.NoteBody()) {
				t.Errorf("the document is not drawn; texts %q", tt.Texts())
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// Choosing a note in the list opens it: the draft follows the choice and the
// document shows that note's text.
func TestChoosingANote(t *testing.T) {
	f := newFake()
	s, tt := at(t, f, 1024, 720)
	if err := tt.Click("Writing cards"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if s.Selected != "note-2" || s.Loaded != "note-2" {
		t.Fatalf("the notebook holds %q / %q, want the note chosen", s.Selected, s.Loaded)
	}
	if text := s.Doc.Text().String(); !strings.Contains(text, "A good card asks") {
		t.Errorf("the document holds %q, want the note chosen", text)
	}
	if tt.HasText("The first part") {
		t.Errorf("the outline still lists the note chosen before; texts %q", tt.Texts())
	}
}

// The search narrows the list by title and by the note's first line, and a tag
// chip narrows it to the notes that carry the tag; choosing the chip again puts
// the library back.
func TestSearchAndTagFilter(t *testing.T) {
	f := newFake()
	s, tt := at(t, f, 1024, 720)

	s.Query = "good card"
	tt.Frame()
	if !tt.HasText("Writing cards") || tt.HasText("Memory and recall") {
		t.Fatalf("the search kept the wrong notes; texts %q", tt.Texts())
	}

	s.Query = ""
	tt.Frame()
	if err := tt.Click("#memory"); err != nil {
		t.Fatalf("the filter has no chip: %v", err)
	}
	tt.Frame()
	if s.TagFilter != "memory" {
		t.Fatalf("the filter is %q, want memory", s.TagFilter)
	}
	if !tt.HasText("Memory and recall") || tt.HasText("Writing cards") {
		t.Fatalf("the tag kept the wrong notes; texts %q", tt.Texts())
	}
	if err := tt.Click("#memory"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.TagFilter != "" {
		t.Errorf("the filter is still %q", s.TagFilter)
	}
	if !tt.HasText("Writing cards") {
		t.Errorf("choosing the chip again did not bring the note back; texts %q", tt.Texts())
	}
}

// Choosing an outline entry puts the caret on that paragraph, which is what
// scrolls it into view, and the document takes the focus.
func TestOutlineJumpsToItsParagraph(t *testing.T) {
	f := newFake()
	s, tt := at(t, f, 1024, 720)

	if err := tt.Click("The second part"); err != nil {
		t.Fatalf("the outline does not offer the heading: %v", err)
	}
	tt.Frame()
	want := notes.ParagraphOffset(notes.DocumentText(s.Draft), 3) // the heading's own paragraph
	_, caret := s.Doc.Selection()
	if caret != want {
		t.Errorf("the caret is at %d, want the second heading at %d", caret, want)
	}
	if !tt.Focused("Note") {
		t.Error("the document did not take the focus after the jump")
	}
}

// The handle appears in the gutter of the paragraph under the pointer. Its
// plus adds an empty paragraph there, and typing in it keeps every other
// block's identity, so the cards pointing at them keep their source.
func TestBlockHandleInsertsParagraph(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	s, tt := at(t, f, 1024, 720)
	tt.Frame()

	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	tt.Move(body.X-44, body.Y+12)
	tt.Frame()
	if !tt.HasText(msgs.AddBlockBelow()) {
		t.Fatalf("the gutter shows no add control; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.AddBlockBelow()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("New block")
	tt.Frame()

	var saved core.Note
	if len(f.documents) == 0 {
		t.Fatal("the inserted block was not saved")
	}
	saved = f.documents[len(f.documents)-1]
	if len(saved.Blocks) != len(notebook[0].Blocks)+1 {
		t.Fatalf("saved %d blocks, want %d", len(saved.Blocks), len(notebook[0].Blocks)+1)
	}
	wantIDs := []string{"block-1", "", "block-2", "block-3", "block-4"}
	for i, id := range wantIDs {
		if id == "" {
			if saved.Blocks[i].ID == "" {
				t.Errorf("the inserted block %d has no id", i)
			}
			continue
		}
		if got := string(saved.Blocks[i].ID); got != id {
			t.Errorf("block %d has id %q, want %q", i, got, id)
		}
	}
	if s.Doc.Text().ParagraphCount() <= 4 {
		t.Error("the editor did not keep the inserted paragraph")
	}
}

// A block's menu turns its paragraph into another shape without touching the
// other paragraphs.
func TestBlockMenuConvertsParagraph(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	s, tt := at(t, f, 1024, 720)
	tt.Frame()

	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	tt.Move(body.X-44, body.Y+12)
	tt.Frame()
	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(msgs.StyleHeading()); err != nil {
		t.Fatalf("the block menu does not offer a heading: %v", err)
	}
	tt.Frame()

	if got := s.Doc.Text().ParagraphStyleAt(0).Heading; got != 2 {
		t.Errorf("the first paragraph is heading %d, want 2", got)
	}
	if len(f.documents) == 0 {
		t.Fatal("the converted block was not saved")
	}
	if got := f.documents[len(f.documents)-1].Blocks[0].Text; got != "## The first part" {
		t.Errorf("saved block is %q, want a level-two heading", got)
	}
}

// Duplicating a block copies its text and gives the copy a new identity,
// while the original and every block after it keep theirs.
func TestBlockMenuDuplicatesParagraph(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	_, tt := at(t, f, 1024, 720)
	tt.Frame()

	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	tt.Move(body.X-44, body.Y+12)
	tt.Frame()
	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(msgs.DuplicateBlock()); err != nil {
		t.Fatalf("the block menu does not offer duplication: %v", err)
	}
	tt.Frame()

	if len(f.documents) == 0 {
		t.Fatal("the duplicated block was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	if len(saved.Blocks) != len(notebook[0].Blocks)+1 {
		t.Fatalf("saved %d blocks, want %d", len(saved.Blocks), len(notebook[0].Blocks)+1)
	}
	if saved.Blocks[0].Text != "# The first part" || saved.Blocks[1].Text != "# The first part" {
		t.Fatalf("the copy is not beside its original: %q", []string{saved.Blocks[0].Text, saved.Blocks[1].Text})
	}
	if saved.Blocks[0].ID != "block-1" {
		t.Errorf("the original has id %q, want block-1", saved.Blocks[0].ID)
	}
	if saved.Blocks[1].ID == "" || saved.Blocks[1].ID == saved.Blocks[0].ID {
		t.Errorf("the copy has id %q, want a new one", saved.Blocks[1].ID)
	}
	for i, want := range []string{"block-2", "block-3", "block-4"} {
		if got := string(saved.Blocks[i+2].ID); got != want {
			t.Errorf("block %d has id %q, want %q", i+2, got, want)
		}
	}
}

// The gutter keeps its room when the window is at its smallest, so a block can
// still be added without covering the writing.
func TestBlockHandlesAtMinimumSize(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	_, tt := at(t, f, 720, 520)
	tt.Frame()

	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	tt.Move(body.X-44, body.Y+12)
	tt.Frame()
	handle, ok := tt.Find(msgs.AddBlockBelow())
	if !ok {
		t.Fatalf("the smallest window hides the block handle; texts %q", tt.Texts())
	}
	if handle.X >= body.X {
		t.Errorf("the handle at x %.1f covers the writing at x %.1f", handle.X, body.X)
	}
}

// The block at the caret keeps its handle even when the pointer is elsewhere,
// so the control is reachable without a hover.
func TestBlockHandleAtCaretIsReachable(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	_, tt := at(t, f, 1024, 720)
	tt.Frame()
	if !tt.HasText(msgs.AddBlockBelow()) {
		t.Fatalf("the caret's block has no handle; texts %q", tt.Texts())
	}
}

// A backlink names the note it is in and the paragraph; choosing it asks the app
// to read that note again and leaves the jump to the next frame.
func TestBacklinkOpensItsNote(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	f.backlinks = []notes.Backlink{{Note: "note-2", NoteTitle: "Writing cards", Block: "block-9", Paragraph: 2, Text: "see [[Memory and recall]]"}}
	s, tt := at(t, f, 1024, 720)

	s.ShowPanel(notes.PanelBacklinks)
	tt.Frame()

	label := msgs.BacklinkAt("Writing cards", 2)
	if !tt.HasText(label) {
		t.Fatalf("the panel does not show the backlink %q; texts %q", label, tt.Texts())
	}
	if err := tt.Click(label); err != nil {
		t.Fatalf("choose the backlink: %v", err)
	}
	tt.Frame()
	if f.reloads == 0 {
		t.Error("choosing the backlink did not ask the app to reload")
	}
	if s.Selected != "note-2" {
		t.Fatalf("the notebook chose %q, want note-2", s.Selected)
	}
	want := notes.ParagraphOffset(notes.DocumentText(s.Draft), 2)
	if _, caret := s.Doc.Selection(); caret != want {
		t.Errorf("the caret is at %d, want the linking paragraph at %d", caret, want)
	}
}

// ⌘F's bar counts the query in the note open now and puts the selection on the
// match: the screen never looks at another note.
func TestFindInTheOpenNote(t *testing.T) {
	f := newFake()
	s, tt := at(t, f, 1024, 720)

	s.FindOpen = true
	tt.Frame()
	tt.Type("gaps")
	tt.Frame()

	text := s.Doc.Text().String()
	want := strings.Index(strings.ToLower(text), "gaps")
	if want < 0 {
		t.Fatalf("the note does not hold the word: %q", text)
	}
	anchor, caret := s.Doc.Selection()
	if anchor != want || caret != want+len("gaps") {
		t.Errorf("the selection is %d..%d, want the match at %d..%d", anchor, caret, want, want+len("gaps"))
	}
	if !s.FindOpen {
		t.Error("the bar closed itself")
	}
}

// A card badge is the way to its overrides: the click goes back to the app with
// the card it names.
func TestBadgeOpensTheFieldEditor(t *testing.T) {
	f := newFake()
	_, tt := at(t, f, 1024, 720)

	if err := tt.Click(locales.In("en").CardOnParagraph(2, 1)); err != nil {
		t.Fatalf("open the card's overrides: %v", err)
	}
	if len(f.edits) != 1 || f.edits[0] != "card-1" {
		t.Errorf("the screen asked for %v, want card-1", f.edits)
	}
}

// A retitled note is saved through the app, and the note the store returns is
// what the draft holds.
func TestTitleSavesThroughDeps(t *testing.T) {
	f := newFake()
	s, tt := at(t, f, 1024, 720)

	if err := tt.Click(locales.In("en").NoteTitle()); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("A better habit")
	tt.Frame()

	if len(f.saved) == 0 {
		t.Fatal("the title did not reach the app")
	}
	if got := f.saved[len(f.saved)-1].Title; got != "A better habit" {
		t.Errorf("the app was handed the title %q, want the typed one", got)
	}
	if s.Draft.Title != "A better habit" {
		t.Errorf("the draft holds %q after the save", s.Draft.Title)
	}
}

// What is typed into the document is saved as blocks through the app, which is
// what reconciles the note's cards.
func TestDocumentSavesThroughDeps(t *testing.T) {
	f := newFake()
	s, tt := at(t, f, 1024, 720)

	if err := tt.Click(locales.In("en").NoteBody()); err != nil {
		t.Fatal(err)
	}
	tt.Type("Read this first. ")
	tt.Frame()

	if len(f.documents) == 0 {
		t.Fatal("the document did not reach the app")
	}
	blocks := f.documents[len(f.documents)-1].Blocks
	if len(blocks) == 0 || !strings.Contains(blocks[len(blocks)-1].Text, "Read this first.") {
		t.Errorf("the app was handed %+v, want the typed paragraph", blocks)
	}
	if len(s.Draft.Blocks) != len(blocks) {
		t.Errorf("the draft holds %d blocks, the app was handed %d", len(s.Draft.Blocks), len(blocks))
	}
}

// The view switch is the notebook's: read whole shows the note without the
// editor, and split shows both.
func TestViewModes(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	s, tt := at(t, f, 1024, 720)

	s.View = notes.Read
	tt.Frame()
	if _, ok := tt.Find(msgs.NoteBody()); ok {
		t.Error("the document is still there in reading view")
	}
	if !tt.HasText("The first part") {
		t.Errorf("the reading view does not show the note; texts %q", tt.Texts())
	}

	s.View = notes.Split
	tt.Frame()
	if _, ok := tt.Find(msgs.NoteBody()); !ok {
		t.Errorf("the document is missing in split view; texts %q", tt.Texts())
	}
	if !tt.HasText("The first part") {
		t.Errorf("the reading pane is missing in split view; texts %q", tt.Texts())
	}
}

// The panel the keyboard asks for opens on the smallest window too: the width
// only decides the panel's default.
func TestShowPanelOnANarrowWindow(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	s, tt := at(t, f, 720, 520)

	if _, ok := tt.Find(msgs.Close()); ok {
		t.Fatal("the panel is open before it was asked for")
	}
	s.ShowPanel(notes.PanelBacklinks)
	tt.Frame()
	if _, ok := tt.Find(msgs.Close()); !ok {
		t.Errorf("the panel did not open; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.BacklinksEmpty()) {
		t.Errorf("the backlinks tab is not up; texts %q", tt.Texts())
	}
}

// A notebook with no note says so and offers the one action that fills it.
func TestEmptyLibrary(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	f.library = nil
	_, tt := at(t, f, 1024, 720)

	if !tt.HasText(msgs.NoNotes()) {
		t.Fatalf("the empty state is missing; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.NewNote()); err != nil {
		t.Fatalf("the empty state has no action: %v", err)
	}
	if f.newNotes != 1 {
		t.Errorf("the app was asked for %d notes, want one", f.newNotes)
	}
}
