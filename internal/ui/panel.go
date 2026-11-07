package ui

import (
	"github.com/egoist/mygo/ui"
)

// panelMode is what the user said about the note's panel, when they said
// anything: a narrow window closes it until they ask for it.
type panelMode uint8

const (
	panelAuto panelMode = iota
	panelOpen
	panelClosed
)

// panelShown reports whether the note's panel takes a column this frame.
func (a *App) panelShown(w float32) bool {
	switch a.panel {
	case panelOpen:
		return true
	case panelClosed:
		return false
	}
	return w >= narrowWindow
}

// showPanel opens the note's panel on one of its views, from the keyboard.
func (a *App) showPanel(tab panelTab) {
	if a.mode != modeNotes {
		a.setMode(modeNotes)
	}
	a.panel, a.panelTab = panelOpen, tab
}

// togglePanel opens or closes the note's panel, whatever the window's width.
func (a *App) togglePanel(c *ui.Context) {
	w, _ := c.Size()
	if a.panelShown(w) {
		a.panel = panelClosed
		return
	}
	a.panel = panelOpen
}

// panelTab is which of the panel's two views is showing.
type panelTab uint8

const (
	panelOutline panelTab = iota
	panelBacklinks
)

// notePanel is the pane beside the document: the note's outline or the notes
// that point at it, then what the note feeds. It scrolls on its own and takes
// the focus only when one of its rows is chosen.
func (a *App) notePanel(c *ui.Context, tk tokens) {
	msgs := m(c)
	ui.Column(c).Grow(1).MinWidth(0).Padding(tk.spaceM).Gap(tk.spaceM).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.spaceS).Children(func() {
			chosen := int(a.panelTab)
			if ui.Segmented(c, &chosen, msgs.Outline(), msgs.Backlinks()).Label(msgs.PanelTitle()).Changed() {
				a.panelTab = panelTab(chosen)
			}
			ui.Spacer(c)
			ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
				Children(func() { tk.icon(c, "x", iconSmall) }).
				OnClick(func() { a.panel = panelClosed })
		})

		if a.panelTab == panelBacklinks {
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(tk.spaceXS).Children(func() {
					a.backlinkList(c, tk)
				})
			})
		} else {
			a.outlineList(c, tk)
		}

		a.cardsStrip(c, tk, a.editor.draft)
	})
}

// backlinkList shows the notes that name this one, with the paragraph that names
// it; choosing one opens that note on that paragraph.
func (a *App) backlinkList(c *ui.Context, tk tokens) {
	msgs := m(c)
	if len(a.backlinks) == 0 {
		ui.Text(c, msgs.BacklinksEmpty()).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
		return
	}
	for _, link := range a.backlinks {
		label := msgs.BacklinkAt(link.NoteTitle, link.Paragraph)
		target := link
		ui.ButtonBase(c.Key(string(link.Block))).Justify(ui.Start).
			Padding(tk.spaceXS, tk.spaceS).MinHeight(tk.control - tk.spaceS).
			Radius(tk.radiusControl).Label(label).
			OnClick(func() {
				a.editor.selected = string(target.Note)
				a.editor.jumpTo = target.Paragraph
				a.reload()
			}).
			Children(func() {
				ui.Column(c).Gap(0).Children(func() {
					ui.Text(c, label).FontSize(tk.typeSmall).TextColor(tk.Text).SingleLine()
					ui.Text(c, target.Text).FontSize(tk.typeSmall).TextColor(tk.TextMuted).SingleLine()
				})
			})
	}
}
