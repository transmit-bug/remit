package main

import (
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
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
	s := newAgentService(dir)
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

	off := newAgentService(t.TempDir())
	off.setLibrary(lib)
	if err := off.startIfEnabled(false); err != nil {
		t.Fatalf("start while off: %v", err)
	}
	if off.Status().Address != "" {
		t.Error("the service started while its setting was off")
	}

	on := newAgentService(t.TempDir())
	on.setLibrary(lib)
	if err := on.startIfEnabled(true); err != nil {
		t.Fatalf("start while on: %v", err)
	}
	defer on.Stop()
	if on.Status().Address == "" {
		t.Error("the service did not start while its setting was on")
	}
}
