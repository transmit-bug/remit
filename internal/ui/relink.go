package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/relink"
)

// relinkCandidates derives the picker's rows from the notes snapshot: every
// block of every note, narrowed by query over the note's title and the block's
// text.
func (a *App) relinkCandidates(query string) []relink.Row {
	q := strings.ToLower(strings.TrimSpace(query))
	var rows []relink.Row
	for _, note := range a.data.notes {
		for _, block := range note.Blocks {
			if block.ID == "" {
				continue
			}
			text := strings.Join(strings.Fields(block.Text), " ")
			if q != "" && !strings.Contains(strings.ToLower(note.Title+" "+text), q) {
				continue
			}
			rows = append(rows, relink.Row{Block: block.ID, Text: text, Note: note.Title})
		}
	}
	return rows
}

// openRelink shows the picker for a lost card, with the query cleared and the
// candidates the snapshot holds now.
func (a *App) openRelink(c *ui.Context, card core.ID) {
	a.anim.relink++
	a.relink.Open(c, card, a.relinkCandidates(""))
}

// relinkView draws the picker over the window; the picker itself lives in its
// own package, and this hands it what it asks of the app (ADR 0009).
func (a *App) relinkView(c *ui.Context) {
	relink.View(c, &a.relink, a.relinkDeps(c))
}

// relinkDeps is what the picker asks of the app: the blocks a query finds, the
// count its arrival replays by, and the block the user chose, each run here on
// the UI thread.
func (a *App) relinkDeps(c *ui.Context) relink.Deps {
	return relink.Deps{
		Candidates: a.relinkCandidates,
		Choose:     func(block core.ID) { a.relinkChoose(c, block) },
		Run:        a.anim.relink,
	}
}

// relinkChoose points the lost card at another block, live again, and writes
// it through the store. Only the reference changes: the card keeps its status,
// its reviews and its due date, because those live in the card's history and
// not in the block it reads from.
func (a *App) relinkChoose(c *ui.Context, block core.ID) {
	if block == "" || a.lib == nil {
		return
	}
	card, ok := a.data.card(a.relink.Card())
	if !ok {
		return
	}
	card.Ref = core.Reference{Block: block}
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "re-link card", err)
		return
	}
	a.relink.Close()
	a.reload()
	if a.mode == modeReview {
		// The card the user just fixed is what they wanted to read: keep it
		// on screen even though its due date is elsewhere.
		a.review.Show(card.ID)
	}
	c.Toast(m(c).Relinked())
}
