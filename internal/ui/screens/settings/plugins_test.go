package settings_test

import (
	"sort"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/settings"
	"github.com/transmit-bug/remit/locales"
)

var en = locales.In("en")

// world is the app the plugins and types sections talk to: discovery's answer,
// the roles the library plays, the types it holds, and every write it was
// asked for. The refusals a real app gives — a field a card holds, a type in
// use — are answered here the way the store answers them, so the page's
// explanations are the ones the store's errors become.
type world struct {
	plugins []settings.Plugin
	roles   map[string]string
	types   []settings.Type

	enabled    []pluginChange
	roleWrites []roleChange
	saves      []core.Type
	deletes    []core.ID

	// held maps a type to the fields its cards store a non-empty value in.
	held  map[core.ID]map[string]bool
	state *settings.State
}

type pluginChange struct {
	name string
	on   bool
}

type roleChange struct {
	role, name string
}

func newWorld() *world {
	w := &world{
		roles: map[string]string{
			core.RoleCardsDerive:    "cards",
			core.RoleReviewSchedule: "scheduler",
		},
		held: map[core.ID]map[string]bool{},
	}
	w.plugins = []settings.Plugin{
		{Name: "cards", ID: "remit.cards.standard", Version: "1.0.0", Roles: []string{core.RoleCardsDerive}, Builtin: true, Enabled: true},
		{Name: "scheduler", ID: "remit.scheduler.default", Version: "1.0.0", Roles: []string{core.RoleReviewSchedule}, Builtin: true, Enabled: true},
		{Name: "acme.cards", ID: "acme.cards", Version: "0.3.0", Capabilities: []string{"notes:read"}, Roles: []string{core.RoleCardsDerive}},
		{Name: "acme.sched", ID: "acme.sched", Version: "0.2.0", Capabilities: []string{"review:read", "schedule:fsrs"}, Roles: []string{core.RoleReviewSchedule}},
	}
	w.types = []settings.Type{
		{Type: core.Type{ID: core.TypeBasic, Name: "Basic", Fields: []string{"Front", "Back"}, Front: "**{{Front}}!**", Back: "{{Back}}", Version: 1}, Cards: 3},
		{Type: core.Type{ID: "type_vocab", Name: "Vocabulary", Fields: []string{"Term", "Meaning", "Extra"}, Front: "{{Term}}", Back: "{{Meaning}}", Version: 1}, Cards: 1},
	}
	w.held["type_vocab"] = map[string]bool{"Extra": true}
	return w
}

func (w *world) deps() settings.Deps {
	msgs := en
	return settings.Deps{
		Plugins: func() []settings.Plugin { return w.plugins },
		SetEnabled: func(_ *ui.Context, name string, on bool) {
			w.enabled = append(w.enabled, pluginChange{name, on})
			for i := range w.plugins {
				if w.plugins[i].Name == name {
					w.plugins[i].Enabled = on
				}
			}
		},
		Roles: func() map[string]string { return w.roles },
		SetRole: func(_ *ui.Context, role, name string) {
			w.roleWrites = append(w.roleWrites, roleChange{role, name})
			w.roles[role] = name
		},
		Types: func() []settings.Type { return w.types },
		SaveType: func(_ *ui.Context, typ core.Type) core.ID {
			if field, held := w.refusedRemoval(typ); held {
				w.state.TypeErr = msgs.TypeFieldHeld(field, 1)
				return ""
			}
			if typ.ID == "" {
				typ.ID = core.NewID("type")
			}
			if typ.Version == 0 {
				typ.Version = 1
			}
			w.saves = append(w.saves, typ)
			w.upsert(typ)
			return typ.ID
		},
		DeleteType: func(_ *ui.Context, id core.ID) {
			if w.builtin(id) {
				w.state.TypeErr = msgs.TypeBuiltinDelete()
				return
			}
			for _, typ := range w.types {
				if typ.ID != id {
					continue
				}
				if typ.Cards > 0 {
					w.state.TypeErr = msgs.TypeInUse(typ.Cards)
					return
				}
			}
			w.deletes = append(w.deletes, id)
			w.remove(id)
		},
	}
}

// refusedRemoval stands in for the store's guard: a field the type no longer
// has, which a card still holds a value in.
func (w *world) refusedRemoval(next core.Type) (string, bool) {
	old, ok := w.typeOf(next.ID)
	if !ok {
		return "", false
	}
	kept := make(map[string]bool, len(next.Fields))
	for _, field := range next.Fields {
		kept[field] = true
	}
	for _, field := range old.Fields {
		if !kept[field] && w.held[next.ID][field] {
			return field, true
		}
	}
	return "", false
}

func (w *world) typeOf(id core.ID) (settings.Type, bool) {
	for _, typ := range w.types {
		if typ.ID == id {
			return typ, true
		}
	}
	return settings.Type{}, false
}

func (w *world) upsert(typ core.Type) {
	for i := range w.types {
		if w.types[i].ID == typ.ID {
			w.types[i].Type = typ
			sort.Slice(w.types, func(i, j int) bool { return w.types[i].ID < w.types[j].ID })
			return
		}
	}
	w.types = append(w.types, settings.Type{Type: typ})
	sort.Slice(w.types, func(i, j int) bool { return w.types[i].ID < w.types[j].ID })
}

