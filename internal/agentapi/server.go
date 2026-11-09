// Package agentapi exposes a library to external agents over loopback HTTP.
//
// Reads need no credentials: the data is the user's own, and a local process
// can read the files anyway. They answer with what the window shows — notes and
// cards, search hits, tags and card types, backlinks, the statistics the views
// plot and a stream of the events that change them — and the schema route lists
// them so a client can feature-detect instead of guessing.
//
// Writes need a bearer token, and the API can only add — creating notes, cards
// and reviews, or changing a card's wording and status. It cannot delete a
// card, rewrite review history or change FSRS parameters, so a buggy agent
// cannot destroy a schedule (decision #3).
package agentapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Library is the part of a library the API reads and writes. It is also the
// layer the plugin host shares (ext.Host embeds it): one declaration of what a
// library can be read and written by, for the loopback API and for plugins
// alike. History is here for the plugins that read a card's log; the HTTP
// surface answers with the state that log replays to.
type Library interface {
	Notes() []core.Note
	Note(core.ID) (core.Note, bool)
	SaveNote(core.Note) (core.Note, error)
	CreateNote(string) (core.Note, error)
	SearchNotes(string) ([]core.Note, error)
	SearchCards(string) ([]core.Card, error)
	Cards() []core.Card
	Card(core.ID) (core.Card, bool)
	CreateCard(core.ID) (core.Card, error)
	SaveCard(core.Card) error
	Review(core.ID, schedule.Rating, time.Time) error
	CardState(core.ID) (schedule.State, bool)
	History(core.ID) ([]schedule.Event, error)
	Due(time.Time, int) ([]core.Card, error)
	Remaining(time.Time) (int, error)
	Block(core.ID) (core.Block, bool)
	// NoteOfBlock names the note a block lives in: a search hit is shown where
	// it can be opened.
	NoteOfBlock(core.ID) (core.ID, bool)
	// Types are the card schemas the library holds, as its files store them, and
	// Tags is what the library is tagged with.
	Types() []core.Type
	Type(core.ID) (core.Type, bool)
	Tags() []store.TagCount
	// Backlinks are the paragraphs that name a note.
	Backlinks(core.ID) []store.Backlink
	// The reading queue: what is due to be read, where each note was left, and
	// the way into it. Adding is a write; removing and recording a position
	// are the window's alone.
	ReadingQueue() ([]store.Reading, error)
	ReadingFor(core.ID) (store.Reading, bool)
	ReadingEvents(core.ID) ([]schedule.Event, error)
	AddToReading(core.ID, time.Time) error
	// The two places a reading note can be besides the queue, which the
	// library's own counts report: the archive the terminal states sit in, and
	// the notes read with no excerpt taken from them yet.
	ReadingArchive() ([]store.Reading, error)
	UnExcerptedNotes(limit int) ([]store.Reading, error)
	// Faces is what a card asks, as the type's templates render it — this
	// package has no splitting rules of its own.
	Faces(core.Card) (front, back string, link string)
	// Subscribe tells fn about every library event from now on, until the
	// returned function stops it. Nothing is replayed, and a subscriber that
	// falls behind is dropped rather than slowing a write.
	Subscribe(func(store.Event)) (cancel func())
}

// Stats is what the statistics routes answer: the log and the schedule read as
// one picture. It is wired beside the library rather than declared on it,
// because these numbers are derived from the store by internal/core/stats and
// are nowhere stored; a server built without one answers 501 instead of zeros,
// so a missing wiring shows rather than reading like an empty library.
type Stats interface {
	Summary(time.Time) stats.Summary
	Forecast(time.Time, int) []stats.DayCount
	Retention() stats.Retention
}

// apiVersion is the shape of this HTTP surface, which is also its first path
// segment and what its schema reports. A client that finds another one should
// stop rather than guess.
const apiVersion = "v1"

