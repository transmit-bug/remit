package ui

import (
	"encoding/json"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/pages"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/pluginpage"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
)

// pluginUI is the plugins' declared UI as the window uses it: parsed once when
// the library opens, so a frame only reads it. Nothing here is a plugin call —
// the runners are only reached from a handler or from a job that runs off the
// frame — and the pages' own rules are the runtime's, not this struct's.
type pluginUI struct {
	index contrib.Index
	// commanders is what answers a plugin's callbacks, by manifest name, opened
	// on first use: a plugin with no command never starts.
	commanders map[string]ext.Commander
	// pages is the page runtime: each page's tree, error, in-flight flag, the
	// context it was loaded in and its failures. One field and one state
	// machine (docs/specs/plugin-system.md P5): the window reads it and runs the
	// jobs it hands out, and never keeps a rule of its own beside it.
	pages *pages.Runtime
	// views is each page's drawing state — the tab chosen, the branch opened, the
	// node the window pointed at — by plugin and page id. It is the screen's, and
	// stays in the window because the runtime may not know what a page looks
	// like: it holds data, never a control.
	views map[string]*pluginpage.State
	// failures counts each contribution's consecutive failures and remembers
	// what the third took out: the counts and the hidden set are one value, so
	// the settings page cannot read a count that disagrees with what shows. A
	// page's own count is the runtime's; this one is the command's and the menu
	// item's.
	failures *contrib.Failures
	// problems is what each plugin's manifest declared that this host cannot
	// use, by manifest id, for the log and the settings page.
	problems map[string][]string
	// names is each plugin's manifest id to its name, for the settings page's
	// per-plugin counts.
	names map[string]string
	// area and sub are the part of the window the route shows, which is what the
	// area and sub context keys answer. The route sets them: notes covers three
	// areas — today, write, read — so mode cannot say which one a receiver is in.
	area, sub string
}

// pageKey names a page's state. Ids cannot hold a slash or a control byte, so
// one id can never be read as another's.
func pageKey(plugin, id string) string { return plugin + "/" + id }

// loadContributions reads what the enabled plugins declare for the window. It
// starts nothing: the manifest is the whole input, so this is the same parse
// `ext check` runs, and a broken entry is a log line and a settings-page row
// rather than a plugin that will not enable.
func (a *App) loadContributions() {
	area, sub := a.ui.area, a.ui.sub
	// The page runtime outlives a reload of the declarations: turning a plugin on
	// or off is no reason to forget what a page already answered or how often it
	// failed. A window also has one from its first frame, so that a route can ask
	// which page is on screen before a library opens.
	pageRuntime := a.ui.pages
	a.ui = pluginUI{
		commanders: map[string]ext.Commander{},
		views:      map[string]*pluginpage.State{},
		failures:   contrib.NewFailures(),
		problems:   map[string][]string{},
		names:      map[string]string{},
		pages:      pageRuntime,
		area:       area,
		sub:        sub,
	}
	if a.ui.pages == nil {
		a.ui.pages = pages.New()
	}
	if a.lib == nil {
		return
	}
	manifests, err := a.plugins.Manifests()
	if err != nil {
		slog.Warn("remit: discover plugins", "err", err)
	}
	enabled := map[string]bool{}
	for _, name := range a.lib.EnabledExtensions() {
		enabled[name] = true
	}
	plugins := make([]contrib.Contributions, 0, len(manifests))
	for _, extension := range manifests {
		if !enabled[extension.Manifest.Name] {
			continue
		}
		parsed, problems := contrib.Parse(extension, contrib.Options{Icon: design.HasIcon})
		for _, problem := range problems {
			slog.Warn("remit: a plugin's contribution was dropped", "plugin", extension.Manifest.Name, "problem", problem)
			a.ui.problems[extension.Manifest.ID] = append(a.ui.problems[extension.Manifest.ID], problem.Error())
		}
		plugins = append(plugins, parsed)
	}
	index, problems := contrib.BuildIndex(plugins)
	for _, problem := range problems {
		slog.Warn("remit: two plugins claim one contribution", "problem", problem)
	}
	a.ui.index = index
	for _, plugin := range index.Plugins() {
		a.ui.names[plugin.Plugin] = plugin.Name
	}
	// A page on screen whose plugin went away, or whose page did: the window
	// returns to the notes rather than drawing a page that no longer exists.
	if a.plugin.Page.ID != "" {
		if _, ok := a.ui.index.Page(a.plugin.Plugin, a.plugin.Page.ID); !ok {
			a.goTo(areaWrite, subNotes)
		}
	}
}

