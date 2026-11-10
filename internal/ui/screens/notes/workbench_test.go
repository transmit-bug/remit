package notes_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// inspectorOf is what the fake answers for one block of the seeded note: the
// paragraph, one card that reads it, one note taken out of it, the reading
// position on it, and a board that may place it.
func inspectorOf(onBoard bool) notes.Inspector {
	return notes.Inspector{
		Block: core.Block{ID: "block-2", Text: "spacing :: the gaps"},
		Cards: []notes.InspectorCard{{
			Card:  core.Card{ID: "card-1", Ref: core.Reference{Block: "block-2"}},
			Title: "spacing",
			State: core.LinkLive,
		}},
		Excerpts: []notes.InspectorExcerpt{{
			Note: "note-3", Title: "Excerpt of Memory and recall", Text: "the gaps",
		}},
		ReadingPosition: true,
		OnBoard:         onBoard,
	}
}

// A paragraph's card badge is the way to its inspector: the click opens the
// dialog on that paragraph, with its cards and its excerpts, which is where the
// badge strip above the document used to lead.
func TestDocGutterBadgeOpensTheBlockInspector(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	f.shown = map[core.ID]notes.Inspector{"block-2": inspectorOf(false)}
	s, tt := at(t, f, 1024, 720)
	s.Doc.SetSelection(0, 0)
	tt.Frame()

	badge := msgs.CardOnParagraph(1, 2)
	if err := tt.Click(badge); err != nil {
		t.Fatalf("the paragraph has no badge: %v", err)
	}
	tt.Frame()

	if len(f.inspected) != 1 || f.inspected[0] != "block-2" {
		t.Fatalf("the badge asked for %v, want block-2", f.inspected)
	}
	if !tt.HasText(msgs.BlockInspector()) {
		t.Fatalf("the inspector did not open; texts %q", tt.Texts())
	}
	if !tt.HasText("spacing :: the gaps") || !tt.HasText(msgs.ReadingParagraph(2)) {
		t.Errorf("the inspector does not show its paragraph; texts %q", tt.Texts())
	}
	if !tt.HasText("spacing") {
		t.Errorf("the inspector does not show the card that reads it; texts %q", tt.Texts())
	}
}

// The selection bar's cloze asks the app to rewrite the paragraph, and the block
// it names is the paragraph the selection is in — not the one the caret was in
// before it.
func TestSelectionBarClozesTheSelection(t *testing.T) {
	f := newFake()
	s, tt := at(t, f, 1024, 720)
	start := runeIndex(s.Doc.Text().String(), "the gaps")
	s.Doc.SetSelection(start, start+len([]rune("the gaps")))
	tt.Frame()

	msgs := locales.In("en")
	if err := tt.Click(msgs.SelectionCloze()); err != nil {
		t.Fatalf("the selection raised no cloze action: %v", err)
	}
	tt.Frame()

	if len(f.clozes) != 1 {
		t.Fatalf("the screen asked for %d clozes, want one", len(f.clozes))
	}
	want := [3]string{"note-1", "block-2", "the gaps"}
	if f.clozes[0] != want {
		t.Errorf("the cloze names %v, want %v", f.clozes[0], want)
	}
	if tt.HasText(msgs.SelectionCloze()) {
		t.Error("the bar stayed up after the cloze")
	}
}

// The selection bar goes back to the app for the whole paragraph, not for the
// words selected: an excerpt is a paragraph, which is the unit the reference
// model keeps.
func TestSelectionBarExcerptsTheWholeBlock(t *testing.T) {
	f := newFake()
	s, tt := at(t, f, 1024, 720)
	start := runeIndex(s.Doc.Text().String(), "the gaps")
	s.Doc.SetSelection(start, start+len([]rune("the gaps")))
	tt.Frame()

	msgs := locales.In("en")
	if err := tt.Click(msgs.ExcerptWholeBlock()); err != nil {
		t.Fatalf("the selection raised no excerpt action: %v", err)
	}
	tt.Frame()

	if len(f.excerpts) != 1 || f.excerpts[0] != "block-2" {
		t.Fatalf("the excerpt names %v, want block-2", f.excerpts)
	}
	if len(f.clozes) != 0 {
		t.Errorf("the excerpt also clozed %v", f.clozes)
	}
}

