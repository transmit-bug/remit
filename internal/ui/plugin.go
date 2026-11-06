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
