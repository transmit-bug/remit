package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// selectInDocument puts the editor's selection on a passage of the note, which is
// what a reader does with the pointer before the action bar rises. Offsets are
// the document's own, in runes.
func selectInDocument(t *testing.T, a *App, passage string, withEnd bool) {
	t.Helper()
	text := a.notes.Doc.Text().String()
	at := strings.Index(text, passage)
	if at < 0 {
		t.Fatalf("the note does not hold %q: %q", passage, text)
	}
	start := len([]rune(text[:at]))
	end := start
	if withEnd {
		end = start + len([]rune(passage))
	}
	a.notes.Doc.SetSelection(start, end)
}

// The selection bar's cloze is one rewrite of the paragraph: the words become a
// {{…}} blank in the note, and the blank is what makes the card. A strip above
// the document could not do this at all.
func TestSelectionBarClozesTheBlockAndMakesACard(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	newNoteTyped(t, a, tt, "Practice spread over days keeps memory strong.")

	selectInDocument(t, a, "spread over days", true)
	tt.Frame()
	if err := tt.Click(msgs.SelectionCloze()); err != nil {
		t.Fatalf("the selection raised no cloze action: %v", err)
	}
	tt.Frame()

	if got := a.notes.Doc.Text().String(); !strings.Contains(got, "Practice {{spread over days}}") {
		t.Errorf("the paper holds %q, want the blank where the selection was", got)
	}
	note, ok := a.Library().Note(a.notes.Draft.ID)
	if !ok {
		t.Fatal("the note is gone")
	}
	if source, err := os.ReadFile(filepath.Join(a.Settings().LibraryDir, "notes", noteFile(a, note.ID))); err == nil {
		if !strings.Contains(string(source), "{{spread over days}}") {
			t.Errorf("the note file holds no blank:\n%s", source)
		}
	}
	var cloze []core.Card
	for _, card := range a.Library().Cards() {
		if card.Type == core.TypeCloze && card.Ref.Block == a.notes.Draft.Blocks[0].ID {
			cloze = append(cloze, card)
		}
	}
	if len(cloze) != 1 {
		t.Fatalf("the blank made %d cloze cards, want one", len(cloze))
	}
	prompt, answer, _ := a.faces(cloze[0])
	if prompt != "Practice […] keeps memory strong." || answer != "Practice spread over days keeps memory strong." {
		t.Errorf("the card asks %q and answers %q", prompt, answer)
	}
}

// E in the reading view takes the paragraph on screen out into a note that
// points back at it: the reading position is what the reader chose with the
// pointer, not wherever the note opened.
func TestReadModeExcerptsTheCurrentBlock(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	a.notes.View = notes.Read
	tt.Frame()

	if err := tt.Click(msgs.ReadingParagraph(2)); err != nil {
		t.Fatalf("the reading view has no rows: %v", err)
	}
	tt.Frame()
	if got := a.notes.ReadParagraph(); got != 2 {
		t.Fatalf("the reading view holds paragraph %d, want the second", got)
	}
	before := len(a.Library().Notes())

	tt.Key(0, ui.KeyE)
	tt.Frame()
	tt.Advance(320 * time.Millisecond)

	after := a.Library().Notes()
	if len(after) != before+1 {
		t.Fatalf("the library holds %d notes, want one more", len(after))
	}
	source := seed.memory.Blocks[1].ID
	quoted := false
	for _, note := range after {
		for _, block := range note.Blocks {
			for _, link := range core.Links(block.Text) {
				if link.Block == source {
					quoted = true
				}
			}
		}
	}
	if !quoted {
		t.Errorf("the excerpt does not point at block %q", source)
	}
}

// The find bar counts the query in the note open now and moves the selection
// through the matches: two of them, so Enter goes to the second and wraps.
func TestFindBarCountsMatches(t *testing.T) {
	a, _, tt := openApp(t)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("recall")
	tt.Frame()

	text := strings.ToLower(a.notes.Doc.Text().String())
	first := strings.Index(text, "recall")
	second := strings.Index(text[first+1:], "recall")
	if first < 0 || second < 0 {
		t.Fatalf("the note does not hold two matches: %q", text)
	}
	second += first + 1
	wantSelection := func(at int) {
		t.Helper()
		anchor, caret := a.notes.Doc.Selection()
		if anchor != at || caret != at+len("recall") {
			t.Errorf("the selection is %d..%d, want the match at %d", anchor, caret, at)
		}
	}
	wantSelection(first)

	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	wantSelection(second)

	// Two matches only, so Enter again comes back to the first.
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	wantSelection(first)
	if !a.notes.FindOpen {
		t.Error("the bar closed itself")
	}
}

