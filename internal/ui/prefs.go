package ui

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/core/transfer"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/about"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
	"github.com/transmit-bug/remit/locales"
)

// errCannotUpdate is the settings page's report of a build without update
// keys, named here because the check the app runs is the page's default.
var errCannotUpdate = screensettings.ErrCannotUpdate

// loadSettings reads the app's settings from its data directory, so that a
// choice made in an earlier run is shown in this one. Settings that cannot be
// read leave the defaults and a log line: a broken settings file must not keep
// the window from opening.
func (a *App) loadSettings() {
	if a.settingsDir == "" {
		a.syncPrefs()
		return
	}
	s, err := settings.Load(a.settingsDir)
	if err != nil {
		slog.Warn("remit: read settings", "err", err)
		a.syncPrefs()
		return
	}
	a.settings = s
	if a.dir == "" {
		a.dir = s.LibraryDir
	}
	a.syncPrefs()
}

// Settings returns the app's preferences as the last read from or written to
// disk. It is a copy: change them with UpdateSettings, so the write, the page
// and the services that read one preference all see the change.
func (a *App) Settings() settings.Settings { return a.settings }

// UpdateSettings applies fn to the settings, writes them atomically and with
// owner-only permissions, reads them back and tells whoever registered
// OnSettings. What the app and its page show is then what the file holds; a
// write that fails leaves the settings as they were and returns the error.
func (a *App) UpdateSettings(fn func(*settings.Settings)) error {
	next := a.settings
	fn(&next)
	if a.settingsDir != "" {
		if err := settings.Save(a.settingsDir, next); err != nil {
			return err
		}
		saved, err := settings.Load(a.settingsDir)
		if err != nil {
			return err
		}
		next = saved
	}
	a.settings = next
	a.syncPrefs()
	if a.settingsChanged != nil {
		a.settingsChanged(next)
	}
	return nil
}

// OnSettings registers a function told about the settings whenever the app
// writes them, which is how the tray's pause item follows the page and the page
// follows the tray. Setting it again replaces the previous one.
func (a *App) OnSettings(fn func(settings.Settings)) {
	a.settingsChanged = fn
	if fn != nil {
		fn(a.settings)
	}
}

// remember writes the library the user chose into the settings, keeping every
// other preference, so the next start opens it without asking again.
func (a *App) remember(dir string) error {
	return a.UpdateSettings(func(s *settings.Settings) { s.LibraryDir = dir })
}

// openSettings shows the settings area, remembering where the user was so that
// Esc comes back to it, and reopening the page they were on last. The page
// itself is loaded by loadCurrent, so opening it is the same navigation as any
// other.
func (a *App) openSettings() {
	if a.area != areaSettings {
		a.prefsBack, a.prefsBackSub = a.area, a.sub
	}
	sub := a.settingsSub
	if !hasSub(areaSettings, sub) {
		sub = subSettingsAppearance
	}
	a.setArea(areaSettings, sub)
}

// closeSettings returns to the area the page was opened from.
func (a *App) closeSettings() {
	area, sub := a.prefsBack, a.prefsBackSub
	if area == areaSettings {
		area, sub = areaToday, subNone
	}
	a.setArea(area, sub)
}

// syncPrefs copies the settings, the library's presets and the agent API's own
// report into what the pages show. Every write ends here, so a failed one
// leaves a page showing what the file holds rather than what the user clicked.
func (a *App) syncPrefs() {
	a.prefs.Page = screensettings.Page(subName(a.sub))
	if a.area == areaSettings {
		a.settingsSub = a.sub
	}
	a.prefs.Appearance = a.settings.AppearanceName()
	a.prefs.PaperWidth = a.settings.PaperMeasure()
	a.prefs.Reminder = a.settings.Notifications
	a.prefs.AgentOn = a.settings.AgentServer
	a.prefs.UpdateCheck = a.settings.UpdateCheck
	a.prefs.Hotkey = a.settings.HotkeyAccelerator()
	a.prefs.Agent = screensettings.AgentStatus{}
	if a.agentAPI != nil {
		a.prefs.Agent = a.agentAPI.Status()
	}
	// A token is shown once, by the rotation that minted it: every other write
	// takes it away again.
	a.prefs.Token = ""
	a.prefs.Version = Version
	a.prefs.FormatVersion = store.FormatVersion
	a.prefs.AppName = Name
	a.prefs.LibraryDir = a.dir
	a.prefs.Notes = len(a.data.notes)
	a.prefs.Cards = len(a.data.cards)
	counts := a.queueCounts()
	a.prefs.DueNow = counts.Cards + counts.Reading
	a.prefs.TrayCards, a.prefs.TrayReading = counts.Cards, counts.Reading
	a.syncPresets()
	a.refreshSettingsData()
}

