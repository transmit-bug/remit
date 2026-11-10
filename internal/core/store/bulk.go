package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/transmit-bug/remit/internal/core"
)

// The bulk writes a list makes over what it has selected — suspend, move the due
// date, freeze, tag, change the type, bury the paragraph's other cards, delete —
// each of them one card file or one note source per id, and each of them
// idempotent: doing it twice leaves the library as doing it once did.
//
// They answer a report rather than an error alone, because a batch is not one
// write: a card whose file cannot be written, or an id that names nothing, must
// not take the other forty-nine with it, and must not pass silently either. The
// caller shows the count it wrote and the ids it did not.

// BulkFailure is one id a bulk action could not write, and why.
type BulkFailure struct {
	ID  core.ID
	Err error
}

// BulkResult is what a bulk action did, id by id: how many it wrote, and the ones
// it could not.
type BulkResult struct {
	// Applied is how many ids the action wrote, whether or not the write changed
	// anything: an operation on something that already had that state is done.
	Applied int
	// Failed holds the ids the action could not write, in the order it met them.
	Failed []BulkFailure
}

// Err summarizes a batch's failures as one error, nil when every id was written.
// It is what a caller logs as one line; the ids and their reasons are in Failed
// for the caller that shows them.
func (r BulkResult) Err() error {
	if len(r.Failed) == 0 {
		return nil
	}
	errs := make([]error, len(r.Failed))
	for i, failure := range r.Failed {
		errs[i] = fmt.Errorf("%s: %w", failure.ID, failure.Err)
	}
	return fmt.Errorf("store: %d of %d writes failed: %w", len(r.Failed), r.Applied+len(r.Failed),
		errors.Join(errs...))
}

// fail records one id the action could not write.
func (r *BulkResult) fail(id core.ID, err error) {
	r.Failed = append(r.Failed, BulkFailure{ID: id, Err: err})
}

// SuspendCards takes the cards out of the queue without touching their history:
// a suspended card keeps every review it has, and unsuspending it puts it back
// where its schedule says. at is the moment the action is taken; a suspension
// writes no date of its own.
func (l *Library) SuspendCards(ids []core.ID, at time.Time) (BulkResult, error) {
	return l.bulkUpdateCards(ids, func(card core.Card) (core.Card, error) {
		card.Status = core.StatusSuspended
		return card, nil
	})
}

// FreezeCards stops the cards from following their blocks, keeping what each
// block says at `at`. It is what a list does when the text under a card is about
// to be rewritten: the card keeps the words it was made from. A card already
// frozen keeps the snapshot it has.
func (l *Library) FreezeCards(ids []core.ID, at time.Time) (BulkResult, error) {
	return l.bulkUpdateCards(ids, func(card core.Card) (core.Card, error) {
		if card.Ref.Frozen != nil {
			return card, nil
		}
		ref, ok := l.index.block(card.Ref.Block)
		if !ok {
			return card, fmt.Errorf("store: block %q is gone, so there is nothing to freeze", card.Ref.Block)
		}
		card.Ref = card.Ref.Freeze(core.BlockSet{ref.block.ID: ref.block}, l.instant(at))
		return card, nil
	})
}

// SetDue moves the cards to a due date of the user's choosing, written into each
// card file as the one schedule fact a card carries. It stands until a review is
// written after it: answering a card puts it back on the schedule its answers
// derive, which is what stops a hand-set date from keeping a reviewed card in the
// queue for ever.
//
// A card no review has touched is listed at the date it was moved to; its place
// in the queue is still its creation, which is the store's rule for a new card.
func (l *Library) SetDue(ids []core.ID, at time.Time) (BulkResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return BulkResult{}, ErrClosed
	}
	if at.IsZero() {
		at = l.now()
	}
	set := l.now().UTC()
	var out BulkResult
	queueMoved := false
	for _, id := range ids {
		meta, ok := l.cards[id]
		if !ok {
			out.fail(id, fmt.Errorf("store: no card %q", id))
			continue
		}
		card := cloneCard(meta.card)
		card.Due = &core.DueOverride{At: at.UTC(), Set: set}
		if err := l.saveCard(card); err != nil {
			out.fail(id, err)
			continue
		}
		out.Applied++
		// A reviewed card's queue place is its schedule's due moment, so the new
		// date has to reach it now and not at the next open.
		if current, ok := l.cards[id]; ok && current.state.Reviews > 0 && card.Due.Stands(current.state.LastReview) {
			current.state.Due = card.Due.At
			queueMoved = true
		}
	}
	if queueMoved {
		l.markQueueStale()
	}
	return out, nil
}

