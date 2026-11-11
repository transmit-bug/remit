package contrib

// Context is the window's state a when expression reads, as the context keys of
// the contribution contract name it (docs/specs/plugins/ui.md §3). A key the
// host adds is a version of its API, never a field here that a caller guesses.
type Context struct {
	// Mode is the screen the window shows: notes, review, browse, stats or
	// settings. A plugin page is none of those, so it is empty there and
	// PageID says which page it is.
	Mode string
	// Area is the part of the window a contribution sits in: today, write,
	// read, recall, stats, settings or plugin. Mode says which screen, which
	// leaves three areas inside notes; area is the one a slot's receiver knows.
	// The host fills it as its route changes, and leaves it empty until it does.
	Area string
	// Sub is the stable id of the sub-page inside that area (queue, cards, …),
	// empty where the area has a single screen.
	Sub string
	// LibraryOpen is a library read into the window.
	LibraryOpen bool
	// NoteOpen is a note in the editor.
	NoteOpen bool
	// NoteSelection is words selected inside that note's text: what a
	// contribution that acts on a passage asks for.
	NoteSelection bool
	// CardPresent is a card on screen, in review or in browse.
	CardPresent bool
	// CardStatus is that card's standing: new, learning, review, suspended or
	// reading. A lost reference is a link state, not a status.
	CardStatus string
	// BrowseSelection is a chosen row in the card list.
	BrowseSelection bool
	// ScopeActive is a review scope narrower than the whole library: one note's
	// cards or one tag's, as the recall queue's scope dialog set it.
	ScopeActive bool
	// PageID is the plugin page on screen, and PluginID the plugin it belongs
	// to; both empty elsewhere.
	PageID   string
	PluginID string
}

// Areas are what the area key answers: the window's parts, as the contract
// names them (ui.md §3). Mode stays the five screens it has always been, so a
// when expression written against it keeps its meaning.
var Areas = []string{"today", "write", "read", "recall", "stats", "settings", "plugin"}

// ContextKind is what a context key answers: the three kinds the contract has.
type ContextKind string

const (
	// ContextBool is a key that answers true or false.
	ContextBool ContextKind = "bool"
	// ContextString is a key that answers text.
	ContextString ContextKind = "string"
	// ContextEnum is a key that answers one of the values in ContextEntry.Values.
	ContextEnum ContextKind = "enum"
)

// ContextEntry is one row of the context table: the key a when expression and a
// callback payload both name, what kind of value it answers, and the reader that
// answers it.
type ContextEntry struct {
	// Key is the name a when expression writes and a payload carries.
	Key string
	// Kind is what the key answers.
	Kind ContextKind
	// Type is the contract's type for the key: boolean, string, or the name of
	// the enum in Values (Mode, Area, CardStatus).
	Type string
	// Values is an enum's vocabulary in contract order; nil for the other kinds.
	Values []string
	// Read answers the key from the window's context.
	Read func(Context) any
}

// ContextTable is the one context table, in the order the contract lists the
// keys (docs/specs/plugins/ui.md §3). The key list, a when expression's values,
// a callback payload and remit.d.ts all read it: a key cannot exist on one side
// of the contract and not another, and adding one is one edit.
var ContextTable = []ContextEntry{
	{Key: "mode", Kind: ContextEnum, Type: "Mode",
		Values: []string{"notes", "review", "browse", "stats", "settings", ""},
		Read:   func(c Context) any { return c.Mode }},
	{Key: "area", Kind: ContextEnum, Type: "Area", Values: Areas,
		Read: func(c Context) any { return c.Area }},
	{Key: "sub", Kind: ContextString, Type: "string",
		Read: func(c Context) any { return c.Sub }},
	{Key: "library.open", Kind: ContextBool, Type: "boolean",
		Read: func(c Context) any { return c.LibraryOpen }},
	{Key: "note.open", Kind: ContextBool, Type: "boolean",
		Read: func(c Context) any { return c.NoteOpen }},
	{Key: "note.selection", Kind: ContextBool, Type: "boolean",
		Read: func(c Context) any { return c.NoteSelection }},
	{Key: "card.present", Kind: ContextBool, Type: "boolean",
		Read: func(c Context) any { return c.CardPresent }},
	{Key: "card.status", Kind: ContextEnum, Type: "CardStatus",
		Values: []string{"new", "learning", "review", "suspended", "reading"},
		Read:   func(c Context) any { return c.CardStatus }},
	{Key: "browse.selection", Kind: ContextBool, Type: "boolean",
		Read: func(c Context) any { return c.BrowseSelection }},
	{Key: "scope.active", Kind: ContextBool, Type: "boolean",
		Read: func(c Context) any { return c.ScopeActive }},
	{Key: "page.id", Kind: ContextString, Type: "string",
		Read: func(c Context) any { return c.PageID }},
	{Key: "plugin.id", Kind: ContextString, Type: "string",
		Read: func(c Context) any { return c.PluginID }},
}

// ContextKeys are the table's keys, in its order: what a when expression may
// name and what a callback payload carries.
var ContextKeys = tableKeys()

// contextReads is the table's readers by key, so a key's value is read once and
// the same way wherever the host needs it.
var contextReads = tableReads()

func tableKeys() []string {
	keys := make([]string, 0, len(ContextTable))
	for _, entry := range ContextTable {
		keys = append(keys, entry.Key)
	}
	return keys
}

func tableReads() map[string]func(Context) any {
	readers := make(map[string]func(Context) any, len(ContextTable))
	for _, entry := range ContextTable {
		// A row without a reader is a coding error the table's test names; at
		// run time the key reads as one this host does not know, which is false.
		if entry.Read != nil {
			readers[entry.Key] = entry.Read
		}
	}
	return readers
}

// Value is what one context key answers in this context, and whether the host
// knows the key: the reading a when expression makes.
func (c Context) Value(key string) (any, bool) { return contextValue(key, c) }

// Payload is the context a plugin callback reads: every key of the table, read
// the way a when expression reads it. A command that appeared for a key is
// handed that key, with the value it was shown for.
func (c Context) Payload() map[string]any {
	payload := make(map[string]any, len(ContextKeys))
	for _, key := range ContextKeys {
		if value, ok := c.Value(key); ok {
			payload[key] = value
		}
	}
	return payload
}

// contextValue is what one context key answers, and whether the host knows it.
// A value is a bool, a string or a number and nothing else; comparisons are
// total — two values of different kinds are unequal rather than an error — so a
// when expression can never fail at run time and hide a contribution by
// accident.
func contextValue(key string, c Context) (any, bool) {
	read, ok := contextReads[key]
	if !ok {
		return nil, false
	}
	return read(c), true
}
