// Package read is the read area's two pages: the queue of notes due to read
// today, and the archive of notes that left that queue for good.
//
// The rows are the app's — it derives them from the store's reading entries —
// so the pages read them through Deps and report what the user did; every write
// stays in the app. Nothing here reads the store, a clock or the disk.
package read

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// Page is which of the area's two pages is up. The app sets it from the page the
// sub-navigation chose, so the page on screen and the page the rail names are one.
type Page uint8

const (
	// Queue is the notes due to read now, with the rhythm of the whole queue
	// under them.
	Queue Page = iota
	// Archive is what left the queue for good: the archived and the dismissed.
	Archive
)

// Item is one note a page lists: what it is called, how far into it the reader
// got, and when that was. The app derives it, so a page needs no note file.
type Item struct {
	Note  core.ID
	Title string
	// Blocks is how many paragraphs the note holds; At is the paragraph the
	// last session stopped at, counted from one, 0 when none did.
	Blocks int
	At     int
	// LastSeen is when a session last read it, zero when none did.
	LastSeen time.Time
	// Due is when the queue holds it: when an unread note joined, or the day
	// the schedule brought it back.
	Due time.Time
}

// Progress is how much of the note a session has taken in, from 0 to 1: what a
// row's meter shows.
func (i Item) Progress() float64 {
	if i.Blocks <= 0 {
		return 0
	}
	return min(1, float64(i.At)/float64(i.Blocks))
}

// Counts is the queue's rhythm: how many notes are due today, and how many the
// reading queue holds at all — due or waiting for the schedule.
type Counts struct {
	Due    int
	Queued int
}

// State is the pages' own memory: which page is up, what the last load left, and
// each list's place. The app answers the first three through Show and its load;
// the lists' place is the pages' own.
type State struct {
	Page Page
	// Loaded reports whether the app has answered at least once: a page drawn
	// before that says it is loading rather than that it is empty.
	Loaded bool
	// Err is why the last load failed, "" when it did not. A failed page shows
	// it and the one action that asks again.
	Err string

	queue   ui.ListState
	archive ui.ListState
}

// Show tells the page which of the area's pages the window is on.
func (s *State) Show(page Page) { s.Page = page }

// Deps is what the pages ask of the app: the rows it derived, the clock a
// "today" is measured against, and what the user can do with a note. Each runs
// on the UI thread; the app does the reading and the writing.
type Deps struct {
	// Queue is the notes due to read now, in the order the session reads them.
	Queue func() []Item
	// Archive is the notes that left the queue for good, in the two sections the
	// page draws: the archived, then the dismissed.
	Archive func() (archived, dismissed []Item)
	// Counts is the rhythm under the queue: due today, and held at all.
	Counts func() Counts
	// Now is the clock a row's "today" is measured against, so that the pages
	// read no clock of their own.
	Now func() time.Time
	// Start enters the reading session at the head of the queue.
	Start func()
	// Continue enters the session at one note.
	Continue func(core.ID)
	// Add shows the notebook's notes, where a note is queued: the read area
	// cannot make a note, so this is the action that fills an empty queue.
	Add func()
	// Open shows a note in the notebook.
	Open func(core.ID)
	// Restore puts an archived or dismissed note back in the queue.
	Restore func(core.ID)
	// Reload asks for the pages again: the action of a failed load.
	Reload func()
}

// View draws the page the app says is up: the queue of notes due, or the
// archive. A load that failed says so and offers to ask again; a load still on
// its way says it is loading; neither is an empty page.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	title := msgs.ReadQueueTitle()
	page := Queue
	if s.Page == Archive {
		title, page = msgs.ReadArchiveTitle(), Archive
	}
	ready := s.Loaded && s.Err == ""
	var items []Item
	if ready && page == Queue {
		items = d.Queue()
	}

	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.ListWidth).Grow(1).Gap(tk.SpaceL).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, title).FontSize(tk.TypeH1).Bold()
				ui.Spacer(c)
				if ready && page == Queue {
					queueActions(c, tk, msgs, len(items), d)
				}
			})

			switch {
			case s.Err != "":
				failed(c, tk, msgs, s.Err, d)
			case !s.Loaded:
				loading(c, tk, msgs)
			case page == Archive:
				archivePage(c, s, d, msgs, tk)
			default:
				queuePage(c, s, d, msgs, tk, items)
			}
		})
	})
}

// queueActions are the queue page's two ways in: a note to add, and the session
// itself. Start is disabled on an empty queue, and the empty state below it is
// the reason — a disabled control shows no tooltip, so the answer is on screen.
func queueActions(c *ui.Context, tk design.Tokens, msgs locales.Messages, due int, d Deps) {
	if d.Add != nil {
		ui.Button(c, msgs.ReadAdd()).MinHeight(tk.Control).OnClick(d.Add)
	}
	start := ui.PrimaryButton(c, msgs.ReadStart()).MinHeight(tk.Control)
	if due == 0 {
		start.Disabled(true)
	} else if d.Start != nil {
		start.OnClick(d.Start)
	}
}

