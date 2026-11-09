package today

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// State is the Today screen: the day the queue was built for, what the app
// derived besides the queue itself, and the list's own place among the rows.
//
// The app fills the first fields when it loads the day and after every write; a
// frame only reads them, so nothing here is derived from a store or a clock.
type State struct {
	// Day is the moment the day's queue was built at, which names the date the
	// page prints and the day tomorrow counts from.
	Day time.Time
	// Loaded is set once the app has built the day's queue: before that the
	// page shows its skeleton rather than claiming the day is empty.
	Loaded bool
	// Err is why the queue could not be built, "" when it could. The page says
	// so where the queue would be, and offers to load it again.
	Err string
	// Recalled is how many cards were reviewed today: the store counts a card
	// on the day of its last review.
	Recalled int

	// list keeps the queue's rows: which one the pointer or the keys are on,
	// and how far the list is scrolled.
	list ui.ListState
}

// Deps is what the page asks of the app: the day's queue as the last load left
// it, the note the day is written in, the plugin items it holds, and one
// function per thing the user can do. Every action runs on the UI thread and
// the app does the writing (ADR 0004); the frame asks the store for nothing.
type Deps struct {
	// Today is the day's queue: the notes to read, the notes read but not yet
	// excerpted, and the cards due, in the order they should be worked.
	Today func() queue.Today
	// StartReview opens the recall queue.
	StartReview func()
	// OpenReading opens a note in the reading session, at the block its last
	// session stopped at.
	OpenReading func(core.ID)
	// OpenReadingAt opens a note at a block: the passage a row says is waiting.
	OpenReadingAt func(core.ID, core.ID)
	// OpenPalette opens the command palette, which is the page's own command
	// button.
	OpenPalette func()
	// OpenDaily opens today's note in the notebook, writing it the first time
	// the day is opened: the page's one way into a note.
	OpenDaily func()
	// NewNote writes an empty note and opens it in the notebook: the empty
	// page's action.
	NewNote func()
	// Daily is today's note and whether the library holds it yet: the daily
	// row shows what it last said, or that the day is unwritten.
	Daily func() (core.Note, bool)
	// Tomorrow is how many cards and notes come due on the day after.
	Tomorrow func() int
	// Widgets is the today.widget slot's items, in the order the plugins'
	// sections draw them.
	Widgets func() []contrib.MenuRef
	// WidgetText resolves one of a plugin's messages in this frame's language,
	// as the plugin page's Deps.Message does: a widget's button is a command
	// title the host, not the plugin, draws.
	WidgetText func(plugin, raw string) string
	// RunWidget runs one widget item, which is the host's command runner.
	RunWidget func(contrib.CommandRef)
	// Reload builds the day's queue again: the action of a failed load.
	Reload func()
}