// contributionContext is the window as a when expression reads it: state the
// app already holds, so that a frame may ask it without a store read or a
// plugin call. Every key the contract names answers here, and answers what the
// frame is drawing now: a key this host left empty would hide a contribution
// that a manifest asked for correctly.
func (a *App) contributionContext() contrib.Context {
	ctx := contrib.Context{
		Mode:          a.modeName(),
		Area:          a.ui.area,
		Sub:           a.ui.sub,
		LibraryOpen:   a.lib != nil && !a.start && a.openErr == nil,
		NoteOpen:      a.notes.Draft.ID != "",
		NoteSelection: a.notes.HasSelection(),
		ScopeActive:   a.reviewScope.Kind != queue.ScopeAll,
	}
	if a.mode == modePlugin {
		page := a.plugin
		ctx.PluginID, ctx.PageID = page.Plugin, page.Page.ID
	}
	if a.mode == modeBrowse && a.area == areaRecall && a.sub == subRecallCards {
		// The table is on screen: a row the keys are on, or the rows the user
		// checked for an action, is what browse.selection answers.
		ctx.BrowseSelection = a.cardTable.Selected >= 0 || len(a.cardTable.Chosen(a.cardRows)) > 0
	}
	if id, ok := a.currentCard(); ok {
		if card, ok := a.data.card(id); ok {
			ctx.CardPresent = true
			ctx.CardStatus = card.Status.String()
		}
	}
	return ctx
}

// setContributionArea records the part of the window the route is showing, which
// is what the area and sub context keys answer: a slot's items can ask about the
// area they sit in, which mode cannot say for the three areas inside notes. The
// route calls it; no other caller changes it.
func (a *App) setContributionArea(area, sub string) {
	a.ui.area, a.ui.sub = area, sub
}

// modeName is the screen a when expression reads as mode. A plugin page is not
// one of the host's five screens — page.id says which page it is — so it reads
// empty rather than borrowing a screen it is not.
func (a *App) modeName() string {
	switch a.mode {
	case modeNotes:
		return "notes"
	case modeReview:
		return "review"
	case modeBrowse:
		return "browse"
	case modeStats:
		return "stats"
	case modeSettings:
		return "settings"
	}
	return ""
}

// currentCard is the card the window is about now: the one the review session
// shows, or the row the card table has chosen. It is asked of what is on screen,
// so a screen that merely could show a card — the recall queue, the write area's
// search — answers no card rather than one the user cannot see.
func (a *App) currentCard() (core.ID, bool) {
	switch {
	case a.session == sessionReview:
		return a.review.Current()
	case a.area == areaRecall && a.sub == subRecallCards:
		if a.cardTable.Selected >= 0 && a.cardTable.Selected < len(a.cardRows) {
			return a.cardRows[a.cardTable.Selected].ID, true
		}
	}
	return "", false
}

// pluginText resolves one of a plugin's own messages in this frame's language.
func (a *App) pluginText(c *ui.Context, plugin, raw string) string {
	contributions, ok := a.ui.index.Plugin(plugin)
	if !ok {
		return raw
	}
	return contributions.Text(c.Locale(), raw)
}

// visiblePages lists the contributed pages navigation offers now: the ones the
// plugin put in the plugin group, whose when holds for this window, and whose
// data has not failed three times.
func (a *App) visiblePages() []contrib.PageRef {
	ctx := a.contributionContext()
	var out []contrib.PageRef
	for _, ref := range a.ui.index.Pages() {
		if ref.Page.Nav != contrib.NavPlugins || a.ui.pages.Hidden(ref.Plugin, ref.Page.ID) {
			continue
		}
		if !ref.Page.When.Eval(ctx) {
			continue
		}
		out = append(out, ref)
	}
	return out
}

