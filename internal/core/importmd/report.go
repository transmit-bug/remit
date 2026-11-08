// Package importmd moves whole notes between Markdown files and the library:
// a directory of .md files becomes notes, and the library's notes become
// files. A note file is the library's own note format — an H1 title, blocks
// separated by blank lines, a ^block_ marker ending each one — so what is
// exported imports back with every block reference intact, and a file that
// never met the library still imports cleanly: paragraphs without markers get
// identities minted on save.
//
// Markdown files that are not in the library's shape are ordinary Markdown:
// the parser reads title and paragraphs the same way it reads a note file, so
// lists, fences and tables survive as the text they were. Images a note
// references by the library's relative form — ![](assets/<sha>.<ext>) — move
// with the notes (#152): an import stores the directory's assets/ subtree in
// the library, an export writes the stored assets an export's notes name next
// to the files. References left as written are counted as unreachable when
// the file they name is missing.
package importmd

// Report is what one import or export did. An import's counts are what was
// written, so a second import of the same files adds nothing and reports it:
// a note whose file carries its own identity is found by that identity, and
// one without is matched by title.
type Report struct {
	// Notes is how many notes the import wrote. An export leaves it zero and
	// counts files in Files.
	Notes int
	// Files is how many files an export wrote.
	Files int
	// Skipped is what the run left alone: an import counts files whose note
	// already held this exact content, an export counts notes with no title
	// and no text.
	Skipped int
	// Assets is how many asset files the run stored or wrote: an import
	// stores the files of the directory's assets/ subtree in the library,
	// an export writes the assets the notes reference next to the files.
	Assets int
	// Warnings name what read oddly, one per file at most; a run that has
	// more than maxWarnings adds nothing and says so at the end.
	Warnings []string
}

// maxWarnings caps the report the way importanki's does: a hundred odd files
// say enough, and the log carries what the list drops.
const maxWarnings = 100
