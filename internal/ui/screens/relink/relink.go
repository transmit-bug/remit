// Package relink is the picker a lost card is pointed at another block with:
// the blocks the notes hold, narrowed by a search. It is an overlay over the
// window; the candidates are the app's — derived from its snapshot — and the
// choice goes back to the app that writes the card.
package relink

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// panelWidth is the picker's panel: the palette's width, since the two are read
// the same way.
const panelWidth float32 = 480

// panelHeight is the picker's height: enough rows to scan, short enough that it
// never covers the window whole.
const panelHeight float32 = 360

// Row is one block a lost card can be pointed at: the text it would show, and
// the note that holds it.
type Row struct {
	Block core.ID
	Text  string
	Note  string
}

// State is the picker's session: whether it is open, the card it repairs, the
// query the user typed, and the candidates the app derived.
type State struct {
	open     bool
	card     core.ID
	query    string
	selected int
	field    ui.Handle
	rows     []Row
}

// Open shows the picker for a lost card, with the query cleared, the candidates
// the app derived, and the focus asked for before the dialog is built, as the
// palette does.
func (s *State) Open(c *ui.Context, card core.ID, rows []Row) {
	s.open, s.card, s.query, s.selected, s.rows = true, card, "", 0, rows
	s.field.Focus(c)
}

// Close shuts the picker.
func (s *State) Close() { s.open = false }

// Card is the card the picker repairs.
func (s *State) Card() core.ID { return s.card }

// move moves the selection within n rows.
func (s *State) move(n, delta int) {
	if n == 0 {
		return
	}
	s.selected = max(0, min(s.selected+delta, n-1))
}

// Deps is what the picker asks of the app: the blocks a query finds, the count
// its arrival replays by, and the block the user chose. Each runs on the UI
// thread; the app writes the card.
type Deps struct {
	// Candidates are the blocks the notes hold, narrowed by query.
	Candidates func(string) []Row
	// Choose points the card at the block chosen.
	Choose func(core.ID)
	// Run is the count the picker's arrival replays by.
	Run uint64
}

// View draws the picker over the window. DialogBase keeps the keyboard inside
// it; Escape closes it and gives the focus back where it was.
func View(c *ui.Context, s *State, d Deps) {
	if !s.open {
		return
	}
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	reload, choice := false, -1

	ui.DialogBase(c, &s.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(panelWidth).MaxHeight(panelHeight).Padding(tk.SpaceL).Gap(tk.SpaceM))
		panel.Label(msgs.RelinkTitle())
		// The picker arrives the way the palette does.
		arriving := panel.Keyframes("arrive", design.PanelFrames, ui.KeyframeOptions{Run: d.Run})
		scale := design.PanelScale + (1-design.PanelScale)*arriving
		panel.Opacity(arriving).Scale(scale, scale)
		backdrop.Opacity(arriving)

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, msgs.RelinkTitle()).FontSize(tk.TypeH2).Bold().Grow(1)
				// An icon-only button carries its name for the pointer and for a
				// screen reader, since the drawing says nothing on its own.
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { s.open = false })
			})
			field := ui.TextInput(c, &s.query).
				Placeholder(msgs.RelinkSearch()).Bind(&s.field).
				Label(msgs.RelinkSearch()).MinHeight(tk.Control)
			if field.Changed() {
				s.selected = 0
				reload = true
			}
			if field.Submitted() {
				choice = s.selected
			}
			c.OnShortcut(0, ui.KeyDown, func() { s.move(len(s.rows), 1) })
			c.OnShortcut(0, ui.KeyUp, func() { s.move(len(s.rows), -1) })

			if len(s.rows) == 0 {
				ui.Text(c, msgs.NoBlocks()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(tk.SpaceXS).Children(func() {
					for i, row := range s.rows {
						rowView(c, tk, row, i == s.selected, func() { d.Choose(row.Block) })
					}
				})
			})
		})
	})

	// The effects of this pass, once its field has taken what was typed.
	if reload {
		s.rows = d.Candidates(s.query)
		c.Invalidate()
	}
	if choice >= 0 && choice < len(s.rows) {
		d.Choose(s.rows[choice].Block)
	}
}

// rowView draws one candidate: the text the card would show, and the note it
// lives in. A click chooses it there and then, as the palette's rows run their
// action.
func rowView(c *ui.Context, tk design.Tokens, row Row, chosen bool, choose func()) {
	item := ui.ButtonBase(c.Key(row.Block)).Gap(tk.SpaceXS).Justify(ui.Start).
		MinHeight(tk.Control).Padding(tk.SpaceXS, tk.SpaceS).
		Radius(tk.RadiusControl).Label(row.Text)
	if chosen {
		item.Background(tk.Sunken)
	}
	item.OnClick(choose)
	item.Children(func() {
		ui.Column(c).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, row.Text).FontSize(tk.TypeUI).SingleLine()
			ui.Text(c, row.Note).FontSize(tk.TypeUI).TextColor(tk.TextMuted).SingleLine()
		})
	})
}