// visibleCommands lists the contributed commands a when expression shows now:
// what the palette lists and what the menu slots draw.
func (a *App) visibleCommands() []contrib.CommandRef {
	ctx := a.contributionContext()
	var out []contrib.CommandRef
	for _, ref := range a.ui.index.Commands() {
		if a.ui.failures.Hidden(ref.Plugin, ref.Command.ID) || !ref.Command.When.Eval(ctx) {
			continue
		}
		out = append(out, ref)
	}
	return out
}

// slotItems lists the contributed items the given slots hold now, in drawing
// order and once each: what a receiver draws. It reads the index the library load
// left and evaluates the cached when expressions — a frame never asks a plugin
// for a menu item (ui.md §9).
func (a *App) slotItems(slots ...string) []contrib.MenuRef {
	var out []contrib.MenuRef
	seen := map[string]bool{}
	ctx := a.contributionContext()
	for _, slot := range slots {
		for _, item := range a.ui.index.Menu(slot) {
			ref := item.CommandRef
			if seen[ref.Command.ID] || a.ui.failures.Hidden(ref.Plugin, ref.Command.ID) {
				continue
			}
			// The item's when and the command's both decide: a plugin can put
			// one command in two slots and show it in only one.
			if !item.When.Eval(ctx) || !ref.Command.When.Eval(ctx) {
				continue
			}
			seen[ref.Command.ID] = true
			out = append(out, item)
		}
	}
	return out
}

// contributionMenuItems lists the contributed commands the given slots hold now,
// in drawing order and once each: the toolbar's overflow and a note's are one menu
// in this window, so a caller names both slots and gets one list.
func (a *App) contributionMenuItems(slots ...string) []contrib.CommandRef {
	items := a.slotItems(slots...)
	if len(items) == 0 {
		return nil
	}
	out := make([]contrib.CommandRef, len(items))
	for i, item := range items {
		out[i] = item.CommandRef
	}
	return out
}

// overflowItems is the window's `⋯` menu: the toolbar's overflow and a note's,
// merged, one command once.
func (a *App) overflowItems() []contrib.MenuRef {
	return a.slotItems(contrib.SlotToolbarOverflow, contrib.SlotNoteOverflow)
}

// noteBlockItems is the block menu's and the block inspector's plugin section.
func (a *App) noteBlockItems() []contrib.MenuRef { return a.slotItems(contrib.SlotNoteBlock) }

// slashItems is the `/` insert menu's plugin section.
func (a *App) slashItems() []contrib.MenuRef { return a.slotItems(contrib.SlotEditorSlash) }

// reviewCardItems is the review session's plugin actions, for the card on screen.
func (a *App) reviewCardItems() []contrib.MenuRef { return a.slotItems(contrib.SlotReviewCard) }

// browseRowItems is the card table's plugin actions, for the chosen rows.
func (a *App) browseRowItems() []contrib.MenuRef { return a.slotItems(contrib.SlotBrowseRow) }

// sidebarNoteItems is a note row's menu in the sidebar.
func (a *App) sidebarNoteItems() []contrib.MenuRef { return a.slotItems(contrib.SlotSidebarNote) }

// pageHeaderItems is a contributed page's header actions.
func (a *App) pageHeaderItems() []contrib.MenuRef { return a.slotItems(contrib.SlotPageHeader) }

// todayWidgetItems is the today screen's plugin block: one section per plugin
// with items here.
func (a *App) todayWidgetItems() []contrib.MenuRef { return a.slotItems(contrib.SlotTodayWidget) }

// statsBlockItems is the stats overview's plugin block.
func (a *App) statsBlockItems() []contrib.MenuRef { return a.slotItems(contrib.SlotStatsBlock) }

