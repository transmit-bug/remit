package contrib_test

import (
	"reflect"
	"testing"

	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// The three-strike rule is per (plugin, contribution id): two failures are a
// count, the third hides that contribution — and only that one — and the fourth
// does not report a second hiding.
func TestFailuresHideOneContributionAtThree(t *testing.T) {
	failures := contrib.NewFailures()
	const plugin, contribution = "acme.x", "acme.page"

	for i := 1; i <= 2; i++ {
		if failures.Failed(plugin, contribution, "the plugin threw") {
			t.Fatalf("failure %d hid the contribution", i)
		}
	}
	if failures.Hidden(plugin, contribution) {
		t.Error("two failures hid the contribution")
	}
	if got := failures.Count(plugin, contribution); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
	if got := failures.Total(plugin); got != 2 {
		t.Errorf("total = %d, want 2", got)
	}
	if issues := failures.Issues(nil); len(issues) != 0 {
		t.Errorf("a contribution that failed twice is listed as hidden: %+v", issues)
	}

	if !failures.Failed(plugin, contribution, "the plugin threw again") {
		t.Fatal("the third failure did not hide the contribution")
	}
	if !failures.Hidden(plugin, contribution) {
		t.Error("the third failure did not hide the contribution")
	}
	want := []contrib.Issue{{
		Plugin:       plugin,
		Contribution: contribution,
		Reason:       "the plugin threw again",
		Hidden:       true,
	}}
	if got := failures.Issues(nil); !reflect.DeepEqual(got, want) {
		t.Errorf("issues = %+v, want %+v", got, want)
	}

	if failures.Failed(plugin, contribution, "still broken") {
		t.Error("a fourth failure reported a second hiding")
	}
	if got := failures.Count(plugin, contribution); got != 4 {
		t.Errorf("count = %d, want 4", got)
	}

	// Another contribution is its own count, even with the same id under another
	// plugin: one broken page never hides a sibling or another plugin's work.
	failures.Failed("other.y", contribution, "boom")
	if failures.Hidden("other.y", contribution) {
		t.Error("another plugin's contribution was hidden with the first")
	}
	if got := failures.Total("other.y"); got != 1 {
		t.Errorf("other plugin's total = %d, want 1", got)
	}
}

// A contribution that answered again starts over, and re-enabling one plugin
// clears only that plugin's counts and hidden entries.
func TestFailuresSucceedAndReenable(t *testing.T) {
	failures := contrib.NewFailures()
	failures.Failed("acme.x", "acme.page", "boom")
	failures.Failed("acme.x", "acme.page", "boom")
	failures.Succeeded("acme.x", "acme.page")
	if got := failures.Count("acme.x", "acme.page"); got != 0 {
		t.Errorf("count after a success = %d, want 0", got)
	}
	if failures.Failed("acme.x", "acme.page", "boom") {
		t.Error("a failure after a success hid the contribution: the count did not restart")
	}

	failures.Failed("acme.x", "acme.page", "boom")
	if !failures.Failed("acme.x", "acme.page", "boom") {
		t.Fatal("the count did not reach three again")
	}
	failures.Failed("acme.x", "acme.command", "boom")
	failures.Failed("other.y", "acme.command", "boom") // the other plugin's, once

	failures.Reenable("acme.x")
	if failures.Hidden("acme.x", "acme.page") {
		t.Error("re-enabling kept the hidden entry")
	}
	if got := failures.Count("acme.x", "acme.page"); got != 0 {
		t.Errorf("re-enabling kept a count of %d", got)
	}
	if got := failures.Total("acme.x"); got != 0 {
		t.Errorf("re-enabling kept a total of %d", got)
	}
	if got := failures.Total("other.y"); got != 1 {
		t.Errorf("re-enabling one plugin cleared another's count: %d", got)
	}
	if issues := failures.Issues(nil); len(issues) != 0 {
		t.Errorf("issues after re-enabling = %+v", issues)
	}
}

// One snapshot lists both what the manifest could not declare and what failures
// took out, in a stable order: a settings page that reshuffles its rows reads as
// broken.
func TestIssuesListDropsAndHiddenTogether(t *testing.T) {
	dropped := map[string][]string{
		"acme.x":  {`contributes.commands[1]: icon "no-such-icon": the host has no such icon`},
		"other.y": {"contributes.pages[1]: data: a page needs the command that answers its tree", "contributes.menus.status.item: the slot is reserved"},
	}
	failures := contrib.NewFailures()
	for i := 0; i < contrib.FailureLimit; i++ {
		failures.Failed("acme.x", "acme.cmd", "the command threw")
	}

	want := []contrib.Issue{
		{Plugin: "acme.x", Reason: `contributes.commands[1]: icon "no-such-icon": the host has no such icon`},
		{Plugin: "acme.x", Contribution: "acme.cmd", Reason: "the command threw", Hidden: true},
		{Plugin: "other.y", Reason: "contributes.menus.status.item: the slot is reserved"},
		{Plugin: "other.y", Reason: "contributes.pages[1]: data: a page needs the command that answers its tree"},
	}
	got := failures.Issues(dropped)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("issues = %+v, want %+v", got, want)
	}
}

// A count is usable as a value: the window builds one when the library opens, and
// a zero one counts rather than panicking on a nil map.
func TestFailuresZeroValueCounts(t *testing.T) {
	var failures contrib.Failures
	if failures.Hidden("acme.x", "acme.page") || failures.Count("acme.x", "acme.page") != 0 {
		t.Error("a zero count is not empty")
	}
	if failures.Failed("acme.x", "acme.page", "") {
		t.Error("the first failure hid a contribution")
	}
	if got := failures.Count("acme.x", "acme.page"); got != 1 {
		t.Errorf("count on a zero value = %d, want 1", got)
	}
}
