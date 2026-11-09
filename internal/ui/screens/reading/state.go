package reading

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/locales"
)

// State is the reading session: the queue's order, where the reader is in the
// note on screen, what an excerpt does, and which notes it already ended.
//
// What belongs in it is the app's answer to what is due (queue.Reading), and the
// session's own place in it, so that a reader who leaves the screen for the
// notebook comes back to the same note and the same paragraph.
type State struct {
	queue []core.ID // the note on screen, then those after it
	// pos is the block the reader moved to. Empty means the note's own stored
	// position, which is where the last session stopped.
	pos core.ID
	// passed are the notes this session is done with, read or skipped. A skipped
	// note is still due, so the session has to remember it: moving on must not
	// put it back at the end of its own queue.
	passed map[core.ID]bool
	// read counts the notes this session finished, for the progress line.
	read int
	// excerptCard makes an excerpt a card as well as a note.
	excerptCard bool
	// cursor is the block the reading position is drawn on: it takes the focus
	// back after an excerpt, so excerpting twice does not lose the place.
	cursor ui.Handle
}

// Reset forgets the queue and the session's progress.
func (s *State) Reset() {
	s.queue, s.pos, s.read = nil, "", 0
	s.passed = map[core.ID]bool{}
}

// Sync takes the app's answer to what is due now: a note still due keeps its
// place, one that left the queue leaves, and one that became due joins the end —
// unless this session already passed it, which is what skipping means.
func (s *State) Sync(due []core.ID) {
	head, _ := s.Current()
	live := make(map[core.ID]bool, len(due))
	for _, id := range due {
		live[id] = true
	}
	kept := make([]core.ID, 0, len(due))
	for _, id := range s.queue {
		if live[id] {
			kept = append(kept, id)
			delete(live, id)
		}
	}
	for _, id := range due {
		if live[id] && !s.passed[id] {
			kept = append(kept, id)
			delete(live, id)
		}
	}
	s.queue = kept
	if now, ok := s.Current(); !ok || now != head {
		s.forgetPosition()
	}
}

// forgetPosition drops the block the reader was on, which belongs to the note
// that was on screen rather than to the session.
func (s *State) forgetPosition() { s.pos = "" }

// Current is the note on screen, if there is one.
func (s *State) Current() (core.ID, bool) {
	if len(s.queue) == 0 {
		return "", false
	}
	return s.queue[0], true
}

// Remaining is how many notes are still to read.
func (s *State) Remaining() int { return len(s.queue) }

// Progress is how many notes this session finished, and how many it started
// with: what the status bar draws.
func (s *State) Progress() (done, total int) {
	return s.read, s.read + len(s.queue)
}

// Cursor is the block the reading position is on, given the note on screen: the
// one the reader moved to, or the one the last session stopped at, or the first.
func (s *State) Cursor(note Note) core.ID {
	if len(note.Blocks) == 0 {
		return ""
	}
	if s.pos != "" && note.Index(s.pos) >= 0 {
		return s.pos
	}
	if note.Index(note.Position) >= 0 {
		return note.Position
	}
	return note.Blocks[0].ID
}

// At is the paragraph the reading position is on, counted from one as the
// reader counts, and how many paragraphs the note has.
func (s *State) At(note Note) (int, int) {
	at := note.Index(s.Cursor(note))
	if at < 0 {
		at = 0
	}
	return at + 1, len(note.Blocks)
}

// Choose puts the reading position on a block, which is what clicking a
// paragraph does.
func (s *State) Choose(block core.ID) {
	if block == "" {
		return
	}
	s.pos = block
}

// Begin starts the session at a note: the one the reader asked for comes first,
// then the queue's own order. An empty id starts at the head. Beginning at a note
// derives nothing — the app's queue already holds it — so an id the session does
// not hold leaves the order as it is.
//
// Entering the session resumes where the store says the note was left, not where
// the paragraph was when the session was last left open.
func (s *State) Begin(id core.ID) {
	s.forgetPosition()
	if id == "" {
		return
	}
	at := -1
	for i, held := range s.queue {
		if held == id {
			at = i
			break
		}
	}
	if at <= 0 {
		return
	}
	order := make([]core.ID, 0, len(s.queue))
	order = append(order, id)
	order = append(order, s.queue[:at]...)
	order = append(order, s.queue[at+1:]...)
	s.queue = order
}

