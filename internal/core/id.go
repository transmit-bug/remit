package core

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// ID identifies an entity in a library. It is minted once and never derived
// from a path, a title or a position, so renaming or moving anything keeps
// identity and the references that point at it intact.
type ID string

// NewID returns a random identifier tagged with the kind of entity it names,
// so a note ID and a card ID never collide in a log, a URL or a reference.
func NewID(kind string) ID {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("core: no randomness for an ID: " + err.Error())
	}
	return ID(kind + "_" + hex.EncodeToString(b))
}

// Kind returns the tag NewID put on the ID, or "" when the ID is malformed.
func (id ID) Kind() string {
	kind, _, ok := strings.Cut(string(id), "_")
	if !ok || kind == "" {
		return ""
	}
	return kind
}

// Valid reports whether the ID has the shape NewID produces.
func (id ID) Valid() bool {
	kind, rest, ok := strings.Cut(string(id), "_")
	if !ok || kind == "" || len(rest) != 32 {
		return false
	}
	_, err := hex.DecodeString(rest)
	return err == nil
}
