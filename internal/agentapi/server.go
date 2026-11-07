// Package agentapi exposes a library to external agents over loopback HTTP.
//
// Reads need no credentials: the data is the user's own, and a local process
// can read the files anyway. Writes need a bearer token, and the API can only
// add — creating notes, cards and reviews, or changing a card's wording and
// status. It cannot delete a card, rewrite review history or change FSRS
// parameters, so a buggy agent cannot destroy a schedule (decision #3).
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
	"strconv"
	"strings"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Library is the part of a library the API reads and writes. It is also the
// layer the plugin host shares (ext.Host embeds it): one declaration of what a
// library can be read and written by, for the loopback API and for plugins
// alike. The two methods the HTTP surface has no route for — SearchNotes and
// History — are here for the plugins that do.
type Library interface {
	Notes() []core.Note
	Note(core.ID) (core.Note, bool)
	SaveNote(core.Note) (core.Note, error)
	CreateNote(string) (core.Note, error)
	SearchNotes(string) ([]core.Note, error)
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
	// Faces is what a card asks, as the type's templates render it — this
	// package has no splitting rules of its own.
	Faces(core.Card) (front, back string, link string)
}

// Options configures a Server.
type Options struct {
	// Token gates every write. An empty token makes the server read-only,
	// which is the safer failure: no token, no writable API.
	Token string
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
	mux.HandleFunc("GET /v1/library", s.getLibrary)
	mux.HandleFunc("GET /v1/notes", s.getNotes)
	mux.HandleFunc("GET /v1/notes/{id}", s.getNote)
	mux.HandleFunc("GET /v1/cards", s.getCards)
	mux.HandleFunc("GET /v1/cards/{id}", s.getCard)
	mux.HandleFunc("GET /v1/queue", s.getQueue)
	mux.HandleFunc("POST /v1/notes", s.requireToken(s.postNote))
	mux.HandleFunc("POST /v1/cards", s.requireToken(s.postCard))
	mux.HandleFunc("POST /v1/cards/{id}/reviews", s.requireToken(s.postReview))
	mux.HandleFunc("PATCH /v1/cards/{id}", s.requireToken(s.patchCard))
	// A known path with another method is a 405, not a 404: the endpoint exists
	// and the API deliberately has no such operation (no deletes).
	mux.HandleFunc("/v1/notes", methodNotAllowed)
	mux.HandleFunc("/v1/cards", methodNotAllowed)
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
	var due int
	if n, err := s.lib.Remaining(s.opts.Now()); err == nil {
		due = n
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"format": "remit/2",
		"notes":  len(notes),
		"cards":  len(s.lib.Cards()),
		"due":    due,
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
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "bad_limit", "limit must be a non-negative integer")
			return
		}
		limit = n
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
