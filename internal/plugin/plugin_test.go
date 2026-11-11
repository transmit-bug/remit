package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
)

// fakeHost is a library as a plugin reaches one. The supervisor only hands it
// out — a capability call is the runner's, not the provider's — so the embedded
// interface is nil and no method here is called.
type fakeHost struct {
	ext.Host
	name string
}

// fakePlugins is a supervisor over fake runners: what was built, warmed and
// closed, by manifest name, without an engine or a plugin directory.
type fakePlugins struct {
	sup    *Supervisor
	built  map[string]int
	warmed map[string]int
	closed map[string]int
	// fails names the plugins whose construction reports a load error.
	fails map[string]bool
}

// newFake is a supervisor over a fake directory holding the plugins named, which
// is what the js-only rule reads: a role naming a plugin that is not there falls
// back, so a test that resolves one installs it first.
func newFake(t *testing.T, installed ...string) *fakePlugins {
	t.Helper()
	root := t.TempDir()
	for _, name := range installed {
		installPlugin(t, root, name)
	}
	f := &fakePlugins{
		built:  map[string]int{},
		warmed: map[string]int{},
		closed: map[string]int{},
		fails:  map[string]bool{},
	}
	f.sup = New(root)
	f.sup.Construct = f.build
	t.Cleanup(func() { _ = f.sup.Close() })
	return f
}

func (f *fakePlugins) build(name string) (ext.Hook, error) {
	f.built[name]++
	if f.fails[name] {
		return nil, fmt.Errorf("test: %s will not start", name)
	}
	return &fakeHook{name: name, warmed: f.warmed, closed: f.closed}, nil
}

// fakeHook is a runner: it warms like a js one, so a role can be answered with it.
type fakeHook struct {
	name   string
	warmed map[string]int
	closed map[string]int
}

func (h *fakeHook) Name() string { return h.name }
func (h *fakeHook) Warm() error  { h.warmed[h.name]++; return nil }
func (h *fakeHook) Close() error { h.closed[h.name]++; return nil }

// A role resolved before a library opens is the one that answers after it: bound
// to a library, warmed once, never restarted. The regression this guards: the
// bind closed the runners a role held and emptied the cache, leaving the role's
// runner without a host and closed (docs/specs/plugin-system.md, defect A).
func TestARoleStartsOnceAndSurvivesTheBind(t *testing.T) {
	f := newFake(t, "acme")
	role := f.sup.Resolve(core.RoleReviewSchedule, "acme", "scheduler")
	if role == nil || role.Name() != "acme" {
		t.Fatalf("the role resolved to %v, want acme", role)
	}
	if f.built["acme"] != 1 || f.warmed["acme"] != 1 {
		t.Fatalf("built %d, warmed %d times, want one each", f.built["acme"], f.warmed["acme"])
	}

	f.sup.Bind(fakeHost{name: "first"}, []string{"acme"})
	if f.closed["acme"] != 0 {
		t.Fatal("the bind closed the runner a role holds")
	}
	if again := f.sup.Resolve(core.RoleReviewSchedule, "acme", "scheduler"); again != role {
		t.Error("the role was answered another runner after the bind")
	}
	if f.built["acme"] != 1 {
		t.Errorf("acme was constructed %d times, want once", f.built["acme"])
	}
}

// A switch swaps the library a capability call reaches and keeps the runners: the
// provider answers the last library bound, and a name still wanted answers the
// same runner. The regression this guards: a runner restarted per library, which
// costs the plugin its module state and puts a second writer on its store.
func TestASwitchSwapsTheHostAndKeepsTheRunner(t *testing.T) {
	f := newFake(t, "acme")
	first, second := fakeHost{name: "first"}, fakeHost{name: "second"}
	f.sup.Bind(first, []string{"acme"})
	if got := f.sup.currentHost(); got != ext.Host(first) {
		t.Fatalf("the provider answers %v, want the library bound first", got)
	}
	runner, err := f.sup.Runner("acme")
	if err != nil {
		t.Fatalf("Runner(acme): %v", err)
	}

	f.sup.Bind(second, []string{"acme"})
	if got := f.sup.currentHost(); got != ext.Host(second) {
		t.Errorf("the provider still answers %v after the switch", got)
	}
	if again, err := f.sup.Runner("acme"); err != nil || again != runner {
		t.Errorf("Runner(acme) answered %v (%v), want the runner the first library started", again, err)
	}
	if f.built["acme"] != 1 || f.closed["acme"] != 0 {
		t.Errorf("built %d, closed %d times, want one build and no close", f.built["acme"], f.closed["acme"])
	}
}

