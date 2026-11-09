package settings

import (
	"github.com/egoist/mygo/ui"

	appsettings "github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// appearancePage is the look, the paper's measure and the text size. The look
// and the measure are the app's preferences, written where they live; the text
// size is the desktop's and stays read-only, with the reason said on the page
// and a way to the desktop's own setting.
func appearancePage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	var look string
	var width float64
	measure := false

	ui.Fieldset(c, msgs.SettingsLook(), func() {
		choice := lookIndex(s.Appearance)
		seg := ui.Segmented(c, &choice,
			msgs.SettingsLookSystem(), msgs.SettingsLookPaper(), msgs.SettingsLookInk())
		seg.Label(msgs.SettingsLook())
		if seg.Changed() {
			look = lookName(choice)
		}
	}).Description(msgs.SettingsLookBody())

	// The slider comes first in the field, so a click on its label focuses it;
	// the measure reads under it.
	ui.Field(c, msgs.SettingsPaperWidth(), func() {
		width = float64(s.setPaperWidth())
		slider := ui.StepSlider(c, &width,
			appsettings.PaperWidthMin, appsettings.PaperWidthMax, appsettings.PaperWidthStep).
			Label(msgs.SettingsPaperWidth())
		ui.Text(c, msgs.SettingsPaperWidthValue(int(width))).
			FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		measure = slider.Changed()
	}).Description(msgs.SettingsPaperWidthBody())

	ui.Field(c, msgs.SettingsTextSize(), func() {
		if d.OpenDisplaySettings != nil {
			ui.Button(c, msgs.SettingsOpenDisplay()).MinHeight(tk.Control).AlignSelf(ui.Start).
				OnClick(func() { d.OpenDisplaySettings(c) })
		}
	}).Description(msgs.SettingsTextSizeBody())

	// The writes are the app's and happen once the page was built, so the value
	// the user moved to is the value asked for.
	if look != "" {
		d.setAppearance(c, look)
	}
	if measure {
		d.setPaperWidth(c, int(width))
	}
}

// lookIndex is the appearance as the segmented control's index: its own order,
// which is the order the spec names the three in.
func lookIndex(appearance string) int {
	switch appearance {
	case appsettings.AppearancePaper:
		return 1
	case appsettings.AppearanceInk:
		return 2
	}
	return 0
}

// lookName is that index back as the preference stores it.
func lookName(index int) string {
	switch index {
	case 1:
		return appsettings.AppearancePaper
	case 2:
		return appsettings.AppearanceInk
	}
	return appsettings.AppearanceSystem
}

// setPaperWidth is the width the slider starts from: inside the range the
// control offers, whatever the file holds.
func (s *State) setPaperWidth() int {
	if s.PaperWidth <= 0 {
		return appsettings.DefaultPaperWidth
	}
	return min(max(s.PaperWidth, appsettings.PaperWidthMin), appsettings.PaperWidthMax)
}

func (d Deps) setAppearance(c *ui.Context, appearance string) {
	if d.SetAppearance != nil {
		d.SetAppearance(c, appearance)
	}
}

func (d Deps) setPaperWidth(c *ui.Context, width int) {
	if d.SetPaperWidth != nil {
		d.SetPaperWidth(c, width)
	}
}
