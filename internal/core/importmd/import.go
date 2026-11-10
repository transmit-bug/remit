package importmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// ErrNoLibrary is a nil library, the way importanki reports one.
var ErrNoLibrary = errors.New("importmd: no library")

// Options configures an import.
type Options struct {
	// KeepExisting leaves a note the library already holds as it is: a file
	// that names it — by a block's marker, or by its title — is read and
	// counted, never written. Off, such a file replaces the note it names,
	// which is what importing a folder edited outside the library means.
	KeepExisting bool
}

// outcome is what a file's import did to the library.
type outcome uint8

const (
	// wrote means the file's note is in the library with the file's text.
	wrote outcome = iota
	// skipped means the library already said exactly this, or the file could
	// not be written: nothing changed, and a warning names what went wrong.
	skipped
	// kept means the library holds the note the file names, and the import
	// left it alone although the file says something else.
	kept
)

// Import brings Markdown files into the library. Files are the .md files of
// path — one file or a directory, read recursively — and each becomes one
// note: the first H1 is the title, or the file's name when there is none;
// the rest is blocks. A file that carries the marker of a block already in
// the library names that note; one without falls back to its title, and only
// when the text it holds differs from what the titled note already says. What
// the run does with such a note is Options.KeepExisting's: keep it, or write
// the file over it.
//
// Files import in name order, so a run is reproducible: the same directory
// twice leaves the library where the first run left it.
func Import(path string, lib *store.Library, opts Options) (Report, error) {
	var report Report
	if lib == nil {
		return report, ErrNoLibrary
	}
	files, err := markdownFiles(path)
	if err != nil {
		return report, err
	}
	if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
		importAssets(path, lib, &report)
	}

	// Title -> note, for the unnamed files: it is the only provenance a plain
	// Markdown file carries.
	byTitle := map[string]core.ID{}
	for _, note := range lib.Notes() {
		byTitle[note.Title] = note.ID
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return report, fmt.Errorf("read %s: %w", file, err)
		}
		done, warn := importFile(file, data, lib, byTitle, opts)
		if warn != "" {
			warnf(&report, "%s", warn)
		}
		switch done {
		case wrote:
			report.Notes++
		case kept:
			report.Kept++
		default:
			report.Skipped++
		}
	}
	return report, nil
}

// importFile writes one file's note and answers what it did. warn is empty or
// one warning about the file.
func importFile(file string, data []byte, lib *store.Library, byTitle map[string]core.ID, opts Options) (outcome, string) {
	title, blocks := store.ParseNoteSource(data)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(file), ".md")
	}

	// The file names its note when any block carries an ID the library holds:
	// identity, not similarity, is what updates a note.
	var known core.ID
	for _, block := range blocks {
		if block.ID == "" {
			continue
		}
		if _, ok := lib.Block(block.ID); ok {
			if owner := ownerOf(lib, block.ID); owner != "" {
				known = owner
				break
			}
		}
	}
	if known != "" {
		// The same note saying the same thing needs no write: an export
		// imported back leaves the library where it was.
		if same, ok := lib.Note(known); ok && notesEqual(same.Blocks, blocks) && same.Title == title {
			return skipped, ""
		}
		if opts.KeepExisting {
			return kept, ""
		}
	}
	if known == "" {
		// An unnamed file falls back to its title, and only when the text
		// differs: two notes may share a title, so identical text means the
		// run has been here, and a different text is the note's new saying.
		if id, ok := byTitle[title]; ok {
			if same, ok := lib.Note(id); ok && notesEqual(same.Blocks, blocks) {
				return skipped, ""
			}
			if opts.KeepExisting {
				return kept, ""
			}
			known = id
		}
	}

	note := core.Note{Title: title, Blocks: blocks}
	if known != "" {
		note.ID = known
	}
	if _, err := lib.SaveNote(note); err != nil {
		// The note the library holds stays as it was, so the file ranks with the
		// ones that changed nothing; the warning names what went wrong.
		return skipped, fmt.Sprintf("%s: %v", file, err)
	}
	return wrote, ""
}

// notesEqual says whether two block lists carry the same text, ignoring
// identity: a file without markers and the note it was saved as hold the same
// words under different IDs.
func notesEqual(a, b []core.Block) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Text != b[i].Text {
			return false
		}
	}
	return true
}

// ownerOf names the note holding block, or "" when the index and the note
// files disagree.
func ownerOf(lib *store.Library, block core.ID) core.ID {
	for _, note := range lib.Notes() {
		if _, ok := note.Block(block); ok {
			return note.ID
		}
	}
	return ""
}

// markdownFiles lists the .md files path holds: the file itself, or a
// directory walk's, name order so a run reads the same twice.
func markdownFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.EqualFold(filepath.Ext(path), ".md") {
			return []string{path}, nil
		}
		return nil, fmt.Errorf("%s: not a Markdown file", path)
	}
	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".md") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// warnf appends a warning, or drops the ones past the cap.
func warnf(r *Report, format string, args ...any) {
	if len(r.Warnings) >= maxWarnings {
		return
	}
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}