const (
	// defaultSearchLimit is how many hits a search returns when the caller names
	// no limit; an explicit limit of zero returns every hit.
	defaultSearchLimit = 50
	// defaultForecastDays is the window the forecast covers when the caller
	// names no days.
	defaultForecastDays = 30
	// eventQueue is how many events the stream buffers before it stops reading,
	// which is what lets a slow client fall behind without holding up a write.
	eventQueue = 32
	// heartbeatEvery is how often an idle stream says it is still there, so a
	// client and any proxy between know the connection is alive.
	heartbeatEvery = 20 * time.Second
)

// route is one endpoint of the API: what the handler registers, what the schema
// reports and whether a token gates it, from one list so the three cannot
// drift.
type route struct {
	method  string
	path    string
	handler http.HandlerFunc
	// write marks a route that needs the bearer token. Every other route is a
	// read, and reads stay open on loopback.
	write bool
}

// routes is the API's surface, in the order the schema lists it.
func (s *Server) routes() []route {
	return []route{
		{"GET", "/v1/schema", s.getSchema, false},
		{"GET", "/v1/library", s.getLibrary, false},
		{"GET", "/v1/notes", s.getNotes, false},
		{"GET", "/v1/notes/{id}", s.getNote, false},
		{"GET", "/v1/notes/{id}/backlinks", s.getBacklinks, false},
		{"GET", "/v1/cards", s.getCards, false},
		{"GET", "/v1/cards/{id}", s.getCard, false},
		{"GET", "/v1/queue", s.getQueue, false},
		{"GET", "/v1/search", s.getSearch, false},
		{"GET", "/v1/tags", s.getTags, false},
		{"GET", "/v1/types", s.getTypes, false},
		{"GET", "/v1/types/{id}", s.getType, false},
		{"GET", "/v1/stats/summary", s.getStatsSummary, false},
		{"GET", "/v1/stats/forecast", s.getStatsForecast, false},
		{"GET", "/v1/stats/retention", s.getStatsRetention, false},
		{"GET", "/v1/events", s.getEvents, false},
		{"GET", "/v1/reading", s.getReading, false},
		{"GET", "/v1/reading/{id}", s.getReadingEntry, false},
		{"POST", "/v1/notes", s.postNote, true},
		{"POST", "/v1/cards", s.postCard, true},
		{"POST", "/v1/reading", s.postReading, true},
		{"POST", "/v1/cards/{id}/reviews", s.postReview, true},
		{"PATCH", "/v1/cards/{id}", s.patchCard, true},
	}
}

// Options configures a Server.
type Options struct {
	// Token gates every write. An empty token makes the server read-only,
	// which is the safer failure: no token, no writable API.
	Token string
	// Stats answers the statistics routes, from the same library the rest of
	// the API serves. Without one they answer 501.
	Stats Stats
	// Today counts the lines today's queue holds — the notes due to read, the
	// excerpts owed and the review line — as internal/core/queue builds them for
	// that moment. It arrives as a function rather than as a Library method
	// because the queue is built over the concrete library, which that interface
	// does not name. A server built without one reports today as zero, like a
	// count whose library refused the question.
	Today func(time.Time) (int, error)
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Server serves the API for one library.
type Server struct {
	lib  Library
	opts Options
}

// New returns a server for lib.
func New(lib Library, opts Options) *Server {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Server{lib: lib, opts: opts}
}

// Serve serves the API on ln until the listener is closed.
func (s *Server) Serve(ln net.Listener) error {
	return http.Serve(ln, s.Handler())
}

// Handler returns the API's routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	known := make(map[string]bool)
	for _, rt := range s.routes() {
		handler := rt.handler
		if rt.write {
			handler = s.requireToken(handler)
		}
		mux.HandleFunc(rt.method+" "+rt.path, handler)
		known[rt.path] = true
	}
	// A known path with another method is a 405, not a 404: the endpoint exists
	// and the API deliberately has no such operation (no deletes).
	for path := range known {
		mux.HandleFunc(path, methodNotAllowed)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})
	return mux
}

// requireToken rejects a write without the bearer token. An unset token makes
// every write refused rather than open.
func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.opts.Token == "" {
			writeError(w, http.StatusServiceUnavailable, "writes_disabled", "this library has no agent token")
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" || got != s.opts.Token {
			writeError(w, http.StatusUnauthorized, "unauthorized", "a bearer token is required for writes")
			return
		}
		next(w, r)
	}
}

