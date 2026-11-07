package store_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// seedType stores a hand-written type, the way a library holds a type a person
// added before the settings page existed.
func seedType(t *testing.T, lib *store.Library) core.Type {
	t.Helper()
	typ := core.Type{
		ID:     "type_vocab",
		Name:   "Vocabulary",
		Fields: []string{"Term", "Meaning", "Extra"},
		Front:  "{{Term}}",
		Back:   "{{Term}} — {{Meaning}}",
	}
	if _, err := lib.SaveType(typ); err != nil {
		t.Fatalf("SaveType: %v", err)
	}
	return typ
}

// seedCardOf makes one card of a type, with the edited values a field rename
// has to carry through.
func seedCardOf(t *testing.T, lib *store.Library, typ core.Type, fields map[string]string) core.Card {
	t.Helper()
	note := saveNote(t, lib, "Words", "hola :: hello")
	cards, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Type: typ.ID}})
	if err != nil {
		t.Fatalf("CreateCards: %v", err)
	}
	card := cards[0]
	card.Fields = fields
	card.Edited = true
	if err := lib.SaveCard(card); err != nil {
		t.Fatalf("SaveCard: %v", err)
	}
	return card
}

// A new type gets its ID minted in the store and lands in the library as a
// readable file, version 1.
func TestSaveTypeMintsAnID(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	id, err := lib.SaveType(core.Type{Name: "New", Fields: []string{"Front"}, Front: "{{Front}}", Back: "{{Front}}"})
	if err != nil {
		t.Fatalf("SaveType: %v", err)
	}
	if !id.Valid() || id.Kind() != "type" {
		t.Errorf("minted ID = %q, want a type ID", id)
	}
	typ, ok := lib.Type(id)
	if !ok || typ.Name != "New" || typ.Version != 1 {
		t.Fatalf("Type = (%+v, %v)", typ, ok)
	}
	if _, err := os.Stat(filepath.Join(dir, "types", string(id)+".json")); err != nil {
		t.Errorf("the type file was not written: %v", err)
	}
}

// A type a card cannot be rendered against is refused before anything is
// written, and the refusal names what is wrong.
func TestSaveTypeGuards(t *testing.T) {
	cases := []struct {
		name string
		typ  core.Type
	}{
		{"no name", core.Type{ID: "type_x", Fields: []string{"Front"}}},
		{"no fields", core.Type{ID: "type_x", Name: "Empty"}},
		{"field without a name", core.Type{ID: "type_x", Name: "Blank", Fields: []string{""}}},
		{"field named twice", core.Type{ID: "type_x", Name: "Twice", Fields: []string{"Front", "Front"}}},
		{"id that names a path", core.Type{ID: "type/../x", Name: "Path", Fields: []string{"Front"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lib, dir, _ := newLibrary(t)
			if _, err := lib.SaveType(tc.typ); !errors.Is(err, store.ErrBadType) {
				t.Fatalf("SaveType = %v, want ErrBadType", err)
			}
			entries, err := os.ReadDir(filepath.Join(dir, "types"))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Name() == string(tc.typ.ID)+".json" {
					t.Fatalf("a refused type left a file behind: %s", entry.Name())
				}
			}
		})
	}
}

