package core

import "strings"

// TemplateTag marks a note as a note template: a note whose blocks carry it is
// the plain-text starting point a new note can be copied from (#154). It is an
// ordinary tag everywhere else — the template keeps its place in the editor,
// the index, search-anything-but-the-notebook, and the reading queue; only the
// notebook's list and the notes search leave it out.
const TemplateTag = "template"

// IsTemplate reports whether the note is a note template: any of its blocks
// carries the #template tag.
func IsTemplate(note Note) bool {
	for _, block := range note.Blocks {
		for _, tag := range Tags(block.Text) {
			if tag == TemplateTag {
				return true
			}
		}
	}
	return false
}

// WithoutTemplateTag returns the text with the #template tag taken out: the
// tag is the note's role, not its content, so a note started from a template
// does not itself become one (#154). Everything else — spacing between the
// tag's neighbours included — is left as it was, and a text without the tag
// comes back unchanged.
func WithoutTemplateTag(text string) string {
	if !tagList(Tags(text)).hasTemplate() {
		return text
	}
	var out strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		out.WriteString(stripTag(line, TemplateTag))
		out.WriteString("\n")
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// hasTemplate reports whether any of the tags is the template tag.
type tagList []string

func (tags tagList) hasTemplate() bool {
	for _, tag := range tags {
		if tag == TemplateTag {
			return true
		}
	}
	return false
}

// stripTag removes one standalone #tag token from a line, joining its
// neighbours with a single space.
func stripTag(line, tag string) string {
	words := strings.Fields(line)
	kept := words[:0]
	for _, word := range words {
		if strings.HasPrefix(word, "#") && word[1:] == tag {
			continue
		}
		kept = append(kept, word)
	}
	return strings.Join(kept, " ")
}

// Tags returns the tags a block's text carries: a # followed by at least one
// letter, digit, hyphen, underscore or non-ASCII character, at the start of the
// text or after a space. A # inside a fenced code block or an inline code span is
// code, not a tag.
func Tags(text string) []string {
	var out []string
	fenced := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, tag := range tagsIn(stripInlineCode(line)) {
			out = append(out, tag)
		}
	}
	return out
}

// stripInlineCode removes the `code spans` of a line: what is between a pair of
// backticks is code, and a lone backtick is text.
func stripInlineCode(line string) string {
	var out strings.Builder
	for {
		open := strings.IndexByte(line, '`')
		if open < 0 {
			out.WriteString(line)
			return out.String()
		}
		close := strings.IndexByte(line[open+1:], '`')
		if close < 0 {
			out.WriteString(line)
			return out.String()
		}
		out.WriteString(line[:open])
		out.WriteByte(' ')
		line = line[open+1+close+1:]
	}
}

// tagsIn finds the tags of one line of prose.
func tagsIn(line string) []string {
	var out []string
	for i := 0; i < len(line); i++ {
		if line[i] != '#' {
			continue
		}
		if i > 0 && line[i-1] != ' ' && line[i-1] != '\t' {
			// #pinned inside a word is not a tag.
			continue
		}
		end := i + 1
		for end < len(line) && tagByte(line[end]) {
			end++
		}
		if end == i+1 {
			continue
		}
		out = append(out, line[i+1:end])
		i = end - 1
	}
	return out
}

// tagByte reports whether b may be part of a tag: a letter, a digit, a hyphen,
// an underscore, or any byte of a multi-byte (non-ASCII) character.
func tagByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '-' || b == '_':
		return true
	case b >= 0x80:
		return true
	}
	return false
}
