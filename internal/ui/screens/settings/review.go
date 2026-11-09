package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// The ends of the two sliders: what a retention and a longest interval may be.
// They are the range the schedule can mean, not a taste: below 80% a card comes
// back long before it is forgotten, and past a year the schedule stops being the
// thing that decides.
const (
	retentionMin    = 80
	retentionMax    = 97
	maxIntervalMin  = 30
	maxIntervalMax  = 365
	maxIntervalStep = 30
)

// reviewPage is the rhythm this library's reviews keep: the target retention,
// the longest a card may wait, whether a review says what each answer would
// schedule, and whether an excerpt returns the reader to the note. They live in
// the library's config because a pace follows the material, not the machine.
//
// The page's own heading names the group, so the fields sit in the form
// directly rather than under a legend that would repeat the title.
func reviewPage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	if !s.HasLibrary {
		muted(c, tk, msgs.SettingsReviewNeedsLibrary())
		return
	}
	var retention, interval float64
	var preview, returned bool
	retentionSet, intervalSet, previewSet, returnSet := false, false, false, false

	ui.Field(c, msgs.SettingsReviewRetention(), func() {
		retention = float64(clamp(s.RetentionPercent, retentionMin, retentionMax))
		slider := ui.StepSlider(c, &retention, retentionMin, retentionMax, 1).
			Label(msgs.SettingsReviewRetention())
		ui.Text(c, msgs.SettingsReviewRetentionValue(int(retention))).
			FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		retentionSet = slider.Changed()
	}).Description(msgs.SettingsReviewRetentionBody())

	ui.Field(c, msgs.SettingsReviewMaxInterval(), func() {
		interval = float64(clamp(s.MaxIntervalDays, maxIntervalMin, maxIntervalMax))
		slider := ui.StepSlider(c, &interval, maxIntervalMin, maxIntervalMax, maxIntervalStep).
			Label(msgs.SettingsReviewMaxInterval())
		ui.Text(c, msgs.SettingsReviewDays(int(interval))).
			FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		intervalSet = slider.Changed()
	}).Description(msgs.SettingsReviewMaxIntervalBody())

	ui.Field(c, msgs.SettingsReviewPreview(), func() {
		preview = s.IntervalPreview
		previewSet = ui.Switch(c, &preview).Changed()
	}).Description(msgs.SettingsReviewPreviewBody())

	ui.Field(c, msgs.SettingsReviewReturn(), func() {
		returned = s.ReturnAfterExcerpt
		returnSet = ui.Switch(c, &returned).Changed()
	}).Description(msgs.SettingsReviewReturnBody())

	if retentionSet {
		d.setRetention(c, int(retention))
	}
	if intervalSet {
		d.setMaxInterval(c, int(interval))
	}
	if previewSet {
		d.setIntervalPreview(c, preview)
	}
	if returnSet {
		d.setReturnAfterExcerpt(c, returned)
	}
}

// clamp keeps a stored value inside the range its control offers, so a file
// written by hand still draws a control the user can move.
func clamp(value, lo, hi int) int { return min(max(value, lo), hi) }

func (d Deps) setRetention(c *ui.Context, percent int) {
	if d.SetRetention != nil {
		d.SetRetention(c, percent)
	}
}

func (d Deps) setMaxInterval(c *ui.Context, days int) {
	if d.SetMaxInterval != nil {
		d.SetMaxInterval(c, days)
	}
}

func (d Deps) setIntervalPreview(c *ui.Context, on bool) {
	if d.SetIntervalPreview != nil {
		d.SetIntervalPreview(c, on)
	}
}

func (d Deps) setReturnAfterExcerpt(c *ui.Context, on bool) {
	if d.SetReturnAfterExcerpt != nil {
		d.SetReturnAfterExcerpt(c, on)
	}
}