// syncPresets copies the library's own rhythm into the pages, and today's queue
// at it into the budget page. It is a load path: the pages read what it left,
// and no frame asks the queue or the store.
func (a *App) syncPresets() {
	if a.lib == nil {
		// No library is open: the pages show the rhythm a new one starts with,
		// and today's queue has nothing to say.
		p := store.DefaultPresets()
		a.prefs.RetentionPercent = retentionPercent(p)
		a.prefs.MaxIntervalDays = p.Review.MaxIntervalDays
		a.prefs.IntervalPreview = p.Review.Preview
		a.prefs.ReturnAfterExcerpt = p.Reading.ReturnAfterExcerpt
		a.prefs.BudgetCards, a.prefs.BudgetMinutes = p.Budget.Cards, p.Budget.Minutes
		a.prefs.Budget = screensettings.BudgetPreview{}
		return
	}
	p := a.lib.Presets()
	a.prefs.RetentionPercent = retentionPercent(p)
	a.prefs.MaxIntervalDays = p.Review.MaxIntervalDays
	a.prefs.IntervalPreview = p.Review.Preview
	a.prefs.ReturnAfterExcerpt = p.Reading.ReturnAfterExcerpt
	a.prefs.BudgetCards, a.prefs.BudgetMinutes = p.Budget.Cards, p.Budget.Minutes
	a.prefs.Budget = a.budgetPreview(p)
}

// retentionPercent is the target the library schedules at: the override the user
// set, or the scheduling plugin's own while there is none. The page shows one
// number, so a presets file the user never touched and one that holds 90% read
// the same.
func retentionPercent(p store.Presets) int {
	if p.Review.Retention != nil {
		return int(math.Round(*p.Review.Retention * 100))
	}
	return int(math.Round(schedule.DefaultParams().DesiredRetention * 100))
}

// budgetPreview is today's queue at the presets' pace: the same function the
// Today page asks, with the same options, so the page cannot promise a day the
// queue would not build.
func (a *App) budgetPreview(p store.Presets) screensettings.BudgetPreview {
	today, err := queue.Entries(a.lib, time.Now(), todayOptions(p))
	if err != nil {
		slog.Warn("remit: today's queue", "err", err)
		return screensettings.BudgetPreview{}
	}
	preview := screensettings.BudgetPreview{
		Read: today.ReadMinutes, Review: today.ReviewMinutes,
		Total: today.TotalMinutes, Budget: today.Budget, Lines: len(today.Entries),
	}
	for _, entry := range today.Entries {
		if entry.Kind == queue.EntryReview {
			preview.Cards = entry.Cards
		}
	}
	return preview
}

// todayOptions is the day's queue as the presets set it: the minutes budget, the
// excerpt slots, the new-card and new-reading pace, and the cap on the cards a
// day holds. The Today page and the budget page's preview both ask through it.
func todayOptions(p store.Presets) queue.TodayOptions {
	limits := queue.Limits(p)
	return queue.TodayOptions{
		Budget:       time.Duration(p.Budget.Minutes) * time.Minute,
		ExcerptSlots: p.Budget.ExcerptSlots,
		Reading:      limits,
		Cards:        queue.Options{Limit: p.Budget.Cards, NewPerDay: limits.NewPerDay},
	}
}

