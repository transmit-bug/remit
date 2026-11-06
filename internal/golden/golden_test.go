package golden_test

import (
	"bytes"
	"flag"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/golden"
)

var update = flag.Bool("update", false, "rewrite the frozen answers from the code under test")

// The frozen answers are what the implementation gives now, to the byte: a
// change of a face or an interval is a change the ticket has to own, not one a
// tolerant comparison hides.
func TestFrozenAnswers(t *testing.T) {
	got, err := golden.Encode(golden.Build())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	path := filepath.Join("testdata", "golden.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run go test ./internal/golden -update once)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the answers changed.\n--- frozen\n%s\n--- now\n%s", want, got)
	}
}

// The set stays wide enough to be evidence: every shape a card's sides take, and
// every answer from every kind of state.
func TestFrozenSetCovers(t *testing.T) {
	set := golden.Build()

	if len(set.Faces) < 11 {
		t.Errorf("the set holds %d faces, want at least the eleven shapes", len(set.Faces))
	}
	states := map[string]bool{}
	kinds := map[string]bool{}
	for _, face := range set.Faces {
		if face.Proves == "" {
			t.Errorf("a face (%s) does not say what it proves", face.Kind)
		}
		states[face.State] = true
		kinds[face.Kind] = true
	}
	for _, want := range []string{"live", "frozen", "lost"} {
		if !states[want] {
			t.Errorf("no face is %s", want)
		}
	}
	for _, want := range []string{"basic", "cloze", "bidirectional"} {
		if !kinds[want] {
			t.Errorf("no face is a %s card", want)
		}
	}

	ratings := map[string]bool{}
	froms := map[string]bool{}
	for _, step := range set.Schedule {
		if step.Proves == "" {
			t.Errorf("a step (%s/%s) does not say what it proves", step.From, step.Rating)
		}
		ratings[step.Rating] = true
		froms[step.From] = true
	}
	for _, want := range []string{"again", "hard", "good", "easy"} {
		if !ratings[want] {
			t.Errorf("no step answers %s", want)
		}
	}
	for _, want := range []string{"new", "learned", "reviewed", "capped"} {
		if !froms[want] {
			t.Errorf("no step starts from %s", want)
		}
	}
	// The cap is a step whose due date is the latest in the set: a card that
	// cannot be scheduled further waits the longest.
	latest := ""
	for _, step := range set.Schedule {
		if step.Due > latest {
			latest = step.Due
		}
	}
	capped := false
	for _, step := range set.Schedule {
		if step.From == "capped" && step.Due == latest {
			capped = true
		}
	}
	if !capped {
		t.Error("no capped step waits the longest")
	}
	if !strings.Contains(set.Note, "go test ./internal/golden -update") {
		t.Error("the set does not say how to regenerate it")
	}
}

// The set is deterministic: built twice, it is the same bytes. The scheduler
// already runs with fuzz off (internal/core/schedule/schedule.go), which is what
// makes an interval worth freezing.
func TestFrozenSetIsDeterministic(t *testing.T) {
	first, err := golden.Encode(golden.Build())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		again, err := golden.Encode(golden.Build())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("build %d differs from the first", i+2)
		}
	}
}

// The frozen numbers carry no floating-point noise: nine decimals is where
// rounding stops, and a value that needs more is a platform's answer rather than
// the schedule's (arm64 and amd64 fuse differently in the last bits).
func TestFrozenNumbersAreRounded(t *testing.T) {
	for _, step := range golden.Build().Schedule {
		for name, value := range map[string]float64{"stability": step.Stability, "difficulty": step.Difficulty} {
			rounded := math.Round(value*1e9) / 1e9
			if rounded != value {
				t.Errorf("%s/%s %s is %v, which is not nine decimals", step.From, step.Rating, name, value)
			}
		}
	}
}
