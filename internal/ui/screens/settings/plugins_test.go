package settings_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/settings"
	"github.com/transmit-bug/remit/locales"
)

var en = locales.In("en")

// world is the app the plugins section talks to: discovery's answer, the roles
// the library plays, and every write it was asked for.
type world struct {
	plugins []settings.Plugin
	roles   map[string]string

	enabled    []pluginChange
	roleWrites []roleChange
	reenabled  []string

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
	}
	w.plugins = []settings.Plugin{
		{Name: "cards", ID: "remit.cards.standard", Version: "1.0.0", Roles: []string{core.RoleCardsDerive}, Builtin: true, Enabled: true},
		{Name: "scheduler", ID: "remit.scheduler.default", Version: "1.0.0", Roles: []string{core.RoleReviewSchedule}, Builtin: true, Enabled: true},
		{Name: "acme.cards", ID: "acme.cards", Version: "0.3.0", Capabilities: []string{"notes:read"}, Roles: []string{core.RoleCardsDerive}},
		{Name: "acme.sched", ID: "acme.sched", Version: "0.2.0", Capabilities: []string{"review:read", "schedule:fsrs"}, Roles: []string{core.RoleReviewSchedule}},
	}
	return w
}

func (w *world) deps() settings.Deps {
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
		Reenable: func(_ *ui.Context, name string) {
			w.reenabled = append(w.reenabled, name)
			for i := range w.plugins {
				if w.plugins[i].Name == name {
					w.plugins[i].Failures = 0
					w.plugins[i].Problems = nil
				}
			}
		},
		Roles: func() map[string]string { return w.roles },
		SetRole: func(_ *ui.Context, role, name string) {
			w.roleWrites = append(w.roleWrites, roleChange{role, name})
			w.roles[role] = name
		},
	}
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
			w.state = &settings.State{HasLibrary: true, Page: settings.PagePlugins}
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

