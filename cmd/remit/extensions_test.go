package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// The plugin directory is cmd's: discovery lists the plugins the app ships
// first, then what the user installed, each with the roles its manifest
// implements, and a role's name starts the runner it names.
func TestPluginServiceDiscoversAndOpens(t *testing.T) {
	s := newPluginService(filepath.Join("..", "..", "examples", "plugins"))
	found, err := s.Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(found) < 3 {
		t.Fatalf("discovered %d plugins, want the two built-ins and the example", len(found))
	}
	if found[0].Name != "cards" || !found[0].Builtin || !slices.Contains(found[0].Roles, core.RoleCardsDerive) {
		t.Errorf("the first plugin is %+v, want the built-in cards with its role", found[0])
	}
	if found[1].Name != "scheduler" || !found[1].Builtin || !slices.Contains(found[1].Roles, core.RoleReviewSchedule) {
		t.Errorf("the second plugin is %+v, want the built-in scheduler with its role", found[1])
	}

	var example *appui.Plugin
	for i := range found {
		if found[i].Name == "cards-mini" {
			example = &found[i]
		}
	}
	if example == nil {
		t.Fatalf("the example plugin is not discovered: %+v", found)
	}
	if example.Builtin || !slices.Contains(example.Roles, core.RoleCardsDerive) {
		t.Errorf("the example is %+v, want a user plugin that derives", example)
	}
	if example.Version != "0.1.0" || example.ID != "example.cards.mini" {
		t.Errorf("the example's manifest reads as %+v", example)
	}

	runner, err := s.Open("cards-mini")
	if err != nil {
		t.Fatalf("Open(cards-mini): %v", err)
	}
	if runner.Name() != "cards-mini" {
		t.Errorf("Open answered the runner named %q, want cards-mini", runner.Name())
	}
	if again, _ := s.Open("cards-mini"); again != runner {
		t.Error("the service started the same plugin twice")
	}
	if _, err := s.Open("ghost"); err == nil {
		t.Error("Open(ghost) answered a runner for a plugin that is not installed")
	}
}

// A role whose plugin is the app's own opens the runner the app ships, by
// name, so the role path is the same as any other.
func TestPluginServiceOpensTheBuiltins(t *testing.T) {
	s := newPluginService(t.TempDir())
	for _, name := range []string{"cards", "scheduler"} {
		runner, err := s.Open(name)
		if err != nil {
			t.Fatalf("Open(%s): %v", name, err)
		}
		if runner.Name() != name {
			t.Errorf("Open(%s) answered %q", name, runner.Name())
		}
	}
}

// A contributed page reads the library through the capability it declared: the
// runner the window is handed is opened with the open library's host, not
// without one. Without it every remit.notes.* call fails and the page shows an
// empty library, which is the bug a screen test with its own host cannot see.
func TestPluginServiceGivesAContributedPageTheLibrary(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	if _, err := lib.SaveNote(core.Note{
		Title:  "Leech farm",
		Blocks: []core.Block{{Text: "too big :: an answer"}},
	}); err != nil {
		t.Fatalf("save note: %v", err)
	}

	s := newPluginService(filepath.Join("..", "..", "examples", "plugins"))
	s.SetLibrary(lib)
	runner, err := s.Open("leeches")
	if err != nil {
		t.Fatalf("Open(leeches): %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	var page json.RawMessage
	if err := runner.Call(ext.CommandHook, map[string]any{"name": "leechesData", "context": map[string]any{}}, &page); err != nil {
		t.Fatalf("the page's data command: %v", err)
	}
	if !strings.Contains(string(page), "Leech farm") {
		t.Fatalf("the page did not read the library: %s", page)
	}
}
