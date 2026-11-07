package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/locales"
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

// viewMode is how the notebook shows the note: written in, written beside the
// same note as it reads, or read whole.
type viewMode uint8

const (
	viewEdit viewMode = iota
	viewSplit
	viewRead
)

// notesView is editing mode: the notes on the left, and on the right the note
// written in, read beside its own source, or read whole. Each edge is a pane the
// user drags — a split remembers the width it was given — and a narrow window
// drops the note's panel so that the document keeps room to be typed.
func (a *App) notesView(c *ui.Context, tk tokens) {
	w, _ := c.Size()
	ui.Split(c, &a.notesWidth, func() {
		a.notesList(c, tk)
	}, func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			switch a.view {
			case viewRead:
				a.readView(c, tk)
			case viewSplit:
				ui.SplitPercent(c, &a.readShare, func() {
					a.editorView(c, tk)
				}, func() {
					a.secondPane(c, tk)
				}).SplitReset(defaultReadShare).Fill()
			default:
				if !a.panelShown(w) {
					a.editorView(c, tk)
					return
				}
				ui.SplitPercent(c, &a.editorShare, func() {
					a.editorView(c, tk)
				}, func() {
					a.notePanel(c, tk)
				}).SplitReset(defaultEditorShare).Fill()
			}
		})
	}).SplitReset(defaultNotesWidth).Fill()
}

// newNote adds an empty note, selects it and puts the caret in its first
// block. The note exists on disk before it is shown, so what the user types
// next has somewhere to go.
func (a *App) newNote(c *ui.Context) {
	if a.lib == nil {
		return
	}
	note, err := a.lib.SaveNote(core.Note{
		ID:     core.NewID("note"),
		Title:  locales.UntitledNote(),
		Blocks: []core.Block{{ID: core.NewID("block")}},
	})
	if err != nil {
		a.report(c, "new note", err)
		return
	}
	a.reload()
	a.editor.selected = string(note.ID)
	a.editor.focus = true
	a.setMode(modeNotes)
}

// dailyTitle is a day's note title: the date, which is also what its file is
// slugged from, so the day's note is found again by name.
func dailyTitle(now time.Time) string { return now.Format("2006-01-02") }

// openDailyNote shows today's note, writing it the first time the day is opened.
// It joins the library like any other note: no separate store, no separate
// format.
func (a *App) openDailyNote(c *ui.Context) {
	if a.lib == nil {
		return
	}
	title := dailyTitle(time.Now())
	for _, note := range a.data.notes {
		if note.Title == title {
			a.editor.selected = string(note.ID)
			a.setMode(modeNotes)
			return
		}
	}
	note, err := a.lib.SaveNote(core.Note{
		ID:     core.NewID("note"),
		Title:  title,
		Blocks: []core.Block{{ID: core.NewID(core.BlockKind)}},
	})
	if err != nil {
		a.report(c, "open today's note", err)
		return
	}
	a.reload()
	a.editor.selected = string(note.ID)
	a.editor.focus = true
	a.setMode(modeNotes)
}

// editNote opens the note a card's block belongs to. A lost card has no note
// to open, and leaves the window where it is.
func (a *App) editNote(card core.Card) {
	note, ok := a.data.noteOfBlock(card.Ref.Block)
	if !ok {
		return
	}
	a.editor.selected = string(note)
	a.setMode(modeNotes)
}
