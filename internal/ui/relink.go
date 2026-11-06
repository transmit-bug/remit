package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// relinkPanel is the picker's height: enough rows to scan, short enough that
// it never covers the window whole.
const relinkPanel float32 = 360

// relinkState is the picker a lost card is pointed at another block with: the
// card, the query the user typed, and the blocks the notes snapshot holds.
// The rows are derived when the picker opens or the query changes, never in
// the pass that draws them.
type relinkState struct {
	open     bool
	card     core.ID
	query    string
	selected int
	field    ui.Handle
	rows     []relinkRow
}

// relinkRow is one block a lost card can be pointed at: the text it would
// show, and the note that holds it.
type relinkRow struct {
	block core.ID
	text  string
	note  string
}

// relinkRows derives the picker's rows from the notes snapshot: every block
// of every note, narrowed by query over the note's title and the block's text.
func (a *App) relinkRows(query string) []relinkRow {
	q := strings.ToLower(strings.TrimSpace(query))
	var rows []relinkRow
	for _, note := range a.data.notes {
		for _, block := range note.Blocks {
			if block.ID == "" {
				continue
			}
			text := strings.Join(strings.Fields(block.Text), " ")
			if q != "" && !strings.Contains(strings.ToLower(note.Title+" "+text), q) {
				continue
			}
			rows = append(rows, relinkRow{block: block.ID, text: text, note: note.Title})
		}
	}
	return rows
}

// openRelink shows the picker for a lost card, with the query cleared and the
// focus asked for before the dialog is built, as the palette does.
func (a *App) openRelink(c *ui.Context, card core.ID) {
	a.anim.relink++
	a.relink.open = true
	a.relink.card = card
	a.relink.query = ""
	a.relink.selected = 0
	a.relink.rows = a.relinkRows("")
	a.relink.field.Focus(c)
}

// relinkView draws the picker over the window. DialogBase keeps the keyboard
// inside it; Escape closes it and gives the focus back where it was.
func (a *App) relinkView(c *ui.Context, tk tokens) {
	if !a.relink.open {
		return
	}
	msgs := m(c)
	sel := clamp(a.relink.selected, 0, len(a.relink.rows)-1)
	reload, choice := false, -1

	ui.DialogBase(c, &a.relink.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.scrim)
		panel = kit.StylePanel(c, panel.Width(paletteWidth).MaxHeight(relinkPanel).Padding(tk.spaceL).Gap(tk.spaceM))
		panel.Label(msgs.RelinkTitle())
		// The picker arrives the way the palette does.
		arriving := panel.Keyframes("arrive", design.PanelFrames, ui.KeyframeOptions{Run: a.anim.relink})
		scale := design.PanelScale + (1-design.PanelScale)*arriving
		panel.Opacity(arriving).Scale(scale, scale)
		backdrop.Opacity(arriving)

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.spaceS).Children(func() {
				ui.Text(c, msgs.RelinkTitle()).FontSize(tk.typeHeading).Bold().Grow(1)
				// An icon-only button carries its name for the pointer and for a
				// screen reader, since the drawing says nothing on its own.
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.icon(c, "x", iconSmall) }).
					OnClick(func() { a.relink.open = false })
			})
			field := ui.TextInput(c, &a.relink.query).
				Placeholder(msgs.RelinkSearch()).Bind(&a.relink.field).
				Label(msgs.RelinkSearch()).MinHeight(tk.control)
			if field.Changed() {
				a.relink.selected = 0
				reload = true
			}
			if field.Submitted() {
				choice = sel
			}
			c.OnShortcut(0, ui.KeyDown, func() { a.moveRelink(len(a.relink.rows), 1) })
			c.OnShortcut(0, ui.KeyUp, func() { a.moveRelink(len(a.relink.rows), -1) })

			if len(a.relink.rows) == 0 {
				ui.Text(c, msgs.NoBlocks()).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(tk.spaceXS).Children(func() {
					for i, row := range a.relink.rows {
						a.relinkRowView(c, tk, row, i == sel, i)
					}
				})
			})
		})
	})

	// The effects of this pass, once its field has taken what was typed.
	if reload {
		a.relink.rows = a.relinkRows(a.relink.query)
		c.Invalidate()
	}
	if choice >= 0 {
		a.relinkChoose(c, choice)
	}
}

// moveRelink moves the picker's selection within n rows.
func (a *App) moveRelink(n, delta int) {
	if n == 0 {
		return
	}
	a.relink.selected = clamp(a.relink.selected+delta, 0, n-1)
}

// relinkRowView draws one candidate: the text the card would show, and the
// note it lives in. A click chooses it there and then, as the palette's rows
// run their action.
func (a *App) relinkRowView(c *ui.Context, tk tokens, row relinkRow, chosen bool, i int) {
	item := ui.ButtonBase(c).Key(row.block).Gap(tk.spaceXS).Justify(ui.Start).
		MinHeight(tk.control).Padding(tk.spaceXS, tk.spaceS).
		Radius(tk.radiusControl).Label(row.text)
	if chosen {
		item.Background(tk.sunken)
	}
	item.OnClick(func() { a.relinkChoose(c, i) })
	item.Children(func() {
		ui.Column(c).Gap(tk.spaceXS).Children(func() {
			ui.Text(c, row.text).FontSize(tk.typeBody).SingleLine()
			ui.Text(c, row.note).FontSize(tk.typeSmall).TextColor(tk.TextMuted).SingleLine()
		})
	})
}

// relinkChoose points the lost card at another block, live again, and writes
// it through the store. Only the reference changes: the card keeps its status,
// its reviews and its due date, because those live in the card's history and
// not in the block it reads from.
func (a *App) relinkChoose(c *ui.Context, i int) {
	if i < 0 || i >= len(a.relink.rows) || a.lib == nil {
		return
	}
	card, ok := a.data.card(a.relink.card)
	if !ok {
		return
	}
	card.Ref = core.Reference{Block: a.relink.rows[i].block}
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "re-link card", err)
		return
	}
	a.relink.open = false
	a.reload()
	if a.mode == modeReview {
		// The card the user just fixed is what they wanted to read: keep it
		// on screen even though its due date is elsewhere.
		a.review.Show(card.ID)
	}
	c.Toast(m(c).Relinked())
}
