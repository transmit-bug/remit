package ui

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/update"
)

// Options configures an App.
type Options struct {
	// LibraryDir is the directory the library lives in. Empty means the one a
	// previous run remembered, and with none remembered the window opens on
	// the start view instead of creating a library behind the user's back.
	// Tests pass t.TempDir() so they never touch the user's Documents.
	LibraryDir string
	// DataDir is the app's user data directory, where settings.json lives
	// (main passes mygo.App.Path(mygo.PathUserData)). Empty means settings are
	// neither read nor written, as in a test that drives the window alone.
	DataDir string
	// ChooseLibrary picks the folder a library lives in. It is mygo.Dialog's
	// folder picker unless a test replaces it: a native dialog cannot be
	// answered headlessly.
	ChooseLibrary func() (string, error)
	// ChooseAnki picks the Anki package the palette's import action reads,
	// mygo.Dialog's file picker unless a test replaces it.
	ChooseAnki func() (string, error)
	// ImportAnki imports the package at path into the open library. It is
	// importanki.Import unless a test replaces it.
	ImportAnki func(path string) (importanki.Report, error)
	// BindHotkey binds the global shortcut, replacing the one bound now, and
	// reports a combination that is invalid or already taken. cmd registers it
	// on MyGo's GlobalShortcut; a test replaces it.
	BindHotkey func(accelerator string) error
	// AgentControls is the loopback agent API as the settings page drives it.
	// cmd passes its own; nil leaves the page saying there is no such service.
	AgentControls AgentControls
	// CheckUpdate looks for a newer version. It is checkForUpdate unless a
	// test replaces it.
	CheckUpdate func(context.Context, settings.Settings) (update.Result, error)
}

// libraryData is the library as the views read it: a snapshot the app retakes
// after every write, so that no frame has to touch the store. Everything the
// notes list, the editor, review and browse show is derived here once, per
// write.
type libraryData struct {
	notes  []core.Note
	cards  []core.Card
	byID   map[core.ID]core.Card
	blocks core.BlockSet

	// titles name a note, so a card can say which note it came from.
	titles map[core.ID]string
	// perBlock and perNote are the card counts the editor's badge and the
	// note's summary show.
	perBlock map[core.ID]int
	perNote  map[core.ID]int
	// noteOf is the note a block belongs to, so a card can open its source.
	noteOf map[core.ID]core.ID
}

// newLibraryData derives the snapshot from the store's notes and cards.
func newLibraryData(notes []core.Note, cards []core.Card) libraryData {
	d := libraryData{
		notes:    notes,
		cards:    cards,
		byID:     make(map[core.ID]core.Card, len(cards)),
		blocks:   make(core.BlockSet),
		titles:   make(map[core.ID]string, len(notes)),
		perBlock: make(map[core.ID]int, len(cards)),
		perNote:  make(map[core.ID]int, len(notes)),
		noteOf:   make(map[core.ID]core.ID, len(notes)),
	}
	for _, note := range notes {
		d.titles[note.ID] = note.Title
		for _, block := range note.Blocks {
			if block.ID == "" {
				continue
			}
			d.blocks[block.ID] = block
			d.noteOf[block.ID] = note.ID
		}
	}
	for _, card := range cards {
		d.byID[card.ID] = card
		d.perBlock[card.Ref.Block]++
		if note, ok := d.noteOf[card.Ref.Block]; ok {
			d.perNote[note]++
		}
	}
	return d
}

// card is a card as the library has it now.
func (d libraryData) card(id core.ID) (core.Card, bool) {
	card, ok := d.byID[id]
	return card, ok
}

// cardsForBlock counts the cards that reference a block.
func (d libraryData) cardsForBlock(id core.ID) int { return d.perBlock[id] }

// cardsInNote counts the cards that reference any block of a note.
func (d libraryData) cardsInNote(note core.Note) int { return d.perNote[note.ID] }

// noteOfBlock finds the note that holds a block.
func (d libraryData) noteOfBlock(id core.ID) (core.ID, bool) {
	note, ok := d.noteOf[id]
	return note, ok
}

// titleOf names a note.
func (d libraryData) titleOf(note core.ID) string { return d.titles[note] }

// messageWidth caps a message that is about the library rather than a card:
// the error state's one line, centred in the window.
const messageWidth float32 = 520

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
	lib, err := store.Open(a.dir, store.Options{})
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

