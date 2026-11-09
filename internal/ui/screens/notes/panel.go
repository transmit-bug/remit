package notes

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// notePanel is the pane beside the document: the note's outline or the notes
// that point at it, then what the note feeds. It scrolls on its own and takes
// the focus only when one of its rows is chosen.
func (s *State) notePanel(c *ui.Context, tk design.Tokens, d Deps) {
	msgs := locales.In(c.Locale())
	ui.Column(c).Grow(1).MinWidth(0).Padding(tk.SpaceM).Gap(tk.SpaceM).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
			chosen := int(s.panelTab)
			if ui.Segmented(c, &chosen, msgs.Outline(), msgs.Backlinks()).Label(msgs.PanelTitle()).Changed() {
				s.panelTab = PanelTab(chosen)
			}
			ui.Spacer(c)
			ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
				Children(func() { tk.Icon(c, "x", design.IconSmall) }).
				OnClick(func() { s.panel = panelClosed })
		})

		if s.panelTab == PanelBacklinks {
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(tk.SpaceXS).Children(func() {
					s.backlinkList(c, tk, d)
				})
			})
		} else {
			s.outlineList(c, tk)
		}

		s.cardsStrip(c, tk, d)
	})
}

// backlinkList shows the notes that name this one, with the paragraph that names
// it; choosing one opens that note on that paragraph.
func (s *State) backlinkList(c *ui.Context, tk design.Tokens, d Deps) {
	msgs := locales.In(c.Locale())
	links := d.Backlinks()
	if len(links) == 0 {
		ui.Text(c, msgs.BacklinksEmpty()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		return
	}
	for _, link := range links {
		label := msgs.BacklinkAt(link.NoteTitle, link.Paragraph)
		target := link
		ui.ButtonBase(c.Key(string(link.Block))).Justify(ui.Start).
			Padding(tk.SpaceXS, tk.SpaceS).MinHeight(tk.Control - tk.SpaceS).
			Radius(tk.RadiusControl).Label(label).
			OnClick(func() {
				s.Selected = string(target.Note)
				s.jumpTo = target.Paragraph
				d.Reload()
			}).
			Children(func() {
				ui.Column(c).Gap(0).Children(func() {
					ui.Text(c, label).FontSize(tk.TypeUI).TextColor(tk.Text).SingleLine()
					ui.Text(c, target.Text).FontSize(tk.TypeUI).TextColor(tk.TextMuted).SingleLine()
				})
			})
	}
}
