package agentapi_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/agentapi"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// get reads a route without a token, answering the status, the content type and
// the body.
func get(t *testing.T, url string) (int, string, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), body
}

// readFixture is the library the read routes answer for: two notes that link to
// each other, a tag on a block that feeds a reviewed card, a card that has never
// been reviewed, and a second answer on the first card so retention has a
// prediction to compare against.
type readFixture struct {
	lib      *store.Library
	biology  core.Note
	genetics core.Note
	reviewed core.Card
	queued   core.Card
}

func readLibrary(t *testing.T) readFixture {
	t.Helper()
	lib := newLibrary(t)
	biology, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{
		{Text: "the cell :: the unit of life #science"},
		{Text: "see [[Genetics]] for the rest"},
	}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	genetics, err := lib.SaveNote(core.Note{Title: "Genetics", Blocks: []core.Block{
		{Text: "genes :: the unit of heredity"},
		{Text: "back to [[Biology]]"},
	}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}

	reviewed, err := lib.CreateCards(biology.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	queued, err := lib.CreateCards(genetics.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	for _, at := range []time.Time{now, now.Add(time.Hour)} {
		if err := lib.Review(reviewed[0].ID, schedule.Good, at); err != nil {
			t.Fatalf("review: %v", err)
		}
	}
	return readFixture{lib: lib, biology: biology, genetics: genetics, reviewed: reviewed[0], queued: queued[0]}
}

// Every read route answers JSON without a token, with the shape the API
// documents: the same library the window shows, read-only.
func TestReadRoutesAnswerJSON(t *testing.T) {
	fixture := readLibrary(t)
	srv := newServer(t, fixture.lib)

	const jsonType = "application/json; charset=utf-8"
	tests := []struct {
		name   string
		path   string
		status int
		check  func(t *testing.T, body []byte)
	}{
		{
			name: "schema",
			path: "/v1/schema",
			check: func(t *testing.T, body []byte) {
				got := object(t, body)
				if got["api"] != "v1" || got["format"] != store.FormatVersion {
					t.Errorf("schema = %v, want api v1 and format %s", got, store.FormatVersion)
				}
				listed := map[string]bool{}
				routes, _ := got["routes"].([]any)
				for _, entry := range routes {
					listed[entry.(string)] = true
				}
				for _, want := range []string{"GET /v1/search", "GET /v1/events", "GET /v1/schema", "POST /v1/notes"} {
					if !listed[want] {
						t.Errorf("the schema does not list %q: %v", want, routes)
					}
				}
			},
		},
		{
			name: "library",
			path: "/v1/library",
			check: func(t *testing.T, body []byte) {
				got := object(t, body)
				if got["format"] != store.FormatVersion || got["notes"] != float64(2) || got["cards"] != float64(2) {
					t.Errorf("library = %v, want two notes and two cards", got)
				}
			},
		},
		{
			name: "notes",
			path: "/v1/notes",
			check: func(t *testing.T, body []byte) {
				if got := objects(t, body); len(got) != 2 || got[0]["title"] == nil {
					t.Errorf("notes = %v, want the two seeded notes", got)
				}
			},
		},
		{
			name: "note",
			path: "/v1/notes/" + string(fixture.biology.ID),
			check: func(t *testing.T, body []byte) {
				got := object(t, body)
				if got["title"] != "Biology" || len(got["blocks"].([]any)) != 2 {
					t.Errorf("note = %v, want Biology and its two blocks", got)
				}
			},
		},
		{
			name: "backlinks",
			path: "/v1/notes/" + string(fixture.biology.ID) + "/backlinks",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				if len(got) != 1 {
					t.Fatalf("backlinks = %v, want the one paragraph naming Biology", got)
				}
				link := got[0]
				if link["note"] != string(fixture.genetics.ID) || link["title"] != "Genetics" || link["paragraph"] != float64(2) {
					t.Errorf("backlink = %v, want Genetics' second paragraph", link)
				}
				if link["text"] != "back to [[Biology]]" || link["block"] == "" {
					t.Errorf("backlink = %v, want its text and block", link)
				}
			},
		},
		{
			name: "cards",
			path: "/v1/cards",
			check: func(t *testing.T, body []byte) {
				if got := objects(t, body); len(got) != 2 || got[0]["type"] != string(core.TypeBasic) {
					t.Errorf("cards = %v, want the two basic cards", got)
				}
			},
		},
		{
			name: "card",
			path: "/v1/cards/" + string(fixture.reviewed.ID),
			check: func(t *testing.T, body []byte) {
				got := object(t, body)
				if got["id"] != string(fixture.reviewed.ID) || got["status"] != "review" || got["reviews"] != float64(2) {
					t.Errorf("card = %v, want the reviewed card", got)
				}
			},
		},
		{
			name: "queue",
			path: "/v1/queue",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				if len(got) != 1 || got[0]["id"] != string(fixture.queued.ID) {
					t.Errorf("queue = %v, want the card that has never been reviewed", got)
				}
			},
		},
		{
			name: "search over notes and cards",
			path: "/v1/search?q=cell",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				if len(got) != 2 {
					t.Fatalf("search = %v, want the note and the card", got)
				}
				if got[0]["kind"] != "note" || got[0]["id"] != string(fixture.biology.ID) || got[0]["title"] != "Biology" {
					t.Errorf("first hit = %v, want the Biology note", got[0])
				}
				if got[0]["snippet"] != "the cell :: the unit of life #science" || got[0]["block"] != string(fixture.biology.Blocks[0].ID) {
					t.Errorf("first hit = %v, want the matching block's text", got[0])
				}
				if got[1]["kind"] != "card" || got[1]["id"] != string(fixture.reviewed.ID) {
					t.Errorf("second hit = %v, want the card the block feeds", got[1])
				}
				if got[1]["note"] != string(fixture.biology.ID) || got[1]["title"] != "Biology" {
					t.Errorf("card hit = %v, want the note it lives in", got[1])
				}
			},
		},
		{
			name: "search notes only",
			path: "/v1/search?q=cell&kind=note",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				if len(got) != 1 || got[0]["kind"] != "note" {
					t.Errorf("search = %v, want only the note", got)
				}
			},
		},
		{
			name: "search cards only",
			path: "/v1/search?q=cell&kind=card",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				if len(got) != 1 || got[0]["kind"] != "card" {
					t.Errorf("search = %v, want only the card", got)
				}
			},
		},
		{
			name: "search paged over both kinds",
			path: "/v1/search?q=cell&limit=1&offset=1",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				if len(got) != 1 || got[0]["kind"] != "card" {
					t.Errorf("page = %v, want the card, which is second across the two lists", got)
				}
			},
		},
		{
			name: "tags",
			path: "/v1/tags",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				if len(got) != 1 {
					t.Fatalf("tags = %v, want the one tag", got)
				}
				if got[0]["tag"] != "science" || got[0]["notes"] != float64(1) || got[0]["cards"] != float64(1) {
					t.Errorf("tag = %v, want science on one note and one card", got[0])
				}
			},
		},
		{
			name: "types",
			path: "/v1/types",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				if len(got) != len(core.BuiltinTypes()) {
					t.Fatalf("types = %v, want the three built-ins", got)
				}
				basic := got[0]
				if basic["id"] != string(core.TypeBasic) || basic["name"] != "Basic" {
					t.Fatalf("first type = %v, want Basic", basic)
				}
				templates, _ := basic["templates"].(map[string]any)
				if templates["front"] != "{{Front}}" || templates["back"] != "{{Back}}" {
					t.Errorf("templates = %v, want the sources as stored", templates)
				}
				if fields, _ := basic["fields"].([]any); len(fields) != 2 || fields[0] != "Front" {
					t.Errorf("fields = %v, want the ordered field list", basic["fields"])
				}
			},
		},
		{
			name: "one type",
			path: "/v1/types/" + string(core.TypeBasic),
			check: func(t *testing.T, body []byte) {
				if got := object(t, body); got["id"] != string(core.TypeBasic) || got["version"] == nil {
					t.Errorf("type = %v, want Basic with its version", got)
				}
			},
		},
		{
			name: "stats summary",
			path: "/v1/stats/summary",
			check: func(t *testing.T, body []byte) {
				got := object(t, body)
				if got["reviewed"] != float64(2) || got["accuracy"] != float64(1) || got["due_tomorrow"] == nil {
					t.Errorf("summary = %v, want today's two right answers", got)
				}
			},
		},
		{
			name: "stats forecast",
			path: "/v1/stats/forecast?days=30",
			check: func(t *testing.T, body []byte) {
				got := objects(t, body)
				sum := 0
				for _, day := range got {
					if day["day"] == nil {
						t.Fatalf("forecast = %v, want a day per row", got)
					}
					sum += int(day["count"].(float64))
				}
				if sum < 1 {
					t.Errorf("forecast = %v, want the reviewed card's next appearance", got)
				}
			},
		},
		{
			name: "stats retention",
			path: "/v1/stats/retention",
			check: func(t *testing.T, body []byte) {
				got := object(t, body)
				if got["reviews"] != float64(1) || got["passes"] != float64(1) || got["accuracy"] != float64(1) {
					t.Errorf("retention = %v, want the one prediction that came out right", got)
				}
				if buckets, _ := got["buckets"].([]any); len(buckets) != 4 {
					t.Errorf("buckets = %v, want the four ranges", got["buckets"])
				}
			},
		},
		{
			name: "reading",
			path: "/v1/reading",
			check: func(t *testing.T, body []byte) {
				if got := objects(t, body); len(got) != 0 {
					t.Errorf("reading = %v, want an empty queue, not null", got)
				}
			},
		},
		{
			name:   "search without a query",
			path:   "/v1/search",
			status: http.StatusBadRequest,
			check:  wantError("no_query"),
		},
		{
			name:   "search with an unknown kind",
			path:   "/v1/search?q=cell&kind=video",
			status: http.StatusBadRequest,
			check:  wantError("bad_kind"),
		},
		{
			name:   "search with a negative limit",
			path:   "/v1/search?q=cell&limit=-1",
			status: http.StatusBadRequest,
			check:  wantError("bad_limit"),
		},
		{
			name:   "forecast with a negative window",
			path:   "/v1/stats/forecast?days=-1",
			status: http.StatusBadRequest,
			check:  wantError("bad_days"),
		},
		{
			name:   "an unknown type",
			path:   "/v1/types/remit.nothing",
			status: http.StatusNotFound,
			check:  wantError("not_found"),
		},
		{
			name:   "backlinks of an unknown note",
			path:   "/v1/notes/note_nothing/backlinks",
			status: http.StatusNotFound,
			check:  wantError("not_found"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status := tc.status
			if status == 0 {
				status = http.StatusOK
			}
			code, contentType, body := get(t, srv.URL+tc.path)
			if code != status {
				t.Fatalf("GET %s = %d (%s), want %d", tc.path, code, body, status)
			}
			if contentType != jsonType {
				t.Errorf("content type = %q, want %q", contentType, jsonType)
			}
			tc.check(t, body)
		})
	}
}

