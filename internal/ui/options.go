package ui

import (
	"context"

	"github.com/transmit-bug/remit/internal/core/importanki"
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