// ChangeCardTypes renders the cards through another type. A card's field values
// are its own hand edits: those the new type names are kept, the rest belong to
// a schema the card no longer has and are dropped. A card whose type or fields
// cannot be read is reported, and the rest are still written.
//
// A type this library does not hold is a failure of every id, not of nothing:
// what the caller asked for cannot be rendered at all. The write carries no
// moment — a card file records what a card shows, so nothing here reads a clock.
func (l *Library) ChangeCardTypes(ids []core.ID, typeID core.ID) (BulkResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return BulkResult{}, ErrClosed
	}
	typ, ok := l.typeLocked(typeID)
	if !ok {
		var out BulkResult
		for _, id := range ids {
			out.fail(id, fmt.Errorf("%w: %q", ErrNoType, typeID))
		}
		return out, nil
	}
	keep := make(map[string]bool, len(typ.Fields))
	for _, field := range typ.Fields {
		keep[field] = true
	}
	return l.bulkUpdateCardsLocked(ids, func(card core.Card) (core.Card, error) {
		card.Type = typ.ID
		card.Fields, card.Edited = keptFields(card.Fields, keep)
		return card, nil
	})
}

// keptFields keeps the values a type still names, and reports whether any are
// left: a card with no value of its own resolves every field from its block
// again, which is what an unedited card is.
func keptFields(fields map[string]string, keep map[string]bool) (map[string]string, bool) {
	var out map[string]string
	for field, value := range fields {
		if !keep[field] {
			continue
		}
		if out == nil {
			out = make(map[string]string, len(fields))
		}
		out[field] = value
	}
	return out, len(out) > 0
}

// BuryCards puts the other cards of the chosen cards' paragraphs off until
// `until`: the same paragraph asks the same thing twice, so the second card is
// not worth meeting before the first is learned. The chosen cards keep their
// date — burying is what the paragraph's other cards get, not what the chosen
// ones do.
//
// It is a due override, the same fact SetDue writes, so no review is added and
// no history is rewritten. A sibling already due after `until` is left alone: it
// is not in the way, and pulling it forward would be a schedule regression the
// user did not ask for. The report names what was written and what failed.
func (l *Library) BuryCards(ids []core.ID, until time.Time) (BulkResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return BulkResult{}, ErrClosed
	}
	if until.IsZero() {
		until = l.now()
	}
	chosen := make(map[core.ID]bool, len(ids))
	blocks := make(map[core.ID]bool, len(ids))
	var out BulkResult
	for _, id := range ids {
		chosen[id] = true
		meta, ok := l.cards[id]
		if !ok {
			out.fail(id, fmt.Errorf("store: no card %q", id))
			continue
		}
		blocks[meta.card.Ref.Block] = true
	}
	if len(blocks) == 0 {
		return out, nil
	}
	// Siblings are read from the catalog in id order, so one call writes the
	// same cards in the same order every time.
	siblings := make([]core.ID, 0, len(l.cards))
	for id := range l.cards {
		siblings = append(siblings, id)
	}
	sort.Slice(siblings, func(i, j int) bool { return siblings[i] < siblings[j] })
	set := l.now().UTC()
	queueMoved := false
	for _, id := range siblings {
		meta := l.cards[id]
		if chosen[id] || !blocks[meta.card.Ref.Block] {
			continue
		}
		if !time.Unix(0, dueNano(meta)).Before(until) {
			continue
		}
		card := cloneCard(meta.card)
		card.Due = &core.DueOverride{At: until.UTC(), Set: set}
		if err := l.saveCard(card); err != nil {
			out.fail(id, err)
			continue
		}
		// A reviewed card's queue place is its schedule's due moment, so the new
		// date has to reach it now and not at the next open — SetDue's rule.
		if current, ok := l.cards[id]; ok && current.state.Reviews > 0 && card.Due.Stands(current.state.LastReview) {
			current.state.Due = card.Due.At
			queueMoved = true
		}
		out.Applied++
	}
	if queueMoved {
		l.markQueueStale()
	}
	return out, nil
}

