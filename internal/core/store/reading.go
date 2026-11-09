package store

import (
	"fmt"
	"sort"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// Reading is a note in the reading queue: the schedule its reading history
// gives it, and the block the last session stopped at.
//
// A note that was queued and never read is due at the moment it joined the
// queue, which is what fills the queue in the order notes were added to it.
type Reading struct {
	Note     core.ID
	State    schedule.State
	Position core.ID
	Queued   bool
	// Archived and Dismissed are the two terminal states: the note left today's
	// queue for good, and only being queued again brings it back.
	Archived  bool
	Dismissed bool
	// Until is when a postponed note comes due, while that postpone still
	// governs its due date.
	Until time.Time
}

// Due is when the note comes up in the reading queue.
func (r Reading) Due() time.Time { return r.State.Due }

// ReadingQueue returns the notes the reading queue holds now, soonest due
// first, then by ID so two calls agree. It reads the index's answer: no note
// file is opened and no log line replayed.
func (l *Library) ReadingQueue() ([]Reading, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	out := make([]Reading, 0, len(l.readings))
	for _, row := range l.readings {
		if !row.queued {
			continue
		}
		out = append(out, row.reading())
	}
	sortReadings(out)
	return out, nil
}

// ReadingArchive returns the notes that left the reading queue for good —
// archived and dismissed — soonest due first, then by ID. They are out of
// today's queue and still in the library, which is what the archive lists.
func (l *Library) ReadingArchive() ([]Reading, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	out := make([]Reading, 0, len(l.readings))
	for _, row := range l.readings {
		if !row.archived && !row.dismissed {
			continue
		}
		out = append(out, row.reading())
	}
	sortReadings(out)
	return out, nil
}

// sortReadings puts soonest due first, then by ID, so two calls agree.
func sortReadings(out []Reading) {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Due(), out[j].Due()
		if !a.Equal(b) {
			return a.Before(b)
		}
		return out[i].Note < out[j].Note
	})
}

// ReadingFor returns where one note stands in the reading queue, queued or not:
// the position a session resumes at is kept after a note leaves the queue.
func (l *Library) ReadingFor(noteID core.ID) (Reading, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	row, ok := l.readings[noteID]
	if !ok {
		return Reading{}, false
	}
	return row.reading(), true
}

// ReadingPosition is the block a note's last session stopped at, which is where
// opening it again puts the reader.
func (l *Library) ReadingPosition(noteID core.ID) (core.ID, bool) {
	reading, ok := l.ReadingFor(noteID)
	if !ok || reading.Position == "" {
		return "", false
	}
	return reading.Position, true
}

// AddToReading puts a note in the reading queue, and takes it out of the archive
// if it was archived or dismissed: being queued again is how a note comes back.
// A note that is already there is left alone, so adding it twice writes one
// event. The note has to be in the library: a queue naming nothing would never
// come up.
func (l *Library) AddToReading(noteID core.ID, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if _, ok := l.index.note(noteID); !ok {
		return fmt.Errorf("store: no note %q", noteID)
	}
	if row, ok := l.readings[noteID]; ok && row.queued {
		return nil
	}
	return l.recordReading(schedule.Event{Seq: l.nextSeq + 1, Note: noteID, Kind: schedule.Queued,
		At: l.instant(at)})
}

// RemoveFromReading takes a note out of the reading queue. The note keeps its
// history and the block it stopped at, so adding it back resumes where it left
// off.
func (l *Library) RemoveFromReading(noteID core.ID, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if row, ok := l.readings[noteID]; !ok || !row.queued {
		return nil
	}
	return l.recordReading(schedule.Event{Seq: l.nextSeq + 1, Note: noteID, Kind: schedule.Dropped,
		At: l.instant(at)})
}

// ArchiveReading ends a note's reading: it leaves the queue for good, keeps its
// history and its position, and comes back only if it is queued again.
//
// Archive and Dismiss are the two terminal states a note can reach, and each
// replaces the other. Undoing either is writing the opposite event —
// AddToReading — not a void: a void takes back an answer, not a decision.
func (l *Library) ArchiveReading(noteID core.ID, at time.Time) error {
	return l.endReading(noteID, schedule.Archived, at)
}

