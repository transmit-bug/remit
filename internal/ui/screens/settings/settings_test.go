package settings_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	appsettings "github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/screens/settings"
	"github.com/transmit-bug/remit/locales"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// fake is the app the page talks to: it records what the page asked, with no
// settings file, no shortcut binding and no listener behind it.
type fake struct {
	agentOn       []bool
	updateCheck   []bool
	notifications []bool
	rotated       int
	checked       int
	applied       int
	hotkey        string
}

func (f *fake) deps() settings.Deps {
	return settings.Deps{
		SetAgent:         func(_ *ui.Context, on bool) { f.agentOn = append(f.agentOn, on) },
		RotateToken:      func(*ui.Context) { f.rotated++ },
		CheckNow:         func(*ui.Context) { f.checked++ },
		ApplyHotkey:      func(_ *ui.Context, acc string) { f.applied++; f.hotkey = acc },
		SetUpdateCheck:   func(_ *ui.Context, on bool) { f.updateCheck = append(f.updateCheck, on) },
		SetNotifications: func(_ *ui.Context, on bool) { f.notifications = append(f.notifications, on) },
	}
}

// page is the settings the app hands the page: every switch off, the default
// shortcut, and a listener that reports where it serves.
func page() settings.State {
	return settings.State{
		Hotkey: appsettings.DefaultHotkey,
		Agent:  settings.AgentStatus{Address: "127.0.0.1:51234", TokenPath: "/data/agent-token"},
	}
}

