package importanki

import (
	"html"
	"strings"
	"unicode"
)

// blockTags are the HTML elements Anki uses as line breaks. Remit keeps text a
// line at a time, so each of these ends a block instead of being drawn.
var blockTags = map[string]bool{
	"br": true, "div": true, "p": true, "li": true, "ul": true, "ol": true,
	"tr": true, "td": true, "th": true, "table": true, "blockquote": true,
	"pre": true, "hr": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true,
}

// plainText turns an Anki field into plain text. Tags are dropped, entities are
// decoded, line-breaking tags become newlines, and a media reference is kept by
// name: <img src="x.png"> becomes [img: x.png] and [sound:x.mp3] becomes
// [sound: x.mp3]. Markup is not otherwise interpreted, so <b> and CSS styling
// are lost rather than pretended.
func plainText(raw string, media *[]string) string {
	var out strings.Builder
	for i := 0; i < len(raw); {
		switch {
		case strings.HasPrefix(raw[i:], "<!--"):
			end := strings.Index(raw[i+4:], "-->")
			if end < 0 {
				i = len(raw)
				continue
			}
			i += 4 + end + 3
		case strings.HasPrefix(raw[i:], "[sound:"):
			end := strings.IndexByte(raw[i:], ']')
			if end < 0 {
				out.WriteString(raw[i:])
				i = len(raw)
				continue
			}
			name := strings.TrimSpace(raw[i+len("[sound:") : i+end])
			out.WriteString("[sound: ")
			out.WriteString(name)
			out.WriteByte(']')
			addMedia(media, name)
			i += end + 1
		case raw[i] == '<':
			end := strings.IndexByte(raw[i:], '>')
			if end < 0 {
				out.WriteString(raw[i:])
				i = len(raw)
				continue
			}
			tag := raw[i+1 : i+end]
			i += end + 1
			name, attrs := splitTag(tag)
			switch {
			case name == "style" || name == "script":
				i = skipElement(raw, i, name)
			case name == "img":
				src := attribute(attrs, "src")
				if src == "" {
					continue
				}
				out.WriteString(" [img: ")
				out.WriteString(src)
				out.WriteString("] ")
				addMedia(media, src)
			case blockTags[name]:
				out.WriteByte('\n')
			}
		default:
			out.WriteByte(raw[i])
			i++
		}
	}
	text := html.UnescapeString(out.String())
	return strings.ReplaceAll(text, "\u00a0", " ")
}

func addMedia(media *[]string, name string) {
	if media == nil || name == "" {
		return
	}
	*media = append(*media, name)
}

// splitTag reads a tag body into its lower-case name and its attributes. A
// closing slash is not part of the name, so </div> and <div> break alike.
func splitTag(tag string) (name, attrs string) {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "/")
	cut := strings.IndexFunc(tag, unicode.IsSpace)
	if cut < 0 {
		return strings.ToLower(strings.TrimSuffix(tag, "/")), ""
	}
	return strings.ToLower(strings.TrimSuffix(tag[:cut], "/")), tag[cut:]
}

// skipElement returns the offset after the closing tag of a <style> or <script>
// element, whose contents say nothing about the note.
func skipElement(raw string, from int, name string) int {
	rest := raw[from:]
	closing := strings.Index(strings.ToLower(rest), "</"+name)
	if closing < 0 {
		return len(raw)
	}
	end := strings.IndexByte(rest[closing:], '>')
	if end < 0 {
		return len(raw)
	}
	return from + closing + end + 1
}

// attribute reads a tag attribute by name, quoted or bare.
func attribute(attrs, want string) string {
	lower := strings.ToLower(attrs)
	for from := 0; from < len(lower); {
		at := strings.Index(lower[from:], strings.ToLower(want))
		if at < 0 {
			return ""
		}
		at += from
		before := at == 0 || unicode.IsSpace(rune(lower[at-1]))
		valueAt := at + len(want)
		if !before || valueAt >= len(lower) || lower[valueAt] != '=' {
			from = at + len(want)
			continue
		}
		value := strings.TrimSpace(attrs[valueAt+1:])
		if value == "" {
			return ""
		}
		if value[0] == '"' || value[0] == '\'' {
			quote := value[0]
			if end := strings.IndexByte(value[1:], quote); end >= 0 {
				return value[1 : 1+end]
			}
			return ""
		}
		end := strings.IndexFunc(value, func(r rune) bool { return unicode.IsSpace(r) })
		if end < 0 {
			return value
		}
		return value[:end]
	}
	return ""
}

// clozeAt reports a cloze marker "{{c<digits>::" at i and where its body starts.
func clozeAt(text string, i int) (ord, body int, ok bool) {
	if !strings.HasPrefix(text[i:], "{{c") {
		return 0, 0, false
	}
	digits := i + 3
	j := digits
	ord = 0
	for j < len(text) && text[j] >= '0' && text[j] <= '9' {
		ord = ord*10 + int(text[j]-'0')
		j++
	}
	if j == digits || !strings.HasPrefix(text[j:], "::") {
		return 0, 0, false
	}
	return ord, j + 2, true
}

// clozeBody reads a cloze deletion's contents up to its closing braces, with
// the optional hint after a second "::" removed.
func clozeBody(text string, body int) (inner string, after int, ok bool) {
	end := strings.Index(text[body:], "}}")
	if end < 0 {
		return "", 0, false
	}
	inner = text[body : body+end]
	if cut := strings.Index(inner, "::"); cut >= 0 {
		inner = inner[:cut]
	}
	return inner, body + end + 2, true
}

// clozeUnwrap turns {{c1::Paris::capital}} into Paris, so a note reads as the
// text Anki would show once the card is answered.
func clozeUnwrap(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		if _, body, ok := clozeAt(text, i); ok {
			if inner, after, ok := clozeBody(text, body); ok {
				out.WriteString(inner)
				i = after
				continue
			}
		}
		out.WriteByte(text[i])
		i++
	}
	return out.String()
}

// maskCloze hides the deletion of one card behind [...], which is what Anki
// shows on the question side.
func maskCloze(text string, ord int) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		if number, body, ok := clozeAt(text, i); ok {
			if inner, after, ok := clozeBody(text, body); ok {
				if number == ord {
					out.WriteString("[...]")
				} else {
					out.WriteString(inner)
				}
				i = after
				continue
			}
		}
		out.WriteByte(text[i])
		i++
	}
	return out.String()
}

// normalizeLines collapses each line's inner whitespace and drops empty lines,
// so a field becomes the blocks a note file can hold.
func normalizeLines(text string) string {
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// splitLines returns the blocks a field's text becomes.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// truncateRunes shortens a title without splitting a character.
func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimSpace(string(runes[:limit]))
}