// tokens is the design vocabulary the frame draws with: the appearance folded
// from the user's choice over the desktop's, and the document paper's measure
// from the settings. It is the one place the look is resolved, so the views
// keep reading design.NewTokens' answer and never a second appearance.
func (a *App) tokens(c *ui.Context) design.Tokens {
	base := c.Theme()
	dark := base.Dark
	switch a.settings.AppearanceName() {
	case settings.AppearancePaper:
		dark = false
	case settings.AppearanceInk:
		dark = true
	}
	if dark != base.Dark {
		base = &ui.Theme{Dark: dark, FontSize: base.FontSize}
	}
	tk := design.NewTokens(base)
	tk.PaperWidth = float32(a.settings.PaperMeasure())
	return tk
}

// setAppearance writes the look the user chose. The running window follows it on
// the next frame, through the tokens the frame draws with; the desktop's own
// chrome follows through OnSettings, which cmd answers with MyGo's theme source.
// The look is the app's preference: a library travels between machines, and how
// it looks does not.
func (a *App) setAppearance(c *ui.Context, appearance string) {
	a.writePrefs(c, func(s *settings.Settings) { s.Appearance = appearance })
}

// setPaperWidth writes the document paper's measure.
func (a *App) setPaperWidth(c *ui.Context, width int) {
	a.writePrefs(c, func(s *settings.Settings) { s.PaperWidth = width })
}

// openDisplaySettings opens the desktop's own text-size page: the window draws
// at the desktop's scale, so the setting that changes it is the desktop's.
func (a *App) openDisplaySettings(c *ui.Context) {
	if a.openDisplay == nil {
		c.Toast(m(c).SettingsUnavailable())
		return
	}
	if err := a.openDisplay(); err != nil {
		a.report(c, "open system settings", err)
	}
}

// openLicense opens the license the app is under, in the browser.
func (a *App) openLicense(c *ui.Context) {
	if err := mygo.Shell.OpenExternal(about.LicenseURL); err != nil {
		a.report(c, "open the license", err)
	}
}

// writePresets applies one change to the library's own rhythm and writes it.
// The presets follow the material, so they live in the library's config; a
// refusal leaves the file as it was and says so where the change was made.
func (a *App) writePresets(c *ui.Context, fn func(*store.Presets)) {
	if a.lib == nil {
		c.Toast(m(c).SettingsUpdateNeedsLibrary())
		return
	}
	presets := a.lib.Presets()
	fn(&presets)
	if err := a.lib.SetPresets(presets); err != nil {
		slog.Warn("remit: write presets", "err", err)
		a.report(c, "save the library's presets", err)
		a.syncPrefs() // the file did not take it: show what it holds
		return
	}
	a.reload()
	a.syncPrefs()
}

// setRetention overrides the scheduling plugin's target for this library.
func (a *App) setRetention(c *ui.Context, percent int) {
	a.writePresets(c, func(p *store.Presets) {
		target := float64(percent) / 100
		p.Review.Retention = &target
	})
}

// setMaxInterval writes the most days a card may wait.
func (a *App) setMaxInterval(c *ui.Context, days int) {
	a.writePresets(c, func(p *store.Presets) { p.Review.MaxIntervalDays = days })
}

// setIntervalPreview says whether a review shows what each answer would
// schedule.
func (a *App) setIntervalPreview(c *ui.Context, on bool) {
	a.writePresets(c, func(p *store.Presets) { p.Review.Preview = on })
}

// setReturnAfterExcerpt says whether an excerpt returns the reader to the note.
func (a *App) setReturnAfterExcerpt(c *ui.Context, on bool) {
	a.writePresets(c, func(p *store.Presets) { p.Reading.ReturnAfterExcerpt = on })
}

// setBudget writes what a day holds.
func (a *App) setBudget(c *ui.Context, cards, minutes int) {
	a.writePresets(c, func(p *store.Presets) {
		p.Budget.Cards, p.Budget.Minutes = cards, minutes
	})
}

