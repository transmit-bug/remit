package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
	"github.com/transmit-bug/remit/internal/plugin"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// The settings page's switch is the listener: turning it on binds a loopback
// port now, writes the address and an owner-only token, and serves the library
// to a client that reads them; turning it off withdraws the address.
func TestAgentServiceLifecycle(t *testing.T) {
	lib, err := store.Create(t.TempDir(), store.Options{})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	defer lib.Close()
	if _, err := lib.SaveNote(core.Note{Title: "Memory", Blocks: []core.Block{{Text: "spacing :: beats cramming"}}}); err != nil {
		t.Fatalf("save note: %v", err)
	}

	dir := t.TempDir()
	s := newAgentService(dir, plugin.New(""))
	s.setLibrary(lib)
	if s.Status().Address != "" {
		t.Error("the service claims an address before it started")
	}

	if err := s.SetEnabled(true); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(s.Stop)

	status := s.Status()
	if status.Address == "" {
		t.Fatal("the started service wrote no address")
	}
	if status.TokenPath != filepath.Join(dir, agentTokenFile) {
		t.Errorf("token path = %q", status.TokenPath)
	}
	for _, path := range []string{filepath.Join(dir, agentAddressFile), status.TokenPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s is mode %o, want 600", filepath.Base(path), perm)
		}
	}

	// A client finds the API from the files alone: the address is enough to
	// read the library.
	resp, err := http.Get((&url.URL{Scheme: "http", Host: status.Address}).String() + "/v1/library")
	if err != nil {
		t.Fatalf("GET /v1/library: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/library = %d", resp.StatusCode)
	}

	before, err := os.ReadFile(status.TokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RotateToken(); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	after, err := os.ReadFile(status.TokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) == string(after) {
		t.Error("rotating the token wrote the same one")
	}
	if s.Status().Address == "" {
		t.Error("rotating the token left the listener closed")
	}

	if err := s.SetEnabled(false); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, agentAddressFile)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the stopped service kept its address file: %v", err)
	}
	if s.Status().Address != "" {
		t.Error("the stopped service still reports an address")
	}
}

// A launch honours the switch a previous run left on, and a launch with it off
// opens nothing.
func TestAgentServiceFollowsTheSetting(t *testing.T) {
	lib, err := store.Create(t.TempDir(), store.Options{})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	defer lib.Close()

	off := newAgentService(t.TempDir(), plugin.New(""))
	off.setLibrary(lib)
	if err := off.startIfEnabled(false); err != nil {
		t.Fatalf("start while off: %v", err)
	}
	if off.Status().Address != "" {
		t.Error("the service started while its setting was off")
	}

	on := newAgentService(t.TempDir(), plugin.New(""))
	on.setLibrary(lib)
	if err := on.startIfEnabled(true); err != nil {
		t.Fatalf("start while on: %v", err)
	}
	defer on.Stop()
	if on.Status().Address == "" {
		t.Error("the service did not start while its setting was on")
	}
}

