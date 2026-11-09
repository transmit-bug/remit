package settings

import "github.com/egoist/mygo/ui"

// Page is which of the settings pages the window shows. It is the sub-navigation's
// choice, held here as the page's own copy so a frame draws one page and never
// the others.
type Page string

// The pages, in the order the sub-navigation lists them. The card types are
// not one of them: the recall area's types page owns that editor (S32).
const (
	PageAppearance Page = "appearance"
	PageReview     Page = "review"
	PageBudget     Page = "budget"
	PageKeys       Page = "keys"
	PagePlugins    Page = "plugins"
	PageAgent      Page = "agent"
	PageUpdate     Page = "update"
	PageAbout      Page = "about"
)

// BudgetPreview is today's queue as the presets would build it: what the reading
// and the reviewing are expected to take, and what the day would hold. The app
// derives it from the same function the Today page asks, so the two cannot
// disagree.
type BudgetPreview struct {
	Read, Review, Total, Budget int // minutes
	Lines, Cards                int // lines today's queue holds, of them the cards
}

// State is the settings pages: the values they show, copied from the app's
// settings and the library's presets when a page opens and after every write,
// and what each action answered. Nothing typed here reaches the settings or the
// library until an action is taken.
type State struct {
	// Page is the page on screen; its zero value is the appearance page.
	Page Page

	// Appearance: the look chosen ("system", "paper", "ink") and the document
	// paper's width in DIPs.
	Appearance string
	PaperWidth int

	// The review presets: how likely a card should be recalled when due, the
	// most days it may wait, and whether a review shows what each grade would
	// schedule. ReturnAfterExcerpt is the reading preset beside them.
	RetentionPercent   int
	MaxIntervalDays    int
	IntervalPreview    bool
	ReturnAfterExcerpt bool

	// The budget presets, what today's queue would hold at them, and the two
	// numbers the tray would show.
	BudgetCards   int
	BudgetMinutes int
	Budget        BudgetPreview
	TrayCards     int
	TrayReading   int

	// Reminder is whether the daily reminder is on. Hotkey is the global
	// review shortcut, and HotkeyErr why the last attempt to bind one failed.
	Reminder  bool
	Hotkey    string
	HotkeyErr string

	// HasLibrary is whether a library is open: the presets, the plugins and
	// their pages are per library, so their pages say to open one before there
	// is anything to manage.
	HasLibrary bool
	// PluginErr is why the last plugin or role write was refused, "" when it
	// was taken.
	PluginErr string
	// PluginSettings is the settings the enabled plugins declared, with the
	// values this library holds. The form may change a value in place; a write
	// is still the app's, and it retakes this list.
	PluginSettings []Setting
	// Pages are the pages the enabled plugins contributed, which the plugins
	// page lists with the one action that opens each.
	Pages []PluginPage
	// Details is which plugin's details the user opened, by manifest name. It
	// is the page's own memory across frames, not a setting.
	Details map[string]bool

	// AgentOn is whether the API serves now, Agent where it listens, and Token
	// the bearer token just minted, which is shown once and then forgotten.
	AgentOn bool
	Agent   AgentStatus
	Token   string
	// RouteTable is the route list's own state: its columns and its scroll.
	RouteTable ui.ListState

	// UpdateCheck is whether the app checks for updates at launch, UpdateMsg
	// the last check's answer, and the two versions the update page states.
	UpdateCheck   bool
	UpdateMsg     string
	Version       string
	FormatVersion string

	// The library the about page names, and what it holds.
	AppName    string
	LibraryDir string
	Notes      int
	Cards      int
	DueNow     int
}

// PluginPage is one page a plugin contributed, as the plugins page lists it.
type PluginPage struct {
	Plugin string // the manifest name, which the app opens the page by
	ID     string // the page's id inside its manifest
	Title  string // the page's own title, already in the frame's language
}

// page is the page to draw: the chosen one, or the first when the choice names
// none.
func (s *State) page() Page {
	switch s.Page {
	case PageAppearance, PageReview, PageBudget, PageKeys, PagePlugins,
		PageAgent, PageUpdate, PageAbout:
		return s.Page
	}
	return PageAppearance
}

// detailsOpen reports whether one plugin's details are open, and the page never
// writes a nil map.
func (s *State) detailsOpen(name string) bool { return s.Details[name] }

// toggleDetails opens or closes one plugin's details.
func (s *State) toggleDetails(name string) {
	if s.Details == nil {
		s.Details = map[string]bool{}
	}
	s.Details[name] = !s.Details[name]
}
