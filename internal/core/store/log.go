package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// logRecord is one line of reviews.jsonl. A review names its card — or the note
// it read — its rating and its moment; a reading event may also name where the
// reader stopped, what it did to the queue and when a postponed note comes back;
// a void names only the sequence it cancels.
type logRecord struct {
	Seq      int64   `json:"seq"`
	Card     core.ID `json:"card,omitempty"`
	Note     core.ID `json:"note,omitempty"`
	Kind     string  `json:"kind,omitempty"`
	Rating   string  `json:"rating,omitempty"`
	Position core.ID `json:"position,omitempty"`
	Until    string  `json:"until,omitempty"`
	At       string  `json:"at,omitempty"`
	Void     int64   `json:"void,omitempty"`
	// Scheduler names what wrote this review, once logs carried the name.
	Scheduler string `json:"scheduler,omitempty"`
}

// fileMark is a file's size and modification time: enough to tell whether it
// has changed since a reader saw it. A missing file is a zero mark.
type fileMark struct {
	size  int64
	mtime int64
}

// logMark is how far into the log the index has applied: the file mark it had
// when the events were read, and the last sequence it applied there.
type logMark struct {
	fileMark
	seq int64
}

func (m logMark) String() string {
	return fmt.Sprintf("%d:%d:%d", m.size, m.mtime, m.seq)
}

func parseLogMark(s string) (logMark, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return logMark{}, false
	}
	size, sizeErr := strconv.ParseInt(parts[0], 10, 64)
	mtime, mtimeErr := strconv.ParseInt(parts[1], 10, 64)
	seq, seqErr := strconv.ParseInt(parts[2], 10, 64)
	if sizeErr != nil || mtimeErr != nil || seqErr != nil {
		return logMark{}, false
	}
	return logMark{fileMark: fileMark{size: size, mtime: mtime}, seq: seq}, true
}

func marshalRecord(event schedule.Event) ([]byte, error) {
	rec := logRecord{Seq: event.Seq, Card: event.Card, Note: event.Note, Kind: kindName(event.Kind),
		Position: event.Position, At: formatTime(event.At), Scheduler: event.Scheduler}
	if event.Kind == schedule.Reviewed {
		rec.Rating = event.Rating.String()
	}
	if event.Kind == schedule.Postponed {
		// A line postponing a note to the past is one the reader refuses, and a
		// library holding it does not open: refuse to write it in the first place.
		if event.Until.IsZero() || event.Until.Before(event.At) {
			return nil, fmt.Errorf("store: encode review %d: postponed until %v, not after %v",
				event.Seq, event.Until, event.At)
		}
		rec.Until = formatTime(event.Until)
	}
	if event.IsVoid() {
		rec = logRecord{Seq: event.Seq, Void: event.Void}
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("store: encode review %d: %w", event.Seq, err)
	}
	return append(line, '\n'), nil
}

// kindName writes a review's kind as nothing: a log written before reading
// existed carries no kind, and one review is the only kind a card knows.
func kindName(k schedule.Kind) string {
	if k == schedule.Reviewed {
		return ""
	}
	return k.String()
}

// DecodeLogLine reads one log line. A blank line is not an event. It is
// exported for a reader that must not open the library: a checker reports a
// line the store would refuse instead of failing on it.
func DecodeLogLine(line []byte) (schedule.Event, bool, error) {
	if len(bytes.TrimSpace(line)) == 0 {
		return schedule.Event{}, false, nil
	}
	var rec logRecord
	if err := json.Unmarshal(line, &rec); err != nil {
		return schedule.Event{}, false, err
	}
	if rec.Seq <= 0 {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event without a sequence")
	}
	if rec.Void != 0 {
		return schedule.Event{Seq: rec.Seq, Void: rec.Void}, true, nil
	}
	kind, known := schedule.ParseKind(rec.Kind)
	if !known {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: unknown kind %q", rec.Seq, rec.Kind)
	}
	at, err := parseTime(rec.At)
	if err != nil {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: %w", rec.Seq, err)
	}
	if kind != schedule.Reviewed {
		if rec.Note == "" || rec.Card != "" {
			return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: %s names a note and no card", rec.Seq, kind)
		}
		if rec.Rating != "" {
			return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: %s carries a rating", rec.Seq, kind)
		}
		event := schedule.Event{Seq: rec.Seq, Note: rec.Note, Kind: kind, At: at}
		if kind != schedule.Postponed {
			if rec.Until != "" {
				return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: a %s carries an until, which only a postpone has", rec.Seq, kind)
			}
			return event, true, nil
		}
		until, err := postponedUntil(rec, at)
		if err != nil {
			return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: %w", rec.Seq, err)
		}
		event.Until = until
		return event, true, nil
	}
	rating, ok := schedule.ParseRating(rec.Rating)
	if !ok {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: unknown rating %q", rec.Seq, rec.Rating)
	}
	if (rec.Card == "") == (rec.Note == "") {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: names a card or a note, not both", rec.Seq)
	}
	if rec.Until != "" {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: a review carries an until, which only a postpone has", rec.Seq)
	}
	return schedule.Event{Seq: rec.Seq, Card: rec.Card, Note: rec.Note, Rating: rating,
		Position: rec.Position, At: at, Scheduler: rec.Scheduler}, true, nil
}

