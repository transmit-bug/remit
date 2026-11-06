package agentapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"errors"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

var now = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

const token = "test-token"

func newLibrary(t *testing.T) *store.Library {
	t.Helper()
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return lib
}

func newServer(t *testing.T, lib *store.Library) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(New(newPluginSource(t, lib), Options{Token: token, Now: func() time.Time { return now }}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, method, url, tok string, body any) (int, []byte) {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, data
}

// object decodes one JSON object, failing the test when the body is not one.
func object(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode object from %s: %v", data, err)
	}
	return out
}

// objects decodes a JSON array of objects.
func objects(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode list from %s: %v", data, err)
	}
	return out
}

func TestReadsNeedNoTokenWritesDo(t *testing.T) {
	lib := newLibrary(t)
	srv := newServer(t, lib)

	if code, _ := do(t, http.MethodGet, srv.URL+"/v1/library", "", nil); code != http.StatusOK {
		t.Errorf("GET library without token = %d, want 200", code)
	}

	code, raw := do(t, http.MethodPost, srv.URL+"/v1/notes", "", map[string]any{"title": "x"})
	if code != http.StatusUnauthorized {
		t.Fatalf("POST note without token = %d, want 401", code)
	}
	if object(t, raw)["error"] == nil {
		t.Error("unauthorized response carries no error object")
	}

	if code, _ := do(t, http.MethodPost, srv.URL+"/v1/notes", token, map[string]any{"title": "x"}); code != http.StatusCreated {
		t.Errorf("POST note with token = %d, want 201", code)
	}
}

func TestNoTokenMakesWritesUnavailable(t *testing.T) {
	lib := newLibrary(t)
	srv := httptest.NewServer(New(newPluginSource(t, lib), Options{Now: func() time.Time { return now }}).Handler())
	defer srv.Close()

	if code, _ := do(t, http.MethodPost, srv.URL+"/v1/notes", "", map[string]any{"title": "x"}); code != http.StatusServiceUnavailable {
		t.Errorf("write with no configured token = %d, want 503", code)
	}
}

func TestCreateCardAndReview(t *testing.T) {
	lib := newLibrary(t)
	note, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{{Text: "what is a cell :: the unit of life"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	srv := newServer(t, lib)

	code, raw := do(t, http.MethodGet, srv.URL+"/v1/notes", "", nil)
	if code != http.StatusOK {
		t.Fatalf("GET notes = %d", code)
	}
	if n := len(objects(t, raw)); n == 0 {
		t.Fatal("GET notes returned nothing after a note was saved")
	}

	blockID := string(note.Blocks[0].ID)
	code, raw = do(t, http.MethodPost, srv.URL+"/v1/cards", token, map[string]any{"block": blockID})
	created := object(t, raw)
	if code != http.StatusCreated {
		t.Fatalf("POST card = %d (%v), want 201", code, created)
	}
	if created["prompt"] != "what is a cell" || created["answer"] != "the unit of life" {
		t.Errorf("card sides = (%v, %v)", created["prompt"], created["answer"])
	}
	if created["link"] != "live" {
		t.Errorf("link = %v, want live", created["link"])
	}

	cardID, _ := created["id"].(string)
	code, raw = do(t, http.MethodPost, srv.URL+"/v1/cards/"+cardID+"/reviews", token, map[string]any{"rating": "good"})
	reviewed := object(t, raw)
	if code != http.StatusOK {
		t.Fatalf("POST review = %d (%v)", code, reviewed)
	}
	if reviewed["due"] == nil {
		t.Error("review did not set a due date")
	}
	if n, _ := reviewed["reviews"].(float64); n != 1 {
		t.Errorf("reviews = %v, want 1", reviewed["reviews"])
	}

	code, raw = do(t, http.MethodPost, srv.URL+"/v1/cards/"+cardID+"/reviews", token, map[string]any{"rating": "nope"})
	bad := object(t, raw)
	if code != http.StatusBadRequest || bad["error"] == nil {
		t.Errorf("bad rating = %d (%v), want 400 with an error", code, bad)
	}
}

func TestPatchIsLimitedToWordingAndStatus(t *testing.T) {
	lib := newLibrary(t)
	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	cards, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Kind: core.KindBasic}})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	card := cards[0]
	srv := newServer(t, lib)

	code, raw := do(t, http.MethodPatch, srv.URL+"/v1/cards/"+string(card.ID), token,
		map[string]any{"prompt": "sharper question", "status": "suspended"})
	patched := object(t, raw)
	if code != http.StatusOK {
		t.Fatalf("PATCH = %d (%v)", code, patched)
	}
	if patched["prompt"] != "sharper question" || patched["status"] != "suspended" {
		t.Errorf("patched card = %v", patched)
	}

	// The queue is a read, and a suspended card is not in it.
	code, raw = do(t, http.MethodGet, srv.URL+"/v1/queue", "", nil)
	queue := objects(t, raw)
	if code != http.StatusOK {
		t.Fatalf("GET queue = %d", code)
	}
	if len(queue) != 0 {
		t.Errorf("queue has %d cards after suspension: %v", len(queue), queue)
	}
}

