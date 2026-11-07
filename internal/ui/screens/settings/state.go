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
}
