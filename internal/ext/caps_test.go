package ext_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
)

// hostFns calls every capability gate once, each from its own hook, so a test
// can name the capability a refusal is about without a plugin that never asked.
const hostFns = `
export function notes() { return remit.notes.list(); }
export function cards() { return remit.cards.create("card_1"); }
export function reviewRead() { return remit.review.dueCount(); }
export function reviewWrite() { return remit.review.record("card_1", "good"); }
export function fsrs() { return remit.fsrs.review({}, "good"); }
export function store() { return remit.store.get(); }
`

// A call to a capability the manifest did not declare is a CapabilityError that
// names the plugin and the capability — not a TypeError saying the function
// does not exist.
func TestCapabilitiesRefuseWhatTheManifestDidNotDeclare(t *testing.T) {
	tests := []struct {
		name       string
		hook       string
		capability string
	}{
		{"notes:read", "notes", ext.CapNotesRead},
		{"cards:write", "cards", ext.CapCardsWrite},
		{"review:read", "reviewRead", ext.CapReviewRead},
		{"review:write", "reviewWrite", ext.CapReviewWrite},
		{"schedule:fsrs", "fsrs", ext.CapScheduleFSRS},
		{"store:own", "store", ext.CapStoreOwn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A runner of its own: three refused calls in a row would disable
			// one, and this test wants the refusal, not the strike count.
			runner, err := ext.OpenJS(writeJS(t, manifestFor("quiet"), hostFns), newTestHost(t), ext.JSLimits{})
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer runner.Close()

			err = runner.Call(tt.hook, nil, nil)
			var refused *ext.CapabilityError
			if !errors.As(err, &refused) {
				t.Fatalf("%s answered %v, want a CapabilityError", tt.hook, err)
			}
			if refused.Extension != "test.quiet" || refused.Capability != tt.capability {
				t.Errorf("refused %+v, want test.quiet and %s", refused, tt.capability)
			}
			for _, want := range []string{"test.quiet", tt.capability} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// Declaring the capability is the whole of the grant: the same call answers.
func TestCapabilitiesOpenTheDoorTheyName(t *testing.T) {
	host := newTestHost(t)
	if _, err := host.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{{Text: "q :: a"}}}); err != nil {
		t.Fatalf("note: %v", err)
	}
	dir := writeJS(t, manifestFor("reader", ext.CapNotesRead), `
export function list() { return remit.notes.list(); }
`)
	runner, err := ext.OpenJS(dir, host, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()

	var notes []struct {
		Title string `json:"title"`
	}
	if err := runner.Call("list", nil, &notes); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(notes) != 1 || notes[0].Title != "Biology" {
		t.Errorf("list answered %+v", notes)
	}
}

// The host is what the capabilities are checked against, and the check comes
// first: a runner that declares a capability but was given no host says so, and
// one that declares nothing hears about the capability.
func TestCapabilitiesWithoutAHost(t *testing.T) {
	dir := writeJS(t, manifestFor("unhosted", ext.CapNotesRead), `
export function list() { return remit.notes.list(); }
`)
	runner, err := ext.OpenJS(dir, nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer runner.Close()
	err = runner.Call("list", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no host") {
		t.Fatalf("a declared capability with no host answered %v", err)
	}
}
