package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/about"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/internal/ui/screens/reading"
	"github.com/transmit-bug/remit/internal/ui/screens/review"
	"github.com/transmit-bug/remit/locales"
)

// overflowMenu is the top bar's ⋯: the actions that are not worth a button on
// the page, and whatever the page's plugins contributed to it. It is the
// window's one overflow menu, so it is the receiver for the contract's toolbar
// and note slots (docs/specs/plugins/ui.md §4).
func (a *App) overflowMenu(c *ui.Context, menu *ui.Menu) {
	msgs := m(c)
	if menu.Item(msgs.ImportTitle()).Chosen() {
		a.openImport(c)
	}
	if menu.Item(msgs.ExportTitle()).Chosen() {
		a.openExport(c)
	}
	if menu.Item(msgs.MakeCard()).Chosen() {
		a.makeCardAtCaret(c)
	}
	if menu.Item(msgs.OpenSettings()).Chosen() {
		a.openSettings()
	}
	items := a.contributionMenuItems("toolbar.overflow", "note.overflow")
	if len(items) > 0 {
		menu.Separator()
	}
	for _, ref := range items {
		if menu.Item(a.pluginText(c, ref.Plugin, string(ref.Command.Title))).Chosen() {
			a.runContribution(c, ref)
		}
	}
}

// contextActions are the controls that belong to the page on screen: the
// document's ways of looking at it, and its panel. Every other page leaves the
// bar to the frame's own controls, so the bar never offers an action the page
// does not have.
func (a *App) contextActions(c *ui.Context, tk design.Tokens) {
	msgs := m(c)
	switch {
	case a.area == areaRecall && a.sub == subRecallQueue:
		// The queue draws from the whole library, a note or a tag; which one is
		// the page's own action, so it sits in the bar above it.
		toolbarRule(c, tk)
		a.reviewScopeMenu(c, msgs)
		return
	case a.area != areaWrite || a.sub != subNotes:
		return
	}
	toolbarRule(c, tk)
	a.viewSwitch(c, tk, msgs)
	toolbarRule(c, tk)
	iconButton(c, tk, msgs.PanelTitle(), "panel-right", false, func() {
		a.notes.TogglePanel(c)
	})
}

// viewSwitch is the note page's three ways of looking at the same document:
// quiet, compact and grouped as one control. It is a SegmentedBase because its
// face is the note's, not the window theme's default field.
func (a *App) viewSwitch(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	view := int(a.notes.View)
	names := []string{msgs.ViewEdit(), msgs.ViewSplit(), msgs.ViewRead()}
	if segmentedChoice(c, tk, &view, names, msgs.ViewMode()) {
		a.notes.View = notes.ViewMode(view)
	}
}

// segmentedChoice draws the compact, quiet switch used for a view or a queue.
// It reports a new choice and writes it through the pointer, as Segmented does.
func segmentedChoice(c *ui.Context, tk design.Tokens, selected *int, names []string, label string) bool {
	seg := ui.SegmentedBase(c, selected, len(names))
	seg.Track.Height(tk.Control - 2).Padding(2).Gap(1).AlignItems(ui.Stretch).
		Radius(tk.RadiusControl).Background(tk.Sunken).Label(label).Children(func() {
		for i, name := range names {
			chosen := i == *selected
			segment := seg.Segment(i).Padding(0, tk.SpaceS).Radius(tk.RadiusControl - 2).Label(name)
			colour := tk.TextMuted
			if chosen {
				segment.Background(tk.Surface).TextColor(tk.Text)
				colour = tk.Text
			} else if segment.Hovered() {
				segment.Background(tk.SurfaceHover)
			}
			segment.Children(func() {
				ui.Text(c, name).FontSize(tk.TypeUI).FontWeight(weight(chosen, 600, 400)).
					TextColor(colour).SingleLine()
			})
		}
	})
	return seg.Track.Changed()
}

// toolbarRule is a hairline between groups of the top bar.
func toolbarRule(c *ui.Context, tk design.Tokens) {
	ui.Divider(c).Height(tk.SpaceXL).AlignSelf(ui.Center)
}

// iconButton is a page's icon-only action: a named hit area, a quiet face, and
// no border until it is hovered or turned on.
func iconButton(c *ui.Context, tk design.Tokens, label, icon string, on bool, click func()) ui.Element {
	button := ui.Button(c, "").Label(label).Tooltip(label).
		MinWidth(tk.Hit).MinHeight(tk.Hit).Justify(ui.Center).
		Radius(tk.RadiusControl).TextColor(tk.TextMuted)
	if on {
		button.Background(tk.Accent.Alpha(0.12)).TextColor(tk.Accent)
	}
	button.Children(func() { tk.Icon(c, icon, design.IconSmall) })
	if click != nil {
		button.OnClick(click)
	}
	return button
}

// weight chooses a font weight by whether the item is the chosen one.
func weight(chosen bool, on, off int) int {
	if chosen {
		return on
	}
	return off
}