// The smallest stage keeps the paragraph's controls inside the paper and above
// the writing: the gutter has no room beside a 720-wide window, so the badge and
// the handle sit in one compact row over the text, clipped by nothing.
func TestPaperKeepsGutterAtMinimumSize(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	s, tt := at(t, f, 720, 520)
	// The row a narrow stage shows is the caret's paragraph's: the caret goes
	// to the paragraph that carries a card.
	at := runeIndex(s.Doc.Text().String(), "the gaps")
	s.Doc.SetSelection(at, at)
	tt.Frame()

	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	badge, ok := tt.Find(msgs.CardOnParagraph(1, 2))
	if !ok {
		t.Fatalf("the paragraph has no badge; texts %q", tt.Texts())
	}
	add, ok := tt.Find(msgs.AddBlockBelow())
	if !ok {
		t.Fatalf("the caret's paragraph has no handle; texts %q", tt.Texts())
	}
	for _, control := range []struct {
		name string
		box  ui.Rect
	}{{"badge", badge}, {"handle", add}} {
		if control.box.W <= 0 || control.box.H <= 0 {
			t.Errorf("the %s has an empty box %+v", control.name, control.box)
		}
		if control.box.X < 0 || control.box.X+control.box.W > 720 {
			t.Errorf("the %s at %v..%v is outside the window", control.name,
				control.box.X, control.box.X+control.box.W)
		}
		if control.box.Y+control.box.H > body.Y+1 {
			t.Errorf("the %s ending at %v covers the writing starting at %v",
				control.name, control.box.Y+control.box.H, body.Y)
		}
	}
}

// The reading view marks what each paragraph carries — its cards, an excerpt of
// it, and the place a reading session stopped — and holds no writing tools.
func TestReadModeMarksCardsAndExcerpts(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	f.marks = map[core.ID]notes.Mark{
		"block-2": {Cards: 2, Excerpted: true},
		"block-4": {LastStop: true},
	}
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			s, tt := at(t, f, size.w, size.h)
			s.View = notes.Read
			tt.Frame()

			if _, ok := tt.Find(msgs.NoteBody()); ok {
				t.Error("the reading view still draws the editor")
			}
			for _, want := range []string{
				msgs.CardCount(2), msgs.ReadExcerptedMark(), msgs.ReadLastStop(),
				msgs.ReadingParagraph(1), msgs.ReadingParagraph(4),
			} {
				if !tt.HasText(want) {
					t.Fatalf("missing %q; texts %q", want, tt.Texts())
				}
			}
		})
	}
}

// A paragraph chosen in the reading view is the one an excerpt takes: the row
// takes the click, and the view names the paragraph it holds.
func TestReadModeChoosesItsParagraph(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	s, tt := at(t, f, 1024, 720)
	s.View = notes.Read
	tt.Frame()
	if s.ReadParagraph() != 1 {
		t.Fatalf("the reading view opens on paragraph %d, want the first", s.ReadParagraph())
	}

	if err := tt.Click(msgs.ReadingParagraph(3)); err != nil {
		t.Fatalf("the reading view has no rows: %v", err)
	}
	tt.Frame()
	if got := s.ReadParagraph(); got != 3 {
		t.Errorf("the reading view holds paragraph %d, want the third", got)
	}
	if _, block, ok := s.CurrentReadBlock(); !ok || block.ID != "block-3" {
		t.Errorf("the paragraph named is %q, want block-3", block.ID)
	}
}

