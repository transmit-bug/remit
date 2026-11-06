package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/transmit-bug/remit/internal/core"
)

// cardFile is the on-disk shape of a card. Overrides may be multi-line and are
// edited by hand, so the file is indented JSON rather than prose.
type cardFile struct {
	Block  core.ID `json:"block"`
	Prompt string  `json:"prompt"`
	Answer string  `json:"answer"`
	Status string  `json:"status"`
	// Kind and Ordinal are written only when they are not a basic card's, so a
	// plain card file stays as it was.
	Kind    string      `json:"kind,omitempty"`
	Ordinal int         `json:"ordinal,omitempty"`
	Created string      `json:"created"`
	Frozen  *frozenFile `json:"frozen,omitempty"`
}

// frozenFile is a card's snapshot of a block it stopped following.
type frozenFile struct {
	Text string `json:"text"`
	At   string `json:"at"`
}

// encodeCard renders a card to its on-disk form.
func encodeCard(card core.Card, created time.Time) ([]byte, error) {
	file := cardFile{
		Block:   card.Ref.Block,
		Prompt:  card.Prompt,
		Answer:  card.Answer,
		Status:  card.Status.String(),
		Ordinal: card.Ordinal,
		Created: formatTime(created),
	}
	if card.Kind != core.KindBasic {
		file.Kind = card.Kind.String()
	}
	if card.Ref.Frozen != nil {
		file.Frozen = &frozenFile{
			Text: card.Ref.Frozen.Text,
			At:   formatTime(card.Ref.Frozen.At),
		}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: encode card %q: %w", card.ID, err)
	}
	return append(data, '\n'), nil
}

// decodeCard reads a card file. A file a person wrote by hand may leave the
// status or the creation time out; an unknown status is still an error, because
// guessing it would put the card in the wrong queue.
func decodeCard(id core.ID, data []byte) (core.Card, time.Time, error) {
	var file cardFile
	if err := json.Unmarshal(data, &file); err != nil {
		return core.Card{}, time.Time{}, fmt.Errorf("store: decode card %q: %w", id, err)
	}
	status := core.StatusNew
	if file.Status != "" {
		parsed, ok := core.ParseStatus(file.Status)
		if !ok {
			return core.Card{}, time.Time{}, fmt.Errorf("store: card %q: unknown status %q", id, file.Status)
		}
		status = parsed
	}
	kind := core.KindBasic
	if file.Kind != "" {
		parsed, ok := core.ParseKind(file.Kind)
		if !ok {
			return core.Card{}, time.Time{}, fmt.Errorf("store: card %q: unknown kind %q", id, file.Kind)
		}
		kind = parsed
	}
	card := core.Card{
		ID:      id,
		Ref:     core.Reference{Block: file.Block},
		Prompt:  file.Prompt,
		Answer:  file.Answer,
		Status:  status,
		Kind:    kind,
		Ordinal: file.Ordinal,
	}
	if file.Frozen != nil {
		at, err := parseTime(file.Frozen.At)
		if err != nil {
			return core.Card{}, time.Time{}, fmt.Errorf("store: card %q: snapshot time: %w", id, err)
		}
		card.Ref.Frozen = &core.Snapshot{Text: file.Frozen.Text, At: at}
	}
	var created time.Time
	if file.Created != "" {
		parsed, err := parseTime(file.Created)
		if err != nil {
			return core.Card{}, time.Time{}, fmt.Errorf("store: card %q: created time: %w", id, err)
		}
		created = parsed
	}
	return card, created, nil
}

// formatTime writes an instant so that reading it back gives the same one.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
