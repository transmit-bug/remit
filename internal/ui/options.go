package ui

import (
	"context"

	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/importmd"
	"github.com/transmit-bug/remit/internal/plugin"
	"github.com/transmit-bug/remit/internal/settings"
	screensettings "github.com/transmit-bug/remit/internal/ui/screens/settings"
	"github.com/transmit-bug/remit/internal/update"
)

// AgentControls is the loopback agent API as the settings page drives it. The
// page owns the interface; the alias keeps cmd from importing a screen package
// for one interface. A nil one leaves the page saying there is no such service.
type AgentControls = screensettings.AgentControls

// AgentStatus is what that service reports: where it listens, and the file
// holding the bearer token.
type AgentStatus = screensettings.AgentStatus

// Options configures an App.
type Options struct {
	// LibraryDir is the directory the library lives in. Empty means the one a
	// previous run remembered, and with none remembered the window opens on
	// the start view instead of creating a library behind the user's back.
	// Tests pass t.TempDir() so they never touch the user's Documents.
	LibraryDir string
	// DataDir is the app's user data directory, where settings.json lives
	// (main passes mygo.App.Path(mygo.PathUserData)). Empty means settings are
	// neither read nor written, as in a test that drives the window alone.
	DataDir string
	// ChooseLibrary picks the folder a library lives in. It is mygo.Dialog's
	// folder picker unless a test replaces it: a native dialog cannot be
	// answered headlessly.
	ChooseLibrary func() (string, error)
	// ChooseAnki picks the Anki package the import wizard's package source
	// reads, mygo.Dialog's file picker unless a test replaces it.
	ChooseAnki func() (string, error)
	// ImportAnki imports the package at path into the open library. It is
	// importanki.Import unless a test replaces it.
	ImportAnki func(path string) (importanki.Report, error)
	// ChooseArchive picks the .remitlib the import wizard's archive source
	// reads, mygo.Dialog's file picker unless a test replaces it.
	ChooseArchive func() (string, error)
	// ChooseMarkdown picks the Markdown files the import action reads,
	// mygo.Dialog's file picker (multi-select) unless a test replaces it.
	ChooseMarkdown func() ([]string, error)
	// ImportMarkdown imports the Markdown files at path into the open
	// library, with the options the wizard's preview was set to. It is
	// importmd.Import unless a test replaces it.
	ImportMarkdown func(path string, opts importmd.Options) (importmd.Report, error)
	// ChooseExportFolder picks the folder an export writes into, mygo.Dialog's
	// folder picker unless a test replaces it.
	ChooseExportFolder func(title string) (string, error)
	// ChooseExportFile picks the archive an export of the whole library writes,
	// mygo.Dialog's save dialog unless a test replaces it.
	ChooseExportFile func() (string, error)
	// ShowInFolder reveals an exported file in the platform's file manager,
	// mygo.Shell's own unless a test replaces it: opening Finder is the one thing
	// a headless test cannot do.
	ShowInFolder func(dir string)
	// OpenSystemSettings opens the desktop's text-size page, which is where the
	// window's text size is changed. nil keeps the platform's own, a build
	// without one hides the door rather than drawing a dead button.
	OpenSystemSettings func() error
	// ChooseImage picks one picture for a note, mygo.Dialog's file picker
	// unless a test replaces it.
	ChooseImage func() (string, error)
	// Plugins is the plugin supervisor: what starts a plugin, which runner plays
	// each role, and what the settings page lists. cmd passes its own, over the
	// user's plugin directory; nil is a window with the plugins this build ships
	// and no directory. The window never constructs a plugin itself
	// (docs/specs/plugin-system.md, P1).
	Plugins *plugin.Supervisor
	// WithoutCards opens the library with no card plugin at all: no paragraph
	// makes a new card, and a card reads its paragraph's own words. It is what a
	// test uses to force the no-plugin fallback.
	WithoutCards bool
	// WithoutScheduler schedules with the app's own defaults, and stamps no
	// scheduler on new reviews.
	WithoutScheduler bool
	// BindHotkey binds the global shortcut, replacing the one bound now, and
	// reports a combination that is invalid or already taken. cmd registers it
	// on MyGo's GlobalShortcut; a test replaces it.
	BindHotkey func(accelerator string) error
	// AgentControls is the loopback agent API as the settings page drives it.
	// cmd passes its own; nil leaves the page saying there is no such service.
	AgentControls AgentControls
	// CheckUpdate looks for a newer version. It is the settings page's default
	// check unless a test replaces it.
	CheckUpdate func(context.Context, settings.Settings) (update.Result, error)
}