// DismissReading says a note is not worth reading: the same terminal state as an
// archive, with the other meaning, so the two queries stay apart.
func (l *Library) DismissReading(noteID core.ID, at time.Time) error {
	return l.endReading(noteID, schedule.Dismissed, at)
}

func (l *Library) endReading(noteID core.ID, kind schedule.Kind, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if _, ok := l.readings[noteID]; !ok {
		return fmt.Errorf("%w: note %q", ErrNoReading, noteID)
	}
	return l.recordReading(schedule.Event{Seq: l.nextSeq + 1, Note: noteID, Kind: kind,
		At: l.instant(at)})
}

// PostponeReading moves a note's due date to until without touching its place in
// the queue or its rating history: the reader means to come back to it later.
func (l *Library) PostponeReading(noteID core.ID, until, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if _, ok := l.index.note(noteID); !ok {
		return fmt.Errorf("store: no note %q", noteID)
	}
	until = l.instant(until)
	at = l.instant(at)
	if until.Before(at) {
		return fmt.Errorf("store: postponed until %s, before %s", until.Format(time.RFC3339), at.Format(time.RFC3339))
	}
	return l.recordReading(schedule.Event{Seq: l.nextSeq + 1, Note: noteID, Kind: schedule.Postponed,
		Until: until, At: at})
}

// ReadNote records a reading session: how hard the note was to digest, and the
// block the reading stopped at. The note joins the reading queue if it was not
// in it, because a note someone read is a note they mean to remember.
func (l *Library) ReadNote(noteID core.ID, rating schedule.Rating, position core.ID, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if _, ok := l.index.note(noteID); !ok {
		return fmt.Errorf("store: no note %q", noteID)
	}
	if !rating.Valid() {
		return fmt.Errorf("store: unknown rating %d", uint8(rating))
	}
	return l.recordReading(schedule.Event{Seq: l.nextSeq + 1, Note: noteID, Rating: rating,
		Position: position, At: l.instant(at)})
}

// UndoReading cancels a note's newest live reading session by appending the void
// that names it, and derives the note's place in the queue again from what is
// left: the due date and the reading position go back to those of the session
// before it. The log is never rewritten, so an undo is as durable as the session
// it undoes.
//
// A terminal state is not an answer, so it is not voided: an archived or
// dismissed note comes back by being queued again, not by voiding the event that
// took it out. A note with no live session to take back reports ErrNoReview.
func (l *Library) UndoReading(noteID core.ID, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if _, ok := l.index.note(noteID); !ok {
		return fmt.Errorf("store: no note %q", noteID)
	}

	events, err := l.entryEvents(noteID)
	if err != nil {
		return err
	}
	applier := schedule.NewApplierFor(l.params, l.readingParams())
	if err := applier.Apply(events...); err != nil {
		return fmt.Errorf("store: review log: %w", err)
	}
	last, ok := applier.Last(noteID)
	if !ok {
		return fmt.Errorf("%w: note %q", ErrNoReview, noteID)
	}

	void := schedule.Event{Seq: l.nextSeq + 1, Void: last.Seq, At: l.instant(at)}
	if err := appendLog(l.logPath(), void); err != nil {
		return err
	}
	if err := applier.Apply(void); err != nil {
		return fmt.Errorf("store: review log: %w", err)
	}
	undone, known := applier.ReadingOf(noteID)
	row := readingRowFrom(noteID, undone)
	row.lastSeq = newestLiveSeq(append(events, void), noteID)

	marks, err := l.sourceMarks()
	if err != nil {
		return err
	}
	applied, markOK := l.appliedLogMark(void.Seq)
	err = l.index.write(func(e execer) error {
		if known {
			if err := putReading(e, row); err != nil {
				return err
			}
		} else if err := deleteReading(e, noteID); err != nil {
			// The void took the note's last live event with it: it is in no
			// queue and has no place to resume at, exactly as replay reads it.
			return err
		}
		if err := l.putMarks(e, marks); err != nil {
			return err
		}
		if !markOK {
			return clearLogMark(e)
		}
		return l.putLogMark(e, applied)
	})
	if err != nil {
		return err
	}
	l.nextSeq = void.Seq
	if known {
		l.readings[noteID] = &row
	} else {
		delete(l.readings, noteID)
	}
	l.emit(EventReadingRecorded, noteID)
	return nil
}