// The panel switches between the outline and the notes that point here, and a
// backlink row says what its own paragraph carries.
func TestPanelSwitchesToBacklinks(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	f.backlinks = []notes.Backlink{{
		Note: "note-2", NoteTitle: "Writing cards", Block: "block-9", Paragraph: 2,
		Text: "see [[Memory and recall]]", Cards: 2, Excerptable: true,
	}}
	s, tt := at(t, f, 1440, 900)
	if _, ok := tt.Find(msgs.BacklinksTab(1)); !ok {
		t.Fatalf("the panel has no backlinks tab; texts %q", tt.Texts())
	}

	if err := tt.Click(msgs.BacklinksTab(1)); err != nil {
		t.Fatalf("switch the panel: %v", err)
	}
	tt.Frame()
	if !tt.HasText(msgs.BacklinkMeta(2)) {
		t.Fatalf("the panel does not list the backlink; texts %q", tt.Texts())
	}
	if tt.HasText("The first part") {
		t.Errorf("the outline is still up beside the backlinks; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.BacklinkAt("Writing cards", 2)); err != nil {
		t.Fatalf("choose the backlink: %v", err)
	}
	tt.Frame()
	if f.reloads == 0 || s.Selected != "note-2" {
		t.Errorf("the backlink did not open its note: %d reloads, %q chosen", f.reloads, s.Selected)
	}
}

// A paragraph's menu offers its inspector and the actions plugins contributed for
// it, at the bottom under the host's own.
func TestBlockMenuOffersTheInspector(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	f.shown = map[core.ID]notes.Inspector{"block-1": {
		Block: core.Block{ID: "block-1", Text: "# The first part"},
	}}
	_, tt := at(t, f, 1024, 720)
	tt.Frame()

	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatalf("the paragraph has no menu: %v", err)
	}
	tt.Frame()
	if !tt.HasText(msgs.OpenInspector()) {
		t.Fatalf("the menu does not offer the inspector; texts %q", tt.Texts())
	}
	if !tt.HasText("Plugin block action") {
		t.Errorf("the menu does not offer the plugin's action; texts %q", tt.Texts())
	}
	// The menu is longer than the window allows, so its tail scrolls into view.
	if err := scrollMenuTo(t, tt, msgs.OpenInspector()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(msgs.OpenInspector()); err != nil {
		t.Fatalf("open the inspector: %v", err)
	}
	tt.Frame()
	if len(f.inspected) != 1 || f.inspected[0] != "block-1" {
		t.Errorf("the menu asked for %v, want block-1", f.inspected)
	}
	if !tt.HasText(msgs.BlockInspector()) {
		t.Errorf("the inspector did not open; texts %q", tt.Texts())
	}
}

// A note's last paragraph cannot be deleted: the menu's delete is disabled and
// says why, and nothing is written when it is pressed.
func TestBlockMenuKeepsTheLastBlock(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	f.library = []notes.Note{{Note: core.Note{ID: "note-9", Title: "One paragraph", Blocks: []core.Block{
		{ID: "block-9", Text: "the only paragraph"},
	}}}}
	f.groups, f.badges = nil, nil
	s, tt := at(t, f, 1024, 720)
	s.Selected = "note-9"
	tt.Frame()

	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatalf("the paragraph has no menu: %v", err)
	}
	tt.Frame()
	if !tt.HasText(msgs.LastBlockKept()) {
		t.Fatalf("the menu does not say why the paragraph stays; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.DeleteBlock()); err == nil {
		tt.Frame()
		if tt.HasText(msgs.DeleteBlockTitle()) {
			t.Error("a disabled delete asked to confirm")
		}
		if len(f.documents) != 0 {
			t.Errorf("the last paragraph was written away: %+v", f.documents)
		}
	}
	if len(s.Draft.Blocks) != 1 {
		t.Errorf("the note holds %d paragraphs, want its last one", len(s.Draft.Blocks))
	}
}

// The inspector shows a paragraph's three roles at once — its text, the cards
// that read it, and the notes taken out of it — and each row goes where it
// promises: the card to its own detail, the excerpt to its note.
func TestInspectorShowsTheThreeRoles(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f := newFake()
			f.shown = map[core.ID]notes.Inspector{"block-2": inspectorOf(false)}
			s, tt := at(t, f, size.w, size.h)
			// The paragraph's badge is in the gutter where the stage keeps one
			// and in the caret's compact row where it does not.
			at := runeIndex(s.Doc.Text().String(), "the gaps")
			s.Doc.SetSelection(at, at)
			tt.Frame()

			if err := tt.Click(msgs.CardOnParagraph(1, 2)); err != nil {
				t.Fatalf("open the inspector: %v", err)
			}
			tt.Frame()

			for _, want := range []string{
				msgs.InspectorText(), "spacing :: the gaps",
				msgs.InspectorCards(1), "spacing",
				msgs.InspectorExcerpts(1), "Excerpt of Memory and recall",
				msgs.InspectorReadingPosition(),
			} {
				if !tt.HasText(want) {
					t.Fatalf("missing %q; texts %q", want, tt.Texts())
				}
			}
			if err := tt.Click("spacing"); err != nil {
				t.Fatalf("the card row does not open: %v", err)
			}
			tt.Frame()
			if len(f.details) != 1 || f.details[0] != "card-1" {
				t.Errorf("the card row asked for %v, want card-1", f.details)
			}

			// The excerpt row opens the note taken out of the paragraph.
			if err := tt.Click(msgs.CardOnParagraph(1, 2)); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click("Excerpt of Memory and recall"); err != nil {
				t.Fatalf("the excerpt row does not open: %v", err)
			}
			tt.Frame()
			if len(f.opened) != 1 || f.opened[0] != "note-3" {
				t.Errorf("the excerpt row opened %v, want note-3", f.opened)
			}
		})
	}
}

