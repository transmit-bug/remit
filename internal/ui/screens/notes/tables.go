package notes

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui/richtext"

	"github.com/transmit-bug/remit/internal/core"
)

const tableRefPrefix = "remit-table:"

// noteDocument builds the editor's text from the draft's blocks. Ordinary
// Markdown is parsed as usual; a GFM table becomes one opaque attachment, so
// the editor cannot flatten it through a paragraph command. The raw table
// source stays in the draft and is put back when the note is saved.
func (s *State) noteDocument() richtext.Text {
	s.tables = nil
	var paragraphs []richtext.Paragraph
	for _, block := range s.Draft.Blocks {
		if raw, ok := markdownTable(block.Text); ok {
			if s.tables == nil {
				s.tables = map[string]string{}
			}
			id := block.ID
			if id == "" {
				id = core.NewID(core.BlockKind)
			}
			ref := tableRefFor(id)
			s.tables[ref] = raw
			name := s.tableWord
			if name == "" {
				name = "Table"
			}
			if cols, rows := tableShape(raw); cols > 0 {
				name = fmt.Sprintf("%s %d×%d", name, cols, rows)
			}
			paragraphs = append(paragraphs, richtext.Paragraph{
				Runs: []richtext.Run{{
					Text: string(richtext.ObjectReplacement),
					Attributes: richtext.Attributes{Attachment: &richtext.Attachment{
						Name:   name,
						URL:    ref,
						Width:  168,
						Height: 28,
					}},
				}},
			})
			continue
		}
		for _, paragraph := range richtext.ParseMarkdown(block.Text).Paragraphs() {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	return richtext.TextOf(paragraphs...)
}

// blocksFromLive reads the editor's Markdown back into blocks and puts each
// table's source back where its placeholder stood. The attachment's reference
// carries the block ID, so a table keeps its identity even when the note is
// reordered or blocks are inserted around it.
func (s *State) blocksFromLive(live string) []core.Block {
	blocks := blocksOfDocument(live, s.Draft.Blocks)
	for i := range blocks {
		ref, ok := tableRefOf(blocks[i].Text)
		if !ok {
			continue
		}
		raw, ok := s.tables[ref]
		if !ok {
			continue
		}
		blocks[i].Text = raw
		if id := tableIDOf(ref); id != "" {
			blocks[i].ID = core.ID(id)
		}
	}
	return blocks
}

// tableRefFor is the attachment URL that stands for one block's table.
func tableRefFor(id core.ID) string {
	return tableRefPrefix + string(id)
}

// tableRefOf recognizes the one-image paragraph Text.Markdown writes for a
// table placeholder.
func tableRefOf(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "![") {
		return "", false
	}
	at := strings.Index(text, "](")
	if at < 0 || !strings.HasSuffix(text, ")") {
		return "", false
	}
	ref := text[at+2 : len(text)-1]
	if !strings.HasPrefix(ref, tableRefPrefix) {
		return "", false
	}
	return ref, true
}

// tableIDOf is the block ID a table reference carries.
func tableIDOf(ref string) string {
	return strings.TrimPrefix(ref, tableRefPrefix)
}

// markdownTable reports whether a block is a GFM table, and returns its
// source without surrounding space. A fenced code block is never a table,
// even when the code shows one.
func markdownTable(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "```") || strings.HasPrefix(text, "~~~") {
		return "", false
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 2 || !strings.Contains(lines[0], "|") {
		return "", false
	}
	if !tableDelimiter(lines[1]) {
		return "", false
	}
	return text, true
}

// tableDelimiter reports whether a line is a GFM table's header separator:
// cells of dashes, optionally aligned with colons.
func tableDelimiter(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || (!strings.Contains(line, "|") && !strings.Contains(line, "-")) {
		return false
	}
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cells := strings.Split(line, "|")
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		cell = strings.TrimPrefix(cell, ":")
		cell = strings.TrimSuffix(cell, ":")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

// tableShape counts a table's columns and data rows for its placeholder name.
func tableShape(text string) (cols, rows int) {
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return 0, 0
	}
	cols = len(strings.Split(strings.Trim(strings.TrimSpace(lines[0]), "|"), "|"))
	rows = len(lines) - 1 // the header and data rows, without the separator
	return cols, rows
}
