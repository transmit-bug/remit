package ui

import (
	"context"

	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/importmd"
	"github.com/transmit-bug/remit/internal/ext"
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

// Plugin is one plugin this build can run, as the settings page lists it. The
// alias keeps cmd from importing a screen package for one struct.
type Plugin = screensettings.Plugin

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
	// ChooseAnki picks the Anki package the palette's import action reads,
	// mygo.Dialog's file picker unless a test replaces it.
	ChooseAnki func() (string, error)
	// ImportAnki imports the package at path into the open library. It is
	// importanki.Import unless a test replaces it.
	ImportAnki func(path string) (importanki.Report, error)
	// ChooseMarkdown picks the Markdown files the import action reads,
	// mygo.Dialog's file picker (multi-select) unless a test replaces it.
	ChooseMarkdown func() ([]string, error)
	// ImportMarkdown imports the Markdown files at path into the open
	// library. It is importmd.Import unless a test replaces it.
	ImportMarkdown func(path string) (importmd.Report, error)
	// ChooseExportFolder picks the folder an export writes into, mygo.Dialog's
	// folder picker unless a test replaces it.
	ChooseExportFolder func(title string) (string, error)
	// Cards replaces the card plugin the app ships. A test hands in one that
	// throws, to prove the review goes on without it.
	Cards *ext.JSRunner
	// WithoutCards opens the library with no card plugin at all: no paragraph
	// makes a new card, and a card reads its paragraph's own words.
	WithoutCards bool
	// Scheduler replaces the scheduling plugin the app ships.
	Scheduler *ext.JSRunner
	// WithoutScheduler schedules with the app's own defaults, and stamps no
	// scheduler on new reviews.
	WithoutScheduler bool
	// Discover lists the plugins this build can run: the built-ins and what
	// the user installed, each with what its manifest declares. cmd owns the
	// extensions directory; nil is a build with none, whose settings page
	// says so and whose roles stay at the built-ins.
	Discover func() ([]Plugin, error)
	// Plugin starts the plugin a manifest name names, for the role that chose
	// it. The app asks once per library open, before the library reads. nil
	// leaves Options.Cards and Options.Scheduler, or the built-ins, playing
	// every role.
	Plugin func(name string) (*ext.JSRunner, error)
	// Manifests lists every plugin this build can run, by manifest, without
	// starting one: what the app reads to index the pages, commands, menus and
	// settings the library's plugins declare. nil is a build where plugins
	// contribute no UI.
	Manifests func() ([]ext.Extension, error)
	// Contribution opens the runner that answers a contributed command or a
	// page's data, by manifest name. cmd passes the plugin service's own; nil
	// leaves a contribution that calls back with an error the page shows and a
	// command toasts.
	Contribution func(name string) (CommandRunner, error)
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
