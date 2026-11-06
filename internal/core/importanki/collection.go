package importanki

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// notetypeCloze is Anki's notetype type for cloze deletion notes.
const notetypeCloze = 1

// notetype is what a note row and its cards need from col.models: the field
// names in order and the templates that turn fields into two card sides.
type notetype struct {
	id        int64
	kind      int
	fields    []string
	templates []template
}

type template struct {
	ord   int
	name  string
	front string
	back  string
}

type modelFile struct {
	Type   int            `json:"type"`
	Fields []fieldFile    `json:"flds"`
	Tmpls  []templateFile `json:"tmpls"`
}

type fieldFile struct {
	Name string `json:"name"`
	Ord  int    `json:"ord"`
}

type templateFile struct {
	Name string `json:"name"`
	Ord  int    `json:"ord"`
	QFmt string `json:"qfmt"`
	AFmt string `json:"afmt"`
}

// parseModels reads the JSON object col.models. An empty one means the
// collection uses Anki's schema 18 split tables, which this importer does not
// read; saying so beats importing notes with no templates.
func parseModels(raw string) (map[int64]*notetype, error) {
	var file map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &file); err != nil {
		return nil, fmt.Errorf("importanki: col.models: %w", err)
	}
	if len(file) == 0 {
		return nil, fmt.Errorf("importanki: col.models is empty: only Anki schema 11 collections are supported; re-export with legacy support")
	}

	models := make(map[int64]*notetype, len(file))
	for key, data := range file {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("importanki: col.models: %q is not a notetype id", key)
		}
		var model modelFile
		if err := json.Unmarshal(data, &model); err != nil {
			return nil, fmt.Errorf("importanki: col.models: notetype %s: %w", key, err)
		}
		nt := &notetype{id: id, kind: model.Type}
		for _, field := range sortedFields(model.Fields) {
			nt.fields = append(nt.fields, field.Name)
		}
		for _, tmpl := range sortedTemplates(model.Tmpls) {
			nt.templates = append(nt.templates, template{
				ord:   tmpl.Ord,
				name:  tmpl.Name,
				front: tmpl.QFmt,
				back:  tmpl.AFmt,
			})
		}
		models[id] = nt
	}
	return models, nil
}

func sortedFields(fields []fieldFile) []fieldFile {
	out := append([]fieldFile(nil), fields...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Ord < out[j].Ord })
	return out
}

func sortedTemplates(templates []templateFile) []templateFile {
	out := append([]templateFile(nil), templates...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Ord < out[j].Ord })
	return out
}

// parseDecks reads col.decks into deck id → name. A deck name is cosmetic, so a
// damaged map yields no names rather than a failed import.
func parseDecks(raw string) map[int64]string {
	var file map[string]struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &file); err != nil {
		return nil
	}
	decks := make(map[int64]string, len(file))
	for key, deck := range file {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || deck.Name == "" {
			continue
		}
		decks[id] = deck.Name
	}
	return decks
}

// fieldIndex finds a field by the name a template used. Anki matches field
// names case-insensitively.
func (n *notetype) fieldIndex(name string) int {
	for i, field := range n.fields {
		if field == name {
			return i
		}
	}
	for i, field := range n.fields {
		if strings.EqualFold(field, name) {
			return i
		}
	}
	return -1
}

// templateFor picks the template a card row's ordinal names. A cloze card's
// ordinal is the cloze number, not a template index, so every cloze card uses
// the single cloze template.
func (n *notetype) templateFor(ord int) (template, bool) {
	if n.kind == notetypeCloze {
		if len(n.templates) == 0 {
			return template{}, false
		}
		return n.templates[0], true
	}
	for _, tmpl := range n.templates {
		if tmpl.ord == ord {
			return tmpl, true
		}
	}
	return template{}, false
}

// frontFieldName is the first field the front template names, ignoring section
// markers, comments and the built-in variables every template may use. For a
// reverse template such as {{#Add Reverse}}{{Back}}{{/Add Reverse}} that is
// "Back", which is the side the card is shown first.
func frontFieldName(tmpl string) string {
	for i := 0; i < len(tmpl); {
		start := strings.Index(tmpl[i:], "{{")
		if start < 0 {
			return ""
		}
		i += start
		end := strings.Index(tmpl[i+2:], "}}")
		if end < 0 {
			return ""
		}
		inner := strings.TrimSpace(tmpl[i+2 : i+2+end])
		i += 2 + end + 2
		if inner == "" || strings.ContainsAny(inner[:1], "#/^!") {
			continue
		}
		name := inner
		if cut := strings.IndexByte(name, ':'); cut >= 0 {
			name = name[cut+1:]
		}
		name = strings.TrimSpace(name)
		if name == "" || strings.EqualFold(name, "frontside") {
			continue
		}
		return name
	}
	return ""
}

// renderTemplate fills an Anki template with a note's field text. It handles
// the parts that change what a card says — field substitution, the text: and
// cloze: filters, {{#Field}}/{{^Field}} sections, and comments — and drops what
// it does not know. {{FrontSide}} renders as nothing: the prompt is already on
// Remit's other side.
func renderTemplate(tmpl string, values map[string]string) string {
	type section struct {
		name string
		show bool
	}
	var open []section
	visible := func() bool {
		for _, s := range open {
			if !s.show {
				return false
			}
		}
		return true
	}

	var out strings.Builder
	for i := 0; i < len(tmpl); {
		start := strings.Index(tmpl[i:], "{{")
		if start < 0 {
			if visible() {
				out.WriteString(tmpl[i:])
			}
			break
		}
		if start > 0 && visible() {
			out.WriteString(tmpl[i : i+start])
		}
		i += start
		end := strings.Index(tmpl[i+2:], "}}")
		if end < 0 {
			if visible() {
				out.WriteString(tmpl[i:])
			}
			break
		}
		inner := strings.TrimSpace(tmpl[i+2 : i+2+end])
		i += 2 + end + 2
		if inner == "" {
			continue
		}
		switch inner[0] {
		case '#', '^':
			name := strings.TrimSpace(inner[1:])
			present := values[lookupKey(name)] != ""
			if inner[0] == '^' {
				present = !present
			}
			open = append(open, section{name: name, show: present})
		case '/':
			name := strings.TrimSpace(inner[1:])
			for len(open) > 0 {
				last := open[len(open)-1]
				open = open[:len(open)-1]
				if strings.EqualFold(last.name, name) {
					break
				}
			}
		case '!':
			// A comment renders nothing.
		default:
			if visible() {
				out.WriteString(substitute(inner, values))
			}
		}
	}
	return out.String()
}

// lookupKey names a value the way a template does: filters stripped and case
// ignored.
func lookupKey(name string) string {
	if cut := strings.IndexByte(name, ':'); cut >= 0 {
		name = name[cut+1:]
	}
	return strings.ToLower(strings.TrimSpace(name))
}

func substitute(token string, values map[string]string) string {
	return values[lookupKey(token)]
}
