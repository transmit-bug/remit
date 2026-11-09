package ui

import (
	"log/slog"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/importmd"
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

// pickMarkdown is the Markdown picker: one .md file or several, from a
// folder the user opens. Like every dialog here it blocks, so a handler
// calls it.
func pickMarkdown() ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       locales.ImportMarkdown(),
		ButtonLabel: locales.ImportMarkdown(),
		Filters:     []mygo.FileFilter{{Name: locales.MarkdownFiles(), Extensions: []string{"md", "markdown"}}},
		Multiple:    true,
	})
}

// pickNoteImage is the picture picker the slash menu opens: one file the
// library can decode and store as an asset.
func pickNoteImage() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       locales.PickImage(),
		ButtonLabel: locales.InsertImage(),
		Filters: []mygo.FileFilter{{
			Name:       locales.ImageFiles(),
			Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "bmp", "svg"},
		}},
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// pickExportFolder is the folder picker an export writes into. It blocks,
// so a handler calls it.
func pickExportFolder(title string) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:             title,
		ButtonLabel:       locales.ChooseExportFolder(),
		Directory:         true,
		CreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return paths[0], nil
}

// importMarkdownFiles asks for Markdown files and imports them as notes, the
// way importAnkiPackage does: pick, write, reload, report.
func (a *App) importMarkdownFiles(c *ui.Context) {
	if a.lib == nil || a.importMd == nil {
		return
	}
	paths, err := a.pickMd()
	if err != nil {
		a.report(c, "choose Markdown files", err)
		return
	}
	if len(paths) == 0 {
		return // the dialog was canceled: nothing to do, and nothing to report
	}
	var report importmd.Report
	for _, path := range paths {
		one, err := a.importMd(path)
		if err != nil {
			slog.Error("remit: import Markdown", "path", path, "err", err)
			c.Toast(m(c).ImportMdFailed(err.Error()))
			return
		}
		report.Notes += one.Notes
		report.Skipped += one.Skipped
		for _, warning := range one.Warnings {
			slog.Warn("remit: Markdown import", "warning", warning)
		}
	}
	a.reload()
	slog.Info("remit: imported Markdown files", "files", len(paths),
		"notes", report.Notes, "skipped", report.Skipped)
	c.Toast(m(c).ImportMdReport(report.Notes, report.Skipped))
}

// exportNoteMarkdown writes the note open in the editor as one Markdown file.
func (a *App) exportNoteMarkdown(c *ui.Context) {
	if a.lib == nil {
		return
	}
	id := a.openNoteID()
	if id == "" {
		c.Toast(m(c).ExportNothingOpen())
		return
	}
	dir, err := a.pickFolder(m(c).ExportNote())
	if err != nil {
		a.report(c, "choose an export folder", err)
		return
	}
	if dir == "" {
		return
	}
	path, wrote, err := importmd.ExportNote(dir, id, a.lib)
	if err != nil {
		slog.Error("remit: export note", "note", id, "dir", dir, "err", err)
		c.Toast(m(c).ExportFailed(err.Error()))
		return
	}
	if wrote {
		slog.Info("remit: exported a note", "path", path)
		c.Toast(m(c).ExportReport(1, 0))
	}
}

// exportAllMarkdown writes every note's file into one folder.
func (a *App) exportAllMarkdown(c *ui.Context) {
	if a.lib == nil {
		return
	}
	dir, err := a.pickFolder(m(c).ExportAllNotes())
	if err != nil {
		a.report(c, "choose an export folder", err)
		return
	}
	if dir == "" {
		return
	}
	report, err := importmd.Export(dir, a.lib)
	if err != nil {
		slog.Error("remit: export notes", "dir", dir, "err", err)
		c.Toast(m(c).ExportFailed(err.Error()))
		return
	}
	slog.Info("remit: exported notes", "dir", dir, "files", report.Files, "skipped", report.Skipped)
	c.Toast(m(c).ExportReport(report.Files, report.Skipped))
}