// apply makes lib the open library and shows the review queue, which is the
// window's first screen once a library is open (decision #11). Notes are one
// key away.
func (a *App) apply(lib *store.Library, dir string) {
	a.lib, a.dir = lib, dir
	a.openErr, a.start = nil, false
	a.mode = modeReview
	a.review.Reset()
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

// close releases the library open now, if any.
func (a *App) close() {
	if a.lib == nil {
		return
	}
	if err := a.lib.Close(); err != nil {
		slog.Warn("remit: close library", "err", err)
	}
	a.lib = nil
}

// loadSettings reads the app's settings from its data directory, so that a
// choice made in an earlier run is shown in this one. Settings that cannot be
// read leave the defaults and a log line: a broken settings file must not keep
// the window from opening.
func (a *App) loadSettings() {
	if a.settingsDir == "" {
		a.syncPrefs()
		return
	}
	s, err := settings.Load(a.settingsDir)
	if err != nil {
		slog.Warn("remit: read settings", "err", err)
		a.syncPrefs()
		return
	}
	a.settings = s
	if a.dir == "" {
		a.dir = s.LibraryDir
	}
	a.syncPrefs()
}

// Settings returns the app's preferences as the last read from or written to
// disk. It is a copy: change them with UpdateSettings, so the write, the page
// and the services that read one preference all see the change.
func (a *App) Settings() settings.Settings { return a.settings }

// UpdateSettings applies fn to the settings, writes them atomically and with
// owner-only permissions, reads them back and tells whoever registered
// OnSettings. What the app and its page show is then what the file holds; a
// write that fails leaves the settings as they were and returns the error.
func (a *App) UpdateSettings(fn func(*settings.Settings)) error {
	next := a.settings
	fn(&next)
	if a.settingsDir != "" {
		if err := settings.Save(a.settingsDir, next); err != nil {
			return err
		}
		saved, err := settings.Load(a.settingsDir)
		if err != nil {
			return err
		}
		next = saved
	}
	a.settings = next
	a.syncPrefs()
	if a.settingsChanged != nil {
		a.settingsChanged(next)
	}
	return nil
}

// OnSettings registers a function told about the settings whenever the app
// writes them, which is how the tray's pause item follows the page and the page
// follows the tray. Setting it again replaces the previous one.
func (a *App) OnSettings(fn func(settings.Settings)) {
	a.settingsChanged = fn
	if fn != nil {
		fn(a.settings)
	}
}

// remember writes the library the user chose into the settings, keeping every
// other preference, so the next start opens it without asking again.
func (a *App) remember(dir string) error {
	return a.UpdateSettings(func(s *settings.Settings) { s.LibraryDir = dir })
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

// dueToday is the store's answer to what is due now.
func (a *App) dueToday() []core.ID {
	cards, err := queue.Today(a.lib, time.Now(), queue.Options{})
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
	a.data = newLibraryData(a.lib.Notes(), a.lib.Cards())
	a.due = a.dueToday()
	a.review.Sync(a.due)
	if a.remaining != nil {
		a.remaining(a.review.Remaining())
	}
	a.loadCurrent()
}

// summaryFor is the finished session's summary as the store reads it: today's
// answers, what tomorrow holds, how many were right.
func (a *App) summaryFor() (stats.Summary, bool) {
	if a.lib == nil {
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
	case modeNotes:
		a.backlinks = nil
		if a.lib != nil && a.editor.draft.ID != "" {
			a.backlinks = a.lib.Backlinks(a.editor.draft.ID)
		}
	case modeReview:
		a.preview, a.hasPreview = a.previewFor()
		a.summary, a.hasSummary = a.summaryFor()
	}
}

// report logs a write that failed and says so where the action was, so a
// failure never looks like a click that did nothing.
func (a *App) report(c *ui.Context, what string, err error) {
	slog.Error("remit: "+what, "err", err)
	c.Toast(m(c).SaveFailed())
}

// libraryErrorView is the window when the library could not be opened: what
// went wrong, and the one action that tries again.
func (a *App) libraryErrorView(c *ui.Context, tk tokens) {
	ui.Column(c).Grow(1).Center().Gap(tk.spaceM).Padding(tk.spaceXXL).Children(func() {
		ui.Text(c, m(c).LibraryError(a.openErr.Error())).
			FontSize(tk.typeBody).TextColor(tk.TextMuted).MaxWidth(messageWidth)
		ui.Button(c, m(c).RetryLibrary()).MinHeight(tk.control).
			OnClick(func() { a.open() })
	})
}
