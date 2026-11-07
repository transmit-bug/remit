package core

// Type is a named card schema: an ordered field list plus the front and back
// template sources rendered by the host template engine (spec §2.1). Types
// are plain-text library data; card faces are never stored, they are computed
// from a card's field values at render time.
type Type struct {
	ID      ID // minted once; built-ins are the remit.* constants
	Name    string
	Fields  []string // ordered, unique, non-empty
	Front   string   // template source
	Back    string
	Version int
}

// Built-in type IDs.
const (
	TypeBasic         = ID("remit.basic")
	TypeBidirectional = ID("remit.bidirectional")
	TypeCloze         = ID("remit.cloze")
)

// BuiltinTypes returns the three built-in card types (spec §2.2): basic
// ("::" prompt/answer), bidirectional (":::", both directions derived by
// swapping the fields — the ordinal records the direction) and cloze
// ("{{…}}" blanks, one card per blank, behaviour follows the rendered side).
func BuiltinTypes() []Type {
	return []Type{
		{
			ID:      TypeBasic,
			Name:    "Basic",
			Fields:  []string{"Front", "Back"},
			Front:   "{{Front}}",
			Back:    "{{Back}}",
			Version: 1,
		},
		{
			ID:      TypeBidirectional,
			Name:    "Bidirectional",
			Fields:  []string{"Front", "Back"},
			Front:   "{{Front}}",
			Back:    "{{Back}}",
			Version: 1,
		},
		{
			ID:      TypeCloze,
			Name:    "Cloze",
			Fields:  []string{"Text"},
			Front:   "{{cloze:Text}}",
			Back:    "{{cloze:Text}}",
			Version: 1,
		},
	}
}
