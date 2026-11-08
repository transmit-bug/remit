package store

import (
	"fmt"
	"strings"

	"github.com/transmit-bug/remit/internal/core"
)

const (
	// titlePrefix starts the line a note's title is written on.
	titlePrefix = "# "
	// blockKind is the tag every block ID carries, so a stray ^card_… in the
	// prose is not mistaken for a marker.
	blockKind = core.BlockKind
)

// line is one line of a note file and the byte range it occupies there.
type line struct {
	start, end int // end is just past the last byte, before the line ending
	text       string
}

// parsedBlock is a block read from a note file, with the offset its text ends
// at: that is where a marker goes when one is appended.
type parsedBlock struct {
	core.Block
	end int
}

// parseNote reads a note file into its title and blocks. A block ends at the
// first line carrying an ID marker or at a blank line; text is trimmed and the
// marker is not part of it. An unmarked block has no ID.
func parseNote(src []byte) (string, []parsedBlock) {
	lines := splitLines(src)

	title := ""
	if len(lines) > 0 {
		if name, ok := strings.CutPrefix(lines[0].text, titlePrefix); ok {
			title = strings.TrimSpace(name)
			lines = lines[1:]
		}
	}

	var (
		blocks []parsedBlock
		open   []line
	)
	flush := func(id core.ID) {
		if len(open) == 0 {
			return
		}
		blocks = append(blocks, assemble(open, id))
		open = nil
	}

	for _, l := range lines {
		if strings.TrimSpace(l.text) == "" {
			flush("")
			continue
		}
		if id, text, ok := markerAt(l.text); ok {
			open = append(open, line{start: l.start, end: l.start + len(text), text: text})
			flush(id)
			continue
		}
		open = append(open, l)
	}
	flush("")

	return title, blocks
}

// assemble turns the lines of one block into a block, recording where its
// trimmed text ends.
func assemble(lines []line, id core.ID) parsedBlock {
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = l.text
	}
	return parsedBlock{
		Block: core.Block{ID: id, Text: strings.TrimSpace(strings.Join(texts, "\n"))},
		end:   contentEnd(lines),
	}
}

// contentEnd is the offset just past the last non-space byte of a block.
func contentEnd(lines []line) int {
	for i := len(lines) - 1; i >= 0; i-- {
		text := lines[i].text
		end := len(text)
		for end > 0 && isSpace(text[end-1]) {
			end--
		}
		if end > 0 {
			return lines[i].start + end
		}
	}
	return lines[0].start
}

// splitLines cuts a note file into lines. A line's range excludes its line
// ending, and a trailing carriage return is not part of the text, so a CRLF
// file and an LF file read the same and offsets stay byte-exact.
func splitLines(src []byte) []line {
	var lines []line
	for start := 0; start < len(src); {
		end := start
		for end < len(src) && src[end] != '\n' {
			end++
		}
		text := string(src[start:end])
		text = strings.TrimSuffix(text, "\r")
		lines = append(lines, line{start: start, end: start + len(text), text: text})
		start = end + 1
	}
	return lines
}

// markerAt reads the ID a line ends with, as the content model defines the
// marker: this package only checks that it names a block.
func markerAt(text string) (core.ID, string, bool) {
	id, rest, ok := core.SplitBlockMarker(text)
	if !ok || id.Kind() != blockKind {
		return "", text, false
	}
	return id, rest, true
}

// normalizeNote is the note as the file can hold it: a trimmed title, trimmed
// block text, an ID on every block, and no empty block a reader would take for
// a separator. Saving returns this, so a library and its files agree.
func normalizeNote(note core.Note) core.Note {
	blocks := make([]core.Block, 0, len(note.Blocks))
	for _, block := range note.Blocks {
		block.Text = strings.TrimSpace(block.Text)
		if block.Text == "" && block.ID == "" {
			continue
		}
		if block.ID == "" {
			block.ID = core.NewID(blockKind)
		}
		blocks = append(blocks, block)
	}
	note.Title = strings.TrimSpace(note.Title)
	note.Blocks = blocks
	return note
}

// renderNote writes a normalized note in the file format: the title, then
// blocks separated by blank lines, then nothing else.
func renderNote(note core.Note) []byte {
	var out strings.Builder
	if note.Title != "" {
		out.WriteString(titlePrefix)
		out.WriteString(note.Title)
		out.WriteString("\n\n")
	} else if first := firstLine(note); strings.HasPrefix(first, titlePrefix) {
		// Without a title line, a block that opens with "# " would read back as
		// one. An empty first line says there is no title.
		out.WriteString("\n")
	}
	for i, block := range note.Blocks {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(block.Text)
		out.WriteString(" ")
		out.WriteString(core.MarkerSigil)
		out.WriteString(string(block.ID))
		out.WriteString("\n")
	}
	return []byte(out.String())
}

// ParseNoteSource reads a note file's bytes back into title and blocks: the
// same parse an open library does, so an importer and a library agree on what
// a note file says. Blocks carrying no marker keep ID "" and SaveNote mints
// them one.
func ParseNoteSource(src []byte) (title string, blocks []core.Block) {
	name, parsed := parseNote(src)
	for _, block := range parsed {
		blocks = append(blocks, block.Block)
	}
	return name, blocks
}

// RenderNoteSource writes a note in the file format: the same render a
// library's note files carry, so an exported file and a stored file agree.
func RenderNoteSource(note core.Note) []byte {
	return renderNote(note)
}

// withMarker returns src with id appended to the block whose text is text, and
// every other byte of the file untouched. A block that already carries the
// marker is left alone. The block is found by position when the text at that
// position is still the one the card is for, and by text otherwise, so a stale
// handle cannot put a marker on the wrong paragraph.
func withMarker(src []byte, id core.ID, text string, at int) ([]byte, error) {
	_, blocks := parseNote(src)
	for _, b := range blocks {
		if b.ID == id {
			return src, nil
		}
	}

	text = strings.TrimSpace(text)
	target := -1
	if at >= 0 && at < len(blocks) && blocks[at].ID == "" && blocks[at].Text == text {
		target = at
	} else {
		for i, b := range blocks {
			if b.ID == "" && b.Text == text {
				target = i
				break
			}
		}
	}
	if target < 0 {
		return nil, fmt.Errorf("store: block %q is not in the note source", id)
	}

	block := blocks[target]
	out := make([]byte, 0, len(src)+len(id)+2)
	out = append(out, src[:block.end]...)
	out = append(out, ' ')
	out = append(out, core.MarkerSigil...)
	out = append(out, id...)
	return append(out, src[block.end:]...), nil
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\v' || b == '\f'
}

// firstLine is the line the first block would start with.
func firstLine(note core.Note) string {
	for _, block := range note.Blocks {
		if text := strings.TrimSpace(block.Text); text != "" || block.ID != "" {
			return text
		}
	}
	return ""
}