// pluginPaletteActions appends the contributed commands the window shows now to
// the palette's own.
func (a *App) pluginPaletteActions(c *ui.Context) []paletteAction {
	refs := a.visibleCommands()
	if len(refs) == 0 {
		return nil
	}
	out := make([]paletteAction, 0, len(refs))
	for _, ref := range refs {
		command := ref
		out = append(out, paletteAction{
			label: a.pluginText(c, command.Plugin, string(command.Command.Title)),
			run:   func() { a.runContribution(c, command) },
		})
	}
	return out
}

// runContribution runs one contributed command. A host action is the app's own,
// and runs here; a callback runs off the frame, and what it answers is applied
// at the start of a later one — a command never changes the app's state itself.
func (a *App) runContribution(c *ui.Context, ref contrib.CommandRef) {
	if host := ref.Command.Action.Host; host != "" {
		a.runHostAction(c, host)
		return
	}
	commander, err := a.commanderFor(ref.Name)
	if err != nil {
		a.contributionFailed(c, ref.Plugin, ref.Command.ID, m(c).PluginCommandFailed(a.pluginText(c, ref.Plugin, string(ref.Command.Title)), err.Error()))
		return
	}
	in := ext.CommandIn{Name: ref.Command.Action.Name, Context: a.contributionPayload()}
	a.background(c, func() func(*ui.Context) {
		answer, err := commander.RunCommand(in)
		return func(c *ui.Context) {
			if err != nil {
				a.contributionFailed(c, ref.Plugin, ref.Command.ID, m(c).PluginCommandFailed(a.pluginText(c, ref.Plugin, string(ref.Command.Title)), err.Error()))
				return
			}
			a.ui.failures.Succeeded(ref.Plugin, ref.Command.ID)
			a.applyCommandAnswer(c, ref, answer)
		}
	})
}

// applyCommandAnswer applies one answer's result on the UI thread: a page to
// open, a refresh of the page on screen, or a toast.
func (a *App) applyCommandAnswer(c *ui.Context, ref contrib.CommandRef, answer ext.CommandResult) {
	switch answer.Result {
	case "", "none":
	case "navigate":
		page, ok := a.ui.index.Page(ref.Plugin, answer.Page)
		if !ok {
			page, ok = a.ui.index.PageByID(answer.Page)
		}
		if !ok {
			slog.Warn("remit: a plugin command asked for a page that is not there",
				"plugin", ref.Name, "page", answer.Page)
			return
		}
		a.openPage(c, page)
	case "refresh":
		if a.plugin.Page.ID != "" {
			a.enterPage(c, a.plugin, true)
		}
	case "toast":
		c.Toast(a.pluginText(c, ref.Plugin, answer.Message))
	default:
		slog.Warn("remit: a plugin command answered a result the host does not have",
			"plugin", ref.Name, "result", answer.Result)
	}
}

// runHostAction runs one of the app's own actions, by the name a manifest used.
// The list is contrib.HostActions': a name outside it never gets here.
func (a *App) runHostAction(c *ui.Context, name string) {
	switch name {
	case "remit.command.openSettings":
		a.openSettings()
	case "remit.command.reviewCards":
		a.goTo(areaRecall, subRecallQueue)
	case "remit.command.editNotes":
		a.goTo(areaWrite, subNotes)
	case "remit.command.browse":
		a.goTo(areaRecall, subRecallCards)
	case "remit.command.stats":
		a.goTo(areaStats, subStatsOverview)
	case "remit.command.newNote":
		a.newNote(c)
	case "remit.command.makeCard":
		a.makeCardAtCaret(c)
	}
}

// commanderFor is what answers a plugin's callbacks, opened on first use: a
// plugin that contributes a menu item costs nothing until it is asked.
func (a *App) commanderFor(name string) (ext.Commander, error) {
	if commander, ok := a.ui.commanders[name]; ok {
		return commander, nil
	}
	runner, err := a.plugins.Runner(name)
	if err != nil {
		return nil, err
	}
	commander := ext.HooksOf(runner)
	a.ui.commanders[name] = commander
	return commander, nil
}

// contributionPayload is the context a callback reads: every key the contract
// names, with the value a when expression read it as, plus the note and card the
// command acts on.
func (a *App) contributionPayload() map[string]any {
	payload := a.contributionContext().Payload()
	if id, ok := a.currentCard(); ok {
		payload["card.id"] = string(id)
	}
	if a.mode == modeNotes && a.notes.Draft.ID != "" {
		payload["note.id"] = string(a.notes.Draft.ID)
	}
	return payload
}