// Move puts the reading position delta paragraphs further on, within the note.
func (s *State) Move(note Note, delta int) {
	if len(note.Blocks) == 0 {
		return
	}
	at := note.Index(s.Cursor(note))
	if at < 0 {
		at = 0
	}
	at = max(0, min(at+delta, len(note.Blocks)-1))
	s.pos = note.Blocks[at].ID
}

// Excerpt takes the block at the reading position out into a child note, and
// leaves the reader where they were: excerpting a passage is a note in the
// margin, not a move.
func (s *State) Excerpt(c *ui.Context, note Note, d Deps) {
	block, ok := note.Block(s.Cursor(note))
	if !ok {
		return
	}
	d.Excerpt(note.ID, block.ID, s.excerptCard)
	s.pos = block.ID
	s.cursor.Focus(c)
}

// ExcerptWithCard reports whether an excerpt makes a card too, which is what
// the screen's checkbox holds.
func (s *State) ExcerptWithCard() bool { return s.excerptCard }

// Decision is what the reader decided about the note on screen: to put it away,
// to say it is not worth reading, or to come back to it tomorrow.
type Decision uint8

const (
	// ArchiveDecision takes the note out of the queue for good, keeping its
	// history and its place to resume at.
	ArchiveDecision Decision = iota
	// DismissDecision says the note is not worth reading: the other terminal
	// state, with the other meaning.
	DismissDecision
	// PostponeDecision brings the note back tomorrow, without touching its place
	// in the queue or its rating history.
	PostponeDecision
)

// End carries out a decision on the note on screen: the app writes the event,
// the session moves past the note without recording an answer — ending a note is
// not a difficulty — and the reader is told what happened with the one action
// that takes it back. It is one gesture behind both the buttons and the keys.
func (s *State) End(c *ui.Context, d Deps, note Note, decision Decision) {
	msgs := locales.In(c.Locale())
	var (
		act  func()
		told string
	)
	switch decision {
	case ArchiveDecision:
		act, told = d.Archive, msgs.ReadArchived(note.Title)
	case DismissDecision:
		act, told = d.Dismiss, msgs.ReadDismissed(note.Title)
	default:
		act, told = d.Postpone, msgs.ReadPostponed(note.Title)
	}
	if act == nil {
		return
	}
	act()
	s.AdvancePast(note.ID)
	if d.QueueNote == nil {
		c.Toast(told)
		return
	}
	c.ToastAction(told, msgs.ReadUndo(), func() { d.QueueNote(note.ID) })
}

// Advance moves past the note on screen, counting it when it was read rather
// than skipped. Either way the session is done with it: it stays out of the
// queue until the next session, whatever the store says about its due date.
func (s *State) Advance(read bool) {
	if len(s.queue) == 0 {
		return
	}
	if read {
		s.read++
	}
	s.pass(s.queue[0])
	s.queue = s.queue[1:]
	s.forgetPosition()
}

// AdvancePast moves past a note the reader ended — archived, dismissed or
// postponed — wherever the queue holds it, and records no answer: ending a note
// is not a difficulty. The store's own answer may have taken the note out of the
// queue already, so it is found by its id rather than by being the note on
// screen.
func (s *State) AdvancePast(id core.ID) {
	if id == "" {
		return
	}
	s.pass(id)
	kept := s.queue[:0]
	for i, held := range s.queue {
		if held != id {
			kept = append(kept, held)
			continue
		}
		if i == 0 {
			s.forgetPosition()
		}
	}
	s.queue = kept
}

// pass remembers that this session is done with a note: a skipped note is still
// due, so moving on must not put it back at the end of its own queue.
func (s *State) pass(id core.ID) {
	if s.passed == nil {
		s.passed = map[core.ID]bool{}
	}
	s.passed[id] = true
}
