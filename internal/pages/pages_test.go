package pages_test

import (
	"errors"
	"testing"

	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/pages"
)

// leeches is the page every test drives: one contributed page, its data command
// and the title the plugin wrote.
var leeches = contrib.PageRef{
	Plugin: "acme.report",
	Name:   "report",
	Page: contrib.Page{
		ID:    "report",
		Title: "%page.title%",
		Nav:   contrib.NavPlugins,
		Data:  "command:data",
	},
}

// tree is a page tree the contract allows, as a data command answers it: one
// text node, so a test can say which answer the page is drawing.
func tree(text string) string {
	return `{"kind":"page","children":[{"kind":"text","text":"` + text + `"}]}`
}

// errThrown is a plugin that threw, as the runner reports it.
var errThrown = errors.New("the plugin threw")

// stub is a plugin as a test drives it: the page's data command answers what the
// test set and counts the contexts it was asked in one by one. Nothing here is
// reached by entering — the runtime hands out a job instead — so a call in the
// log is a call the test made.
type stub struct {
	calls []ext.CommandIn
	tree  string
	err   error
}

func (s *stub) PageData(in ext.CommandIn) (ext.PageTree, error) {
	s.calls = append(s.calls, in)
	return ext.PageTree(s.tree), s.err
}

func (s *stub) RunCommand(ext.CommandIn) (ext.CommandResult, error) {
	panic("the page runtime asked a command callback")
}

// ask runs a job the way the window does: the call it names, and the answer back
// on the next frame. It is the whole "background" this module's test needs — the
// job is a value, so there is no scheduler to fake.
func ask(t *testing.T, r *pages.Runtime, job *pages.Job, plugin *stub) (hidden bool) {
	t.Helper()
	if job == nil {
		t.Fatalf("the runtime handed out no job")
	}
	answer, err := plugin.PageData(job.In)
	return r.Apply(job, answer, err)
}

// windowContext is one window state as a page's data command reads it.
type windowContext map[string]any

func windowState(mode string) windowContext {
	return windowContext{"mode": mode, "library.open": true}
}

// drawn is the text of the page's first node, which is what a test asserts the
// screen would say.
func drawn(t *testing.T, v pages.View) string {
	t.Helper()
	if len(v.Tree.Children) == 0 {
		return ""
	}
	return v.Tree.Children[0].Text
}

// A page loaded in one window state keeps its tree in that state, asks again
// after a library write, and draws the old tree until the new one arrives: the
// state machine a window used to be the only way to exercise.
func TestAPageAsksAgainOnlyWhenItsContextIsOld(t *testing.T) {
	r := pages.New()
	plugin := &stub{tree: tree("loaded")}
	ctx := windowState("notes")

	view, job := r.Enter(leeches, ctx)
	if job == nil {
		t.Fatalf("entering a page with no tree handed out no job: %+v", view)
	}
	if !view.Loading {
		t.Error("the page does not say its data is on the way")
	}
	ask(t, r, job, plugin)

	view, job = r.Enter(leeches, ctx)
	if job != nil {
		t.Errorf("a second enter in the same context asked again: %+v", job.In)
	}
	if got := drawn(t, view); got != "loaded" {
		t.Errorf("the page draws %q, want the tree the load left", got)
	}
	if got := len(plugin.calls); got != 1 {
		t.Fatalf("the plugin was entered %d times, want once", got)
	}

	// A library write makes every page old, and the tree stays on screen until
	// the new answer arrives.
	r.Invalidate()
	view, job = r.Enter(leeches, ctx)
	if job == nil {
		t.Fatal("entering an invalidated page handed out no job")
	}
	if got := drawn(t, view); got != "loaded" {
		t.Errorf("the invalidated page draws %q, want the tree it had", got)
	}
	plugin.tree = tree("fresh")
	ask(t, r, job, plugin)
	view, job = r.Enter(leeches, ctx)
	if job != nil {
		t.Errorf("the reloaded page asked a third time: %+v", job.In)
	}
	if got := drawn(t, view); got != "fresh" {
		t.Errorf("the page draws %q, want the tree the reload answered", got)
	}

	// A window-state change is a different context: the same page asks again.
	if _, job = r.Enter(leeches, windowState("review")); job == nil {
		t.Error("a page entered in another window state kept the tree of the old one")
	}
}

// Entering while a load is out asks nothing more: the page is in flight, and the
// second enter draws the skeleton rather than a second call.
func TestASecondEnterWhileInFlightAsksOnce(t *testing.T) {
	r := pages.New()
	plugin := &stub{tree: tree("loaded")}
	ctx := windowState("notes")

	_, first := r.Enter(leeches, ctx)
	if first == nil {
		t.Fatal("the first enter handed out no job")
	}
	view, second := r.Enter(leeches, ctx)
	if second != nil {
		t.Fatalf("a second enter while the load was in flight asked again: %+v", second.In)
	}
	if !view.Loading {
		t.Error("the page does not draw its skeleton while the load is out")
	}

	ask(t, r, first, plugin)
	if _, job := r.Enter(leeches, ctx); job != nil {
		t.Errorf("the page asked again after its answer arrived: %+v", job.In)
	}
	if got := len(plugin.calls); got != 1 {
		t.Errorf("the plugin was entered %d times, want once", got)
	}
}