// A runner whose name the next library does not want is closed and dropped. The
// regression this guards: a plugin the previous library enabled kept running
// against the new one, writing a store nothing asked it to write.
func TestASwitchClosesWhatItNoLongerWants(t *testing.T) {
	f := newFake(t)
	f.sup.Bind(nil, []string{"acme", "beta"})
	f.sup.Bind(nil, []string{"acme"})

	if f.closed["beta"] != 1 {
		t.Errorf("beta was closed %d times, want once", f.closed["beta"])
	}
	if f.closed["acme"] != 0 {
		t.Error("the switch closed a plugin it still wants")
	}
	if got := names(f.sup.Enabled()); !slices.Equal(got, []string{"acme"}) {
		t.Errorf("the enabled list answers %v, want acme alone", got)
	}
}

// A name that still plays a role is wanted even after it leaves the enabled
// list: the role's answer is that runner, so closing it would leave the library
// without the plugin it chose.
func TestARoleSurvivesLeavingTheEnabledList(t *testing.T) {
	f := newFake(t, "acme")
	role := f.sup.Resolve(core.RoleCardsDerive, "acme", "cards")
	f.sup.Bind(nil, []string{"acme"})
	f.sup.Bind(nil, nil) // the next library enables it no longer

	if f.closed["acme"] != 0 {
		t.Fatal("the bind closed a runner a role still holds")
	}
	if again, err := f.sup.Runner("acme"); err != nil || again != role {
		t.Errorf("the role's runner answered %v (%v) after the switch", again, err)
	}
}

// One name, one runner: whichever path asks first — a contributed command, the
// enabled list, a role — answers the same runner. The regression this guards: the
// enabled list and the roles each started a copy, two runtime pools, two writers
// of one plugin store (defect C).
func TestOneNameOneRunner(t *testing.T) {
	f := newFake(t, "both")
	first, err := f.sup.Runner("both")
	if err != nil {
		t.Fatalf("Runner(both): %v", err)
	}
	f.sup.Bind(nil, []string{"both"})
	role := f.sup.Resolve(core.RoleCardsDerive, "both", "cards")
	again, err := f.sup.Runner("both")
	if err != nil {
		t.Fatalf("Runner(both) again: %v", err)
	}

	if first != again || first != role {
		t.Error("the same plugin was answered as more than one runner")
	}
	if f.built["both"] != 1 {
		t.Errorf("both was constructed %d times, want once", f.built["both"])
	}
}

// The resolver every door derives through is the card role's plugin: a
// supervisor whose role resolved answers with that runner, not with a second
// one it opened for the asker. The regression this guards: a resolver that
// quietly falls back to the built-in card plugin, so a library whose plugin
// did not start derives a card nobody chose.
func TestCardResolverAnswersTheRolePlugin(t *testing.T) {
	root := t.TempDir()
	installPlugin(t, root, "acme") // deriveBlock answers no cards
	s := New(root)
	t.Cleanup(func() { _ = s.Close() })
	if role := s.Resolve(core.RoleCardsDerive, "acme", "cards"); role == nil || role.Name() != "acme" {
		t.Fatalf("the card role resolved to %v, want acme", role)
	}

	derived, ok := s.CardResolver(core.BuiltinTypes()).Derive("a :: b")
	// ok says the plugin answered; the built-in would also have made a card of
	// this, so an empty answer is the role's own.
	if !ok || len(derived) != 0 {
		t.Errorf("the resolver derived %+v (ok %v), want the role plugin's empty answer", derived, ok)
	}
}

