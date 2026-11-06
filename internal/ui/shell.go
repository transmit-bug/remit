package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/internal/ui/review"
	"github.com/transmit-bug/remit/locales"
)

const (
	// sidebarWidth is the notes column, a fixed 260 DIP column rather than a
	// pane the user resizes: review mode hides it instead of sharing space. On
	// the smallest window it narrows, so that the note keeps room to be typed.
	sidebarWidth  float32 = 260
	sidebarNarrow float32 = 220
	// panelWidth is the note's own panel, to the right of the document: its
	// cards, and what #97 and #98 will put there.
	panelWidth float32 = 280
	// paletteWidth is the command palette's panel.
	paletteWidth float32 = 480
)

// toolbar is the window's one row of controls, the same in every mode so that
// switching never moves what the user is aiming at. Before a library is open
// there is nothing to switch between, so the row holds the app's name alone.
func (a *App) toolbar(c *ui.Context, tk tokens) ui.Element {
	toolbar := ui.Toolbar(c, func() {
		ui.Text(c, Name).FontSize(tk.typeHeading).Bold()
		if a.lib == nil {
			ui.Spacer(c)
			return
		}

		chosen := int(a.mode)
		if ui.Segmented(c, &chosen, m(c).ModeNotes(), m(c).ModeReview(), m(c).ModeBrowse(), m(c).ModeStats(), m(c).ModeSettings()).
			Label(m(c).ModeLabel()).Changed() {
			a.setMode(mode(chosen))
		}

		ui.Spacer(c)
		ui.Button(c, "").Label(m(c).NewNote()).
			Children(func() { tk.iconText(c, "plus", m(c).NewNote(), iconSmall) }).
			OnClick(func() { a.newNote(c) })
	}).Label(m(c).Toolbar(Name)).
		Padding(tk.spaceS, tk.spaceL).
		Background(tk.Surface)
	return toolbar
}

// notesView is editing mode: the notes on the left, the note in the middle, and
// the note's own panel on the right. A narrow window drops the panel first, then
// narrows the notes column, so the document always keeps room to be typed.
func (a *App) notesView(c *ui.Context, tk tokens) {
	w, _ := c.Size()
	narrow := w < narrowWindow
	side := sidebarWidth
	if narrow {
		side = sidebarNarrow
	}
	ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
		a.notesList(c, tk, side)
		ui.Divider(c)
		a.editorView(c, tk)
		if a.panelShown(narrow) {
			ui.Divider(c)
			a.notePanel(c, tk)
		}
	})
}

// narrowWindow is the width below which the notebook drops its panel, puts the
// split away and narrows its notes column.
const narrowWindow float32 = 900

// panelMode is what the user said about the note's panel, when they said
// anything: a narrow window closes it until they ask for it.
type panelMode uint8

const (
	panelAuto panelMode = iota
	panelOpen
	panelClosed
)

// panelShown reports whether the note's panel takes a column this frame.
func (a *App) panelShown(narrow bool) bool {
	switch a.panel {
	case panelOpen:
		return true
	case panelClosed:
		return false
	}
	return !narrow
}

// showPanel opens the note's panel on one of its views, from the keyboard.
func (a *App) showPanel(tab panelTab) {
	if a.mode != modeNotes {
		a.setMode(modeNotes)
	}
	a.panel, a.panelTab = panelOpen, tab
}

// togglePanel opens or closes the note's panel, whatever the window's width.
func (a *App) togglePanel() {
	if a.panelShown(true) {
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

// notePanel is the column beside the document: the note's outline or the notes
// that point at it, then what the note feeds. It scrolls on its own and takes
// the focus only when one of its rows is chosen.
func (a *App) notePanel(c *ui.Context, tk tokens) {
	msgs := m(c)
	ui.Column(c).Width(panelWidth).MaxWidth(panelWidth).Padding(tk.spaceM).Gap(tk.spaceM).Children(func() {
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

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(tk.spaceXS).Children(func() {
				if a.panelTab == panelBacklinks {
					a.backlinkList(c, tk)
				} else {
					a.outlineList(c, tk)
				}
			})
		})

		a.cardsStrip(c, tk, a.editor.draft)
	})
}

// outlineEntry is one heading of the note: what it says, and where it starts in
// the document the editor holds.
type outlineEntry struct {
	label  string
	offset int
}