// On a window too narrow for a second column the panel floats over the paper,
// closes with Escape, and hands the keyboard back to the button that opened it —
// in either language.
func TestPanelBecomesAnOverlayWhenNarrow(t *testing.T) {
	for _, locale := range []string{"en", "zh-Hans"} {
		t.Run(locale, func(t *testing.T) {
			msgs := locales.In(locale)
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, 720, 520)
			tt.SetLocale(locale)
			settle(tt)
			notesMode(t, a, tt)
			openNote(t, tt, "Memory and recall")

			if _, ok := tt.Find(msgs.Close()); ok {
				t.Fatal("the panel is up before it was asked for")
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyB)
			tt.Frame()

			panel, ok := tt.Find(msgs.Close())
			if !ok {
				t.Fatalf("the panel did not open; texts %q", tt.Texts())
			}
			// It is a drawer at the window's edge, over the paper rather than in
			// a column of its own: the title runs on under it.
			title, ok := tt.Find(msgs.NoteTitle())
			if !ok {
				t.Fatalf("the paper is missing; texts %q", tt.Texts())
			}
			if right := panel.X + panel.W; right < float32(720)-design.PanelWidth/2 {
				t.Errorf("the drawer ends at %v, not at the window's edge", right)
			}
			if title.X+title.W <= panel.X {
				t.Errorf("the paper ends at %v before the drawer at %v: it is a column, not an overlay",
					title.X+title.W, panel.X)
			}
			for _, want := range []string{msgs.Outline(), msgs.BacklinksTab(0)} {
				if !tt.HasText(want) {
					t.Errorf("the panel does not show %q; texts %q", want, tt.Texts())
				}
			}

			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			if _, ok := tt.Find(msgs.Close()); ok {
				t.Errorf("Escape left the panel up; texts %q", tt.Texts())
			}
			if !tt.Focused(msgs.PanelTitle()) {
				t.Errorf("the focus did not go back to the panel's button; texts %q", tt.Texts())
			}
			if a.notes.PanelVisible(720) {
				t.Error("the panel is still counted as up")
			}
		})
	}
}

// The panel beside a wide window is a column of its own, and its rows are the
// note's: the outline, or the notes that point at it.
func TestPanelTakesAColumnOnAWideWindow(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")

	panel, ok := tt.Find(msgs.Close())
	if !ok {
		t.Fatalf("the panel is missing; texts %q", tt.Texts())
	}
	doc, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the document is missing; texts %q", tt.Texts())
	}
	if panel.X < doc.X+doc.W {
		t.Errorf("the panel starts at %v, over the paper ending at %v", panel.X, doc.X+doc.W)
	}
	if want := design.PanelWidth; panel.W > want+1 {
		t.Errorf("the panel is %v wide, want at most its %v-rail", panel.W, want)
	}
	if !a.notes.PanelVisible(1024) {
		t.Error("the panel is a column but the top bar says it is down")
	}
}

// A paragraph whose card stopped following it says so in its badge: the number
// takes a `!`, and the words for it are on the badge's description, so the shape
// and the colour never carry the meaning alone.
func TestGutterBadgeSaysWhenACardStoppedFollowing(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	notesMode(t, a, tt)
	// The seeded library froze "Writing cards"' first card at old words.
	openNote(t, tt, "Writing cards")
	tt.Frame()

	if !tt.HasText("!1") {
		t.Fatalf("the frozen card's badge does not say it stopped following; texts %q", tt.Texts())
	}
	badge, ok := tt.Find(msgs.CardOnParagraph(1, 1))
	if !ok {
		t.Fatalf("the paragraph has no badge; texts %q", tt.Texts())
	}
	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the document is missing; texts %q", tt.Texts())
	}
	if badge.X >= body.X {
		t.Errorf("the badge at %v is not in the paper's gutter, left of the writing at %v", badge.X, body.X)
	}
}
