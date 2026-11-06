package core

import "strings"

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