// queuePage is the notes due to read now, each with where it resumes, how far a
// session got, and the way back in; the rhythm of the whole queue sits under
// them.
func queuePage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens, items []Item) {
	if len(items) == 0 {
		kit.Empty(c, "book-open", msgs.ReadQueueEmpty(), func() {
			if d.Add != nil {
				ui.PrimaryButton(c, msgs.ReadAddNote()).MinHeight(tk.Control).OnClick(d.Add)
			}
		})
		// The rhythm stays under the empty state: it says whether the queue is
		// empty or only quiet, which is the difference between nothing waiting
		// and nothing due today.
		rhythm(c, tk, msgs, d.Counts())
		return
	}
	now := d.Now()
	s.queue.Key = func(row int) any { return items[row].Note }
	s.queue.Label = func(row int) string { return items[row].Title }
	ui.Divider(c)
	// The queue arrives as a list: each note behind the one above it, once, when
	// the page appears.
	holder := ui.Column(c).Grow(1)
	arrive := kit.Stagger(holder, "read-queue")
	holder.Children(func() {
		ui.List(c, &s.queue, len(items), func(i int) {
			queueRow(c, tk, msgs, items[i], arrive(i), now, d)
		}).Grow(1).Label(msgs.ReadQueueTitle())
	})
	rhythm(c, tk, msgs, d.Counts())
}

// queueRow is one note waiting to be read: its title, where the last session
// stopped, its paragraph progress, and the button that opens the session there.
func queueRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, it Item, shown float32, now time.Time, d Deps) {
	kit.Arrive(ui.Row(c).Gap(tk.SpaceM).AlignItems(ui.Center).Padding(tk.SpaceM, tk.SpaceS).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border), shown).Children(func() {
		ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, it.Title).FontSize(tk.TypeUI).SingleLine()
			ui.Text(c, place(msgs, it, now)).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted).SingleLine()
		})
		ui.Meter(c, it.Progress(), 0, 1, nil).Width(tk.MeterWidth).
			Label(meterLabel(msgs, it))
		if d.Continue != nil {
			// A row's buttons carry the note's name for a screen reader: three
			// rows of "Continue" say nothing about which one continues where.
			ui.Button(c, msgs.ReadContinue()).Label(msgs.ReadContinue() + " " + it.Title).
				MinHeight(tk.Control).OnClick(func() { d.Continue(it.Note) })
		}
	})
}

// meterLabel names what a row's meter measures, for a screen reader: the
// paragraph a session stopped at, or that none did.
func meterLabel(msgs locales.Messages, it Item) string {
	if it.At == 0 {
		return msgs.ReadRowUnread()
	}
	return msgs.ReadRowPlace(it.At, it.Blocks)
}

// place is where a row stands in one line: the paragraph it resumes at, or that
// it was never started, and the day that was.
func place(msgs locales.Messages, it Item, now time.Time) string {
	stopped := day(msgs, it.LastSeen, now)
	if it.At > 0 {
		line := msgs.ReadRowPlace(it.At, it.Blocks)
		if stopped != "" {
			line += " · " + msgs.ReadRowStopped(stopped)
		}
		return line
	}
	line := msgs.ReadRowUnread()
	if joined := day(msgs, it.Due, now); joined != "" {
		line += " · " + msgs.ReadRowQueued(joined)
	}
	return line
}

// day names the day something happened on: today and yesterday by their word,
// anything older by its date, so a row never asks the reader to count back. A
// time that never was names no day at all.
func day(msgs locales.Messages, at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	switch at = at.In(now.Location()); {
	case sameDay(at, now):
		return msgs.ReadDayToday()
	case sameDay(at, now.AddDate(0, 0, -1)):
		return msgs.ReadDayYesterday()
	}
	return at.Format("2006-01-02")
}

// sameDay reports whether two times fall on one calendar day.
func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// rhythm is the three readings under the queue: how many notes are due today,
// how many the queue holds, and the three answers a reading session records.
// They are numbers to read, not controls to press.
func rhythm(c *ui.Context, tk design.Tokens, msgs locales.Messages, counts Counts) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		kit.SectionHead(c, tk, msgs.ReadRhythm(), "", nil)
		kit.StyleCard(c, ui.Row(c).Padding(tk.SpaceM, tk.SpaceL).Gap(tk.SpaceL).
			AlignItems(ui.Center)).Children(func() {
			kit.Stat(c, count(counts.Due), msgs.ReadRhythmDue()).Grow(1)
			ui.Divider(c).Height(tk.SpaceXL).AlignSelf(ui.Center)
			kit.Stat(c, count(counts.Queued), msgs.ReadRhythmQueued()).Grow(1)
			ui.Divider(c).Height(tk.SpaceXL).AlignSelf(ui.Center)
			ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
				ui.Row(c).Gap(tk.SpaceXS).Wrap().Children(func() {
					for _, tier := range []struct {
						label  string
						colour ui.Color
					}{
						{msgs.ReadingTooHard(), tk.Danger},
						{msgs.ReadingJustRight(), tk.Success},
						{msgs.ReadingTooShallow(), tk.Accent},
					} {
						ui.Badge(c, tier.label).
							Background(tier.colour.Alpha(0.12)).
							TextColor(tier.colour.Mix(tk.Text, 0.35))
					}
				})
				ui.Text(c, msgs.ReadRhythmTiers()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			})
		})
	})
}

