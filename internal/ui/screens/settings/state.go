package settings

// State is the settings page: the values it shows, copied from the app's
// settings when it opens and after every write, and what each action answered.
// Nothing typed here reaches the settings until an action is taken.
type State struct {
	Notifications bool
	AgentOn       bool
	UpdateCheck   bool
	Hotkey        string

	HotkeyErr string // why the shortcut was not applied, "" when it was
	UpdateMsg string // the last update check's outcome
	Agent     AgentStatus

	// HasLibrary is whether a library is open: plugins are per library, so their
	// section says to open one before there is anything to manage.
	HasLibrary bool
	// PluginErr is why the last plugin or role write was refused, "" when it
	// was taken.
	PluginErr string
	// PluginSettings is the settings the enabled plugins declared, with the
	// values this library holds. The form may change a value in place; a write
	// is still the app's, and it retakes this list.
	PluginSettings []Setting
}