// reindex throws the derived index away and derives it from the plaintext
// again. The library's own files are not touched, which is what the page
// promises.
func (a *App) reindex(c *ui.Context) {
	if a.lib == nil {
		c.Toast(m(c).SettingsUpdateNeedsLibrary())
		return
	}
	if err := a.lib.Rebuild(); err != nil {
		a.report(c, "rebuild the index", err)
		return
	}
	a.reload()
	a.refreshSettingsData()
	c.Toast(m(c).SettingsReindexed())
}

// exportLibrary writes the plaintext sources into the .remitlib archive the user
// chooses, the same archive the offline -export writes.
func (a *App) exportLibrary(c *ui.Context) {
	if a.lib == nil {
		c.Toast(m(c).SettingsUpdateNeedsLibrary())
		return
	}
	path, err := a.pickArchive()
	if err != nil {
		a.report(c, "choose an archive", err)
		return
	}
	if path == "" {
		return // the dialog was canceled: nothing to do, and nothing to report
	}
	if err := writeArchive(a.dir, path); err != nil {
		a.report(c, "export the library", err)
		return
	}
	c.Toast(m(c).SettingsExported(path))
}

// writeArchive packages the library at dir into path, through a temporary file
// in the same directory so a failed write never leaves half an archive behind.
func writeArchive(dir, path string) error {
	tmp := path + ".part"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := transfer.Write(dir, file); err != nil {
		file.Close()
		os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// openPluginPage opens a page a plugin contributed, by its manifest name. It is
// the plugins page's one navigation: a page is chosen, not copied.
func (a *App) openPluginPage(c *ui.Context, plugin, id string) {
	for _, page := range a.visiblePages() {
		if page.Plugin == plugin && (id == "" || page.Page.ID == id) {
			a.openPage(c, page)
			return
		}
	}
}

// pickLibraryArchive is the save dialog an export of the whole library writes
// through: one file, named for the app, filtered to the archive's extension.
func pickLibraryArchive() (string, error) {
	return mygo.Dialog.Save(mygo.SaveDialogOptions{
		Title:       locales.SettingsExport(),
		DefaultPath: Name + ".remitlib",
		ButtonLabel: locales.SettingsExport(),
		Filters: []mygo.FileFilter{
			{Name: Name, Extensions: []string{"remitlib"}},
		},
	})
}

// openDisplaySettings opens the desktop's page where its text size is set. macOS
// keeps it under Accessibility's display settings, and the app can name that
// pane; the platforms without a page of their own have the default left unset,
// so the settings page explains the size without offering a door that would not
// open.
func openDisplaySettings() error {
	return mygo.Shell.OpenExternal("x-apple.systempreferences:com.apple.Accessibility-Settings.extension?Display")
}

// refreshSettingsData retakes what the plugins and types sections list: what
// discovery finds, the roles this library plays and its types. It runs on the
// UI thread when the page opens and after every write, so no frame reads the
// plugins directory or the store for them.
func (a *App) refreshSettingsData() {
	a.prefs.HasLibrary = a.lib != nil
	a.pluginList = a.discoveredPlugins()
	a.roleList = map[string]string{}
	a.typeList = nil
	if a.lib == nil {
		return
	}
	a.roleList = a.lib.Roles()
	enabled := make(map[string]bool)
	for _, name := range a.lib.EnabledExtensions() {
		enabled[name] = true
	}
	problems := a.pluginProblems()
	for i := range a.pluginList {
		name := a.pluginList[i].Name
		a.pluginList[i].Enabled = a.pluginList[i].Builtin || enabled[name]
		a.pluginList[i].Problems = problems[name]
		a.pluginList[i].Failures = a.pluginFailures(name)
	}
	a.prefs.PluginSettings = a.pluginSettings()
	a.prefs.Pages = nil
	for _, page := range a.visiblePages() {
		a.prefs.Pages = append(a.prefs.Pages, screensettings.PluginPage{
			Plugin: page.Plugin, ID: page.Page.ID, Title: string(page.Page.Title),
		})
	}
	used := make(map[core.ID]int, 8)
	for _, card := range a.data.cards {
		used[card.Type]++
	}
	for _, typ := range a.lib.Types() {
		a.typeList = append(a.typeList, screensettings.Type{Type: typ, Cards: used[typ.ID]})
	}
}

// discoveredPlugins is what the plugin directory holds, built-ins first and
// then by name. A discovery that fails is a log line and what it did find: a
// broken directory must not keep the settings page from opening.
func (a *App) discoveredPlugins() []screensettings.Plugin {
	if a.discover == nil {
		return nil
	}
	found, err := a.discover()
	if err != nil {
		slog.Warn("remit: discover plugins", "err", err)
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].Builtin != found[j].Builtin {
			return found[i].Builtin
		}
		return found[i].Name < found[j].Name
	})
	return found
}

