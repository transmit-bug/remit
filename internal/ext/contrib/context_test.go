package contrib

import "testing"

// The key list is the contract artifact's source and the parser's table at once:
// a key in one and not the other is a key the docs promise and the host refuses,
// or the reverse.
func TestContextKeysMatchTheTable(t *testing.T) {
	if len(ContextKeys) != len(contextReads) {
		t.Fatalf("%d context keys, %d readers", len(ContextKeys), len(contextReads))
	}
	for _, key := range ContextKeys {
		if _, ok := contextReads[key]; !ok {
			t.Errorf("context key %q has no reader", key)
		}
	}
	for key := range contextReads {
		if _, warnings, err := Compile(key); err != nil || len(warnings) > 0 {
			t.Errorf("context key %q does not compile clean: %v %v", key, err, warnings)
		}
	}
}

// A key answers the field it names, so a reader wired to the wrong field shows
// the wrong contribution.
func TestContextKeysReadTheirField(t *testing.T) {
	ctx := Context{
		Mode: "review", LibraryOpen: true, NoteOpen: true, NoteSelection: true,
		CardPresent: true, CardStatus: "review", BrowseSelection: true,
		ScopeActive: true, PageID: "acme.leeches", PluginID: "acme",
	}
	for _, key := range ContextKeys {
		value, ok := ctx.value(key)
		if !ok {
			t.Fatalf("key %q does not resolve", key)
		}
		if value == "" || value == false {
			t.Errorf("key %q answered %#v on a context that sets it", key, value)
		}
	}
}
