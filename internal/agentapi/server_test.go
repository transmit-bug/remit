// The API's tests live outside the package: they build a library through the
// plugin host's side of the shared source, which imports the API package — a
// test in the package itself would be an import cycle.
package agentapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"errors"

	"github.com/transmit-bug/remit/internal/agentapi"
	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
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
	source := newPluginSource(t, lib)
	srv := httptest.NewServer(agentapi.New(source, agentapi.Options{
		Token: token,
		Stats: source,
		// The day's queue as the app builds it: the API reports the number of
		// lines internal/core/queue gives for the injected clock's moment.
		Today: func(at time.Time) (int, error) {
			today, err := queue.Entries(lib, at, queue.TodayOptions{})
			return len(today.Entries), err
		},
		Now: func() time.Time { return now },
	}).Handler())
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
	srv := httptest.NewServer(agentapi.New(newPluginSource(t, lib), agentapi.Options{Now: func() time.Time { return now }}).Handler())
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
	if created["front"] != "what is a cell" || created["back"] != "the unit of life" {
		t.Errorf("card faces = (%v, %v)", created["front"], created["back"])
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
	cards, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	card := cards[0]
	srv := newServer(t, lib)

	code, raw := do(t, http.MethodPatch, srv.URL+"/v1/cards/"+string(card.ID), token,
		map[string]any{"fields": map[string]string{"Front": "sharper question"}, "status": "suspended"})
	patched := object(t, raw)
	if code != http.StatusOK {
		t.Fatalf("PATCH = %d (%v)", code, patched)
	}
	fields, _ := patched["fields"].(map[string]any)
	if fields["Front"] != "sharper question" || patched["status"] != "suspended" {
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
	// Every known path answers an unsupported method with a 405: the endpoint
	// exists and the API deliberately has no such operation (no deletes).
	for _, target := range []struct{ method, path string }{
		{http.MethodDelete, "/v1/notes"},
		{http.MethodDelete, "/v1/cards"},
		{http.MethodPost, "/v1/search"},
		{http.MethodDelete, "/v1/types"},
		{http.MethodDelete, "/v1/notes/note_x/backlinks"},
		{http.MethodDelete, "/v1/stats/summary"},
		{http.MethodPost, "/v1/events"},
		{http.MethodPut, "/v1/schema"},
	} {
		if code, raw := do(t, target.method, srv.URL+target.path, token, nil); code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d (%s), want 405: the API has no such operation", target.method, target.path, code, raw)
		}
	}
}

func TestLoadOrCreateToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-token")

	first, err := agentapi.LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := agentapi.LoadOrCreateToken(path)
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
	first, err := agentapi.LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := agentapi.RotateToken(path)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if second == first {
		t.Error("rotation kept the old token")
	}
	again, err := agentapi.LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if again != second {
		t.Errorf("reload = %q, want the rotated token", again)
	}
}

// pluginSource is the library as the API sees it in these tests: a card is
// made and shaped through the shared resolver, as it is in the app. The API
// itself knows neither.
type pluginSource struct {
	*store.Library
	faces *cards.Resolver
}

func newPluginSource(t *testing.T, lib *store.Library) pluginSource {
	t.Helper()
	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the card plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return pluginSource{Library: lib, faces: cards.New(runner, lib.Types())}
}

