// Command remit is the Remit desktop application.
package main

import (
	"flag"
	"log"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/logging"
	"github.com/transmit-bug/remit/internal/settings"
	appui "github.com/transmit-bug/remit/internal/ui"
)

func main() {
	library := flag.String("library", "", "the library directory (default: Remit in Documents)")
	export := flag.String("export", "", "write the library to a .remitlib file and exit")
	importPath := flag.String("import", "", "unpack a .remitlib into the library directory and exit")
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

	// One instance per user: a second launch forwards its URLs and exits, and
	// opening a library never races the running app.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	a := appui.NewApp(appui.Options{LibraryDir: dir, DataDir: dataDir})
	a.OnLibraryOpen(startAgentServer)
	a.OnLibraryOpen(startExtensions)
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

	// The window the About window belongs to, set once the app is ready.
	var mainWin *mygo.Window

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
		installHotkey(a)
		installTray(a, func() { mainWin.Focus() })
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