// The way to the board appears only when a board places the paragraph: a control
// that would do nothing is not drawn.
func TestInspectorOffersTheWhiteboardWhenANodeExists(t *testing.T) {
	msgs := locales.In("en")
	for _, onBoard := range []bool{false, true} {
		name := "without a node"
		if onBoard {
			name = "with a node"
		}
		t.Run(name, func(t *testing.T) {
			f := newFake()
			f.shown = map[core.ID]notes.Inspector{"block-2": inspectorOf(onBoard)}
			_, tt := at(t, f, 1024, 720)
			if err := tt.Click(msgs.CardOnParagraph(1, 2)); err != nil {
				t.Fatalf("open the inspector: %v", err)
			}
			tt.Frame()

			if got := tt.HasText(msgs.InspectorLocateOnBoard()); got != onBoard {
				t.Fatalf("locate-on-board shown %v, want %v; texts %q", got, onBoard, tt.Texts())
			}
			if !onBoard {
				return
			}
			if err := tt.Click(msgs.InspectorLocateOnBoard()); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(f.boarded) != 1 || f.boarded[0] != "block-2" {
				t.Errorf("the inspector asked for the board on %v, want block-2", f.boarded)
			}
		})
	}
}

// scrollMenuTo scrolls an open block menu until the row named is in view,
// which is how a reader reaches the tail of a menu taller than the window.
func scrollMenuTo(t *testing.T, tt *ui.Tester, label string) error {
	t.Helper()
	over, ok := tt.Find(locales.In("en").DeleteBlock())
	if !ok {
		return errors.New("the block menu is not open")
	}
	x, y := over.X+over.W/2, over.Y+over.H/2
	for i := 0; i < 12; i++ {
		if in, ok := tt.Find(label); ok && in.W > 0 && in.H > 0 {
			return nil
		}
		tt.Scroll(x, y, 0, 60)
		tt.Frame()
	}
	return errors.New("the row never scrolled into view: " + label)
}

// A write that does not reach the store says so where the writing is, and its
// retry writes the same note again: a toast alone would leave the paper looking
// saved.
func TestSaveFailureOffersRetry(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	f.failSaves = 1
	_, tt := at(t, f, 1024, 720)

	if err := tt.Click(msgs.NoteBody()); err != nil {
		t.Fatal(err)
	}
	tt.Type("Read this first. ")
	tt.Frame()
	if !tt.HasText(msgs.SaveFailed()) {
		t.Fatalf("the failed write is silent; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.NoteRetry()); err != nil {
		t.Fatalf("the notice has no retry: %v", err)
	}
	tt.Frame()
	if tt.HasText(msgs.SaveFailed()) {
		t.Errorf("the notice stayed after the retry; texts %q", tt.Texts())
	}
	if len(f.documents) != 2 {
		t.Fatalf("the app was handed %d documents, want the failed one and the retry", len(f.documents))
	}
	if last := f.documents[len(f.documents)-1]; !strings.Contains(last.Blocks[len(last.Blocks)-1].Text, "Read this first.") {
		t.Errorf("the retry wrote %q, want the paper as it stands", last.Blocks)
	}
}

// runeIndex is where a substring starts, counted in runes as a document counts
// its offsets.
func runeIndex(text, sub string) int {
	at := strings.Index(text, sub)
	if at < 0 {
		return 0
	}
	return len([]rune(text[:at]))
}