// postponedUntil reads the time a postponed note comes back, which has to be
// there and has to be after the moment it was postponed: a line that postpones a
// note to the past schedules nothing.
func postponedUntil(rec logRecord, at time.Time) (time.Time, error) {
	if rec.Until == "" {
		return time.Time{}, fmt.Errorf("postponed without an until")
	}
	until, err := parseTime(rec.Until)
	if err != nil {
		return time.Time{}, err
	}
	if until.Before(at) {
		return time.Time{}, fmt.Errorf("postponed until %s, before it was postponed at %s",
			until.UTC().Format(time.RFC3339), at.UTC().Format(time.RFC3339))
	}
	return until, nil
}

// streamLog calls fn for every event from byte offset to the end of the log,
// in the order it was written, and returns the offset just past the last event
// it applied. A final line without a newline is a write that never finished: it
// is not applied and it does not count, so the offset stops at its first byte.
// A line that does not parse anywhere else is an error: the log is the only
// record of what happened.
func (l *Library) streamLog(offset int64, fn func(schedule.Event) error) (int64, error) {
	file, err := os.Open(l.logPath())
	if errors.Is(err, os.ErrNotExist) {
		return offset, nil
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()
	l.sources.log++

	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return 0, err
		}
	}
	reader := bufio.NewReaderSize(file, 64*1024)
	applied := offset
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				break // a torn last line: leave it, and stop before it
			}
			event, ok, decodeErr := DecodeLogLine(line)
			if decodeErr != nil {
				return 0, decodeErr
			}
			if ok {
				if applyErr := fn(event); applyErr != nil {
					return 0, applyErr
				}
			}
			applied += int64(len(line))
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, err
		}
	}
	return applied, nil
}

// readLogFrom reads the log from an offset, and says how far it got: the
// offset just past the last event it applied.
func (l *Library) readLogFrom(offset int64) ([]schedule.Event, int64, error) {
	var events []schedule.Event
	applied, err := l.streamLog(offset, func(ev schedule.Event) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return events, applied, nil
}

// liveBetween keeps the reviews the log still counts, in the order it holds
// them: void events and the reviews they name are both left out, and so is
// everything outside [from, to). A zero bound is open, so
// liveBetween(events, time.Time{}, time.Time{}) is the whole live history.
func liveBetween(events []schedule.Event, from, to time.Time) []schedule.Event {
	voided := map[int64]bool{}
	for _, ev := range events {
		if ev.IsVoid() {
			voided[ev.Void] = true
		}
	}
	out := make([]schedule.Event, 0, len(events))
	for _, ev := range events {
		if ev.IsVoid() || voided[ev.Seq] {
			continue
		}
		if !from.IsZero() && ev.At.Before(from) {
			continue
		}
		if !to.IsZero() && !ev.At.Before(to) {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// History returns one card's live reviews, oldest first: the reviews an undo
// or a plugin's "what happened" view asks for. It is read-only, and a void
// takes its review with it, so an undone answer is not in the history.
func (l *Library) History(cardID core.ID) ([]schedule.Event, error) {
	return l.entryHistory(cardID)
}

// ReadingEvents returns one note's live reading events, oldest first: the
// sessions that recorded where the reader stopped, and the changes to the
// queue's membership. It is what the agent API shows of a reading entry.
func (l *Library) ReadingEvents(noteID core.ID) ([]schedule.Event, error) {
	return l.entryHistory(noteID)
}

// entryHistory returns one entry's live events, oldest first.
func (l *Library) entryHistory(id core.ID) ([]schedule.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	return l.entryEvents(id)
}

// entryEvents returns one entry's live events, oldest first, from a log read.
// The lock is the caller's.
func (l *Library) entryEvents(id core.ID) ([]schedule.Event, error) {
	events, _, err := l.readLogFrom(0)
	if err != nil {
		return nil, err
	}
	live := liveBetween(events, time.Time{}, time.Time{})
	out := make([]schedule.Event, 0, 4)
	for _, ev := range live {
		if ev.Entry() == id {
			out = append(out, ev)
		}
	}
	return out, nil
}

// appendLog writes one event and makes sure it survives a crash: the log is the
// only place scheduling history lives.
func appendLog(path string, event schedule.Event) error {
	line, err := marshalRecord(event)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(line); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
