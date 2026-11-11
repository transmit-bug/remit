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
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/internal/ui/screens/read"
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
	if _, err := a.openLibrary(a.dir, false); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// The place has no library: the start view offers to make one here or
			// to open another.
			a.start = true
			return
		}
		a.openErr = err
		slog.Error("remit: open library", "dir", a.dir, "err", err)
	}
}

// openLibrary opens the library at dir and makes it the one on screen, whatever
// sent the user there: the start of the run, the start view, the list of
// libraries opened before, or a link. It is the one path into a library, so the
// four doors cannot drift apart. create makes one where the folder holds none:
// a place the user picked to be their library, or one a link points at.
//
// The target's roles are resolved first, and into locals: what schedules has to
// be known before the index replays, so the scheduling plugin is resolved
// before the store opens, and the policy the library opens with is the one its
// own plugin gives — never the previous library's, and not none at all. A
// library that does not open commits none of it: the window keeps the library,
// the roles and the role plugins it had, so a failed switch is never half a
// library. On success the previous library is closed and the target's roles and
// library are the window's.
func (a *App) openLibrary(dir string, create bool) (*store.Library, error) {
	cards, scheduler := a.rolesFor(dir)
	options := libraryOptions(scheduler)
	lib, err := store.Open(dir, options)
	if errors.Is(err, fs.ErrNotExist) && create {
		lib, err = store.Create(dir, options)
	}
	if err != nil {
		return nil, err
	}
	a.close()
	a.cards, a.scheduler = cards, scheduler
	a.apply(lib, dir)
	return lib, nil
}

// libraryOptions is what the library at a place opens with: the policy of the
// scheduling plugin resolved for that library, and the name every review it
// records is stamped with. The policy is the target's, so a library created now
// starts with its plugin's retention rather than the host's default.
func libraryOptions(scheduler *schedulerPlugin) store.Options {
	options := store.Options{}
	if policy, ok := scheduler.Policy(); ok {
		options.Scheduler = policy.Identity()
		if policy.Retention > 0 && policy.Retention <= 1 {
			options.Params.DesiredRetention = policy.Retention
		}
	}
	return options
}

// rolesFor resolves what derives the cards and what schedules the reviews for
// the library at dir, scheduling first: its policy is what the library opens
// with. The supervisor owns the resolution, and a role naming a plugin that is
// missing or will not start falls back to the one this build ships for it and
// says so in the log: a library always opens, whatever its plugins do. A role
// change is written at once and picked up here, on the next library open.
func (a *App) rolesFor(dir string) (*cardPlugin, *schedulerPlugin) {
	roles := store.ReadRoles(dir)
	scheduler := a.schedulerRole(roles[core.RoleReviewSchedule])
	return a.cardRole(roles[core.RoleCardsDerive]), scheduler
}

// cardRole resolves the plugin the card role names, through the supervisor: no
// paragraph derives a card through a plugin that is not there.
func (a *App) cardRole(name string) *cardPlugin {
	if a.withoutCards {
		return nil
	}
	return openCards(a.plugins.Resolve(core.RoleCardsDerive, name, shippedPlugin(core.RoleCardsDerive)))
}

// schedulerRole resolves the plugin the scheduling role names, through the
// supervisor: scheduling falls back, never stops.
func (a *App) schedulerRole(name string) *schedulerPlugin {
	if a.withoutScheduler {
		return nil
	}
	return openScheduler(a.plugins.Resolve(core.RoleReviewSchedule, name, shippedPlugin(core.RoleReviewSchedule)))
}

// shippedPlugin is the plugin this build ships for role: what the role falls
// back to when the library's own choice cannot play it.
func shippedPlugin(role string) string {
	name, _ := store.DefaultRole(role)
	return name
}

// apply makes lib the open library and opens the window on today, the first
// screen a library earns: what is due to be read and reviewed today, against
// its budget (spec §1.2). Every other area is one key or one click away.
func (a *App) apply(lib *store.Library, dir string) {
	a.lib, a.dir = lib, dir
	a.openErr, a.start = nil, false
	a.welcome.Failed = nil
	a.page = contrib.PageRef{}
	a.area, a.sub, a.session = areaToday, subNone, sessionNone
	a.back = a.back[:0]
	a.syncMode()
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
	if _, err := a.openLibrary(dir, true); err != nil {
		a.report(c, "open library", err)
		return
	}
	if err := a.remember(dir); err != nil {
		a.report(c, "remember library", err)
	}
}

// startView draws the window before a library is open: the screen itself lives
// in its own package, and this hands it the place, the libraries opened before
// and the app's own actions (ADR 0004). The screen keeps its list's state
// between frames, so it is drawn from the app's own value.
func (a *App) startView(c *ui.Context) {
	start.View(c, &a.welcome, a.startDeps())
}

