package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// notesView draws the notebook; the screen itself lives in its own package, and
// this hands it the snapshot the app derived and the actions it owns (ADR 0004).
func (a *App) notesView(c *ui.Context) {
	notes.View(c, &a.notes, a.notesDeps())
}

// notesDeps is what the notebook asks of the app: the notes and cards it reads,
// and one function per thing the user can do, each run here on the UI thread.
func (a *App) notesDeps() notes.Deps {
	return notes.Deps{
		Notes:        func() []notes.Note { return a.notesRows },
		Tags:         func() map[string]int { return a.data.tags },
		Backlinks:    func() []notes.Backlink { return a.backlinks },
		Cards:        a.cardGroups,
		Due:          func() int { return a.review.Remaining() },
		Reload:       a.reload,
		OpenReview:   func() { a.setMode(modeReview) },
		NewNote:      a.newNote,
		SaveNote:     a.saveNote,
		SaveDocument: a.saveDocument,
		PutAsset:     a.putAsset,
		GetAsset:     a.getAsset,
		OpenCardEdit: a.openCardEdit,
	}
}

// showPanel opens the note's panel on one of its views from the keyboard,
// switching to the notebook when another screen was up.
func (a *App) showPanel(tab notes.PanelTab) {
	if a.mode != modeNotes {
		a.setMode(modeNotes)
	}
	a.notes.ShowPanel(tab)
}

// noteRows is the snapshot as the notebook's list reads it: each note with how
// many cards it feeds and the tags it carries. It is derived after every write,
// so no frame walks the cards.
func (a *App) noteRows() []notes.Note {
	rows := make([]notes.Note, 0, len(a.data.notes))
	for _, note := range a.data.notes {
		rows = append(rows, notes.Note{
			Note:  note,
			Cards: a.data.cardsInNote(note),
			Tags:  a.data.noteTags[note.ID],
		})
	}
	return rows
}

// saveNote stores a note and reloads what the window reads: a title or a
// document saved, with nothing else to write.
func (a *App) saveNote(c *ui.Context, note core.Note) (core.Note, bool) {
	saved, ok := a.writeNote(c, note)
	if !ok {
		return core.Note{}, false
	}
	a.reload()
	return saved, true
}

// saveDocument stores the note an edited document produced and reconciles the
// cards its paragraphs ask for: typing :: is what makes a paragraph a card, and
// deleting it is what takes the card out of the queue.
func (a *App) saveDocument(c *ui.Context, note core.Note) (core.Note, bool) {
	saved, ok := a.writeNote(c, note)
	if !ok {
		return core.Note{}, false
	}
	a.reload()
	a.reconcileCards(c, saved)
	return saved, true
}

// putAsset stores a picture in the library open now, which a dropped archive
// can replace, so the library is taken at call time.
func (a *App) putAsset(data []byte, ext string) (string, error) {
	return a.lib.PutAsset(data, ext)
}

// getAsset reads a stored asset's bytes; a closed library holds nothing.
func (a *App) getAsset(rel string) ([]byte, bool) {
	if a.lib == nil {
		return nil, false
	}
	data, ok, err := a.lib.Asset(rel)
	if err != nil {
		return nil, false
	}
	return data, ok
}

// writeNote saves a note without reloading, for an action that writes more than
// the note and reloads once at the end. A block read from a hand-written note
// arrives without an ID; minting it here gives a card a source to point at.
func (a *App) writeNote(c *ui.Context, note core.Note) (core.Note, bool) {
	for i := range note.Blocks {
		if note.Blocks[i].ID == "" {
			note.Blocks[i].ID = core.NewID(core.BlockKind)
		}
	}
	saved, err := a.lib.SaveNote(note)
	if err != nil {
		a.report(c, "save note", err)
		return core.Note{}, false
	}
	return saved, true
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
	a.notes.Selected = string(note.ID)
	a.notes.FocusTitle()
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
			a.notes.Selected = string(note.ID)
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
	a.notes.Selected = string(note.ID)
	a.notes.FocusTitle()
	a.setMode(modeNotes)
}

// editNote opens the note a card's block belongs to. A lost card has no note
// to open, and leaves the window where it is.
func (a *App) editNote(card core.Card) {
	note, ok := a.data.noteOfBlock(card.Ref.Block)
	if !ok {
		return
	}
	a.notes.Selected = string(note)
	a.setMode(modeNotes)
}