// outline reads a note's blocks into its headings, in document order. The
// blocks carry the Markdown source — the editor keeps a heading's hashes as
// style, not text — and the offset is where the heading starts in the document
// the editor holds.
func outline(blocks []core.Block) []outlineEntry {
	var out []outlineEntry
	offset := 0
	for i, block := range blocks {
		if i > 0 {
			offset += 2 // the blank line between blocks
		}
		if level, label := heading(strings.TrimSpace(block.Text)); level > 0 {
			out = append(out, outlineEntry{label: label, offset: offset})
		}
		offset += len(block.Text)
	}
	return out
}

// heading reads a Markdown heading, returning its level and its text.
func heading(body string) (level int, label string) {
	for level < len(body) && body[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(body) || body[level] != ' ' {
		return 0, ""
	}
	// A heading is its first line: what follows a blank line is the paragraph
	// under it, not part of its name.
	label = body[level+1:]
	if end := strings.IndexByte(label, '\n'); end >= 0 {
		label = label[:end]
	}
	return level, strings.TrimSpace(label)
}

// outlineList shows the note's headings; choosing one puts the caret on its
// paragraph, which is what scrolls it into view.
func (a *App) outlineList(c *ui.Context, tk tokens) {
	if a.editor.doc == nil {
		ui.Text(c, m(c).OutlineEmpty()).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
		return
	}
	entries := outline(a.editor.draft.Blocks)
	if len(entries) == 0 {
		ui.Text(c, m(c).OutlineEmpty()).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
		return
	}
	for _, entry := range entries {
		at := entry.offset
		ui.ButtonBase(c).Key(entry.label).Justify(ui.Start).
			Padding(tk.spaceXS, tk.spaceS).MinHeight(tk.control - tk.spaceS).
			Radius(tk.radiusControl).Label(entry.label).
			OnClick(func() { a.jumpInDocument(c, at) }).
			Children(func() {
				ui.Text(c, entry.label).FontSize(tk.typeSmall).TextColor(tk.Text).SingleLine()
			})
	}
}

// jumpInDocument puts the caret on a paragraph of the note open now, which the
// editor scrolls into view.
func (a *App) jumpInDocument(c *ui.Context, offset int) {
	if a.editor.doc == nil {
		return
	}
	a.editor.doc.SetSelection(offset, offset)
	a.editor.body.Focus(c)
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
		ui.ButtonBase(c).Key(string(link.Block)).Justify(ui.Start).
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

// tagFilterBar is the row above the notes: every tag the library carries, with
// how many notes hold it. Choosing one filters the list; choosing it again — or
// the × — puts the whole library back.
func (a *App) tagFilterBar(c *ui.Context, tk tokens) {
	if len(a.data.tags) == 0 {
		return
	}
	tags := make([]string, 0, len(a.data.tags))
	for tag := range a.data.tags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	ui.ScrollHorizontal(c).Children(func() {
		ui.Row(c).Gap(tk.spaceXS).Padding(tk.spaceXS, 0).Children(func() {
			if a.tagFilter != "" {
				ui.Button(c, m(c).ClearFilter()).MinHeight(tk.control - tk.spaceS).
					OnClick(func() { a.tagFilter = "" })
			}
			for _, tag := range tags {
				chosen := a.tagFilter == tag
				label := "#" + tag
				chip := ui.ButtonBase(c).Key(label).
					Padding(tk.spaceXS, tk.spaceS).MinHeight(tk.control-tk.spaceS).
					Radius(tk.radiusControl).Label(label).
					Border(tk.hairline, tk.Border)
				if chosen {
					chip.Background(tk.Surface.Mix(tk.Accent, 0.16)).
						Border(tk.hairline, tk.Accent)
				}
				chip.OnClick(func() {
					if a.tagFilter == tag {
						a.tagFilter = ""
						return
					}
					a.tagFilter = tag
				})
				chip.Children(func() {
					ui.Text(c, label).FontSize(tk.typeSmall).TextColor(tk.Text)
					if n := a.data.tags[tag]; n > 1 {
						ui.Text(c, fmt.Sprint(n)).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
					}
				})
			}
		})
	})
}

// hasTag reports whether a note carries one of the tags.
func hasTag(tags []string, tag string) bool {
	for _, got := range tags {
		if got == tag {
			return true
		}
	}
	return false
}

// notesList is the source list of notes: today at the top, then every note
// newest first, with how many cards each one feeds.
func (a *App) notesList(c *ui.Context, tk tokens, width float32) {
	ui.Column(c).Width(width).Children(func() {
		// The tags are their own row: inside a Sidebar a click belongs to the
		// list's selection, not to a button.
		a.tagFilterBar(c, tk)
		ui.Sidebar(c, &a.editor.selected, func() {
			ui.SidebarItem(c, "today", design.Icon("layers"), m(c).Today()).
				Children(func() { ui.Badge(c, fmt.Sprint(a.review.Remaining())) }).
				OnClick(func() { a.setMode(modeReview) })
			ui.SidebarSection(c, m(c).Notebook(), nil, func() {
				for _, n := range a.data.notes {
					if a.tagFilter != "" && !hasTag(a.data.noteTags[n.ID], a.tagFilter) {
						continue
					}
					item := ui.SidebarItem(c, string(n.ID), design.Icon("file-text"), n.Title)
					if cards := a.data.cardsInNote(n); cards > 0 {
						item.Children(func() { ui.Badge(c, fmt.Sprint(cards)) })
					}
				}
			})
		})
	})
}

// shortcuts declares the app's keys. They are window shortcuts, so they wait
// while the palette is open. The settings page is the app's, not the library's,
// so its key works before one is open; with no library the rest has no action
// yet.
func (a *App) shortcuts(c *ui.Context) {
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { a.openSettings() })
	if a.mode == modeSettings {
		c.OnShortcut(0, ui.KeyEscape, func() { a.closeSettings() })
	}
	if a.lib == nil {
		return
	}
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.openPalette(c) })
	c.OnShortcut(ui.Cmd, ui.Key1, func() { a.setMode(modeReview) })
	c.OnShortcut(ui.Cmd, ui.Key2, func() { a.setMode(modeNotes) })
	c.OnShortcut(ui.Cmd, ui.Key3, func() { a.setMode(modeBrowse) })
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		if a.mode == modeNotes && a.editor.draft.ID != "" {
			// The notebook's find looks in the note that is open; the library's
			// search is the browse page's.
			a.editor.findOpen = true
			return
		}
		a.focusSearch(c)
	})
	c.OnShortcut(ui.Cmd, ui.KeyN, func() { a.newNote(c) })
	c.OnShortcut(ui.Cmd, ui.KeyD, func() { a.openDailyNote(c) })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyO, func() { a.showPanel(panelOutline) })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyB, func() { a.showPanel(panelBacklinks) })

	switch a.mode {
	case modeReview:
		c.OnShortcut(0, ui.KeySpace, func() { a.review.Reveal() })
		c.OnShortcut(0, ui.Key1, func() { a.gradeCurrent(c, review.Again) })
		c.OnShortcut(0, ui.Key2, func() { a.gradeCurrent(c, review.Hard) })
		c.OnShortcut(0, ui.Key3, func() { a.gradeCurrent(c, review.Good) })
		c.OnShortcut(0, ui.Key4, func() { a.gradeCurrent(c, review.Easy) })
		c.OnShortcut(0, ui.KeyU, func() { a.undo(c) })
		c.OnShortcut(0, ui.KeyS, func() { a.suspendCurrent(c) })
		c.OnShortcut(0, ui.KeyE, a.editCurrent)
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	case modeBrowse, modeStats:
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	}
}