// shortcuts declares the app's keys. They are window shortcuts, so they wait
// while the palette is open. The settings page is the app's, not the library's,
// so its key works before one is open; with no library the rest has no action
// yet.
//
// The digits are the areas, in rail order: Today, Write, Read, Recall, Stats,
// Settings. They are the window's primary navigation from the keyboard, as the
// rail is with the pointer.
func (a *App) shortcuts(c *ui.Context) {
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { a.openSettings() })
	// The keyboard map is the app's, not the library's: it works before one is
	// open, and it is how a user finds the key that does.
	c.OnShortcut(ui.Cmd, ui.KeySlash, func() { a.openKeys(c) })
	if a.area == areaSettings {
		c.OnShortcut(0, ui.KeyEscape, func() { a.closeSettings() })
	}
	if a.lib == nil {
		return
	}
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.openPalette(c) })
	c.OnShortcut(ui.Cmd, ui.Key1, func() { a.setArea(areaToday, subNone) })
	c.OnShortcut(ui.Cmd, ui.Key2, func() { a.setArea(areaWrite, subNotes) })
	c.OnShortcut(ui.Cmd, ui.Key3, func() { a.setArea(areaRead, subReadQueue) })
	c.OnShortcut(ui.Cmd, ui.Key4, func() { a.setArea(areaRecall, subRecallQueue) })
	c.OnShortcut(ui.Cmd, ui.Key5, func() { a.setArea(areaStats, subStatsOverview) })
	c.OnShortcut(ui.Cmd, ui.Key6, func() { a.setArea(areaSettings, subSettingsAppearance) })
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		if a.area == areaWrite && a.sub == subNotes && a.notes.Draft.ID != "" {
			// The notebook's find looks in the note that is open; the library's
			// search is the write area's search page.
			a.notes.FindOpen = true
			return
		}
		a.focusSearch(c)
	})
	c.OnShortcut(ui.Cmd, ui.KeyN, func() { a.newNote(c) })
	c.OnShortcut(ui.Cmd, ui.KeyD, func() { a.openDailyNote(c) })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyO, func() { a.showPanel(notes.PanelOutline) })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyB, func() { a.showPanel(notes.PanelBacklinks) })

	switch a.mode {
	case modeReview:
		// A card that takes a typed answer owns the keyboard until it is
		// checked: Space is a space there, and the grades only come back once
		// the answer is on screen, so a digit typed into the input is not a grade.
		c.OnShortcut(0, ui.KeySpace, func() {
			if !a.typing() {
				a.review.Reveal()
			}
		})
		if a.review.Answered() {
			c.OnShortcut(0, ui.Key1, func() { a.gradeCurrent(c, review.Again) })
			c.OnShortcut(0, ui.Key2, func() { a.gradeCurrent(c, review.Hard) })
			c.OnShortcut(0, ui.Key3, func() { a.gradeCurrent(c, review.Good) })
			c.OnShortcut(0, ui.Key4, func() { a.gradeCurrent(c, review.Easy) })
		}
		c.OnShortcut(0, ui.KeyU, func() { a.undo(c) })
		c.OnShortcut(0, ui.KeyS, func() { a.suspendCurrent(c) })
		c.OnShortcut(0, ui.KeyE, a.editCurrent)
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	case modeReading:
		// The reading screen is read with the keyboard: J and K move the reading
		// position, the digits record how the note digested, and E excerpts the
		// block the position is on.
		c.OnShortcut(0, ui.KeyJ, func() { a.moveReading(1) })
		c.OnShortcut(0, ui.KeyK, func() { a.moveReading(-1) })
		c.OnShortcut(0, ui.KeyDown, func() { a.moveReading(1) })
		c.OnShortcut(0, ui.KeyUp, func() { a.moveReading(-1) })
		c.OnShortcut(0, ui.Key1, func() { a.readCurrent(c, reading.TooHard) })
		c.OnShortcut(0, ui.Key2, func() { a.readCurrent(c, reading.JustRight) })
		c.OnShortcut(0, ui.Key3, func() { a.readCurrent(c, reading.TooShallow) })
		c.OnShortcut(0, ui.KeyE, func() { a.excerptCurrent(c) })
		c.OnShortcut(0, ui.KeyS, func() { a.skipCurrentReading() })
		c.OnShortcut(0, ui.KeyX, func() { a.dropCurrentReading(c) })
		c.OnShortcut(0, ui.KeyA, func() { a.endCurrentReading(c, reading.ArchiveDecision) })
		c.OnShortcut(0, ui.KeyD, func() { a.endCurrentReading(c, reading.DismissDecision) })
		c.OnShortcut(0, ui.KeyP, func() { a.endCurrentReading(c, reading.PostponeDecision) })
		c.OnShortcut(0, ui.KeyM, a.editCurrentReading)
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	case modeBrowse, modeStats, modePlugin:
		c.OnShortcut(0, ui.KeyEscape, func() { a.setMode(modeNotes) })
	}
}

// ShowAbout opens the About window above parent, or brings it back to the
// front when it is already open. Opening a window is assembly, so it lives
// here and the page lives in its own package (ADR 0009).
func (a *App) ShowAbout(parent *mygo.Window) {
	if a.aboutWin != nil {
		a.aboutWin.Focus()
		return
	}
	state := about.State{Name: Name, Version: Version}
	a.aboutWin = mygo.NewWindow(mygo.WindowOptions{
		Title:         "About " + Name,
		Width:         about.Width,
		Height:        about.Height,
		DisableResize: true,
		Content:       ui.View(func(c *ui.Context) { about.View(c, state, about.Deps{}) }),
	})
	a.aboutWin.OnClosed(func() { a.aboutWin = nil })
}

// focusSearch shows the write area's search page and puts the caret in its
// field, from anywhere: the one way to search the library by text.
func (a *App) focusSearch(c *ui.Context) {
	a.setArea(areaWrite, subSearch)
	a.browse.Focus()
}
