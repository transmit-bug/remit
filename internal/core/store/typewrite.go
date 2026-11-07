package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/transmit-bug/remit/internal/core"
)

// Errors a type write may answer.
var (
	// ErrNoType is a type this library does not hold.
	ErrNoType = errors.New("store: no such card type")
	// ErrBadType is a type a card cannot be rendered against: no name, no
	// fields, a field named twice, or an ID that cannot name a file.
	ErrBadType = errors.New("store: the card type is not usable")
	// ErrBuiltinType is a delete of one of the three types every library
	// seeds: it would come back on the next open, so it is refused.
	ErrBuiltinType = errors.New("store: a built-in card type cannot be deleted")
	// ErrTypeInUse is a delete of a type some card still references.
	ErrTypeInUse = errors.New("store: cards still use this card type")
	// ErrFieldInUse is a delete of a field a card holds a value in.
	ErrFieldInUse = errors.New("store: a card holds a value in this field")
)

// TypeInUseError is a delete refused because cards reference the type. It says
// how many, so the page explains why rather than only refusing.
type TypeInUseError struct {
	ID    core.ID
	Cards int
}

func (e *TypeInUseError) Error() string {
	return fmt.Sprintf("store: %d cards use type %q", e.Cards, e.ID)
}

func (e *TypeInUseError) Unwrap() error { return ErrTypeInUse }

// FieldInUseError is a field deletion refused because cards of the type still
// hold a value in it (spec §2.3).
type FieldInUseError struct {
	Field string
	Cards int
}

func (e *FieldInUseError) Error() string {
	return fmt.Sprintf("store: %d cards hold a value in field %q", e.Cards, e.Field)
}

func (e *FieldInUseError) Unwrap() error { return ErrFieldInUse }

// SaveType writes a card type and answers its ID: a type with none is a new
// type, and gets one minted here. A field renamed between the type the library
// holds and the one given has every card's stored value rekeyed in the store's
// own write path, so no caller ever edits card files; deleting a field a card
// holds a value in is refused before anything is written (spec §2.3).
func (l *Library) SaveType(typ core.Type) (core.ID, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return "", ErrClosed
	}
	typ = normalizeType(typ)
	if err := checkType(typ); err != nil {
		return "", err
	}
	old, exists := l.typeLocked(typ.ID)
	var renamed map[string]string
	var removed []string
	if exists {
		renamed, removed = fieldChanges(old.Fields, typ.Fields)
		if err := l.checkFieldsHeld(typ.ID, removed); err != nil {
			return "", err
		}
	}
	// The type is the authority: write it before the cards that follow it, so
	// a failure part way leaves values keyed by a name the type no longer has
	// rather than cards that lost the type's names altogether.
	if err := writeType(l.dir, typ); err != nil {
		return "", err
	}
	if err := l.rekeyCards(typ.ID, renamed, removed); err != nil {
		return "", err
	}
	l.setType(typ)
	return typ.ID, nil
}

// DeleteType removes a type no card references. A built-in is refused: the
// next open seeds it again. A referenced type is refused with the count, so the
// page can say which cards keep it.
func (l *Library) DeleteType(id core.ID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if isBuiltinType(id) {
		return fmt.Errorf("%w: %q", ErrBuiltinType, id)
	}
	if _, ok := l.typeLocked(id); !ok {
		return fmt.Errorf("%w: %q", ErrNoType, id)
	}
	if used := l.cardsOfType(id); used > 0 {
		return &TypeInUseError{ID: id, Cards: used}
	}
	if err := os.Remove(filepath.Join(l.dir, typesDir, string(id)+".json")); err != nil {
		return err
	}
	l.removeType(id)
	return nil
}

// normalizeType trims what a person typed and mints the ID of a new type.
func normalizeType(typ core.Type) core.Type {
	typ.Name = strings.TrimSpace(typ.Name)
	fields := make([]string, len(typ.Fields))
	for i, field := range typ.Fields {
		fields[i] = strings.TrimSpace(field)
	}
	typ.Fields = fields
	if typ.ID == "" {
		typ.ID = core.NewID("type")
	}
	if typ.Version == 0 {
		typ.Version = 1
	}
	return typ
}

// checkType reports a type a card could not be rendered against.
func checkType(typ core.Type) error {
	if typ.Name == "" {
		return fmt.Errorf("%w: the type needs a name", ErrBadType)
	}
	if len(typ.Fields) == 0 {
		return fmt.Errorf("%w: the type needs at least one field", ErrBadType)
	}
	if !validTypeID(typ.ID) {
		return fmt.Errorf("%w: %q cannot name a type file", ErrBadType, typ.ID)
	}
	seen := make(map[string]bool, len(typ.Fields))
	for _, field := range typ.Fields {
		if field == "" {
			return fmt.Errorf("%w: a field needs a name", ErrBadType)
		}
		if seen[field] {
			return fmt.Errorf("%w: field %q appears twice", ErrBadType, field)
		}
		seen[field] = true
	}
	return nil
}

