package ext_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
)

// hostNow is the clock a plugin's host answers with: fixed, so that what a
// call records is what the test expects.
var hostNow = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

// testHost is the library a plugin reaches in these tests: the store itself,
// plus the one write the store leaves to the app — deriving a card from a
// block. It is the same shape cmd hands a downloaded extension.
type testHost struct {
	*store.Library
}

func newTestHost(t *testing.T) testHost {
	t.Helper()
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return hostNow }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return testHost{Library: lib}
}

// CreateCard derives the cards a block asks for, as the app's card plugin does.
func (h testHost) CreateCard(blockID core.ID) (core.Card, error) {
	cards, err := h.Library.CreateCards(blockID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		return core.Card{}, err
	}
	return cards[0], nil
}

// Faces is the API layer's rendering half; a plugin has no use for it.
func (h testHost) Faces(core.Card) (front, back, link string) { return "", "", core.LinkLive.String() }

// writeJS lays out an extension in a temporary directory and returns it.
func writeJS(t *testing.T, manifest string, source string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// manifestFor is a js manifest that declares the capabilities given, carrying
// every field the js contract requires.
func manifestFor(name string, caps ...string) string {
	body, err := json.Marshal(map[string]any{
		"id":           "test." + name,
		"name":         name,
		"version":      "1.0.0",
		"apiVersion":   1,
		"runtime":      "js",
		"entry":        "main.js",
		"capabilities": caps,
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

// A js extension is loaded, called and answered like any other: the same
// manifest, the same call shape.
func TestJSCall(t *testing.T) {
	dir := writeJS(t, manifestFor("cards"), `
export function describe() {
  return { name: "cards", protocol: 1 };
}
export function cards(block) {
  const text = block.text || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{ block: block.id, front: text.slice(0, at).trim(), back: text.slice(at + 2).trim() }];
}
`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if runner.Name() != "cards" {
		t.Errorf("name is %q", runner.Name())
	}

	var described struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	}
	if err := runner.Call("describe", nil, &described); err != nil {
		t.Fatalf("describe: %v", err)
	}
	if described.Name != "cards" || described.Protocol != 1 {
		t.Errorf("describe answered %+v", described)
	}

	var cards []struct {
		Block string `json:"block"`
		Front string `json:"front"`
		Back  string `json:"back"`
	}
	err = runner.Call("cards", map[string]any{"id": "block_1", "text": "spacing :: the gaps"}, &cards)
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != 1 || cards[0].Front != "spacing" || cards[0].Back != "the gaps" || cards[0].Block != "block_1" {
		t.Errorf("cards answered %+v", cards)
	}
}

// The host object is what the manifest declares and nothing else: an extension
// reaches no door it did not name.
func TestJSCapabilitiesGateTheHost(t *testing.T) {
	source := `
export function echo(x) { return remit.echo(x); }
`
	without := writeJS(t, manifestFor("quiet"), source)
	quiet, err := ext.OpenJS(without, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer quiet.Close()
	var echoed string
	if err := quiet.Call("echo", "hello", &echoed); err == nil {
		t.Errorf("an undeclared capability reached the host: %q", echoed)
	}
	if quiet.Has(ext.CapEcho) {
		t.Error("Has reports a capability the manifest never declared")
	}

	with := writeJS(t, manifestFor("loud", ext.CapEcho), source)
	loud, err := ext.OpenJS(with, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer loud.Close()
	if err := loud.Call("echo", "hello", &echoed); err != nil {
		t.Fatalf("declared capability failed: %v", err)
	}
	if echoed != "hello" {
		t.Errorf("echo answered %q, want hello", echoed)
	}
}

// A hook that never returns is interrupted, and the caller gets its time back.
func TestJSTimeout(t *testing.T) {
	dir := writeJS(t, manifestFor("spin"), `export function spin() { for (;;) {} }`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	start := time.Now()
	err = runner.Call("spin", nil, nil)
	if !errors.Is(err, ext.ErrTimeout) {
		t.Fatalf("call answered %v, want a timeout", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("the interrupt took %v", took)
	}
}

// A hook that allocates without end hits the limit and is reported, not allowed
// to take the process down.
func TestJSMemoryLimit(t *testing.T) {
	dir := writeJS(t, manifestFor("hungry"), `
export function hungry() {
  const kept = [];
  for (;;) kept.push(new Array(4096).fill(7));
}
`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{Memory: 2 << 20, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	err = runner.Call("hungry", nil, nil)
	if !errors.Is(err, ext.ErrMemoryLimit) && !errors.Is(err, ext.ErrTimeout) {
		t.Fatalf("call answered %v, want the memory limit or the time", err)
	}
}

// Dynamic code is off: a hook cannot build code to run.
func TestJSNoDynamicCode(t *testing.T) {
	dir := writeJS(t, manifestFor("dynamic"), `export function dynamic() { return eval("1 + 1"); }`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if err := runner.Call("dynamic", nil, nil); err == nil {
		t.Error("eval ran in an extension")
	}
}

// A hook the extension does not carry, and an entry that points outside its own
// directory, are refused.
func TestJSRefusesWhatItCannotRun(t *testing.T) {
	dir := writeJS(t, manifestFor("small"), `export function one() { return 1; }`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if err := runner.Call("two", nil, nil); !errors.Is(err, ext.ErrNoHook) {
		t.Errorf("a missing hook answered %v", err)
	}

	escaping := writeJS(t, `{"id":"test.bad","name":"bad","apiVersion":1,"runtime":"js","entry":"../../etc/passwd"}`, `export function one() { return 1; }`)
	if _, err := ext.OpenJS(escaping, nil, ext.JSLimits{}); err == nil {
		t.Error("an entry outside the extension was accepted")
	}
}

// A warmed pool serves calls without making another runtime, and two calls at
// once share the pool's depth without deadlocking.
func TestJSPoolWarmAndReuse(t *testing.T) {
	dir := writeJS(t, manifestFor("pooled"), `export function one() { return 1; }`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if err := runner.Warm(); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if got := runner.Runtimes(); got != 2 {
		t.Fatalf("warm made %d runtimes, want 2", got)
	}
	if err := runner.Warm(); err != nil {
		t.Fatalf("second warm: %v", err)
	}

	var one int
	if err := runner.Call("one", nil, &one); err != nil {
		t.Fatalf("call: %v", err)
	}
	if one != 1 {
		t.Errorf("one answered %d", one)
	}
	if got := runner.Runtimes(); got != 2 {
		t.Errorf("a warmed pool made another runtime: %d", got)
	}

	// Two calls at once: both borrow, both return, neither blocks the other.
	done := make(chan error, 2)
	for range 2 {
		go func() {
			var n int
			done <- runner.Call("one", nil, &n)
		}()
	}
	for range 2 {
		if err := <-done; err != nil {
			t.Errorf("concurrent call: %v", err)
		}
	}
	if got := runner.Runtimes(); got != 2 {
		t.Errorf("two calls over a warm pool made %d runtimes, want 2", got)
	}
}

// A pooled runtime is kept, not thrown away: module-level state survives from
// one call to the next. This is the trade ADR 0006 accepted for not paying a
// load per call, written down as a test so a change to the pool is a decision.
func TestJSPoolKeepsModuleState(t *testing.T) {
	dir := writeJS(t, manifestFor("counting"), `
let seen = 0;
export function tick() { seen += 1; return seen; }
`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	var seen int
	for i := 1; i <= 3; i++ {
		if err := runner.Call("tick", nil, &seen); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		if seen != i {
			t.Fatalf("tick %d answered %d: the runtime was replaced", i, seen)
		}
	}
}

// Three failed calls in a row stop the extension; the fourth answers
// ErrDisabled without touching a runtime. A success in between clears the
// streak.
func TestJSThreeStrikesDisable(t *testing.T) {
	dir := writeJS(t, manifestFor("flaky"), `
export function go(ok) {
  if (!ok) throw new Error("no");
  return 1;
}
`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	fail := false
	pass := true
	for i := range 2 {
		if err := runner.Call("go", fail, nil); err == nil {
			t.Fatalf("call %d succeeded", i)
		}
	}
	if got := runner.Failures(); got != 2 {
		t.Fatalf("failures = %d, want 2", got)
	}
	if err := runner.Call("go", pass, nil); err != nil {
		t.Fatalf("success after failures: %v", err)
	}
	if got := runner.Failures(); got != 0 {
		t.Fatalf("a success left the streak at %d", got)
	}

	for i := range 3 {
		if err := runner.Call("go", fail, nil); err == nil {
			t.Fatalf("strike %d succeeded", i)
		}
	}
	if !runner.Disabled() {
		t.Fatal("three strikes left the runner enabled")
	}
	before := runner.Runtimes()
	if err := runner.Call("go", pass, nil); !errors.Is(err, ext.ErrDisabled) {
		t.Fatalf("a disabled runner answered %v, want ErrDisabled", err)
	}
	if got := runner.Runtimes(); got != before {
		t.Errorf("the disabled call made a runtime: %d, want %d", got, before)
	}
}

// Warming is when a broken module is heard from, not the first review.
func TestJSWarmSurfacesLoadErrors(t *testing.T) {
	// A module that throws while evaluating: compileJS accepts it (it parses),
	// the load does not.
	dir := writeJS(t, manifestFor("boom"), `throw new Error("boom");`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	if err := runner.Warm(); err == nil {
		t.Error("warming a module that throws at load succeeded")
	}
}

// The six capabilities are real functions, not declarations: each one is
// called through a runtime and answers what the library holds.
func TestJSCapabilityFunctionsRoundTrip(t *testing.T) {
	host := newTestHost(t)
	note, err := host.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{
		{Text: "what is a cell :: the unit of life"},
	}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	blockID := string(note.Blocks[0].ID)

	dir := writeJS(t, manifestFor("hosted",
		ext.CapNotesRead, ext.CapCardsWrite, ext.CapReviewRead, ext.CapReviewWrite,
		ext.CapScheduleFSRS, ext.CapStoreOwn), `
export function list() { return remit.notes.list(); }
export function get(id) { return remit.notes.get(id); }
export function search(text) { return remit.notes.search(text); }
export function create(block) { return remit.cards.create(block); }
export function update(id, patch) { return remit.cards.update(id, patch); }
export function remove(id) { return remit.cards.delete(id); }
export function history(id) { return remit.review.history(id); }
export function dueCount() { return remit.review.dueCount(); }
export function record(id, grade) { return remit.review.record(id, grade, "2026-11-06T10:00:00Z"); }
export function step(state, grade) { return remit.fsrs.review(state, grade, "2026-11-07T09:00:00Z"); }
export function own() { return remit.store.get(); }
export function keep(value) { return remit.store.set(value); }
`)
	runner, err := ext.OpenJS(dir, host, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	// notes.list and notes.get: what the library holds.
	var notes []struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Blocks []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"blocks"`
	}
	if err := runner.Call("list", nil, &notes); err != nil {
		t.Fatalf("notes.list: %v", err)
	}
	if len(notes) != 1 || notes[0].ID != string(note.ID) || notes[0].Title != "Biology" {
		t.Fatalf("notes.list answered %+v", notes)
	}
	if len(notes[0].Blocks) != 1 || notes[0].Blocks[0].ID != blockID || notes[0].Blocks[0].Text != note.Blocks[0].Text {
		t.Errorf("notes.list blocks = %+v", notes[0].Blocks)
	}

	var got struct {
		Title string `json:"title"`
	}
	if err := runner.Call("get", string(note.ID), &got); err != nil {
		t.Fatalf("notes.get: %v", err)
	}
	if got.Title != "Biology" {
		t.Errorf("notes.get answered %+v", got)
	}
	var missing json.RawMessage
	if err := runner.Call("get", "note_nope", &missing); err != nil {
		t.Fatalf("notes.get missing: %v", err)
	}
	if string(missing) != "null" {
		t.Errorf("notes.get of a note that is not there answered %s, want null", missing)
	}

	// notes.search: the same text search as everywhere else.
	var found []struct {
		Title string `json:"title"`
	}
	if err := runner.Call("search", "UNIT OF LIFE", &found); err != nil {
		t.Fatalf("notes.search: %v", err)
	}
	if len(found) != 1 || found[0].Title != "Biology" {
		t.Errorf("notes.search answered %+v", found)
	}

	// cards.create: derived from the block, by the host.
	var created struct {
		ID      string `json:"id"`
		Block   string `json:"block"`
		Type    string `json:"type"`
		Status  string `json:"status"`
		Reviews int    `json:"reviews"`
	}
	if err := runner.Call("create", blockID, &created); err != nil {
		t.Fatalf("cards.create: %v", err)
	}
	if created.ID == "" || created.Block != blockID || created.Type != string(core.TypeBasic) || created.Status != "new" {
		t.Fatalf("cards.create answered %+v", created)
	}

	// cards.update: the fields and the status, as the API's patch takes them.
	var patched struct {
		Fields map[string]string `json:"fields"`
		Edited bool              `json:"edited"`
		Status string            `json:"status"`
	}
	patch := map[string]any{"fields": map[string]string{"Front": "sharper question"}, "status": "suspended"}
	if err := runner.Call("update", []any{created.ID, patch}, &patched); err != nil {
		t.Fatalf("cards.update: %v", err)
	}
	if patched.Fields["Front"] != "sharper question" || !patched.Edited || patched.Status != "suspended" {
		t.Fatalf("cards.update answered %+v", patched)
	}
	if err := runner.Call("update", []any{created.ID, map[string]any{"status": "new"}}, &patched); err != nil {
		t.Fatalf("cards.update status: %v", err)
	}
	if patched.Status != "new" || patched.Fields["Front"] != "sharper question" {
		t.Errorf("a status-only patch lost the fields: %+v", patched)
	}

	// review.record: the answer is in the log and the card is scheduled.
	var recorded struct {
		Reviews int    `json:"reviews"`
		Due     string `json:"due"`
	}
	if err := runner.Call("record", []any{created.ID, "good"}, &recorded); err != nil {
		t.Fatalf("review.record: %v", err)
	}
	if recorded.Reviews != 1 || recorded.Due == "" {
		t.Fatalf("review.record answered %+v", recorded)
	}

	var history []struct {
		Card   string `json:"card"`
		Rating string `json:"rating"`
		At     string `json:"at"`
	}
	if err := runner.Call("history", created.ID, &history); err != nil {
		t.Fatalf("review.history: %v", err)
	}
	if len(history) != 1 || history[0].Card != created.ID || history[0].Rating != "good" || history[0].At != "2026-11-06T10:00:00Z" {
		t.Fatalf("review.history answered %+v", history)
	}

	var due int
	if err := runner.Call("dueCount", nil, &due); err != nil {
		t.Fatalf("review.dueCount: %v", err)
	}
	if due != 0 {
		t.Errorf("dueCount = %d after the card was scheduled forward", due)
	}

	// fsrs.review: the host's maths, one answer further.
	var next struct {
		Stability  float64 `json:"stability"`
		Due        string  `json:"due"`
		LastReview string  `json:"lastReview"`
		Reviews    int     `json:"reviews"`
	}
	state := map[string]any{"stability": 1.5, "difficulty": 5.0, "reviews": 1, "lastReview": "2026-11-01T09:00:00Z"}
	if err := runner.Call("step", []any{state, "good"}, &next); err != nil {
		t.Fatalf("fsrs.review: %v", err)
	}
	if next.Reviews != 2 || next.Due == "" || next.LastReview != "2026-11-07T09:00:00Z" || next.Stability <= 0 {
		t.Fatalf("fsrs.review answered %+v", next)
	}

	// store.set / store.get: the plugin's own namespace.
	var own any
	if err := runner.Call("own", nil, &own); err != nil {
		t.Fatalf("store.get: %v", err)
	}
	if own != nil {
		t.Errorf("a plugin that stored nothing read %v", own)
	}
	if err := runner.Call("keep", `{"streak":3}`, nil); err != nil {
		t.Fatalf("store.set: %v", err)
	}
	var kept string
	if err := runner.Call("own", nil, &kept); err != nil {
		t.Fatalf("store.get after set: %v", err)
	}
	if kept != `{"streak":3}` {
		t.Errorf("store.get = %q", kept)
	}

	// cards.delete: the card is gone, its reviews stay.
	if err := runner.Call("remove", created.ID, nil); err != nil {
		t.Fatalf("cards.delete: %v", err)
	}
	if _, ok := host.Card(core.ID(created.ID)); ok {
		t.Error("cards.delete left the card in the library")
	}
	if events, err := host.History(core.ID(created.ID)); err != nil || len(events) != 1 {
		t.Errorf("history after the delete = %+v (%v), want the one review", events, err)
	}
}

// Two plugins keep their own namespaces: one cannot read the other's value, and
// the library holds both under the id that wrote each.
func TestJSStoreNamespacesAreThePluginsOwn(t *testing.T) {
	host := newTestHost(t)
	source := `
export function get() { return remit.store.get(); }
export function set(value) { return remit.store.set(value); }
`
	open := func(name string) *ext.JSRunner {
		t.Helper()
		runner, err := ext.OpenJS(writeJS(t, manifestFor(name, ext.CapStoreOwn), source), host, ext.JSLimits{})
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		t.Cleanup(func() { _ = runner.Close() })
		return runner
	}
	first, second := open("first"), open("second")

	if err := first.Call("set", "one", nil); err != nil {
		t.Fatalf("first set: %v", err)
	}
	if err := second.Call("set", "two", nil); err != nil {
		t.Fatalf("second set: %v", err)
	}
	var got string
	if err := first.Call("get", nil, &got); err != nil || got != "one" {
		t.Errorf("first read %q (%v), want one", got, err)
	}
	if err := second.Call("get", nil, &got); err != nil || got != "two" {
		t.Errorf("second read %q (%v), want two", got, err)
	}
	for id, want := range map[string]string{"test.first": "one", "test.second": "two"} {
		if value, ok := host.PluginData(id); !ok || value != want {
			t.Errorf("library data for %s = %q, %v; want %q", id, value, ok, want)
		}
	}
}
