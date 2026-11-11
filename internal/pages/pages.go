// Package pages is the plugin page runtime: the rules a contributed page's life
// follows, in one place a test can drive without building a window.
//
// The rules are real: entering shows what the last load left, a page loaded in
// this context keeps its tree instead of asking again, a load in flight is not
// asked for twice, a failure is drawn with a retry, three failures in a row hide
// the page, and a library write makes every page old. They used to live as seven
// methods over eight tables on the window, so the only way to test them was to
// build a window and look at it from outside.
//
// The runtime draws nothing and calls no plugin. Enter reads the cache and hands
// out a Job — a value naming the call, which the caller runs wherever and
// whenever it likes — and Apply takes the answer back. That is the whole
// discipline of "a frame never calls a plugin": a frame can only enter and
// apply, and both touch this module's own memory.
package pages

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sort"

	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// Runtime holds every page entered, and what each one holds now: the tree the
// last load left, why it has none, whether a load is in flight, and the context
// that tree belongs to.
//
// Its failures are its own count in the contribution layer, never a second
// counter: a page's count, its reason and the hidden set are one value, so the
// settings page cannot read a count that disagrees with what shows. The count is
// per page, so a broken page never carries its plugin's commands away with it
// (docs/specs/plugins/ui.md §8) — the plugin-level layer is the supervisor's and
// stays separate (docs/specs/plugin-system.md P1).
type Runtime struct {
	failures *contrib.Failures
	pages    map[string]*page
	current  contrib.PageRef
}

// page is one contributed page's state, under the key its plugin and id make.
type page struct {
	ref     contrib.PageRef
	tree    contrib.Node
	err     string // why the page has no tree, "" when it has one
	loading bool   // a load is in flight
	loaded  bool   // a tree was applied for hash
	hash    string // the context the tree was loaded in
}

// New returns an empty runtime: a page is made on first use, and holds no tree
// until one of its jobs is applied.
func New() *Runtime {
	return &Runtime{failures: contrib.NewFailures(), pages: map[string]*page{}}
}

// View is a page as it can be drawn now: the tree the last load left, or the
// reason there is none and whether more is on its way. It is data — the screen
// decides every pixel of it.
type View struct {
	Ref     contrib.PageRef
	Tree    contrib.Node
	Err     string
	Loading bool
}

// Job is one page's data call, as a value: which plugin to ask, the call the
// page's data command takes and the context it reads, and the page it answers
// for. A job holds no function and starts nothing; the caller decides when and
// where it runs, which is what makes this package's test's "background" a plain
// call.
type Job struct {
	// Ref is the page the call answers for.
	Ref contrib.PageRef
	// Plugin is the manifest name to ask: what a runner is resolved by.
	Plugin string
	// In is the call itself: the page's data command and its context.
	In ext.CommandIn

	key  string
	hash string
}

// Enter makes ref the page on screen and answers what can be drawn for it in
// ctx now. It asks nothing: when the tree it holds belongs to another context —
// a write invalidated it — it hands out the job that asks again, and the caller
// runs that call off its frame. A page with a failure is not asked again by
// entering: its error is what it draws, and Retry is the call that asks.
func (r *Runtime) Enter(ref contrib.PageRef, ctx map[string]any) (View, *Job) {
	return r.enter(ref, ctx, false)
}

// Retry asks again even when the page has a tree or a failure in hand: a failed
// page's retry and the page's own refresh are its two callers. A load already in
// flight is not asked twice, and neither is a page three failures took out,
// which is the same rule entering follows.
func (r *Runtime) Retry(ref contrib.PageRef, ctx map[string]any) (View, *Job) {
	return r.enter(ref, ctx, true)
}

func (r *Runtime) enter(ref contrib.PageRef, ctx map[string]any, force bool) (View, *Job) {
	p := r.state(ref)
	r.current = p.ref
	hash := contextHash(ctx)
	loaded := p.loaded && p.hash == hash
	hidden := r.failures.Hidden(p.ref.Plugin, p.ref.Page.ID)
	if p.loading || hidden || (!force && (p.err != "" || loaded)) {
		// Nothing to ask: in flight, hidden, drawn already, or failed — and a
		// failure is what the page shows until its retry asks.
		return p.view(), nil
	}
	p.loading = true
	// The retry is on its way: the page draws its skeleton rather than the error
	// it is asking about again. A failure puts the reason back.
	p.err = ""
	return p.view(), &Job{
		Ref:    p.ref,
		Plugin: p.ref.Name,
		In:     ext.CommandIn{Name: p.ref.Page.Data, Context: ctx},
		key:    key(p.ref),
		hash:   hash,
	}
}

