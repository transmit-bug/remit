package ui

import (
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/internal/ui/screens/start"
	screenstats "github.com/transmit-bug/remit/internal/ui/screens/stats"
	"github.com/transmit-bug/remit/locales"
)

// open opens the library the app was given or a previous run remembered, and
// reads it into the app. No library at that place is the start view, not a
// library created silently: creating or opening one is the user's choice. A
// library that cannot be opened is a state the window explains, never a reason
// to panic: what the user sees is one line and one action.
func (a *App) open() {
	a.openErr, a.start = nil, false
	if a.dir == "" {
		a.start = true
		return
	}
	a.openRolePlugins()
	lib, err := store.Open(a.dir, a.storeOptions())
	if errors.Is(err, fs.ErrNotExist) {
		// The place has no library: the start view offers to make one here or
		// to open another.
		a.start = true
		return
	}
	if err != nil {
		a.openErr = err
		slog.Error("remit: open library", "dir", a.dir, "err", err)
		return
	}
	a.apply(lib, a.dir)
}

// storeOptions is what the library opens with: the scheduling plugin's policy
// and the name to stamp on the reviews it records.
func (a *App) storeOptions() store.Options {
	options := store.Options{}
	if policy, ok := a.scheduler.Policy(); ok {
		options.Scheduler = policy.Identity()
		if policy.Retention > 0 && policy.Retention <= 1 {
			options.Params.DesiredRetention = policy.Retention
		}
	}
	return options
}

// openRolePlugins resolves each role to the plugin the library names and starts
// it, before the library reads: what schedules has to be known before the index
// replays. A role naming a plugin that is missing or will not start falls back
// to the built-in and says so in the log — a library always opens, whatever its
// plugins do. A role change is written at once and picked up here, on the next
// library open.
func (a *App) openRolePlugins() {
	if a.openPlugin == nil {
		return
	}
	roles := store.ReadRoles(a.dir)
	a.cards = a.cardRole(roles[core.RoleCardsDerive])
	a.scheduler = a.schedulerRole(roles[core.RoleReviewSchedule])
}

// cardRole starts the plugin the card role names, or the app's own when it
// cannot: no paragraph derives a card through a plugin that is not there.
func (a *App) cardRole(name string) *cardPlugin {
	if a.withoutCards {
		return nil
	}
	runner := a.cardsChoice
	if runner == nil {
		opened, err := a.openPlugin(name)
		if err != nil {
			slog.Warn("remit: the card role's plugin did not start", "name", name, "err", err)
		} else {
			runner = opened
		}
	}
	return openCards(runner, false)
}

// schedulerRole starts the plugin the scheduling role names, or the app's own
// when it cannot: scheduling falls back, never stops.
func (a *App) schedulerRole(name string) *schedulerPlugin {
	if a.withoutScheduler {
		return nil
	}
	runner := a.schedulerChoice
	if runner == nil {
		opened, err := a.openPlugin(name)
		if err != nil {
			slog.Warn("remit: the scheduling role's plugin did not start", "name", name, "err", err)
		} else {
			runner = opened
		}
	}
	return openScheduler(runner, false)
}

// apply makes lib the open library and shows the review queue, which is the
// window's first screen once a library is open (decision #11). Notes are one
// key away.
func (a *App) apply(lib *store.Library, dir string) {
	a.lib, a.dir = lib, dir
	a.openErr, a.start = nil, false
	a.mode, a.page = modeReview, contrib.PageRef{}
	a.review.Reset()
	a.loadContributions()
	a.reload()
	if a.opened != nil {
		a.opened(lib)
	}
}

// install opens the library at dir, creating one when the folder holds none,
// remembers it for the next start, and says so where the action was when it
// could not. It is what the start view's two actions run.
func (a *App) install(c *ui.Context, dir string) {
	lib, err := openOrCreate(dir)
	if err != nil {
		a.report(c, "open library", err)
		return
	}
	a.close()
	a.apply(lib, dir)
	if err := a.remember(dir); err != nil {
		a.report(c, "remember library", err)
	}
}

// openOrCreate opens the library at dir, making one when the folder holds
// none: a place the user picked to be their library.
func openOrCreate(dir string) (*store.Library, error) {
	lib, err := store.Open(dir, store.Options{})
	if errors.Is(err, fs.ErrNotExist) {
		return store.Create(dir, store.Options{})
	}
	return lib, err
}

// startView draws the window before a library is open: the screen itself lives
// in its own package, and this hands it the place and the app's own actions
// (ADR 0004).
func (a *App) startView(c *ui.Context) {
	start.View(c, start.State{Dir: a.startDir()}, a.startDeps())
}