func (s *Server) getLibrary(w http.ResponseWriter, r *http.Request) {
	notes := s.lib.Notes()
	now := s.opts.Now()
	var due, reading, today, archived, unexcerpted int
	if n, err := s.lib.Remaining(now); err == nil {
		due = n
	}
	if queue, err := s.lib.ReadingQueue(); err == nil {
		for _, entry := range queue {
			if entry.State.Reviews == 0 || !entry.State.Due.After(now) {
				reading++
			}
		}
	}
	if s.opts.Today != nil {
		if n, err := s.opts.Today(now); err == nil {
			today = n
		}
	}
	if rows, err := s.lib.ReadingArchive(); err == nil {
		archived = len(rows)
	}
	if rows, err := s.lib.UnExcerptedNotes(0); err == nil {
		unexcerpted = len(rows)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"format":      store.FormatVersion,
		"notes":       len(notes),
		"cards":       len(s.lib.Cards()),
		"due":         due,
		"reading":     reading,
		"today":       today,
		"archived":    archived,
		"unexcerpted": unexcerpted,
	})
}

func (s *Server) getNotes(w http.ResponseWriter, r *http.Request) {
	notes := s.lib.Notes()
	out := make([]noteJSON, 0, len(notes))
	for _, n := range notes {
		out = append(out, newNoteJSON(n))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getNote(w http.ResponseWriter, r *http.Request) {
	n, ok := s.lib.Note(core.ID(r.PathValue("id")))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "no such note")
		return
	}
	writeJSON(w, http.StatusOK, newNoteJSON(n))
}

