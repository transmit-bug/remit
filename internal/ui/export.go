package ui

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/importmd"
	"github.com/transmit-bug/remit/internal/core/transfer"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// exportFormat is what an export writes.
type exportFormat uint8

const (
	exportMarkdown exportFormat = iota // one .md file per note
	exportArchive                      // the whole library as a .remitlib
	exportCards                        // a card CSV, which 1.0 does not write
)

// exportState is the export dialog: open or not, and which format it writes. The
// choice is the state; the write happens in the handler that closes it.
type exportState struct {
	open   bool
	format exportFormat
	focus  ui.Handle
}

// openExport shows the export dialog on the format a library wants first.
func (a *App) openExport(c *ui.Context) {
	a.anim.export++
	a.export = exportState{open: true, format: exportMarkdown}
	a.export.focus.Focus(c)
}

// exportDisabled is why a format cannot be written now, and "" when it can: the
// library holds a note, and the format exists at all. A disabled choice carries
// its reason beside it rather than leaving a control that does nothing.
func (a *App) exportDisabled(c *ui.Context, format exportFormat) string {
	msgs := m(c)
	if format == exportCards {
		// Card CSV is not in 1.0 (§10.1): the option stays visible, with the
		// reason it is not offered, so the window does not read as unfinished.
		return msgs.ExportNoCSV()
	}
	if len(a.data.notes) == 0 {
		return msgs.ExportEmptyLibrary()
	}
	return ""
}

// exportView draws the export dialog over the window: the three formats, why one
// is not offered, and the one action that writes.
func (a *App) exportView(c *ui.Context, tk design.Tokens) {
	if !a.export.open {
		return
	}
	msgs := m(c)
	ui.DialogBase(c, &a.export.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(tk.SummaryWidth).Padding(tk.SpaceL).Gap(tk.SpaceM))
		panel.Label(msgs.ExportTitle())
		arriving := panel.Keyframes("arrive", design.PanelFrames, ui.KeyframeOptions{Run: a.anim.export})
		scale := design.PanelScale + (1-design.PanelScale)*arriving
		panel.Opacity(arriving).Scale(scale, scale)
		backdrop.Opacity(arriving)

		panel.Children(func() {
			ui.Text(c, msgs.ExportTitle()).FontSize(tk.TypeH2).Bold()
			// One radio per format, each with what it writes under it. The
			// disabled ones say why below the group.
			ui.RadioGroup(c, func() {
				a.exportChoice(c, tk, msgs, exportMarkdown, msgs.ExportMarkdown(), msgs.ExportMarkdownHint())
				a.exportChoice(c, tk, msgs, exportArchive, msgs.ExportArchive(), msgs.ExportArchiveHint())
				a.exportChoice(c, tk, msgs, exportCards, msgs.ExportCards(), msgs.ExportCardsHint())
			}).Label(msgs.ExportTitle())
			ui.Divider(c)
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Spacer(c)
				ui.Button(c, msgs.ExportCancel()).OnClick(func() { a.export.open = false })
				ui.PrimaryButton(c, msgs.ExportRun()).
					Disabled(a.exportDisabled(c, a.export.format) != "").
					OnClick(func() { a.runExport(c) })
			})
		})
	})
}

// exportChoice is one format: its radio, its name, and under it what it writes —
// or, when it cannot be written, why. A disabled choice carries its reason where
// the choice is, since a disabled control shows no tooltip.
func (a *App) exportChoice(c *ui.Context, tk design.Tokens, msgs locales.Messages, format exportFormat, name, hint string) {
	radio := ui.Radio(c, &a.export.format, format, name).FontSize(tk.TypeUI)
	if format == exportMarkdown {
		// The dialog's focus lands on the format a library wants first.
		radio.Bind(&a.export.focus)
	}
	line, ink := hint, tk.TextMuted
	if reason := a.exportDisabled(c, format); reason != "" {
		radio.Disabled(true)
		line, ink = reason, tk.Danger
	}
	ui.Text(c, line).FontSize(tk.TypeLabel).TextColor(ink).Padding(0, tk.SpaceXL)
}

// runExport writes the chosen format where the user names, and reports what it
// wrote. A canceled folder writes nothing and says nothing: a click taken back is
// not an outcome.
func (a *App) runExport(c *ui.Context) {
	msgs := m(c)
	if a.lib == nil || a.exportDisabled(c, a.export.format) != "" {
		return
	}
	title := msgs.ExportMarkdown()
	if a.export.format == exportArchive {
		title = msgs.ExportArchive()
	}
	dir, err := a.pickFolder(title)
	if err != nil {
		a.report(c, "choose an export folder", err)
		return
	}
	if dir == "" {
		return
	}
	if a.export.format == exportArchive {
		a.writeArchive(c, dir)
		return
	}
	report, err := importmd.Export(dir, a.lib)
	if err != nil {
		slog.Error("remit: export notes", "dir", dir, "err", err)
		c.Toast(msgs.ExportFailed(err.Error()))
		return
	}
	slog.Info("remit: exported notes", "dir", dir, "files", report.Files, "skipped", report.Skipped)
	a.export.open = false
	a.exportReported(c, msgs.ExportReport(report.Files, report.Skipped), dir)
}

// writeArchive packages the library into a .remitlib in dir, through the same
// transfer.Write an archive is read back with.
func (a *App) writeArchive(c *ui.Context, dir string) {
	msgs := m(c)
	name := archiveName(a.dir)
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		slog.Error("remit: create a library archive", "path", path, "err", err)
		c.Toast(msgs.ExportFailed(err.Error()))
		return
	}
	if err := transfer.Write(a.dir, file); err != nil {
		file.Close()
		slog.Error("remit: write a library archive", "path", path, "err", err)
		c.Toast(msgs.ExportFailed(err.Error()))
		return
	}
	if err := file.Close(); err != nil {
		slog.Error("remit: close a library archive", "path", path, "err", err)
		c.Toast(msgs.ExportFailed(err.Error()))
		return
	}
	slog.Info("remit: exported a library archive", "path", path)
	a.export.open = false
	a.exportReported(c, msgs.ExportArchiveReport(name), dir)
}

// exportReported is what a finished export says: the outcome where the action
// was, with the one follow-up it has — showing the files it wrote.
func (a *App) exportReported(c *ui.Context, sentence, dir string) {
	c.ToastAction(sentence, m(c).ExportOpenFolder(), func() {
		mygo.Shell.ShowItemInFolder(dir)
	})
}

// archiveName is what an exported library is called: the library's own folder
// name, so the file says which library it is.
func archiveName(dir string) string {
	base := filepath.Base(strings.TrimRight(dir, string(filepath.Separator)))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "remit"
	}
	return fmt.Sprintf("%s.remitlib", base)
}