func (w *world) remove(id core.ID) {
	kept := make([]settings.Type, 0, len(w.types))
	for _, typ := range w.types {
		if typ.ID != id {
			kept = append(kept, typ)
		}
	}
	w.types = kept
}

func (w *world) builtin(id core.ID) bool {
	for _, typ := range core.BuiltinTypes() {
		if typ.ID == id {
			return true
		}
	}
	return false
}

// atWorld draws the page over the world's own state, so a fake app's refusal,
// written into that state, is what the page reads next frame.
func atWorld(t *testing.T, w *world, width, height int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { settings.View(c, w.state, w.deps()) }, width, height)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// approve approves one plugin through the page's switch and returns the call
// the app recorded.
func approve(t *testing.T, tt *ui.Tester, w *world, name string) pluginChange {
	t.Helper()
	if err := tt.Click(name); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(w.enabled) != 1 {
		t.Fatalf("the switch reported %v, want one approval of %q", w.enabled, name)
	}
	return w.enabled[0]
}

// chooseRole clicks a role's label and moves the radio group one option on,
// which is how a keyboard user changes it.
func chooseRole(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
}

// A plugin is listed with the capabilities approving it grants, and approving
// it is the switch. The approval is the write: the page asks the app once, with
// the plugin's name and its new state.
func TestPluginApproval(t *testing.T) {
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			w := newWorld()
			w.state = &settings.State{HasLibrary: true}
			tt := atWorld(t, w, size.w, size.h)

			for _, want := range []string{
				en.SettingsPlugins(), en.SettingsPluginsBody(),
				"cards", "scheduler", "acme.cards", "acme.sched",
				en.PluginBuiltin(), "notes:read", "schedule:fsrs",
				en.PluginRoleCards(), en.PluginRoleCardsBody(),
				en.PluginRoleSchedule(), en.PluginRoleScheduleBody(),
			} {
				reveal(t, tt, size.w, size.h, want)
				if !tt.HasText(want) {
					t.Fatalf("the page is missing %q; texts %q", want, tt.Texts())
				}
			}

			reveal(t, tt, size.w, size.h, en.PluginEnable("acme.cards"))
			got := approve(t, tt, w, en.PluginEnable("acme.cards"))
			if got != (pluginChange{"acme.cards", true}) {
				t.Fatalf("the approval reached the app as %+v, want acme.cards on", got)
			}
			if !w.plugins[2].Enabled {
				t.Error("the page's list did not follow the approval")
			}
		})
	}
}

// A role is a radio group over the approved plugins that can play it. Moving
// the choice asks the app to write it, and the choice is the plugin the library
// then names.
func TestRoleSwitch(t *testing.T) {
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			w := newWorld()
			w.plugins[2].Enabled = true // acme.cards is approved
			w.state = &settings.State{HasLibrary: true}
			tt := atWorld(t, w, size.w, size.h)

			reveal(t, tt, size.w, size.h, "acme.cards 0.3.0")
			chooseRole(t, tt, "cards 1.0.0")
			if len(w.roleWrites) != 1 {
				t.Fatalf("the role changed %v, want one write", w.roleWrites)
			}
			want := roleChange{core.RoleCardsDerive, "acme.cards"}
			if w.roleWrites[0] != want {
				t.Fatalf("the role reached the app as %+v, want %+v", w.roleWrites[0], want)
			}
		})
	}
}

// A role a plugin is not approved for is not offered: a role chooses among what
// the user permitted, and the built-in is always there.
func TestRoleOffersOnlyApprovedPlugins(t *testing.T) {
	w := newWorld()
	w.state = &settings.State{HasLibrary: true}
	tt := atWorld(t, w, 1024, 720)

	reveal(t, tt, 1024, 720, "cards 1.0.0")
	// One Down from the built-in is the only option: unapproved acme.cards is
	// not in the group, so the choice stays put.
	chooseRole(t, tt, "cards 1.0.0")
	if len(w.roleWrites) != 0 {
		t.Fatalf("the role changed to %v, want no change", w.roleWrites)
	}
}

// Without a plugin directory the page states that, and without a library it
// says which library to open: neither is a panic, both keep the page usable.
func TestPluginsDegradeToAStatedEmptyState(t *testing.T) {
	msgs := en
	empty := settings.Deps{}
	state := settings.State{HasLibrary: true}
	tt := at(t, state, empty, 1024, 720)
	reveal(t, tt, 1024, 720, msgs.PluginEmpty())
	if !tt.HasText(msgs.PluginEmpty()) {
		t.Fatalf("no plugin service says nothing: %q", tt.Texts())
	}

	closed := at(t, settings.State{}, empty, 1024, 720)
	reveal(t, closed, 1024, 720, msgs.SettingsPluginsNeedsLibrary())
	if !closed.HasText(msgs.SettingsPluginsNeedsLibrary()) {
		t.Error("the plugins section did not ask for a library")
	}
	reveal(t, closed, 1024, 720, msgs.SettingsTypesNeedsLibrary())
	if !closed.HasText(msgs.SettingsTypesNeedsLibrary()) {
		t.Error("the types section did not ask for a library")
	}
}
