package ui

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/importmd"
	"github.com/transmit-bug/remit/internal/core/store"
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
	exportCards                        // the card table as a CSV file
)

// cardCSVName is what a card export is called: the name the export dialog and
// its report use, and the one a spreadsheet tool expects for a card table.
const cardCSVName = "cards.csv"

// cardCSVHeader is the first line of a card export: one column per cardCSVRow
// field, in that order, so a spreadsheet names them without a legend.
var cardCSVHeader = []string{
	"id", "note", "block", "type", "prompt", "answer", "status", "due", "interval", "lapses",
}

// cardCSVRow is one card as a CSV line: what identifies it, what it asks, and
// the schedule it is on. The field order is cardCSVHeader's.
type cardCSVRow struct {
	ID       core.ID
	Note     string
	Block    core.ID
	Type     string
	Prompt   string
	Answer   string
	Status   core.Status
	Due      time.Time
	Interval int
	Lapses   int
}

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
// library holds what the format writes. A disabled choice carries its reason
// beside it rather than leaving a control that does nothing.
func (a *App) exportDisabled(c *ui.Context, format exportFormat) string {
	msgs := m(c)
	if format == exportCards {
		if len(a.data.cards) == 0 {
			return msgs.ExportNoCards()
		}
		return ""
	}
	if len(a.data.notes) == 0 {
		return msgs.ExportEmptyLibrary()
	}
	return ""
}

// exportView draws the export dialog over the window: the three formats, why one
// cannot be written, and the one action that writes.
func (a *App) exportView(c *ui.Context, tk design.Tokens) {
	if !a.export.open {
		return
	}
	msgs := m(c)
	ui.DialogBase(c, &a.export.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(tk.SummaryWidth).Padding(tk.SpaceL).Gap(tk.SpaceM))
		panel.Label(msgs.ExportTitle())
		panel = kit.ArrivePanel(panel, a.anim.export)
		kit.ArriveScrim(backdrop, a.anim.export)

		panel.Children(func() {
			ui.Text(c, msgs.ExportTitle()).FontSize(tk.TypeH2).Bold()
			// One radio per format, each with what it writes under it — or, when
			// it cannot be written now, why.
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
	switch a.export.format {
	case exportArchive:
		title = msgs.ExportArchive()
	case exportCards:
		title = msgs.ExportCards()
	}
	dir, err := a.pickFolder(title)
	if err != nil {
		a.report(c, "choose an export folder", err)
		return
	}
	if dir == "" {
		return
	}
	switch a.export.format {
	case exportArchive:
		a.writeArchive(c, dir)
		return
	case exportCards:
		a.writeCards(c, dir)
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

// writeCards writes the card table into a CSV in dir: the one export whose
// rows the window composes, because what a card asks is what the card plugin
// renders and no file holds it.
func (a *App) writeCards(c *ui.Context, dir string) {
	msgs := m(c)
	rows, err := a.cardCSVRows()
	if err != nil {
		slog.Error("remit: read the cards to export", "err", err)
		c.Toast(msgs.ExportFailed(err.Error()))
		return
	}
	path := filepath.Join(dir, cardCSVName)
	file, err := os.Create(path)
	if err != nil {
		slog.Error("remit: create a card export", "path", path, "err", err)
		c.Toast(msgs.ExportFailed(err.Error()))
		return
	}
	writeErr := writeCardCSV(file, rows)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		slog.Error("remit: write a card export", "path", path, "err", err)
		c.Toast(msgs.ExportFailed(err.Error()))
		return
	}
	slog.Info("remit: exported the card table", "path", path, "rows", len(rows))
	a.export.open = false
	a.exportReported(c, msgs.ExportCardsReport(cardCSVName, len(rows)), dir)
}

// cardCSVRows is every card the library holds as an export line, in the order
// the card table reads them: due first, which is the order a schedule is
// reviewed in.
func (a *App) cardCSVRows() ([]cardCSVRow, error) {
	summaries, _, err := a.lib.QueryCards(store.CardQuery{})
	if err != nil {
		return nil, err
	}
	rows := make([]cardCSVRow, 0, len(summaries))
	for _, summary := range summaries {
		row := cardCSVRow{
			ID:       summary.ID,
			Block:    summary.Block,
			Type:     a.typeName(summary.Type),
			Status:   summary.Status,
			Due:      summary.Due,
			Interval: summary.IntervalDays,
			Lapses:   summary.Lapses,
		}
		if card, ok := a.data.card(summary.ID); ok {
			row.Prompt, row.Answer, _ = a.faces(card)
		}
		if summary.Note != "" {
			row.Note = a.data.titleOf(summary.Note)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// writeCardCSV writes the header and one line per card, UTF-8 CSV as
// encoding/csv reads back: a field holding a comma, a quote or a line break is
// quoted, and a quote inside it is doubled.
func writeCardCSV(w io.Writer, rows []cardCSVRow) error {
	out := csv.NewWriter(w)
	if err := out.Write(cardCSVHeader); err != nil {
		return err
	}
	for _, row := range rows {
		if err := out.Write([]string{
			string(row.ID),
			row.Note,
			string(row.Block),
			row.Type,
			row.Prompt,
			row.Answer,
			row.Status.String(),
			dueStamp(row.Due),
			strconv.Itoa(row.Interval),
			strconv.Itoa(row.Lapses),
		}); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

// dueStamp is a card's due moment as the CSV carries it: RFC 3339 in UTC, so a
// spreadsheet sorts it and another tool reads it back without a guess. A card
// with no due moment says nothing rather than a made-up date.
func dueStamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}