// fieldChanges reads what a save does to a type's fields: the names it renames,
// paired in order among those the two lists no longer share, and the names it
// removes. A reorder or a plain addition is neither.
func fieldChanges(oldFields, newFields []string) (renamed map[string]string, removed []string) {
	before := make(map[string]bool, len(oldFields))
	after := make(map[string]bool, len(newFields))
	for _, field := range oldFields {
		before[field] = true
	}
	for _, field := range newFields {
		after[field] = true
	}
	var gone, added []string
	for _, field := range oldFields {
		if !after[field] {
			gone = append(gone, field)
		}
	}
	for _, field := range newFields {
		if !before[field] {
			added = append(added, field)
		}
	}
	renamed = make(map[string]string, len(gone))
	for i := 0; i < len(gone) && i < len(added); i++ {
		renamed[gone[i]] = added[i]
	}
	if len(gone) > len(added) {
		removed = gone[len(added):]
	}
	return renamed, removed
}

// checkFieldsHeld reports the first removed field a card of the type still
// holds a value in, with how many cards do.
func (l *Library) checkFieldsHeld(id core.ID, removed []string) error {
	for _, field := range removed {
		held := 0
		for _, meta := range l.cards {
			if meta.card.Type != id {
				continue
			}
			if meta.card.Fields[field] != "" {
				held++
			}
		}
		if held > 0 {
			return &FieldInUseError{Field: field, Cards: held}
		}
	}
	return nil
}

// rekeyCards carries every card's stored field values through a type's field
// rename or removal: the values stay with their card under the names the type
// now has.
func (l *Library) rekeyCards(id core.ID, renamed map[string]string, removed []string) error {
	if len(renamed) == 0 && len(removed) == 0 {
		return nil
	}
	gone := make(map[string]bool, len(removed))
	for _, field := range removed {
		gone[field] = true
	}
	// A copy first: saveCard writes into the map the catalog holds.
	ids := make([]core.ID, 0, len(l.cards))
	for cid, meta := range l.cards {
		if meta.card.Type == id && len(meta.card.Fields) > 0 {
			ids = append(ids, cid)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, cid := range ids {
		card := l.cards[cid].card
		next := make(map[string]string, len(card.Fields))
		changed := false
		for field, value := range card.Fields {
			if to, ok := renamed[field]; ok {
				next[to] = value
				changed = true
				continue
			}
			if gone[field] {
				changed = true // the guard proved the value empty
				continue
			}
			next[field] = value
		}
		if !changed {
			continue
		}
		card.Fields = next
		card.Edited = true
		if err := l.saveCard(card); err != nil {
			return err
		}
	}
	return nil
}

// typeLocked is Type without the lock.
func (l *Library) typeLocked(id core.ID) (core.Type, bool) {
	for _, typ := range l.types {
		if typ.ID == id {
			return typ, true
		}
	}
	return core.Type{}, false
}

// setType puts a saved type in the catalog, in ID order.
func (l *Library) setType(typ core.Type) {
	for i, held := range l.types {
		if held.ID == typ.ID {
			l.types[i] = typ
			return
		}
	}
	l.types = append(l.types, typ)
	sort.Slice(l.types, func(i, j int) bool { return l.types[i].ID < l.types[j].ID })
}

// removeType takes a deleted type out of the catalog.
func (l *Library) removeType(id core.ID) {
	kept := l.types[:0]
	for _, typ := range l.types {
		if typ.ID != id {
			kept = append(kept, typ)
		}
	}
	l.types = kept
}

// cardsOfType counts the cards a type renders.
func (l *Library) cardsOfType(id core.ID) int {
	count := 0
	for _, meta := range l.cards {
		if meta.card.Type == id {
			count++
		}
	}
	return count
}

// isBuiltinType reports whether id is one of the three types every library
// seeds.
func isBuiltinType(id core.ID) bool {
	for _, typ := range core.BuiltinTypes() {
		if typ.ID == id {
			return true
		}
	}
	return false
}

// validTypeID reports whether id can name a file inside types/: a hand-written
// type may carry a plain name, and a minted one carries the type_ tag.
func validTypeID(id core.ID) bool {
	name := string(id)
	if name == "" || name == "." || name == ".." || strings.Contains(name, "..") {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}
