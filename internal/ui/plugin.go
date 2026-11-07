package ui

import (
	"log/slog"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// cardPlugin is the extension that answers what a paragraph asks. It is an
// ordinary extension — the one the app ships, loaded through the same loader
// and capability rules as any other (ADR 0006/0007).
//
// A library without one still works: no paragraph makes a new card, and a card
// reads its paragraph's own words.
type cardPlugin struct {
	runner *ext.JSRunner
}

// openCards starts the card plugin: the one a test hands in, none at all when it
// asked for none, and otherwise the one the app ships. The app's own plugins are
// opened before a library is and declare no capabilities, so they are given no
// host; a downloaded extension is started by cmd, beside the open library.
func openCards(chosen *ext.JSRunner, without bool) *cardPlugin {
	if without {
		return nil
	}
	if chosen != nil {
		warm(chosen, "card")
		return &cardPlugin{runner: chosen}
	}
	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		// A plugin that cannot start is not a reason to refuse the library: the
		// cards that exist keep reading their paragraphs.
		slog.Error("remit: the card plugin did not start", "err", err)
		return nil
	}
	warm(runner, "card")
	return &cardPlugin{runner: runner}
}

// warm fills a plugin's runtime pool at activation, so the first review pays
// no load cost and a syntax error surfaces now rather than mid-review
// (ADR 0006). A plugin that cannot warm still works: its calls load lazily.
func warm(runner *ext.JSRunner, what string) {
	if err := runner.Warm(); err != nil {
		slog.Error("remit: the "+what+" plugin did not warm", "err", err)
	}
}

// schedulerPlugin is the extension that decides scheduling policy and says what
// a finished session is worth: the arithmetic stays the app's (ADR 0007).
type schedulerPlugin struct {
	runner *ext.JSRunner
}

// openScheduler starts the scheduling plugin, or returns nil when the app is
// asked to run without one.
func openScheduler(chosen *ext.JSRunner, without bool) *schedulerPlugin {
	if without {
		return nil
	}
	if chosen != nil {
		warm(chosen, "scheduler")
		return &schedulerPlugin{runner: chosen}
	}
	runner, err := builtin.Scheduler(nil, ext.JSLimits{})
	if err != nil {
		slog.Error("remit: the scheduler plugin did not start", "err", err)
		return nil
	}
	warm(runner, "scheduler")
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

// faces is what browse and re-link read of a card's answer: its two sides and
// how its source stands.
func (a *App) faces(card core.Card) (prompt, answer string, state core.LinkState) {
	got := a.face(card)
	return got.Front, got.Back, got.State
}