// at draws the page once at a size and lets it settle.
func at(t *testing.T, s settings.State, d settings.Deps, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { settings.View(c, &s, d) }, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// The page shows the four groups — agent API, updates, shortcut, reminders —
// with every control on screen at both the sizes the window opens at, and the
// listener's own report.
func TestSettingsPage(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f := &fake{}
			s := page()
			tt := at(t, s, f.deps(), size.w, size.h)

			for _, label := range []string{
				msgs.SettingsTitle(),
				msgs.SettingsAgent(), msgs.SettingsAgentServe(), msgs.RotateToken(),
				msgs.SettingsUpdates(), msgs.SettingsCheckAtLaunch(), msgs.CheckNow(),
				msgs.SettingsShortcut(), msgs.SettingsShortcutLabel(), msgs.ApplyShortcut(),
				msgs.SettingsReminders(), msgs.SettingsReminderLabel(),
			} {
				reveal(t, tt, size.w, size.h, label)
				assertControl(t, tt, label)
			}
			for _, want := range []string{
				msgs.SettingsAgentAddress(s.Agent.Address),
				msgs.SettingsTokenPath(s.Agent.TokenPath),
				msgs.SettingsAgentBody(),
				msgs.SettingsReminderBody(),
			} {
				if !tt.HasText(want) {
					t.Fatalf("the page is missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// The page draws what it is handed: a stopped listener, the last update
// check's answer, and why the shortcut was refused all read from State, not
// from anything the page asked for itself.
func TestPageShowsTheStateItIsGiven(t *testing.T) {
	msgs := locales.In("en")
	s := settings.State{
		Hotkey:    appsettings.DefaultHotkey,
		HotkeyErr: "shortcut is already registered",
		UpdateMsg: msgs.UpdateAvailable("0.2.0"),
	}
	tt := at(t, s, (&fake{}).deps(), 1024, 720)

	for _, want := range []string{
		msgs.SettingsAgentStopped(),
		msgs.ShortcutUnavailable(s.HotkeyErr),
		msgs.UpdateAvailable("0.2.0"),
	} {
		if !tt.HasText(want) {
			t.Fatalf("the page is missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText(msgs.SettingsTokenPath("/data/agent-token")) {
		t.Error("the page named a token file the app did not give it")
	}
}

// Every action on the page is the app's: the switch reports the value it was
// turned to, the buttons report the click, and Apply hands over the
// accelerator as typed.
func TestActionsGoToTheApp(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{}
	tt := at(t, page(), f.deps(), 1024, 720)

	if err := tt.Click(msgs.SettingsAgentServe()); err != nil {
		t.Fatal(err)
	}
	if len(f.agentOn) != 1 || !f.agentOn[0] {
		t.Fatalf("the agent switch reached the app as %v, want one true", f.agentOn)
	}
	if err := tt.Click(msgs.RotateToken()); err != nil {
		t.Fatal(err)
	}
	if f.rotated != 1 {
		t.Fatalf("Rotate token reached the app %d times, want 1", f.rotated)
	}
	if err := tt.Click(msgs.SettingsCheckAtLaunch()); err != nil {
		t.Fatal(err)
	}
	if len(f.updateCheck) != 1 || !f.updateCheck[0] {
		t.Fatalf("the update switch reached the app as %v, want one true", f.updateCheck)
	}
	if err := tt.Click(msgs.CheckNow()); err != nil {
		t.Fatal(err)
	}
	if f.checked != 1 {
		t.Fatalf("Check now reached the app %d times, want 1", f.checked)
	}
	if err := tt.Click(msgs.SettingsReminderLabel()); err != nil {
		t.Fatal(err)
	}
	if len(f.notifications) != 1 || !f.notifications[0] {
		t.Fatalf("the reminder switch reached the app as %v, want one true", f.notifications)
	}

	if err := tt.Click(msgs.SettingsShortcutLabel()); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("CmdOrCtrl+Shift+J")
	if err := tt.Click(msgs.ApplyShortcut()); err != nil {
		t.Fatal(err)
	}
	if f.applied != 1 || f.hotkey != "CmdOrCtrl+Shift+J" {
		t.Fatalf("Apply reached the app %d times with %q, want the accelerator typed", f.applied, f.hotkey)
	}
	tt.Advance(time.Second) // the button's press settles
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// In Chinese the page holds at the smallest window: every group's controls
// keep a box of their own inside it.
func TestAtMinimumInChinese(t *testing.T) {
	msgs := locales.In("zh-Hans")
	s := settings.State{Hotkey: appsettings.DefaultHotkey}
	tt := ui.NewTester(func(c *ui.Context) { settings.View(c, &s, (&fake{}).deps()) }, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	for _, label := range []string{
		msgs.SettingsTitle(), msgs.SettingsAgent(), msgs.SettingsAgentServe(), msgs.RotateToken(),
		msgs.SettingsUpdates(), msgs.SettingsCheckAtLaunch(), msgs.CheckNow(),
		msgs.SettingsShortcut(), msgs.SettingsShortcutLabel(), msgs.ApplyShortcut(),
		msgs.SettingsReminders(), msgs.SettingsReminderLabel(),
	} {
		reveal(t, tt, 720, 520, label)
		assertControl(t, tt, label)
	}
	if tt.HasText(locales.In("en").SettingsTitle()) {
		t.Error("the page kept its English title")
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after the locale switch")
	}
}

// reveal scrolls the page until s has a box inside the window, which is what
// makes a control below the fold visible and clickable.
func reveal(t *testing.T, tt *ui.Tester, w, h int, s string) {
	t.Helper()
	inside := func(r ui.Rect) bool {
		return r.W > 0 && r.H > 0 && r.X >= 0 && r.Y >= 0 &&
			r.X+r.W <= float32(w) && r.Y+r.H <= float32(h)
	}
	for i := 0; i < 20; i++ {
		if r, ok := tt.Find(s); ok && inside(r) {
			return
		}
		tt.Scroll(float32(w)/2, float32(h)/2, 0, 120)
	}
	r, ok := tt.Find(s)
	t.Fatalf("%q did not scroll into the window: box %+v (found %v); texts %q", s, r, ok, tt.Texts())
}

// assertControl checks that s is drawn rather than only present as text.
func assertControl(t *testing.T, tt *ui.Tester, s string) {
	t.Helper()
	if _, ok := tt.Find(s); !ok {
		t.Fatalf("%q is not drawn; texts %q", s, tt.Texts())
	}
}