// pagePayload is the context a page's data command reads: the window's own, with
// the page it is entering named as it. The runtime learns which page is on screen
// when Enter returns, so the page cannot be read off the window before that — and
// the key has to be the same one every frame would build, or the same page would
// ask again on the next frame.
func (a *App) pagePayload(ref contrib.PageRef) map[string]any {
	payload := a.contributionPayload()
	payload["page.id"], payload["plugin.id"] = ref.Page.ID, ref.Plugin
	return payload
}

// openPage shows a contributed page: what its last load left, or its skeleton,
// and the job that asks for the page's data, run off this frame. The place is
// what remembers it, so back, a session's return and the rail all land on that
// page rather than on whichever one the runtime entered last.
func (a *App) openPage(c *ui.Context, ref contrib.PageRef) {
	if ref.Page.ID == "" {
		return
	}
	a.goToPlace(pluginPlace(ref))
	a.enterPage(c, ref, false)
}

// enterPage enters a page and runs the job the runtime hands out, when it hands
// one out: an enter whose tree is the one this context loaded is a read, and a
// second one while a load is out asks nothing.
func (a *App) enterPage(c *ui.Context, ref contrib.PageRef, retry bool) {
	if ref.Page.ID == "" {
		return
	}
	var job *pages.Job
	if retry {
		_, job = a.ui.pages.Retry(ref, a.pagePayload(ref))
	} else {
		_, job = a.ui.pages.Enter(ref, a.pagePayload(ref))
	}
	if job != nil {
		a.askPage(c, job)
	}
}

// askPage runs one page's data call off the frame and hands the answer back on
// the UI thread. The plugin is opened here, in a handler: a frame never starts
// one, and a job it cannot run is dropped rather than answered with a failure.
func (a *App) askPage(c *ui.Context, job *pages.Job) {
	commander, err := a.commanderFor(job.Plugin)
	if err != nil {
		a.applyPage(job, nil, err)
		return
	}
	a.background(c, func() func(*ui.Context) {
		tree, err := commander.PageData(job.In)
		return func(*ui.Context) { a.applyPage(job, tree, err) }
	})
}

// applyPage takes a job's answer: the runtime keeps the tree, or counts the
// failure and hides the page at the third. A page the third failure hid is one
// the window cannot draw any more, so the route leaves it rather than naming a
// page that is not there.
func (a *App) applyPage(job *pages.Job, tree []byte, err error) {
	if !a.ui.pages.Apply(job, tree, err) {
		return
	}
	// The third failure hid the page: the route leaves it rather than naming a
	// page the window cannot draw any more.
	if pageArea(a.plugin) == pageArea(job.Ref) {
		a.goTo(areaWrite, subNotes)
	}
}

// contributionFailed counts a command's failure, toasts what went wrong, and
// hides that command — and only that command — at the third.
func (a *App) contributionFailed(c *ui.Context, plugin, id, message string) {
	key := contrib.FailureKey(plugin, id)
	slog.Warn("remit: a plugin command failed", "contribution", key)
	hidden := a.ui.failures.Failed(plugin, id, message)
	if message != "" {
		c.Toast(message)
	}
	if hidden {
		slog.Warn("remit: a plugin command was hidden after three failures", "contribution", key)
	}
}

// pageState answers one page's drawing state — its highlight, its chosen tab,
// its open branches — made on first use. The page's data is the runtime's, never
// this: a frame draws what the last load left and decides no content here.
func (a *App) pageState(ref contrib.PageRef) *pluginpage.State {
	if a.ui.views == nil {
		a.ui.views = map[string]*pluginpage.State{}
	}
	key := pageKey(ref.Plugin, ref.Page.ID)
	state := a.ui.views[key]
	if state == nil {
		state = &pluginpage.State{}
		a.ui.views[key] = state
	}
	return state
}

