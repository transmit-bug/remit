package main

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
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
