package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/update"
)

// fakeAgent is the agent API as the settings page drives it: it records what
// it was told, and reports an address and a token file the way cmd writes
// them, without binding a port.
type fakeAgent struct {
	on       bool
	rotated  int
	fail     error
	status   AgentStatus
	servedOn []bool
}

func newFakeAgent() *fakeAgent {
	return &fakeAgent{status: AgentStatus{Address: "127.0.0.1:51234", TokenPath: "/data/agent-token"}}
}

func (f *fakeAgent) SetEnabled(on bool) error {
	if f.fail != nil {
		return f.fail
	}
	f.on = on
	f.servedOn = append(f.servedOn, on)
	if !on {
		f.status.Address = ""
	}
	return nil
}

func (f *fakeAgent) RotateToken() error {
	if f.fail != nil {
		return f.fail
	}
	f.rotated++
	// A restarted listener takes the port the system gives it next.
	if f.on {
		f.status.Address = "127.0.0.1:51999"
	}
	return nil
}

func (f *fakeAgent) Status() AgentStatus { return f.status }

// settingsApp is a window on a seeded library whose settings page has every
// seam replaced: no dialog, no port, no network.
func settingsApp(t *testing.T, opts Options) *App {
	t.Helper()
	dir := t.TempDir()
	seedLibrary(t, dir)
	opts.LibraryDir = dir
	if opts.DataDir == "" {
		opts.DataDir = t.TempDir()
	}
	return NewApp(opts)
}