// setMode switches the window's face and loads what the new one reads. What a
// mode derives from the store is loaded here, never in its view, and leaving
// the settings page this way is the same as any other switch.
func (a *App) setMode(m mode) {
	if m != a.mode {
		a.anim.mode++
	}
	a.mode = m
	a.loadCurrent()
}

// focusSearch shows browse and puts the caret in its search field, from any
// mode: the one way to search the library by text.
func (a *App) focusSearch(c *ui.Context) {
	a.setMode(modeBrowse)
	a.browse.field.Focus(c)
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

// paletteAction is one command the palette lists.
type paletteAction struct {
	label string
	run   func()
}

// paletteState is the command palette: open or not, what has been typed, and
// which of the matching actions the keys are on.
type paletteState struct {
	open  bool
	query string
	sel   int
	field ui.Handle
}

// openPalette opens the palette on a fresh query and asks for the focus before
// the dialog is built: AutoFocus would take it before the dialog recorded what
// had it, and Escape would then have nowhere to give it back.
func (a *App) openPalette(c *ui.Context) {
	a.anim.palette++
	a.palette.open = true
	a.palette.query = ""
	a.palette.sel = 0
	a.palette.field.Focus(c)
}

// paletteActions lists what the app can do from anywhere, with the block
// actions of the note open in the editor. They are listed whenever the library
// holds a note; each runs against the note the editor has loaded, loading it
// first when the user has not been there yet.
func (a *App) paletteActions(c *ui.Context) []paletteAction {
	actions := []paletteAction{
		{label: m(c).ActionReviewCards(), run: func() { a.setMode(modeReview) }},
		{label: m(c).ActionEditNotes(), run: func() { a.setMode(modeNotes) }},
		{label: m(c).BrowseTitle(), run: func() { a.setMode(modeBrowse) }},
		{label: m(c).StatsTitle(), run: func() { a.setMode(modeStats) }},
		{label: m(c).NewNote(), run: func() { a.newNote(c) }},
		{label: m(c).OpenSettings(), run: func() { a.openSettings() }},
	}
	if a.lib == nil {
		return actions
	}
	actions = append(actions,
		paletteAction{label: m(c).ImportAnki(), run: func() { a.importAnkiPackage(c) }},
		// A note's shape is typed, not commanded: paragraphs are separated by
		// blank lines. Making a card from the paragraph at the caret stays a
		// command until the typing gesture lands (#94).
		paletteAction{label: m(c).MakeCard(), run: func() { a.makeCardAtCaret(c) }},
	)
	return actions
}

// paletteView draws the palette over the window. DialogBase keeps the keyboard
// in it, Escape closes it and gives the focus back where it was.
func (a *App) paletteView(c *ui.Context, tk tokens) {
	if !a.palette.open {
		return
	}
	items := filterActions(a.paletteActions(c), a.palette.query)
	sel := clamp(a.palette.sel, 0, len(items)-1)

	ui.DialogBase(c, &a.palette.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.scrim)
		panel = kit.StylePanel(c, panel.Width(paletteWidth).Padding(tk.spaceL).Gap(tk.spaceM))
		panel.Label(m(c).CommandPalette())
		// The palette arrives a little small over a scrim that fades in.
		arriving := panel.Keyframes("arrive", design.PanelFrames, ui.KeyframeOptions{Run: a.anim.palette})
		scale := design.PanelScale + (1-design.PanelScale)*arriving
		panel.Opacity(arriving).Scale(scale, scale)
		backdrop.Opacity(arriving)

		panel.Children(func() {
			field := ui.TextInput(c, &a.palette.query).
				Placeholder(m(c).SearchActions()).Bind(&a.palette.field).
				Label(m(c).SearchActions()).MinHeight(tk.control)
			if field.Changed() {
				a.palette.sel = 0
			}
			if field.Submitted() {
				a.runPalette(items, sel)
			}
			c.OnShortcut(0, ui.KeyDown, func() { a.movePalette(len(items), 1) })
			c.OnShortcut(0, ui.KeyUp, func() { a.movePalette(len(items), -1) })

			if len(items) == 0 {
				ui.Text(c, m(c).NoActions()).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
			}
			for i, it := range items {
				row := ui.ButtonBase(c).Key(it.label).Gap(tk.spaceS).Justify(ui.Start).
					MinHeight(tk.control).Padding(tk.spaceXS, tk.spaceS).
					Radius(tk.radiusControl).Label(it.label)
				if i == sel {
					row.Background(tk.sunken)
				}
				row.OnClick(func() { a.runPaletteAction(it) })
				row.Children(func() { ui.Text(c, it.label).FontSize(tk.typeBody) })
			}
		})
	})
}

// runPalette runs the action at sel and closes the palette.
func (a *App) runPalette(items []paletteAction, sel int) {
	if sel < 0 || sel >= len(items) {
		return
	}
	a.runPaletteAction(items[sel])
}

// runPaletteAction closes the palette, then runs the action.
func (a *App) runPaletteAction(act paletteAction) {
	a.palette.open, a.palette.query, a.palette.sel = false, "", 0
	act.run()
}

// movePalette moves the palette's selection within n actions.
func (a *App) movePalette(n, delta int) {
	if n == 0 {
		return
	}
	a.palette.sel = clamp(a.palette.sel+delta, 0, n-1)
}

// filterActions keeps the actions whose label holds query, ignoring case.
func filterActions(actions []paletteAction, query string) []paletteAction {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return actions
	}
	var out []paletteAction
	for _, a := range actions {
		if strings.Contains(strings.ToLower(a.label), q) {
			out = append(out, a)
		}
	}
	return out
}

// clamp keeps v between lo and hi; an empty range gives lo.
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return max(lo, min(v, hi))
}