func (s *Server) getCards(w http.ResponseWriter, r *http.Request) {
	cards := s.lib.Cards()
	out := make([]cardJSON, 0, len(cards))
	for _, c := range cards {
		out = append(out, s.newCardJSON(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getCard(w http.ResponseWriter, r *http.Request) {
	c, ok := s.lib.Card(core.ID(r.PathValue("id")))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "no such card")
		return
	}
	writeJSON(w, http.StatusOK, s.newCardJSON(c))
}

func (s *Server) getQueue(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryInt(w, r, "limit", 0)
	if !ok {
		return
	}
	cards, err := s.lib.Due(s.opts.Now(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "queue_failed", err.Error())
		return
	}
	out := make([]cardJSON, 0, len(cards))
	for _, c := range cards {
		out = append(out, s.newCardJSON(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// getSearch answers a text search over the library. Notes come first, newest
// save first as the notebook lists them, then cards; offset and limit page the
// two together, so a client walks one list rather than merging two.
func (s *Server) getSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "no_query", "q is the text to find")
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "note" && kind != "card" {
		writeError(w, http.StatusBadRequest, "bad_kind", "kind must be note or card")
		return
	}
	limit, ok := queryInt(w, r, "limit", defaultSearchLimit)
	if !ok {
		return
	}
	offset, ok := queryInt(w, r, "offset", 0)
	if !ok {
		return
	}

	var notes []core.Note
	var cards []core.Card
	if kind != "card" {
		found, err := s.lib.SearchNotes(query)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		notes = found
	}
	if kind != "note" {
		found, err := s.lib.SearchCards(query)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		cards = found
	}

	// The page is taken over the two lists as one, and only the rows on it are
	// filled in: a note hit reads its own blocks, a card hit reads its note.
	seen := len(notes) + len(cards)
	if limit > 0 && limit < seen {
		seen = limit
	}
	out := make([]searchJSON, 0, seen)
	page := func(at int) bool {
		return at >= offset && (limit <= 0 || at < offset+limit)
	}
	for i, note := range notes {
		if page(i) {
			out = append(out, newNoteSearchJSON(note, query))
		}
	}
	for i, card := range cards {
		if page(len(notes) + i) {
			out = append(out, s.newCardSearchJSON(card, query))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// searchJSON is one hit: what was found, what kind of thing it is, and where
// the matching text is. A note has a title of its own; a card has none, so it
// carries the title of the note its block lives in.
type searchJSON struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Note    string `json:"note,omitempty"`
	Block   string `json:"block,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

func newNoteSearchJSON(note core.Note, query string) searchJSON {
	out := searchJSON{ID: string(note.ID), Kind: "note", Title: note.Title, Note: string(note.ID)}
	if hit, ok := matchedBlock(note, query); ok {
		out.Block = string(hit.ID)
		out.Snippet = hit.Text
	}
	return out
}

// matchedBlock is the block a note's hit came from: the first one whose text
// carries the query, or the first block when none does — SQLite's LIKE folds
// case more narrowly than Go's does, and a hit is never shown without its text.
func matchedBlock(note core.Note, query string) (core.Block, bool) {
	if len(note.Blocks) == 0 {
		return core.Block{}, false
	}
	for _, block := range note.Blocks {
		if containsFold(block.Text, query) {
			return block, true
		}
	}
	return note.Blocks[0], true
}

// newCardSearchJSON is a card hit: the block it renders, the note that block
// lives in, and the text that matched — the block, or a field value the user
// edited over it.
func (s *Server) newCardSearchJSON(card core.Card, query string) searchJSON {
	out := searchJSON{ID: string(card.ID), Kind: "card", Block: string(card.Ref.Block)}
	if noteID, ok := s.lib.NoteOfBlock(card.Ref.Block); ok {
		out.Note = string(noteID)
		if note, ok := s.lib.Note(noteID); ok {
			out.Title = note.Title
		}
	}
	if block, ok := s.lib.Block(card.Ref.Block); ok {
		out.Snippet = block.Text
		if containsFold(block.Text, query) {
			return out
		}
	}
	if value, ok := matchedField(card.Fields, query); ok {
		out.Snippet = value
	}
	return out
}

// matchedField is the first edited field value carrying the query, by field
// name so two searches agree which value they show.
func matchedField(fields map[string]string, query string) (string, bool) {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if containsFold(fields[name], query) {
			return fields[name], true
		}
	}
	return "", false
}

// containsFold reports whether text holds query, ignoring case.
func containsFold(text, query string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(query))
}

// tagJSON is one tag with how much of the library carries it.
type tagJSON struct {
	Tag   string `json:"tag"`
	Notes int    `json:"notes"`
	Cards int    `json:"cards"`
}

func (s *Server) getTags(w http.ResponseWriter, r *http.Request) {
	counts := s.lib.Tags()
	out := make([]tagJSON, 0, len(counts))
	for _, count := range counts {
		out = append(out, tagJSON{Tag: count.Tag, Notes: count.Notes, Cards: count.Cards})
	}
	writeJSON(w, http.StatusOK, out)
}

// typeJSON is a card type as the library stores it: the fields a card fills, in
// order, and the template sources the host renders.
type typeJSON struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Fields    []string          `json:"fields"`
	Templates typeTemplatesJSON `json:"templates"`
	Version   int               `json:"version"`
}

type typeTemplatesJSON struct {
	Front string `json:"front"`
	Back  string `json:"back"`
}

func newTypeJSON(typ core.Type) typeJSON {
	fields := typ.Fields
	if fields == nil {
		fields = []string{}
	}
	return typeJSON{
		ID:        string(typ.ID),
		Name:      typ.Name,
		Fields:    fields,
		Templates: typeTemplatesJSON{Front: typ.Front, Back: typ.Back},
		Version:   typ.Version,
	}
}

func (s *Server) getTypes(w http.ResponseWriter, r *http.Request) {
	types := s.lib.Types()
	out := make([]typeJSON, 0, len(types))
	for _, typ := range types {
		out = append(out, newTypeJSON(typ))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getType(w http.ResponseWriter, r *http.Request) {
	id := core.ID(r.PathValue("id"))
	typ, ok := s.lib.Type(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "no such type")
		return
	}
	writeJSON(w, http.StatusOK, newTypeJSON(typ))
}

// summaryJSON is today's work as the statistics view reports it.
type summaryJSON struct {
	Reviewed    int     `json:"reviewed"`
	DueTomorrow int     `json:"due_tomorrow"`
	Accuracy    float64 `json:"accuracy"`
}

func (s *Server) getStatsSummary(w http.ResponseWriter, r *http.Request) {
	provider, ok := s.stats(w)
	if !ok {
		return
	}
	summary := provider.Summary(s.opts.Now())
	writeJSON(w, http.StatusOK, summaryJSON{
		Reviewed:    summary.Reviewed,
		DueTomorrow: summary.DueTomorrow,
		Accuracy:    summary.Accuracy,
	})
}

// dayCountJSON is one calendar day's count.
type dayCountJSON struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

func (s *Server) getStatsForecast(w http.ResponseWriter, r *http.Request) {
	provider, ok := s.stats(w)
	if !ok {
		return
	}
	days, ok := queryInt(w, r, "days", defaultForecastDays)
	if !ok {
		return
	}
	if days == 0 {
		days = defaultForecastDays
	}
	counts := provider.Forecast(s.opts.Now(), days)
	out := make([]dayCountJSON, 0, len(counts))
	for _, day := range counts {
		out = append(out, dayCountJSON{Day: day.Day, Count: day.Count})
	}
	writeJSON(w, http.StatusOK, out)
}

// retentionJSON is the recall picture: how often a card was remembered, by how
// likely the schedule thought it was.
type retentionJSON struct {
	Reviews  int          `json:"reviews"`
	Passes   int          `json:"passes"`
	Accuracy float64      `json:"accuracy"`
	Buckets  []bucketJSON `json:"buckets"`
}

type bucketJSON struct {
	Low      float64 `json:"low"`
	High     float64 `json:"high"`
	Reviews  int     `json:"reviews"`
	Passes   int     `json:"passes"`
	Accuracy float64 `json:"accuracy"`
}

func (s *Server) getStatsRetention(w http.ResponseWriter, r *http.Request) {
	provider, ok := s.stats(w)
	if !ok {
		return
	}
	report := provider.Retention()
	out := retentionJSON{
		Reviews:  report.Reviews,
		Passes:   report.Passes,
		Accuracy: ratio(report.Passes, report.Reviews),
		Buckets:  make([]bucketJSON, 0, len(report.Buckets)),
	}
	for _, bucket := range report.Buckets {
		out.Buckets = append(out.Buckets, bucketJSON{
			Low:      bucket.Low,
			High:     bucket.High,
			Reviews:  bucket.Reviews,
			Passes:   bucket.Passes,
			Accuracy: ratio(bucket.Passes, bucket.Reviews),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ratio is the share of reviews that went right, zero when there were none.
func ratio(passes, reviews int) float64 {
	if reviews == 0 {
		return 0
	}
	return float64(passes) / float64(reviews)
}

// stats is the configured statistics provider, or the error that says the
// server has none.
func (s *Server) stats(w http.ResponseWriter) (Stats, bool) {
	if s.opts.Stats == nil {
		writeError(w, http.StatusNotImplemented, "stats_unavailable", "this server was built without statistics")
		return nil, false
	}
	return s.opts.Stats, true
}

// backlinkJSON is one paragraph that names a note.
type backlinkJSON struct {
	Note      string `json:"note"`
	Title     string `json:"title"`
	Block     string `json:"block"`
	Paragraph int    `json:"paragraph"`
	Text      string `json:"text"`
}

func (s *Server) getBacklinks(w http.ResponseWriter, r *http.Request) {
	id := core.ID(r.PathValue("id"))
	if _, ok := s.lib.Note(id); !ok {
		writeError(w, http.StatusNotFound, "not_found", "no such note")
		return
	}
	links := s.lib.Backlinks(id)
	out := make([]backlinkJSON, 0, len(links))
	for _, link := range links {
		out = append(out, backlinkJSON{
			Note:      string(link.Note),
			Title:     link.NoteTitle,
			Block:     string(link.Block),
			Paragraph: link.Paragraph,
			Text:      link.Text,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// eventJSON is one library event as the stream sends it.
type eventJSON struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// getEvents streams what happens to the library as Server-Sent Events until the
// client goes away. It is a read, so it needs no token. Nothing is replayed: a
// subscriber hears what happens after it subscribes, a heartbeat comment keeps
// an idle stream alive, and a client that stops reading is dropped by the
// library rather than allowed to slow a write.
func (s *Server) getEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "no_stream", "this connection cannot stream")
		return
	}

	// Subscribe before the first byte goes out: a client that has seen the
	// stream open has a subscription behind it, so a write it makes next is
	// never missed.
	events := make(chan store.Event, eventQueue)
	cancel := s.lib.Subscribe(func(ev store.Event) {
		select {
		case events <- ev:
		case <-r.Context().Done():
		}
	})
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	beat := time.NewTicker(heartbeatEvery)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-events:
			data, err := json.Marshal(eventJSON{Kind: ev.Kind, ID: string(ev.ID)})
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-beat.C:
			// A comment frame is a heartbeat: a client ignores it, a proxy does
			// not time the connection out, and a dead one is noticed here.
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// getSchema reports the API's version, the data format version and the routes
// it serves, so a client can feature-detect instead of guessing what a build
// supports.
func (s *Server) getSchema(w http.ResponseWriter, r *http.Request) {
	routes := s.routes()
	paths := make([]string, 0, len(routes))
	for _, rt := range routes {
		paths = append(paths, rt.method+" "+rt.path)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"api":    apiVersion,
		"format": store.FormatVersion,
		"routes": paths,
	})
}

// getReading lists the reading queue: the notes due to be read, soonest first,
// with where each one was left. Reading needs no token, like every read.
func (s *Server) getReading(w http.ResponseWriter, r *http.Request) {
	entries, err := s.lib.ReadingQueue()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := make([]readingJSON, 0, len(entries))
	for _, entry := range entries {
		out = append(out, s.newReadingJSON(entry, nil))
	}
	writeJSON(w, http.StatusOK, out)
}

// getReadingEntry shows one note's place in the reading queue with the events
// that put it there: the sessions and the changes of membership, in order.
func (s *Server) getReadingEntry(w http.ResponseWriter, r *http.Request) {
	id := core.ID(r.PathValue("id"))
	entry, ok := s.lib.ReadingFor(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "the reading queue does not hold that note")
		return
	}
	events, err := s.lib.ReadingEvents(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := make([]readingEventJSON, 0, len(events))
	for _, ev := range events {
		out = append(out, newReadingEventJSON(ev))
	}
	writeJSON(w, http.StatusOK, s.newReadingJSON(entry, out))
}

// postReading puts a note in the reading queue. It is the one write the reading
// queue takes: removing a note and recording where a session stopped are the
// window's, so an agent cannot take a note away from its reader.
func (s *Server) postReading(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Note string `json:"note"`
		At   string `json:"at"`
	}
	if !decode(w, r, &body) {
		return
	}
	id := core.ID(body.Note)
	if id == "" {
		writeError(w, http.StatusBadRequest, "no_note", "note must name a note in this library")
		return
	}
	at := s.opts.Now()
	if body.At != "" {
		parsed, err := time.Parse(time.RFC3339, body.At)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_time", "at must be RFC 3339")
			return
		}
		at = parsed
	}
	if err := s.lib.AddToReading(id, at); err != nil {
		writeStoreError(w, err)
		return
	}
	entry, _ := s.lib.ReadingFor(id)
	writeJSON(w, http.StatusOK, s.newReadingJSON(entry, nil))
}

// readingJSON is one note's place in the reading queue as the API shows it.
// Due is when it comes up, RFC 3339 and always present — empty for a row no
// schedule has given a date, so a client reads the key without asking whether
// it exists. Archived and Dismissed say the note is in a terminal state: the
// single-entry route reports them for a note that left the queue.
type readingJSON struct {
	Note      string             `json:"note"`
	Title     string             `json:"title"`
	Position  string             `json:"position,omitempty"`
	Status    string             `json:"status"`
	Due       string             `json:"due"`
	LastSeen  *time.Time         `json:"last_review,omitempty"`
	Rating    string             `json:"last_rating,omitempty"`
	Reviews   int                `json:"reviews"`
	Archived  bool               `json:"archived"`
	Dismissed bool               `json:"dismissed"`
	Events    []readingEventJSON `json:"events,omitempty"`
}

func (s *Server) newReadingJSON(entry store.Reading, events []readingEventJSON) readingJSON {
	out := readingJSON{
		Note:      string(entry.Note),
		Position:  string(entry.Position),
		Status:    "reading",
		Reviews:   entry.State.Reviews,
		Archived:  entry.Archived,
		Dismissed: entry.Dismissed,
		Events:    events,
	}
	if entry.Queued {
		out.Status = "queued"
	}
	if note, ok := s.lib.Note(entry.Note); ok {
		out.Title = note.Title
	}
	if due := entry.Due(); !due.IsZero() {
		out.Due = due.UTC().Format(time.RFC3339)
	}
	if entry.State.Reviews > 0 {
		last := entry.State.LastReview
		out.LastSeen = &last
		out.Rating = entry.State.LastRating.String()
	}
	return out
}

// readingEventJSON is one line of a note's reading history.
type readingEventJSON struct {
	Seq      int64  `json:"seq"`
	Kind     string `json:"kind"`
	Rating   string `json:"rating,omitempty"`
	Position string `json:"position,omitempty"`
	At       string `json:"at"`
}

func newReadingEventJSON(ev schedule.Event) readingEventJSON {
	out := readingEventJSON{Seq: ev.Seq, Kind: ev.Kind.String(), At: ev.At.UTC().Format(time.RFC3339)}
	if ev.Kind == schedule.Reviewed {
		out.Rating = ev.Rating.String()
	}
	out.Position = string(ev.Position)
	return out
}

func (s *Server) postNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title  string   `json:"title"`
		Blocks []string `json:"blocks"`
	}
	if !decode(w, r, &body) {
		return
	}
	note := core.Note{Title: body.Title}
	for _, text := range body.Blocks {
		note.Blocks = append(note.Blocks, core.Block{Text: text})
	}
	saved, err := s.lib.SaveNote(note)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, newNoteJSON(saved))
}

func (s *Server) postCard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Block  string            `json:"block"`
		Fields map[string]string `json:"fields"`
	}
	if !decode(w, r, &body) {
		return
	}
	block := core.ID(body.Block)
	if _, ok := s.lib.Block(block); !ok {
		writeError(w, http.StatusBadRequest, "no_such_block", "block not found in this library")
		return
	}
	card, err := s.lib.CreateCard(block)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if len(body.Fields) > 0 {
		card.Fields = body.Fields
		card.Edited = true
		if err := s.lib.SaveCard(card); err != nil {
			writeStoreError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, s.newCardJSON(card))
}

func (s *Server) postReview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Rating string `json:"rating"`
		At     string `json:"at"`
	}
	if !decode(w, r, &body) {
		return
	}
	rating, ok := schedule.ParseRating(body.Rating)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_rating", "rating must be again, hard, good or easy")
		return
	}
	at := s.opts.Now()
	if body.At != "" {
		parsed, err := time.Parse(time.RFC3339, body.At)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_time", "at must be RFC 3339")
			return
		}
		at = parsed
	}
	id := core.ID(r.PathValue("id"))
	if err := s.lib.Review(id, rating, at); err != nil {
		writeStoreError(w, err)
		return
	}
	card, _ := s.lib.Card(id)
	writeJSON(w, http.StatusOK, s.newCardJSON(card))
}

func (s *Server) patchCard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Fields *map[string]string `json:"fields"`
		Status *string            `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	card, ok := s.lib.Card(core.ID(r.PathValue("id")))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "no such card")
		return
	}
	if body.Fields != nil {
		card.Fields = *body.Fields
		card.Edited = len(*body.Fields) > 0
	}
	if body.Status != nil {
		status, ok := core.ParseStatus(*body.Status)
		if !ok {
			writeError(w, http.StatusBadRequest, "bad_status", "unknown status")
			return
		}
		card.Status = status
	}
	if err := s.lib.SaveCard(card); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.newCardJSON(card))
}

// noteJSON is a note as the API shows it.
type noteJSON struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Blocks []blockJSON `json:"blocks"`
}

type blockJSON struct {
	ID   string `json:"id,omitempty"`
	Text string `json:"text"`
}

func newNoteJSON(n core.Note) noteJSON {
	out := noteJSON{ID: string(n.ID), Title: n.Title, Blocks: make([]blockJSON, 0, len(n.Blocks))}
	for _, b := range n.Blocks {
		out.Blocks = append(out.Blocks, blockJSON{ID: string(b.ID), Text: b.Text})
	}
	return out
}

// cardJSON is a card as the API shows it, with the schedule state a caller
// needs to reason about it: its type and fields as the library holds them, and
// the rendered faces as a reader sees them.
type cardJSON struct {
	ID       string            `json:"id"`
	Block    string            `json:"block"`
	Type     string            `json:"type"`
	Ordinal  int               `json:"ordinal"`
	Fields   map[string]string `json:"fields,omitempty"`
	Edited   bool              `json:"edited,omitempty"`
	Front    string            `json:"front"`
	Back     string            `json:"back"`
	Status   string            `json:"status"`
	Link     string            `json:"link"`
	Due      *time.Time        `json:"due,omitempty"`
	LastSeen *time.Time        `json:"last_review,omitempty"`
	Reviews  int               `json:"reviews"`
}

func (s *Server) newCardJSON(c core.Card) cardJSON {
	front, back, link := s.lib.Faces(c)
	out := cardJSON{
		ID:      string(c.ID),
		Block:   string(c.Ref.Block),
		Type:    string(c.Type),
		Ordinal: c.Ordinal,
		Fields:  c.Fields,
		Edited:  c.Edited,
		Front:   front,
		Back:    back,
		Status:  c.Status.String(),
		Link:    link,
	}
	if state, ok := s.lib.CardState(c.ID); ok {
		due, last := state.Due, state.LastReview
		out.Due = &due
		if state.Reviews > 0 {
			out.LastSeen = &last
		}
		out.Reviews = state.Reviews
	}
	return out
}

// queryInt reads a non-negative integer query parameter, answering def when
// the caller names none. The second result is false when the request has
// already been answered with an error.
func queryInt(w http.ResponseWriter, r *http.Request, name string, def int) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return def, true
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		writeError(w, http.StatusBadRequest, "bad_"+name, name+" must be a non-negative integer")
		return 0, false
	}
	return n, true
}

