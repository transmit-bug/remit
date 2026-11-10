package settings

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// keyRow is one line of the keyboard map: what the key does, and the keys that
// do it. A row with two sets of keys means either works.
type keyRow struct {
	what string
	keys [][]string
}

// keyGroup is one group of the map, in the order the page lists them.
type keyGroup struct {
	name string
	rows []keyRow
}

// shortcutMap is the keyboard map the window binds, grouped as the page shows
// it. It is the page's own copy of what window.go declares: a row appears here
// once its key is bound, never before, so the page cannot promise a key the
// window does not answer. W8's shortcut help (S48) reads the same map, which is
// why the rows carry no host state.
func shortcutMap(msgs locales.Messages) []keyGroup {
	cmd, shift := primaryModifier(), shiftModifier()
	return []keyGroup{
		{name: msgs.SettingsKeysGlobal(), rows: []keyRow{
			{msgs.SettingsKeysPalette(), [][]string{{cmd, "K"}}},
			{msgs.SettingsKeysOpenSettings(), [][]string{{cmd, ","}}},
			{msgs.SettingsKeysNewNote(), [][]string{{cmd, "N"}}},
			{msgs.SettingsKeysDailyNote(), [][]string{{cmd, "D"}}},
			{msgs.SettingsKeysFind(), [][]string{{cmd, "F"}}},
			{msgs.SettingsKeysAreas(), [][]string{{cmd, "1"}, {cmd, "6"}}},
			{msgs.SettingsKeysBack(), [][]string{{"Esc"}}},
		}},
		{name: msgs.SettingsKeysReading(), rows: []keyRow{
			{msgs.SettingsKeysMove(), [][]string{{"J"}, {"K"}, {"↓"}, {"↑"}}},
			{msgs.SettingsKeysReadingGrade(), [][]string{{"1"}, {"2"}, {"3"}}},
			{msgs.SettingsKeysExcerpt(), [][]string{{"E"}}},
			{msgs.SettingsKeysSkip(), [][]string{{"S"}}},
			{msgs.SettingsKeysDrop(), [][]string{{"X"}}},
			{msgs.SettingsKeysOpenNote(), [][]string{{"M"}}},
		}},
		{name: msgs.SettingsKeysReview(), rows: []keyRow{
			{msgs.SettingsKeysReveal(), [][]string{{"Space"}}},
			{msgs.SettingsKeysGrade(), [][]string{{"1"}, {"4"}}},
			{msgs.SettingsKeysUndo(), [][]string{{"U"}}},
			{msgs.SettingsKeysSuspend(), [][]string{{"S"}}},
			{msgs.SettingsKeysEditCard(), [][]string{{"E"}}},
		}},
		{name: msgs.SettingsKeysDocument(), rows: []keyRow{
			{msgs.SettingsKeysPanel(), [][]string{{cmd, shift, "P"}}},
			{msgs.SettingsKeysOutline(), [][]string{{cmd, shift, "O"}}},
			{msgs.SettingsKeysBacklinks(), [][]string{{cmd, shift, "B"}}},
		}},
	}
}

// primaryModifier and shiftModifier are the modifiers as this desktop writes
// them: the symbols macOS shows, the names the others use. MyGo's ui.Cmd is the
// platform's primary modifier, which is what the map prints here.
func primaryModifier() string {
	if runtime.GOOS == "darwin" {
		return "⌘"
	}
	return "Ctrl"
}

func shiftModifier() string {
	if runtime.GOOS == "darwin" {
		return "⇧"
	}
	return "Shift"
}

// keysPage is the keyboard map: read-only, with the one key that is a
// preference above it — the global shortcut that starts a review — and the
// reason the rest cannot be changed yet said where the change would be made.
func keysPage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	ui.Row(c).Gap(tk.SpaceM).AlignItems(ui.Center).Children(func() {
		ui.Button(c, msgs.SettingsKeysChange()).MinHeight(tk.Control).
			Disabled(true).Label(msgs.SettingsKeysChange())
		muted(c, tk, msgs.SettingsKeysLocked())
	})

	for _, group := range shortcutMap(msgs) {
		ui.Fieldset(c, group.name, func() {
			if group.name == msgs.SettingsKeysGlobal() {
				hotkeyField(c, s, d, msgs, tk)
			}
			for _, row := range group.rows {
				keyLine(c, tk, row)
			}
		})
	}
}

// hotkeyField is the global review accelerator: the one key the user owns,
// because it is the app's shortcut rather than the map's. A combination another
// app owns leaves the old binding in place and is reported here.
func hotkeyField(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	ui.Field(c, msgs.SettingsShortcutLabel(), func() {
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &s.Hotkey).
				Placeholder(msgs.SettingsShortcutLabel()).MinHeight(tk.Control).Grow(1).
				Label(msgs.SettingsShortcutLabel())
			ui.Button(c, msgs.ApplyShortcut()).MinHeight(tk.Control).
				OnClick(func() { d.applyHotkey(c, s.Hotkey) })
		})
	}).Description(msgs.SettingsShortcutBody()).Error(s.hotkeyError(msgs))
}

// hotkeyError is the shortcut field's inline error, empty while the last
// attempt worked.
func (s *State) hotkeyError(msgs locales.Messages) string {
	if s.HotkeyErr == "" {
		return ""
	}
	return msgs.ShortcutUnavailable(s.HotkeyErr)
}

// keyLine is one row of the map: its sentence, then its keys, each on the face
// of a key. The sentence is the row's text, read before the keys that do it.
func keyLine(c *ui.Context, tk design.Tokens, row keyRow) {
	ui.Row(c).FillWidth().Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
		ui.Text(c, row.what).FontSize(tk.TypeUI).Grow(1)
		for _, keys := range row.keys {
			ui.Row(c).Gap(tk.SpaceXS).AlignItems(ui.Center).Children(func() {
				for _, key := range keys {
					kit.Keycap(c, key)
				}
			})
		}
	})
}

func (d Deps) applyHotkey(c *ui.Context, accelerator string) {
	if d.ApplyHotkey != nil {
		d.ApplyHotkey(c, accelerator)
	}
}
