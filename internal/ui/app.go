// Package ui draws Remit's window. It owns presentation and input handling and
// keeps domain rules in internal/core.
package ui

import (
	"context"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/importmd"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/screens/browse"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/internal/ui/screens/read"
	"github.com/transmit-bug/remit/internal/ui/screens/reading"
	"github.com/transmit-bug/remit/internal/ui/screens/relink"
	"github.com/transmit-bug/remit/internal/ui/screens/review"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
	screenstats "github.com/transmit-bug/remit/internal/ui/screens/stats"
	screenToday "github.com/transmit-bug/remit/internal/ui/screens/today"
	"github.com/transmit-bug/remit/internal/ui/shell"
	"github.com/transmit-bug/remit/internal/update"
)

// Name and Version are the app's identity: the About page shows them, the
// window title and the menus use Name, and mygo.json carries Version for
// packaging. TestVersionMatchesConfig keeps the two in step.
const (
	Name    = "Remit"
	Version = "0.1.0"
)

// mode is the screen a plugin's when expression reads: the legacy five-screen
// vocabulary the contract has always had (notes, review, browse, stats,
// settings, or empty where no legacy screen answers). It is derived, not set: the
// window's own state is the area, its page and the session over them, and
// syncMode is the one place the two are reconciled.
type mode uint8

const (
	// modeNone is where no legacy screen answers: Today, the reading queue and
	// a page a plugin contributed. A when expression reads it as empty.
	modeNone mode = iota
	modeNotes
	modeReview
	modeReading
	modeBrowse
	modeStats
	modeSettings
	// modePlugin is a page a plugin contributed: not one of the host's five
	// screens, which is why a when expression reads mode empty there and
	// page.id names it instead.
	modePlugin
)

// animState counts the moments the window's own motion replays by: a screen
// changing, a panel opening. The count is the run of a preset, so when it moves
// the preset plays again from its first frame.
type animState struct {
	mode    uint64 // the screen the window shows
	palette uint64 // the command palette
	relink  uint64 // the re-link picker
	keys    uint64 // the keyboard help
	wizard  uint64 // the import wizard
	export  uint64 // the export dialog
}