// Renaming a field rewrites every card's stored key in the store's write path:
// the value stays with the card, under the name the type now has, on disk and
// after a reopen.
func TestFieldRenameKeepsEveryReference(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	typ := seedType(t, lib)
	card := seedCardOf(t, lib, typ, map[string]string{"Term": "hola", "Meaning": "hello", "Extra": "greeting"})

	typ.Fields = []string{"Term", "Meaning", "Note"}
	typ.Back = "{{Term}} — {{Meaning}} ({{Note}})"
	if _, err := lib.SaveType(typ); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	stored, ok := lib.Card(card.ID)
	if !ok {
		t.Fatal("the card is gone")
	}
	if got := stored.Fields["Note"]; got != "greeting" {
		t.Errorf("Note = %q, want the value the old field held", got)
	}
	if _, ok := stored.Fields["Extra"]; ok {
		t.Errorf("the old field name survived: %v", stored.Fields)
	}
	if stored.Fields["Term"] != "hola" || stored.Fields["Meaning"] != "hello" {
		t.Errorf("a rename touched fields it did not name: %v", stored.Fields)
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, "cards", string(card.ID)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(onDisk, []byte(`"Note"`)) || bytes.Contains(onDisk, []byte(`"Extra"`)) {
		t.Errorf("the card file still holds the old name:\n%s", onDisk)
	}

	lib = reopen(t, lib, dir, clk)
	stored, _ = lib.Card(card.ID)
	if got := stored.Fields["Note"]; got != "greeting" {
		t.Errorf("after reopen Note = %q, want greeting", got)
	}
	saved, ok := lib.Type(typ.ID)
	if !ok || len(saved.Fields) != 3 || saved.Fields[2] != "Note" {
		t.Errorf("the type on disk = %+v, want the renamed field", saved)
	}
}

// Editing a template alone rewrites no card: the card's own values are not the
// type's business.
func TestTemplateEditLeavesCardsAlone(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	typ := seedType(t, lib)
	card := seedCardOf(t, lib, typ, map[string]string{"Extra": "greeting"})
	before, err := os.ReadFile(filepath.Join(dir, "cards", string(card.ID)+".json"))
	if err != nil {
		t.Fatal(err)
	}

	typ.Front = "**{{Term}}**"
	typ.Back = "{{Meaning}}"
	if _, err := lib.SaveType(typ); err != nil {
		t.Fatalf("SaveType: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "cards", string(card.ID)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("a template edit rewrote the card:\n%s\nwant\n%s", after, before)
	}
}

// A field a card holds a value in cannot be deleted: the refusal says which
// field and how many cards keep it, and nothing is written.
func TestFieldRemovalWithAValueIsRefused(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	typ := seedType(t, lib)
	card := seedCardOf(t, lib, typ, map[string]string{"Extra": "greeting"})
	before, err := os.ReadFile(filepath.Join(dir, "types", string(typ.ID)+".json"))
	if err != nil {
		t.Fatal(err)
	}

	typ.Fields = []string{"Term", "Meaning"}
	_, err = lib.SaveType(typ)
	var held *store.FieldInUseError
	if !errors.As(err, &held) {
		t.Fatalf("SaveType = %v, want a FieldInUseError", err)
	}
	if held.Field != "Extra" || held.Cards != 1 {
		t.Errorf("the refusal names (%q, %d cards), want (Extra, 1 card)", held.Field, held.Cards)
	}
	if !errors.Is(err, store.ErrFieldInUse) {
		t.Errorf("SaveType = %v, want it to match ErrFieldInUse", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "types", string(typ.ID)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("a refused save rewrote the type:\n%s\nwant\n%s", after, before)
	}
	if stored, _ := lib.Card(card.ID); stored.Fields["Extra"] != "greeting" {
		t.Errorf("a refused save touched the card: %v", stored.Fields)
	}
}

// A field no card holds a value in can go, and a value a card carries under it
// (empty) is dropped with it.
func TestFieldRemovalOfAnEmptyValueIsAllowed(t *testing.T) {
	lib, _, _ := newLibrary(t)
	typ := seedType(t, lib)
	card := seedCardOf(t, lib, typ, map[string]string{"Term": "hola", "Extra": ""})

	typ.Fields = []string{"Term", "Meaning"}
	if _, err := lib.SaveType(typ); err != nil {
		t.Fatalf("SaveType: %v", err)
	}
	stored, _ := lib.Card(card.ID)
	if _, ok := stored.Fields["Extra"]; ok {
		t.Errorf("the removed field left an empty key behind: %v", stored.Fields)
	}
	if stored.Fields["Term"] != "hola" {
		t.Errorf("the removal touched another field: %v", stored.Fields)
	}
}

// A type some card references cannot be deleted; one nothing uses can, and the
// built-ins never can.
func TestDeleteTypeGuards(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	typ := seedType(t, lib)
	seedCardOf(t, lib, typ, nil)

	err := lib.DeleteType(typ.ID)
	var used *store.TypeInUseError
	if !errors.As(err, &used) {
		t.Fatalf("DeleteType = %v, want a TypeInUseError", err)
	}
	if used.Cards != 1 || used.ID != typ.ID {
		t.Errorf("the refusal names (%q, %d cards), want (%q, 1 card)", used.ID, used.Cards, typ.ID)
	}
	if _, ok := lib.Type(typ.ID); !ok {
		t.Error("the referenced type was deleted anyway")
	}
	if _, err := os.Stat(filepath.Join(dir, "types", string(typ.ID)+".json")); err != nil {
		t.Errorf("the referenced type's file is gone: %v", err)
	}

	if err := lib.DeleteType(core.TypeBasic); !errors.Is(err, store.ErrBuiltinType) {
		t.Errorf("DeleteType(builtin) = %v, want ErrBuiltinType", err)
	}
	if err := lib.DeleteType("type_ghost"); !errors.Is(err, store.ErrNoType) {
		t.Errorf("DeleteType(unknown) = %v, want ErrNoType", err)
	}
}

// An unused type is deleted, file and catalog, and stays gone after a reopen.
func TestDeleteTypeRemovesAnUnusedType(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	typ := seedType(t, lib)
	if err := lib.DeleteType(typ.ID); err != nil {
		t.Fatalf("DeleteType: %v", err)
	}
	if _, ok := lib.Type(typ.ID); ok {
		t.Error("the type is still in the catalog")
	}
	if _, err := os.Stat(filepath.Join(dir, "types", string(typ.ID)+".json")); !os.IsNotExist(err) {
		t.Errorf("the type file is still there: %v", err)
	}
	lib = reopen(t, lib, dir, clk)
	if _, ok := lib.Type(typ.ID); ok {
		t.Error("the deleted type came back on reopen")
	}
	for _, builtin := range core.BuiltinTypes() {
		if _, ok := lib.Type(builtin.ID); !ok {
			t.Errorf("the built-in %q is missing after a reopen", builtin.ID)
		}
	}
}