func (s pluginSource) CreateCard(blockID core.ID) (core.Card, error) {
	block, ok := s.Block(blockID)
	if !ok {
		return core.Card{}, errNoSuchBlock
	}
	derived, ok := s.faces.Derive(block.Text)
	if !ok {
		return core.Card{}, errNoSuchBlock
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

func (s pluginSource) Faces(card core.Card) (front, back string, link string) {
	answer := s.faces.Face(card, s.Blocks())
	return answer.Front, answer.Back, answer.State.String()
}

// The statistics routes read the same library as every other route, through
// the package the statistics view reads.
func (s pluginSource) Summary(at time.Time) stats.Summary { return stats.Summarize(s.Library, at) }

func (s pluginSource) Forecast(from time.Time, days int) []stats.DayCount {
	return stats.ForecastAll(s.Library, from, days)
}

func (s pluginSource) Retention() stats.Retention {
	return stats.RetentionOf(s.Library, s.ScheduleParams())
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

// The reading queue is readable without a token and carries what a reader needs:
// which notes wait, where each was left, and the events that put it there.
func TestReadingIsReadableAndJoiningNeedsAToken(t *testing.T) {
	lib := newLibrary(t)
	note, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{
		{Text: "the cell :: the unit of life"},
		{Text: "the membrane :: what holds it together"},
	}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	srv := newServer(t, lib)

	// Joining the queue is a write, so it needs the token.
	if code, _ := do(t, http.MethodPost, srv.URL+"/v1/reading", "", map[string]any{"note": note.ID}); code != http.StatusUnauthorized {
		t.Fatalf("POST reading without token = %d, want 401", code)
	}
	code, raw := do(t, http.MethodPost, srv.URL+"/v1/reading", token, map[string]any{"note": note.ID})
	if code != http.StatusOK {
		t.Fatalf("POST reading = %d (%s), want 200", code, raw)
	}
	if status := object(t, raw)["status"]; status != "queued" {
		t.Errorf("status = %v, want queued", status)
	}

	code, raw = do(t, http.MethodGet, srv.URL+"/v1/reading", "", nil)
	if code != http.StatusOK {
		t.Fatalf("GET reading = %d", code)
	}
	entries := objects(t, raw)
	if len(entries) != 1 || entries[0]["note"] != string(note.ID) || entries[0]["title"] != "Biology" {
		t.Fatalf("reading queue = %v, want the note that joined", entries)
	}
	// Every item says when it comes up and that it is in no terminal state: the
	// three fields are there whether or not the answer is the interesting one.
	if due, ok := entries[0]["due"].(string); !ok || !dueAt(t, due).Equal(now) {
		t.Errorf("due = %v, want the moment it joined the queue", entries[0]["due"])
	}
	if entries[0]["archived"] != false || entries[0]["dismissed"] != false {
		t.Errorf("item = %v, want neither terminal state", entries[0])
	}

	// A session the window recorded shows in the entry: the difficulty and the
	// block the reading stopped at.
	if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[1].ID, now.Add(time.Hour)); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}
	code, raw = do(t, http.MethodGet, srv.URL+"/v1/reading/"+string(note.ID), "", nil)
	if code != http.StatusOK {
		t.Fatalf("GET reading entry = %d (%s)", code, raw)
	}
	entry := object(t, raw)
	if entry["position"] != string(note.Blocks[1].ID) {
		t.Errorf("position = %v, want the second paragraph", entry["position"])
	}
	if entry["reviews"] != float64(1) || entry["last_rating"] != "good" {
		t.Errorf("entry = %v, want one good session", entry)
	}
	events, _ := entry["events"].([]any)
	if len(events) != 2 {
		t.Fatalf("events = %v, want the join and the session", events)
	}
	first, _ := events[0].(map[string]any)
	if first["kind"] != "queued" {
		t.Errorf("first event = %v, want the join", first)
	}
	last, _ := events[1].(map[string]any)
	if last["kind"] != "reviewed" || last["rating"] != "good" || last["position"] != string(note.Blocks[1].ID) {
		t.Errorf("last event = %v, want the session", last)
	}

	// The API takes notes into the queue and nothing out of it.
	if code, _ := do(t, http.MethodDelete, srv.URL+"/v1/reading", token, nil); code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE reading = %d, want 405: removing is the window's", code)
	}
	if code, _ := do(t, http.MethodGet, srv.URL+"/v1/reading/"+string(core.NewID("note")), "", nil); code != http.StatusNotFound {
		t.Errorf("GET an unknown reading entry = %d, want 404", code)
	}

	// The library's own counts report both queues.
	code, raw = do(t, http.MethodGet, srv.URL+"/v1/library", "", nil)
	if code != http.StatusOK {
		t.Fatalf("GET library = %d", code)
	}
	if reading := object(t, raw)["reading"]; reading != float64(0) {
		t.Errorf("reading count = %v, want none due after the session", reading)
	}
}

// readingNote saves a note of paragraphs paragraphs, so a session can stop at
// any one of them.
func readingNote(t *testing.T, lib *store.Library, title string, paragraphs int) core.Note {
	t.Helper()
	blocks := make([]core.Block, paragraphs)
	for i := range blocks {
		blocks[i] = core.Block{Text: "paragraph " + strconv.Itoa(i+1)}
	}
	note, err := lib.SaveNote(core.Note{Title: title, Blocks: blocks})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	return note
}

// dueAt is a reading item's due time, which the API writes as RFC 3339.
func dueAt(t *testing.T, raw string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("due = %q, want RFC 3339: %v", raw, err)
	}
	return at
}

