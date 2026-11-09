package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// The keyboard help is the window's map: Cmd+/ opens it, every group and every
// chord on it, and Escape closes it. It reads the same table the rows are built
// from, so a key added to the map is on the page — a row that is dropped from the
// dialog fails here.
func TestKeysHelpListsTheMap(t *testing.T) {
	msgs := locales.In("en")
	_, _, tt := openApp(t)

	tt.Key(ui.Cmd, ui.KeySlash)
	tt.Frame()
	if !tt.HasText(msgs.KeysTitle()) {
		t.Fatalf("Cmd+/ opened no keyboard help; texts %q", tt.Texts())
	}
	groups := shortcutGroups()
	if len(groups) != 4 {
		t.Fatalf("the help has %d groups, want the four the window has", len(groups))
	}
	for _, group := range groups {
		if !tt.HasText(group.title(msgs)) {
			t.Errorf("the help has no %q group; texts %q", group.title(msgs), tt.Texts())
		}
		for _, row := range group.keys {
			if !tt.HasText(row.action(msgs)) {
				t.Errorf("the help does not list %q", row.action(msgs))
			}
		}
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if _, open := tt.Find(msgs.KeysTitle()); open {
		t.Error("Escape did not close the keyboard help")
	}
}

// Every overlay the window opens — the palette, the keys, the import wizard and
// the export dialog — fits the smallest window in the other language: a control
// laid out past the edge of the window is one nobody can reach.
func TestOverlaysAtTheMinimumSizeInChinese(t *testing.T) {
	msgs := locales.In("zh-Hans")
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	settle(tt)

	const w, h = 720, 520
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	assertControl(t, tt, w, h, msgs.PaletteSearch())
	assertControl(t, tt, w, h, msgs.PaletteCommands())
	tt.Key(0, ui.KeyEscape)

	tt.Key(ui.Cmd, ui.KeySlash)
	tt.Frame()
	assertControl(t, tt, w, h, msgs.KeysTitle())
	assertControl(t, tt, w, h, msgs.KeysGroupGlobal())
	tt.Key(0, ui.KeyEscape)

	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type(msgs.ImportTitle())
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	assertControl(t, tt, w, h, msgs.ImportTitle())
	assertControl(t, tt, w, h, msgs.ImportFromMarkdown())
	assertControl(t, tt, w, h, msgs.ImportNext())
	tt.Key(0, ui.KeyEscape)

	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type(msgs.ExportTitle())
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	assertControl(t, tt, w, h, msgs.ExportTitle())
	assertControl(t, tt, w, h, msgs.ExportCards())
	assertControl(t, tt, w, h, msgs.ExportRun())
	tt.Key(0, ui.KeyEscape)

	tt.Frame()
	if tt.FramePending() {
		t.Error("view requests frames after the overlays closed")
	}
}