// settingsDeps is what the settings pages ask of the app: one function per
// thing the user can do there, each run on the UI thread. The reads answer the
// snapshots syncPrefs and refreshSettingsData left, so a frame never reaches the
// store.
func (a *App) settingsDeps() screensettings.Deps {
	return screensettings.Deps{
		SetAppearance:       a.setAppearance,
		SetPaperWidth:       a.setPaperWidth,
		OpenDisplaySettings: a.openDisplaySettings,

		SetRetention:          a.setRetention,
		SetMaxInterval:        a.setMaxInterval,
		SetIntervalPreview:    a.setIntervalPreview,
		SetReturnAfterExcerpt: a.setReturnAfterExcerpt,
		SetBudget:             a.setBudget,
		SetNotifications:      a.setNotifications,
		ApplyHotkey:           a.applyHotkey,

		SetAgent:    a.setAgent,
		RotateToken: a.rotateToken,

		SetUpdateCheck: a.setUpdateCheck,
		CheckNow:       a.checkNow,
		Reindex:        a.reindex,
		ExportLibrary:  a.exportLibrary,
		OpenLicense:    a.openLicense,

		Plugins:    func() []screensettings.Plugin { return a.pluginList },
		SetEnabled: a.setPluginEnabled,
		Roles:      a.rolesSnapshot,
		SetRole:    a.setRole,
		OpenPage:   a.openPluginPage,

		Types:      func() []screensettings.Type { return a.typeList },
		SaveType:   a.saveType,
		DeleteType: a.deleteType,

		SetPluginSetting: a.setPluginSetting,
		PluginMessage:    a.pluginText,
		Reenable:         a.reenablePlugin,
	}
}

// rolesSnapshot is the roles the page draws, as a copy: the page reads it every
// frame and never writes it.
func (a *App) rolesSnapshot() map[string]string {
	roles := make(map[string]string, len(a.roleList))
	for role, name := range a.roleList {
		roles[role] = name
	}
	return roles
}

// setPluginEnabled approves a plugin for this library or withdraws it, which is
// the grant its declared capabilities hang on. The write is immediate; the
// plugin starts on the next library open, as enabling always has.
func (a *App) setPluginEnabled(c *ui.Context, name string, on bool) {
	a.prefs.PluginErr = ""
	if a.lib == nil {
		a.prefs.PluginErr = m(c).SettingsPluginsNeedsLibrary()
		return
	}
	names := a.lib.EnabledExtensions()
	next := make([]string, 0, len(names)+1)
	for _, held := range names {
		if held != name {
			next = append(next, held)
		}
	}
	if on {
		next = append(next, name)
	}
	if err := a.lib.SetEnabledExtensions(next); err != nil {
		slog.Warn("remit: enable plugin", "name", name, "on", on, "err", err)
		a.prefs.PluginErr = m(c).PluginEnableFailed(err.Error())
	}
	// Approving or withdrawing a plugin takes its pages, commands and settings
	// out of every receiver now: they are declarations, so nothing running has
	// to stop for them to go (ui.md §8).
	a.loadContributions()
	a.refreshSettingsData()
}

