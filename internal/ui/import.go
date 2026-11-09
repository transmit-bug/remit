package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/transfer"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// The import wizard is the window's one way in: a `.remitlib` archive, a set of
// Markdown files, or an Anki package, each read by the importer that already
// exists — transfer.Read, importmd.Import and importanki.Import. The wizard adds
// no format of its own: it asks which of the three, shows what the source holds
// before writing, and reports what the write did.

// importStep is where the wizard is.
type importStep uint8

const (
	importStepSource importStep = iota
	importStepPreview
	importStepDone
)

// importSource is one place a library comes from.
type importSource uint8

const (
	importFromArchive  importSource = iota // a .remitlib, unpacked into a folder of its own
	importFromMarkdown                     // one or more .md files
	importFromAnki                         // an Anki .apkg package
)

// importWizard is the dialog's session: the step, the source chosen and what its
// picker answered, the two options a run takes, and what the run wrote.
type importWizard struct {
	open   bool
	step   importStep
	source importSource
	// paths is the picker's answer: the archive or package chosen, or the
	// Markdown files. Both steps read it as the proof that a source answered.
	paths []string
	// unmatched is why the last picker answer could not be used, shown on the
	// step that asks for one.
	unmatched string
	// rebuild indexes the library again once the run wrote, and queue adds the
	// notes it brought to the reading queue. Both are on by default: they are
	// what a reading library wants of an import.
	rebuild, queue bool
	// written is what the run wrote, for the page that closes it; failed is why
	// it did not, shown where the run was started with the dialog still open.
	written importWritten
	failed  string
	focus   ui.Handle
	list    ui.ListState
}

// importWritten is what one run of the wizard wrote, in the counts the importers
// report.
type importWritten struct {
	notes, cards, reviews, skipped int
	// opened is the folder a .remitlib was unpacked into: that import writes a
	// library of its own rather than notes into the one open.
	opened string
}

// openImport starts the wizard at its first step, with nothing chosen.
func (a *App) openImport(c *ui.Context) {
	a.anim.wizard++
	a.wizard = importWizard{open: true, step: importStepSource, rebuild: true, queue: true}
	a.wizard.focus.Focus(c)
}

// chooseImportSource asks the source's picker for what it reads and moves on to
// the preview. A canceled picker chooses nothing and stays where it was: a click
// taken back is not a source.
func (a *App) chooseImportSource(c *ui.Context, source importSource) {
	w := &a.wizard
	w.unmatched = ""
	var picked []string
	var err error
	switch source {
	case importFromArchive:
		var path string
		path, err = a.pickArchive()
		picked = nonEmpty(path)
	case importFromMarkdown:
		picked, err = a.pickMd()
	case importFromAnki:
		var path string
		path, err = a.pickAnki()
		picked = nonEmpty(path)
	}
	if err != nil {
		slog.Error("remit: choose an import source", "err", err)
		w.unmatched = err.Error()
		c.Toast(m(c).ImportFailed(err.Error()))
		return
	}
	if len(picked) == 0 {
		return
	}
	w.source, w.paths, w.step = source, picked, importStepPreview
}

// nonEmpty is a chosen path as the wizard's list, or nothing when the dialog was
// canceled.
func nonEmpty(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}

// importPreview is what the preview step reports of the source chosen: what it
// is, how much of it there is, and where the run writes. The counts come from the
// source itself — the files chosen, or the one package — so that nothing is
// written to find them out.
func (a *App) importPreview(c *ui.Context) [][2]string {
	msgs := m(c)
	w := &a.wizard
	source, destination := "", a.dir
	switch w.source {
	case importFromArchive:
		// An archive is a library of its own: it lands in a folder beside
		// itself, named after it.
		source = msgs.ImportFromArchive()
		if len(w.paths) > 0 {
			destination = archiveDir(w.paths[0])
		}
	case importFromMarkdown:
		source = msgs.ImportFromMarkdown()
	case importFromAnki:
		source = msgs.ImportFromAnki()
	}
	return [][2]string{
		{msgs.ImportRowSource(), source},
		{msgs.ImportRowFiles(), fmt.Sprint(len(w.paths))},
		{msgs.ImportRowDestination(), shortPath(destination)},
	}
}

