package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
	screenstats "github.com/transmit-bug/remit/internal/ui/screens/stats"
	"github.com/transmit-bug/remit/internal/ui/shell"
)

// View draws the window. It reads state and returns; anything that changes
// state belongs in a handler.
func (a *App) View(c *ui.Context) {
	// What a plugin's callback answered since the last frame is applied first,
	// so the frame that draws it draws the result. The work itself already ran
	// off this pass: no frame ever waits on a plugin.
	a.applyBackground(c)
	// The library's writes are the contract's event: drop what each page was
	// loaded against, so the page on screen asks again off this frame.
	if a.stale.Swap(false) {
		a.invalidatePages()
	}

	// One place resolves the look: the appearance the user chose folded over the
	// desktop's, and the paper's measure from the settings (a.tokens).
	tk := a.tokens(c)
	c.SetTheme(tk.Theme)

	// Window shortcuts belong to the window, not to the column they are built
	// beside, so that they work with no control focused.
	a.shortcuts(c)

	ui.Column(c).Fill().Background(tk.Background).LineHeight(design.LineHeight).Children(func() {
		// A session takes the window whole: no rail, no bars, just the card or
		// the note. So does the welcome before a library opens, since there is
		// nothing yet to navigate — and the settings page, which is the app's
		// rather than the library's, keeps the frame.
		if a.framed() {
			shell.View(c, &a.shell, a.shellDeps(c), func() { a.stage(c, tk) })
		} else {
			a.stage(c, tk)
		}
		// The overlays come last: each takes the window's keyboard while it
		// shows, and the one opened last is the one on top.
		a.paletteView(c, tk)
		a.keysView(c, tk)
		a.importView(c, tk)
		a.exportView(c, tk)
		a.recallScopeView(c)
		a.relinkView(c)
		a.cardEditView(c, tk)
		a.cardDetailView(c, tk)
	})
}

// framed reports whether the window draws its frame around the stage. A session
// takes the window whole, and so does the welcome before a library opens: there
// is nothing for a rail to navigate yet. The settings page is the app's, not the
// library's, so it keeps the frame without one.
func (a *App) framed() bool {
	if a.takingOver() {
		return false
	}
	if a.area == areaSettings {
		return true
	}
	return a.lib != nil && !a.start && a.openErr == nil
}

// stage draws the page on screen. One area — or one page of it — replacing
// another fades in and rises, played by the count the navigation keeps, and the
// page itself asks the app for nothing but what a load path already left.
func (a *App) stage(c *ui.Context, tk design.Tokens) {
	stage := ui.Column(c).Grow(1).MinWidth(0)
	rise := stage.Keyframes("area", design.ModeFrames, ui.KeyframeOptions{Run: a.anim.mode})
	stage.Opacity(rise).Translate(0, design.Rise*(1-rise)).Children(func() {
		switch {
		case a.openErr != nil:
			a.libraryErrorView(c, tk)
		case a.area == areaSettings:
			// The settings page does not need a library: a first run turns
			// reminders and update checks on before it makes one.
			screensettings.View(c, &a.prefs, a.settingsDeps())
		case a.start:
			a.startView(c)
		case a.session == sessionReview:
			a.reviewView(c, tk)
		case a.session == sessionReading:
			a.readingView(c)
		case a.area == areaWrite:
			a.writeView(c)
		case a.area == areaRead:
			a.readView(c)
		case a.area == areaRecall:
			a.recallStage(c)
		case a.area == areaStats:
			screenstats.View(c, statsPage(a.sub), a.stats, screenstats.Deps{
				Items:      a.statsBlockItems(),
				PluginText: a.pluginText,
				Run:        a.runContribution,
			})
		case a.area == areaPlugin:
			a.pluginPageView(c)
		case a.area == areaToday:
			a.todayView(c)
		}
	})
}

// shellDeps is the frame as the app hands it over: where the user is, what the
// rail counts, and how to go somewhere. Every action is a callback; the frame
// reads no store.
func (a *App) shellDeps(c *ui.Context) shell.Deps {
	return shell.Deps{
		Title:      Name,
		Areas:      a.railItems(c),
		Pages:      a.railPages(c),
		Current:    a.railChoice(),
		OnSelect:   func(item shell.Area) { a.selectRailItem(c, item) },
		Subnav:     a.subnavItems(c),
		CurrentSub: subName(a.sub),
		OnSub:      func(id string) { a.setSub(subOfName(a.area, id)) },
		Crumbs:     a.crumbs(c),
		OnCrumb:    a.chooseCrumb,
		Actions:    a.contextActions,
		OnPalette:  func() { a.openPalette(c) },
		Overflow:   func(menu *ui.Menu) { a.overflowMenu(c, menu) },
		Status:     a.docStatus(c),
	}
}

