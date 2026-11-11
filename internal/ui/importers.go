package ui

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/suggest"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// The importers plugins declare. A plugin that contributes importers
// (contributes.importers, #153) appears in the import entries beside the
// app's own Markdown and Anki: choosing it asks the plugin's importFile hook
// what the file suggests, and the host — this code — checks every suggestion
// against the library before anything is written. Output is a suggestion;
// the write is the host's, as the contract says.

// importerRef is one enabled plugin's importer, with the plugin that
// declares it: an entry in the import list.
type importerRef struct {
	// Plugin is the plugin's manifest id: what its strings, its failures and the
	// producer stamp are keyed by.
	Plugin string
	// Name is the plugin's manifest name: what a runner is opened by.
	Name  string
	Entry contrib.Importer
}

// importers lists the importers the enabled plugins declare, sorted by
// format, from the manifests the host already parsed — no plugin starts for
// the list.
func (a *App) importers() []importerRef {
	var out []importerRef
	for _, plugin := range a.ui.index.Plugins() {
		for _, entry := range plugin.Importers {
			out = append(out, importerRef{Plugin: plugin.Plugin, Name: plugin.Name, Entry: entry})
		}
	}
	return out
}

// importerByFormat answers the enabled importer that declared format, for a
// file whose extension matched it.
func (a *App) importerByFormat(format string) (importerRef, bool) {
	for _, ref := range a.importers() {
		if ref.Entry.Format == format {
			return ref, true
		}
	}
	return importerRef{}, false
}

// importerForFile answers the first enabled importer whose extensions carry
// the file's, in the plugins' own order: one file, one importer.
func (a *App) importerForFile(path string) (importerRef, bool) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	for _, ref := range a.importers() {
		for _, known := range ref.Entry.Extensions {
			if known == ext {
				return ref, true
			}
		}
	}
	return importerRef{}, false
}

// importWithPluginEntry is the palette entry's run: a file picker filtered to
// the format's extensions, then the same hand-off one file takes. The picker
// is a seam, like every dialog in this app: an importer that arrives with a
// picker of its own uses it, the window's dialog is the fallback.
func (a *App) importWithPluginEntry(c *ui.Context, ref importerRef) {
	if a.lib == nil {
		return
	}
	label := a.pluginText(c, ref.Plugin, string(ref.Entry.Label))
	pick := a.pickPlugin
	if pick != nil {
		paths, err := pick(label, ref.Entry.Extensions)
		if err != nil {
			a.report(c, "choose the files to import", err)
			return
		}
		for _, path := range paths {
			a.importWithPlugin(c, ref, path)
		}
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       label,
		ButtonLabel: label,
		Filters:     []mygo.FileFilter{{Name: ref.Entry.Format, Extensions: ref.Entry.Extensions}},
		Multiple:    true,
	})
	if err != nil {
		a.report(c, "choose the files to import", err)
		return
	}
	for _, path := range paths {
		a.importWithPlugin(c, ref, path)
	}
}

// importWithPlugin hands one file to a plugin's importFile hook and applies
// what survives the host's checks. The plugin is the same door a contributed
// command opens; the hook asks for the file as the host's shape — the bytes, not
// base64 — and answers suggestions, and Apply is the write.
func (a *App) importWithPlugin(c *ui.Context, ref importerRef, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		a.report(c, "read the file", err)
		return
	}
	runner, err := a.plugins.Runner(ref.Name)
	if err != nil {
		a.report(c, "start the importer plugin", err)
		return
	}
	importer := ext.HooksOf(runner)
	answer, err := importer.ImportFile(ext.ImportIn{Data: data, Name: filepath.Base(path), Format: ref.Entry.Format})
	if err != nil {
		slog.Error("remit: a plugin's import failed", "plugin", ref.Plugin, "file", path, "err", err)
		c.Toast(m(c).ImportFailed(err.Error()))
		return
	}
	producer := ext.ProducerOf(runner)
	if producer.ID == "" {
		producer.ID = ref.Plugin
	}
	report, err := suggest.Apply(a.lib, suggestions(answer), producer)
	if err != nil {
		slog.Error("remit: an import's suggestions did not apply", "plugin", ref.Plugin, "err", err)
		c.Toast(m(c).ImportFailed(err.Error()))
		return
	}
	a.reload()
	slog.Info("remit: a plugin imported a file", "plugin", ref.Plugin, "file", path,
		"notes", report.Notes, "cards", report.Cards, "dropped", report.Dropped)
	c.Toast(m(c).PluginImportReport(report.Notes, report.Cards, report.Dropped))
}

// suggestions is an importer's answer as the host applies it: what a plugin
// suggested, in the shape the host's own check reads.
func suggestions(answer ext.ImportSuggestions) suggest.Suggestions {
	out := suggest.Suggestions{Warnings: answer.Warnings}
	for _, note := range answer.Notes {
		out.Notes = append(out.Notes, suggest.Note{Title: note.Title, Blocks: note.Blocks})
	}
	for _, card := range answer.Cards {
		out.Cards = append(out.Cards, suggest.Card{
			Note: card.Note, Block: card.Block, Type: card.TypeID,
			Ordinal: card.Ordinal, Fields: card.Fields,
		})
	}
	return out
}