// shortPath is a path as the dialog shows it: the home directory as ~, so a
// destination fits the panel.
func shortPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// runImport writes what the source holds, through the importer that owns its
// format, and moves to the page that reports it. A failure stays on the preview
// with its reason: the dialog closes on a write, not on an error.
func (a *App) runImport(c *ui.Context) {
	w := &a.wizard
	if a.lib == nil {
		return
	}
	w.failed = ""
	switch w.source {
	case importFromArchive:
		a.unpackArchive(c)
		return
	case importFromMarkdown:
		var written importWritten
		for _, path := range w.paths {
			report, err := a.importMd(path)
			if err != nil {
				slog.Error("remit: import Markdown", "path", path, "err", err)
				w.failed = err.Error()
				return
			}
			written.notes += report.Notes
			written.skipped += report.Skipped
			logWarnings("Markdown import", report.Warnings)
		}
		w.written = written
	case importFromAnki:
		path := w.paths[0]
		report, err := a.importAnki(path)
		if err != nil {
			slog.Error("remit: import Anki package", "path", path, "err", err)
			w.failed = err.Error()
			return
		}
		w.written = importWritten{
			notes: report.Notes, cards: report.Cards,
			reviews: report.Reviews, skipped: report.SkippedRows,
		}
		logWarnings("Anki import", report.Warnings)
	}
	a.finishImport(c)
}

// finishImport applies the two options and reports the run: the notes are read
// back, the index is rebuilt if it was asked for, and the notes that arrived are
// queued for reading.
func (a *App) finishImport(c *ui.Context) {
	w := &a.wizard
	before := make(map[core.ID]bool, len(a.data.notes))
	for _, note := range a.data.notes {
		before[note.ID] = true
	}
	if w.rebuild {
		if err := a.lib.Rebuild(); err != nil {
			a.report(c, "rebuild the index", err)
		}
	}
	if w.queue {
		now := c.Now()
		for _, note := range a.lib.Notes() {
			if before[note.ID] {
				continue
			}
			if err := a.lib.AddToReading(note.ID, now); err != nil {
				a.report(c, "add to reading", err)
				break
			}
		}
	}
	a.reload()
	w.step = importStepDone
	slog.Info("remit: imported a source", "source", w.source, "files", len(w.paths),
		"notes", w.written.notes, "cards", w.written.cards, "skipped", w.written.skipped)
}

// logWarnings puts an importer's warnings in the log: there can be a hundred of
// them, and the report is for what the user counts.
func logWarnings(what string, warnings []string) {
	for _, warning := range warnings {
		slog.Warn("remit: "+what, "warning", warning)
	}
}

// unpackArchive reads a .remitlib into a folder of its own beside the archive and
// opens the library it holds. The archive is a library, not notes in the one
// open: transfer.Read refuses a folder that already holds one, so the wizard
// looks for a free folder rather than overwriting anything.
func (a *App) unpackArchive(c *ui.Context) {
	path := a.wizard.paths[0]
	dir, err := freeDir(archiveDir(path))
	if err != nil {
		a.wizard.failed = err.Error()
		return
	}
	f, err := os.Open(path)
	if err != nil {
		a.wizard.failed = err.Error()
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		a.wizard.failed = err.Error()
		return
	}
	if err := transfer.Read(f, info.Size(), dir); err != nil {
		slog.Error("remit: unpack a library archive", "path", path, "dir", dir, "err", err)
		a.wizard.failed = err.Error()
		return
	}
	a.OpenLibrary(dir)
	if a.dir != dir {
		// The archive read cleanly, but the library in it did not open: the
		// window keeps the one it had, and the wizard says so rather than
		// reporting a write that did not become the library.
		a.wizard.failed = m(c).ImportArchiveUnreadable(shortPath(dir))
		return
	}
	a.wizard.written = importWritten{opened: dir}
	a.wizard.step = importStepDone
	slog.Info("remit: unpacked a library archive", "path", path, "dir", dir)
}

