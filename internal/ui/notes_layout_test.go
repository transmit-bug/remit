package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// documentOnTheNotebook opens the document on the seeded library and returns
// the tester: the write area's notes page, and a note entered from it.
func documentOnTheNotebook(t *testing.T, w, h int) (*App, *ui.Tester, testLibrary) {
	t.Helper()
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, w, h)
	settle(tt)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")
	return a, tt, seed
}

// A document is the paper, and the panel only where the stage holds it: the
// library's notes are a page of their own, never a column beside the writing. At
// either size the page is one note, and the notes it was opened from are not on
// it.
func TestDocumentHasNoNotesColumn(t *testing.T) {
	for _, locale := range []string{"en", "zh-Hans"} {
		msgs := locales.In(locale)
		for _, size := range windowSizes {
			t.Run(locale+"/"+size.name, func(t *testing.T) {
				a, _ := newTestApp(t)
				tt := ui.NewTester(a.View, size.w, size.h)
				tt.SetLocale(locale)
				settle(tt)
				notesMode(t, a, tt)
				openNote(t, tt, "Memory and recall")

				if _, ok := tt.Find(msgs.NoteBody()); !ok {
					t.Fatalf("the document is missing; texts %q", tt.Texts())
				}
				// The list of notes is its own page: no other note is on the
				// document, and the tree the column was is not drawn.
				for _, other := range []string{"Writing cards", "A daily habit"} {
					if tt.HasText(other) {
						t.Errorf("the document lists the note %q; texts %q", other, tt.Texts())
					}
				}
				if _, ok := tt.AccessByRole(ui.RoleTree, msgs.ModeNotes()); ok {
					t.Errorf("the document draws a notes column; texts %q", tt.Texts())
				}
			})
		}
	}
}

// The paper is the document's whole width of writing: at 1024 it keeps its 760
// DIP measure, centred in the stage, and the panel that opens over it takes no
// column from it.
func TestPaperKeepsItsMeasureAt1024(t *testing.T) {
	msgs := locales.In("en")
	a, tt, _ := documentOnTheNotebook(t, 1024, 720)

	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the paper is missing; texts %q", tt.Texts())
	}
	if want := design.PaperWidth - 2*design.Gutter; body.W > want+1 || body.W < want-1 {
		t.Errorf("the paper's text is %v wide, want the measure's %v", body.W, want)
	}
	// Centred in the stage the frame left it, with the rail to its left.
	stage := a.stageWidth(1024)
	left := body.X - shellRailWidth(1024, a)
	right := stage - (body.X + body.W - shellRailWidth(1024, a))
	if diff := left - right; diff > 2 || diff < -2 {
		t.Errorf("the paper leaves %v on its left and %v on its right", left, right)
	}
	title, ok := tt.Find(msgs.NoteTitle())
	if !ok {
		t.Fatalf("the note's title is missing; texts %q", tt.Texts())
	}
	if title.X != body.X {
		t.Errorf("the title starts at %v and the text at %v: the paper's edge is not shared", title.X, body.X)
	}

	// The panel over the paper is an overlay: the writing keeps every DIP.
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	tt.Frame()
	again, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the document left the screen; texts %q", tt.Texts())
	}
	if again.W != body.W || again.X != body.X {
		t.Errorf("the panel took the paper from %+v to %+v", body, again)
	}
}

// shellRailWidth is the rail's width in the window this frame: the stage's room
// is the window less it.
func shellRailWidth(w float32, a *App) float32 {
	return w - a.stageWidth(w)
}

// panelUp is whether the note's panel shows in this window, as the top bar's
// button reads it.
func panelUp(a *App, w int) bool {
	return a.notes.PanelVisible(design.NewTokens(design.Theme(false)), a.stageWidth(float32(w)))
}

