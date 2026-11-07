package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/transmit-bug/remit/internal/core"
)

// cardFile is the on-disk shape of a card (format remit/2). A card persists
// only what identifies it — type, ordinal, producer — plus the values the user
// hand-edited; its other field values are resolved from the block (live) or
// the frozen snapshot at render time, never stored here. Files are edited by
// hand, so the JSON is indented rather than prose.
type cardFile struct {
	Block core.ID `json:"block"`
	Type  core.ID `json:"type"`
	// Ordinal is written only when it is not the first card of its paragraph,
	// so a plain card file stays as it was.
	Ordinal int `json:"ordinal,omitempty"`
	// Producer is written only when a plugin made the card; host-made cards
	// carry none.
	Producer *producerFile `json:"producer,omitempty"`
	// Fields holds only hand-edited values (Edited says there are any). They
	// override what the source resolves, the way prompt/answer overrides used
	// to before format remit/2.
	Fields  map[string]string `json:"fields,omitempty"`
	Edited  bool              `json:"edited,omitempty"`
	Status  string            `json:"status"`
	Created string            `json:"created"`
	Frozen  *frozenFile       `json:"frozen,omitempty"`
}

// producerFile is a card's record of what derived it.
type producerFile struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
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
		Type:    card.Type,
		Ordinal: card.Ordinal,
		Fields:  card.Fields,
		Edited:  card.Edited,
		Status:  card.Status.String(),
		Created: formatTime(created),
	}
	if card.Producer.ID != "" {
		file.Producer = &producerFile{ID: card.Producer.ID, Version: card.Producer.Version}
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
// status or the creation time out; an unknown status or type is still an
// error, because guessing it would put the card in the wrong queue or render
// it with the wrong schema.
//
// A remit/1 file — prompt/answer/kind instead of type/fields — reads back as
// the card it meant (see migrateCardFile); migrated reports that this file
// still needs the rewrite.
func decodeCard(id core.ID, data []byte) (card core.Card, created time.Time, migrated bool, err error) {
	var file cardFile
	if err := json.Unmarshal(data, &file); err != nil {
		return core.Card{}, time.Time{}, false, fmt.Errorf("store: decode card %q: %w", id, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return core.Card{}, time.Time{}, false, fmt.Errorf("store: decode card %q: %w", id, err)
	}
	_, hasPrompt := raw["prompt"]
	_, hasAnswer := raw["answer"]
	_, hasKind := raw["kind"]
	_, hasType := raw["type"]
	// A file with none of the legacy keys but no type either is a remit/1
	// card that asked nothing: a plain basic card.
	legacy := hasPrompt || hasAnswer || hasKind || !hasType
	if legacy {
		file, err = migrateCardFile(id, raw)
		if err != nil {
			return core.Card{}, time.Time{}, false, err
		}
	}

	status := core.StatusNew
	if file.Status != "" {
		parsed, ok := core.ParseStatus(file.Status)
		if !ok {
			return core.Card{}, time.Time{}, false, fmt.Errorf("store: card %q: unknown status %q", id, file.Status)
		}
		status = parsed
	}
	created, err = parseTime(file.Created)
	if err != nil {
		return core.Card{}, time.Time{}, false, fmt.Errorf("store: card %q: %w", id, err)
	}
	card = core.Card{
		ID:       id,
		Ref:      core.Reference{Block: file.Block},
		Type:     file.Type,
		Fields:   file.Fields,
		Edited:   file.Edited,
		Status:   status,
		Ordinal:  file.Ordinal,
		Producer: producerOf(file.Producer),
	}
	if file.Frozen != nil {
		at, err := parseTime(file.Frozen.At)
		if err != nil {
			return core.Card{}, time.Time{}, false, fmt.Errorf("store: card %q: %w", id, err)
		}
		card.Ref.Frozen = &core.Snapshot{Text: file.Frozen.Text, At: at}
	}
	return card, created, legacy, nil
}

// producerOf reads a producer back, with nil meaning "the host made this".
func producerOf(file *producerFile) core.Producer {
	if file == nil {
		return core.Producer{}
	}
	return core.Producer{ID: file.ID, Version: file.Version}
}

// legacyKindToType maps a remit/1 kind onto the type that replaced it. It
// exists only here: the kind vocabulary itself is gone from the model.
var legacyKindToType = map[string]core.ID{
	"basic":         core.TypeBasic,
	"cloze":         core.TypeCloze,
	"bidirectional": core.TypeBidirectional,
}

// migrateCardFile reads a remit/1 card — prompt/answer overrides plus a kind —
// into the remit/2 shape: the kind names the type, and any non-empty override
// becomes a Front/Back field value the user edited.
func migrateCardFile(id core.ID, raw map[string]json.RawMessage) (cardFile, error) {
	file := cardFile{}
	if v, ok := raw["block"]; ok {
		if err := json.Unmarshal(v, &file.Block); err != nil {
			return cardFile{}, fmt.Errorf("store: card %q: %w", id, err)
		}
	}
	kind := ""
	if v, ok := raw["kind"]; ok {
		if err := json.Unmarshal(v, &kind); err != nil {
			return cardFile{}, fmt.Errorf("store: card %q: %w", id, err)
		}
	}
	typ := core.TypeBasic
	if kind != "" {
		mapped, ok := legacyKindToType[kind]
		if !ok {
			return cardFile{}, fmt.Errorf("store: card %q: unknown kind %q", id, kind)
		}
		typ = mapped
	}
	file.Type = typ
	var prompt, answer string
	if v, ok := raw["prompt"]; ok {
		if err := json.Unmarshal(v, &prompt); err != nil {
			return cardFile{}, fmt.Errorf("store: card %q: %w", id, err)
		}
	}
	if v, ok := raw["answer"]; ok {
		if err := json.Unmarshal(v, &answer); err != nil {
			return cardFile{}, fmt.Errorf("store: card %q: %w", id, err)
		}
	}
	if prompt != "" || answer != "" {
		file.Fields = map[string]string{"Front": prompt, "Back": answer}
		file.Edited = true
	}
	if v, ok := raw["ordinal"]; ok {
		if err := json.Unmarshal(v, &file.Ordinal); err != nil {
			return cardFile{}, fmt.Errorf("store: card %q: %w", id, err)
		}
	}
	if v, ok := raw["status"]; ok {
		if err := json.Unmarshal(v, &file.Status); err != nil {
			return cardFile{}, fmt.Errorf("store: card %q: %w", id, err)
		}
	}
	if v, ok := raw["created"]; ok {
		if err := json.Unmarshal(v, &file.Created); err != nil {
			return cardFile{}, fmt.Errorf("store: card %q: %w", id, err)
		}
	}
	if v, ok := raw["frozen"]; ok {
		var frozen frozenFile
		if err := json.Unmarshal(v, &frozen); err != nil {
			return cardFile{}, fmt.Errorf("store: card %q: %w", id, err)
		}
		file.Frozen = &frozen
	}
	return file, nil
}

// formatTime writes an instant so that reading it back gives the same one.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
