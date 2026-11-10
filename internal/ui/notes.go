package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// notesDeps is what the document asks of the app: the notes and cards it reads,
// and one function per thing the user can do, each run here on the UI thread.
// The stage is the room the frame left the document — the window less the rail —
// which is what the panel rule is computed from.
func (a *App) notesDeps(c *ui.Context) notes.Deps {
	// The stage is this frame's, taken now: the screen keeps its deps across
	// passes, so it holds the room rather than the frame that measured it.
	w, _ := c.Size()
	stage := a.stageWidth(w)
	return notes.Deps{
		Notes:      func() []notes.Note { return a.notesRows },
		Stage:      func() float32 { return stage },
		Backlinks:  func() []notes.Backlink { return a.backlinks },
		Cards:      a.cardGroups,
		Badges:     a.blockBadges,
		Marks:      a.readMarks,
		Inspector:  a.blockInspector,
		Due:        func() int { return a.review.Remaining() },
		Reload:     a.reload,
		OpenReview: func() { a.setMode(modeReview) },
		NewNote:    a.newNote,
		Templates:  a.noteTemplates,
		NewFromTemplate: func(c *ui.Context, id core.ID) {
			a.newNoteFromTemplate(c, id)
		},
		SaveNote:       a.saveNote,
		SaveDocument:   a.saveDocument,
		PutAsset:       a.putAsset,
		GetAsset:       a.getAsset,
		ChooseImage:    a.pickImage,
		OpenCardEdit:   a.openCardEdit,
		OpenCardEditor: a.openCardEditor,
		OpenCardDetail: a.openCardDetail,
		ClozeSelection: a.clozeSelection,
		Excerpt: func(c *ui.Context, note, block core.ID) {
			a.excerptBlock(c, note, block, false)
		},
		LocateOnBoard: a.LocateOnBoard,
		OnBoard:       a.OnBoard,
		BlockItems: func(c *ui.Context) []notes.BlockItem {
			return a.blockMenuItems(c)
		},
		SlashItems: func(c *ui.Context) []notes.BlockItem {
			return a.slotMenuItems(c, a.slashItems())
		},
		RowItems: func(c *ui.Context) []notes.BlockItem {
			return a.slotMenuItems(c, a.sidebarNoteItems())
		},
		RenameNote: a.renameNote,
		DeleteNote: a.deleteNote,

		Rows:      func() ([]notes.Row, int) { return a.write.rows, a.write.total },
		TagRows:   func() []notes.TagRow { return a.write.tags },
		AssetRows: func() []notes.AssetRow { return a.write.assets },
		ImportAssets: func(c *ui.Context) {
			a.importAssets(c)
		},
		SearchResults: func() (notes, cards []notes.Hit, total int) {
			return a.write.hits, a.write.cardHits, a.write.hitsTotal
		},
		Excerpts:     func() []notes.TreeNode { return a.write.tree },
		Now:          time.Now,
		OpenNote:     a.openNote,
		OpenNoteRead: a.openNoteRead,
		QueueReading: a.addReading,
		OpenTag:      a.openTag,
		OpenReadingQueue: func() {
			a.setArea(areaRead, subReadQueue)
		},
		OpenCard: a.openCardSource,
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

// noteRows is the snapshot as the document reads it: each note with how many
// cards it feeds. Note templates are left out — they are starting points, not
// documents; only the list and the notes search hide them (#154). It is derived
// after every write, so no frame walks the cards.
func (a *App) noteRows() []notes.Note {
	rows := make([]notes.Note, 0, len(a.data.notes))
	for _, note := range a.data.notes {
		if core.IsTemplate(note) {
			continue
		}
		rows = append(rows, notes.Note{
			Note:  note,
			Cards: a.data.cardsInNote(note),
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

// noteTemplates lists the library's note templates from the snapshot: the
// notes the notebook's list leaves out, newest save first. No frame touches the
// store.
func (a *App) noteTemplates() []core.Note {
	var out []core.Note
	for _, note := range a.data.notes {
		if core.IsTemplate(note) {
			out = append(out, note)
		}
	}
	return out
}

// newNoteFromTemplate starts a note from a template: the store copies the
// text, field blocks and card-asking paragraphs under fresh IDs, the copy is
// selected like any new note, and the cards its paragraphs ask for are
// reconciled on the first save. The template itself is untouched, so one
// template starts many notes.
func (a *App) newNoteFromTemplate(c *ui.Context, templateID core.ID) {
	if a.lib == nil {
		return
	}
	note, err := a.lib.NoteFromTemplate(templateID, locales.UntitledNote())
	if err != nil {
		a.report(c, "new note from template", err)
		return
	}
	a.reload()
	a.notes.Selected = string(note.ID)
	a.notes.FocusTitle()
	a.setMode(modeNotes)
}

// renameNote writes a note's new title from its row. Nothing moves on disk: the
// note's file was minted with it, so a link or a card still finds it, and the
// cards the note feeds are untouched.
func (a *App) renameNote(c *ui.Context, id core.ID, title string) bool {
	if a.lib == nil {
		return false
	}
	title = strings.TrimSpace(title)
	if title == "" {
		// A note with no name is not a name: the field said nothing, and the
		// note keeps the title it had.
		return false
	}
	if _, err := a.lib.RenameNote(id, title, time.Now()); err != nil {
		a.report(c, "rename note", err)
		return false
	}
	a.reload()
	return true
}

// deleteNote removes a note from the library. The cards its paragraphs fed stay
// where they are — lost, with every review they were given — which is the
// re-link page's work, and the note the editor held open is dropped so a frame
// never saves it back.
func (a *App) deleteNote(c *ui.Context, id core.ID) bool {
	if a.lib == nil {
		return false
	}
	if err := a.lib.DeleteNote(id, time.Now(), store.CascadeLose); err != nil {
		a.report(c, "delete note", err)
		return false
	}
	if a.notes.Loaded == string(id) {
		a.notes.Forget()
	}
	a.reload()
	c.Toast(m(c).NoteDeleted())
	return true
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

// blockBadges is the paragraph gutter's badges: how many cards each paragraph
// of the note feeds, and the state the badge's shape draws. A suspended card
// feeds nothing, so it is not counted; a frozen one keeps its words while the
// paragraph moves on, which is the `!` the badge shows.
func (a *App) blockBadges(note core.Note) []notes.Badge {
	byBlock := make(map[core.ID][]core.Card, len(note.Blocks))
	for _, card := range a.data.cards {
		if card.Status == core.StatusSuspended {
			continue
		}
		byBlock[card.Ref.Block] = append(byBlock[card.Ref.Block], card)
	}
	due := make(map[core.ID]bool, len(a.due))
	for _, id := range a.due {
		due[id] = true
	}
	var out []notes.Badge
	for i, block := range note.Blocks {
		cards := byBlock[block.ID]
		if len(cards) == 0 {
			continue
		}
		out = append(out, notes.Badge{
			Paragraph: i + 1,
			Cards:     len(cards),
			State:     badgeState(cards, due),
		})
	}
	return out
}

// badgeState is the shape one paragraph's badge draws: a card that stopped
// following it first, then whether every card is new, whether one is due now,
// and otherwise that they come later.
func badgeState(cards []core.Card, due map[core.ID]bool) notes.BadgeState {
	fresh, now := true, false
	for _, card := range cards {
		if card.Ref.Frozen != nil {
			return notes.BadgeLost
		}
		if card.Status != core.StatusNew {
			fresh = false
		}
		if due[card.ID] {
			now = true
		}
	}
	switch {
	case fresh:
		return notes.BadgeNew
	case now:
		return notes.BadgeDue
	}
	return notes.BadgeLater
}

// readMarks is what read mode says beside each paragraph: the cards it feeds,
// whether something already quotes it, and whether a reading session stopped
// there. It is derived from the snapshot, so the reading view asks no store.
func (a *App) readMarks(note core.Note) map[core.ID]notes.Mark {
	out := make(map[core.ID]notes.Mark, len(note.Blocks))
	position := a.readingPositions[note.ID]
	for _, block := range note.Blocks {
		mark := notes.Mark{
			Cards:     a.data.cardsForBlock(block.ID),
			Excerpted: a.excerpted[block.ID],
			LastStop:  position != "" && block.ID == position,
		}
		if mark.Cards == 0 && !mark.Excerpted && !mark.LastStop {
			continue
		}
		out[block.ID] = mark
	}
	return out
}

// blockInspector is what the block inspector shows of one paragraph (S20): the
// paragraph, the cards that read it with what they ask now, the notes taken out
// of it, whether a reading session stopped there, and whether a board places it.
// It is derived once, as the dialog opens, so a frame walks nothing for it.
func (a *App) blockInspector(blockID core.ID) (notes.Inspector, bool) {
	note, ok := a.data.noteOfBlock(blockID)
	if !ok {
		return notes.Inspector{}, false
	}
	held, ok := a.data.note(note)
	if !ok {
		return notes.Inspector{}, false
	}
	block, ok := held.Block(blockID)
	if !ok {
		return notes.Inspector{}, false
	}
	out := notes.Inspector{
		Block:           block,
		ReadingPosition: a.readingPositions[note] == blockID,
		OnBoard:         a.OnBoard(blockID),
	}
	for _, card := range a.data.cards {
		if card.Ref.Block != blockID {
			continue
		}
		prompt, _, state := a.faces(card)
		out.Cards = append(out.Cards, notes.InspectorCard{
			Card: card, Title: oneLine(prompt), State: state,
		})
	}
	for _, excerpt := range a.excerptsOf(blockID) {
		out.Excerpts = append(out.Excerpts, excerpt)
	}
	return out, true
}

// excerptsOf lists the notes taken out of one paragraph, newest first: an
// excerpt is a note whose block links back to the paragraph it quotes, which is
// the link the index reads too.
func (a *App) excerptsOf(blockID core.ID) []notes.InspectorExcerpt {
	var out []notes.InspectorExcerpt
	for _, note := range a.data.notes {
		for _, block := range note.Blocks {
			quotes := false
			for _, link := range core.Links(block.Text) {
				if link.Block == blockID {
					quotes = true
					break
				}
			}
			if !quotes {
				continue
			}
			out = append(out, notes.InspectorExcerpt{
				Note: note.ID, Title: note.Title, Text: excerptLine(block.Text),
			})
			break
		}
	}
	return out
}

// excerptLine is an excerpt block's passage without the reference under it: the
// line a reader recognizes the note by.
func excerptLine(text string) string {
	if i := strings.Index(text, "\n"); i >= 0 {
		return oneLine(text[:i])
	}
	return oneLine(text)
}

// clozeSelection is the selection bar's cloze: the words the user selected become
// a {{…}} blank in their paragraph, the note is stored, and the paragraph then
// asks for the cards a blank makes. It answers whether the note was written, so
// the screen keeps the selection when it was not.
func (a *App) clozeSelection(c *ui.Context, noteID, blockID core.ID, selected string) bool {
	if a.lib == nil || noteID == "" || blockID == "" {
		return false
	}
	note, ok := a.data.note(noteID)
	if !ok {
		return false
	}
	block, ok := note.Block(blockID)
	if !ok {
		return false
	}
	rewritten := core.Cloze(block.Text, selected)
	if rewritten == block.Text {
		// The paragraph already holds exactly this blank: nothing to write, and
		// the cards it asks for are already made.
		return false
	}
	for i := range note.Blocks {
		if note.Blocks[i].ID == blockID {
			note.Blocks[i].Text = rewritten
			break
		}
	}
	saved, ok := a.writeNote(c, note)
	if !ok {
		return false
	}
	a.reload()
	a.reconcileCards(c, saved)
	return true
}

// blockMenuItems are the actions plugins contributed to a paragraph's menu and
// its inspector, in drawing order, with the labels the window's language gives
// them (ui.md §4, the note.block slot).
func (a *App) blockMenuItems(c *ui.Context) []notes.BlockItem {
	return a.slotMenuItems(c, a.noteBlockItems())
}

// slotMenuItems turns a slot's items into the labels and actions a screen draws,
// in the window's language: every receiver of a menu slot goes through it, so
// one slot reads the same wherever it lands.
func (a *App) slotMenuItems(c *ui.Context, items []contrib.MenuRef) []notes.BlockItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]notes.BlockItem, 0, len(items))
	for _, item := range items {
		ref := item.CommandRef
		out = append(out, notes.BlockItem{
			Label: a.pluginText(c, ref.Plugin, string(ref.Command.Title)),
			Run:   func(c *ui.Context) { a.runContribution(c, ref) },
		})
	}
	return out
}

// docStatus is the window's status line while a note is up: what the note holds,
// where the reader is in it, and that it is stored, in the smallest type the
// window has. Every other page leaves the line to the frame's own.
func (a *App) docStatus(c *ui.Context) []string {
	if a.area != areaWrite || a.sub != subNotes || a.notes.Draft.ID == "" {
		return nil
	}
	msgs := m(c)
	note := a.notes.Draft
	text := ""
	if a.notes.Doc != nil {
		text = a.notes.Doc.Text().String()
	}
	out := []string{
		msgs.WordCount(len(strings.Fields(text))),
		msgs.ParagraphCount(len(note.Blocks)),
		msgs.CardCount(a.data.cardsInNote(note)),
	}
	// Where the reader is: the caret's paragraph while writing, and the
	// paragraph on screen while reading.
	at := 0
	if a.notes.View == notes.Read {
		at = a.notes.ReadParagraph()
	} else if caret, ok := a.notes.ParagraphAtCaret(); ok {
		at = caret + 1
	}
	if at > 0 {
		out = append(out, msgs.ReadingParagraph(at))
	}
	if tags := a.data.noteTags[note.ID]; len(tags) > 0 {
		out = append(out, "#"+strings.Join(tags, " #"))
	}
	out = append(out, msgs.Saved())
	return out
}

// excerptReadBlock is the read mode's excerpt key: it takes the paragraph on
// screen out into a note of its own, through the same handler the reading
// session and the selection bar use.
func (a *App) excerptReadBlock(c *ui.Context) {
	a.notes.Follow(a.notesRows)
	_, block, ok := a.notes.CurrentReadBlock()
	if !ok {
		return
	}
	a.excerptBlock(c, a.notes.Draft.ID, block.ID, false)
}