// DeleteCards removes the card files the user chose. The review log is
// append-only, so the history stays where it was written: a delete stops a card
// from being scheduled, it does not take back what the user learned or how often
// they missed it.
//
// A card that is not there is a failure of that id, not of the batch.
func (l *Library) DeleteCards(ids []core.ID) (BulkResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return BulkResult{}, ErrClosed
	}
	var out BulkResult
	for _, id := range ids {
		if err := l.deleteCardLocked(id); err != nil {
			out.fail(id, err)
			continue
		}
		out.Applied++
	}
	return out, nil
}

// AddTagToNotes tags notes: the tag becomes one block at the end of each note
// whose text does not carry it yet, which is what typing it as its own line
// writes. A note already carrying the tag is left alone — its file is not
// rewritten for nothing — and counts as done.
//
// The tag goes in its own block rather than onto the end of the last one: a
// block's text is what its cards render from, and tagging a note may not change
// what its cards say.
func (l *Library) AddTagToNotes(ids []core.ID, tag string) (BulkResult, error) {
	tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
	if !isTag(tag) {
		return BulkResult{}, fmt.Errorf("store: %q is not a tag", tag)
	}
	var out BulkResult
	for _, id := range ids {
		note, ok := l.Note(id)
		if !ok {
			out.fail(id, fmt.Errorf("store: no note %q", id))
			continue
		}
		if carriesTag(note, tag) {
			out.Applied++
			continue
		}
		note.Blocks = append(note.Blocks, core.Block{Text: "#" + tag})
		if _, err := l.SaveNote(note); err != nil {
			out.fail(id, err)
			continue
		}
		out.Applied++
	}
	return out, nil
}

// bulkUpdateCards writes one card file per id, so one card's failure does not
// stop the batch: the rest are still written, and the report names the one that
// was not.
func (l *Library) bulkUpdateCards(ids []core.ID, edit func(core.Card) (core.Card, error)) (BulkResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.bulkUpdateCardsLocked(ids, edit)
}

// bulkUpdateCardsLocked is bulkUpdateCards' write, assuming the lock is held: it
// is what an action that also reads the catalog runs under its own lock.
func (l *Library) bulkUpdateCardsLocked(ids []core.ID, edit func(core.Card) (core.Card, error)) (BulkResult, error) {
	if l.index == nil {
		return BulkResult{}, ErrClosed
	}
	var out BulkResult
	for _, id := range ids {
		meta, ok := l.cards[id]
		if !ok {
			out.fail(id, fmt.Errorf("store: no card %q", id))
			continue
		}
		card, err := edit(cloneCard(meta.card))
		if err != nil {
			out.fail(id, err)
			continue
		}
		if err := l.saveCard(card); err != nil {
			out.fail(id, err)
			continue
		}
		out.Applied++
	}
	return out, nil
}

// cloneCard copies a card deeply enough that a bulk write cannot reach the
// catalog's own card through its maps and pointers.
func cloneCard(card core.Card) core.Card {
	if card.Fields != nil {
		fields := make(map[string]string, len(card.Fields))
		for key, value := range card.Fields {
			fields[key] = value
		}
		card.Fields = fields
	}
	if card.Ref.Frozen != nil {
		frozen := *card.Ref.Frozen
		card.Ref.Frozen = &frozen
	}
	if card.Due != nil {
		due := *card.Due
		card.Due = &due
	}
	return card
}

// carriesTag reports whether any of a note's blocks names the tag already.
func carriesTag(note core.Note, tag string) bool {
	for _, block := range note.Blocks {
		for _, got := range core.Tags(block.Text) {
			if got == tag {
				return true
			}
		}
	}
	return false
}

// isTag reports whether the text is one tag core.Tags can find: non-empty, made
// of what a tag may hold, and read back whole — "study" is a tag, "a b" and
// "study!" are not, and storing half of what was meant would be worse than
// saying no.
func isTag(tag string) bool {
	if tag == "" {
		return false
	}
	tags := core.Tags("#" + tag)
	return len(tags) == 1 && tags[0] == tag
}
