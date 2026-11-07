// Package ui draws Remit's window. It owns presentation and input handling and
// keeps domain rules in internal/core.
package ui

import (
	"context"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/screens/browse"
	"github.com/transmit-bug/remit/internal/ui/screens/review"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
	screenstats "github.com/transmit-bug/remit/internal/ui/screens/stats"
	"github.com/transmit-bug/remit/internal/update"
)

// Name and Version are the app's identity: the About page shows them, the
// window title and the menus use Name, and mygo.json carries Version for
// packaging. TestVersionMatchesConfig keeps the two in step.
const (
	Name    = "Remit"
	Version = "0.1.0"
)

// mode is which of Remit's faces the window shows: editing notes, reviewing
// the queue pulled from them, browsing every card, the statistics page, or the
// settings page.
type mode uint8

const (
	modeNotes mode = iota
	modeReview
	modeBrowse
	modeStats
	modeSettings
)

// takingOver reports whether the screen takes the window whole: review hides
// the navigation, so that the card and its status bar are all the user sees.
func (a *App) takingOver() bool {
	return a.lib != nil && !a.start && a.openErr == nil && a.mode == modeReview
}

// animState counts the moments the window's own motion replays by: a screen
// changing, a panel opening. The count is the run of a preset, so when it moves
// the preset plays again from its first frame.
type animState struct {
	mode    uint64 // the screen the window shows
	palette uint64 // the command palette
	relink  uint64 // the re-link picker
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

	mode           mode
	review         review.State
	editor         editorState
	browse         browse.State
	browseRows     []browse.Row // the cards browse lists, as loadBrowse derived them
	stats          screenstats.State
	palette        paletteState
	relink         relinkState
	cardEdit       cardEditState
	prefs          screensettings.State
	prefsBack      mode                 // the face the settings page goes back to
	cards          *cardPlugin          // what a paragraph asks and how a card reads
	scheduler      *schedulerPlugin     // what schedules a review, and what a session is worth
	summaryWord    summaryAnswer        // what the scheduling plugin says about a session
	summaryWordFor string               // the numbers that answer was given
	resolver       *cards.Resolver      // what a paragraph asks and how a card reads, asked once per text
	panel          panelMode            // what the user said about the note's panel
	panelTab       panelTab             // outline or backlinks
	notesWidth     float32              // the notes column, as the divider left it
	editorShare    float32              // the document's percentage of the note's room
	view           viewMode             // written in, beside its source, or read whole
	readShare      float32              // the document's percentage when written beside its source
	outline        ui.OutlineState[int] // the note's headings, as the panel lists them
	outlineChosen  int                  // the row the user chose in the outline, -1 for none
	outlineCurrent int                  // the heading the caret is under, -1 for none
	outlineFor     core.ID              // the note the outline was opened for
	outlineShape   int                  // how many headings that note had
	pendingJump    int                  // where a panel row asks the caret to go, -1 for none
	tagFilter      string               // the tag the notes are filtered by, empty for none
	notesQuery     string               // what the sidebar's search looks for
	backlinks      []store.Backlink     // what points at the note open now
	aboutWin       *mygo.Window         // the open About window, or nil

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

	// remaining is told the queue's length whenever it is rebuilt, so the
	// tray can show it without asking the window.
	remaining func(int)

	// opened is told about the library once it opens, so a service that
	// serves it (the agent API) can start without a second open.
	opened func(*store.Library)

	// settingsChanged is told about the settings whenever they are written,
	// so the tray's pause item follows the page and the page follows the tray.
	settingsChanged func(settings.Settings)
}

// NewApp returns an app on the library in opts.LibraryDir, or on the one a
// previous run remembered in its settings, in review mode. With no library at
// either place it shows the start view — Create library or Open library… —
// rather than creating one the user did not ask for. A library that cannot be
// opened leaves the window showing that, with the one action that tries again;
// it never panics over a damaged library.
func NewApp(opts ...Options) *App {
	a := &App{
		browse:         browse.State{Selected: -1},
		notesWidth:     defaultNotesWidth,
		editorShare:    defaultEditorShare,
		readShare:      defaultReadShare,
		outlineChosen:  -1,
		outlineCurrent: -1,
		pendingJump:    -1,
		outlineFor:     "",
		editor:         editorState{},
		relink:         relinkState{selected: -1},
		pickLibrary:    pickLibraryDir,
		pickAnki:       pickAnkiPackage,
		checkUpdate:    screensettings.CheckForUpdate,
	}
	// The importer reads the library open now, which a dropped archive can
	// replace, so it takes it at call time rather than at construction.
	a.importAnki = func(path string) (importanki.Report, error) { return importanki.Import(path, a.lib) }
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
		if o.ImportAnki != nil {
			a.importAnki = o.ImportAnki
		}
		if o.CheckUpdate != nil {
			a.checkUpdate = o.CheckUpdate
		}
		a.bindHotkey = o.BindHotkey
		a.agentAPI = o.AgentControls
		chosenCards, withoutCards = o.Cards, o.WithoutCards
		chosenScheduler, withoutScheduler = o.Scheduler, o.WithoutScheduler
	}
	a.cards = openCards(chosenCards, withoutCards)
	a.scheduler = openScheduler(chosenScheduler, withoutScheduler)
	if dir, err := DefaultLibraryDir(); err == nil {
		a.defaultLibrary = dir
	}
	a.loadSettings()
	a.open()
	return a
}
