package ui

import (
	"log/slog"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// importAnkiPackage asks for an Anki package and imports it into the open
// library, then reloads the window and reports what the import wrote. The
// warnings go to the log: there can be a hundred of them, and the toast is for
// what the user counts.
//
// The import runs on the UI thread, as every write in this app does (see
// reload): a package is read and written once, from a handler, so the report
// is in the next frame instead of arriving from a goroutine that a newer
// screen would have to guard against.
func (a *App) importAnkiPackage(c *ui.Context) {
	if a.lib == nil || a.importAnki == nil {
		return
	}
	path, err := a.pickAnki()
	if err != nil {
		a.report(c, "choose an Anki package", err)
		return
	}
	if path == "" {
		return // the dialog was canceled: nothing to do, and nothing to report
	}

	report, err := a.importAnki(path)
	if err != nil {
		slog.Error("remit: import Anki package", "path", path, "err", err)
		c.Toast(m(c).ImportFailed(err.Error()))
		return
	}
	for _, warning := range report.Warnings {
		slog.Warn("remit: Anki import", "warning", warning)
	}
	a.reload()
	slog.Info("remit: imported an Anki package", "path", path,
		"notes", report.Notes, "cards", report.Cards, "reviews", report.Reviews,
		"skipped", report.SkippedRows)
	c.Toast(m(c).ImportReport(report.Notes, report.Cards, report.Reviews, report.SkippedRows))
}

// pickAnkiPackage is the package picker: the system's file dialog, limited to
// Anki's .apkg. It blocks until the user answers, which is why it is only ever
// called from a handler.
func pickAnkiPackage() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       locales.ImportAnki(),
		ButtonLabel: locales.ImportAnki(),
		Filters:     []mygo.FileFilter{{Name: locales.AnkiPackage(), Extensions: []string{"apkg"}}},
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return paths[0], nil
}