func TestUnknownRoutesAndMethods(t *testing.T) {
	lib := newLibrary(t)
	srv := newServer(t, lib)

	if code, raw := do(t, http.MethodGet, srv.URL+"/v2/nothing", "", nil); code != http.StatusNotFound || object(t, raw)["error"] == nil {
		t.Errorf("unknown path = %d (%s), want 404 with an error", code, raw)
	}
	if code, _ := do(t, http.MethodDelete, srv.URL+"/v1/notes", token, nil); code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE notes = %d, want 405: deletion is not part of the API", code)
	}
}

func TestLoadOrCreateToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-token")

	first, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first == "" || first != second {
		t.Errorf("token not stable: %q then %q", first, second)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token mode = %o, want 600", perm)
	}
}

func TestScheduleParamsKeepDefaults(t *testing.T) {
	// A library's parameters are the caller's, not the API's: the server never
	// changes them, so a default library still schedules.
	if got := schedule.DefaultParams().DesiredRetention; got != schedule.DefaultRetention {
		t.Errorf("desired retention = %v, want %v", got, schedule.DefaultRetention)
	}
}

func TestRotateTokenReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-token")
	first, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := RotateToken(path)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if second == first {
		t.Error("rotation kept the old token")
	}
	again, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if again != second {
		t.Errorf("reload = %q, want the rotated token", again)
	}
}

// pluginSource is the library as the API sees it in these tests: a card is made
// and shaped by the card plugin, as it is in the app. The API itself knows
// neither.
type pluginSource struct {
	*store.Library
	plugin *ext.JSRunner
}

func newPluginSource(t *testing.T, lib *store.Library) pluginSource {
	t.Helper()
	runner, err := builtin.Cards(ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the card plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return pluginSource{Library: lib, plugin: runner}
}

func (s pluginSource) CreateCard(blockID core.ID) (core.Card, error) {
	block, ok := s.Block(blockID)
	if !ok {
		return core.Card{}, errNoSuchBlock
	}
	var answered []struct {
		Kind    string `json:"kind"`
		Ordinal int    `json:"ordinal"`
	}
	if err := s.plugin.Call("derive", map[string]any{"text": block.Text}, &answered); err != nil {
		return core.Card{}, err
	}
	derived := make([]core.Derived, 0, len(answered))
	for _, card := range answered {
		kind, ok := core.ParseKind(card.Kind)
		if !ok {
			continue
		}
		derived = append(derived, core.Derived{Kind: kind, Ordinal: card.Ordinal})
	}
	cards, err := s.Library.CreateCards(blockID, derived)
	if err != nil {
		return core.Card{}, err
	}
	if len(cards) == 0 {
		return core.Card{}, errNoSuchBlock
	}
	return cards[0], nil
}

func (s pluginSource) Faces(card core.Card) (prompt, answer, link string) {
	text, state := card.Ref.Text(s.Blocks())
	prompt, answer = text, ""
	var answered struct {
		Front string `json:"front"`
		Back  string `json:"back"`
	}
	params := map[string]any{"text": text, "kind": card.Kind.String(), "ordinal": card.Ordinal}
	if err := s.plugin.Call("faces", params, &answered); err == nil {
		prompt, answer = answered.Front, answered.Back
	}
	if card.Prompt != "" {
		prompt = card.Prompt
	}
	if card.Answer != "" {
		answer = card.Answer
	}
	return prompt, answer, state.String()
}

// Blocks is the library's blocks as the model reads them.
func (s pluginSource) Blocks() core.BlockSet {
	blocks := core.BlockSet{}
	for _, note := range s.Notes() {
		for _, block := range note.Blocks {
			blocks[block.ID] = block
		}
	}
	return blocks
}

var errNoSuchBlock = errors.New("agentapi: the block asks nothing")
