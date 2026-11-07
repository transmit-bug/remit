package ui

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/settings"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
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

// openSettings shows the settings page, remembering the face to come back to.
func (a *App) openSettings() {
	if a.mode != modeSettings {
		a.prefsBack = a.mode
	}
	a.mode = modeSettings
	a.syncPrefs()
}

// closeSettings returns to the face the page was opened from.
func (a *App) closeSettings() {
	back := a.prefsBack
	if back == modeSettings {
		back = modeNotes
	}
	a.setMode(back)
}

// syncPrefs copies the settings, and the agent API's own report, into what the
// page shows. Every write ends here, so a failed one leaves the page showing
// what the file holds rather than what the user clicked.
func (a *App) syncPrefs() {
	a.prefs.Notifications = a.settings.Notifications
	a.prefs.AgentOn = a.settings.AgentServer
	a.prefs.UpdateCheck = a.settings.UpdateCheck
	a.prefs.Hotkey = a.settings.HotkeyAccelerator()
	a.prefs.Agent = screensettings.AgentStatus{}
	if a.agentAPI != nil {
		a.prefs.Agent = a.agentAPI.Status()
	}
	a.refreshSettingsData()
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

// settingsDeps is what the settings page asks of the app: one function per
// thing the user can do there, each run on the UI thread. The reads answer the
// snapshots refreshSettingsData left, so a frame never reaches the store.
func (a *App) settingsDeps() screensettings.Deps {
	return screensettings.Deps{
		SetAgent:         a.setAgent,
		RotateToken:      a.rotateToken,
		CheckNow:         a.checkNow,
		ApplyHotkey:      a.applyHotkey,
		SetUpdateCheck:   a.setUpdateCheck,
		SetNotifications: a.setNotifications,

		Plugins:    func() []screensettings.Plugin { return a.pluginList },
		SetEnabled: a.setPluginEnabled,
		Roles:      a.rolesSnapshot,
		SetRole:    a.setRole,
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
	c.Toast(m(c).TokenRotated())
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