// A plugin's row is what approving it grants and what the host had to drop: the
// capabilities, the contributions it could not use, the entries failures hid,
// and the one action that gives them another try.
func TestPluginsShowCapabilitiesAndFailures(t *testing.T) {
	msgs := en
	w := newWorld()
	w.plugins[2].Enabled = true
	w.plugins[2].Capabilities = []string{"notes:read", "cards:write"}
	w.plugins[2].Problems = []string{`icon "no-such-icon": the host has no such icon`}
	w.plugins[2].Failures = 3
	w.state = &settings.State{HasLibrary: true, Page: settings.PagePlugins}
	tt := atWorld(t, w, 1024, 720)

	for _, want := range []string{
		"acme.cards", "notes:read", "cards:write",
		msgs.PluginProblems(), `icon "no-such-icon": the host has no such icon`,
		msgs.PluginFailures(3), msgs.PluginReenable(),
	} {
		reveal(t, tt, 1024, 720, want)
		if !tt.HasText(want) {
			t.Fatalf("the plugin's row is missing %q; texts %q", want, tt.Texts())
		}
	}
	// The built-ins carry a badge instead of a switch: withdrawing the app's own
	// card plugin is not a choice the page offers.
	reveal(t, tt, 1024, 720, msgs.PluginBuiltin())

	reveal(t, tt, 1024, 720, "Re-enable")
	if err := tt.Click("Re-enable"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(w.reenabled) != 1 || w.reenabled[0] != "acme.cards" {
		t.Fatalf("Re-enable reached the app as %v, want acme.cards", w.reenabled)
	}
	if tt.HasText(msgs.PluginFailures(3)) {
		t.Error("the failure count stayed after the app cleared it")
	}
}

// One plugin's details are opened by its own button, and say what the row has no
// room for: the manifest id, its version and the roles it can play.
func TestPluginDetailsOpenOnTheirButton(t *testing.T) {
	msgs := en
	w := newWorld()
	w.state = &settings.State{HasLibrary: true, Page: settings.PagePlugins}
	tt := atWorld(t, w, 1024, 720)

	reveal(t, tt, 1024, 720, msgs.SettingsPluginDetails())
	if tt.HasText("remit.cards.standard") {
		t.Error("the details show before their button was used")
	}
	info, ok := tt.AccessByRole(ui.RoleButton, msgs.SettingsPluginDetailsOf("acme.cards"))
	if !ok {
		t.Fatalf("the page draws no details button for acme.cards; texts %q", tt.Texts())
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	tt.Frame()
	reveal(t, tt, 1024, 720, "acme.cards")
	if !tt.HasText("acme.cards · 0.3.0") {
		t.Fatalf("the details did not open; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.SettingsPluginRoles()) {
		t.Errorf("the details do not name the roles; texts %q", tt.Texts())
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
			w.state = &settings.State{HasLibrary: true, Page: settings.PagePlugins}
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
	w.state = &settings.State{HasLibrary: true, Page: settings.PagePlugins}
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
	state := settings.State{HasLibrary: true, Page: settings.PagePlugins}
	tt := ui.NewTester(func(c *ui.Context) { settings.View(c, &state, empty) }, 1024, 720)
	tt.Frame()
	reveal(t, tt, 1024, 720, msgs.PluginEmpty())
	if !tt.HasText(msgs.PluginEmpty()) {
		t.Fatalf("no plugin service says nothing: %q", tt.Texts())
	}

	closed := settings.State{Page: settings.PagePlugins}
	tt = ui.NewTester(func(c *ui.Context) { settings.View(c, &closed, empty) }, 1024, 720)
	tt.Frame()
	reveal(t, tt, 1024, 720, msgs.SettingsPluginsNeedsLibrary())
	if !tt.HasText(msgs.SettingsPluginsNeedsLibrary()) {
		t.Error("the plugins page did not ask for a library")
	}
}

// The S47 sheet is the plugin's own: what approving it grants, how far it is
// trusted, the switch that grants it, the entries failures took out with the way
// back, and the pages it contributes. It reads the list's own data, so the row and
// the sheet cannot disagree.
func TestPluginDetailShowsCapabilitiesAndFailures(t *testing.T) {
	msgs := en
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			w := newWorld()
			w.plugins[2].Enabled = true
			w.plugins[2].Capabilities = []string{"notes:read", "cards:write"}
			w.plugins[2].Failures = 3
			w.plugins[2].Problems = []string{"status.item: the slot is reserved"}
			state := &settings.State{
				HasLibrary: true, Page: settings.PagePlugins,
				Pages: []settings.PluginPage{
					{Plugin: "acme.cards", ID: "report", Title: "Card report"},
					{Plugin: "acme.sched", ID: "sched", Title: "Schedule"},
				},
			}
			tt := ui.NewTester(func(c *ui.Context) {
				settings.View(c, state, w.deps())
			}, size.w, size.h)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)

			// One switch before the sheet: the row's own grant.
			if got := countText(tt, msgs.PluginEnable("acme.cards")); got != 1 {
				t.Fatalf("the row draws %d switches for acme.cards, want 1", got)
			}
			reveal(t, tt, size.w, size.h, msgs.SettingsPluginDetails())
			info, ok := tt.AccessByRole(ui.RoleButton, msgs.SettingsPluginDetailsOf("acme.cards"))
			if !ok {
				t.Fatalf("no details button for acme.cards; texts %q", tt.Texts())
			}
			tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
			tt.Frame()

			for _, want := range []string{
				msgs.SettingsPluginTrust(), msgs.SettingsPluginTrustThirdParty(),
				msgs.SettingsPluginTrustThirdPartyBody(),
				msgs.SettingsPluginCapabilities(), "notes:read", "cards:write",
				"acme.cards · 0.3.0", msgs.PluginFailures(3), msgs.PluginReenable(),
				msgs.SettingsPluginPages() + ": Card report",
			} {
				if !tt.HasText(want) {
					t.Fatalf("the sheet is missing %q; texts %q", want, tt.Texts())
				}
			}
			// Its pages are the plugin's own, and its switch is the same grant the
			// row carries: the sheet draws a second one, and nothing of another
			// plugin's.
			if tt.HasText("Card report, Schedule") {
				t.Errorf("the sheet lists another plugin's page; texts %q", tt.Texts())
			}
			if got := countText(tt, msgs.PluginEnable("acme.cards")); got != 2 {
				t.Errorf("the sheet draws %d switches for acme.cards, want its own beside the row's", got-1)
			}
			if tt.FramePending() {
				t.Error("the sheet still asks for frames after it settled")
			}

			// Escape closes it, and the page is where it was.
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			if tt.HasText(msgs.SettingsPluginTrust()) {
				t.Fatalf("Escape left the sheet open; texts %q", tt.Texts())
			}
		})
	}
}

// countText is how many elements of the frame show s, which tells a control the
// page draws once from one it draws twice.
func countText(tt *ui.Tester, s string) int {
	n := 0
	for _, shown := range tt.Texts() {
		if shown == s {
			n++
		}
	}
	return n
}

// A built-in is the app's own: its sheet states that trust level and offers no
// switch, because withdrawing what the build ships is not a choice.
func TestPluginDetailOfABuiltinOffersNoSwitch(t *testing.T) {
	w := newWorld()
	state := &settings.State{HasLibrary: true, Page: settings.PagePlugins}
	tt := ui.NewTester(func(c *ui.Context) {
		settings.View(c, state, w.deps())
	}, 1024, 720)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)

	info, ok := tt.AccessByRole(ui.RoleButton, en.SettingsPluginDetailsOf("cards"))
	if !ok {
		t.Fatalf("no details button for the built-in; texts %q", tt.Texts())
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	tt.Frame()

	if !tt.HasText(en.SettingsPluginTrustBuiltin()) || !tt.HasText(en.SettingsPluginTrustBuiltinBody()) {
		t.Fatalf("the sheet does not state the built-in trust level; texts %q", tt.Texts())
	}
	if tt.HasText(en.PluginEnable("cards")) {
		t.Error("the built-in's sheet offers a switch that would change nothing")
	}
	// Escape closes it, and the page is where it was.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText(en.SettingsPluginTrustBuiltinBody()) {
		t.Fatalf("Escape left the sheet open; texts %q", tt.Texts())
	}
	if !tt.HasText(en.SettingsPlugins()) {
		t.Error("the plugins page did not come back")
	}
}
