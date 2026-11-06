package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// An external agent drives the app's own agent server over a real socket: it
// reads the library, creates a card from a block and records a review.
func TestAgentRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	lib, err := store.Create(filepath.Join(t.TempDir(), "library"), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()

	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "spacing :: beats cramming"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}

	ln, err := serveAgent(lib, "secret")
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	base := (&url.URL{Scheme: "http", Host: ln.Addr().String()}).String()

	get := func(path string) (*http.Response, error) { return http.Get(base + path) }
	resp, err := get("/v1/library")
	if err != nil {
		t.Fatalf("GET library: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET library = %d", resp.StatusCode)
	}

	body, _ := json.Marshal(map[string]any{"block": string(note.Blocks[0].ID)})
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/cards", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST card: %v", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || created.ID == "" {
		t.Fatalf("POST card = %d (%+v)", resp.StatusCode, created)
	}

	body, _ = json.Marshal(map[string]any{"rating": "good"})
	req, _ = http.NewRequest(http.MethodPost, base+"/v1/cards/"+created.ID+"/reviews", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST review: %v", err)
	}
	var reviewed struct {
		Due     *time.Time `json:"due"`
		Reviews int        `json:"reviews"`
	}
	json.NewDecoder(resp.Body).Decode(&reviewed)
	resp.Body.Close()
	if reviewed.Reviews != 1 || reviewed.Due == nil {
		t.Fatalf("review did not schedule: %+v", reviewed)
	}

	// Without the token the same write is refused.
	req, _ = http.NewRequest(http.MethodPost, base+"/v1/cards", bytes.NewReader(body))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST without token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("write without a token = %d, want 401", resp.StatusCode)
	}
}