// pluginPageView draws the page on screen. The runtime answers what the last
// load left and, when the context moved under it, the job that asks again: that
// job runs off this frame and its answer is drawn on a later one. A page whose
// commander is not open yet is a page no reader entered — opening a plugin is a
// handler's job — so the job is dropped rather than started here.
func (a *App) pluginPageView(c *ui.Context) {
	ref := a.plugin
	if ref.Page.ID == "" {
		return
	}
	view, job := a.ui.pages.Enter(ref, a.pagePayload(ref))
	if job != nil {
		if _, ready := a.ui.commanders[job.Plugin]; ready {
			a.askPage(c, job)
		} else {
			a.ui.pages.Abandon(job)
		}
	}
	pluginpage.View(c, a.pageState(ref), pluginpage.Data{
		Title:   string(ref.Page.Title),
		Tree:    view.Tree,
		Err:     view.Err,
		Loading: view.Loading,
	}, pluginpage.Deps{
		Message: func(c *ui.Context, raw string) string { return a.pluginText(c, ref.Plugin, raw) },
		Run:     func(c *ui.Context, command string) { a.runPageCommand(c, ref, command) },
		Retry:   func(c *ui.Context) { a.enterPage(c, ref, true) },
		Header:  a.pageHeaderActions(c),
		Node:    a.boardNodeInfo,
		OpenBlock: func(c *ui.Context, note, block string) {
			a.openBoardBlock(c, note, block)
		},
	})
}

// pageHeaderActions is a page's own header actions: the page.header slot's items
// for this window, in this frame's language, all read from the cache the library
// load left. A frame asks no plugin for them (ui.md §9).
func (a *App) pageHeaderActions(c *ui.Context) []pluginpage.Action {
	items := a.pageHeaderItems()
	if len(items) == 0 {
		return nil
	}
	actions := make([]pluginpage.Action, 0, len(items))
	for _, item := range items {
		command := item.CommandRef
		actions = append(actions, pluginpage.Action{
			Label: a.pluginText(c, command.Plugin, string(command.Command.Title)),
			Run:   func(c *ui.Context) { a.runContribution(c, command) },
		})
	}
	return actions
}

// runPageCommand runs a command one of a page's actions names: its own plugin's
// first, then any plugin's, which is the same reach a command's navigate result
// has.
func (a *App) runPageCommand(c *ui.Context, page contrib.PageRef, command string) {
	if ref, ok := a.ui.index.Command(command); ok {
		a.runContribution(c, ref)
		return
	}
	contributions, ok := a.ui.index.Plugin(page.Plugin)
	if !ok {
		return
	}
	if declared, ok := contributions.Command(command); ok {
		a.runContribution(c, contrib.CommandRef{Plugin: contributions.Plugin, Name: contributions.Name, Command: declared})
	}
}

// background runs one job away from the window and applies its answer on the UI
// thread. The app's own is a goroutine whose answer the next frame picks up;
// a test runs it inline, where there is only one thread.
func (a *App) background(c *ui.Context, job func() func(*ui.Context)) {
	if a.runBackground != nil {
		a.runBackground(c, job)
		return
	}
	// The services outlive the pass; the context does not.
	services := c.Services()
	go func() {
		apply := job()
		a.pending.mu.Lock()
		a.pending.jobs = append(a.pending.jobs, apply)
		a.pending.mu.Unlock()
		services.Invalidate()
	}()
}

// applyBackground applies what background jobs answered since the last frame.
// The work itself already ran off the frame; this is the UI thread taking its
// result, the one thing a frame ever does with a plugin's answer.
func (a *App) applyBackground(c *ui.Context) {
	a.pending.mu.Lock()
	jobs := a.pending.jobs
	a.pending.jobs = nil
	a.pending.mu.Unlock()
	for _, apply := range jobs {
		apply(c)
	}
}

// pending is the answers a background job left for the next frame.
type pendingWork struct {
	mu   sync.Mutex
	jobs []func(*ui.Context)
}

// LibraryEvent is the library saying something was written. It is the one call
// the event handler makes into the window, so it takes no lock and touches no
// state: the next frame picks the flag up on the UI thread, where the page
// states live, and makes them old.
func (a *App) LibraryEvent() {
	a.stale.Store(true)
}