// App is the state the window shows. The view rebuilds the interface from it
// on every frame; handlers change it.
type App struct {
	lib     *store.Library // the open library, nil until open succeeds
	dir     string         // where the library lives, once resolved
	openErr error          // why the library did not open, or nil
	start   bool           // no library yet: the start view asks where to put one
	data    libraryData    // the library as the view reads it
	due     []core.ID      // today's queue, as the last reload derived it

	mode             mode
	area             areaID          // which area the rail shows as chosen
	sub              subID           // which of its pages; subNone when it has none
	session          sessionKind     // the session over the stage, when one is open; none until the session screens land
	page             contrib.PageRef // the contributed page on screen, when area is areaPlugin
	review           review.State
	reviewScope      queue.Scope   // what the review session draws from; within the session only, never stored
	reading          reading.State // the reading session: the queue's order and where the reader is
	readingPositions map[core.ID]core.ID
	readingCount     int
	notes            notes.State       // the notebook: the note open, its draft and its panes
	notesRows        []notes.Note      // the notebook's list, as the snapshot derived it
	today            screenToday.State // today: the day's queue and the list's place
	todayDay         queue.Today       // the day's queue, as loadToday derived it
	todayTomorrow    int               // what comes due on the day after
	// read is the read area's own pages — the queue and the archive — and readPages
	// is what the last load derived for them: the rows and the rhythm under them.
	read      read.State
	readPages readPages
	// excerpted are the blocks something already quotes, so that a paragraph can
	// say it was excerpted without a frame reading the library's links.
	excerpted map[core.ID]bool
	browse           browse.State
	browseRows       []browse.Row // the cards browse lists, as loadBrowse derived them
	stats            screenstats.State
	palette          paletteState
	keys             keysState
	wizard           importWizard
	export           exportState
	relink           relink.State
	cardEdit         cardEditState
	cardDetail       cardDetailState
	prefs            screensettings.State
	prefsBack        areaID                                  // the area the settings page goes back to
	prefsBackSub     subID                                   // and the page of it
	cards            *cardPlugin                             // what a paragraph asks and how a card reads
	scheduler        *schedulerPlugin                        // what schedules a review, and what a session is worth
	cardsChoice      *ext.JSRunner                           // the runner Options.Cards named, nil for the app's own
	schedulerChoice  *ext.JSRunner                           // the runner Options.Scheduler named
	withoutCards     bool                                    // no card plugin at all
	withoutScheduler bool                                    // schedule with the app's own defaults
	discover         func() ([]screensettings.Plugin, error) // what the plugin directory holds
	openPlugin       func(string) (*ext.JSRunner, error)     // the runner a role names
	// manifests and contribution are the plugin service as the window reads it:
	// what each plugin declares without being started, and the runner that
	// answers a contributed command. cmd owns both; nil is a build whose plugins
	// contribute no UI and whose callbacks are unavailable.
	manifests    func() ([]ext.Extension, error)
	contribution func(name string) (CommandRunner, error)

	// ui is the plugins' declared UI, parsed once when the library opens, and
	// pending is what their callbacks answered since the last frame: what a
	// frame reads instead of ever calling a plugin.
	ui      pluginUI
	pending pendingWork
	// stale is set by the library's event handler, from whichever goroutine
	// wrote: the next frame drops what each page's cache key was, so a page
	// reads the library it is now.
	stale atomic.Bool
	// runBackground runs one job off the UI thread. nil is the app's own: a
	// goroutine whose answer the next frame applies. A headless test runs it
	// inline, where there is one thread.
	runBackground func(*ui.Context, func() func(*ui.Context))
	// pluginList, roleList and typeList are the settings page's snapshots,
	// retaken when the page opens and after every write: no frame reads the
	// plugins directory or the store for them.
	pluginList     []screensettings.Plugin
	roleList       map[string]string
	typeList       []screensettings.Type
	summaryWord    summaryAnswer    // what the scheduling plugin says about a session
	summaryWordFor string           // the numbers that answer was given
	resolver       *cards.Resolver  // what a paragraph asks and how a card reads, asked once per text
	backlinks      []notes.Backlink // what points at the note open now
	aboutWin       *mygo.Window     // the open About window, or nil

	// preview is what the four answers would schedule for the card on screen,
	// loaded with the rest of what a mode reads so that no frame reads the
	// store.
	preview    []schedule.Outlook
	hasPreview bool
	summary    stats.Summary
	hasSummary bool

	// anim counts how often each moment played: a screen change, a panel
	// opening. A counter that moves replays its preset.
	anim animState
	// shell is the frame's own state: what the user folded, which survives the
	// page behind it changing.
	shell shell.State

	// settingsDir is where settings.json lives, empty when the app keeps no
	// settings; defaultLibrary is <Documents>/Remit, resolved once.
	settingsDir    string
	settings       settings.Settings
	defaultLibrary string
	pickLibrary    func() (string, error)

	// The seams the settings page and the palette's import act through: cmd
	// registers them and a test replaces them. A nil one leaves the page
	// saying that this build has no such service.
	bindHotkey  func(accelerator string) error
	agentAPI    AgentControls
	checkUpdate func(context.Context, settings.Settings) (update.Result, error)
	pickAnki    func() (string, error)
	importAnki  func(path string) (importanki.Report, error)
	// pickArchive is the .remitlib picker the import wizard's archive source
	// opens; a test replaces it.
	pickArchive func() (string, error)
	// The Markdown import's seams, shaped like the Anki ones: a test replaces
	// the picker and the importer; a nil importer leaves the actions quiet.
	pickMd     func() ([]string, error)
	importMd   func(path string) (importmd.Report, error)
	pickFolder func(title string) (string, error)
	pickImage  func() (string, error)
	// pickPlugin is the file picker a plugin importer's entry opens through,
	// shaped like the app's own: the label names the entry, the extensions
	// filter the files. nil falls back to the window's dialog.
	pickPlugin func(label string, extensions []string) ([]string, error)

	// counts is told the queues' lengths whenever they are rebuilt, so the tray
	// can show them without asking the window.
	counts func(Counts)

	// opened is told about the library once it opens, so a service that
	// serves it (the agent API) can start without a second open.
	opened func(*store.Library)

	// settingsChanged is told about the settings whenever they are written,
	// so the tray's pause item follows the page and the page follows the tray.
	settingsChanged func(settings.Settings)
}