// A card role that resolved to nothing answers a resolver with no plugin: no
// paragraph makes a new card, and a card reads its own words. The regression
// this guards: a fallback here that opens the built-in card plugin — a second
// road beside internal/cards' own "a card reads its own paragraph".
func TestCardResolverWithoutARoleDerivesNothing(t *testing.T) {
	s := New(t.TempDir())
	t.Cleanup(func() { _ = s.Close() })
	if role := s.Resolve(core.RoleCardsDerive, "gone", ""); role != nil {
		t.Fatalf("a role nothing plays resolved to %v", role)
	}

	resolver := s.CardResolver(core.BuiltinTypes())
	if derived, ok := resolver.Derive("a :: b"); ok || len(derived) != 0 {
		t.Errorf("a paragraph derived %+v with no card plugin", derived)
	}
	got := resolver.Face(core.Card{Type: core.TypeBasic, Ref: core.Reference{Block: "b1"}},
		core.BlockSet{"b1": {ID: "b1", Text: "asked :: answered"}})
	if got.Front != "asked :: answered" || got.Back != "" {
		t.Errorf("the card reads %q / %q, want its own words, unsplit", got.Front, got.Back)
	}
}

// A role takes a js plugin only: the rule is checked before anything starts, said
// once in the log, and the role falls back to the plugin this build ships for it.
// The regression this guards: a role that accepts what the enabled list accepts,
// spawning a process it can never ask a hook of (ADR 0006).
func TestARoleTakesJSOnly(t *testing.T) {
	root := t.TempDir()
	installProcessPlugin(t, root, "probe")
	f := newFakeOn(t, root)

	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	role := f.sup.Resolve(core.RoleReviewSchedule, "probe", "scheduler")
	if role == nil || role.Name() != "scheduler" {
		t.Fatalf("the role fell back to %v, want the shipped scheduler", role)
	}
	if f.built["probe"] != 0 {
		t.Error("the refused plugin was constructed: a role never starts what it cannot run")
	}
	if n := strings.Count(logged.String(), "a role takes a js plugin only"); n != 1 {
		t.Fatalf("the refusal logged %d lines saying why, want one: %s", n, logged.String())
	}
}

// A role whose plugin will not start falls back to the one this build ships for
// it, and one that is missing does the same: a library always opens. The log
// names the real reason — a plugin that failed to start is not a plugin that
// runs on the wrong substrate — so a reader is not sent looking for an exec
// manifest.
func TestARoleFallsBackWhenItsPluginWillNotStart(t *testing.T) {
	f := newFake(t)
	f.fails["acme"] = true

	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	role := f.sup.Resolve(core.RoleCardsDerive, "acme", "cards")
	if role == nil || role.Name() != "cards" {
		t.Fatalf("the role resolved to %v, want the shipped cards plugin", role)
	}
	if !strings.Contains(logged.String(), "a role's plugin did not start") {
		t.Errorf("the failure was not reported as a start failure: %s", logged.String())
	}
	if strings.Contains(logged.String(), "takes a js plugin only") {
		t.Errorf("a plugin that failed to start was reported as the substrate rule: %s", logged.String())
	}
	f.sup.Bind(nil, []string{"acme"})
	if f.built["cards"] != 1 {
		t.Errorf("the fallback was constructed %d times, want once", f.built["cards"])
	}
	// A plugin that will not start is one the page can report: Health keeps what
	// the constructor said, so the row says why it is not working.
	if got := f.sup.Health("acme"); got.State != HealthFailed || !strings.Contains(got.Reason, "will not start") {
		t.Errorf("Health reports the plugin as %+v, want it recorded as one that will not start", got)
	}
}

// The enabled list takes both substrates (ADR 0006): a process plugin starts and
// is answered by Enabled, so the app still hears it. The regression this guards: a
// registry that only builds js runners drops a plugin the library enabled.
func TestTheEnabledListTakesAProcessPlugin(t *testing.T) {
	requireShell(t)
	root := t.TempDir()
	installProcessPlugin(t, root, "probe")
	s := New(root)
	s.ProcessTimeout = 10 * time.Second
	t.Cleanup(func() { _ = s.Close() })

	s.Bind(nil, []string{"probe"})
	enabled := s.Enabled()
	if len(enabled) != 1 || enabled[0].Name() != "probe" {
		t.Fatalf("the enabled list answers %v, want the process plugin", names(enabled))
	}
	probe, ok := enabled[0].(*ext.Runner)
	if !ok {
		t.Fatalf("the process plugin came back as %T", enabled[0])
	}
	if _, err := probe.Describe(); err != nil {
		t.Errorf("the process plugin did not answer: %v", err)
	}
}