// archiveDir is where a .remitlib unpacks: a folder of its own named after the
// archive, beside it.
func archiveDir(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if base == "" || strings.HasPrefix(base, ".") || base == string(filepath.Separator) {
		base = "remitlib"
	}
	return filepath.Join(filepath.Dir(path), base)
}

// freeDir is dir, or the first of dir-2 to dir-99 that does not exist yet: an
// archive never writes over a library that is already there.
func freeDir(dir string) (string, error) {
	for i := 1; i < 100; i++ {
		candidate := dir
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", dir, i)
		}
		if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("remit: no free folder beside %s", dir)
}

// importView draws the wizard over the window.
func (a *App) importView(c *ui.Context, tk design.Tokens) {
	if !a.wizard.open {
		return
	}
	msgs := m(c)
	ui.DialogBase(c, &a.wizard.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(tk.SettingsWidth).Padding(tk.SpaceL).Gap(tk.SpaceM))
		panel.Label(msgs.ImportTitle())
		panel = kit.ArrivePanel(panel, a.anim.wizard)
		kit.ArriveScrim(backdrop, a.anim.wizard)

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, msgs.ImportTitle()).FontSize(tk.TypeH2).Bold().Grow(1)
				ui.Button(c, "").Label(msgs.ImportClose()).Tooltip(msgs.ImportClose()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { a.wizard.open = false })
			})
			importSteps(c, tk, msgs, a.wizard.step)
			ui.Divider(c)
			switch a.wizard.step {
			case importStepSource:
				a.importSourceStep(c, tk, msgs)
			case importStepPreview:
				a.importPreviewStep(c, tk, msgs)
			default:
				a.importDoneStep(c, tk, msgs)
			}
		})
	})
}

// importSteps is the three step names: a badge each, the step reached as the
// accent one. ui.Stepper steps a number, not a process, so the row is a
// composition of the two widgets that carry a name and a state.
func importSteps(c *ui.Context, tk design.Tokens, msgs locales.Messages, step importStep) {
	names := []string{msgs.ImportStepSource(), msgs.ImportStepPreview(), msgs.ImportStepDone()}
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		for i, name := range names {
			if i > 0 {
				// The dividers are the row: a badge names a step, and a rule
				// between two of them is what a step row is made of.
				ui.Divider(c).Height(tk.TypeLabel * 2).AlignSelf(ui.Center)
			}
			badge := ui.Badge(c, name).FontSize(tk.TypeLabel)
			if importStep(i) == step {
				badge.Background(tk.Accent.Alpha(0.12)).TextColor(tk.Accent)
			} else {
				badge.Background(tk.Sunken).TextColor(tk.TextMuted)
			}
		}
	})
}

// importSourceStep is the first step: the three places a library comes from, one
// row each, and the one way on. Each row asks its own picker, so what is chosen
// and what is read are the same answer.
func (a *App) importSourceStep(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		a.importSourceRow(c, tk, "inbox", msgs.ImportFromArchive(), msgs.ImportFromArchiveHint(), importFromArchive)
		a.importSourceRow(c, tk, "file-text", msgs.ImportFromMarkdown(), msgs.ImportFromMarkdownHint(), importFromMarkdown)
		a.importSourceRow(c, tk, "layers", msgs.ImportFromAnki(), msgs.ImportFromAnkiHint(), importFromAnki)
	})
	if a.wizard.unmatched != "" {
		ui.Text(c, a.wizard.unmatched).FontSize(tk.TypeLabel).TextColor(tk.Danger)
	}
	ui.Divider(c)
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		// Next is live only once a source answered its picker: what the step
		// asks for is the source, so without one there is nothing to preview.
		reason := ""
		if len(a.wizard.paths) == 0 {
			reason = msgs.ImportChooseFirst()
		}
		if reason != "" {
			ui.Text(c, reason).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).Grow(1)
		} else {
			ui.Spacer(c)
		}
		ui.PrimaryButton(c, msgs.ImportNext()).Disabled(reason != "").OnClick(func() {
			a.wizard.step = importStepPreview
		})
	})
}

