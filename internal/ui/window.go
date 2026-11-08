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

// toolbar is the window's app bar: a brand, one stable mode rail, and the
// controls that belong to the mode on screen. The rail never moves between
// modes; only the contextual group at its end changes. Before a library opens,
// the bar keeps the brand and says nothing about pages there are none of.
func (a *App) toolbar(c *ui.Context, tk design.Tokens) ui.Element {
	msgs := m(c)
	toolbar := ui.Toolbar(c, func() {
		a.brand(c, tk)
		if a.lib == nil {
			ui.Spacer(c)
			return
		}

		toolbarRule(c, tk)
		ui.Spacer(c)
		a.modeTabs(c, tk, msgs)
		a.pluginPagesMenu(c, msgs)

		ui.Spacer(c)
		if a.mode == modeNotes {
			a.viewSwitch(c, tk, msgs)
			toolbarRule(c, tk)
			iconButton(c, tk, msgs.PanelTitle(), "panel-right", false, func() { a.notes.TogglePanel(c) })
		}
		iconButton(c, tk, msgs.More(), "ellipsis", false, nil).
			Menu(func(menu *ui.Menu) {
				if menu.Item(msgs.ImportAnki()).Chosen() {
					a.importAnkiPackage(c)
				}
				if menu.Item(msgs.ImportMarkdown()).Chosen() {
					a.importMarkdownFiles(c)
				}
				if menu.Item(msgs.ExportNote()).Chosen() {
					a.exportNoteMarkdown(c)
				}
				if menu.Item(msgs.ExportAllNotes()).Chosen() {
					a.exportAllMarkdown(c)
				}
				if menu.Item(msgs.MakeCard()).Chosen() {
					a.makeCardAtCaret(c)
				}
				if menu.Item(msgs.OpenSettings()).Chosen() {
					a.openSettings()
				}
				// The plugins' menu items follow the host's: this window's one
				// overflow menu is the receiver for the contract's toolbar and
				// note slots (docs/specs/plugins/ui.md §4).
				items := a.contributionMenuItems("toolbar.overflow", "note.overflow")
				if len(items) > 0 {
					menu.Separator()
				}
				for _, ref := range items {
					if menu.Item(a.pluginText(c, ref.Plugin, string(ref.Command.Title))).Chosen() {
						a.runContribution(c, ref)
					}
				}
			})
	}).Label(msgs.Toolbar(Name)).
		Height(tk.FrameHeight).
		Padding(0, tk.SpaceL).
		Background(tk.Surface).
		BorderWidth(0, 0, tk.Hairline, 0).
		BorderColor(tk.Border)
	return toolbar
}

// brand is the quiet identity at the head of the app bar: a mark whose colour
// is the product's accent, then the wordmark. It is not a button; the pages
// it names are in the rail beside it.
func (a *App) brand(c *ui.Context, tk design.Tokens) {
	ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Padding(0, tk.SpaceS).Children(func() {
		ui.Box(c).Size(tk.SpaceXL+2, tk.SpaceXL+2).Radius(tk.RadiusControl).
			Background(tk.Accent).Center().Label(Name).Children(func() {
			tk.Icon(c, "notebook-pen", design.IconSmall).TextColor(tk.AccentText)
		})
		if w, _ := c.Size(); w >= 800 {
			ui.Text(c, Name).FontSize(tk.TypeBody).FontWeight(650).LetterSpacing(-0.15)
		}
	})
}