// Discovery lists what this build can run: the two it ships with the roles they
// play, then what the directory holds with what each manifest declares — without
// starting any of it.
func TestDiscoveryListsTheShippedThenTheInstalled(t *testing.T) {
	s := New(filepath.Join("..", "..", "examples", "plugins"))
	found, err := s.Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(found) < 3 {
		t.Fatalf("discovered %d plugins, want the two shipped and the examples", len(found))
	}
	if found[0].Name != "cards" || !found[0].Builtin || !slices.Contains(found[0].Roles, core.RoleCardsDerive) {
		t.Errorf("the first plugin is %+v, want the shipped cards with its role", found[0])
	}
	if found[1].Name != "scheduler" || !found[1].Builtin || !slices.Contains(found[1].Roles, core.RoleReviewSchedule) {
		t.Errorf("the second plugin is %+v, want the shipped scheduler with its role", found[1])
	}
	var example *Info
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
}

// Close ends every runner once, and a supervisor asked again answers nothing: the
// process's end is not a library's.
func TestCloseEndsEveryRunnerOnce(t *testing.T) {
	f := newFake(t, "acme")
	f.sup.Resolve(core.RoleCardsDerive, "acme", "cards")
	f.sup.Bind(nil, []string{"beta"})
	if err := f.sup.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := f.sup.Close(); err != nil {
		t.Fatalf("Close again: %v", err)
	}
	for _, name := range []string{"acme", "beta"} {
		if f.closed[name] != 1 {
			t.Errorf("%s was closed %d times, want once", name, f.closed[name])
		}
	}
	if _, err := f.sup.Runner("acme"); err == nil {
		t.Error("a closed supervisor still started a plugin")
	}
}

// broken is the js plugin these tests drive: one hook that throws when the block
// it is asked about says so, and answers otherwise, so a test can stop it and
// start it again.
const brokenPlugin = `
export function deriveBlock(block) {
  if (block && block.text === "broken") throw new Error("the plugin is broken");
  return [];
}
`