// The library's counts say what the first screen says: how many lines today
// holds, how many notes are in the archive, and how many were read with no
// excerpt taken from them yet.
func TestLibraryCountsTheDayTheArchiveAndTheExcerptsOwed(t *testing.T) {
	// read is a session that stopped at the first paragraph.
	read := func(t *testing.T, lib *store.Library, note core.Note) {
		t.Helper()
		if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, now); err != nil {
			t.Fatalf("ReadNote: %v", err)
		}
	}
	queued := func(t *testing.T, lib *store.Library, note core.Note) {
		t.Helper()
		if err := lib.AddToReading(note.ID, now); err != nil {
			t.Fatalf("AddToReading: %v", err)
		}
	}

	for _, tt := range []struct {
		name string
		// setup is the library the counts are asked about.
		setup func(t *testing.T, lib *store.Library)
		want  struct{ today, archived, unexcerpted int }
	}{
		{name: "an empty library"},
		{name: "a note waiting to read", setup: func(t *testing.T, lib *store.Library) {
			queued(t, lib, readingNote(t, lib, "Waiting", 2))
		}, want: struct{ today, archived, unexcerpted int }{today: 1}},
		{name: "a note read today", setup: func(t *testing.T, lib *store.Library) {
			note := readingNote(t, lib, "Read today", 2)
			queued(t, lib, note)
			read(t, lib, note)
		}, want: struct{ today, archived, unexcerpted int }{today: 1, unexcerpted: 1}},
		{name: "a note read and excerpted", setup: func(t *testing.T, lib *store.Library) {
			note := readingNote(t, lib, "Kept", 2)
			queued(t, lib, note)
			read(t, lib, note)
			if _, err := lib.Excerpt(note.ID, note.Blocks[0].ID, note.Blocks[0].Text, "Excerpt of Kept"); err != nil {
				t.Fatalf("Excerpt: %v", err)
			}
		}},
		{name: "a note archived", setup: func(t *testing.T, lib *store.Library) {
			note := readingNote(t, lib, "Archived", 2)
			queued(t, lib, note)
			read(t, lib, note)
			if err := lib.ArchiveReading(note.ID, now); err != nil {
				t.Fatalf("ArchiveReading: %v", err)
			}
		}, want: struct{ today, archived, unexcerpted int }{archived: 1}},
		{name: "a note dismissed", setup: func(t *testing.T, lib *store.Library) {
			note := readingNote(t, lib, "Dismissed", 2)
			queued(t, lib, note)
			read(t, lib, note)
			if err := lib.DismissReading(note.ID, now); err != nil {
				t.Fatalf("DismissReading: %v", err)
			}
		}, want: struct{ today, archived, unexcerpted int }{archived: 1}},
		{name: "a card due today", setup: func(t *testing.T, lib *store.Library) {
			note := readingNote(t, lib, "Cards", 1)
			if _, err := lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}}); err != nil {
				t.Fatalf("CreateCards: %v", err)
			}
		}, want: struct{ today, archived, unexcerpted int }{today: 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib := newLibrary(t)
			if tt.setup != nil {
				tt.setup(t, lib)
			}
			code, raw := do(t, http.MethodGet, newServer(t, lib).URL+"/v1/library", "", nil)
			if code != http.StatusOK {
				t.Fatalf("GET library = %d (%s)", code, raw)
			}
			got := object(t, raw)
			for _, field := range []struct {
				name string
				want int
			}{{"today", tt.want.today}, {"archived", tt.want.archived}, {"unexcerpted", tt.want.unexcerpted}} {
				if got[field.name] != float64(field.want) {
					t.Errorf("%s = %v, want %d", field.name, got[field.name], field.want)
				}
			}
		})
	}
}

// An item says when it comes up and whether it is in a terminal state, so a
// client can read a note that left the queue without guessing why it did.
func TestReadingItemsReportTheirDueAndTerminalState(t *testing.T) {
	for _, tt := range []struct {
		name string
		// then is what became of the note after the session.
		then      func(t *testing.T, lib *store.Library, note core.Note)
		archived  bool
		dismissed bool
	}{
		{name: "left in the queue", then: func(*testing.T, *store.Library, core.Note) {}},
		{name: "archived", then: func(t *testing.T, lib *store.Library, note core.Note) {
			if err := lib.ArchiveReading(note.ID, now); err != nil {
				t.Fatalf("ArchiveReading: %v", err)
			}
		}, archived: true},
		{name: "dismissed", then: func(t *testing.T, lib *store.Library, note core.Note) {
			if err := lib.DismissReading(note.ID, now); err != nil {
				t.Fatalf("DismissReading: %v", err)
			}
		}, dismissed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lib := newLibrary(t)
			note := readingNote(t, lib, "Long read", 2)
			if err := lib.AddToReading(note.ID, now); err != nil {
				t.Fatalf("AddToReading: %v", err)
			}
			if err := lib.ReadNote(note.ID, schedule.Good, note.Blocks[0].ID, now); err != nil {
				t.Fatalf("ReadNote: %v", err)
			}
			tt.then(t, lib, note)

			code, raw := do(t, http.MethodGet, newServer(t, lib).URL+"/v1/reading/"+string(note.ID), "", nil)
			if code != http.StatusOK {
				t.Fatalf("GET reading entry = %d (%s)", code, raw)
			}
			entry := object(t, raw)
			if entry["archived"] != tt.archived || entry["dismissed"] != tt.dismissed {
				t.Errorf("archived/dismissed = %v/%v, want %v/%v", entry["archived"], entry["dismissed"], tt.archived, tt.dismissed)
			}
			// The archive keeps the schedule the note had: its due date is still
			// the one the session gave it.
			due, ok := entry["due"].(string)
			if !ok {
				t.Fatalf("due = %v, want an RFC 3339 string", entry["due"])
			}
			if at := dueAt(t, due); !at.After(now) {
				t.Errorf("due = %s, want the date the good session set, after %s", at, now)
			}
		})
	}
}