// modeTabs is the app's primary navigation: text tabs with a mark-coloured
// underline, not another boxed segmented control. TabsBase keeps the arrow
// keys, the tab list role and the chosen page's state.
func (a *App) modeTabs(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	modes := []mode{modeNotes, modeReview, modeBrowse, modeStats, modeSettings}
	chosen := 0
	for i, candidate := range modes {
		if candidate == a.mode {
			chosen = i
			break
		}
	}
	tabs := ui.TabsBase(c, &chosen, 5)
	names := []string{msgs.ModeNotes(), msgs.ModeReview(), msgs.ModeBrowse(), msgs.ModeStats(), msgs.ModeSettings()}
	tabs.List.Label(msgs.ModeLabel()).Height(tk.FrameHeight - tk.Hairline).
		AlignItems(ui.Stretch).Gap(0).Children(func() {
		for i, name := range names {
			selected := i == chosen
			tab := tabs.Tab(i).Padding(0, tk.SpaceS).Justify(ui.Center).Radius(0).Label(name)
			colour := tk.TextMuted
			if selected {
				colour = tk.Text
			}
			if tab.Hovered() && !selected {
				tab.Background(tk.SurfaceHover)
			}
			tab.Children(func() {
				ui.Text(c, name).FontSize(tk.TypeChrome).FontWeight(weight(selected, 600, 400)).
					TextColor(colour).SingleLine()
			})
			if selected {
				tab.DrawOver(func(p *ui.Painter, r ui.Rect) {
					p.Fill(ui.Rect{X: r.X + tk.SpaceS, Y: r.Y + r.H - 2, W: max(1, r.W-2*tk.SpaceS), H: 2},
						tk.Accent, 1)
				})
			}
		}
	})
	if tabs.List.Changed() {
		a.setMode(modes[chosen])
	}
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
				ui.Text(c, name).FontSize(tk.TypeSmall).FontWeight(weight(chosen, 600, 400)).
					TextColor(colour).SingleLine()
			})
		}
	})
	return seg.Track.Changed()
}

// toolbarRule is a hairline between groups of the app bar.
func toolbarRule(c *ui.Context, tk design.Tokens) {
	ui.Divider(c).Height(tk.SpaceXL).AlignSelf(ui.Center)
}

// iconButton is the app bar's icon-only action: a named hit area, a quiet face,
// and no border until it is hovered or turned on.
func iconButton(c *ui.Context, tk design.Tokens, label, icon string, on bool, click func()) ui.Element {
	button := ui.Button(c, "").Label(label).Tooltip(label).
		MinWidth(tk.Control + 2).MinHeight(tk.Control + 2).Justify(ui.Center).
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

// pluginPagesMenu offers the pages plugins contributed, when the library has
// one to offer: the host's plugin-pages group, which the contract guarantees
// exists. How pages are grouped and sorted beyond it is #129's (ui.md §2).
func (a *App) pluginPagesMenu(c *ui.Context, msgs locales.Messages) {
	pages := a.visiblePages()
	if len(pages) == 0 {
		return
	}
	button := ui.Button(c, "").Label(msgs.PluginPages()).Tooltip(msgs.PluginPages()).
		MinWidth(design.ControlHeight + 2).MinHeight(design.ControlHeight + 2).
		Justify(ui.Center).Radius(design.RadiusControl).TextColor(c.Theme().TextMuted)
	button.Children(func() { ui.Icon(c, design.Icon("layers")).Width(design.IconSmall).Height(design.IconSmall) })
	button.Menu(func(menu *ui.Menu) {
		for _, page := range pages {
			if menu.Item(a.pluginText(c, page.Plugin, string(page.Page.Title))).Chosen() {
				a.openPage(c, page)
			}
		}
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
		if a.mode == modeNotes && a.notes.Draft.ID != "" {
			// The notebook's find looks in the note that is open; the library's
			// search is the browse page's.
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
		Parent:        parent,
		Width:         about.Width,
		Height:        about.Height,
		DisableResize: true,
		Content:       ui.View(func(c *ui.Context) { about.View(c, state, about.Deps{}) }),
	})
	a.aboutWin.OnClosed(func() { a.aboutWin = nil })
}

// focusSearch shows browse and puts the caret in its search field, from any
// mode: the one way to search the library by text.
func (a *App) focusSearch(c *ui.Context) {
	a.setMode(modeBrowse)
	a.browse.Focus(c)
}