// realPlugins is a supervisor over the engine: the one plugin it holds is a real
// js runner, written to a directory of this test's own, so the three-strikes rule
// under test is the one a user's plugin meets.
func realPlugins(t *testing.T, name string) *Supervisor {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	manifest := fmt.Sprintf(`{"id":"test.%s","name":%q,"version":"1.0.0","apiVersion":1,"runtime":"js","entry":"main.js","implements":{"deriveBlock":1}}`, name, name)
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(brokenPlugin), 0o644); err != nil {
		t.Fatalf("entry: %v", err)
	}
	s := New(root)
	s.Construct = func(name string) (ext.Hook, error) {
		if name == "" || name == "cards" || name == "scheduler" {
			return s.Build(name)
		}
		return ext.OpenJS(filepath.Join(root, name), nil, ext.JSLimits{})
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// stop breaks a runner three times in a row: three failed calls leave it stopped.
func stop(t *testing.T, hook ext.Hook) {
	t.Helper()
	for i := range 3 {
		if _, err := derive(t, hook, brokenText); err == nil {
			t.Fatalf("call %d succeeded, want the plugin's own failure", i+1)
		}
	}
}

// brokenText is the text the plugin above throws on, and an ordinary one it
// answers: the two halves of a streak under test.
const (
	brokenText = "broken"
	plainText  = "spacing :: the gaps"
)

// derive asks the one hook these tests stop and start, through the capability a
// card role is answered with: a call site above never names it.
func derive(t *testing.T, hook ext.Hook, text string) ([]ext.DerivedCard, error) {
	t.Helper()
	return ext.HooksOf(hook).DeriveBlock(ext.DeriveIn{Block: ext.Block{Text: text}})
}

// A role whose plugin three failed calls stopped falls back to the plugin this
// build ships for it, and the role is the built-in's. The regression this
// guards: a stopped role plugin used as if it worked, every call answering
// ErrDisabled and the cards it should have made silently missing
// (docs/specs/plugin-system.md P1: 停用之后回退内置 runner).
func TestARoleFallsBackWhenItsPluginIsDisabled(t *testing.T) {
	s := realPlugins(t, "acme")
	role := s.Resolve(core.RoleCardsDerive, "acme", "cards")
	if role == nil || role.Name() != "acme" {
		t.Fatalf("the role resolved to %v, want acme", role)
	}
	stop(t, role)

	// The library opens again with the same choice: the stopped plugin plays no
	// role, and the shipped one takes it.
	again := s.Resolve(core.RoleCardsDerive, "acme", "cards")
	if again == nil || again.Name() != "cards" {
		t.Fatalf("the role resolved to %v, want the shipped cards plugin", again)
	}
	if _, err := derive(t, again, plainText); err != nil {
		t.Errorf("the fallback does not answer: %v", err)
	}
	if got := s.Health("acme"); len(got.Fallback) != 1 || got.Fallback[0] != (Fallback{Role: core.RoleCardsDerive, Player: "cards"}) {
		t.Errorf("Health reports %q's fallbacks as %+v, want the shipped cards plugin playing its role", "acme", got.Fallback)
	}
}

// Three failed calls stop a plugin whole, and Health says so in the plugin's own
// words: the state, the count and the reason, which the settings page repeats
// rather than translates. The regression this guards: a stopped plugin nobody
// can see — no state and no reason (issue #266, criterion 1).
func TestHealthReportsAStoppedPluginAndItsReason(t *testing.T) {
	s := realPlugins(t, "acme")
	hook, err := s.Runner("acme")
	if err != nil {
		t.Fatalf("Runner(acme): %v", err)
	}
	stop(t, hook)

	got := s.Health("acme")
	if got.State != HealthDisabled || got.Failures != 3 {
		t.Fatalf("the stopped plugin reads as %+v, want three failures and disabled", got)
	}
	if !strings.Contains(got.Reason, "the plugin is broken") {
		t.Errorf("Health keeps %q, want the plugin's own reason", got.Reason)
	}

	// The list the settings page draws carries the same answer.
	found, err := s.Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	var acme *Info
	for i := range found {
		if found[i].Name == "acme" {
			acme = &found[i]
		}
	}
	if acme == nil {
		t.Fatalf("the plugin is not discovered: %+v", found)
	}
	if acme.Health.State != HealthDisabled || acme.Health.Failures != 3 {
		t.Errorf("discovery reports the plugin as %+v, want it stopped", acme.Health)
	}
}

// Re-enable is the way back: the plugin-level count goes, the runner is opened
// again and the next call is answered. The regression this guards: a stopped
// plugin only a restart of the app could revive (issue #266, criterion 2).
func TestReenableOpensAStoppedPluginAgain(t *testing.T) {
	s := realPlugins(t, "acme")
	hook, err := s.Runner("acme")
	if err != nil {
		t.Fatalf("Runner(acme): %v", err)
	}
	stop(t, hook)
	if _, err := derive(t, hook, plainText); !errors.Is(err, ext.ErrDisabled) {
		t.Fatalf("a stopped runner answered %v, want ErrDisabled", err)
	}

	if err := s.Reenable("acme"); err != nil {
		t.Fatalf("Reenable: %v", err)
	}
	if got := s.Health("acme"); got.State != HealthOK || got.Failures != 0 || got.Reason != "" {
		t.Fatalf("after Re-enable the plugin reads as %+v, want it running with no count", got)
	}
	if _, err := derive(t, hook, plainText); err != nil {
		t.Fatalf("the runner was not opened again: %v", err)
	}
}

// A plugin that failed once or twice is failing, not stopped: Health reports the
// streak with its count and its reason, and a call that succeeds clears it. The
// regression this guards: a plugin reported as stopped before it is, so the row
// offers a way back to one that never needed it.
func TestHealthCountsAStreakBeforeItStops(t *testing.T) {
	s := realPlugins(t, "acme")
	hook, err := s.Runner("acme")
	if err != nil {
		t.Fatalf("Runner(acme): %v", err)
	}
	if got := s.Health("acme"); got.State != HealthOK || got.Failures != 0 {
		t.Fatalf("a fresh plugin reads as %+v, want it running", got)
	}

	if _, err := derive(t, hook, brokenText); err == nil {
		t.Fatal("the plugin's own failure was answered")
	}
	if got := s.Health("acme"); got.State != HealthFailing || got.Failures != 1 || got.Reason == "" {
		t.Fatalf("one failure reads as %+v, want a failing streak of one", got)
	}

	if _, err := derive(t, hook, plainText); err != nil {
		t.Fatalf("the plugin's own answer: %v", err)
	}
	if got := s.Health("acme"); got.State != HealthOK || got.Failures != 0 || got.Reason != "" {
		t.Errorf("a success left the plugin at %+v, want it running with no count", got)
	}
}

// Re-enable starts a plugin that would not start, once: a row that offers the
// action for a plugin Health calls failed must be a row that does something. The
// regression this guards: an action that only clears a count, leaving a plugin
// that never ran behind a row that says it is fine.
func TestReenableStartsAPluginThatWouldNotStart(t *testing.T) {
	f := newFake(t, "acme")
	f.fails["acme"] = true
	f.sup.Bind(nil, []string{"acme"})
	if got := f.sup.Health("acme"); got.State != HealthFailed {
		t.Fatalf("the plugin reads as %+v, want one that will not start", got)
	}

	f.fails["acme"] = false
	if err := f.sup.Reenable("acme"); err != nil {
		t.Fatalf("Reenable: %v", err)
	}
	if got := f.sup.Health("acme"); got.State != HealthOK || got.Reason != "" {
		t.Errorf("after Re-enable the plugin reads as %+v, want it running", got)
	}
	if f.built["acme"] != 2 {
		t.Errorf("acme was constructed %d times, want the retry to be the second", f.built["acme"])
	}
}

// Re-enabled, a plugin a role lost falls back no longer: the next resolve answers
// it and the row stops naming a built-in. The regression this guards: a fallback
// that is a history rather than a state, so the page keeps saying a built-in plays
// a role its plugin has again.
func TestARolePlaysItsPluginAgainAfterReenable(t *testing.T) {
	s := realPlugins(t, "acme")
	role := s.Resolve(core.RoleCardsDerive, "acme", "cards")
	if role == nil || role.Name() != "acme" {
		t.Fatalf("the role resolved to %v, want acme", role)
	}
	stop(t, role)
	if again := s.Resolve(core.RoleCardsDerive, "acme", "cards"); again == nil || again.Name() != "cards" {
		t.Fatalf("the stopped plugin's role resolved to %v, want the shipped cards plugin", again)
	}

	if err := s.Reenable("acme"); err != nil {
		t.Fatalf("Reenable: %v", err)
	}
	if again := s.Resolve(core.RoleCardsDerive, "acme", "cards"); again == nil || again.Name() != "acme" {
		t.Fatalf("the re-enabled plugin's role resolved to %v, want acme", again)
	}
	if got := s.Health("acme"); len(got.Fallback) != 0 {
		t.Errorf("the row still says %+v plays its role", got.Fallback)
	}
}

// names is the manifest names of the runners given, for a readable assertion.
func names(hooks []ext.Hook) []string {
	out := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		out = append(out, hook.Name())
	}
	return out
}

