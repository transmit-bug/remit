package notes

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// notePanel is the note's second column: its outline or the notes that point at
// it, on one switch, with the control that puts the panel away. It is a rail at
// PanelWidth, giving way to the paper when the window is narrow — the paper is
// where the user writes, and the panel is what they consult.
func (s *State) notePanel(c *ui.Context, tk design.Tokens, d Deps) {
	ui.Column(c).Width(tk.PanelWidth).Shrink(0).FillHeight().Children(func() {
		s.panelBody(c, tk, d)
	})
}

// panelBody builds the panel's content, so that the column and the overlay a
// narrow window shows are the same panel.
func (s *State) panelBody(c *ui.Context, tk design.Tokens, d Deps) {
	msgs := locales.In(c.Locale())
	links := d.Backlinks()
	body := ui.Column(c).Grow(1).MinWidth(0).Padding(tk.SpaceM).Gap(tk.SpaceM)
	body.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
			chosen := int(s.panelTab)
			// The tabs take the focus as the panel opens, so the keyboard works
			// the panel at once; it is asked for once, not every frame it shows.
			tabs := ui.Segmented(c, &chosen, msgs.Outline(), msgs.BacklinksTab(len(links))).
				Label(msgs.PanelTitle()).Bind(&s.panelFocus)
			if s.panelWant {
				s.panelFocus.Focus(c)
				s.panelWant = false
			}
			if tabs.Changed() {
				s.panelTab = PanelTab(chosen)
			}
			ui.Spacer(c)
			ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
				Children(func() { tk.Icon(c, "x", design.IconSmall) }).
				OnClick(func() { s.closePanel(c) })
		})

		if s.panelTab == PanelBacklinks {
			s.backlinkList(c, tk, msgs, links, d)
			return
		}
		s.outlineList(c, tk, body)
	})
}

// backlinkList shows the notes that name this one, with the paragraph that
// names it and what that paragraph holds: choosing a row opens that note on that
// paragraph, which is the way back to where the reference was written.
func (s *State) backlinkList(c *ui.Context, tk design.Tokens, msgs locales.Messages, links []Backlink, d Deps) {
	if len(links) == 0 {
		ui.Text(c, msgs.BacklinksEmpty()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		return
	}
	s.backlinks.Key = func(row int) any { return links[row].Block }
	s.backlinks.Label = func(row int) string { return msgs.BacklinkAt(links[row].NoteTitle, links[row].Paragraph) }
	s.backlinks.Selected = &s.backlinkChose
	list := ui.List(c, &s.backlinks, len(links), func(row int) {
		backlinkRow(c, tk, msgs, links[row])
	}).Grow(1).Label(msgs.Backlinks())
	// A choice opens the note that points here, on the paragraph that does;
	// Enter or a double click on the row chosen opens it again.
	if list.Changed() || list.Submitted() {
		if row := s.backlinkChose; row >= 0 && row < len(links) {
			link := links[row]
			s.Selected = string(link.Note)
			s.jumpTo = link.Paragraph
			d.Reload()
		}
	}
}

// backlinkRow is one note pointing here: its title, the line that opens it, and
// what its paragraph carries — the cards it feeds, and whether it can still be
// excerpted.
func backlinkRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, link Backlink) {
	face := kit.HoverFace(c, ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(tk.SpaceS).
		Padding(tk.SpaceXS, tk.SpaceS).Radius(tk.RadiusControl))
	if face.Hovered() {
		face.Background(tk.SurfaceHover)
	}
	face.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, msgs.BacklinkAt(link.NoteTitle, link.Paragraph)).
				FontSize(tk.TypeUI).TextColor(tk.Text).SingleLine()
			ui.Text(c, link.Text).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
			meta := msgs.CardCount(link.Cards)
			if link.Excerptable {
				meta = msgs.BacklinkMeta(link.Cards)
			}
			ui.Text(c, meta).FontSize(tk.TypeLabel).TextColor(tk.TextFaint).SingleLine()
		})
	})
}
