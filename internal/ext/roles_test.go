package ext_test

import (
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
)

// extFor names an extension by what it implements, which is all a role reads.
func extFor(name string, implements ...string) ext.Extension {
	hooks := make(map[string]int, len(implements))
	for _, hook := range implements {
		hooks[hook] = 1
	}
	return ext.Extension{Manifest: ext.Manifest{Name: name, Implements: hooks}}
}

// A role is played by a plugin whose manifest implements the hook it asks for,
// with no plugin loaded to find out.
func TestForRole(t *testing.T) {
	cards := extFor("acme.cards", "describe", "deriveBlock")
	scheduler := extFor("acme.scheduler", "policy", "summary")
	both := extFor("acme.both", "deriveBlock", "policy")
	exts := []ext.Extension{scheduler, both, cards}

	cases := []struct {
		name string
		role string
		want string
		ok   bool
	}{
		{"card derivation picks the first that derives", core.RoleCardsDerive, "acme.both", true},
		{"scheduling picks the first that sets a policy", core.RoleReviewSchedule, "acme.scheduler", true},
		{"a role nobody plays", "cards.render", "", false},
		{"a role the host does not know", "review.summary", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ext.ForRole(exts, tc.role)
			if ok != tc.ok || got.Manifest.Name != tc.want {
				t.Fatalf("ForRole(%q) = (%q, %v), want (%q, %v)", tc.role, got.Manifest.Name, ok, tc.want, tc.ok)
			}
		})
	}
}

// An empty implements map plays nothing, and Plays answers the role's own hook
// rather than any hook the plugin carries.
func TestPlays(t *testing.T) {
	bare := ext.Extension{Manifest: ext.Manifest{Name: "bare"}}
	if ext.Plays(bare, core.RoleCardsDerive) {
		t.Error("a manifest with no hooks plays card derivation")
	}
	renderer := extFor("acme.render", "describe", "on_event")
	if ext.Plays(renderer, core.RoleCardsDerive) || ext.Plays(renderer, core.RoleReviewSchedule) {
		t.Error("a manifest without deriveBlock or policy plays a role")
	}
	if !ext.Plays(extFor("acme.cards", "deriveBlock"), core.RoleCardsDerive) {
		t.Error("deriveBlock does not play cards.derive")
	}
	if !ext.Plays(extFor("acme.scheduler", "policy"), core.RoleReviewSchedule) {
		t.Error("policy does not play review.schedule")
	}
}