// At 1024 the stage holds neither the paper's measure nor the panel: the panel
// floats over the paper at the window's edge, closes with Escape, and hands the
// keyboard back to the button that opened it.
func TestPanelBecomesAnOverlayAt1024(t *testing.T) {
	msgs := locales.In("en")
	a, tt, _ := documentOnTheNotebook(t, 1024, 720)

	if _, ok := tt.Find(msgs.Close()); ok {
		t.Fatalf("the panel is a column before it was asked for; texts %q", tt.Texts())
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	tt.Frame()

	panel, ok := tt.Find(msgs.Close())
	if !ok {
		t.Fatalf("the panel did not open; texts %q", tt.Texts())
	}
	// It is a drawer at the window's edge, over the paper rather than in a
	// column of its own: the title runs on under it.
	drawer := float32(1024) - design.PanelWidth
	if panel.X < drawer-1 {
		t.Errorf("the drawer's own controls start at %v, left of its %v edge", panel.X, drawer)
	}
	if right := panel.X + panel.W; right < float32(1024)-design.PanelWidth/2 {
		t.Errorf("the drawer ends at %v, not at the window's edge", right)
	}
	title, ok := tt.Find(msgs.NoteTitle())
	if !ok {
		t.Fatalf("the paper is missing; texts %q", tt.Texts())
	}
	if title.X+title.W <= drawer {
		t.Errorf("the paper ends at %v before the drawer at %v: it is a column, not an overlay",
			title.X+title.W, drawer)
	}
	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the paper's text is missing; texts %q", tt.Texts())
	}
	if want := design.PaperWidth - 2*design.Gutter; body.W > want+1 || body.W < want-1 {
		t.Errorf("the open drawer squeezed the paper to %v, want its measure's %v", body.W, want)
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
	if panelUp(a, 1024) {
		t.Error("the panel is still counted as up")
	}
}

// ⌘⇧P is the panel's key, and the help lists the keys the window answers: the
// chord it opens and closes with here is the chord the keyboard map shows.
func TestPanelShortcutIsCmdShiftP(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, tt, _ := documentOnTheNotebook(t, size.w, size.h)

			tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
			tt.Frame()
			if _, ok := tt.Find(msgs.Close()); !ok {
				t.Fatalf("⌘⇧P did not open the panel; texts %q", tt.Texts())
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
			tt.Frame()
			if _, ok := tt.Find(msgs.Close()); ok {
				t.Fatalf("⌘⇧P did not close the panel; texts %q", tt.Texts())
			}
			// The panel's button says what the key did, so the two agree.
			if panelUp(a, size.w) {
				t.Error("the top bar says the panel is up after the key closed it")
			}
		})
	}

	t.Run("help", func(t *testing.T) {
		_, tt, _ := documentOnTheNotebook(t, 1024, 720)
		tt.Key(ui.Cmd, ui.KeySlash)
		tt.Advance(320 * time.Millisecond)
		chord := design.Chord(design.KeyShift, design.KeyCmd, "P")
		if !tt.HasText(chord) {
			t.Fatalf("the help does not list %s; texts %q", chord, tt.Texts())
		}
		if !tt.HasText(msgs.KeysPanel()) {
			t.Errorf("the help does not say what %s does; texts %q", chord, tt.Texts())
		}
		if !tt.HasText(msgs.KeysOutline()) || !tt.HasText(msgs.KeysBacklinks()) {
			t.Errorf("an alias of the panel left the help; texts %q", tt.Texts())
		}
	})

	t.Run("settings", func(t *testing.T) {
		a, tt, _ := documentOnTheNotebook(t, 1024, 720)
		settingsPage(t, tt, a, subSettingsKeys)
		if !tt.HasText(msgs.SettingsKeysPanel()) {
			t.Fatalf("the keys page does not list the panel; texts %q", tt.Texts())
		}
	})
}