// archivePage lists what left the queue for good, in two sections: the notes put
// away after reading, and those marked as not worth reading. A section with no
// rows is not drawn: an empty heading promises rows that are not there.
func archivePage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	archived, dismissed := d.Archive()
	rows := make([]archiveRow, 0, len(archived)+len(dismissed)+2)
	rows = appendSection(rows, msgs.ReadArchivedSection(), archived)
	rows = appendSection(rows, msgs.ReadDismissedSection(), dismissed)
	if len(rows) == 0 {
		kit.Empty(c, "square-check", msgs.ReadArchiveEmpty(), func() {
			ui.Text(c, msgs.ReadArchiveHint()).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted).Wrap()
		})
		return
	}
	s.archive.Key = func(row int) any { return rows[row].key() }
	s.archive.Label = func(row int) string {
		if rows[row].section != "" {
			return rows[row].section
		}
		return rows[row].item.Title
	}
	s.archive.Header = func(row int) bool { return rows[row].section != "" }
	ui.List(c, &s.archive, len(rows), func(i int) {
		if rows[i].section != "" {
			sectionHead(c, tk, msgs, rows[i].section, rows[i].count)
			return
		}
		archiveRowView(c, tk, msgs, rows[i].item, d)
	}).Grow(1).Label(msgs.ReadArchiveTitle())
}

// archiveRow is one line of the archive page: either a section's heading with
// how many notes it holds, or a note.
type archiveRow struct {
	section string // "" on a note's row
	count   int    // the heading's notes
	item    Item
}

// key is the row's identity in the list: a section's heading is the section, a
// note is the note — the two can never collide.
func (r archiveRow) key() string {
	if r.section != "" {
		return "section:" + r.section
	}
	return string(r.item.Note)
}

func appendSection(rows []archiveRow, section string, items []Item) []archiveRow {
	if len(items) == 0 {
		return rows
	}
	rows = append(rows, archiveRow{section: section, count: len(items)})
	for _, item := range items {
		rows = append(rows, archiveRow{item: item})
	}
	return rows
}

// sectionHead is one section's name and how many notes it holds. It stays at the
// top of the list while its own rows scroll under it.
func sectionHead(c *ui.Context, tk design.Tokens, msgs locales.Messages, section string, count int) {
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Padding(tk.SpaceS, 0).
		Background(tk.Surface).Children(func() {
		ui.Text(c, section).FontSize(tk.TypeUI).FontWeight(600)
		ui.Text(c, msgs.ReadNotesMark(count)).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		ui.Spacer(c)
	})
}

// archiveRowView is one note the archive holds: what it is, how long it is, and
// the two things that can be done with it — open it in the notebook, or put it
// back in the queue.
func archiveRowView(c *ui.Context, tk design.Tokens, msgs locales.Messages, it Item, d Deps) {
	ui.Row(c).Gap(tk.SpaceM).AlignItems(ui.Center).Padding(tk.SpaceM, tk.SpaceS).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border).Children(func() {
		ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, it.Title).FontSize(tk.TypeUI).SingleLine()
			ui.Text(c, archivePlace(msgs, it)).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted).SingleLine()
		})
		if d.Open != nil {
			ui.Button(c, msgs.ReadOpen()).Label(msgs.ReadOpen() + " " + it.Title).
				MinHeight(tk.Control).OnClick(func() { d.Open(it.Note) })
		}
		if d.Restore != nil {
			ui.Button(c, msgs.ReadRestore()).Label(msgs.ReadRestore() + " " + it.Title).
				MinHeight(tk.Control).OnClick(func() { d.Restore(it.Note) })
		}
	})
}

// archivePlace is how far into an archived note a session got. It is the note's
// length when it was put away without being read, so the row still says what the
// note is worth opening.
func archivePlace(msgs locales.Messages, it Item) string {
	if it.At > 0 {
		return msgs.ReadRowPlace(it.At, it.Blocks)
	}
	return msgs.ReadRowBlocks(it.Blocks)
}

// loading is the page before its first answer: the head is up and the body says
// the queue is on its way.
func loading(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	ui.Row(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).
		Gap(tk.SpaceS).Children(func() {
		ui.Spinner(c)
		ui.Text(c, msgs.ReadLoading()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	})
}

// failed is a page whose rows did not arrive: what went wrong, and the one
// action that asks again.
func failed(c *ui.Context, tk design.Tokens, msgs locales.Messages, reason string, d Deps) {
	ui.Column(c).Grow(1).Gap(tk.SpaceM).Children(func() {
		ui.Text(c, msgs.ReadLoadFailed(reason)).FontSize(tk.TypeUI).TextColor(tk.Danger)
		if d.Reload != nil {
			ui.Button(c, msgs.ReadRetry()).MinHeight(tk.Control).OnClick(d.Reload)
		}
	})
}

// count is a number a statistic shows.
func count(n int) string { return strconv.Itoa(n) }
