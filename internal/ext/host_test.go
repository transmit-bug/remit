package ext_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
)

// noteReader is a plugin that reads one note: the call a role plugin makes as
// soon as its library exists.
const noteReader = `
export function get(id) { return remit.notes.get(id); }
`

// A runner with no library answers ErrNoLibrary, names the reason in words a
// reader can act on, and does not treat it as the plugin's failure: three of
// them in a row would otherwise disable the role plugin that merely started
// before the library it is waiting for.
func TestRunnerWithoutAHostSaysNoLibraryYet(t *testing.T) {
	runner, err := ext.OpenJS(writeJS(t, manifestFor("waiting", ext.CapNotesRead), noteReader), nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	var callErr error
	for range 3 {
		if callErr = runner.Call("get", "note_1", nil); !errors.Is(callErr, ext.ErrNoLibrary) {
			t.Fatalf("with no library the call answered %v, want ErrNoLibrary", callErr)
		}
		if _, disabled, _ := runner.Streak(); disabled {
			t.Fatalf("call %d disabled the runner: a library that is not open yet is not the plugin's failure", callErr)
		}
	}
	for _, want := range []string{"no library yet", "waiting", ext.CapNotesRead} {
		if !strings.Contains(callErr.Error(), want) {
			t.Errorf("the error %q does not name %q", callErr, want)
		}
	}
	if failures, _, _ := runner.Streak(); failures != 0 {
		t.Errorf("three no-library answers left the failure streak at %d", failures)
	}
}

// The provider a runner is given after construction is what its capability
// calls use: one runner answers "no library yet", then the library, without
// being restarted — which is the whole point of a role plugin starting before
// the library opens, and what makes opening one no longer close it.
func TestUseHostGivesARunningRunnerItsLibrary(t *testing.T) {
	host := newTestHost(t)
	note, err := host.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	runner, err := ext.OpenJS(writeJS(t, manifestFor("late", ext.CapNotesRead), noteReader), nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	// Warmed while no library is open, as a role's plugin is at activation.
	if err := runner.Warm(); err != nil {
		t.Fatalf("warm: %v", err)
	}
	made := runner.Runtimes()

	var got struct {
		Title string `json:"title"`
	}
	if err := runner.Call("get", string(note.ID), &got); !errors.Is(err, ext.ErrNoLibrary) {
		t.Fatalf("before the library the call answered %v, want ErrNoLibrary", err)
	}

	runner.UseHost(func() ext.Host { return host })

	if err := runner.Call("get", string(note.ID), &got); err != nil {
		t.Fatalf("after the library was given: %v", err)
	}
	if got.Title != "Biology" {
		t.Errorf("the same runner read %+v", got)
	}
	if runner.Runtimes() != made {
		t.Errorf("the runner was restarted: %d runtimes, want the %d it had", runner.Runtimes(), made)
	}
}

// The provider is asked once per capability call, not once when it is set: a
// runner that outlives its library reads the library that is open now. The
// regression: a provider that handed back the host it was bound with would pin
// a switched library, and #265 hands the same provider to the supervisor.
func TestHostProviderIsAskedOnEveryCall(t *testing.T) {
	first := newTestHost(t)
	firstNote, err := first.SaveNote(core.Note{Title: "First", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("first note: %v", err)
	}
	second := newTestHost(t)
	secondNote, err := second.SaveNote(core.Note{Title: "Second", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("second note: %v", err)
	}

	var current ext.Host = first
	runner, err := ext.OpenJS(writeJS(t, manifestFor("switching", ext.CapNotesRead), noteReader), nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	runner.UseHost(func() ext.Host { return current })

	var got struct {
		Title string `json:"title"`
	}
	if err := runner.Call("get", string(firstNote.ID), &got); err != nil || got.Title != "First" {
		t.Fatalf("the first library answered %+v (%v)", got, err)
	}
	current = second
	if err := runner.Call("get", string(secondNote.ID), &got); err != nil || got.Title != "Second" {
		t.Fatalf("the second library answered %+v (%v)", got, err)
	}
}
