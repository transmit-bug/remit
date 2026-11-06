package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

// logRecord is one line of reviews.jsonl. A review names its card, its rating
// and its moment; a void names only the sequence it cancels.
type logRecord struct {
	Seq    int64   `json:"seq"`
	Card   core.ID `json:"card,omitempty"`
	Rating string  `json:"rating,omitempty"`
	At     string  `json:"at,omitempty"`
	Void   int64   `json:"void,omitempty"`
}

func marshalRecord(event schedule.Event) ([]byte, error) {
	rec := logRecord{Seq: event.Seq, Card: event.Card, Rating: event.Rating.String(), At: formatTime(event.At)}
	if event.IsVoid() {
		rec = logRecord{Seq: event.Seq, Void: event.Void}
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("store: encode review %d: %w", event.Seq, err)
	}
	return append(line, '\n'), nil
}

// decodeRecord reads one log line. A blank line is not an event.
func decodeRecord(line []byte) (schedule.Event, bool, error) {
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
	rating, ok := schedule.ParseRating(rec.Rating)
	if !ok {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: unknown rating %q", rec.Seq, rec.Rating)
	}
	at, err := parseTime(rec.At)
	if err != nil {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: %w", rec.Seq, err)
	}
	if rec.Card == "" {
		return schedule.Event{}, false, fmt.Errorf("store: review log: event %d: no card", rec.Seq)
	}
	return schedule.Event{Seq: rec.Seq, Card: rec.Card, Rating: rating, At: at}, true, nil
}

// readLog reads the review log in the order it was written. A final line that
// does not parse is a write that never finished, so it is dropped rather than
// made to fail the whole library; anything broken earlier is an error.
func readLog(path string) ([]schedule.Event, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(data, []byte("\n"))
	var events []schedule.Event
	for i, line := range lines {
		event, ok, err := decodeRecord(line)
		if err != nil {
			if i == len(lines)-1 && !bytes.HasSuffix(data, []byte("\n")) {
				break
			}
			return nil, err
		}
		if ok {
			events = append(events, event)
		}
	}
	return events, nil
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