// At 760 and below the block gutter has no room beside the paper: the current
// paragraph's badge and handles stand in a compact row above the writing, inside
// the paper, and above it the 48 DIP gutter is what draws them.
func TestGutterMovesAboveTheParagraphAt760(t *testing.T) {
	for _, locale := range []string{"en", "zh-Hans"} {
		msgs := locales.In(locale)
		t.Run(locale, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, 720, 520)
			tt.SetLocale(locale)
			settle(tt)
			notesMode(t, a, tt)
			openNote(t, tt, "Memory and recall")
			// The compact row is the caret's paragraph's, so the caret goes to
			// the paragraph that feeds a card.
			a.notes.Doc.SetSelection(0, 0)
			tt.Frame()

			body, ok := tt.Find(msgs.NoteBody())
			if !ok {
				t.Fatalf("the paper is missing; texts %q", tt.Texts())
			}
			badge, ok := tt.Find(msgs.CardOnParagraph(1, 1))
			if !ok {
				t.Fatalf("the paragraph has no badge; texts %q", tt.Texts())
			}
			add, ok := tt.Find(msgs.AddBlockBelow())
			if !ok {
				t.Fatalf("the paragraph has no handle; texts %q", tt.Texts())
			}
			for _, control := range []struct {
				name string
				box  ui.Rect
			}{{"badge", badge}, {"handle", add}} {
				if control.box.X < 0 || control.box.X+control.box.W > 720 {
					t.Errorf("the %s at %v..%v leaves the window", control.name,
						control.box.X, control.box.X+control.box.W)
				}
				// Above the writing, not beside it: nothing covers the text.
				if control.box.Y+control.box.H > body.Y+1 {
					t.Errorf("the %s ending at %v covers the writing starting at %v",
						control.name, control.box.Y+control.box.H, body.Y)
				}
				if control.box.X < body.X-1 {
					t.Errorf("the %s at %v is left of the paper at %v: the gutter is still drawn",
						control.name, control.box.X, body.X)
				}
			}
			// The window scrolls neither way: the paper, its controls included,
			// sits inside the width it was given.
			if body.X+body.W > float32(720) {
				t.Errorf("the paper ends at %v, past the window's 720", body.X+body.W)
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}

	t.Run("gutter above", func(t *testing.T) {
		msgs := locales.In("en")
		a, tt, _ := documentOnTheNotebook(t, 1024, 720)
		a.notes.Doc.SetSelection(0, 0)
		tt.Frame()
		body, ok := tt.Find(msgs.NoteBody())
		if !ok {
			t.Fatalf("the paper is missing; texts %q", tt.Texts())
		}
		badge, ok := tt.Find(msgs.CardOnParagraph(1, 1))
		if !ok {
			t.Fatalf("the paragraph has no badge; texts %q", tt.Texts())
		}
		if badge.X >= body.X {
			t.Errorf("the badge at %v is not in the paper's gutter, left of the writing at %v", badge.X, body.X)
		}
		if want := design.Gutter; body.X-badge.X-badge.W > want {
			t.Errorf("the badge hangs %v from the writing, past the %v gutter", body.X-badge.X-badge.W, want)
		}
	})
}

// A document is a detail state of the write area, not a seventh page of it: the
// sub-navigation has the area's six pages and never a document tab, and it goes
// while a note is open, leaving the breadcrumb and the rail to say where the
// user is and how to leave.
func TestDocumentIsNotASubnavTab(t *testing.T) {
	msgs := locales.In("en")
	a, tt, _ := documentOnTheNotebook(t, 1024, 720)

	// The note was entered from the notes page, and leaving it goes back there.
	if err := crumbTo(t, tt, msgs.AreaWrite()); err != nil {
		t.Fatalf("the breadcrumb does not lead back to the area: %v", err)
	}
	tt.Frame()
	if a.area != areaWrite || a.sub != subNoteList {
		t.Fatalf("the window is on %v/%v, want the notes page", a.area, a.sub)
	}
	if _, ok := tt.Find(msgs.DocumentPage()); ok {
		t.Errorf("the sub-navigation offers a document tab; texts %q", tt.Texts())
	}
	for _, page := range []string{
		msgs.ModeNotes(), msgs.SearchPage(), msgs.WriteTagsTitle(),
		msgs.WriteAssetsTitle(), msgs.WriteTemplatesTitle(), msgs.WriteTreeTitle(),
	} {
		if !tt.HasText(page) {
			t.Errorf("the sub-navigation lost the page %q; texts %q", page, tt.Texts())
		}
	}

	// A note opens into the document, which takes the sub-navigation's row.
	openNote(t, tt, "Memory and recall")
	if a.sub != subNotes {
		t.Fatalf("opening a note left the window on %v", a.sub)
	}
	if !tt.HasText(msgs.DocumentPage()) {
		t.Errorf("the breadcrumb does not name the document; texts %q", tt.Texts())
	}
	for _, page := range []string{msgs.SearchPage(), msgs.WriteTagsTitle(), msgs.WriteAssetsTitle()} {
		if _, ok := tt.Find(page); ok {
			t.Errorf("the document still shows the page %q; texts %q", page, tt.Texts())
		}
	}
	if !tt.HasText(msgs.AreaNav()) {
		t.Errorf("the rail went with the sub-navigation; texts %q", tt.Texts())
	}

	// The breadcrumb goes back to the notes page, and its row comes back.
	if err := crumbTo(t, tt, msgs.AreaWrite()); err != nil {
		t.Fatalf("the breadcrumb does not leave the document: %v", err)
	}
	tt.Frame()
	if _, ok := tt.Find(msgs.SearchPage()); a.sub != subNoteList || !ok {
		t.Errorf("the breadcrumb did not return to the notes page: %v, texts %q", a.sub, tt.Texts())
	}
}

// crumbTo chooses one segment of the path in the top bar, which is a link and
// never the current page's own name.
func crumbTo(t *testing.T, tt *ui.Tester, name string) error {
	t.Helper()
	info, ok := tt.AccessByRole(ui.RoleLink, name)
	if !ok {
		return os.ErrNotExist
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	return nil
}

// A note reached from a narrow window is still the document: the list it came
// from is gone, the panel is down, and the writing is typeable and saved.
func TestNarrowWindowDropsThePanel(t *testing.T) {
	msgs := locales.In("en")
	a, tt, seed := documentOnTheNotebook(t, 720, 520)

	if _, ok := tt.Find(msgs.Close()); ok {
		t.Error("the panel is open on the smallest window, over the writing")
	}
	if _, ok := tt.Find(msgs.NoteBody()); !ok {
		t.Fatalf("the document is missing; texts %q", tt.Texts())
	}
	if tt.HasText("Writing cards") {
		t.Errorf("the notes list is still beside the paper; texts %q", tt.Texts())
	}

	// The document keeps its room: typing reaches the file.
	if err := tt.Click("Note"); err != nil {
		t.Fatalf("the document is not typeable: %v", err)
	}
	tt.Type("Still writable. ")
	tt.Advance(320 * time.Millisecond)
	source, err := os.ReadFile(filepath.Join(seed.dir, "notes", "memory-and-recall-"+string(seed.memory.ID)+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "Still writable.") {
		t.Errorf("the edit did not reach the file:\n%s", source)
	}
	if a.notes.Draft.ID != seed.memory.ID {
		t.Errorf("the document holds %q, want the note opened from the list", a.notes.Draft.ID)
	}
}

// The document is elastic: the paper takes the height the window gives it, not
// the height its content happens to have. A window taller than the content used
// to leave the writing stopping halfway down.
func TestTheDocumentFillsTheWindowHeight(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range []struct {
		name string
		w, h int
	}{
		{"default", 1024, 720},
		{"tall", 1024, 1400},
		{"wide", 1600, 1000},
	} {
		t.Run(size.name, func(t *testing.T) {
			_, tt, _ := documentOnTheNotebook(t, size.w, size.h)
			assertDocumentFills(t, tt, msgs, size.h)
		})
	}

	t.Run("resized", func(t *testing.T) {
		_, tt, _ := documentOnTheNotebook(t, 1024, 720)
		tt.SetSize(1600, 1000)
		tt.Frame()
		assertDocumentFills(t, tt, msgs, 1000)
	})
}

// assertDocumentFills checks the document reaches the window's bottom edge,
// allowing for the paper's own padding and the status line under it.
func assertDocumentFills(t *testing.T, tt *ui.Tester, msgs locales.Messages, h int) {
	t.Helper()
	note, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the document is missing; texts %q", tt.Texts())
	}
	// Under the document are the status line and the paper's own padding: at
	// most 96 DIPs from the window's bottom edge.
	if bottom := note.Y + note.H; bottom < float32(h)-96 {
		t.Errorf("the document ends at %v, want it to take the %v-high window it is given", bottom, h)
	}
}
