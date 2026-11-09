package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/pluginpage"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
)

// CommandRunner is how a contributed command's callback is called: one hook with
// JSON-shaped parameters and a JSON-shaped answer. *ext.JSRunner is the one the
// app uses; a test hands in its own to count what a frame did not call.
type CommandRunner interface {
	Call(method string, params, out any) error
}

// CommandHook is the export a plugin answers its contributed commands and its
// pages' data with: command({name, context}), answering either a command's
// result or a page's node tree. One hook, because a page's data command is a
// command (docs/specs/plugins/ui.md §5, §9). It is ext's name for the hook, so
// the loader, the checker and the window cannot drift apart.
const CommandHook = ext.CommandHook

// pluginUI is the plugins' declared UI as the window uses it: parsed once when
// the library opens, so a frame only reads it. Nothing here is a plugin call —
// the runners are only reached from a handler, off the frame.
type pluginUI struct {
	index contrib.Index
	// runners is the runner that answers a plugin's callbacks, by manifest name,
	// opened on first use: a plugin with no command never starts.
	runners map[string]CommandRunner
	// pages is each page's state, by plugin and page id: the tree the last load
	// left, and the reader's choices in it. A page the window dropped keeps its
	// state until the library reopens.
	pages map[string]*pluginpage.State
	// pagesFor is each page's context hash, which is what a cache hit compares:
	// entering a page again with the same context keeps the tree it has.
	pagesFor map[string]string
	// loading marks the pages whose data is in flight, so a second entry asks
	// once.
	loading map[string]bool
	// failures counts each contribution's consecutive failures and remembers
	// what the third took out: the counts and the hidden set are one value, so
	// the settings page cannot read a count that disagrees with what shows.
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
	a.ui = pluginUI{
		runners:  map[string]CommandRunner{},
		pages:    map[string]*pluginpage.State{},
		pagesFor: map[string]string{},
		loading:  map[string]bool{},
		failures: contrib.NewFailures(),
		problems: map[string][]string{},
		names:    map[string]string{},
		area:     area,
		sub:      sub,
	}
	if a.lib == nil || a.manifests == nil {
		return
	}
	manifests, err := a.manifests()
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
	if a.mode == modePlugin {
		if _, ok := a.ui.index.Page(a.page.Plugin, a.page.Page.ID); !ok {
			a.mode = modeNotes
			a.page = contrib.PageRef{}
		}
	}
}