// setRole chooses the plugin that plays a role in this library. Like enabling,
// it takes effect on the next library open; the page shows what the file holds.
func (a *App) setRole(c *ui.Context, role, name string) {
	a.prefs.PluginErr = ""
	if a.lib == nil {
		a.prefs.PluginErr = m(c).SettingsPluginsNeedsLibrary()
		return
	}
	if err := a.lib.SetRole(role, name); err != nil {
		slog.Warn("remit: set plugin role", "role", role, "name", name, "err", err)
		a.prefs.PluginErr = m(c).PluginRoleFailed(err.Error())
	}
	a.refreshSettingsData()
}

// saveType writes a type through the store, which mints a new ID and carries a
// field rename through every card. The library is reloaded so the cards on
// screen render through the type as it now is.
func (a *App) saveType(c *ui.Context, typ core.Type) core.ID {
	a.prefs.TypeErr = ""
	if a.lib == nil {
		a.prefs.TypeErr = m(c).SettingsTypesNeedsLibrary()
		return ""
	}
	id, err := a.lib.SaveType(typ)
	if err != nil {
		slog.Warn("remit: save type", "type", typ.ID, "err", err)
		a.prefs.TypeErr = a.typeWriteError(c, err)
		a.refreshSettingsData()
		return ""
	}
	a.reload()
	a.refreshSettingsData()
	c.Toast(m(c).TypeSaved())
	return id
}

// deleteType removes a type no card references, and says why when one does.
func (a *App) deleteType(c *ui.Context, id core.ID) {
	a.prefs.TypeErr = ""
	if a.lib == nil {
		a.prefs.TypeErr = m(c).SettingsTypesNeedsLibrary()
		return
	}
	if err := a.lib.DeleteType(id); err != nil {
		slog.Warn("remit: delete type", "type", id, "err", err)
		a.prefs.TypeErr = a.typeWriteError(c, err)
		a.refreshSettingsData()
		return
	}
	a.reload()
	a.refreshSettingsData()
	c.Toast(m(c).TypeDeleted())
}

// typeWriteError is the page's sentence for a refusal: which field holds the
// values, how many cards keep the type, or what the store would not take.
func (a *App) typeWriteError(c *ui.Context, err error) string {
	msgs := m(c)
	var held *store.FieldInUseError
	if errors.As(err, &held) {
		return msgs.TypeFieldHeld(held.Field, held.Cards)
	}
	var used *store.TypeInUseError
	if errors.As(err, &used) {
		return msgs.TypeInUse(used.Cards)
	}
	switch {
	case errors.Is(err, store.ErrBuiltinType):
		return msgs.TypeBuiltinDelete()
	case errors.Is(err, store.ErrNoType):
		return msgs.TypeUnknown()
	}
	return msgs.TypeBad(err.Error())
}

// setUpdateCheck remembers whether the app checks for updates at launch.
func (a *App) setUpdateCheck(c *ui.Context, on bool) {
	a.writePrefs(c, func(s *settings.Settings) { s.UpdateCheck = on })
}

// setNotifications remembers whether the daily reminder is on. It is the same
// setting the tray's pause item flips.
func (a *App) setNotifications(c *ui.Context, on bool) {
	a.writePrefs(c, func(s *settings.Settings) { s.Notifications = on })
}

// NoteHotkeyFailure records why the accelerator could not be registered, so
// that the next visit to the settings page says it. A startup that cannot bind
// the shortcut must never be a silent one.
func (a *App) NoteHotkeyFailure(err error) {
	if err == nil {
		a.prefs.HotkeyErr = ""
		return
	}
	a.prefs.HotkeyErr = err.Error()
}