// A card plugin the user switched to is the one that derives, whichever door a
// card comes through: the API's card and a capability's card carry one producer
// stamp, and it is the role plugin's. The regression this guards: the window
// showed the user's plugin while the API and remit.cards.create each opened the
// built-in one, so the stamps disagreed and the plugin ran three times
// (docs/specs/plugin-system.md, defect H).
func TestOneCardRunnerDerivesEveryDoor(t *testing.T) {
	root := t.TempDir()
	installCards(t, root, "acme.cards", "example.acme", "2.0.0", "ACME")
	installCardWriter(t, root, "writer")

	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	note, err := lib.SaveNote(core.Note{Title: "Spacing", Blocks: []core.Block{{Text: "spacing :: beats cramming"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	block := note.Blocks[0].ID
	if err := lib.SetEnabledExtensions([]string{"acme.cards", "writer"}); err != nil {
		t.Fatalf("enable the plugins: %v", err)
	}
	if err := lib.SetRole(core.RoleCardsDerive, "acme.cards"); err != nil {
		t.Fatalf("give acme.cards the card role: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close the seeded library: %v", err)
	}

	built := map[string]int{}
	plugins := plugin.New(root)
	plugins.Construct = func(name string) (ext.Hook, error) {
		built[name]++
		return plugins.Build(name)
	}
	t.Cleanup(func() { _ = plugins.Close() })

	// The window resolves the roles the library names, then binds what the
	// plugins act on — main.go's callback, without a frame drawn.
	app := appui.NewApp(appui.Options{LibraryDir: dir, Plugins: plugins})
	opened := app.Library()
	if opened == nil {
		t.Fatal("the library did not open")
	}
	plugins.Bind(hostFor(opened, plugins), opened.EnabledExtensions())

	// one card through the capability the writer declared
	writer, err := plugins.Runner("writer")
	if err != nil {
		t.Fatalf("Runner(writer): %v", err)
	}
	var made struct {
		ID string `json:"id"`
	}
	command := map[string]any{"name": "makeCard", "context": map[string]any{"block": string(block)}}
	if err := writer.Call(ext.CommandHook, command, &made); err != nil {
		t.Fatalf("remit.cards.create: %v", err)
	}

	// one card through the loopback API
	service := newAgentService(t.TempDir(), plugins)
	service.setLibrary(opened)
	if err := service.SetEnabled(true); err != nil {
		t.Fatalf("start the agent api: %v", err)
	}
	t.Cleanup(service.Stop)
	created := postCard(t, service, block)

	producer := func(id string) core.Producer {
		t.Helper()
		card, ok := opened.Card(core.ID(id))
		if !ok {
			t.Fatalf("the library holds no card %q", id)
		}
		return card.Producer
	}
	want := core.Producer{ID: "example.acme", Version: "2.0.0"}
	if got := producer(made.ID); got != want {
		t.Errorf("the capability's card is stamped %+v, want the role plugin %+v", got, want)
	}
	if got := producer(created); got != want {
		t.Errorf("the API's card is stamped %+v, want the role plugin %+v", got, want)
	}
	// What the stamps are evidence of, counted: the card plugin the role names was
	// built once, and no built-in card plugin beside it. A door that opened its own
	// never reaches this counter — it is stamped differently above instead.
	if built["acme.cards"] != 1 {
		t.Errorf("the card plugin was constructed %d times, want once", built["acme.cards"])
	}
	if built[builtin.CardsName] != 0 {
		t.Errorf("the built-in card plugin was constructed %d times beside the role's", built[builtin.CardsName])
	}
}

// No card plugin starts: every door reads a card the same way — the block's own
// words, with the card's own edits on top — and none of them makes a new one,
// because the fallback is internal/cards' single one. The regression this guards:
// cmd opened the built-in card plugin for the API and for a capability when the
// library's own plugin did not start, so those two derived a card nobody chose
// (split at the separator, and read differently from the window) while the
// window read the block's own words.
func TestNoCardPluginIsOneFallback(t *testing.T) {
	plugins := plugin.New(t.TempDir())
	plugins.Construct = func(name string) (ext.Hook, error) {
		return nil, fmt.Errorf("nothing starts: %s", name)
	}
	t.Cleanup(func() { _ = plugins.Close() })
	// A role that cannot start is the supervisor's answer, not cmd's: it resolves
	// the role, fails, falls back to the shipped one, fails that too, and holds
	// nothing.
	plugins.Resolve(core.RoleCardsDerive, "acme.cards", store.DefaultCardsPlugin)

	lib, err := store.Create(t.TempDir(), store.Options{})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	defer lib.Close()
	note, err := lib.SaveNote(core.Note{Title: "Spacing", Blocks: []core.Block{{Text: "spacing :: beats cramming"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	block := note.Blocks[0].ID
	if _, err := lib.CreateCards(block, []core.Derived{{
		Type:    core.TypeBasic,
		Fields:  map[string]string{"Front": "spacing", "Back": "beats cramming"},
		Ordinal: 0,
	}}); err != nil {
		t.Fatalf("make a card: %v", err)
	}
	card := cardOf(t, lib, block)
	card.Fields = map[string]string{"Front": "what is spacing"}
	card.Edited = true
	if err := lib.SaveCard(card); err != nil {
		t.Fatalf("edit the card: %v", err)
	}

	// The library a plugin reaches and the API read the card alike, and neither
	// invents a card: without a card plugin a paragraph makes none.
	host := hostFor(lib, plugins)
	hostFront, hostBack, hostLink := host.Faces(card)
	if _, err := host.CreateCard(block); !errors.Is(err, errNoDerivation) {
		t.Errorf("a capability made a card with no card plugin: %v", err)
	}

	ln, err := serveAgent(lib, plugins, "secret")
	if err != nil {
		t.Fatalf("the API did not start without a card plugin: %v", err)
	}
	defer ln.Close()
	apiFront, apiBack, apiLink := getCardFaces(t, ln, card.ID)

	if hostFront != "what is spacing" || hostBack != "" || hostLink != "live" {
		t.Errorf("the host reads %q / %q (%s), want the block's words with the card's own edit", hostFront, hostBack, hostLink)
	}
	if apiFront != hostFront || apiBack != hostBack || apiLink != hostLink {
		t.Errorf("the API reads %q / %q (%s), the host %q / %q (%s): one fallback, not two",
			apiFront, apiBack, apiLink, hostFront, hostBack, hostLink)
	}
}

// cardOf answers the card a block has.
func cardOf(t *testing.T, lib *store.Library, block core.ID) core.Card {
	t.Helper()
	for _, card := range lib.Cards() {
		if card.Ref.Block == block {
			return card
		}
	}
	t.Fatalf("the block %q holds no card", block)
	return core.Card{}
}

// postCard makes one card through the loopback API, with the token the service
// wrote, and answers its id.
func postCard(t *testing.T, service *agentService, block core.ID) string {
	t.Helper()
	token, err := os.ReadFile(service.Status().TokenPath)
	if err != nil {
		t.Fatalf("read the token: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"block": string(block)})
	req, err := http.NewRequest(http.MethodPost,
		"http://"+service.Status().Address+"/v1/cards", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST card: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST card: %v", err)
	}
	defer resp.Body.Close()
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST card = %d (%+v, %v)", resp.StatusCode, created, err)
	}
	return created.ID
}

// getCardFaces reads a card's two sides through the loopback API.
func getCardFaces(t *testing.T, ln net.Listener, id core.ID) (front, back, link string) {
	t.Helper()
	resp, err := http.Get("http://" + ln.Addr().String() + "/v1/cards/" + string(id))
	if err != nil {
		t.Fatalf("GET card: %v", err)
	}
	defer resp.Body.Close()
	var card struct {
		Front string `json:"front"`
		Back  string `json:"back"`
		Link  string `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET card = %d (%v)", resp.StatusCode, err)
	}
	return card.Front, card.Back, card.Link
}

// installCards writes the third-party card plugin a test switches to: it derives
// one basic card per separator, tagged with tag, so a test can see which plugin
// made it.
func installCards(t *testing.T, root, name, id, version, tag string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	manifest := fmt.Sprintf(`{"id":%q,"name":%q,"version":%q,"apiVersion":1,"runtime":"js","entry":"main.js","implements":{"deriveBlock":1}}`,
		id, name, version)
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	source := fmt.Sprintf(`
export function deriveBlock(block) {
  const text = (block && block.text) || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{ typeId: "remit.basic", ordinal: 0,
    fields: { Front: %q + " " + text.slice(0, at).trim(), Back: text.slice(at + 2).trim() } }];
}
`, tag)
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatalf("entry: %v", err)
	}
}

// installCardWriter writes a plugin that makes its card through the capability it
// declares: the cards:write door, as a third-party plugin reaches it.
func installCardWriter(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("plugin dir: %v", err)
	}
	manifest := fmt.Sprintf(`{"id":"test.%s","name":%q,"version":"1.0.0","apiVersion":1,"runtime":"js","entry":"main.js","capabilities":["cards:write"],"implements":{"on_event":1}}`,
		name, name)
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	source := `
export function command(input) {
  const block = (input && input.context && input.context.block) || "";
  return { id: remit.cards.create(block).id };
}
export function on_event() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatalf("entry: %v", err)
	}
}
