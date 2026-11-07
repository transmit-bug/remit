package settings

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/transmit-bug/remit/internal/core"
)

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

	// HasLibrary is whether a library is open: plugins and types are per
	// library, so their sections say to open one before there is anything to
	// manage.
	HasLibrary bool
	// PluginErr is why the last plugin or role write was refused, "" when it
	// was taken.
	PluginErr string
	// PluginSettings is the settings the enabled plugins declared, with the
	// values this library holds. The form may change a value in place; a write
	// is still the app's, and it retakes this list.
	PluginSettings []Setting

	// TypeList is the type table's own state: the row the user chose. TypeRow
	// repeats the choice, so the editor can tell the row it shows from one the
	// user just chose.
	TypeList ui.ListState
	TypeRow  int
	// Draft is the type being edited: the chosen type as the editor holds it,
	// or a new one with no ID yet. Nothing typed here is written until Save.
	Draft core.Type
	// TypeErr is why the last type write was refused, "" when it was taken.
	TypeErr string
	// PreviewFront and PreviewBack keep the parsed previews between frames,
	// so the markdown view reparses only what changed. Pointers because the
	// options carry a cache the frame must not copy.
	PreviewFront, PreviewBack *markdown.Options
}
