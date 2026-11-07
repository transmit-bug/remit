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
		out = append(out, Reading{Note: row.note, State: row.state, Position: row.position, Queued: true})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Due(), out[j].Due()
		if !a.Equal(b) {
			return a.Before(b)
		}
		return out[i].Note < out[j].Note
	})
	return out, nil
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
	return Reading{Note: row.note, State: row.state, Position: row.position, Queued: row.queued}, true
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

// AddToReading puts a note in the reading queue. A note that is already there is
// left alone, so adding it twice writes one event. The note has to be in the
// library: a queue naming nothing would never come up.
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

// readingParams is the schedule reading entries are replayed with: the host's
// own defaults, independent of the library's card schedule.
func (l *Library) readingParams() schedule.Params { return l.readingDefaults.Normalized() }

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
func applyReading(row readingRow, ev schedule.Event, params schedule.Params) readingRow {
	if row.note == "" {
		row.note = ev.Note
	}
	switch ev.Kind {
	case schedule.Queued:
		row.queued = true
		if row.state.Reviews == 0 {
			// A note never read is due when it joined the queue: the reading
			// queue is filled in the order notes were added to it.
			row.state.Due = ev.At.UTC()
		}
	case schedule.Dropped:
		row.queued = false
	case schedule.Reviewed:
		if ev.Rating.Valid() {
			row.state = params.Normalized().Step(row.state, ev)
		}
		if ev.Position != "" {
			row.position = ev.Position
		}
		row.lastSeq = ev.Seq
		row.queued = true
	}
	return row
}
