package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core/store"
)

func TestPluginDataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, ok := lib.PluginData("remit.cards.standard"); ok {
		t.Error("a plugin that stored nothing has data")
	}
	if err := lib.SetPluginData("remit.cards.standard", `{"streak":3}`); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, ok := lib.PluginData("remit.cards.standard")
	if !ok || got != `{"streak":3}` {
		t.Fatalf("get = %q, %v", got, ok)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reopened.Close()
	got, ok = reopened.PluginData("remit.cards.standard")
	if !ok || got != `{"streak":3}` {
		t.Fatalf("after reopen get = %q, %v", got, ok)
	}
}

// The file a plugin's data lives in is a JSON object a person can read, at the
// path the spec names — asserted against the disk, not the API.
func TestPluginDataLiesOnDiskAsPlaintext(t *testing.T) {
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()
	if err := lib.SetPluginData("remit.scheduler.default", `{"seen":[1,2]}`); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "plugins-data", "remit.scheduler.default.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), `"value"`) || !strings.Contains(string(raw), `[1,2]`) {
		t.Errorf("the file on disk is not readable plugin data: %s", raw)
	}
}

// A plugin id names a file inside the library, so anything that could walk out
// of plugins-data is refused.
func TestPluginDataRefusesBadIDs(t *testing.T) {
	lib, err := store.Create(t.TempDir(), store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()
	for _, id := range []string{"", "..", "../remit", "a/b", `a\b`, "a..b"} {
		if err := lib.SetPluginData(id, "x"); err == nil {
			t.Errorf("id %q was accepted", id)
		}
		if _, ok := lib.PluginData(id); ok {
			t.Errorf("id %q read back data", id)
		}
	}
}

// A value the file already holds is not rewritten: the write that changed
// nothing leaves the bytes alone.
func TestPluginDataSkipsUnchangedWrites(t *testing.T) {
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()
	if err := lib.SetPluginData("remit.cards.standard", "same"); err != nil {
		t.Fatalf("set: %v", err)
	}
	// Replace the file's bytes by hand; an unchanged SetPluginData must not
	// put the original formatting back.
	path := filepath.Join(dir, "plugins-data", "remit.cards.standard.json")
	// The same value, written compactly by hand: different bytes, same meaning.
	if err := os.WriteFile(path, []byte(`{"value":"same"}`), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	if err := lib.SetPluginData("remit.cards.standard", "same"); err != nil {
		t.Fatalf("set again: %v", err)
	}
	planted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(planted) != `{"value":"same"}` {
		t.Error("an unchanged value rewrote the file")
	}
	if err := lib.SetPluginData("remit.cards.standard", "different"); err != nil {
		t.Fatalf("set changed: %v", err)
	}
	if got, ok := lib.PluginData("remit.cards.standard"); !ok || got != "different" {
		t.Fatalf("get = %q, %v", got, ok)
	}
}