// pluginSettings is the settings the enabled plugins declared, with the value
// this library holds for each: what the settings page's host form draws.
func (a *App) pluginSettings() []screensettings.Setting {
	if a.lib == nil {
		return nil
	}
	var out []screensettings.Setting
	for _, plugin := range a.ui.index.Plugins() {
		if len(plugin.Settings) == 0 {
			continue
		}
		values := a.pluginSettingValues(plugin.Plugin)
		for _, setting := range plugin.Settings {
			out = append(out, screensettings.Setting{
				Plugin:     plugin.Plugin,
				PluginName: plugin.Name,
				Key:        setting.Key,
				Type:       string(setting.Type),
				Title:      string(setting.Title),
				On:         a.boolSetting(values, setting),
				Number:     a.numberSetting(values, setting),
				Text:       a.textSetting(values, setting),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PluginName != out[j].PluginName {
			return out[i].PluginName < out[j].PluginName
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// setPluginSetting writes one contributed setting into the plugin's own store
// document, under its "settings" key: the host owns the write, and the plugin
// reads the value itself. A document that is not a JSON object is the plugin's
// own shape, and the host says it could not write rather than replacing it.
func (a *App) setPluginSetting(c *ui.Context, plugin, key, value string) {
	if a.lib == nil {
		return
	}
	a.prefs.PluginErr = ""
	document := map[string]json.RawMessage{}
	if raw, ok := a.lib.PluginData(plugin); ok && strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &document); err != nil {
			a.prefs.PluginErr = m(c).PluginSettingFailed(err.Error())
			return
		}
	}
	settings := map[string]json.RawMessage{}
	if current, ok := document["settings"]; ok {
		if err := json.Unmarshal(current, &settings); err != nil {
			a.prefs.PluginErr = m(c).PluginSettingFailed(err.Error())
			return
		}
	}
	contributions, ok := a.ui.index.Plugin(plugin)
	if !ok {
		return
	}
	typ := contrib.SettingType("")
	for _, setting := range contributions.Settings {
		if setting.Key == key {
			typ = setting.Type
		}
	}
	if typ == "" {
		a.prefs.PluginErr = m(c).PluginSettingFailed(errNoSetting.Error())
		return
	}
	encoded, err := settingJSON(typ, value)
	if err != nil {
		a.prefs.PluginErr = m(c).PluginSettingFailed(err.Error())
		return
	}
	settings[key] = encoded
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		a.prefs.PluginErr = m(c).PluginSettingFailed(err.Error())
		return
	}
	document["settings"] = settingsJSON
	documentJSON, err := json.Marshal(document)
	if err != nil {
		a.prefs.PluginErr = m(c).PluginSettingFailed(err.Error())
		return
	}
	if err := a.lib.SetPluginData(plugin, string(documentJSON)); err != nil {
		a.prefs.PluginErr = m(c).PluginSettingFailed(err.Error())
	}
	a.refreshSettingsData()
}

// settingJSON reads the form's value as the type the plugin declared.
func settingJSON(typ contrib.SettingType, value string) (json.RawMessage, error) {
	switch typ {
	case contrib.SettingBool:
		on, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return json.Marshal(on)
	case contrib.SettingNumber:
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		return json.Marshal(number)
	}
	return json.Marshal(value)
}

// reenablePlugin gives a plugin's failed contributions another try: the counts
// go back to zero and the hidden ones show again. It is the settings page's
// action beside the failure count (ui.md §8), and it moves only this plugin's
// entries — a command's in the counter, a page's in the runtime.
func (a *App) reenablePlugin(c *ui.Context, name string) {
	for id, idName := range a.ui.names {
		if idName != name {
			continue
		}
		a.ui.failures.Reenable(id)
		a.ui.pages.Reenable(id)
	}
	a.refreshSettingsData()
	c.Toast(m(c).PluginReenabled())
}

// contributionIssues is the settings page's one snapshot of what the window is
// not showing: what a manifest declared that this host could not use, and the
// contributions three failures took out — a command's in the counter, a page's
// in the runtime — each with its plugin, its contribution id, the reason and
// whether failures hid it. It is data: it reads what the load and the two counts
// left, and calls no plugin. The order is stable for the reason the counter's is:
// a settings page that reshuffles its rows on every frame reads as broken.
func (a *App) contributionIssues() []contrib.Issue {
	issues := a.ui.failures.Issues(a.ui.problems)
	for _, health := range a.ui.pages.Health() {
		if !health.Hidden {
			continue
		}
		issues = append(issues, contrib.Issue{
			Plugin:       health.Plugin,
			Contribution: health.Page,
			Reason:       health.Reason,
			Hidden:       true,
		})
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Plugin != issues[j].Plugin {
			return issues[i].Plugin < issues[j].Plugin
		}
		if issues[i].Contribution != issues[j].Contribution {
			return issues[i].Contribution < issues[j].Contribution
		}
		if issues[i].Hidden != issues[j].Hidden {
			return !issues[i].Hidden
		}
		return issues[i].Reason < issues[j].Reason
	})
	return issues
}

// pluginProblems is what each plugin's manifest could not offer, by manifest
// name: the settings page lists it under the plugin.
func (a *App) pluginProblems() map[string][]string {
	problems := map[string][]string{}
	for id, reasons := range a.ui.problems {
		name := a.ui.names[id]
		if name == "" {
			name = id
		}
		problems[name] = append(problems[name], reasons...)
	}
	return problems
}

// hiddenContributions names each contribution three failures took out, by
// manifest name, as the settings page lists it: which one it was and what failed,
// beside the one action that shows it again. It reads the counters the handlers
// left, so a frame may ask it.
func (a *App) hiddenContributions() map[string][]string {
	out := map[string][]string{}
	for _, issue := range a.contributionIssues() {
		if !issue.Hidden {
			continue
		}
		name := a.ui.names[issue.Plugin]
		if name == "" {
			name = issue.Plugin
		}
		line := issue.Contribution
		if issue.Reason != "" {
			line += ": " + issue.Reason
		}
		out[name] = append(out[name], line)
	}
	return out
}

// pluginFailures counts a plugin's failed contributions by manifest name: the
// commands the counter took out and the pages the runtime did.
func (a *App) pluginFailures(name string) int {
	total := 0
	for id, idName := range a.ui.names {
		if idName == name {
			total += a.ui.failures.Total(id)
			total += a.ui.pages.Total(id)
		}
	}
	return total
}

// errNoSetting is a setting the plugin no longer declares.
var errNoSetting = errorString("remit: the plugin no longer declares this setting")

type errorString string

func (e errorString) Error() string { return string(e) }

// pluginSettingValues reads the settings a plugin's document holds.
func (a *App) pluginSettingValues(plugin string) map[string]json.RawMessage {
	values := map[string]json.RawMessage{}
	if a.lib == nil {
		return values
	}
	raw, ok := a.lib.PluginData(plugin)
	if !ok || strings.TrimSpace(raw) == "" {
		return values
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		return values
	}
	if current, ok := document["settings"]; ok {
		_ = json.Unmarshal(current, &values)
	}
	return values
}

func (a *App) boolSetting(values map[string]json.RawMessage, setting contrib.Setting) bool {
	if raw, ok := values[setting.Key]; ok {
		var v bool
		if json.Unmarshal(raw, &v) == nil {
			return v
		}
	}
	fallback, _ := setting.Default.(bool)
	return fallback
}

func (a *App) numberSetting(values map[string]json.RawMessage, setting contrib.Setting) float64 {
	if raw, ok := values[setting.Key]; ok {
		var v float64
		if json.Unmarshal(raw, &v) == nil {
			return v
		}
	}
	fallback, _ := setting.Default.(float64)
	return fallback
}

func (a *App) textSetting(values map[string]json.RawMessage, setting contrib.Setting) string {
	if raw, ok := values[setting.Key]; ok {
		var v string
		if json.Unmarshal(raw, &v) == nil {
			return v
		}
	}
	fallback, _ := setting.Default.(string)
	return fallback
}