// startDeps is what the screen asks of the app: where a library would go, and
// the two ways to begin, each run here on the UI thread.
func (a *App) startDeps() start.Deps {
	return start.Deps{
		Dir:    a.startDir,
		Create: a.createLibrary,
		Choose: a.chooseLibrary,
	}
}

// startDir is the folder the start view names: the one the app was pointed at,
// or Remit's own place in Documents.
func (a *App) startDir() string {
	if a.dir != "" {
		return a.dir
	}
	return a.defaultLibrary
}

// createLibrary makes a library at the place the start view named, opens it,
// and remembers it for the next start.
func (a *App) createLibrary(c *ui.Context) {
	dir := a.startDir()
	if dir == "" {
		slog.Error("remit: create library: no place for it")
		c.Toast(m(c).NoLibraryPlace())
		return
	}
	a.install(c, dir)
}

// chooseLibrary asks the system for a folder and opens the library in it,
// making one when the folder holds none. A canceled dialog changes nothing.
func (a *App) chooseLibrary(c *ui.Context) {
	dir, err := a.pickLibrary()
	if err != nil {
		a.report(c, "choose library", err)
		return
	}
	if dir == "" {
		return
	}
	a.install(c, dir)
}

// pickLibraryDir is the start view's folder picker: the system's, attached to
// no window so it is a free-standing panel on every platform. It blocks until
// the user answers, which is why it is only ever called from a handler.
func pickLibraryDir() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:             locales.OpenLibrary(),
		ButtonLabel:       locales.OpenLibrary(),
		Directory:         true,
		CreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return paths[0], nil
}

// close releases the library open now, if any.
func (a *App) close() {
	if a.lib == nil {
		return
	}
	if err := a.lib.Close(); err != nil {
		slog.Warn("remit: close library", "err", err)
	}
	a.lib = nil
	a.loadContributions()
}

// DefaultLibraryDir is where a library goes when the caller names none:
// Remit's own folder in the user's Documents.
func DefaultLibraryDir() (string, error) {
	docs, err := mygo.App.Path(mygo.PathDocuments)
	if err != nil {
		return "", err
	}
	return filepath.Join(docs, Name), nil
}

// Library is the open library, for the local agent server. The store is
// safe for concurrent use; a caller must not close it.
func (a *App) Library() *store.Library { return a.lib }

// OpenLibrary switches the window to the library at dir, closing the one open
// now. A directory without a library becomes one, so a link or a dropped
// archive can point at a place the app has not been before. The place is
// remembered for the next start. A library that will not open leaves the one
// on screen alone, or the error state when there is none.
func (a *App) OpenLibrary(dir string) {
	lib, err := openOrCreate(dir)
	if err != nil {
		slog.Error("remit: open library", "dir", dir, "err", err)
		if a.lib == nil {
			a.openErr, a.start = err, false
		}
		return
	}
	a.close()
	if err := a.remember(dir); err != nil {
		slog.Warn("remit: remember library", "err", err)
	}
	a.apply(lib, dir)
}

// OnLibraryOpen registers a function told about the library once it opens,
// and again after a retry succeeds. Setting it again replaces the previous
// one; it is not called for a library that already opened.
func (a *App) OnLibraryOpen(fn func(*store.Library)) {
	a.opened = fn
	if fn != nil && a.lib != nil {
		fn(a.lib)
	}
}

// dueToday is the store's answer to what is due now, narrowed to the review
// session's scope: the whole library, one note, or one tag. Browse's Due filter
// shows the same answer, so a scoped session narrows both.
func (a *App) dueToday() []core.ID {
	cards, err := queue.Cards(a.lib, time.Now(), queue.Options{Scope: a.reviewScope})
	if err != nil {
		slog.Error("remit: today's queue", "err", err)
		return nil
	}
	ids := make([]core.ID, len(cards))
	for i, card := range cards {
		ids[i] = card.ID
	}
	return ids
}

