package recall

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// maxIntervalChoices are the maximum intervals the failure group offers, in
// days. The preset stores a number, so a library holding one of these keeps it
// chosen; one holding another value keeps that value as its own option.
var maxIntervalChoices = []int{30, 90, 180, 365}

// The broken-link policies, in the order the segmented control shows them.
const (
	brokenKeep = iota
	brokenSnapshot
)

// PresetsState is the presets page: the library's rhythm as the form holds it,
// and what the last write answered. The draft is the app's snapshot copied when
// the page opens, so nothing typed here is written until a control changes.
type PresetsState struct {
	Presets store.Presets
	// MaxInterval is the failure group's select, by the option's label; empty
	// while the page has not built it yet.
	MaxInterval string
	// Broken is the segmented control's index: keep the link broken, or keep a
	// snapshot of the text.
	Broken int
	// Err is why the last write was refused, "" when it was taken.
	Err string
	// confirm is the reset dialog, open while the user has not answered it.
	confirm bool
}

// Sync copies the library's presets into the form: when the page opens and
// after every write, so the page shows what the file holds.
func (s *PresetsState) Sync(p store.Presets) {
	s.Presets = p
	s.MaxInterval = ""
	if p.Broken.Policy == store.BrokenSnapshot {
		s.Broken = brokenSnapshot
	} else {
		s.Broken = brokenKeep
	}
}

// PresetsDeps is what the page asks of the app: one write for the whole form,
// and the reset. Both run on the UI thread and report through PresetsState.
type PresetsDeps struct {
	// SetPresets writes the rhythm into the library's remit.json.
	SetPresets func(*ui.Context, store.Presets)
	// ResetPresets writes the defaults back.
	ResetPresets func(*ui.Context)
}

// PresetsView draws the library's presets: the four groups, the note that a
// change is written at once, and the reset behind a question.
func PresetsView(c *ui.Context, s *PresetsState, d PresetsDeps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	write := false

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.SettingsWidth).Gap(tk.SpaceL).Children(func() {
				pageHead(c, tk, msgs.RecallPresetsTitle(), msgs.RecallPresetsBody(), -1)
				ui.Form(c, func() {
					stepsFieldset(c, tk, msgs)
					write = limitsFieldset(c, s, tk, msgs) || write
					write = failureFieldset(c, s, tk, msgs) || write
					write = brokenFieldset(c, s, tk, msgs) || write
				})
				ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
					ui.Button(c, msgs.RecallPresetsReset()).MinHeight(tk.Control).
						OnClick(func() { s.confirm = true })
					ui.Text(c, msgs.RecallPresetsWrites()).FontSize(tk.TypeCaption).
						TextColor(tk.TextFaint)
				})
				if s.Err != "" {
					kit.Notice(c, "circle-alert", s.Err, tk.Danger, nil)
				}
			})
		})
	})

	// A control that moved writes the whole form once: one file, one rhythm.
	if write {
		d.SetPresets(c, s.Presets)
	}
	resetDialog(c, s, d, msgs)
}

// stepsFieldset is the learning steps, read-only: the schedule has no per-card
// step counter, so this build asks an again card the next day and the page says
// so rather than offering a control that would write nothing.
func stepsFieldset(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	ui.Fieldset(c, msgs.RecallPresetsSteps(), func() {
		ui.Field(c, msgs.RecallPresetsStepsFixed(), func() {
			ui.Text(c, msgs.RecallPresetsStepsAgain()).FontSize(tk.TypeUI)
		}).Description(msgs.RecallPresetsStepsReason())
	})
}

// limitsFieldset is what a day holds: new cards, cards in all, and minutes.
func limitsFieldset(c *ui.Context, s *PresetsState, tk design.Tokens, msgs locales.Messages) bool {
	write := false
	ui.Fieldset(c, msgs.RecallPresetsLimits(), func() {
		ui.Field(c, msgs.RecallPresetsNewCards(), func() {
			value := float64(s.Presets.Review.NewPerDay)
			ui.StepSlider(c, &value, 0, 50, 1).Label(msgs.RecallPresetsNewCards())
			if int(value) != s.Presets.Review.NewPerDay {
				s.Presets.Review.NewPerDay = int(value)
				write = true
			}
		}).Description(msgs.RecallPresetsNewCardsBody())
		ui.Field(c, msgs.RecallPresetsDayCards(), func() {
			value := float64(s.Presets.Budget.Cards)
			ui.StepSlider(c, &value, 0, 200, 10).Label(msgs.RecallPresetsDayCards())
			if int(value) != s.Presets.Budget.Cards {
				s.Presets.Budget.Cards = int(value)
				write = true
			}
		})
		ui.Field(c, msgs.RecallPresetsMinutes(), func() {
			value := float64(s.Presets.Budget.Minutes)
			ui.StepSlider(c, &value, 0, 120, 5).Label(msgs.RecallPresetsMinutes())
			if int(value) != s.Presets.Budget.Minutes {
				s.Presets.Budget.Minutes = int(value)
				write = true
			}
		}).Description(msgs.RecallPresetsLimitsBody())
	})
	return write
}

