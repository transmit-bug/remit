// Command remit is the Remit desktop application.
package main

import (
	"log"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"

	appui "github.com/transmit-bug/remit/internal/ui"
)

func main() {
	a := appui.NewApp()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "Remit",
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
