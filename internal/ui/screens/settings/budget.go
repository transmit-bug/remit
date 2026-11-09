package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// The budget sliders' range: a day of five cards or ten minutes is a decision
// the queue can honour, and past an hour and a half the day is a list nobody
// finishes.
const (
	budgetCardsMin    = 5
	budgetCardsMax    = 60
	budgetCardsStep   = 5
	budgetMinutesMin  = 10
	budgetMinutesMax  = 90
	budgetMinutesStep = 5
)

// budgetPage is what one day holds: the cards and the minutes the presets set,
// today's queue at that pace — derived by the same function the Today page asks
// — the daily reminder, and the title the tray shows while the app runs.
func budgetPage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	var cards, minutes float64
	cardsSet, minutesSet := false, false
	var reminder, reminderSet bool

	if s.HasLibrary {
		// The page's own heading names the presets, so the fields sit in the
		// form directly and only the groups beside them carry a legend.
		ui.Field(c, msgs.SettingsBudgetCards(), func() {
			cards = float64(clamp(s.BudgetCards, budgetCardsMin, budgetCardsMax))
			slider := ui.StepSlider(c, &cards, budgetCardsMin, budgetCardsMax, budgetCardsStep).
				Label(msgs.SettingsBudgetCards())
			ui.Text(c, msgs.SettingsBudgetCardsValue(int(cards))).
				FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			cardsSet = slider.Changed()
		}).Description(msgs.SettingsBudgetCardsBody())

		ui.Field(c, msgs.SettingsBudgetMinutes(), func() {
			minutes = float64(clamp(s.BudgetMinutes, budgetMinutesMin, budgetMinutesMax))
			slider := ui.StepSlider(c, &minutes, budgetMinutesMin, budgetMinutesMax, budgetMinutesStep).
				Label(msgs.SettingsBudgetMinutes())
			ui.Text(c, msgs.SettingsBudgetMinutesValue(int(minutes))).
				FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			minutesSet = slider.Changed()
		}).Description(msgs.SettingsBudgetMinutesBody())

		ui.Field(c, msgs.SettingsBudgetPreview(), func() {
			previewCard(c, s, msgs, tk)
		})
	} else {
		muted(c, tk, msgs.SettingsBudgetNeedsLibrary())
	}

	ui.Fieldset(c, msgs.SettingsReminders(), func() {
		ui.Field(c, msgs.SettingsReminderLabel(), func() {
			reminder = s.Reminder
			reminderSet = ui.Switch(c, &reminder).Changed()
		}).Description(msgs.SettingsReminderBody())
	})

	ui.Fieldset(c, msgs.SettingsBudgetTray(), func() {
		ui.Text(c, trayTitle(msgs, s)).
			Font("monospace").FontSize(tk.TypeMono).TextColor(tk.Text).SingleLine()
		muted(c, tk, msgs.SettingsBudgetTrayBody())
	})

	if cardsSet || minutesSet {
		d.setBudget(c, int(cards), int(minutes))
	}
	if reminderSet {
		d.setNotifications(c, reminder)
	}
}

// previewCard is today's queue as the presets would build it: the two kinds of
// a sitting against the day's budget, and the number of lines the queue holds.
// The defaults are the app's own numbers for a day with nothing due, and the
// card says so rather than drawing an empty meter.
func previewCard(c *ui.Context, s *State, msgs locales.Messages, tk design.Tokens) {
	kit.Card(c, func() {
		ui.Column(c).Gap(tk.SpaceS).Children(func() {
			if s.Budget.Lines == 0 {
				muted(c, tk, msgs.SettingsBudgetPreviewEmpty())
				return
			}
			ui.Text(c, msgs.SettingsBudgetPreviewBody(
				s.Budget.Read, s.Budget.Review, s.Budget.Budget, s.Budget.Lines)).
				FontSize(tk.TypeUI).TextColor(tk.Text)
			kit.Progress(c, kit.ProgressOptions{
				Value:  float64(s.Budget.Total) / float64(max(s.Budget.Budget, 1)),
				Height: tk.SpaceS,
				Label:  msgs.SettingsBudgetPreview(),
			})
		})
	}).Padding(tk.SpaceM)
}

// trayTitle is the title the tray would show for the queues as the page knows
// them: the same catalogue messages the tray itself sets, so the preview is the
// product's own wording rather than a second copy of it.
func trayTitle(msgs locales.Messages, s *State) string {
	if s.TrayReading > 0 {
		return msgs.TrayTitleReading(s.TrayCards, s.TrayReading)
	}
	return msgs.TrayTitle(s.TrayCards)
}

func (d Deps) setBudget(c *ui.Context, cards, minutes int) {
	if d.SetBudget != nil {
		d.SetBudget(c, cards, minutes)
	}
}

func (d Deps) setNotifications(c *ui.Context, on bool) {
	if d.SetNotifications != nil {
		d.SetNotifications(c, on)
	}
}
