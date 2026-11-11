package ts

import (
	"reflect"
	"strings"

	"github.com/transmit-bug/remit/internal/ext"
)

// This file is the hook section of the contract as data: the declarations
// remit.d.ts carries, in its order, and the prose that belongs to a reader of
// the contract rather than to a reader of the Go.
//
// A declaration whose fields come from a Go type names that type and nothing
// else: its fields, their order, their contract names and their optionality are
// reflection's (types.go). What is written here is what reflection cannot see — the
// contract's own name for a type, the sentence above a declaration or a field,
// and the host's views that this table does not carry yet.
//
// A field's prose is keyed by the field's contract name, and a key no field
// answers fails the section's test rather than quietly dropping the sentence
// (declarations_test.go).

// declaration is one declaration of the hook section.
type declaration struct {
	// Go is the type the declaration's fields come from; nil for a literal.
	Go reflect.Type
	// Name is the contract's name for the type, when it is not the Go name
	// (SummaryIn is SummaryInput, and the contract has spelled it that way since
	// its first version).
	Name string
	// Doc is the one-line sentence the contract prints above the declaration.
	Doc string
	// Fields is the sentence the contract prints above one field, by the
	// field's contract name.
	Fields map[string]string
	// Literal is a declaration written out in full, or the prose between them.
	Literal string
	// Hooks is the Hooks interface, rendered from ext.HookTable where it sits.
	Hooks bool
}

// hookSection is the contract's hook section: every declaration, one blank line
// apart, closed by the newline the file's last declaration ends on.
func hookSection() string {
	parts := make([]string, 0, len(hookDeclarations))
	for _, d := range hookDeclarations {
		parts = append(parts, d.render())
	}
	return strings.Join(parts, "\n\n") + "\n"
}

// hookDeclarations is the hook section, declaration by declaration.
var hookDeclarations = []declaration{
	{Literal: hostHeader},
	{Go: reflect.TypeOf(ext.Block{}), Fields: map[string]string{
		"id": "Absent in the view a derive hook is handed; the text is what it reads.",
	}},
	{Go: reflect.TypeOf(ext.CardType{})},
	{Go: reflect.TypeOf(ext.DerivedCard{}), Fields: map[string]string{
		"unknown": "Field names the type does not carry; the host logs and drops them.",
	}},

	// The host's own views: what a capability call answers. They are the note and
	// card a plugin reads, not a hook's shape, so this table does not carry them
	// yet — the migration that narrows the capability surface (#269) brings them
	// here, and the hook shapes they sit among keep the contract's order.
	{Literal: noteView},
	{Literal: cardView},
	{Literal: reviewEventView},
	{Literal: ratingUnion},
	{Literal: scheduleStateView},

	{Go: reflect.TypeOf(ext.Policy{})},
	{Go: reflect.TypeOf(ext.SummaryIn{}), Name: "SummaryInput"},
	{Go: reflect.TypeOf(ext.Summary{}), Fields: map[string]string{
		"lines": "Markdown the host draws under its own wording.",
	}},
	{Go: reflect.TypeOf(ext.Event{}),
		Doc:    "A notification, not a veto: the host ignores the answer.",
		Fields: map[string]string{"kind": "note.saved, card.created or review.recorded."},
	},
	{Go: reflect.TypeOf(ext.CommandIn{}), Name: "CommandInput",
		Doc:    "What a contributed command and a page's data are asked with.",
		Fields: map[string]string{"name": "The name the contribution declared after command:."},
	},
	{Go: reflect.TypeOf(ext.CommandResult{}),
		Doc: "What a command callback answers: the host applies the result.",
		Fields: map[string]string{
			"page":    `The page to open when result is "navigate".`,
			"message": `The message when result is "toast": a %key% or a literal.`,
		},
	},

	// What a plugin reaches in the host: the capability function set, unchanged.
	{Literal: remitObject},
	{Literal: exportsHeader},
	{Hooks: true},

	{Go: reflect.TypeOf(ext.ImportIn{}), Name: "ImportInput",
		Doc: "One file handed to an importer: its name, its bytes in base64, and which of the plugin's declared formats matched.",
	},
	{Go: reflect.TypeOf(ext.SuggestedNote{}),
		Doc: "A suggested note: the host mints identities and checks every block.",
	},
	{Go: reflect.TypeOf(ext.SuggestedCard{}),
		Doc: "A suggested card, on a block of a suggested note, by index into the same answer.",
	},
	{Go: reflect.TypeOf(ext.ImportSuggestions{})},
}

// spellings are the contract's spellings for the Go types reflection cannot
// carry on its own: the two shapes whose Go job is JSON.
var spellings = map[reflect.Type]string{
	// A window context is the host's Context, partial, plus the two ids of the
	// moment; the contribution layer fills it and ext only carries it.
	reflect.TypeOf(ext.ContextJSON(nil)): `Partial<Context> & { "card.id"?: string; "note.id"?: string }`,
	// A page's tree is raw JSON, and it is declared with the contribution surface:
	// a command's answer names it and does not declare it here.
	reflect.TypeOf(ext.PageTree(nil)): "PageTree",
}