// reload retakes the snapshot, asks the store what is due now, and loads what
// the mode on screen reads beyond both. Every write ends here, which is what
// keeps the build pass free of store calls.
//
// Writes themselves run on the UI thread. A review append is a durable file
// write the store finishes before it returns — about 9 ms at a 10k-card
// library, measured in internal/core/store — and the queue is derived again
// from the library after each one. That is a hitch, not yet a dropped frame,
// so it stands for now. Follow-up: hand writes to a goroutine of the app's
// own, bring the result back through Window.Update, and drop a result a newer
// screen has overtaken. The snapshot in App is what makes that change local.
func (a *App) reload() {
	if a.lib == nil {
		return
	}
	var runner *ext.JSRunner
	if a.cards != nil {
		runner = a.cards.runner
	}
	a.resolver = cards.New(runner, a.lib.Types())
	a.data = newLibraryData(a.lib.Notes(), a.lib.Cards())
	a.notesRows = a.noteRows()
	now := time.Now()
	a.due = a.dueToday()
	a.review.Sync(a.due)
	a.loadReading(now)
	// The plugin pages are rebuilt from the library that just changed, and the
	// queues' numbers are reported to whoever watches them: one reload, one
	// answer for every reader.
	a.invalidatePages()
	if a.counts != nil {
		a.counts(a.queueCounts())
	}
	a.loadCurrent()
}

// reviewScopeEmpty reports whether the scoped session has neither cards to
// grade nor cards graded: nothing came from the scope, so the screen should say
// the scope is empty rather than celebrate a day it did not have.
func (a *App) reviewScopeEmpty() bool {
	done, _ := a.review.Progress()
	return a.review.Remaining() == 0 && done == 0
}

// summaryFor is the finished session's summary as the store reads it: today's
// answers, what tomorrow holds, how many were right. A scoped session with no
// cards in scope gets no summary: the empty state says the scope is empty, not
// that the day was.
func (a *App) summaryFor() (stats.Summary, bool) {
	if a.lib == nil {
		return stats.Summary{}, false
	}
	if a.reviewScope.Kind != queue.ScopeAll && a.reviewScopeEmpty() {
		return stats.Summary{}, false
	}
	return stats.Summarize(a.lib, time.Now()), true
}

// previewFor asks the store what each answer would schedule for the card on
// screen, through the same parameters the review will use.
func (a *App) previewFor() ([]schedule.Outlook, bool) {
	if a.lib == nil {
		return nil, false
	}
	id, ok := a.review.Current()
	if !ok {
		return nil, false
	}
	return a.lib.Preview(id, time.Now())
}

// loadCurrent derives what the mode on screen reads beyond the snapshot: the
// browse rows and the statistics replay. It is the only path that asks the
// store for them, so no frame does.
func (a *App) loadCurrent() {
	switch a.mode {
	case modeBrowse:
		a.loadBrowse()
	case modeStats:
		a.loadStats()
	case modeReading:
		// The reading screen reads the queue reload derived; it has nothing else
		// to load, and a frame never asks the store.
	case modeNotes:
		a.backlinks = nil
		if a.lib != nil && a.notes.Draft.ID != "" {
			for _, link := range a.lib.Backlinks(a.notes.Draft.ID) {
				a.backlinks = append(a.backlinks, notes.Backlink{
					Note:      link.Note,
					NoteTitle: link.NoteTitle,
					Block:     link.Block,
					Paragraph: link.Paragraph,
					Text:      link.Text,
				})
			}
		}
	case modeReview:
		a.preview, a.hasPreview = a.previewFor()
		a.summary, a.hasSummary = a.summaryFor()
	}
}

// loadStats derives the statistics page's datasets: the review heatmap, the
// retention picture and the forecast. It is a load path: the view reads what it
// left, so no frame replays the review log.
func (a *App) loadStats() {
	if a.lib == nil {
		a.stats = screenstats.State{}
		return
	}
	now := time.Now()
	from, to := screenstats.HeatRange(now)
	a.stats = screenstats.Build(now,
		stats.ReviewsPerDay(a.lib, from, to),
		stats.RetentionOf(a.lib, schedule.DefaultParams()),
		stats.ForecastAll(a.lib, now, screenstats.ForecastDays),
	)
}

// report logs a write that failed and says so where the action was, so a
// failure never looks like a click that did nothing.
func (a *App) report(c *ui.Context, what string, err error) {
	slog.Error("remit: "+what, "err", err)
	c.Toast(m(c).SaveFailed())
}

// libraryErrorView is the window when the library could not be opened: what
// went wrong, and the one action that tries again.
func (a *App) libraryErrorView(c *ui.Context, tk design.Tokens) {
	ui.Column(c).Grow(1).Center().Gap(tk.SpaceM).Padding(tk.SpaceXXL).Children(func() {
		ui.Text(c, m(c).LibraryError(a.openErr.Error())).
			FontSize(tk.TypeBody).TextColor(tk.TextMuted).MaxWidth(design.MessageWidth)
		ui.Button(c, m(c).RetryLibrary()).MinHeight(tk.Control).
			OnClick(func() { a.open() })
	})
}
