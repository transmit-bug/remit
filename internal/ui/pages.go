package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/internal/ui/screens/read"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
	screenstats "github.com/transmit-bug/remit/internal/ui/screens/stats"
	"github.com/transmit-bug/remit/locales"
)

// form is a page's skeleton: the arrangement the stage lays out, whatever the
// page puts inside it (docs/ui-layout-rules.md §1). It is the shape #281's
// layout rules hang on; the screens still choose their own measure, so nothing
// reads it yet.
type form uint8

const (
	// formPaper is one column of content in a measure of its own, centred in
	// the stage: the areas' lists, forms and pages.
	formPaper form = iota
	// formWorkspace is a paper and the panel beside it: the document.
	formWorkspace
	// formSplit is a draggable master column and the body it selects, across
	// the whole stage: the type list.
	formSplit
)

// page is one place the window can be, declared once. The route, the
// sub-navigation, the breadcrumb, the rail and a link's landing all read these
// rather than each keeping a list of its own, so adding a page is one record
// here and one view function.
type page struct {
	area  areaID
	sub   subID
	name  string                        // the stable id a link and the plugin contract spell
	label func(locales.Messages) string // the word this frame shows
	form  form
	view  func(a *App, c *ui.Context) // draws the page
	load  func(a *App)                // derives what the page reads; nil when nothing is derived
	// detail is a place entered from a page — the document of a note — rather
	// than one the area's sub-navigation offers.
	detail bool
}

// pageTable is every page the window has, in the order its area's
// sub-navigation reads. An area with no pages of its own — today, and a page a
// plugin contributed — is not here: today is the area itself, and a plugin page
// is a group of its own.
//
// It is filled in init because a page's view is code that reads the route, which
// reads this table: a plain initializer would be an initialization cycle, and
// the table has to be complete before the first frame asks it anything.
var pageTable []page

