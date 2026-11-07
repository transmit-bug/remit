// Command remit is the Remit desktop application.
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/logging"
	"github.com/transmit-bug/remit/internal/settings"
	appui "github.com/transmit-bug/remit/internal/ui"
	"github.com/transmit-bug/remit/internal/update"
)

func main() {
	library := flag.String("library", "", "the library directory (default: Remit in Documents)")
	export := flag.String("export", "", "write the library to a .remitlib file and exit")
	importPath := flag.String("import", "", "unpack a .remitlib into the library directory and exit")
	ankiPath := flag.String("import-apkg", "", "import an Anki .apkg into the library and exit")
	flag.Parse()

	// The settings hold the library an earlier run chose; the flag wins over
	// them. No library at all is the window's start view, not an error.
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatalf("user data directory: %v", err)
	}
	s, err := settings.Load(dataDir)
	if err != nil {
		log.Printf("settings: %v", err)
	}
	dir := *library
	if dir == "" {
		dir = s.LibraryDir
	}

	// The offline commands and a dropped archive need a place before the
	// window exists: the flag's, the remembered one, else Remit's own.
	place := dir
	if place == "" {
		place, err = appui.DefaultLibraryDir()
		if err != nil {
			log.Fatalf("library directory: %v", err)
		}
	}
	if runOffline(place, *export, *importPath) {
		return
	}
	if runImportAnki(dir, *ankiPath) {
		return
	}

	// One instance per user: a second launch forwards its URLs and exits, and
	// opening a library never races the running app.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	// The window the About window belongs to, and the binder that owns the
	// global shortcut: both are set once the app and its window exist, and the
	// seams below read them when they run.
	var (
		mainWin *mygo.Window
		hotkey  *hotkeyBinder
	)

	// The agent API and the shortcut are the window's services, so the app is
	// told about them as seams: the settings page turns the port on and off and
	// rebinds the key while the app runs.
	agent := newAgentService(dataDir)
	// The plugin directory is cmd's: the app lists it for the settings page and
	// asks it for the runner each role names, once per library open.
	plugins := newPluginService(filepath.Join(dataDir, "extensions"))
	a := appui.NewApp(appui.Options{
		LibraryDir:    dir,
		DataDir:       dataDir,
		BindHotkey:    func(accelerator string) error { return hotkey.apply(accelerator) },
		AgentControls: agent,
		Discover:      plugins.Discover,
		Plugin:        plugins.Open,
	})
	hotkey = newHotkeyBinder(func() {
		a.ShowQueue()
		if mainWin != nil {
			mainWin.Focus()
		}
	})
	a.OnLibraryOpen(func(lib *store.Library) {
		agent.setLibrary(lib)
		if err := agent.startIfEnabled(a.Settings().AgentServer); err != nil {
			slog.Warn("remit: agent api", "err", err)
		}
		startExtensions(lib)
	})
	mygo.App.SetName(appui.Name)
	mygo.App.SetVersion(appui.Version)

	// Diagnostics go to the app's own log directory and nowhere else.
	logDir, err := mygo.App.Path(mygo.PathLogs)
	if err != nil {
		log.Fatalf("log directory: %v", err)
	}
	logger, closeLog, err := logging.Open(logDir)
	if err != nil {
		log.Fatalf("logging: %v", err)
	}
	defer closeLog()
	slog.SetDefault(logger)

	// About lives in macOS's app menu, the only menu bar this app sets: on
	// Linux and Windows that menu would add File, Edit and View to the window
	// without an About item, as the app menu is macOS's alone.
	if runtime.GOOS == "darwin" {
		mygo.App.SetMenu(menuBar(func() { a.ShowAbout(mainWin) }))
	}

	installDeepLinks(a)
	installFileAssociations(a, filepath.Dir(place))

	mygo.App.WhenReady(func() {
		mainWin = mygo.NewWindow(mygo.WindowOptions{
			Title:     appui.Name,
			Width:     1024,
			Height:    720,
			MinWidth:  720,
			MinHeight: 520,
			// Opens where the user left it last time.
			StateKey: "main",
			Content:  myui.View(a.View),
		})

		// The shortcut the user chose, or the default; a combination another
		// app owns is reported on the settings page and in the log, never
		// swallowed.
		acc := a.Settings().HotkeyAccelerator()
		if err := hotkey.apply(acc); err != nil {
			slog.Warn("remit: global shortcut unavailable", "shortcut", acc, "err", err)
			a.NoteHotkeyFailure(err)
		}

		tray := installTray(a, func() { mainWin.Focus() })
		remindAtStartup(a, systemNotify)

		// One check after the window is up, when the user turned checks on, off
		// the UI thread: the tray item opens the releases page in the browser.
		// The settings are read here, on the UI thread, so the check's own
		// goroutine never reads state the page can change.
		started := a.Settings()
		startUpdateCheck(started,
			func(ctx context.Context) (update.Result, error) { return update.Check(ctx, started) },
			func(version string) {
				mainWin.Update(func() { tray.addUpdate(version, mygo.Shell.OpenExternal) })
			})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// menuBar is MyGo's standard menu bar, with the app menu's About item opening
// Remit's own page in place of the system's standard panel.
func menuBar(showAbout func()) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Label: "About " + appui.Name, Click: func(*mygo.MenuItem, *mygo.Window) { showAbout() }},
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleFileMenu},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu},
		{Role: mygo.RoleWindowMenu},
	})
}