// contributionContext is the window as a when expression reads it: state the
// app already holds, so that a frame may ask it without a store read or a
// plugin call.
func (a *App) contributionContext() contrib.Context {
	ctx := contrib.Context{
		Mode:        a.modeName(),
		Area:        a.ui.area,
		Sub:         a.ui.sub,
		LibraryOpen: a.lib != nil && !a.start && a.openErr == nil,
		NoteOpen:    a.notes.Draft.ID != "",
	}
	if a.mode == modePlugin {
		ctx.PluginID, ctx.PageID = a.page.Plugin, a.page.Page.ID
	}
	if a.mode == modeBrowse {
		ctx.BrowseSelection = a.browse.Selected >= 0 && a.browse.Selected < len(a.browseRows)
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

// currentCard is the card the window is about now: the one in review, or the row
// browse has chosen. It is what a contributed command acts on.
func (a *App) currentCard() (core.ID, bool) {
	switch a.mode {
	case modeReview:
		return a.review.Current()
	case modeBrowse:
		if a.browse.Selected >= 0 && a.browse.Selected < len(a.browseRows) {
			return a.browseRows[a.browse.Selected].ID, true
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
		if ref.Page.Nav != contrib.NavPlugins || a.ui.failures.Hidden(ref.Plugin, ref.Page.ID) {
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
	runner, err := a.runnerFor(ref.Name)
	if err != nil {
		a.contributionFailed(c, ref.Plugin, ref.Command.ID, m(c).PluginCommandFailed(a.pluginText(c, ref.Plugin, string(ref.Command.Title)), err.Error()))
		return
	}
	input := map[string]any{"name": ref.Command.Action.Name, "context": a.contributionPayload()}
	a.background(c, func() func(*ui.Context) {
		var answer commandAnswer
		err := runner.Call(CommandHook, input, &answer)
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

// commandAnswer is what a command callback answers: a result the host applies,
// never a change to the host's state of its own.
type commandAnswer struct {
	Result  string `json:"result"`
	Page    string `json:"page"`
	Message string `json:"message"`
}

// applyCommandAnswer applies one answer's result on the UI thread: a page to
// open, a refresh of the page on screen, or a toast.
func (a *App) applyCommandAnswer(c *ui.Context, ref contrib.CommandRef, answer commandAnswer) {
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
		if a.mode == modePlugin {
			a.loadPage(c, a.page, true)
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
		a.setMode(modeReview)
	case "remit.command.editNotes":
		a.setMode(modeNotes)
	case "remit.command.browse":
		a.setMode(modeBrowse)
	case "remit.command.stats":
		a.setMode(modeStats)
	case "remit.command.newNote":
		a.newNote(c)
	case "remit.command.makeCard":
		a.makeCardAtCaret(c)
	}
}

// runnerFor is the runner that answers a plugin's callbacks, opened on first
// use: a plugin that contributes a menu item costs nothing until it is asked.
func (a *App) runnerFor(name string) (CommandRunner, error) {
	if runner, ok := a.ui.runners[name]; ok {
		return runner, nil
	}
	if a.contribution == nil {
		return nil, errNoPluginService
	}
	runner, err := a.contribution(name)
	if err != nil {
		return nil, err
	}
	a.ui.runners[name] = runner
	return runner, nil
}

// contributionPayload is the context a callback reads: the same keys a when
// expression reads, plus the note and card the command acts on.
func (a *App) contributionPayload() map[string]any {
	ctx := a.contributionContext()
	payload := map[string]any{
		"mode":         ctx.Mode,
		"library.open": ctx.LibraryOpen,
		"note.open":    ctx.NoteOpen,
		"card.present": ctx.CardPresent,
		"page.id":      ctx.PageID,
		"plugin.id":    ctx.PluginID,
	}
	if ctx.Area != "" {
		payload["area"] = ctx.Area
	}
	if ctx.Sub != "" {
		payload["sub"] = ctx.Sub
	}
	if ctx.CardStatus != "" {
		payload["card.status"] = ctx.CardStatus
	}
	if id, ok := a.currentCard(); ok {
		payload["card.id"] = string(id)
	}
	if a.mode == modeNotes && a.notes.Draft.ID != "" {
		payload["note.id"] = string(a.notes.Draft.ID)
	}
	return payload
}

// openPage shows a contributed page and asks for its data, off the frame.
func (a *App) openPage(c *ui.Context, ref contrib.PageRef) {
	if ref.Page.ID == "" {
		return
	}
	a.page = ref
	if a.mode != modePlugin {
		a.anim.mode++
	}
	a.mode = modePlugin
	a.loadPage(c, ref, false)
}

// loadPage asks a page's data command for its tree, off the frame. force asks
// again when the window is already holding an answer for this context, which is
// what a retry and a refresh result do.
func (a *App) loadPage(c *ui.Context, ref contrib.PageRef, force bool) {
	key := pageKey(ref.Plugin, ref.Page.ID)
	if a.ui.failures.Hidden(ref.Plugin, ref.Page.ID) {
		return
	}
	state := a.pageState(ref)
	payload := a.contributionPayload()
	hash := contextHash(payload)
	if !force && state.Err == "" && state.Tree.Kind != "" && a.ui.pagesFor[key] == hash {
		return
	}
	if a.ui.loading[key] {
		return
	}
	runner, err := a.runnerFor(ref.Name)
	if err != nil {
		state.Err = err.Error()
		state.Loading = false
		a.pageFailed(ref, err)
		return
	}
	state.Loading = true
	state.Err = ""
	a.ui.loading[key] = true
	input := map[string]any{"name": ref.Page.Data, "context": payload}
	a.background(c, func() func(*ui.Context) {
		var raw json.RawMessage
		err := runner.Call(CommandHook, input, &raw)
		return func(c *ui.Context) {
			delete(a.ui.loading, key)
			state := a.pageState(ref)
			state.Loading = false
			if err != nil {
				state.Err = err.Error()
				a.pageFailed(ref, err)
				return
			}
			tree, err := contrib.ParsePage(raw)
			if err != nil {
				state.Err = err.Error()
				a.pageFailed(ref, err)
				return
			}
			a.ui.failures.Succeeded(ref.Plugin, ref.Page.ID)
			a.ui.pagesFor[key] = hash
			state.Tree = tree
			state.Err = ""
		}
	})
}

// pageFailed counts a page's failure, says so in the page (which shows its error
// state and a retry) and hides that page — and only that page — at the third.
func (a *App) pageFailed(ref contrib.PageRef, err error) {
	slog.Warn("remit: a plugin page failed", "plugin", ref.Name, "page", ref.Page.ID, "err", err)
	if !a.ui.failures.Failed(ref.Plugin, ref.Page.ID, err.Error()) {
		return
	}
	slog.Warn("remit: a plugin page was hidden after three failures", "plugin", ref.Name, "page", ref.Page.ID)
	if a.mode == modePlugin && a.page.Plugin == ref.Plugin && a.page.Page.ID == ref.Page.ID {
		a.mode = modeNotes
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

// pageState answers one page's state, made on first use.
func (a *App) pageState(ref contrib.PageRef) *pluginpage.State {
	key := pageKey(ref.Plugin, ref.Page.ID)
	state := a.ui.pages[key]
	if state == nil {
		state = &pluginpage.State{}
		a.ui.pages[key] = state
	}
	state.Title = string(ref.Page.Title)
	return state
}

// pluginPageView draws the page on screen. The screen only reads what the last
// load left: a frame never asks a plugin for anything. A page whose cache key no
// longer matches what it was loaded in — a write invalidated it — asks again off
// this frame, which is how the contract's event refreshes an open page.
func (a *App) pluginPageView(c *ui.Context) {
	ref := a.page
	if ref.Page.ID == "" {
		return
	}
	state := a.pageState(ref)
	key := pageKey(ref.Plugin, ref.Page.ID)
	// A page that has been drawn before has its runner; opening one is the
	// handler's job (openPage), so a frame never starts a plugin.
	_, ready := a.ui.runners[ref.Name]
	if ready && state.Err == "" && !a.ui.loading[key] && a.ui.pagesFor[key] != contextHash(a.contributionPayload()) {
		a.loadPage(c, ref, false)
	}
	pluginpage.View(c, state, pluginpage.Deps{
		Message: func(c *ui.Context, raw string) string { return a.pluginText(c, ref.Plugin, raw) },
		Run:     func(c *ui.Context, command string) { a.runPageCommand(c, ref, command) },
		Retry:   func(c *ui.Context) { a.loadPage(c, ref, true) },
	})
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

// invalidatePages forgets what each page's cache key was, so that the next entry
// asks its plugin again: a page is what the library was when it loaded, and a
// write makes it old. A page on screen asks again on the next frame, off that
// frame (ui.md §9).
func (a *App) invalidatePages() {
	clear(a.ui.pagesFor)
}

// LibraryEvent is the library saying something was written. It is the one call
// the event handler makes into the window, so it takes no lock and touches no
// state: the next frame picks the flag up on the UI thread, where the page
// states live.
func (a *App) LibraryEvent() {
	a.stale.Store(true)
}

// contextHash is a page load's cache key: the context it was asked in, as JSON,
// hashed. The same page in the same window state keeps its tree.
func contextHash(payload map[string]any) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:8])
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
// entries.
func (a *App) reenablePlugin(c *ui.Context, name string) {
	for id, idName := range a.ui.names {
		if idName != name {
			continue
		}
		a.ui.failures.Reenable(id)
	}
	a.refreshSettingsData()
	c.Toast(m(c).PluginReenabled())
}

// contributionIssues is the settings page's one snapshot of what the window is
// not showing: what a manifest declared that this host could not use, and the
// contributions three failures took out, each with its plugin, its contribution
// id, the reason and whether failures hid it. It is data — it reads what the load
// and the counters left, and calls no plugin.
func (a *App) contributionIssues() []contrib.Issue {
	return a.ui.failures.Issues(a.ui.problems)
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

// pluginFailures counts a plugin's failed contributions by manifest name.
func (a *App) pluginFailures(name string) int {
	total := 0
	for id, idName := range a.ui.names {
		if idName == name {
			total += a.ui.failures.Total(id)
		}
	}
	return total
}

// errNoPluginService is a build with no plugin service: a contribution that
// calls back says so in its page or its toast, and the window goes on.
var errNoPluginService = errorString("remit: this build has no plugin service")

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
