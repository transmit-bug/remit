package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// legacyLibrary makes a library and rewinds it to the remit/1 shape by hand:
// remit.json declares the old format and the card file carries the old
// prompt/answer/kind bytes, as an upgrade would find them.
func legacyLibrary(t *testing.T) (*store.Library, string, core.Card) {
	t.Helper()
	lib, dir, _ := newLibrary(t)
	note := saveNote(t, lib, "Legacy", "the question :: the answer", "the {{first}} one")
	plain, err := firstCard(lib, note.Blocks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	cloze, err := lib.CreateCards(note.Blocks[1].ID, []core.Derived{
		{Type: core.TypeCloze, Ordinal: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	created := "2026-11-06T09:00:00Z"
	for _, card := range []core.Card{plain, cloze[0]} {
		kind := "basic"
		if card.Type == core.TypeCloze {
			kind = "cloze"
		}
		legacy := map[string]any{
			"block":   string(card.Ref.Block),
			"prompt":  "kept prompt",
			"answer":  "kept answer",
			"status":  "new",
			"kind":    kind,
			"created": created,
		}
		if card.Ordinal != 0 {
			legacy["ordinal"] = card.Ordinal
		}
		data, err := json.Marshal(legacy)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cards", string(card.ID)+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	config := map[string]any{"format": "remit/1"}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	// A hand-edit leaves the old config's fsrs block behind; only the format
	// matters to the migration gate.
	if err := os.WriteFile(filepath.Join(dir, "remit.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	return lib, dir, plain
}

// Opening a remit/1 library rewrites it once: card files trade prompt/answer
// and kind for type, fields and edited, remit.json declares remit/2, and the
// cards a rebuild reads are the cards a reopen reads.
func TestOpenMigratesLegacyCardFiles(t *testing.T) {
	_, dir, plain := legacyLibrary(t)

	lib, err := store.Open(dir, store.Options{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("open legacy library: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	// The config now declares the format this build writes.
	configData := read(t, filepath.Join(dir, "remit.json"))
	var config struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatal(err)
	}
	if config.Format != "remit/2" {
		t.Errorf("format = %q, want remit/2", config.Format)
	}

	// The card file on disk is the new shape: no legacy keys, the prompt and
	// answer survive as edited Front/Back fields, the kind named the type.
	cardData := read(t, filepath.Join(dir, "cards", string(plain.ID)+".json"))
	var file map[string]any
	if err := json.Unmarshal(cardData, &file); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"prompt", "answer", "kind"} {
		if _, ok := file[key]; ok {
			t.Errorf("card file still has %q: %s", key, cardData)
		}
	}
	if file["type"] != string(core.TypeBasic) {
		t.Errorf("type = %v, want %q", file["type"], core.TypeBasic)
	}
	fields, _ := file["fields"].(map[string]any)
	if fields["Front"] != "kept prompt" || fields["Back"] != "kept answer" {
		t.Errorf("fields = %v, want the kept prompt and answer", fields)
	}
	if file["edited"] != true {
		t.Errorf("edited = %v, want true", file["edited"])
	}

	// The card reads back as what the file says, and a rebuild of the whole
	// index agrees with the file-by-file read.
	card, ok := lib.Card(plain.ID)
	if !ok || !card.Edited || card.Fields["Front"] != "kept prompt" ||
		card.Type != core.TypeBasic || card.Status != core.StatusNew {
		t.Fatalf("Card = (%+v, %v)", card, ok)
	}
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	rebuilt, ok := lib.Card(plain.ID)
	if !reflect.DeepEqual(rebuilt, card) {
		t.Fatalf("after rebuild Card = (%+v, %v), want (%+v, true)", rebuilt, ok, card)
	}

	// The cloze card kept its ordinal through the migration.
	cloze := lib.Cards()
	found := false
	for _, card := range cloze {
		if card.Type == core.TypeCloze {
			found = true
			if card.Ordinal != 1 {
				t.Errorf("cloze ordinal = %d, want 1", card.Ordinal)
			}
		}
	}
	if !found {
		t.Fatal("the cloze card did not survive the migration")
	}

	// Reopening the migrated library changes nothing: the file is already the
	// shape this build writes.
	before := read(t, filepath.Join(dir, "cards", string(plain.ID)+".json"))
	again, err := store.Open(dir, store.Options{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = again.Close() })
	if after := read(t, filepath.Join(dir, "cards", string(plain.ID)+".json")); string(after) != string(before) {
		t.Errorf("reopen rewrote the migrated card:\n%s\nwant\n%s", after, before)
	}
	reread, ok := again.Card(plain.ID)
	if !ok || !reflect.DeepEqual(reread, card) {
		t.Fatalf("Card after reopen = (%+v, %v)", reread, ok)
	}
}

// A legacy card that asked nothing (empty prompt and answer, no kind) becomes
// a plain unedited basic card.
func TestOpenMigratesEmptyLegacyCard(t *testing.T) {
	lib, dir, plain := legacyLibrary(t)
	_ = lib
	legacy := map[string]any{
		"block":   string(plain.Ref.Block),
		"status":  "new",
		"created": "2026-11-06T09:00:00Z",
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cards", string(plain.ID)+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	opened, err := store.Open(dir, store.Options{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	card, ok := opened.Card(plain.ID)
	if !ok || card.Type != core.TypeBasic || card.Edited || card.Fields != nil {
		t.Fatalf("Card = (%+v, %v), want an unedited basic card", card, ok)
	}
}

// The built-in types are library data: Create writes them, an open reseeds
// whatever is missing, and a type a person wrote by hand reads back.
func TestTypesLiveOnDisk(t *testing.T) {
	lib, dir, _ := newLibrary(t)

	builtins := lib.Types()
	if len(builtins) != 3 {
		t.Fatalf("Types = %v, want the three built-ins", builtins)
	}
	for _, want := range []core.ID{core.TypeBasic, core.TypeBidirectional, core.TypeCloze} {
		if _, ok := lib.Type(want); !ok {
			t.Errorf("Type(%q) not found", want)
		}
		data := read(t, filepath.Join(dir, "types", string(want)+".json"))
		if !strings.Contains(string(data), "\"id\"") || !strings.Contains(string(data), "\"fields\"") {
			t.Errorf("type file %s is not readable type data:\n%s", want, data)
		}
	}
	if _, ok := lib.Type(core.ID("nope")); ok {
		t.Error("Type found an ID the library does not hold")
	}

	// A deleted file comes back on the next open.
	if err := os.Remove(filepath.Join(dir, "types", "remit.cloze.json")); err != nil {
		t.Fatal(err)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = again.Close() })
	if _, ok := again.Type(core.TypeCloze); !ok {
		t.Error("the deleted built-in type was not reseeded")
	}

	// A type written by hand joins the library's own.
	custom := `{
  "id": "mine.vocab",
  "name": "Vocabulary",
  "fields": ["Term", "Definition"],
  "front": "{{Term}}",
  "back": "{{Definition}}",
  "version": 3
}`
	if err := os.WriteFile(filepath.Join(dir, "types", "mine.vocab.json"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("reopen with a hand-written type: %v", err)
	}
	t.Cleanup(func() { _ = third.Close() })
	typ, ok := third.Type("mine.vocab")
	if !ok || typ.Name != "Vocabulary" || len(typ.Fields) != 2 || typ.Version != 3 ||
		typ.Front != "{{Term}}" || typ.Back != "{{Definition}}" {
		t.Fatalf("Type = (%+v, %v), want the hand-written type", typ, ok)
	}
}