// startDeps is what the screen asks of the app: where a library would go, and
// the three ways to begin, each run here on the UI thread.
func (a *App) startDeps() start.Deps {
	return start.Deps{
		Dir:    a.startDir,
		Create: a.createLibrary,
		Choose: a.chooseLibrary,
		Import: a.importLibrary,
		OpenAt: a.openLibraryAt,
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

// importLibrary is the welcome screen's third door: an import has to arrive in
// a library, so the app opens the one it would have made here and then the
// wizard that reads the source.
func (a *App) importLibrary(c *ui.Context) {
	if a.lib == nil {
		dir := a.startDir()
		if dir == "" {
			slog.Error("remit: import library: no place for it")
			c.Toast(m(c).NoLibraryPlace())
			return
		}
		a.install(c, dir)
		if a.lib == nil {
			return // install said why
		}
	}
	a.openImport(c)
}

// openLibraryAt opens a library the welcome screen remembers. One that is no
// longer there is dropped from the list it was offered in and the card says
// why: a recent row that cannot open is a state, not a click that did nothing.
func (a *App) openLibraryAt(c *ui.Context, dir string) {
	if dir == "" {
		return
	}
	if _, err := a.openLibrary(dir, false); err != nil {
		slog.Error("remit: open recent library", "dir", dir, "err", err)
		a.welcome.Failed = &start.Failure{Dir: dir, Err: err.Error()}
		if errors.Is(err, fs.ErrNotExist) {
			// The library is gone, not merely unreadable: a row that will not
			// open is not offered again.
			a.forgetLibrary(dir)
		}
		return
	}
	if err := a.remember(dir); err != nil {
		a.report(c, "remember library", err)
	}
}

// forgetLibrary takes a path out of the libraries the welcome screen lists. A
// settings write that fails leaves the row where it was; the log keeps the
// reason.
func (a *App) forgetLibrary(dir string) {
	err := a.UpdateSettings(func(s *settings.Settings) { *s = s.ForgetRecent(dir) })
	if err != nil {
		slog.Warn("remit: forget library", "dir", dir, "err", err)
	}
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
	if _, err := a.openLibrary(dir, true); err != nil {
		slog.Error("remit: open library", "dir", dir, "err", err)
		if a.lib == nil {
			a.openErr, a.start = err, false
		}
		return
	}
	if err := a.remember(dir); err != nil {
		slog.Warn("remit: remember library", "err", err)
	}
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
	options := queue.Limits(a.lib.Presets())
	options.Scope = a.reviewScope
	cards, err := queue.Cards(a.lib, time.Now(), options)
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
	a.resolver = cards.New(a.cards.engine(), a.lib.Types())
	a.data = newLibraryData(a.lib.Notes(), a.lib.Cards())
	a.loadTypes()
	a.excerpted = a.excerptedBlocks()
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

// loadCurrent derives what the page on screen reads beyond the snapshot: the
// card table's rows, the statistics replay, the settings page's lists, the card
// on screen's outlook. It is the only path that asks the store for them, so no
// frame does.
func (a *App) loadCurrent() {
	switch a.session {
	case sessionReview:
		a.preview, a.hasPreview = a.previewFor()
		a.summary, a.hasSummary = a.summaryFor()
		a.blockCards = a.cardsByBlock()
		return
	case sessionReading:
		// The reading screen reads the queue reload derived; it has nothing
		// else to load, and a frame never asks the store.
		return
	}
	switch a.area {
	case areaToday:
		a.loadToday()
	case areaRead:
		// The read area shows the page its sub-navigation names: the queue, or
		// the archive beside it.
		if a.sub == subReadArchive {
			a.read.Show(read.Archive)
		} else {
			a.read.Show(read.Queue)
		}
	case areaWrite:
		a.loadWrite()
		a.loadBacklinks()
	case areaRecall:
		switch a.sub {
		case subRecallCards:
			a.loadCards()
		case subRecallTypes:
			// The type list is retaken with the library, so the page reads it.
		case subRecallPresets:
			a.loadPresets()
		case subRecallHistory:
			a.loadHistory()
		case subRecallRelink:
			a.loadRelink()
		default:
			a.loadRecall()
		}
	case areaStats:
		a.loadStats()
	case areaSettings:
		a.syncPrefs()
	}
}

// loadBacklinks derives what points at the note the notebook has open.
func (a *App) loadBacklinks() {
	a.backlinks = nil
	if a.lib == nil || a.notes.Draft.ID == "" {
		return
	}
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

// loadStats derives the statistics area's data: the review calendar, the
// retention picture and its weeks, the production, the time share, today's
// numbers and the forecast. It is a load path: the pages read what it left, so
// no frame replays the review log.
func (a *App) loadStats() {
	if a.lib == nil {
		a.stats = screenstats.State{}
		return
	}
	now := time.Now()
	from, to := screenstats.HeatRange(now)
	due, _ := queue.CountsOf(a.lib, now, queue.Options{})
	reading, _ := a.lib.ReadingQueue()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	reviewed, _ := a.lib.CountReviewed(today, today.AddDate(0, 0, 1))
	params := a.lib.ScheduleParams()
	a.stats = screenstats.Build(screenstats.Input{
		Now:        now,
		Reviews:    stats.ReviewsPerDay(a.lib, from, to),
		History:    stats.HistoryOf(a.lib, from, to).Events,
		Retention:  stats.RetentionOf(a.lib, params),
		Target:     params.DesiredRetention,
		Forecast:   stats.ForecastAll(a.lib, now, screenstats.ForecastDays),
		Production: stats.ProductionOf(a.lib),
		TimeShare:  stats.TimeShareOf(a.lib, from, now),
		Due:        due.Due + due.Learning,
		Reading:    len(reading),
		Reviewed:   reviewed,
	})
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
	ui.Column(c).Grow(1).Center().Padding(tk.SpaceXXL).Children(func() {
		ui.Column(c).MaxWidth(design.MessageWidth).Children(func() {
			kit.Notice(c, "circle-alert", m(c).LibraryError(a.openErr.Error()), tk.Danger, func() {
				ui.Button(c, m(c).RetryLibrary()).MinHeight(tk.Control).
					OnClick(func() { a.open() })
			})
		})
	})
}