// importSourceRow is one place a library comes from: its name, what it reads, and
// the picker that asks for it. The row is the button — one click both chooses the
// source and asks for its files. The first row carries the dialog's focus, so the
// keyboard starts inside it.
func (a *App) importSourceRow(c *ui.Context, tk design.Tokens, icon, name, hint string, source importSource) {
	row := ui.ButtonBase(c.Key("import-"+name)).Justify(ui.Start).Gap(tk.SpaceM).
		Padding(tk.SpaceS, tk.SpaceM).MinHeight(tk.Hit).Radius(tk.RadiusControl).
		Border(tk.Hairline, tk.Border).Label(name)
	if a.wizard.source == source && len(a.wizard.paths) > 0 {
		row.Background(tk.Accent.Alpha(0.10))
	}
	row.OnClick(func() { a.chooseImportSource(c, source) })
	if source == importFromArchive {
		row.Bind(&a.wizard.focus)
	}
	row.Children(func() {
		tk.Icon(c, icon, design.IconMedium)
		ui.Column(c).Gap(tk.SpaceXS).Grow(1).Children(func() {
			ui.Text(c, name).FontSize(tk.TypeUI).SingleLine()
			ui.Text(c, hint).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
		})
	})
}

// importPreviewStep is the second step: what the source holds, where the run
// writes, the two options, and the one action that writes.
func (a *App) importPreviewStep(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	preview := a.importPreview(c)
	columns := []ui.TableColumn{{Title: msgs.ImportRowItem()}, {Title: msgs.ImportRowValue()}}
	ui.Table(c, &a.wizard.list, columns, len(preview), func(row, col int) {
		text := preview[row][col]
		value := ui.Text(c, text).FontSize(tk.TypeUI).SingleLine()
		if col == 1 {
			value.TextColor(tk.TextMuted)
		}
	}).Label(msgs.ImportStepPreview())
	// The archive is a library of its own and rebuilds its index as it opens, so
	// the two options below belong to the imports that write into this one.
	if a.wizard.source != importFromArchive {
		ui.Form(c, func() {
			ui.Field(c, msgs.ImportRebuild(), func() { ui.Switch(c, &a.wizard.rebuild) })
			ui.Field(c, msgs.ImportQueue(), func() { ui.Switch(c, &a.wizard.queue) })
		})
	}
	ui.Divider(c)
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		// An empty source cannot be imported, and the step says why rather than
		// leaving a button that does nothing.
		reason := ""
		if len(a.wizard.paths) == 0 {
			reason = msgs.ImportEmptySource()
		}
		if a.wizard.failed != "" {
			reason = a.wizard.failed
		}
		if reason != "" {
			ui.Text(c, reason).FontSize(tk.TypeLabel).TextColor(tk.Danger).Grow(1)
		} else {
			ui.Spacer(c)
		}
		ui.Button(c, msgs.ImportBack()).OnClick(func() {
			a.wizard.step, a.wizard.failed = importStepSource, ""
		})
		ui.PrimaryButton(c, msgs.ImportRun()).Disabled(reason != "").OnClick(func() { a.runImport(c) })
	})
}

// importDoneStep is the third step: what the run wrote, and the one way out.
func (a *App) importDoneStep(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	written := a.wizard.written
	sentence := msgs.ImportMdReport(written.notes, written.skipped)
	if a.wizard.source == importFromAnki {
		sentence = msgs.ImportReport(written.notes, written.cards, written.reviews, written.skipped)
	}
	if written.opened != "" {
		sentence = msgs.ImportArchiveDone(shortPath(written.opened))
	}
	kit.Empty(c, "check", sentence, func() {
		ui.PrimaryButton(c, msgs.Close()).OnClick(func() { a.wizard.open = false })
	})
}

// pickRemitlib is the archive picker: the system's file dialog, limited to a
// library archive. It blocks until the user answers, which is why a handler calls
// it.
func pickRemitlib() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       locales.ImportFromArchive(),
		ButtonLabel: locales.ImportFromArchive(),
		Filters:     []mygo.FileFilter{{Name: locales.RemitlibArchive(), Extensions: []string{"remitlib"}}},
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
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
	if err != nil || len(paths) == 0 {
		return "", err
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
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}
