package main

import (
	"context"
	"log/slog"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/settings"
	appui "github.com/transmit-bug/remit/internal/ui"
	"github.com/transmit-bug/remit/internal/update"
	"github.com/transmit-bug/remit/locales"
)

// releasesRepo is `updates.github` in mygo.json: the repository whose releases
// hold this app's builds. mygo.json is read by the packaging tool and not by
// the app, so the slug is repeated here, and TestReleasesRepoMatchesConfig keeps
// the two equal whenever the config names one.
const releasesRepo = "transmit-bug/remit"

// releasesURL is the page the tray's update item opens in the browser. Nothing
// is fetched by the app itself.
func releasesURL() string { return "https://github.com/" + releasesRepo + "/releases" }

// updateChecker is one update check as cmd runs it. update.Check asks nothing
// while the setting is off, which is the gate every check goes through.
type updateChecker func(context.Context) (update.Result, error)

// startUpdateCheck asks once, after the window is up, when the user turned
// update checks on, and hands a newer version to apply. A build without update
// keys reports nothing newer, so no item appears.
func startUpdateCheck(s settings.Settings, check updateChecker, apply func(version string)) {
	if !s.UpdateCheck || check == nil {
		return
	}
	go func() {
		result, err := check(context.Background())
		if err != nil {
			slog.Warn("remit: update check", "err", err)
			return
		}
		if result.Version == "" {
			return
		}
		slog.Info("remit: newer version available", "version", result.Version, "running", appui.Version)
		apply(result.Version)
	}()
}

// updateMenuItem is the tray item that offers a newer version: it opens the
// releases page in the browser, and does nothing else.
func updateMenuItem(version string, open func(url string) error) *mygo.MenuItem {
	return &mygo.MenuItem{
		ID: "update", Label: locales.UpdateTo(version),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			if err := open(releasesURL()); err != nil {
				slog.Warn("remit: open the releases page", "err", err)
			}
		},
	}
}