// applyHotkey binds the accelerator the field holds. A combination that is
// invalid or already taken leaves the old binding in place: the seam registers
// before it unregisters, and nothing is written unless that worked.
func (a *App) applyHotkey(c *ui.Context, draft string) {
	acc := strings.TrimSpace(draft)
	if acc == "" {
		acc = settings.DefaultHotkey
	}
	if a.bindHotkey == nil {
		a.prefs.HotkeyErr = m(c).SettingsUnavailable()
		return
	}
	if err := a.bindHotkey(acc); err != nil {
		a.prefs.HotkeyErr = err.Error()
		slog.Warn("remit: global shortcut not applied", "accelerator", acc, "err", err)
		return
	}
	a.prefs.HotkeyErr = ""
	if err := a.UpdateSettings(func(s *settings.Settings) { s.Hotkey = acc }); err != nil {
		a.report(c, "save settings", err)
		return
	}
	c.Toast(m(c).ShortcutApplied(acc))
}

// checkNow asks whether a newer version exists and says what came back. It
// asks only when the user turned the check on, and reports a build that cannot
// update as such rather than as being up to date.
func (a *App) checkNow(c *ui.Context) {
	a.prefs.UpdateMsg = ""
	if !a.settings.UpdateCheck {
		a.prefs.UpdateMsg = m(c).UpdateOff()
		return
	}
	if a.checkUpdate == nil {
		a.prefs.UpdateMsg = m(c).SettingsUnavailable()
		return
	}
	result, err := a.checkUpdate(context.Background(), a.settings)
	switch {
	case errors.Is(err, errCannotUpdate):
		a.prefs.UpdateMsg = m(c).UpdateCannot()
	case err != nil:
		a.prefs.UpdateMsg = m(c).UpdateFailed(err.Error())
		slog.Warn("remit: update check", "err", err)
	case result.Version == "":
		a.prefs.UpdateMsg = m(c).UpdateUpToDate()
	default:
		a.prefs.UpdateMsg = m(c).UpdateAvailable(result.Version)
		slog.Info("remit: newer version available", "version", result.Version, "running", Version)
	}
}

// rotateToken withdraws the agent API's token and reports the one now active,
// which is a new one on a restarted listener.
func (a *App) rotateToken(c *ui.Context) {
	if a.agentAPI == nil {
		c.Toast(m(c).SettingsUnavailable())
		return
	}
	if err := a.agentAPI.RotateToken(); err != nil {
		slog.Warn("remit: agent token", "err", err)
		c.Toast(m(c).TokenRotateFailed(err.Error()))
		return
	}
	a.syncPrefs()
	a.prefs.Token = readToken(a.prefs.Agent.TokenPath)
	c.Toast(m(c).TokenRotated())
}

// readToken is the token the agent API just wrote, read back so the page can
// show it once. A file that cannot be read leaves the page without one: the
// rotation itself already succeeded, and a secret is not worth a toast.
func readToken(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("remit: read the agent token", "err", err)
		return ""
	}
	return strings.TrimSpace(string(data))
}

// setAgent turns the agent API on or off now, and remembers the choice for the
// next start only when the listener did as it was told.
func (a *App) setAgent(c *ui.Context, on bool) {
	if a.agentAPI == nil {
		a.syncPrefs()
		c.Toast(m(c).SettingsUnavailable())
		return
	}
	if err := a.agentAPI.SetEnabled(on); err != nil {
		slog.Warn("remit: agent api", "on", on, "err", err)
		a.syncPrefs()
		c.Toast(m(c).AgentFailed(err.Error()))
		return
	}
	// A choice that could not be remembered is undone: the port follows the
	// settings rather than serving what they deny.
	if !a.writePrefs(c, func(s *settings.Settings) { s.AgentServer = on }) && on {
		if err := a.agentAPI.SetEnabled(false); err != nil {
			slog.Warn("remit: agent api", "err", err)
		}
	}
	a.syncPrefs()
}

// writePrefs writes the page's change and reports a failure where the change
// was made; the page ends up showing what the file holds either way.
func (a *App) writePrefs(c *ui.Context, fn func(*settings.Settings)) bool {
	if err := a.UpdateSettings(fn); err != nil {
		a.report(c, "save settings", err)
		a.syncPrefs() // the file did not take it: show what it holds
		return false
	}
	return true
}