// readingParams is the schedule reading entries are replayed with: the host's
// own defaults, independent of the library's card schedule.
func (l *Library) readingParams() schedule.Params { return l.readingDefaults.Normalized() }

// readingRowFrom is the row a replay result spells, so the write path and the
// rebuild store the same thing.
func readingRowFrom(note core.ID, reading schedule.Reading) readingRow {
	return readingRow{note: note, state: reading.State, position: reading.Position,
		queued: reading.Queued, archived: reading.Archived, dismissed: reading.Dismissed,
		until: reading.Until}
}

// reading is the row as the queue and the archive show it.
func (r *readingRow) reading() Reading {
	return Reading{Note: r.note, State: r.state, Position: r.position, Queued: r.queued,
		Archived: r.archived, Dismissed: r.dismissed, Until: r.until}
}

// recordReading appends a reading event to the log and brings the note's place
// in the queue with it. The log line is written first, as a card's review is: an
// event that survives a crash is replayed, and one that does not was never
// confirmed to the caller.
func (l *Library) recordReading(event schedule.Event) error {
	if err := appendLog(l.logPath(), event); err != nil {
		return err
	}
	row := readingRow{note: event.Note}
	if current, ok := l.readings[event.Note]; ok {
		row = *current
	}
	row = applyReading(row, event, l.readingParams())

	marks, err := l.sourceMarks()
	if err != nil {
		return err
	}
	applied, ok := l.appliedLogMark(event.Seq)
	err = l.index.write(func(e execer) error {
		if err := putReading(e, row); err != nil {
			return err
		}
		if err := l.putMarks(e, marks); err != nil {
			return err
		}
		if !ok {
			return clearLogMark(e)
		}
		return l.putLogMark(e, applied)
	})
	if err != nil {
		return err
	}
	l.nextSeq = event.Seq
	l.readings[event.Note] = &row
	switch event.Kind {
	case schedule.Queued:
		l.emit(EventReadingQueued, event.Note)
	case schedule.Dropped:
		l.emit(EventReadingDropped, event.Note)
	default:
		l.emit(EventReadingRecorded, event.Note)
	}
	return nil
}

// applyReading is what one reading event does to a note's row. Replay and the
// write path share it, so an event applied now and one replayed at the next open
// leave the same row.
//
// The rules are the ones schedule's derive replays: a session brings the note
// into the queue and sets the place it stopped at; the two terminal kinds take
// it out and replace each other; a postpone moves the due date and leaves
// membership and the rating history alone; a queued event ends either terminal
// state and schedules an unread note from now.
func applyReading(row readingRow, ev schedule.Event, params schedule.Params) readingRow {
	if row.note == "" {
		row.note = ev.Note
	}
	row.lastSeq = ev.Seq
	switch ev.Kind {
	case schedule.Queued:
		row.queued, row.archived, row.dismissed = true, false, false
		if row.state.Reviews == 0 {
			// A note never read is due when it joined the queue: the reading
			// queue is filled in the order notes were added to it.
			row.state.Due = ev.At.UTC()
			row.until = time.Time{}
		}
	case schedule.Dropped:
		row.queued = false
	case schedule.Archived:
		row.queued, row.archived, row.dismissed = false, true, false
	case schedule.Dismissed:
		row.queued, row.archived, row.dismissed = false, false, true
	case schedule.Postponed:
		row.state.Due = ev.Until.UTC()
		row.until = ev.Until.UTC()
	case schedule.Reviewed:
		if ev.Rating.Valid() {
			row.state = params.Normalized().Step(row.state, ev)
		}
		if ev.Position != "" {
			row.position = ev.Position
		}
		row.queued, row.archived, row.dismissed = true, false, false
		row.until = time.Time{}
	}
	return row
}

// newestLiveSeq is the sequence of the newest live event of one entry, which is
// what the index caches beside its state.
func newestLiveSeq(events []schedule.Event, id core.ID) int64 {
	var seq int64
	for _, ev := range liveBetween(events, time.Time{}, time.Time{}) {
		if ev.Entry() == id {
			seq = ev.Seq
		}
	}
	return seq
}
