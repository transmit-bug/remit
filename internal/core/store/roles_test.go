package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// remitJSON reads the library's declaration as the file holds it, for the
// assertions about what a role write left behind.
func remitJSON(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "remit.json"))
	if err != nil {
		t.Fatalf("read remit.json: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("remit.json: %v", err)
	}
	return config
}

// A new library plays every role with the plugin the app ships, and says so
// without writing a roles field: an unset role is the built-in.
func TestRolesDefaultToTheBuiltins(t *testing.T) {
	lib, dir, _ := newLibrary(t)

	roles := lib.Roles()
	if got, want := roles[core.RoleCardsDerive], store.DefaultCardsPlugin; got != want {
		t.Errorf("cards.derive = %q, want %q", got, want)
	}
	if got, want := roles[core.RoleReviewSchedule], store.DefaultSchedulerPlugin; got != want {
		t.Errorf("review.schedule = %q, want %q", got, want)
	}
	if _, ok := remitJSON(t, dir)["roles"]; ok {
		t.Error("a fresh remit.json carries a roles field, want it left out")
	}
	if got := store.ReadRoles(dir); got[core.RoleCardsDerive] != store.DefaultCardsPlugin {
		t.Errorf("ReadRoles before any choice = %v, want the built-in", got)
	}
}

// A role write round-trips through remit.json, survives a reopen, and answers
// the defaults when the role is taken back out.
func TestSetRoleRoundTrips(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	if err := lib.SetEnabledExtensions([]string{"acme.cards"}); err != nil {
		t.Fatalf("SetEnabledExtensions: %v", err)
	}
	if err := lib.SetRole(core.RoleCardsDerive, "acme.cards"); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	if got := lib.Roles()[core.RoleCardsDerive]; got != "acme.cards" {
		t.Fatalf("cards.derive = %q, want acme.cards", got)
	}
	if got := store.ReadRoles(dir)[core.RoleCardsDerive]; got != "acme.cards" {
		t.Errorf("ReadRoles = %q, want what the file says", got)
	}

	lib = reopen(t, lib, dir, clk)
	if got := lib.Roles()[core.RoleCardsDerive]; got != "acme.cards" {
		t.Errorf("after reopen cards.derive = %q, want acme.cards", got)
	}

	if err := lib.SetRole(core.RoleCardsDerive, ""); err != nil {
		t.Fatalf("SetRole back to the built-in: %v", err)
	}
	if got := lib.Roles()[core.RoleCardsDerive]; got != store.DefaultCardsPlugin {
		t.Errorf("cards.derive = %q, want the built-in after clearing", got)
	}
	if roles, ok := remitJSON(t, dir)["roles"].(map[string]any); ok && len(roles) != 0 {
		t.Errorf("remit.json still names a role: %v", roles)
	}
}

// A role guard names what it refuses: a role the host does not have, and a
// plugin the library never approved.
func TestSetRoleGuards(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	if err := lib.SetEnabledExtensions([]string{"acme.cards"}); err != nil {
		t.Fatalf("SetEnabledExtensions: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "remit.json"))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		role string
		who  string
		want error
	}{
		{"unknown role", "cards.render", "acme.cards", store.ErrUnknownRole},
		{"plugin not enabled", core.RoleCardsDerive, "acme.other", store.ErrPluginNotEnabled},
		{"plugin not installed", core.RoleReviewSchedule, "acme.scheduler", store.ErrPluginNotEnabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := lib.SetRole(tc.role, tc.who); !errors.Is(err, tc.want) {
				t.Fatalf("SetRole(%q, %q) = %v, want %v", tc.role, tc.who, err, tc.want)
			}
		})
	}
	after, err := os.ReadFile(filepath.Join(dir, "remit.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("a refused role write changed remit.json:\n%s\nwant\n%s", after, before)
	}
}

// Withdrawing a plugin takes it out of the role it played, so remit.json never
// names a plugin the library does not run.
func TestDisablingAPluginClearsItsRole(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	if err := lib.SetEnabledExtensions([]string{"acme.cards"}); err != nil {
		t.Fatalf("SetEnabledExtensions: %v", err)
	}
	if err := lib.SetRole(core.RoleCardsDerive, "acme.cards"); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	if err := lib.SetEnabledExtensions(nil); err != nil {
		t.Fatalf("SetEnabledExtensions(nil): %v", err)
	}
	if got := lib.Roles()[core.RoleCardsDerive]; got != store.DefaultCardsPlugin {
		t.Errorf("cards.derive = %q, want the built-in after disabling", got)
	}
	if roles, ok := remitJSON(t, dir)["roles"].(map[string]any); ok && roles["cards.derive"] != nil {
		t.Errorf("remit.json names a disabled plugin: %v", roles)
	}
}

// A hand-edited file naming a plugin the library does not enable reads as the
// built-in: a dangling role is a default, never a plugin nothing can start.
func TestRolesFallBackWhenTheFileNamesADisabledPlugin(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	config := remitJSON(t, dir)
	config["roles"] = map[string]any{core.RoleCardsDerive: "ghost.cards"}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "remit.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := store.ReadRoles(dir)[core.RoleCardsDerive]; got != store.DefaultCardsPlugin {
		t.Errorf("ReadRoles = %q, want the built-in for a plugin that is not enabled", got)
	}

	lib = reopen(t, lib, dir, clk)
	if got := lib.Roles()[core.RoleCardsDerive]; got != store.DefaultCardsPlugin {
		t.Errorf("cards.derive = %q, want the built-in for a plugin that is not enabled", got)
	}
}