// Three failures in a row hide the page, and the settings page reads the count,
// the plugin, the page and the reason from one health snapshot; re-enabling is
// the action that shows it again.
func TestThreeFailuresHideThePageAndHealthSaysWhy(t *testing.T) {
	r := pages.New()
	plugin := &stub{err: errThrown}
	ctx := windowState("notes")

	_, job := r.Enter(leeches, ctx)
	for attempt := 1; attempt <= contrib.FailureLimit; attempt++ {
		hidden := ask(t, r, job, plugin)
		if want := attempt == contrib.FailureLimit; hidden != want {
			t.Fatalf("failure %d answered hidden=%v, want %v", attempt, hidden, want)
		}
		if attempt == contrib.FailureLimit {
			break
		}
		// A failed page shows its error; its retry is the call that asks again,
		// and the page draws its skeleton rather than the error it is asking
		// about again.
		view, retry := r.Retry(leeches, ctx)
		if retry == nil {
			t.Fatalf("the retry after failure %d handed out no job", attempt)
		}
		if !view.Loading || view.Err != "" {
			t.Errorf("the page after retry %d draws %+v, want its skeleton", attempt, view)
		}
		job = retry
	}

	health := r.Health()
	if len(health) != 1 {
		t.Fatalf("health = %+v, want the failed page alone", health)
	}
	got := health[0]
	if got.Plugin != "acme.report" || got.Page != "report" || got.Failures != contrib.FailureLimit || !got.Hidden {
		t.Errorf("health reads %+v, want the report page hidden after %d failures", got, contrib.FailureLimit)
	}
	if got.Reason != errThrown.Error() {
		t.Errorf("health's reason = %q, want what the plugin threw", got.Reason)
	}
	if !r.Hidden("acme.report", "report") {
		t.Error("the hidden page is not reported hidden")
	}
	// What it draws is the reason, not a skeleton: the error is kept until a
	// retry asks again.
	if view, _ := r.Enter(leeches, ctx); view.Err != errThrown.Error() || view.Loading {
		t.Errorf("the hidden page draws %+v, want the failure it kept", view)
	}
	// A hidden page is not asked again, by entering or by a retry.
	if _, job := r.Enter(leeches, ctx); job != nil {
		t.Errorf("the hidden page was entered again: %+v", job.In)
	}
	if _, job := r.Retry(leeches, ctx); job != nil {
		t.Errorf("the hidden page was asked again by a retry: %+v", job.In)
	}

	// Re-enabling clears the count, the hidden set and the error the page kept:
	// entering it asks again.
	r.Reenable("acme.report")
	if health := r.Health(); len(health) != 0 {
		t.Errorf("health after re-enabling = %+v, want nothing failing", health)
	}
	plugin.err, plugin.tree = nil, tree("loaded")
	view, job := r.Enter(leeches, ctx)
	if job == nil {
		t.Fatal("the re-enabled page was not asked again")
	}
	if view.Err != "" {
		t.Errorf("the re-enabled page still shows the old error %q", view.Err)
	}
	ask(t, r, job, plugin)
	if health := r.Health(); len(health) != 0 {
		t.Errorf("health after a good answer = %+v, want nothing failing", health)
	}
}

// Entering hands out a job and calls nothing: the module's half of "a frame
// never calls a plugin". The call happens when the test makes it, and the answer
// is the only thing Apply takes.
func TestEnterHandsOutAJobAndCallsNothing(t *testing.T) {
	r := pages.New()
	plugin := &stub{tree: tree("loaded")}

	view, job := r.Enter(leeches, windowState("notes"))
	if len(plugin.calls) != 0 {
		t.Fatalf("entering called the plugin %d times", len(plugin.calls))
	}
	if job == nil {
		t.Fatal("entering handed out no job")
	}
	if job.Plugin != "report" {
		t.Errorf("the job asks plugin %q, want the manifest name a runner is resolved by", job.Plugin)
	}
	if job.In.Name != "command:data" {
		t.Errorf("the job runs %q, want the page's data command", job.In.Name)
	}
	if job.In.Context["mode"] != "notes" {
		t.Errorf("the job reads context %+v, want the window state it was entered in", job.In.Context)
	}
	if view.Tree.Kind != "" {
		t.Error("a page with no load yet draws a tree")
	}

	ask(t, r, job, plugin)
	if len(plugin.calls) != 1 {
		t.Fatalf("the test's call reached the plugin %d times", len(plugin.calls))
	}
	// Applying a second time is not a second draw of a stale answer.
	if hidden := r.Apply(job, []byte(tree("stale")), nil); hidden {
		t.Error("a dropped answer hid the page")
	}
	view, _ = r.Enter(leeches, windowState("notes"))
	if got := drawn(t, view); got != "loaded" {
		t.Errorf("the page draws %q after a stale apply, want the tree it was loaded with", got)
	}
}

// A job no one runs does not hold the page in flight: the next enter hands it
// out again rather than leaving a skeleton on screen for a call nobody made.
func TestAnAbandonedJobIsHandedOutAgain(t *testing.T) {
	r := pages.New()
	ctx := windowState("notes")

	_, job := r.Enter(leeches, ctx)
	if job == nil {
		t.Fatal("the first enter handed out no job")
	}
	r.Abandon(job)
	if view, again := r.Enter(leeches, ctx); again == nil {
		t.Errorf("the page was left loading after its job was abandoned: %+v", view)
	}
}
