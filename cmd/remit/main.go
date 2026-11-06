// Command remit is the Remit desktop application.
package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"

	appui "github.com/transmit-bug/remit/internal/ui"
)

func main() {
	a := appui.NewApp()
	mygo.App.SetName(appui.Name)
	mygo.App.SetVersion(appui.Version)

	// The window the About window belongs to, set once the app is ready.
	var mainWin *mygo.Window

	// About lives in macOS's app menu, the only menu bar this app sets: on
	// Linux and Windows that menu would add File, Edit and View to the window
	// without an About item, as the app menu is macOS's alone.
	if runtime.GOOS == "darwin" {
		mygo.App.SetMenu(menuBar(func() { a.ShowAbout(mainWin) }))
	}

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
