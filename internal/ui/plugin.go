package ui

import (
	"log/slog"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/plugin"
)

// cardPlugin is the extension that answers what a paragraph asks, as the window
// holds it: the runner the supervisor resolved for the role. It is an ordinary
// extension — the one the app ships is loaded through the same loader and
// capability rules as any other (ADR 0006/0007) — and the window never starts
// one of its own.
//
// A library without one still works: no paragraph makes a new card, and a card
// reads its paragraph's own words.
type cardPlugin struct {
	runner plugin.Role
}

// openCards is the card role as the window draws with it, or none when the role
// resolved to nothing: the supervisor's answer, never a plugin built here.
func openCards(runner plugin.Role) *cardPlugin {
	if runner == nil {
		return nil
	}
	return &cardPlugin{runner: runner}
}

// engine is the runner behind the role, as the card resolver takes it today: a
// role is js only (internal/plugin enforces it), so it is the engine's own
// runner. #267 hands the resolver the capability it asks for instead.
func (c *cardPlugin) engine() *ext.JSRunner {
	if c == nil {
		return nil
	}
	runner, _ := c.runner.(*ext.JSRunner)
	return runner
}

// schedulerPlugin is the extension that decides scheduling policy and says what
// a finished session is worth: the arithmetic stays the app's (ADR 0007).
type schedulerPlugin struct {
	runner plugin.Role
}

// openScheduler is the scheduling role as the window draws with it, or none when
// the role resolved to nothing.
func openScheduler(runner plugin.Role) *schedulerPlugin {
	if runner == nil {
		return nil
	}
	return &schedulerPlugin{runner: runner}
}

// policy is what the plugin schedules with, and how it names itself.
type policy struct {
	Family    string  `json:"family"`
	Version   string  `json:"version"`
	Retention float64 `json:"retention"`
}

// Policy answers the plugin's policy, and false when it has none to give: the
// app then schedules as it did before plugins, with its own defaults.
func (p *schedulerPlugin) Policy() (policy, bool) {
	if p == nil {
		return policy{}, false
	}
	var answered policy
	if err := p.runner.Call("policy", nil, &answered); err != nil {
		slog.Error("remit: the scheduler plugin answered no policy", "err", err)
		return policy{}, false
	}
	return answered, answered.Family != ""
}

// Identity is what a review record carries: the family and version that
// scheduled it.
func (p policy) Identity() string {
	if p.Family == "" {
		return ""
	}
	if p.Version == "" {
		return p.Family
	}
	return p.Family + "-" + p.Version
}

// summary is what a finished session is worth, as a plugin may add to it.
type summaryAnswer struct {
	Tone  string   `json:"tone"`
	Lines []string `json:"lines"`
}

// SummaryOf asks what the session is worth saying beyond its numbers.
func (p *schedulerPlugin) SummaryOf(reviewed, dueTomorrow int, accuracy float64) (summaryAnswer, bool) {
	if p == nil {
		return summaryAnswer{}, false
	}
	var answered summaryAnswer
	params := map[string]any{"reviewed": reviewed, "dueTomorrow": dueTomorrow, "accuracy": accuracy}
	if err := p.runner.Call("summary", params, &answered); err != nil {
		slog.Error("remit: the scheduler plugin answered no summary", "err", err)
		return summaryAnswer{}, false
	}
	return answered, true
}

// deriveCards asks the resolver what a paragraph makes. ok is false when there
// is no plugin or it failed — a paragraph then makes no new card, and nothing
// the user sees turns red over it. The resolver asks the plugin once per text
// and stamps each card with the plugin's identity.
func (a *App) deriveCards(text string) ([]core.Derived, bool) {
	if a.resolver == nil {
		return nil, false
	}
	return a.resolver.Derive(text)
}

// face is what a card asks: the resolver's answer — the fields the paragraph
// gives (the block for a live reference, the snapshot for a frozen one), the
// card's own edited fields on top, rendered by the type's templates, with what
// the review screen reads beyond the two sides. A plugin that is absent or
// failed leaves the paragraph's own words, unsplit.
func (a *App) face(card core.Card) cards.Answer {
	if a.resolver == nil {
		text, state := card.Ref.Text(a.data.blocks)
		return cards.Answer{Front: text, State: state}
	}
	return a.resolver.Face(card, a.data.blocks)
}

// faces is what the card table and re-link read of a card's answer: its two
// sides and how its source stands.
func (a *App) faces(card core.Card) (prompt, answer string, state core.LinkState) {
	got := a.face(card)
	return got.Front, got.Back, got.State
}