func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrExists):
		writeError(w, http.StatusConflict, "exists", err.Error())
	case errors.Is(err, store.ErrClosed):
		writeError(w, http.StatusServiceUnavailable, "closed", err.Error())
	case errors.Is(err, store.ErrNoReview):
		writeError(w, http.StatusConflict, "no_review", err.Error())
	default:
		writeError(w, http.StatusBadRequest, "rejected", err.Error())
	}
}

// methodNotAllowed answers a known path with an unsupported method. The API
// has no deletes, so DELETE on a collection is a method problem, not a typo in
// the path.
func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "this API does not support "+r.Method)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// LoadOrCreateToken returns the token at path, writing a fresh one when the
// file is missing. The file is readable only by its owner.
func LoadOrCreateToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("agentapi: token file %s is empty", path)
		}
		return token, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("agentapi: read token: %w", err)
	}
	return writeToken(path)
}

// RotateToken writes a fresh token at path, replacing the old one, so a token
// that leaked can be withdrawn without deleting the file by hand.
func RotateToken(path string) (string, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("agentapi: replace token: %w", err)
	}
	return writeToken(path)
}

func writeToken(path string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("agentapi: create token directory: %w", err)
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("agentapi: token: %w", err)
	}
	token := hex.EncodeToString(b)
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("agentapi: write token: %w", err)
	}
	return token, nil
}
