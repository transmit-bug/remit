package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// importerContributes is one plugin declaring one format it can bring in.
const importerContributes = `{
	"importers": [
		{"format": "acme.obsidian", "extensions": ["md"], "label": "%importer.label%"}
	],
	"locales": {"en": {"importer.label": "Obsidian export"}}
}`

// importFileAnswer is what a plugin suggests for one file: one note of two
// blocks, one card on the first block, one suggestion the host will drop.
const importFileAnswer = `{
	"notes": [{"title": "From the plugin", "blocks": ["first block", "second block"]}],
	"cards": [
		{"note": 0, "block": 0, "typeId": "remit.basic", "ordinal": 0,
		 "fields": {"Front": "first block", "Back": "the answer"}},
		{"note": 0, "block": 9, "typeId": "remit.basic", "ordinal": 1}
	]
}`

// TestPluginImporterLivesInThePaletteAndApplies walks the whole road: the
// palette lists the importer, choosing it runs the file picker, the hook's
// suggestions are checked and the ones that hold up are written, and the
// window reports what was written and what the host dropped.
func TestPluginImporterLivesInThePaletteAndApplies(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(file, []byte("# From the plugin"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{reply: map[string]string{"importFile": importFileAnswer}}
	a, _ := contribApp(t, "obsidian", importerContributes, runner)

	picked := 0
	a.pickPlugin = func(label string, extensions []string) ([]string, error) {
		picked++
		if label != "Obsidian export" {
			t.Errorf("picker label = %q, want the plugin's own", label)
		}
		if len(extensions) != 1 || extensions[0] != "md" {
			t.Errorf("picker extensions = %v, want the declared ones", extensions)
		}
		return []string{file}, nil
	}

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("Obsidian")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(320 * time.Millisecond)

	if picked != 1 {
		t.Fatalf("the picker ran %d times", picked)
	}
	if runner.calls() == 0 {
		t.Fatalf("the importFile hook never ran")
	}
	found := false
	for _, note := range a.Library().Notes() {
		if note.Title == "From the plugin" && len(note.Blocks) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("the suggested note is not in the library with its two blocks")
	}
	// The suggested card carries the importing plugin's stamp.
	for _, card := range a.Library().Cards() {
		if card.Producer.ID == "acme.obsidian" {
			if card.Producer.Version != "1" {
				t.Errorf("card producer = %+v, want the manifest's version", card.Producer)
			}
			found = true
		}
	}
	if !found {
		t.Error("no card carries the importer's producer stamp")
	}
	if !tt.HasText("Imported 1 notes, 1 cards; 1 suggestions dropped.") {
		t.Errorf("the report is not on the window; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A dropped plugin leaves no import entry: the palette lists what the window
// can do, and a withdrawn plugin cannot be imported through.
func TestPluginImporterDisappearsWithThePlugin(t *testing.T) {
	a, _ := contribApp(t, "obsidian", importerContributes, &fakeRunner{reply: map[string]string{"importFile": importFileAnswer}})
	a.pickPlugin = func(string, []string) ([]string, error) { return nil, nil }
	if entries := a.importers(); len(entries) != 1 {
		t.Fatalf("importers = %+v, want the enabled plugin's one", entries)
	}
	a.lib.SetEnabledExtensions(nil)
	a.loadContributions()
	if entries := a.importers(); len(entries) != 0 {
		t.Errorf("importers after withdrawal = %+v, want none", entries)
	}
}