func init() {
	pageTable = []page{
		// The write area: the document, and the six pages beside it.
		{
			area: areaWrite, sub: subNotes, name: "notes",
			label: func(m locales.Messages) string { return m.DocumentPage() },
			form:  formWorkspace, detail: true,
			view: func(a *App, c *ui.Context) { notes.View(c, &a.notes, a.notesDeps(c)) },
		},
		{
			area: areaWrite, sub: subNoteList, name: "list",
			label: func(m locales.Messages) string { return m.ModeNotes() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { notes.ListView(c, &a.notes, a.notesDeps(c)) },
			load:  func(a *App) { a.loadNoteList() },
		},
		{
			area: areaWrite, sub: subSearch, name: "search",
			label: func(m locales.Messages) string { return m.SearchPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { notes.SearchView(c, &a.notes, a.notesDeps(c)) },
			load:  func(a *App) { a.loadSearch() },
		},
		{
			area: areaWrite, sub: subTags, name: "tags",
			label: func(m locales.Messages) string { return m.WriteTagsTitle() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { notes.TagsView(c, &a.notes, a.notesDeps(c)) },
			load:  func(a *App) { a.loadTagRows() },
		},
		{
			area: areaWrite, sub: subAssets, name: "assets",
			label: func(m locales.Messages) string { return m.WriteAssetsTitle() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { notes.AssetsView(c, &a.notes, a.notesDeps(c)) },
			load:  func(a *App) { a.loadAssetRows() },
		},
		{
			area: areaWrite, sub: subTemplates, name: "templates",
			label: func(m locales.Messages) string { return m.WriteTemplatesTitle() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { notes.TemplatesView(c, &a.notes, a.notesDeps(c)) },
		},
		{
			area: areaWrite, sub: subTree, name: "tree",
			label: func(m locales.Messages) string { return m.WriteTreeTitle() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { notes.TreeView(c, &a.notes, a.notesDeps(c)) },
			load:  func(a *App) { a.loadExcerptTree() },
		},

		// The read area: today's queue and the archive behind it.
		{
			area: areaRead, sub: subReadQueue, name: "queue",
			label: func(m locales.Messages) string { return m.QueuePage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { read.View(c, &a.read, a.readDeps(c)) },
			load:  func(a *App) { a.read.Show(read.Queue) },
		},
		{
			area: areaRead, sub: subReadArchive, name: "archive",
			label: func(m locales.Messages) string { return m.ArchivePage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { read.View(c, &a.read, a.readDeps(c)) },
			load:  func(a *App) { a.read.Show(read.Archive) },
		},

		// The recall area: the queue, the card table, the types, the presets, the
		// history and the broken links.
		{
			area: areaRecall, sub: subRecallQueue, name: "queue",
			label: func(m locales.Messages) string { return m.QueuePage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.recallView(c) },
			load:  func(a *App) { a.loadRecall() },
		},
		{
			area: areaRecall, sub: subRecallCards, name: "cards",
			label: func(m locales.Messages) string { return m.CardsPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.cardTableView(c) },
			load:  func(a *App) { a.loadCards() },
		},
		{
			area: areaRecall, sub: subRecallTypes, name: "types",
			label: func(m locales.Messages) string { return m.TypesPage() },
			form:  formSplit,
			view:  func(a *App, c *ui.Context) { a.typesView(c) },
			// The type list is retaken with the library, so the page reads it.
		},
		{
			area: areaRecall, sub: subRecallPresets, name: "presets",
			label: func(m locales.Messages) string { return m.PresetsPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.presetsView(c) },
			load:  func(a *App) { a.loadPresets() },
		},
		{
			area: areaRecall, sub: subRecallHistory, name: "history",
			label: func(m locales.Messages) string { return m.HistoryPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.historyView(c) },
			load:  func(a *App) { a.loadHistory() },
		},
		{
			area: areaRecall, sub: subRecallRelink, name: "relink",
			label: func(m locales.Messages) string { return m.RelinkPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.relinkPageView(c) },
			load:  func(a *App) { a.loadRelink() },
		},

		// The statistics area: the summary, the calendar and the forecast, each a
		// page of its own.
		{
			area: areaStats, sub: subStatsOverview, name: "overview",
			label: func(m locales.Messages) string { return m.OverviewPage() },
			form:  formPaper,
			view: func(a *App, c *ui.Context) {
				screenstats.View(c, screenstats.Overview, a.stats, a.statsDeps())
			},
			load: func(a *App) { a.loadStats() },
		},
		{
			area: areaStats, sub: subStatsHeatmap, name: "heatmap",
			label: func(m locales.Messages) string { return m.StatsHeat() },
			form:  formPaper,
			view: func(a *App, c *ui.Context) {
				screenstats.View(c, screenstats.HeatmapPage, a.stats, a.statsDeps())
			},
			load: func(a *App) { a.loadStats() },
		},
		{
			area: areaStats, sub: subStatsForecast, name: "forecast",
			label: func(m locales.Messages) string { return m.StatsForecastTitle() },
			form:  formPaper,
			view: func(a *App, c *ui.Context) {
				screenstats.View(c, screenstats.ForecastPage, a.stats, a.statsDeps())
			},
			load: func(a *App) { a.loadStats() },
		},

		// The settings area: the app's pages, which draw without a library.
		{
			area: areaSettings, sub: subSettingsAppearance, name: "appearance",
			label: func(m locales.Messages) string { return m.AppearancePage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.settingsView(c) },
			load:  func(a *App) { a.syncPrefs() },
		},
		{
			area: areaSettings, sub: subSettingsReview, name: "review",
			label: func(m locales.Messages) string { return m.ReviewPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.settingsView(c) },
			load:  func(a *App) { a.syncPrefs() },
		},
		{
			area: areaSettings, sub: subSettingsBudget, name: "budget",
			label: func(m locales.Messages) string { return m.BudgetPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.settingsView(c) },
			load:  func(a *App) { a.syncPrefs() },
		},
		{
			area: areaSettings, sub: subSettingsKeys, name: "keys",
			label: func(m locales.Messages) string { return m.KeysPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.settingsView(c) },
			load:  func(a *App) { a.syncPrefs() },
		},
		{
			area: areaSettings, sub: subSettingsPlugins, name: "plugins",
			label: func(m locales.Messages) string { return m.SettingsPlugins() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.settingsView(c) },
			load:  func(a *App) { a.syncPrefs() },
		},
		{
			area: areaSettings, sub: subSettingsAgent, name: "agent",
			label: func(m locales.Messages) string { return m.AgentPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.settingsView(c) },
			load:  func(a *App) { a.syncPrefs() },
		},
		{
			area: areaSettings, sub: subSettingsUpdate, name: "update",
			label: func(m locales.Messages) string { return m.UpdatePage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.settingsView(c) },
			load:  func(a *App) { a.syncPrefs() },
		},
		{
			area: areaSettings, sub: subSettingsAbout, name: "about",
			label: func(m locales.Messages) string { return m.AboutPage() },
			form:  formPaper,
			view:  func(a *App, c *ui.Context) { a.settingsView(c) },
			load:  func(a *App) { a.syncPrefs() },
		},
	}
}

// pagesOf is an area's pages in the order its sub-navigation shows them,
// leaving out a place entered from a page.
func pagesOf(area areaID) []page {
	var out []page
	for _, p := range pageTable {
		if p.area == area && !p.detail {
			out = append(out, p)
		}
	}
	return out
}

// pageOf is the page at a place, whether or not the sub-navigation offers it.
func pageOf(area areaID, sub subID) (page, bool) {
	for _, p := range pageTable {
		if p.area == area && p.sub == sub {
			return p, true
		}
	}
	return page{}, false
}

// pageAt is the page at a place: the one the table declares there, or the area's
// first when the place is not one the area has, so that a request for a page puts
// the user somewhere rather than nowhere. An area that declares no page of its
// own — today, a plugin's page — is a place with no page: the zero page has no
// name and no view, and the area's own screen answers for it.
func pageAt(area areaID, sub subID) page {
	if p, ok := pageOf(area, sub); ok {
		return p
	}
	if p, ok := pageOf(area, defaultSub(area)); ok {
		return p
	}
	return page{area: area}
}

// pageOfName is the page an id names within area. A name a link spells that the
// area does not have is not a page: the caller decides where such a request
// lands.
func pageOfName(area areaID, name string) (page, bool) {
	for _, p := range pageTable {
		if p.area == area && p.name == name {
			return p, true
		}
	}
	return page{}, false
}

// settingsView draws the settings page the preferences were last synced to:
// every settings page is one screen that reads where it is from the state.
func (a *App) settingsView(c *ui.Context) {
	screensettings.View(c, &a.prefs, a.settingsDeps())
}
