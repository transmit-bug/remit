package ext

import "encoding/json"

// This file is the shape of every hook: the Go type a hook is asked with and the
// Go type it answers, one pair per hook, with the JSON the wire already carries.
// The types are the host's source of truth — the JSON tags name the fields, and
// omitempty makes a field optional — so the contract artifact remit.d.ts is
// generated from them (internal/ext/ts) rather than written beside them.
//
// A hook's method takes and returns these types, and the row beside it in
// HookTable says how its input reaches the wire: as the object the caller built,
// or — for deriveBlock, the one hook the wire gives two arguments — as that
// input's fields. A shape is what the host and a plugin agree on, not a wire
// message of its own.

// Block is one block as a plugin sees it: the text it reads, and the id a live
// reference carries. It is the derive hook's input and a note's blocks, because a
// plugin meets one shape of block however it arrives.
type Block struct {
	// ID is absent in the view a derive hook is handed: a plugin reads the text,
	// and the host owns what the block is called.
	ID string `json:"id,omitempty"`
	// Text is the block's markdown, as the note holds it.
	Text string `json:"text"`
}

// CardType is one of a library's card types as a plugin sees it: the id it
// answers cards with, the field names the type carries, and the type's own
// version.
type CardType struct {
	ID      string   `json:"id"`
	Fields  []string `json:"fields"`
	Version int      `json:"version"`
}

// DerivedCard is one card a derive hook answers: which of the library's types it
// is, where it is numbered, and the field values the type renders with. The host
// validates it before writing — a type the library does not hold is dropped,
// with the fields it does not carry.
type DerivedCard struct {
	TypeID  string            `json:"typeId"`
	Ordinal int               `json:"ordinal"`
	Fields  map[string]string `json:"fields"`
	// Unknown is a plugin's own report of the fields it answered that the type
	// does not carry. The host logs them and drops them; the card is still made
	// from the fields that do fit.
	Unknown []string `json:"unknown,omitempty"`
}

// DeriveIn is what a deriveBlock call is asked with: the block's view of the
// text and the library's types. deriveBlock is the one hook whose wire takes two
// arguments rather than one object, so the table's two parameters are this
// input's fields, in this order (hooks.go).
type DeriveIn struct {
	// Block is the text being derived. Its id is not passed: a plugin reads the
	// text, and the host owns what the block is called.
	Block Block
	// Types are the library's card types, so a plugin can fill a type it did not
	// compile in.
	Types []CardType
}

// Policy is what a scheduling plugin schedules with, and how it names itself: the
// arithmetic stays the host's (ADR 0007), so a plugin names a retention target
// rather than an interval.
type Policy struct {
	Family    string  `json:"family"`
	Version   string  `json:"version"`
	Retention float64 `json:"retention"`
}

// SummaryIn is what a finished session is worth asking about: how many cards
// were reviewed, how many are due tomorrow, and how much was answered right.
type SummaryIn struct {
	Reviewed    int     `json:"reviewed"`
	DueTomorrow int     `json:"dueTomorrow"`
	Accuracy    float64 `json:"accuracy"`
}

// Summary is what a session is worth saying beyond its numbers: a tone, and
// markdown the host draws under its own wording. Empty lines let the host word
// the numbers itself, in the user's language.
type Summary struct {
	// Tone is "quiet" or "celebrated"; the union is the tag's, which the
	// contract reads, because reflection cannot see string literals.
	Tone string `json:"tone" remit:"quiet|celebrated"`
	// Lines is markdown.
	Lines []string `json:"lines"`
}

// Event is what happened, told to a plugin that asked to be told: a
// notification, not a veto, so the host ignores whatever the plugin answers.
type Event struct {
	// Kind is one of note.saved, card.created or review.recorded.
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// CommandIn is what a contributed command's callback and a page's data command
// are asked with: the name the contribution declared, and the window's context.
type CommandIn struct {
	// Name is the name after command: in the contribution.
	Name string `json:"name"`
	// Context is the window's state: the keys a when expression reads, plus the
	// card and note id of the moment. The contribution layer fills it.
	Context ContextJSON `json:"context"`
}

// ContextJSON is the window's context as a command is handed it: the host's
// twelve keys, partial, plus the per-call card and note ids. ext carries it as
// JSON so that it need not import the contribution layer that computes it.
type ContextJSON map[string]any

// CommandResult is what a command's callback answers. The host applies it — opens
// a page, refreshes the one on screen, says a message — so a command never
// changes the app's state itself.
type CommandResult struct {
	// Result is the action the host takes; the union is the tag's.
	Result string `json:"result,omitempty" remit:"navigate|refresh|toast|none"`
	// Page is the page to open, named by the contribution or by its id.
	Page string `json:"page,omitempty"`
	// Message is a %key% of the plugin's own catalog, or a literal.
	Message string `json:"message,omitempty"`
}

// PageTree is the other half of a command's answer: the node tree a page's data
// command returns. It is raw JSON — the contribution layer parses it, because ext
// does not import that layer and a tree is not a hook's shape — and it is an
// alias rather than a type of its own so that it decodes as one.
type PageTree = json.RawMessage

// ImportIn is one file handed to a plugin that declared it can bring one in: its
// name, its bytes, and which of the plugin's declared formats matched. The bytes
// cross the wire base64-encoded, which is what []byte marshals to; a caller hands
// the plugin bytes and never base64 of its own.
type ImportIn struct {
	Name   string `json:"name"`
	Data   []byte `json:"data"`
	Format string `json:"format"`
}

// ImportSuggestions is what an importer answers: notes to make, cards to make on
// them, and what it could not use. Suggestions, not writes — the host checks
// every one, mints the identities and writes (ADR 0007).
type ImportSuggestions struct {
	Notes    []SuggestedNote `json:"notes"`
	Cards    []SuggestedCard `json:"cards,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
}

// SuggestedNote is a note an importer suggests: its title and its blocks, as
// text. The host mints the identities, so a block here has no id.
type SuggestedNote struct {
	Title  string   `json:"title"`
	Blocks []string `json:"blocks"`
}

// SuggestedCard is a card an importer suggests, on a block of one of the notes in
// the same answer, by index: the note's index in Notes, the block's index in that
// note's Blocks.
type SuggestedCard struct {
	Note    int               `json:"note"`
	Block   int               `json:"block"`
	TypeID  string            `json:"typeId"`
	Ordinal int               `json:"ordinal"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// DescribeAnswer is what a plugin answers when the host asks what it is, in its
// own words: the name the log and the settings screen show, and its version.
type DescribeAnswer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