// wantError checks the error object a refusal carries.
func wantError(code string) func(t *testing.T, body []byte) {
	return func(t *testing.T, body []byte) {
		t.Helper()
		failure, _ := object(t, body)["error"].(map[string]any)
		if failure["code"] != code || failure["message"] == "" {
			t.Errorf("error = %v, want code %q with a message", failure, code)
		}
	}
}

// The write routes are the only ones gated by the token, and every one the
// schema advertises refuses an anonymous request: a new route cannot slip in
// without the gate it is supposed to carry.
func TestEveryAdvertisedWriteNeedsTheToken(t *testing.T) {
	srv := newServer(t, newLibrary(t))

	_, _, body := get(t, srv.URL+"/v1/schema")
	routes, _ := object(t, body)["routes"].([]any)
	writes := 0
	for _, entry := range routes {
		line, _ := entry.(string)
		method, path, ok := strings.Cut(line, " ")
		if !ok || method == http.MethodGet {
			continue
		}
		writes++
		url := srv.URL + strings.ReplaceAll(path, "{id}", "card_missing")
		if code, raw := do(t, method, url, "", map[string]any{}); code != http.StatusUnauthorized {
			t.Errorf("%s without a token = %d (%s), want 401", line, code, raw)
		}
	}
	if writes == 0 {
		t.Fatal("the schema advertises no writes: the token gate is untested")
	}
}