// The settings page is one key or one palette action away, at both window
// sizes, and every control on it is on screen and clickable. Esc goes back to
// the face it was opened from.
func TestSettingsPage(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			agent := newFakeAgent()
			a := settingsApp(t, Options{AgentControls: agent})
			tt := ui.NewTester(a.View, size.w, size.h)
			tt.Frame()
			tt.Key(ui.Cmd, ui.Key2)
			tt.Advance(200 * time.Millisecond) // the review card's exit

			tt.Key(ui.Cmd, ui.KeyComma)
			if a.mode != modeSettings {
				t.Fatalf("Cmd+, opened %v, want the settings page; texts %q", a.mode, tt.Texts())
			}
			for _, label := range []string{"Agent API", "Serve to agents", "Rotate token", "Updates", "Check at launch", "Check now"} {
				reveal(t, tt, size.w, size.h, label)
				assertControl(t, tt, size.w, size.h, label)
			}
			// The page says where the agent API is, from the files cmd writes.
			for _, want := range []string{agent.status.Address, "Token file: " + agent.status.TokenPath} {
				if !tt.HasText(want) {
					t.Fatalf("the page is missing %q; texts %q", want, tt.Texts())
				}
			}
			for _, label := range []string{"Review shortcut", "Apply", "Reminders", "Daily reminder"} {
				reveal(t, tt, size.w, size.h, label)
				assertControl(t, tt, size.w, size.h, label)
			}

			// Esc returns to the face the page was opened from.
			tt.Key(0, ui.KeyEscape)
			if a.mode != modeNotes {
				t.Fatalf("Esc left the page on %v, want notes; texts %q", a.mode, tt.Texts())
			}
			if tt.HasText("Rotate token") {
				t.Error("the settings page stayed up after Esc")
			}

			// The palette reaches it too.
			tt.Key(ui.Cmd, ui.KeyK)
			tt.Type("settings")
			tt.Key(0, ui.KeyEnter)
			if a.mode != modeSettings || !tt.HasText("Rotate token") {
				t.Fatalf("the palette did not open the settings page; texts %q", tt.Texts())
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// A switch on the page is the setting: it is written to settings.json, and the
// page reads it back, so it is still on after a restart.
func TestSettingsSurviveRestart(t *testing.T) {
	agent := newFakeAgent()
	data := t.TempDir()
	a := settingsApp(t, Options{DataDir: data, AgentControls: agent})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(200 * time.Millisecond) // the review card's exit
	tt.Key(ui.Cmd, ui.KeyComma)

	for _, field := range []string{"Serve to agents", "Check at launch", "Daily reminder"} {
		if err := tt.Click(field); err != nil {
			t.Fatalf("click %q: %v", field, err)
		}
	}

	want := settings.Settings{LibraryDir: a.Settings().LibraryDir, AgentServer: true, UpdateCheck: true, Notifications: true}
	got, err := settings.Load(data)
	if err != nil {
		t.Fatalf("settings.Load: %v", err)
	}
	if got != want {
		t.Fatalf("settings on disk = %+v, want %+v", got, want)
	}
	if !agent.on {
		t.Error("the agent switch did not start the listener")
	}
	if !a.prefs.agentOn || !a.prefs.updateCheck || !a.prefs.notifications {
		t.Errorf("the page did not read its own write back: %+v", a.prefs)
	}

	// The next start shows what the file holds, without being asked again.
	next := NewApp(Options{LibraryDir: a.dir, DataDir: data, AgentControls: newFakeAgent()})
	tt = ui.NewTester(next.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)
	if !next.prefs.agentOn || !next.prefs.updateCheck || !next.prefs.notifications {
		t.Fatalf("the restarted app shows %+v, want the settings written", next.prefs)
	}
	tt.Advance(time.Second) // the switch knobs settle
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Turning the agent API on or off is the listener, not a note for the next
// launch: the switch starts and stops it now. Rotating the token restarts the
// listener with the new one and the page shows the address it took.
func TestAgentSwitchesNow(t *testing.T) {
	agent := newFakeAgent()
	a := settingsApp(t, Options{AgentControls: agent})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)

	if err := tt.Click("Serve to agents"); err != nil {
		t.Fatal(err)
	}
	if !agent.on {
		t.Fatal("the switch did not start the listener")
	}
	if !tt.HasText("Listening on 127.0.0.1:51234") {
		t.Fatalf("the page does not say where it listens; texts %q", tt.Texts())
	}

	if err := tt.Click("Rotate token"); err != nil {
		t.Fatal(err)
	}
	if agent.rotated != 1 {
		t.Fatalf("Rotate token ran %d times", agent.rotated)
	}
	if !tt.HasText("New token active on the restarted listener.") {
		t.Fatalf("rotating the token said nothing; texts %q", tt.Texts())
	}
	if !tt.HasText("Listening on 127.0.0.1:51999") {
		t.Errorf("the page kept the old address; texts %q", tt.Texts())
	}

	tt.Advance(5 * time.Second) // let the toast go
	if err := tt.Click("Serve to agents"); err != nil {
		t.Fatal(err)
	}
	if agent.on {
		t.Error("the switch did not stop the listener")
	}
	if !tt.HasText("Not listening.") {
		t.Errorf("the page still claims to listen; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // past the toast and the switch knob
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A listener that will not start leaves the switch showing what the settings
// say, and says why, rather than pretending to serve.
func TestAgentFailureKeepsTheSwitch(t *testing.T) {
	agent := newFakeAgent()
	agent.fail = errors.New("address already in use")
	data := t.TempDir()
	a := settingsApp(t, Options{DataDir: data, AgentControls: agent})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)

	if err := tt.Click("Serve to agents"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("The agent API did not start: address already in use") {
		t.Fatalf("the failure was not reported; texts %q", tt.Texts())
	}
	if a.prefs.agentOn {
		t.Error("the switch stayed on after the listener refused")
	}
	got, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentServer {
		t.Error("a refused listener was remembered as on")
	}
}

// Check now is the check, and each answer is said where the action is: a newer
// version, being up to date, a build that cannot update itself, or the error.
// While the setting is off nothing is asked at all.
func TestCheckNow(t *testing.T) {
	tests := []struct {
		name  string
		on    bool
		check func(context.Context, settings.Settings) (update.Result, error)
		want  string
		calls int
	}{
		{
			name: "newer version",
			on:   true,
			check: func(context.Context, settings.Settings) (update.Result, error) {
				return update.Result{Version: "0.2.0"}, nil
			},
			want: "Version 0.2.0 is available.", calls: 1,
		},
		{
			name: "up to date",
			on:   true,
			check: func(context.Context, settings.Settings) (update.Result, error) {
				return update.Result{}, nil
			},
			want: "Remit is up to date.", calls: 1,
		},
		{
			name: "cannot update",
			on:   true,
			check: func(context.Context, settings.Settings) (update.Result, error) {
				return update.Result{}, errCannotUpdate
			},
			want: "This build cannot update itself.", calls: 1,
		},
		{
			name: "failure",
			on:   true,
			check: func(context.Context, settings.Settings) (update.Result, error) {
				return update.Result{}, errors.New("network down")
			},
			want: "The check failed: network down", calls: 1,
		},
		{
			name: "off",
			on:   false,
			check: func(context.Context, settings.Settings) (update.Result, error) {
				return update.Result{Version: "0.2.0"}, nil
			},
			want: "Update checks are off, so nothing was asked.", calls: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			a := settingsApp(t, Options{CheckUpdate: func(ctx context.Context, s settings.Settings) (update.Result, error) {
				calls++
				return test.check(ctx, s)
			}})
			if test.on {
				if err := a.UpdateSettings(func(s *settings.Settings) { s.UpdateCheck = true }); err != nil {
					t.Fatal(err)
				}
			}
			tt := ui.NewTester(a.View, 1024, 720)
			tt.Frame()
			tt.Key(ui.Cmd, ui.KeyComma)
			if err := tt.Click("Check now"); err != nil {
				t.Fatal(err)
			}
			if calls != test.calls {
				t.Fatalf("the check ran %d times, want %d", calls, test.calls)
			}
			if !tt.HasText(test.want) {
				t.Fatalf("missing %q; texts %q", test.want, tt.Texts())
			}
			tt.Advance(time.Second) // the button's press settles
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// Apply binds the accelerator, writes it, and leaves the old one bound when the
// combination is invalid or already taken. A startup that could not bind the
// shortcut says so on the page rather than only in the log.
func TestHotkeyApply(t *testing.T) {
	bound := ""
	fail := errors.New("shortcut is already registered")
	a := settingsApp(t, Options{BindHotkey: func(accelerator string) error {
		if accelerator == "Bogus" {
			return fail
		}
		bound = accelerator
		return nil
	}})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)

	// The page starts on the default accelerator; typing a new one binds it and
	// writes it.
	if got := a.prefs.hotkey; got != settings.DefaultHotkey {
		t.Errorf("the field holds %q, want the default %q", got, settings.DefaultHotkey)
	}
	if err := tt.Click("Review shortcut"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("CmdOrCtrl+Shift+J")
	if err := tt.Click("Apply"); err != nil {
		t.Fatal(err)
	}
	if bound != "CmdOrCtrl+Shift+J" {
		t.Fatalf("bound %q, want the accelerator typed", bound)
	}
	if got := a.Settings().Hotkey; got != "CmdOrCtrl+Shift+J" {
		t.Errorf("settings hold %q, want the accelerator typed", got)
	}
	if !tt.HasText("Shortcut set to CmdOrCtrl+Shift+J") {
		t.Fatalf("applying said nothing; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // past the toast

	// A combination that is taken is refused on the page and the binding that
	// worked stays: the seam never saw a second successful bind, and neither
	// did the settings.
	if err := tt.Click("Review shortcut"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("Bogus")
	if err := tt.Click("Apply"); err != nil {
		t.Fatal(err)
	}
	if bound != "CmdOrCtrl+Shift+J" {
		t.Fatalf("a rejected accelerator took the binding: %q", bound)
	}
	if !tt.HasText("That shortcut is not available: " + fail.Error()) {
		t.Fatalf("the rejection is not on the page; texts %q", tt.Texts())
	}
	if got := a.Settings().Hotkey; got != "CmdOrCtrl+Shift+J" {
		t.Errorf("settings hold %q after a rejected accelerator", got)
	}

	// A startup that could not bind the shortcut says so the next time the page
	// opens, not only in the log.
	a.NoteHotkeyFailure(errors.New("another app owns it"))
	tt.Advance(5 * time.Second)
	tt.Key(0, ui.KeyEscape)
	tt.Key(ui.Cmd, ui.KeyComma)
	if !tt.HasText("That shortcut is not available: another app owns it") {
		t.Fatalf("the startup failure is not on the page; texts %q", tt.Texts())
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// The settings page in Chinese at the smallest window: every control still has
// its box inside the window, not only its text.
func TestSettingsChineseAtTheSmallestWindow(t *testing.T) {
	a := settingsApp(t, Options{AgentControls: newFakeAgent()})
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)

	for _, label := range []string{"设置", "代理接口", "对代理开放", "更换令牌", "更新", "启动时检查", "立即检查", "复习快捷键", "应用", "每日提醒"} {
		reveal(t, tt, 720, 520, label)
		assertControl(t, tt, 720, 520, label)
	}
	if tt.HasText("Settings") {
		t.Error("the page kept its English title")
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
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

// The queue's number reaches a service without the service asking the window:
// the tray shows it. Grading a card moves it.
func TestRemainingReachesAService(t *testing.T) {
	a, _ := newTestApp(t)
	var got []int
	a.OnRemaining(func(n int) { got = append(got, n) })
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()
	if len(got) == 0 || got[len(got)-1] != 4 {
		t.Fatalf("a service was told %v, want the queue's 4 cards", got)
	}

	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key3)
	if last := got[len(got)-1]; last != 3 {
		t.Fatalf("after one grade the service was told %d, want 3", last)
	}
}

// The settings page is the app's, not the library's: a first run that has not
// made a library yet can still turn reminders and update checks on.
func TestSettingsWithoutALibrary(t *testing.T) {
	data := t.TempDir()
	a := NewApp(Options{LibraryDir: filepath.Join(t.TempDir(), "Remit"), DataDir: data})
	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()
	if !tt.HasText("Start your library") {
		t.Fatalf("the window is not on the start view; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.KeyComma)
	if a.mode != modeSettings {
		t.Fatalf("Cmd+, opened %v with no library", a.mode)
	}
	if err := tt.Click("Daily reminder"); err != nil {
		t.Fatal(err)
	}
	saved, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Notifications {
		t.Error("the reminder switch did not write the setting")
	}

	tt.Key(0, ui.KeyEscape)
	if !tt.HasText("Start your library") || a.mode != modeNotes {
		t.Fatalf("Esc did not go back to the start view; texts %q", tt.Texts())
	}
}

// Writing a preference tells whoever registered OnSettings, which is how the
// tray's pause item follows the page.
func TestOnSettingsIsTold(t *testing.T) {
	a := settingsApp(t, Options{})
	var told []bool
	a.OnSettings(func(s settings.Settings) { told = append(told, s.Notifications) })
	if err := a.UpdateSettings(func(s *settings.Settings) { s.Notifications = true }); err != nil {
		t.Fatal(err)
	}
	if len(told) != 2 || told[0] || !told[1] {
		t.Fatalf("told %v, want the current value then the new one", told)
	}
	if !a.Settings().Notifications {
		t.Error("the app does not hold the value it wrote")
	}
}

// A settings write that fails says so and leaves the page showing the file
// rather than the click: a switch the disk refused must not read as taken.
func TestSettingsWriteFailureRevertsTheSwitch(t *testing.T) {
	data := t.TempDir()
	a := settingsApp(t, Options{DataDir: data})
	if err := os.Chmod(data, 0o500); err != nil {
		t.Fatalf("make the data directory read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(data, 0o700) })

	tt := ui.NewTester(a.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)
	if err := tt.Click("Daily reminder"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Could not save the change.") {
		t.Fatalf("the failed write was not reported; texts %q", tt.Texts())
	}
	if a.prefs.notifications || a.Settings().Notifications {
		t.Error("the switch stayed on after the write failed")
	}
}
