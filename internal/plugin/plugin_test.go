package plugin

import (
	"bytes"
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

func (h *fakeHook) Name() string                { return h.name }
func (h *fakeHook) Call(string, any, any) error { return nil }
func (h *fakeHook) Warm() error                 { h.warmed[h.name]++; return nil }
func (h *fakeHook) Close() error                { h.closed[h.name]++; return nil }

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
	if err := enabled[0].Call("describe", nil, nil); err != nil {
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