// The event stream is a read over the library's writes: one event arrives as it
// happens, and the stream ends where the request does.
func TestEventsStreamDeliversAndEnds(t *testing.T) {
	lib := newLibrary(t)
	srv := newServer(t, lib)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/events", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect to the stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET events = %d, want 200", resp.StatusCode)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("content type = %q, want text/event-stream", contentType)
	}

	frames := make(chan string, 8)
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			frames <- scanner.Text()
		}
	}()

	code, raw := do(t, http.MethodPost, srv.URL+"/v1/notes", token, map[string]any{"title": "streamed"})
	if code != http.StatusCreated {
		t.Fatalf("POST note = %d (%s), want 201", code, raw)
	}
	wrote := object(t, raw)["id"]

	var frame string
	select {
	case frame = <-frames:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream delivered nothing after a write")
	}
	payload, ok := strings.CutPrefix(frame, "data: ")
	if !ok {
		t.Fatalf("frame = %q, want a data line", frame)
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatalf("decode %q: %v", payload, err)
	}
	if event["kind"] != store.EventNoteSaved || event["id"] != wrote {
		t.Errorf("event = %v, want the saved note", event)
	}

	// The stream is the request's: cancelling it ends the handler, which is
	// what closes the body.
	stop()
	for {
		select {
		case _, open := <-frames:
			if !open {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the stream outlived the request that opened it")
		}
	}
}

// A server built without a statistics provider says so rather than answering
// zeros that would read like a library with no work behind it.
func TestStatisticsWithoutAProviderAreUnavailable(t *testing.T) {
	lib := newLibrary(t)
	srv := httptest.NewServer(agentapi.New(newPluginSource(t, lib), agentapi.Options{Now: func() time.Time { return now }}).Handler())
	defer srv.Close()

	code, contentType, body := get(t, srv.URL+"/v1/stats/summary")
	if code != http.StatusNotImplemented {
		t.Fatalf("GET summary = %d (%s), want 501", code, body)
	}
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("content type = %q, want JSON", contentType)
	}
	wantError("stats_unavailable")(t, body)
}