// stageWidth is the room the stage has in a window this wide: the window less
// the rail the frame keeps beside it. A page computes its columns from this,
// never from the window alone, which is not the room the page was given.
func (a *App) stageWidth(w float32) float32 {
	return max(0, w-shell.RailWidth(w, a.shell.Folded))
}

// railItems are the six areas, each with what its own queue holds: the mixed
// queue for today, the notes for writing, the due notes for reading, the due
// cards for recall. A count that is zero is no badge at all.
func (a *App) railItems(c *ui.Context) []shell.Item {
	msgs := m(c)
	counts := a.queueCounts()
	return []shell.Item{
		{ID: shell.Today, Label: msgs.AreaToday(), Icon: "inbox", Count: counts.Cards + counts.Reading},
		{ID: shell.Write, Label: msgs.AreaWrite(), Icon: "notebook-pen", Count: len(a.data.notes)},
		{ID: shell.Read, Label: msgs.AreaRead(), Icon: "book-open", Count: counts.Reading},
		{ID: shell.Recall, Label: msgs.AreaRecall(), Icon: "layers", Count: counts.Cards},
		{ID: shell.Stats, Label: msgs.AreaStats(), Icon: "chart-column"},
		{ID: shell.Settings, Label: msgs.AreaSettings(), Icon: "settings"},
	}
}

// railPages are the pages plugins contributed: the rail's second group, which
// is not drawn at all when there are none.
func (a *App) railPages(c *ui.Context) []shell.Item {
	pages := a.visiblePages()
	if len(pages) == 0 {
		return nil
	}
	out := make([]shell.Item, 0, len(pages))
	for _, page := range pages {
		out = append(out, shell.Item{
			ID:    pageArea(page),
			Label: a.pluginText(c, page.Plugin, string(page.Page.Title)),
			Icon:  page.Page.Icon,
		})
	}
	return out
}

// pageArea is a contributed page's rail id and its route: one stable name for
// the sidebar's choice and the breadcrumb.
func pageArea(page contrib.PageRef) shell.Area {
	return shell.Area("plugin/" + page.Plugin + "/" + page.Page.ID)
}

// railChoice is the rail item that is chosen: an area's id, or the page a plugin
// contributed.
func (a *App) railChoice() shell.Area {
	if a.area == areaPlugin {
		return pageArea(a.page)
	}
	return areaName(a.area)
}

// selectRailItem goes where the rail was told to go: an area at its first page,
// or a plugin page.
func (a *App) selectRailItem(c *ui.Context, item shell.Area) {
	if page, ok := a.pageFor(item); ok {
		a.openPage(c, page)
		return
	}
	for _, area := range []areaID{areaToday, areaWrite, areaRead, areaRecall, areaStats, areaSettings} {
		if areaName(area) == item {
			a.setArea(area, defaultSub(area))
			return
		}
	}
}

// pageFor resolves a rail id back to the page it names, when it names one.
func (a *App) pageFor(item shell.Area) (contrib.PageRef, bool) {
	for _, page := range a.visiblePages() {
		if pageArea(page) == item {
			return page, true
		}
	}
	return contrib.PageRef{}, false
}

// subnavItems are the current area's pages, with what each one holds: the notes
// the notes page lists, the cards the table lists, the queues' lengths. An empty
// list hides the row — Today has no pages of its own, and a document is a note
// entered from the notes page rather than a page beside it, so its row goes and
// the stage gets the height back.
func (a *App) subnavItems(c *ui.Context) []shell.Sub {
	if a.documentOpen() {
		return nil
	}
	msgs := m(c)
	var out []shell.Sub
	for _, sub := range subsOf(a.area) {
		item := shell.Sub{ID: subName(sub), Label: subLabel(msgs, sub)}
		switch sub {
		case subNoteList:
			item.Count = len(a.data.notes)
		case subRecallQueue:
			item.Count = a.queueCounts().Cards
		case subRecallCards:
			item.Count = len(a.data.cards)
		case subRecallTypes:
			item.Count = len(a.typeList)
		case subRecallRelink:
			item.Count = len(a.lostCards)
		case subReadQueue:
			item.Count = a.queueCounts().Reading
		}
		out = append(out, item)
	}
	return out
}

// crumbs is the path to where the user is, which the top bar shows: the app, the
// area and the page, the last one being the page itself.
func (a *App) crumbs(c *ui.Context) []string {
	msgs := m(c)
	if a.area == areaPlugin {
		return []string{Name, a.pluginText(c, a.page.Plugin, string(a.page.Page.Title))}
	}
	out := []string{Name, areaLabel(msgs, a.area)}
	if a.area != areaToday {
		out = append(out, subLabel(msgs, a.sub))
	}
	return out
}

// chooseCrumb goes back to an earlier segment of the path: the app itself is
// today, and an area is that area's first page.
func (a *App) chooseCrumb(index int) {
	if index == 0 {
		a.setArea(areaToday, subNone)
		return
	}
	a.setArea(a.area, defaultSub(a.area))
}
