package importmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Export writes the library's notes as Markdown files into dir, one file per
// note, named as the library names its own (<slug>-<id>.md) so the files and
// the library's notes directory agree. The file is the note's source as the
// library stores it — title line, blocks, ^block_ markers — so an import of
// the same directory finds its notes by identity and changes nothing.
//
// One note can be exported on its own with ExportNote.
func Export(dir string, lib *store.Library) (Report, error) {
	var report Report
	if lib == nil {
		return report, ErrNoLibrary
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return report, err
	}
	for _, note := range lib.Notes() {
		_, wrote, err := writeNote(dir, note.ID, lib)
		if err != nil {
			return report, err
		}
		if wrote {
			report.Files++
		} else {
			report.Skipped++
		}
	}
	exportAssets(dir, lib, &report)
	return report, nil
}

// ExportNote writes one note's file into dir and answers with its path. An
// empty title and no text is skipped rather than written: a file of nothing
// is not a note anyone takes away.
func ExportNote(dir string, id core.ID, lib *store.Library) (string, bool, error) {
	var report Report
	if lib == nil {
		return "", false, ErrNoLibrary
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	path, wrote, err := writeNote(dir, id, lib)
	if err != nil {
		return "", false, err
	}
	if wrote {
		src, _, srcErr := lib.NoteSource(id)
		if srcErr != nil {
			return "", false, srcErr
		}
		exportNoteAssets(dir, src, lib, &report)
	}
	return path, wrote, nil
}

// writeNote renders one note's source into dir and answers with the path and
// whether anything was written.
func writeNote(dir string, id core.ID, lib *store.Library) (string, bool, error) {
	note, ok := lib.Note(id)
	if !ok {
		return "", false, fmt.Errorf("importmd: no note %q", id)
	}
	empty := strings.TrimSpace(note.Title) == ""
	for _, block := range note.Blocks {
		if strings.TrimSpace(block.Text) != "" {
			empty = false
			break
		}
	}
	if empty {
		return "", false, nil
	}
	src, ok, err := lib.NoteSource(id)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, fmt.Errorf("importmd: no file for note %q", id)
	}
	path := filepath.Join(dir, exportName(lib, note))
	if err := os.WriteFile(path, src, 0o644); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// exportName is the file name an export gives one note: the same
// <slug>-<id>.md the library's notes directory holds.
func exportName(lib *store.Library, note core.Note) string {
	return fmt.Sprintf("%s-%s.md", lib.Slugify(note.Title), note.ID)
}
