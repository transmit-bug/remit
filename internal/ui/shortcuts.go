package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// keyGroup is one section of the keyboard help: the scope its keys work in.
type keyGroup struct {
	title func(locales.Messages) string
	keys  []keyRow
}

// keyRow is one line of the help: the chord, and what it does.
type keyRow struct {
	chord  string
	action func(locales.Messages) string
}

// shortcutGroups is the window's keyboard map, in the order the help lists it.
// It describes the keys window.go registers, and only those: a key that does
// nothing is not a shortcut, so a row lands here with the handler that makes it
// work.
func shortcutGroups() []keyGroup {
	esc := design.KeyEsc
	return []keyGroup{
		{
			title: func(msgs locales.Messages) string { return msgs.KeysGroupGlobal() },
			keys: []keyRow{
				{design.Chord(design.KeyCmd, "K"), func(msgs locales.Messages) string { return msgs.KeysPalette() }},
				{design.Chord(design.KeyCmd, "/"), func(msgs locales.Messages) string { return msgs.KeysHelp() }},
				{design.Chord(design.KeyCmd, ","), func(msgs locales.Messages) string { return msgs.KeysSettings() }},
				{design.Chord(design.KeyCmd, "N"), func(msgs locales.Messages) string { return msgs.KeysNewNote() }},
				{design.Chord(design.KeyCmd, "D"), func(msgs locales.Messages) string { return msgs.KeysDailyNote() }},
				{design.Chord(design.KeyCmd, "F"), func(msgs locales.Messages) string { return msgs.KeysSearch() }},
				{design.Chord(design.KeyCmd, "1", design.KeyRange, "6"),
					func(msgs locales.Messages) string { return msgs.KeysAreas() }},
				{esc, func(msgs locales.Messages) string { return msgs.KeysEscape() }},
			},
		},
		{
			title: func(msgs locales.Messages) string { return msgs.KeysGroupReading() },
			keys: []keyRow{
				{design.KeyDown + design.KeySeparator + "J",
					func(msgs locales.Messages) string { return msgs.KeysNextParagraph() }},
				{design.KeyUp + design.KeySeparator + "K",
					func(msgs locales.Messages) string { return msgs.KeysPrevParagraph() }},
				{"1", func(msgs locales.Messages) string { return msgs.KeysTooHard() }},
				{"2", func(msgs locales.Messages) string { return msgs.KeysJustRight() }},
				{"3", func(msgs locales.Messages) string { return msgs.KeysTooShallow() }},
				{"E", func(msgs locales.Messages) string { return msgs.KeysExcerpt() }},
				{"S", func(msgs locales.Messages) string { return msgs.KeysSkip() }},
				{"X", func(msgs locales.Messages) string { return msgs.KeysDropFromQueue() }},
				{"M", func(msgs locales.Messages) string { return msgs.KeysOpenInLibrary() }},
				{esc, func(msgs locales.Messages) string { return msgs.KeysLeaveSession() }},
			},
		},
		{
			title: func(msgs locales.Messages) string { return msgs.KeysGroupReview() },
			keys: []keyRow{
				{design.KeySpace, func(msgs locales.Messages) string { return msgs.KeysReveal() }},
				{"1" + design.KeyRange + "4", func(msgs locales.Messages) string { return msgs.KeysGrade() }},
				{"U", func(msgs locales.Messages) string { return msgs.KeysUndo() }},
				{"S", func(msgs locales.Messages) string { return msgs.KeysSuspend() }},
				{"E", func(msgs locales.Messages) string { return msgs.KeysEditNote() }},
				{esc, func(msgs locales.Messages) string { return msgs.KeysLeaveSession() }},
			},
		},
		{
			title: func(msgs locales.Messages) string { return msgs.KeysGroupDocument() },
			keys: []keyRow{
				{design.Chord(design.KeyCmd, "F"), func(msgs locales.Messages) string { return msgs.KeysFindInNote() }},
				{design.Chord(design.KeyShift, design.KeyCmd, "O"),
					func(msgs locales.Messages) string { return msgs.KeysOutline() }},
				{design.Chord(design.KeyShift, design.KeyCmd, "B"),
					func(msgs locales.Messages) string { return msgs.KeysBacklinks() }},
			},
		},
	}
}

// keysState is the keyboard help: whether the dialog is open. It is read-only,
// so the state is one flag.
type keysState struct {
	open bool
}

// openKeys shows the window's keyboard map.
func (a *App) openKeys(_ *ui.Context) {
	a.anim.keys++
	a.keys.open = true
}

// keysView draws the keyboard help over the window: four groups of chords, each
// a keycap beside what it does. DialogBase keeps the keyboard in it, and Escape
// — the first key it lists — closes it.
func (a *App) keysView(c *ui.Context, tk design.Tokens) {
	if !a.keys.open {
		return
	}
	msgs := m(c)
	ui.DialogBase(c, &a.keys.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(tk.SettingsWidth).
			Padding(tk.SpaceL).Gap(tk.SpaceM))
		panel.Label(msgs.KeysTitle())
		arriving := panel.Keyframes("arrive", design.PanelFrames, ui.KeyframeOptions{Run: a.anim.keys})
		scale := design.PanelScale + (1-design.PanelScale)*arriving
		panel.Opacity(arriving).Scale(scale, scale)
		backdrop.Opacity(arriving)

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, msgs.KeysTitle()).FontSize(tk.TypeH2).Bold().Grow(1)
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { a.keys.open = false })
			})
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(tk.SpaceL).Children(func() {
					for _, group := range shortcutGroups() {
						keyGroupView(c, tk, group, msgs)
					}
				})
			})
		})
	})
}

// keyGroupView is one group of the map: its title, then two columns of chord
// and action. It is read, never operated: nothing here is a control.
func keyGroupView(c *ui.Context, tk design.Tokens, group keyGroup, msgs locales.Messages) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, group.title(msgs)).FontSize(tk.TypeUI).Bold()
		// Two columns as a definition list: the chord in a column of its own,
		// as wide as the window's widest chord needs, and what it does beside
		// it.
		ui.Grid(c).ColumnTracks(ui.Fixed(tk.TypeUI*8), ui.Fr(1)).Gap(tk.SpaceS).Children(func() {
			for _, row := range group.keys {
				kit.Keycap(c, row.chord)
				ui.Text(c, row.action(msgs)).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			}
		})
	})
}
