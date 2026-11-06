package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/update"
)

// The release page the tray offers is built from the repository in mygo.json
// when the config names one: the app never fetches it.
func TestReleasesRepoMatchesConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "mygo.json"))
	if err != nil {
		t.Fatalf("read mygo.json: %v", err)
	}
	var cfg struct {
		Updates struct {
			GitHub string `json:"github"`
		} `json:"updates"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("mygo.json: %v", err)
	}
	if cfg.Updates.GitHub != "" && cfg.Updates.GitHub != releasesRepo {
		t.Errorf("mygo.json updates.github is %q, the app offers %q", cfg.Updates.GitHub, releasesRepo)
	}
	if got := strings.Count(releasesRepo, "/"); got != 1 {
		t.Errorf("releasesRepo %q is not owner/name", releasesRepo)
	}
	if got, want := releasesURL(), "https://github.com/"+releasesRepo+"/releases"; got != want {
		t.Errorf("releasesURL = %q, want %q", got, want)
	}
}

// Update checks are the setting: off asks nothing at all, on asks once, and
// only a newer version is handed on to be offered.
func TestUpdateCheckAtStartup(t *testing.T) {
	t.Run("off asks nothing", func(t *testing.T) {
		calls := 0
		applied := 0
		startUpdateCheck(settings.Settings{}, func(context.Context) (update.Result, error) {
			calls++
			return update.Result{Version: "0.2.0"}, nil
		}, func(string) { applied++ })
		if calls != 0 {
			t.Errorf("the check ran %d times with the setting off", calls)
		}
		if applied != 0 {
			t.Errorf("an update was offered %d times with the setting off", applied)
		}
	})

	t.Run("on checks once", func(t *testing.T) {
		checked := make(chan struct{})
		applied := make(chan string, 1)
		startUpdateCheck(settings.Settings{UpdateCheck: true}, func(context.Context) (update.Result, error) {
			defer close(checked)
			return update.Result{Version: "0.2.0"}, nil
		}, func(version string) { applied <- version })

		select {
		case <-checked:
		case <-time.After(5 * time.Second):
			t.Fatal("the check never ran")
		}
		select {
		case version := <-applied:
			if version != "0.2.0" {
				t.Errorf("offered %q, want 0.2.0", version)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the newer version was never offered")
		}
	})

	// The two cases that must offer nothing wait for the check to finish and
	// then for a late offer to prove itself wrong, through a channel so that
	// the app's goroutine and the test never share a value.
	for _, test := range []struct {
		name  string
		check func(context.Context) (update.Result, error)
	}{
		{"up to date offers nothing", func(context.Context) (update.Result, error) {
			return update.Result{}, nil
		}},
		{"failure offers nothing", func(context.Context) (update.Result, error) {
			return update.Result{}, errors.New("network down")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			checked := make(chan struct{})
			applied := make(chan string, 1)
			startUpdateCheck(settings.Settings{UpdateCheck: true}, func(ctx context.Context) (update.Result, error) {
				defer close(checked)
				return test.check(ctx)
			}, func(version string) { applied <- version })

			select {
			case <-checked:
			case <-time.After(5 * time.Second):
				t.Fatal("the check never ran")
			}
			select {
			case version := <-applied:
				t.Errorf("offered %q", version)
			case <-time.After(50 * time.Millisecond):
			}
		})
	}
}