// Apply takes a job's answer: the tree its data command answered, or the error
// it threw. It answers whether that was the failure that hid the page, which is
// the caller's cue to leave it — a hidden page is not one to sit on.
//
// An answer the page is not waiting for — the page went away, or the library
// reopens and takes its states with it — is dropped: nothing on screen owes it
// anything.
func (r *Runtime) Apply(job *Job, tree []byte, err error) (hidden bool) {
	p := r.pages[job.key]
	if p == nil || !p.loading {
		return false
	}
	p.loading = false
	if err == nil {
		var parsed contrib.Node
		if parsed, err = contrib.ParsePage(tree); err == nil {
			p.tree, p.err, p.hash, p.loaded = parsed, "", job.hash, true
			r.failures.Succeeded(job.Ref.Plugin, job.Ref.Page.ID)
			return false
		}
	}
	p.err = err.Error()
	slog.Warn("remit: a plugin page failed", "plugin", job.Ref.Name, "page", job.Ref.Page.ID, "err", err)
	if !r.failures.Failed(job.Ref.Plugin, job.Ref.Page.ID, p.err) {
		return false
	}
	slog.Warn("remit: a plugin page was hidden after three failures", "plugin", job.Ref.Name, "page", job.Ref.Page.ID)
	if samePage(r.current, job.Ref) {
		// The page on screen is gone: the caller's route has nothing left to
		// draw, and the rail cannot name a page the window no longer has.
		r.current = contrib.PageRef{}
	}
	return true
}

// Abandon drops a job that will not be run — the frame that entered could not
// open the plugin, and starting one is a handler's job. The next enter hands the
// job out again rather than leaving the page on a skeleton for a call nobody
// made.
func (r *Runtime) Abandon(job *Job) {
	if p := r.pages[job.key]; p != nil {
		p.loading = false
	}
}

// Invalidate forgets what each page's tree was loaded against: a library write
// makes every page old. The trees stay, so a page keeps showing what it has
// until the new answer arrives, and the next enter asks again.
func (r *Runtime) Invalidate() {
	for _, p := range r.pages {
		p.loaded, p.hash = false, ""
	}
}

// Leave forgets the page on screen, which is what a reload of the declarations
// asks when the library no longer declares that page.
func (r *Runtime) Leave() { r.current = contrib.PageRef{} }

// Current is the page on screen: the last one entered, empty when none is.
func (r *Runtime) Current() contrib.PageRef { return r.current }

// Loaded is every page whose tree a load left, in key order: what the window's
// readers that look across pages — a board naming a block — ask.
func (r *Runtime) Loaded() []View {
	keys := make([]string, 0, len(r.pages))
	for k, p := range r.pages {
		if p.tree.Kind != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]View, len(keys))
	for i, k := range keys {
		out[i] = r.pages[k].view()
	}
	return out
}

// Hidden reports whether three failures took a page out of the window.
func (r *Runtime) Hidden(plugin, id string) bool {
	return r.failures.Hidden(plugin, id)
}

// Total is how many failures one plugin's pages have in a row: what the settings
// page adds to the plugin's contributed-entry count.
func (r *Runtime) Total(plugin string) int { return r.failures.Total(plugin) }

// Health is one page that is not answering, as the settings page reads it: the
// plugin that declared it, the page itself, how many failures in a row it has,
// why the last one failed, and whether that is what took it out of the window.
type Health struct {
	Plugin   string
	Page     string
	Failures int
	Reason   string
	Hidden   bool
}

// Health is every page with a failure in a row, or three behind it, by plugin
// and page: the settings page's one read of why a page is not showing. It calls
// nothing, so a frame may ask it.
func (r *Runtime) Health() []Health {
	out := make([]Health, 0, len(r.pages))
	for _, p := range r.pages {
		count := r.failures.Count(p.ref.Plugin, p.ref.Page.ID)
		hidden := r.failures.Hidden(p.ref.Plugin, p.ref.Page.ID)
		if count == 0 && !hidden {
			continue
		}
		out = append(out, Health{
			Plugin:   p.ref.Plugin,
			Page:     p.ref.Page.ID,
			Failures: count,
			Reason:   p.err,
			Hidden:   hidden,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Plugin != out[j].Plugin {
			return out[i].Plugin < out[j].Plugin
		}
		return out[i].Page < out[j].Page
	})
	return out
}

// Reenable gives a plugin's failed pages another try: the counts and the hidden
// set go back, and each page forgets the error it kept, so that entering it asks
// again. It moves only this plugin's pages — the plugin-level count three failed
// calls leave is the supervisor's, cleared by the same user action one layer up.
func (r *Runtime) Reenable(plugin string) {
	r.failures.Reenable(plugin)
	for _, p := range r.pages {
		if p.ref.Plugin == plugin {
			p.err, p.hash, p.loaded, p.loading = "", "", false, false
		}
	}
}

// state answers one page's state, made on first use. The ref the page was last
// entered with is the one it draws, so a title a library gained is seen on the
// next enter.
func (r *Runtime) state(ref contrib.PageRef) *page {
	k := key(ref)
	p := r.pages[k]
	if p == nil {
		p = &page{}
		r.pages[k] = p
	}
	p.ref = ref
	return p
}

func (p *page) view() View {
	return View{Ref: p.ref, Tree: p.tree, Err: p.err, Loading: p.loading}
}

func samePage(ref, other contrib.PageRef) bool {
	return ref.Plugin == other.Plugin && ref.Page.ID == other.Page.ID
}

// key names a page's state: the plugin's manifest id and the contribution's id.
// An id cannot hold a slash, so one page can never be read as another's.
func key(ref contrib.PageRef) string { return ref.Plugin + "/" + ref.Page.ID }

// contextHash is a load's cache key: the context a page was asked in, as JSON,
// hashed. The same page in the same window state keeps its tree; a hashed key
// means the payload never has to be compared field by field.
func contextHash(payload map[string]any) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:8])
}