// NewApp returns an app on the library in opts.LibraryDir, or on the one a
// previous run remembered in its settings, opened on today. With no library at
// either place it shows the start view — Create library or Open library… —
// rather than creating one the user did not ask for. A library that cannot be
// opened leaves the window showing that, with the one action that tries again;
// it never panics over a damaged library.
func NewApp(opts ...Options) *App {
	a := &App{
		browse:      browse.State{Selected: -1},
		notes:       notes.New(),
		pickLibrary: pickLibraryDir,
		pickAnki:    pickAnkiPackage,
		pickArchive: pickRemitlib,
		pickMd:      pickMarkdown,
		pickFolder:  pickExportFolder,
		pickImage:   pickNoteImage,
		checkUpdate: screensettings.CheckForUpdate,
	}
	// The importer reads the library open now, which a dropped archive can
	// replace, so it takes it at call time rather than at construction.
	a.importAnki = func(path string) (importanki.Report, error) { return importanki.Import(path, a.lib) }
	a.importMd = func(path string) (importmd.Report, error) { return importmd.Import(path, a.lib) }
	var chosenCards *ext.JSRunner
	var chosenScheduler *ext.JSRunner
	withoutCards, withoutScheduler := false, false
	if len(opts) > 0 {
		o := opts[0]
		a.dir = o.LibraryDir
		a.settingsDir = o.DataDir
		if o.ChooseLibrary != nil {
			a.pickLibrary = o.ChooseLibrary
		}
		if o.ChooseAnki != nil {
			a.pickAnki = o.ChooseAnki
		}
		if o.ChooseArchive != nil {
			a.pickArchive = o.ChooseArchive
		}
		if o.ImportAnki != nil {
			a.importAnki = o.ImportAnki
		}
		if o.ChooseMarkdown != nil {
			a.pickMd = o.ChooseMarkdown
		}
		if o.ImportMarkdown != nil {
			a.importMd = o.ImportMarkdown
		}
		if o.ChooseExportFolder != nil {
			a.pickFolder = o.ChooseExportFolder
		}
		if o.ChooseImage != nil {
			a.pickImage = o.ChooseImage
		}
		if o.CheckUpdate != nil {
			a.checkUpdate = o.CheckUpdate
		}
		a.bindHotkey = o.BindHotkey
		a.agentAPI = o.AgentControls
		chosenCards, withoutCards = o.Cards, o.WithoutCards
		chosenScheduler, withoutScheduler = o.Scheduler, o.WithoutScheduler
		a.discover, a.openPlugin = o.Discover, o.Plugin
		a.manifests, a.contribution = o.Manifests, o.Contribution
	}
	a.cardsChoice, a.schedulerChoice = chosenCards, chosenScheduler
	a.withoutCards, a.withoutScheduler = withoutCards, withoutScheduler
	// With a plugin directory the roles decide what derives and schedules, so
	// the plugins are opened per library; without one the app's own play.
	if a.openPlugin == nil {
		a.cards = openCards(chosenCards, withoutCards)
		a.scheduler = openScheduler(chosenScheduler, withoutScheduler)
	}
	if dir, err := DefaultLibraryDir(); err == nil {
		a.defaultLibrary = dir
	}
	a.loadSettings()
	a.open()
	return a
}