// failureFieldset is what makes a card hard and how far its interval may grow.
func failureFieldset(c *ui.Context, s *PresetsState, tk design.Tokens, msgs locales.Messages) bool {
	write := false
	ui.Fieldset(c, msgs.RecallPresetsFailure(), func() {
		ui.Field(c, msgs.RecallPresetsMaxInterval(), func() {
			labels, days := maxIntervalOptions(msgs, s.Presets.Review.MaxIntervalDays)
			current := intervalLabelFor(msgs, s.Presets.Review.MaxIntervalDays)
			if s.MaxInterval == "" {
				s.MaxInterval = current
			}
			ui.Select(c, &s.MaxInterval, labels).Label(msgs.RecallPresetsMaxInterval()).
				MinHeight(tk.Control)
			for i, label := range labels {
				if s.MaxInterval != current && label == s.MaxInterval && i < len(days) &&
					days[i] != s.Presets.Review.MaxIntervalDays {
					s.Presets.Review.MaxIntervalDays = days[i]
					write = true
				}
			}
		}).Description(msgs.RecallPresetsMaxIntervalBody())
		ui.Field(c, msgs.RecallPresetsHard(), func() {
			value := float64(s.Presets.Hard.Lapses)
			ui.StepSlider(c, &value, 1, 10, 1).Label(msgs.RecallPresetsHard())
			if int(value) != s.Presets.Hard.Lapses {
				s.Presets.Hard.Lapses = int(value)
				write = true
			}
		}).Description(msgs.RecallPresetsHardBody())
	})
	return write
}

// brokenFieldset is what a card whose block is gone becomes.
func brokenFieldset(c *ui.Context, s *PresetsState, tk design.Tokens, msgs locales.Messages) bool {
	write := false
	ui.Fieldset(c, msgs.RecallPresetsBroken(), func() {
		ui.Field(c, msgs.RecallPresetsBrokenLabel(), func() {
			choice := s.Broken
			ui.Segmented(c, &choice, msgs.RecallPresetsBrokenKeep(), msgs.RecallPresetsBrokenSnapshot()).
				Label(msgs.RecallPresetsBroken())
			if choice != s.Broken {
				s.Broken = choice
				policy := store.BrokenKeep
				if choice == brokenSnapshot {
					policy = store.BrokenSnapshot
				}
				if policy != s.Presets.Broken.Policy {
					s.Presets.Broken.Policy = policy
					write = true
				}
			}
		}).Description(msgs.RecallPresetsBrokenBody())
	})
	return write
}

// resetDialog asks before the rhythm goes back to the defaults, since the
// library has no undo for a preset.
func resetDialog(c *ui.Context, s *PresetsState, d PresetsDeps, msgs locales.Messages) {
	if !s.confirm {
		return
	}
	switch ui.AlertDialog(c, &s.confirm, msgs.RecallPresetsResetTitle(),
		msgs.RecallPresetsResetBody(), msgs.Cancel(), msgs.RecallPresetsResetConfirm()) {
	case 0, ui.AlertDismissed:
	case 1:
		d.ResetPresets(c)
	}
}

// intervalLabelFor is a maximum interval in days as the page names it.
func intervalLabelFor(msgs locales.Messages, days int) string {
	return msgs.RecallPresetsDays(days)
}

// maxIntervalOptions is the select's labels and the days they name: the four
// choices, plus the library's own value when it holds one that is not among
// them, so a hand-edited file is never silently replaced.
func maxIntervalOptions(msgs locales.Messages, current int) (labels []string, days []int) {
	days = append(days, maxIntervalChoices...)
	known := false
	for _, choice := range maxIntervalChoices {
		if choice == current {
			known = true
		}
	}
	if !known && current > 0 {
		days = append([]int{current}, days...)
	}
	for _, day := range days {
		labels = append(labels, intervalLabelFor(msgs, day))
	}
	return labels, days
}
