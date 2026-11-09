package store

import (
	"errors"
	"fmt"

	"github.com/transmit-bug/remit/internal/core/schedule"
)

// ErrInvalidPresets is returned by SetPresets for a value a preset cannot mean:
// a negative limit, a retention outside (0, 1], or a broken-link policy this
// build does not know. The file is left as it was.
var ErrInvalidPresets = errors.New("store: invalid presets")

// The policies a card whose block is gone follows.
const (
	// BrokenKeep leaves the card lost until it is pointed at another block,
	// which is what a library has always done.
	BrokenKeep = "keep"
	// BrokenSnapshot keeps a copy of the block, so the card still asks.
	BrokenSnapshot = "snapshot"
)

// Presets is a library's own rhythm (ADR 0011): what a day holds, how much new
// material comes in, the lapse count from which a card counts as hard, and what
// a broken link becomes. It lives in remit.json because a pace follows the
// material rather than the machine. Every limit counts what a day may hold;
// zero means no cap. A presets object in the file is read as written, and one
// the file leaves out reads as DefaultPresets.
type Presets struct {
	Budget  BudgetPreset  `json:"budget"`
	Review  ReviewPreset  `json:"review"`
	Reading ReadingPreset `json:"reading"`
	Hard    HardPreset    `json:"hard"`
	Broken  BrokenPreset  `json:"broken"`
}

// BudgetPreset is what one day's queue holds: cards, minutes, and the slots an
// excerpt may take from today.
type BudgetPreset struct {
	Cards        int `json:"cards"`
	Minutes      int `json:"minutes"`
	ExcerptSlots int `json:"excerptSlots"`
}

// ReviewPreset paces new cards and says what the schedule aims at.
type ReviewPreset struct {
	NewPerDay       int `json:"newPerDay"`
	MaxIntervalDays int `json:"maxIntervalDays"`
	// Retention overrides the scheduling plugin's target for this library. Nil
	// follows the plugin, which is what a library does until someone says
	// otherwise.
	Retention *float64 `json:"retention"`
	// Preview says whether a review shows what each answer would schedule.
	Preview bool `json:"preview"`
}

// ReadingPreset paces new reading, and says whether a session returns to the
// note after an excerpt.
type ReadingPreset struct {
	NewPerDay          int  `json:"newPerDay"`
	ReturnAfterExcerpt bool `json:"returnAfterExcerpt"`
}

// HardPreset is the lapse count from which a card counts as hard.
type HardPreset struct {
	Lapses int `json:"lapses"`
}

// BrokenPreset says what a card whose block is gone becomes: kept as a broken
// link, or given a snapshot of the text it lost.
type BrokenPreset struct {
	Policy string `json:"policy"`
}

// DefaultPresets is the rhythm a new library starts with, and the one a
// remit/2 library gains when it is migrated.
func DefaultPresets() Presets {
	return Presets{
		Budget:  BudgetPreset{Cards: 20, Minutes: 30, ExcerptSlots: 2},
		Review:  ReviewPreset{NewPerDay: 10, MaxIntervalDays: 180, Preview: true},
		Reading: ReadingPreset{NewPerDay: 3, ReturnAfterExcerpt: true},
		Hard:    HardPreset{Lapses: 3},
		Broken:  BrokenPreset{Policy: BrokenKeep},
	}
}

// Presets answers the rhythm this library keeps.
func (l *Library) Presets() Presets {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.presets.clone()
}

// SetPresets records the rhythm this library keeps in remit.json. A value a
// preset cannot mean is refused and the file left as it was; zero is no cap. A
// retention override is what the library schedules with from the next review
// on: the log is untouched, and the next open replays it because the schedule's
// parameters moved.
func (l *Library) SetPresets(presets Presets) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	presets = presets.normalized()
	if err := presets.validate(); err != nil {
		return err
	}
	heldPresets, heldParams := l.presets, l.params
	l.presets = presets.clone()
	l.params = presets.scheduleParams(l.baseParams)
	if err := l.writeConfigLocked(); err != nil {
		l.presets, l.params = heldPresets, heldParams
		return err
	}
	return nil
}

// scheduleParams answers the parameters cards are reviewed with: the library's
// own, with the preset's retention in place of the scheduling plugin's when the
// user set one.
func (p Presets) scheduleParams(base schedule.Params) schedule.Params {
	params := base.Normalized()
	if p.Review.Retention != nil {
		params.DesiredRetention = *p.Review.Retention
	}
	return params
}

// normalized is what gets written and scheduled with: an unset broken-link
// policy is the keep behaviour, and a policy this build does not know is left
// to validate.
func (p Presets) normalized() Presets {
	if p.Broken.Policy == "" {
		p.Broken.Policy = BrokenKeep
	}
	return p
}

// validate refuses a value a preset cannot mean, naming the one it saw.
func (p Presets) validate() error {
	limits := []struct {
		name  string
		value int
	}{
		{"budget.cards", p.Budget.Cards},
		{"budget.minutes", p.Budget.Minutes},
		{"budget.excerptSlots", p.Budget.ExcerptSlots},
		{"review.newPerDay", p.Review.NewPerDay},
		{"review.maxIntervalDays", p.Review.MaxIntervalDays},
		{"reading.newPerDay", p.Reading.NewPerDay},
		{"hard.lapses", p.Hard.Lapses},
	}
	for _, limit := range limits {
		if limit.value < 0 {
			return fmt.Errorf("%w: %s is %d", ErrInvalidPresets, limit.name, limit.value)
		}
	}
	if retention := p.Review.Retention; retention != nil && !(*retention > 0 && *retention <= 1) {
		return fmt.Errorf("%w: review.retention is %v", ErrInvalidPresets, *retention)
	}
	switch p.Broken.Policy {
	case BrokenKeep, BrokenSnapshot:
	default:
		return fmt.Errorf("%w: broken.policy is %q", ErrInvalidPresets, p.Broken.Policy)
	}
	return nil
}

// clone copies the retention, so a caller and the library never answer with
// the same number.
func (p Presets) clone() Presets {
	if p.Review.Retention != nil {
		retention := *p.Review.Retention
		p.Review.Retention = &retention
	}
	return p
}

// orDefault answers the presets a new library starts with: the zero value means
// the defaults, so a caller names only the rhythm it has an opinion about.
func (p Presets) orDefault() (Presets, error) {
	if p == (Presets{}) {
		return DefaultPresets(), nil
	}
	p = p.normalized()
	if err := p.validate(); err != nil {
		return Presets{}, err
	}
	return p, nil
}
