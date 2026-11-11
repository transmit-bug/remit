package contrib

import "testing"

// The table is the one list: ContextKeys is its keys in its order, and every
// row says the kind of value it answers, the contract's type for it and the
// reader that answers it. A row missing a reader is a key a when expression
// reads as false and a contract that types a value nobody computes.
func TestContextTableIsWhole(t *testing.T) {
	if len(ContextKeys) != len(ContextTable) {
		t.Fatalf("%d context keys, %d table rows", len(ContextKeys), len(ContextTable))
	}
	seen := map[string]bool{}
	for i, entry := range ContextTable {
		if seen[entry.Key] {
			t.Errorf("context key %q is in the table twice", entry.Key)
		}
		seen[entry.Key] = true
		if entry.Key != ContextKeys[i] {
			t.Errorf("key %d of the list is %q, the table's is %q", i, ContextKeys[i], entry.Key)
		}
		if entry.Read == nil {
			t.Errorf("context key %q has no reader", entry.Key)
		}
		switch entry.Kind {
		case ContextBool, ContextString:
			if len(entry.Values) != 0 {
				t.Errorf("context key %q is a %s and names values %v", entry.Key, entry.Kind, entry.Values)
			}
		case ContextEnum:
			if len(entry.Values) == 0 {
				t.Errorf("context key %q is an enum and names no values", entry.Key)
			}
			if entry.Type == "boolean" || entry.Type == "string" {
				t.Errorf("context key %q is an enum and the contract types it %q", entry.Key, entry.Type)
			}
		default:
			t.Errorf("context key %q has kind %q, which is none of the three", entry.Key, entry.Kind)
		}
		if entry.Type == "" {
			t.Errorf("context key %q has no type in the contract", entry.Key)
		}
	}
	for key := range contextReads {
		if !seen[key] {
			t.Errorf("a reader is wired to %q, which the table does not name", key)
		}
	}
}

// A key answers the field it names, so a reader wired to the wrong field shows
// the wrong contribution.
func TestContextKeysReadTheirField(t *testing.T) {
	ctx := Context{
		Mode: "review", Area: "recall", Sub: "queue", LibraryOpen: true, NoteOpen: true,
		NoteSelection: true, CardPresent: true, CardStatus: "review",
		BrowseSelection: true, ScopeActive: true, PageID: "acme.leeches", PluginID: "acme",
	}
	for _, key := range ContextKeys {
		value, ok := ctx.Value(key)
		if !ok {
			t.Fatalf("key %q does not resolve", key)
		}
		if value == "" || value == false {
			t.Errorf("key %q answered %#v on a context that sets it", key, value)
		}
	}
}

// Every key's value is of the kind the table claims and the contract types it
// as: a key typed boolean that answers a string is a plugin author's type error.
func TestContextKeysAnswerTheirKind(t *testing.T) {
	ctx := Context{
		Mode: "review", Area: "recall", Sub: "queue", LibraryOpen: true, NoteOpen: true,
		NoteSelection: true, CardPresent: true, CardStatus: "review",
		BrowseSelection: true, ScopeActive: true, PageID: "acme.leeches", PluginID: "acme",
	}
	for _, entry := range ContextTable {
		value, _ := ctx.Value(entry.Key)
		switch entry.Kind {
		case ContextBool:
			if _, ok := value.(bool); !ok {
				t.Errorf("key %q is a bool and answered %#v", entry.Key, value)
			}
		case ContextString, ContextEnum:
			if _, ok := value.(string); !ok {
				t.Errorf("key %q is a %s and answered %#v", entry.Key, entry.Kind, value)
			}
		}
	}
}

// One table, two sides: every key a when expression reads is in the payload a
// callback reads, with the value the same reader answers — on a context that
// sets everything and on one that sets nothing. A command that appeared for a
// selection could otherwise be called with no selection in it.
func TestPayloadCarriesEveryContextKey(t *testing.T) {
	full := Context{
		Mode: "review", Area: "recall", Sub: "queue", LibraryOpen: true, NoteOpen: true,
		NoteSelection: true, CardPresent: true, CardStatus: "review",
		BrowseSelection: true, ScopeActive: true, PageID: "acme.leeches", PluginID: "acme",
	}
	for _, tc := range []struct {
		name string
		ctx  Context
	}{{"a window that sets every key", full}, {"a window that sets none", Context{}}} {
		t.Run(tc.name, func(t *testing.T) {
			payload := tc.ctx.Payload()
			if len(payload) != len(ContextKeys) {
				t.Errorf("payload has %d keys, the table names %d: %v", len(payload), len(ContextKeys), payload)
			}
			for _, key := range ContextKeys {
				want, ok := tc.ctx.Value(key)
				if !ok {
					t.Fatalf("key %q does not resolve", key)
				}
				got, present := payload[key]
				if !present {
					t.Errorf("the payload has no %q; a when expression reads it", key)
					continue
				}
				if got != want {
					t.Errorf("%q is %#v in the payload and %#v to when", key, got, want)
				}
			}
		})
	}
}
