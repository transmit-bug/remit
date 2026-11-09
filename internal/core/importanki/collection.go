package importanki

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// notetypeCloze is Anki's notetype type for cloze deletion notes.
const notetypeCloze = 1

// notetype is what a note row and its cards need: the field names in order
// and the templates that turn fields into two card sides — the shape of a
// Remit card type, however the collection spells it.
type notetype struct {
	id        int64
	name      string
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
	Name   string         `json:"name"`
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

// parseModels reads the JSON object col.models.
func parseModels(raw string) (map[int64]*notetype, error) {
	var file map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &file); err != nil {
		return nil, fmt.Errorf("importanki: col.models: %w", err)
	}
	if len(file) == 0 {
		return nil, errors.New("importanki: col.models holds no note types and the collection has no notetypes table")
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
		nt := &notetype{id: id, name: model.Name, kind: model.Type}
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

// parseNotetypes turns the schema 18 split tables into the same notetype
// shape col.models yields: an ordered field list and the templates a card is
// rendered from.
func parseNotetypes(rows []dbNotetype) map[int64]*notetype {
	models := make(map[int64]*notetype, len(rows))
	for _, row := range rows {
		nt := &notetype{id: row.id, name: row.name, kind: row.kind}
		for _, field := range row.fields {
			nt.fields = append(nt.fields, field.name)
		}
		for _, tmpl := range row.templates {
			nt.templates = append(nt.templates, template{
				ord:   tmpl.ord,
				name:  tmpl.name,
				front: tmpl.front,
				back:  tmpl.back,
			})
		}
		models[row.id] = nt
	}
	return models
}

// parseDeckRows reads the schema 18 decks table into deck id → name.
func parseDeckRows(rows []dbDeck) map[int64]string {
	decks := make(map[int64]string, len(rows))
	for _, row := range rows {
		if name := strings.TrimSpace(row.name); name != "" {
			decks[row.id] = name
		}
	}
	return decks
}

// models returns the collection's note types: the schema 18 split tables when
// the package has them, col.models JSON otherwise.
func (c *collection) models() (map[int64]*notetype, error) {
	if c.split {
		return parseNotetypes(c.notetypes), nil
	}
	return parseModels(c.modelsJSON)
}

// deckNames returns the collection's deck names, and false when the legacy
// JSON could not be read — a split table is plain rows, so an empty one simply
// names no deck.
func (c *collection) deckNames() (map[int64]string, bool) {
	if c.split {
		return parseDeckRows(c.decks), true
	}
	names := parseDecks(c.decksJSON)
	return names, names != nil
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

// label names a notetype in a warning: its name when the collection gave it
// one, its id otherwise.
func (n *notetype) label() string {
	if name := strings.TrimSpace(n.name); name != "" {
		return fmt.Sprintf("notetype %q", name)
	}
	return fmt.Sprintf("notetype %d", n.id)
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

// Protobuf wire types, named for the two a notetype config uses.
const (
	protoWireVarint  = 0
	protoWireFixed64 = 1
	protoWireBytes   = 2
	protoWireFixed32 = 5
)

// notetypeKind reads a notetype config's kind: 0 is a standard notetype and 1
// a cloze one (anki.notetypes.Notetype.Config, field 1). An unreadable blob
// reads as standard, which is what a notetype without a kind means.
func notetypeKind(blob []byte) int {
	kind := 0
	walkProto(blob, func(field, wire int, varint uint64, _ []byte) {
		if field == 1 && wire == protoWireVarint {
			kind = int(varint)
		}
	})
	return kind
}

// templateFormats reads a template config's front and back formats
// (anki.notetypes.Notetype.Template.Config, fields 1 and 2). ok is false when
// the blob is not a message this importer can walk, so the caller can say so
// rather than import a template that silently lost a side.
func templateFormats(blob []byte) (front, back string, ok bool) {
	ok = walkProto(blob, func(field, wire int, _ uint64, bytes []byte) {
		switch {
		case field == 1 && wire == protoWireBytes:
			front = string(bytes)
		case field == 2 && wire == protoWireBytes:
			back = string(bytes)
		}
	})
	return front, back, ok
}

// walkProto visits a protobuf message's fields in order and reports whether
// the buffer is a well-formed message. Only what this importer needs is
// decoded; the other fields are skipped by their wire type.
func walkProto(blob []byte, visit func(field, wire int, varint uint64, bytes []byte)) bool {
	for i := 0; i < len(blob); {
		key, next, ok := protoVarint(blob, i)
		if !ok {
			return false
		}
		field, wire := int(key>>3), int(key&0x7)
		i = next
		switch wire {
		case protoWireVarint:
			value, next, ok := protoVarint(blob, i)
			if !ok {
				return false
			}
			visit(field, wire, value, nil)
			i = next
		case protoWireFixed64:
			if len(blob)-i < 8 {
				return false
			}
			visit(field, wire, 0, nil)
			i += 8
		case protoWireBytes:
			length, next, ok := protoVarint(blob, i)
			if !ok || length > uint64(len(blob)-next) {
				return false
			}
			visit(field, wire, 0, blob[next:next+int(length)])
			i = next + int(length)
		case protoWireFixed32:
			if len(blob)-i < 4 {
				return false
			}
			visit(field, wire, 0, nil)
			i += 4
		default:
			return false
		}
	}
	return true
}

// protoVarint reads a base-128 varint, protobuf's encoding of a field key and
// of the lengths and enum values beside it.
func protoVarint(b []byte, i int) (value uint64, next int, ok bool) {
	var shift uint
	for i < len(b) && shift < 64 {
		c := b[i]
		value |= uint64(c&0x7f) << shift
		i++
		if c&0x80 == 0 {
			return value, i, true
		}
		shift += 7
	}
	return 0, i, false
}
