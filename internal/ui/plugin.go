package ui

import (
	"log/slog"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// cardPlugin is the extension that answers what a paragraph asks and how a card
// reads. It is an ordinary extension — the one the app ships, loaded through the
// same loader and capability rules as any other (ADR 0006/0007).
//
// A library without one still works: no paragraph makes a new card, and a card
// reads its paragraph's own words.
type cardPlugin struct {
	runner *ext.JSRunner
}

// openCards starts the card plugin: the one a test hands in, none at all when it
// asked for none, and otherwise the one the app ships.
func openCards(chosen *ext.JSRunner, without bool) *cardPlugin {
	if without {
		return nil
	}
	if chosen != nil {
		return &cardPlugin{runner: chosen}
	}
	runner, err := builtin.Cards(ext.JSLimits{})
	if err != nil {
		// A plugin that cannot start is not a reason to refuse the library: the
		// cards that exist keep reading their paragraphs.
		slog.Error("remit: the card plugin did not start", "err", err)
		return nil
	}
	return &cardPlugin{runner: runner}
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
		return &schedulerPlugin{runner: chosen}
	}
	runner, err := builtin.Scheduler(ext.JSLimits{})
	if err != nil {
		slog.Error("remit: the scheduler plugin did not start", "err", err)
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

// derived is one card a plugin answered, as the extension speaks it.
type derived struct {
	Kind    string `json:"kind"`
	Ordinal int    `json:"ordinal"`
	Front   string `json:"front"`
	Back    string `json:"back"`
}

// deriveCards asks the plugin what a paragraph makes. ok is false when there is
// no plugin or it failed — a paragraph then makes no new card, and nothing the
// user sees turns red over it.
func (a *App) deriveCards(text string) ([]core.Derived, bool) {
	if a.cards == nil {
		return nil, false
	}
	var answered []derived
	params := map[string]any{"text": text}
	if err := a.cards.runner.Call("derive", params, &answered); err != nil {
		slog.Error("remit: the card plugin could not derive", "err", err)
		return nil, false
	}
	out := make([]core.Derived, 0, len(answered))
	for _, card := range answered {
		kind, ok := core.ParseKind(card.Kind)
		if !ok {
			continue
		}
		out = append(out, core.Derived{Kind: kind, Ordinal: card.Ordinal,
			Prompt: card.Front, Answer: card.Back})
	}
	return out, true
}

// rendered is one card's answer from the plugin: what it was asked, what it
// answered, and whether it answered at all.
type rendered struct {
	askedFront, askedBack string
	front, back           string
	ok                    bool
}

// renderCard asks the plugin how a card reads: markdown the host renders. ok is
// false when there is no plugin or it failed, and the card then reads its
// paragraph's own words rather than nothing.
//
// The answer is kept per card and per sides: the question is asked once, not once
// a frame — a plugin with a whole second to spend would otherwise hold the window
// every pass.
func (a *App) renderCard(card core.ID, prompt, answer string) (string, string, bool) {
	if a.cards == nil {
		return prompt, answer, false
	}
	if got, ok := a.rendered[card]; ok && got.askedFront == prompt && got.askedBack == answer {
		return got.front, got.back, got.ok
	}
	var answered struct {
		Front string `json:"front"`
		Back  string `json:"back"`
	}
	params := map[string]any{"front": prompt, "back": answer}
	// The answer is remembered, and so is a failure: a card's sides are asked
	// about once, not once a frame — a plugin with a whole second to spend would
	// otherwise hold the window every pass, and one that threw would be retried
	// for ever.
	got := rendered{askedFront: prompt, askedBack: answer, front: prompt, back: answer}
	if err := a.cards.runner.Call("render", params, &answered); err != nil {
		slog.Error("remit: the card plugin could not render", "err", err)
		a.rendered[card] = got
		return prompt, answer, false
	}
	got.front, got.back, got.ok = answered.Front, answered.Back, true
	a.rendered[card] = got
	return got.front, got.back, true
}

// facesAnswer is one card's shaping from the plugin, with what it was asked.
type facesAnswer struct {
	askedText      string
	askedKind      core.Kind
	askedOrdinal   int
	prompt, answer string
}

// faces is what a card asks: the plugin's shaping of its paragraph, with the
// card's own overrides on top. A plugin that is absent or failed leaves the
// paragraph's own words — unsplit, because splitting is the plugin's rule and
// not the app's.
//
// The answer is kept per card and per question: a card is shaped once, not once a
// frame.
func (a *App) faces(card core.Card) (prompt, answer string, state core.LinkState) {
	text, state := card.Ref.Text(a.data.blocks)
	prompt, answer = text, ""
	if a.cards != nil {
		if got, ok := a.facesCache[card.ID]; ok && got.askedText == text &&
			got.askedKind == card.Kind && got.askedOrdinal == card.Ordinal {
			prompt, answer = got.prompt, got.answer
		} else {
			var answered struct {
				Front string `json:"front"`
				Back  string `json:"back"`
			}
			params := map[string]any{"text": text, "kind": card.Kind.String(), "ordinal": card.Ordinal}
			got := facesAnswer{askedText: text, askedKind: card.Kind, askedOrdinal: card.Ordinal,
				prompt: text, answer: ""}
			if err := a.cards.runner.Call("faces", params, &answered); err != nil {
				slog.Error("remit: the card plugin could not shape a card", "err", err)
			} else {
				got.prompt, got.answer = answered.Front, answered.Back
			}
			a.facesCache[card.ID] = got
			prompt, answer = got.prompt, got.answer
		}
	}
	if card.Prompt != "" {
		prompt = card.Prompt
	}
	if card.Answer != "" {
		answer = card.Answer
	}
	return prompt, answer, state
}