// hookDocs is the sentence the contract prints above a hook's signature, by hook
// id. Only the two hooks whose signature does not speak for itself carry one. The
// ids are ext's own private constants, so the one that is not exported is spelled
// here and the section's test holds it to a hook the table has.
var hookDocs = map[string]string{
	ext.CommandHook: "What a contributed command and a page's data answer.",
	"importFile":    "One file in, suggestions out; the host checks and writes (#153).",
}

// The literals: the host's views and the section's own headers.
const (
	hostHeader = `// ─── the host a plugin reaches ─────────────────────────────────────────────
// A function below is there only when the manifest declared its capability; a
// call to one it did not declare throws ext.CapabilityError.`

	noteView = `export interface Note {
  id: string;
  title: string;
  blocks: Block[];
}`

	cardView = `export interface Card {
  id: string;
  block: string;
  type: string;
  ordinal: number;
  /** The user's own edited fields; the rest are derived when the card renders. */
  fields?: Record<string, string>;
  edited?: boolean;
  status: CardStatus;
  due?: string;
  reviews: number;
}`

	reviewEventView = `/** One line of the review log, as a plugin reads it: no voided events. */
export interface ReviewEvent {
  seq: number;
  card: string;
  rating: Rating;
  at: string;
  scheduler?: string;
}`

	ratingUnion = `export type Rating = "again" | "hard" | "good" | "easy";`

	scheduleStateView = `/**
 * A card's FSRS state. It is the host's shape and opaque to a plugin: carry it
 * back unchanged; the arithmetic stays the host's.
 */
export interface ScheduleState {
  stability: number;
  difficulty: number;
  due?: string;
  lastReview?: string;
  reviews: number;
  lastRating?: Rating;
}`

	remitObject = `export declare const remit: {
  /** remit.echo: answers the value back. For a plugin's own tests. */
  echo(value: unknown): unknown;
  /** remit.now: the time as RFC 3339. */
  now(): string;
  notes: {
    /** notes:read */
    list(): Note[];
    get(id: string): Note | null;
    search(text: string): Note[];
  };
  cards: {
    /** cards:write */
    create(block: string): Card;
    update(id: string, patch: { fields?: Record<string, string>; status?: CardStatus }): Card;
    delete(id: string): void;
  };
  review: {
    /** review:read */
    history(card: string): ReviewEvent[];
    dueCount(): number;
    /** review:write */
    record(card: string, rating: Rating, at?: string): Card;
  };
  fsrs: {
    /** schedule:fsrs: advances a state by one answer. */
    review(state: ScheduleState, rating: Rating, at?: string): ScheduleState;
  };
  store: {
    /** store:own: the plugin's own value, under its own id. */
    get(): string | null;
    set(value: string): void;
  };
};`

	exportsHeader = `// ─── the exports the host calls ────────────────────────────────────────────
// A plugin exports the hooks its manifest declares; the names are the host's.`
)

// render is one declaration's text: a shape from its Go type, the Hooks
// interface from the table, or a literal.
func (d declaration) render() string {
	switch {
	case d.Literal != "":
		return d.Literal
	case d.Hooks:
		return "export interface Hooks {\n" + strings.Join(hookMethods(), "\n") + "\n}"
	}
	var body []string
	if d.Doc != "" {
		body = append(body, "/** "+d.Doc+" */")
	}
	body = append(body, "export interface "+d.contractName()+" {")
	body = append(body, fieldLines(d.Go, d.Fields)...)
	body = append(body, "}")
	return strings.Join(body, "\n")
}

// contractName is the name the contract declares a shape under.
func (d declaration) contractName() string {
	if d.Name != "" {
		return d.Name
	}
	return d.Go.Name()
}

// fieldLines is one shape's body: a line per field, in the struct's order, with
// the contract's name, type and optionality — all of them the type's own.
func fieldLines(t reflect.Type, docs map[string]string) []string {
	out := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		name, optional := contractField(field)
		if name == "" {
			continue
		}
		if doc := docs[name]; doc != "" {
			out = append(out, docLine(doc))
		}
		out = append(out, "  "+name+optional+": "+fieldType(field)+";")
	}
	return out
}

// docLine is the contract's comment form for a field: a sentence on one line,
// indented with the field it describes.
func docLine(doc string) string { return "  /** " + doc + " */" }

// hookMethods is the Hooks interface's body: a signature per hook in the table,
// in the contract's order.
func hookMethods() []string {
	out := make([]string, 0, len(ext.HookTable))
	for _, row := range ext.HookTable {
		if doc := hookDocs[row.Name]; doc != "" {
			out = append(out, docLine(doc))
		}
		params := make([]string, 0, len(row.Params))
		for _, param := range row.Params {
			params = append(params, param.Name+": "+contractType(param.Type))
		}
		out = append(out, "  "+row.Name+"("+strings.Join(params, ", ")+"): "+returns(row)+";")
	}
	return out
}

// returns is what a hook answers: void, one shape, or the union of the shapes it
// may answer with.
func returns(row ext.HookType) string {
	if len(row.Returns) == 0 {
		return "void"
	}
	shapes := make([]string, 0, len(row.Returns))
	for _, shape := range row.Returns {
		shapes = append(shapes, contractType(shape))
	}
	return strings.Join(shapes, " | ")
}