// newFakeOn is newFake over a directory: the fake runner is built instead of one
// from the directory, but the manifests are read from it, so the js-only rule
// sees a real runtime.
func newFakeOn(t *testing.T, root string) *fakePlugins {
	t.Helper()
	f := &fakePlugins{
		built:  map[string]int{},
		warmed: map[string]int{},
		closed: map[string]int{},
		fails:  map[string]bool{},
	}
	f.sup = New(root)
	f.sup.Construct = f.build
	t.Cleanup(func() { _ = f.sup.Close() })
	return f
}

// installPlugin writes a js plugin under root, so that the js-only rule and
// discovery read a manifest the way they read an installed one.
func installPlugin(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	manifest := fmt.Sprintf(`{"id":"test.%s","name":%q,"version":"1.0.0","apiVersion":1,"runtime":"js","entry":"main.js","implements":{"deriveBlock":1}}`, name, name)
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte("export function deriveBlock() { return [] }\n"), 0o644); err != nil {
		t.Fatalf("entry: %v", err)
	}
}

// installProcessPlugin writes an exec plugin under root: a manifest naming the
// process runtime and a script that answers every request, echoing its id.
func installProcessPlugin(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	script := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(script, []byte(processScript), 0o755); err != nil {
		t.Fatalf("plugin script: %v", err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"version":"1.0.0","protocol":1,"entrypoint":[%q]}`,
		name, script)
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
}

// processScript is a whole exec plugin: it answers every request it hears and
// echoes the id, so the host sees one answered call.
const processScript = `#!/bin/sh
while IFS= read -r line; do
	id=${line#*\"id\":}
	id=${id%%,*}
	printf '{"id":%s,"result":null}\n' "${id:-0}"
done
`

// requireShell skips a test whose fake plugin is a shell script.
func requireShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake process plugin is a shell script")
	}
}
