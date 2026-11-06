package ui

import (
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/store"
)

// Options configures an App.
type Options struct {
	// LibraryDir is the directory the library lives in. Empty means Remit's
	// own place, <Documents>/Remit, which is also where a person looks for
	// their notes; tests pass t.TempDir() so they never touch it.
	LibraryDir string
}

// libraryData is the library as the views read it: a snapshot the app retakes
// after every write, so that no frame has to touch the store. Everything the
// notes list, the editor and review show is derived here once, per write.
type libraryData struct {
	notes  []core.Note
	byID   map[core.ID]core.Card
	blocks core.BlockSet

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
		byID:     make(map[core.ID]core.Card, len(cards)),
		blocks:   make(core.BlockSet),
		perBlock: make(map[core.ID]int, len(cards)),
		perNote:  make(map[core.ID]int, len(notes)),
		noteOf:   make(map[core.ID]core.ID, len(notes)),
	}
	for _, note := range notes {
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

// messageWidth caps a message that is about the library rather than a card:
// the error state's one line, centred in the window.
const messageWidth float32 = 520

// open opens the library, creating it the first time, and reads it into the
// app. A library that cannot be opened is a state the window explains, never a
// reason to panic: what the user sees is one line and one action.
func (a *App) open() {
	a.openErr = nil
	dir, err := a.libraryDir()
	if err != nil {
		a.openErr = err
		slog.Error("remit: locate library", "err", err)
		return
	}
	lib, err := store.Open(dir, store.Options{})
	if errors.Is(err, fs.ErrNotExist) {
		lib, err = store.Create(dir, store.Options{})
	}
	if err != nil {
		a.openErr = err
		slog.Error("remit: open library", "dir", dir, "err", err)
		return
	}
	a.lib, a.dir = lib, dir
	a.review.reset()
	a.reload()
	if a.opened != nil {
		a.opened(lib)
	}
}

// libraryDir is where the library lives: the directory the app was given, or
// Remit's own folder in the user's Documents. It is resolved once and kept, so
// trying again after a failure opens the same place.
func (a *App) libraryDir() (string, error) {
	if a.dir != "" {
		return a.dir, nil
	}
	return DefaultLibraryDir()
}

// DefaultLibraryDir is where a library lives when the caller names none:
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
// now. A missing library is created, so a link or a dropped archive can point
// at a place the app has not been before.
func (a *App) OpenLibrary(dir string) {
	if a.lib != nil {
		if err := a.lib.Close(); err != nil {
			slog.Warn("remit: close library", "err", err)
		}
		a.lib = nil
	}
	a.dir = dir
	a.review.reset()
	a.open()
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

// reload retakes the snapshot and asks the store what is due now. Every write
// ends here, which is what keeps the build pass free of store calls.
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
	a.review.sync(a.dueToday())
	if a.remaining != nil {
		a.remaining(a.review.remaining())
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
			FontSize(tk.typeBody).TextColor(tk.textMuted).MaxWidth(messageWidth)
		ui.Button(c, m(c).RetryLibrary()).MinHeight(tk.control).
			OnClick(func() { a.open() })
	})
}
