package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/update"
	"github.com/transmit-bug/remit/locales"
)

// fakeAgent is the agent API as the settings page drives it: it records what
// it was told, and reports an address and a token file the way cmd writes
// them, without binding a port.
type fakeAgent struct {
	on       bool
	rotated  int
	token    string
	fail     error
	status   AgentStatus
	servedOn []bool
}

func newFakeAgent(t *testing.T) *fakeAgent {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent-token")
	return &fakeAgent{status: AgentStatus{Address: "127.0.0.1:51234", TokenPath: path}}
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
	// The token file is the one cmd writes: owner-only, one line.
	f.token = "fresh-token-" + strconv.Itoa(f.rotated)
	if err := os.WriteFile(f.status.TokenPath, []byte(f.token+"\n"), 0o600); err != nil {
		return err
	}
	// A restarted listener takes the port the system gives it next.
	if f.on {
		f.status.Address = "127.0.0.1:51999"
	}
	return nil
}

func (f *fakeAgent) Status() AgentStatus { return f.status }

// settingsApp is a window on a seeded library whose settings pages have every
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

// The settings area is one key or one palette action away, its eight pages are
// the sub-navigation, and each one opens on its own heading. Esc goes back to
// the face the area was opened from.
func TestSettingsPages(t *testing.T) {
	msgs := locales.In("en")
	pages := []struct {
		sub   subID
		page  string
		title string
	}{
		{subSettingsAppearance, msgs.AppearancePage(), msgs.SettingsLook()},
		{subSettingsReview, msgs.ReviewPage(), msgs.SettingsReviewRetention()},
		{subSettingsBudget, msgs.BudgetPage(), msgs.SettingsBudgetCards()},
		{subSettingsKeys, msgs.KeysPage(), msgs.SettingsKeysChange()},
		{subSettingsPlugins, msgs.SettingsPlugins(), msgs.SettingsPluginsBody()},
		{subSettingsAgent, msgs.AgentPage(), msgs.SettingsAgentServe()},
		{subSettingsUpdate, msgs.UpdatePage(), msgs.SettingsCheckAtLaunch()},
		{subSettingsAbout, msgs.AboutPage(), msgs.SettingsAboutNotes()},
	}
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			agent := newFakeAgent(t)
			a := settingsApp(t, Options{AgentControls: agent})
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			tt.Key(ui.Cmd, ui.Key2)
			tt.Advance(320 * time.Millisecond) // the review card's exit

			tt.Key(ui.Cmd, ui.KeyComma)
			if a.mode != modeSettings || a.sub != subSettingsAppearance {
				t.Fatalf("Cmd+, opened %v/%v, want the appearance page; texts %q", a.mode, a.sub, tt.Texts())
			}
			// No Types page: the card-type editor moves to its own package in
			// W5, and the sub-navigation does not offer it until then.
			if got := len(subsOf(areaSettings)); got != len(pages) {
				t.Errorf("the sub-navigation holds %d pages, want the eight", got)
			}

			for _, page := range pages {
				settingsPage(t, tt, a, page.sub)
				reveal(t, tt, size.w, size.h, page.title)
				if !tt.HasText(page.title) {
					t.Fatalf("the %s page is missing %q; texts %q", page.page, page.title, tt.Texts())
				}
				reveal(t, tt, size.w, size.h, page.page)
				if !tt.HasText(page.page) {
					t.Fatalf("the %s page has no heading; texts %q", page.page, tt.Texts())
				}
			}

			// Esc returns to the face the area was opened from.
			tt.Key(0, ui.KeyEscape)
			if a.mode != modeNotes {
				t.Fatalf("Esc left the page on %v, want notes; texts %q", a.mode, tt.Texts())
			}
			if tt.HasText(msgs.AboutPage()) {
				t.Error("the settings page stayed up after Esc")
			}

			// The palette reaches it too, and it reopens the page the user was
			// on last rather than the first one.
			tt.Key(ui.Cmd, ui.KeyK)
			tt.Type("settings")
			tt.Key(0, ui.KeyEnter)
			if a.mode != modeSettings || a.sub != subSettingsAbout {
				t.Fatalf("the palette opened %v/%v, want the settings page the user left; texts %q",
					a.mode, a.sub, tt.Texts())
			}
			if !tt.HasText(msgs.AboutPage()) {
				t.Fatalf("the palette did not open the settings page; texts %q", tt.Texts())
			}
			tt.Advance(320 * time.Millisecond) // the screen's entrance
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// The look is the app's own preference: choosing ink writes the override into
// settings.json, and the window draws from the dark palette on the next frame,
// whether or not the desktop is dark.
func TestSettingsAppearanceWritesTheOverride(t *testing.T) {
	msgs := locales.In("en")
	data := t.TempDir()
	a := settingsApp(t, Options{DataDir: data})
	var got *ui.Theme
	var tk design.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
		got, tk = c.Theme(), a.tokens(c)
	}, 1024, 720)
	tt.SetDark(false) // the desktop is light: the override must win
	settle(tt)
	settingsPage(t, tt, a, subSettingsAppearance)

	if got.Dark {
		t.Fatal("the window started dark with no override")
	}
	reveal(t, tt, 1024, 720, msgs.SettingsLookInk())
	if err := tt.Click(msgs.SettingsLookInk()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got == nil || !got.Dark {
		t.Fatalf("the window did not draw ink after the choice: dark=%v", got.Dark)
	}
	if !sameColor(got.Background, design.Dark().Page) {
		t.Errorf("the page is %v, want the dark palette's", got.Background)
	}
	saved, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if saved.AppearanceName() != settings.AppearanceInk {
		t.Fatalf("settings.json holds %q, want ink", saved.Appearance)
	}

	// Back to the desktop's own look: the override is gone and the light theme
	// comes back.
	reveal(t, tt, 1024, 720, msgs.SettingsLookSystem())
	if err := tt.Click(msgs.SettingsLookSystem()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got.Dark {
		t.Error("the window stayed dark after choosing to follow the desktop")
	}

	// The paper's measure is the other half of the page, and it reaches the
	// tokens the views draw with.
	stepSlider(t, tt, msgs.SettingsPaperWidth(), float64(design.PaperWidth), settings.PaperWidthMin, settings.PaperWidthMax, 1)
	if want := design.PaperWidth + settings.PaperWidthStep; tk.PaperWidth != want {
		t.Errorf("the views draw %v wide, want %v", tk.PaperWidth, want)
	}
}

// The review page writes the library's presets, not the app's settings: the
// retention override lands in remit.json, and the library schedules with it.
func TestSettingsReviewWritesThePresets(t *testing.T) {
	msgs := locales.In("en")
	a := settingsApp(t, Options{})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	settingsPage(t, tt, a, subSettingsReview)

	stepSlider(t, tt, msgs.SettingsReviewRetention(), 90, 80, 97, 3)
	presets := a.lib.Presets()
	if presets.Review.Retention == nil || *presets.Review.Retention != 0.93 {
		t.Fatalf("the library's retention is %v, want the 93%% the page set", presets.Review.Retention)
	}
	config := configOf(t, a.dir)
	presetConfig, _ := config["presets"].(map[string]any)
	review, _ := presetConfig["review"].(map[string]any)
	if got, _ := review["retention"].(float64); got != 0.93 {
		t.Fatalf("remit.json holds retention %v, want 0.93", review["retention"])
	}

	// The furthest a card may wait is a preset too.
	stepSlider(t, tt, msgs.SettingsReviewMaxInterval(), 180, 30, 365, 1)
	if got := a.lib.Presets().Review.MaxIntervalDays; got != 210 {
		t.Fatalf("the longest interval is %d days, want 210", got)
	}

	reveal(t, tt, 1024, 720, msgs.SettingsReviewPreview())
	if err := tt.Click(msgs.SettingsReviewPreview()); err != nil {
		t.Fatal(err)
	}
	if a.lib.Presets().Review.Preview {
		t.Error("the interval preview stayed on after the page turned it off")
	}

	// The app's own settings file holds none of it: a pace follows the library.
	saved, err := settings.Load(a.settingsDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := (settings.Settings{LibraryDir: saved.LibraryDir}); !reflect.DeepEqual(saved, want) {
		t.Errorf("settings.json gained %+v, want the library path alone", saved)
	}
}

// The budget page's preview is today's queue at the presets' pace, derived by
// the same function and options the Today page asks: the two cannot promise
// different days, and moving a slider moves the numbers.
func TestSettingsBudgetPreviewMatchesToday(t *testing.T) {
	msgs := locales.In("en")
	a := settingsApp(t, Options{})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	settingsPage(t, tt, a, subSettingsBudget)

	// A day of 45 minutes and 30 cards, written by the page.
	stepSlider(t, tt, msgs.SettingsBudgetCards(), 20, 5, 60, 2)
	stepSlider(t, tt, msgs.SettingsBudgetMinutes(), 30, 10, 90, 3)
	presets := a.lib.Presets()
	if presets.Budget.Cards != 30 || presets.Budget.Minutes != 45 {
		t.Fatalf("the presets hold %d cards and %d minutes, want 30 and 45", presets.Budget.Cards, presets.Budget.Minutes)
	}

	today, err := queue.Entries(a.lib, time.Now(), todayOptions(presets))
	if err != nil {
		t.Fatalf("queue.Entries: %v", err)
	}
	want := msgs.SettingsBudgetPreviewBody(
		today.ReadMinutes, today.ReviewMinutes, today.Budget, len(today.Entries))
	reveal(t, tt, 1024, 720, want)
	if !tt.HasText(want) {
		t.Fatalf("the preview says %q, want the day the queue builds %q", tt.Texts(), want)
	}
	if a.prefs.Budget.Total != today.TotalMinutes || a.prefs.Budget.Lines != len(today.Entries) {
		t.Errorf("the page's snapshot is %+v, want the queue's %d minutes in %d lines",
			a.prefs.Budget, today.TotalMinutes, len(today.Entries))
	}

	// The tray title beside it is the message the tray itself sets.
	if !tt.HasText(msgs.TrayTitle(a.queueCounts().Cards)) {
		t.Errorf("the page does not show the tray's title; texts %q", tt.Texts())
	}
}

// A switch on a page is the setting: it is written to settings.json, and the
// page reads it back, so it is still on after a restart.
func TestSettingsSurviveRestart(t *testing.T) {
	agent := newFakeAgent(t)
	data := t.TempDir()
	a := settingsApp(t, Options{DataDir: data, AgentControls: agent})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit

	settingsPage(t, tt, a, subSettingsAgent)
	reveal(t, tt, 1024, 720, "Serve to agents")
	if err := tt.Click("Serve to agents"); err != nil {
		t.Fatalf("click the agent switch: %v", err)
	}
	settingsPage(t, tt, a, subSettingsUpdate)
	reveal(t, tt, 1024, 720, "Check at launch")
	if err := tt.Click("Check at launch"); err != nil {
		t.Fatalf("click the update switch: %v", err)
	}
	settingsPage(t, tt, a, subSettingsBudget)
	reveal(t, tt, 1024, 720, "Daily reminder")
	if err := tt.Click("Daily reminder"); err != nil {
		t.Fatalf("click the reminder switch: %v", err)
	}

	want := settings.Settings{LibraryDir: a.Settings().LibraryDir, AgentServer: true, UpdateCheck: true, Notifications: true}
	got, err := settings.Load(data)
	if err != nil {
		t.Fatalf("settings.Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("settings on disk = %+v, want %+v", got, want)
	}
	if !agent.on {
		t.Error("the agent switch did not start the listener")
	}
	if !a.prefs.AgentOn || !a.prefs.UpdateCheck || !a.prefs.Reminder {
		t.Errorf("the page did not read its own write back: %+v", a.prefs)
	}

	// The next start shows what the file holds, without being asked again.
	next := NewApp(Options{LibraryDir: a.dir, DataDir: data, AgentControls: newFakeAgent(t)})
	tt = ui.NewTester(next.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyComma)
	if !next.prefs.AgentOn || !next.prefs.UpdateCheck || !next.prefs.Reminder {
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
	agent := newFakeAgent(t)
	a := settingsApp(t, Options{AgentControls: agent})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	settingsPage(t, tt, a, subSettingsAgent)

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
	// A token is shown once, from the file the rotation wrote; the next write
	// anywhere takes it off the page.
	if !tt.HasText(agent.token) {
		t.Fatalf("the page does not show the token it minted; texts %q", tt.Texts())
	}
	if err := a.UpdateSettings(func(s *settings.Settings) { s.Notifications = true }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(agent.token) {
		t.Error("the page kept showing a token after another write")
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
	agent := newFakeAgent(t)
	agent.fail = errors.New("address already in use")
	data := t.TempDir()
	a := settingsApp(t, Options{DataDir: data, AgentControls: agent})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	settingsPage(t, tt, a, subSettingsAgent)

	if err := tt.Click("Serve to agents"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("The agent API did not start: address already in use") {
		t.Fatalf("the failure was not reported; texts %q", tt.Texts())
	}
	if a.prefs.AgentOn {
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
			settle(tt)
			settingsPage(t, tt, a, subSettingsUpdate)
			reveal(t, tt, 1024, 720, "Check now")
			if err := tt.Click("Check now"); err != nil {
				t.Fatal(err)
			}
			if calls != test.calls {
				t.Fatalf("the check ran %d times, want %d", calls, test.calls)
			}
			reveal(t, tt, 1024, 720, test.want)
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

// Rebuilding the index derives it again from the plaintext: the library is
// still whole, its notes are there and the counts on the about page agree.
func TestReindexKeepsTheLibrary(t *testing.T) {
	a := settingsApp(t, Options{})
	before := len(a.data.notes)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	settingsPage(t, tt, a, subSettingsUpdate)

	clickButton(t, tt, locales.In("en").SettingsReindex())
	tt.Frame()
	if want := locales.In("en").SettingsReindexed(); !tt.HasText(want) {
		t.Fatalf("the rebuild said nothing; texts %q", tt.Texts())
	}
	if got := len(a.data.notes); got != before {
		t.Fatalf("the library holds %d notes after the rebuild, want %d", got, before)
	}
	if _, ok := a.lib.Note(a.data.notes[0].ID); !ok {
		t.Fatal("the rebuilt index does not find the note back")
	}
}

// The about page states the library's facts: its path and what it holds.
func TestAboutShowsTheLibraryFacts(t *testing.T) {
	msgs := locales.In("en")
	a := settingsApp(t, Options{})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	settingsPage(t, tt, a, subSettingsAbout)

	for _, want := range []string{
		Name, msgs.SettingsVersion(Version),
		a.dir, msgs.SettingsAboutNotes(), msgs.SettingsAboutCards(),
		msgs.SettingsAboutQueue(),
	} {
		reveal(t, tt, 1024, 720, want)
		if !tt.HasText(want) {
			t.Fatalf("the about page is missing %q; texts %q", want, tt.Texts())
		}
	}
	if !tt.HasText(locales.In("en").SettingsAboutTagline()) {
		t.Errorf("the about page does not say what the app is; texts %q", tt.Texts())
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
	settle(tt)
	settingsPage(t, tt, a, subSettingsKeys)

	// The page starts on the default accelerator; typing a new one binds it and
	// writes it.
	if got := a.prefs.Hotkey; got != settings.DefaultHotkey {
		t.Errorf("the field holds %q, want the default %q", got, settings.DefaultHotkey)
	}
	if err := tt.Click("Review shortcut"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("CmdOrCtrl+Shift+J")
	tt.Frame()
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
	tt.Frame()
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
	settingsPage(t, tt, a, subSettingsKeys)
	if !tt.HasText("That shortcut is not available: another app owns it") {
		t.Fatalf("the startup failure is not on the page; texts %q", tt.Texts())
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// reveal scrolls a page until s has a box inside the window, which is what
// makes a control below or above the fold visible and clickable.
func reveal(t *testing.T, tt *ui.Tester, w, h int, s string) {
	t.Helper()
	inside := func(r ui.Rect) bool {
		return r.W > 0 && r.H > 0 && r.X >= 0 && r.Y >= 0 &&
			r.X+r.W <= float32(w) && r.Y+r.H <= float32(h)
	}
	// A box the scroll culled reads as zero-sized, so the target's side of the
	// window cannot be told from it: go back to the top first, then down.
	cx, cy := float32(w)/2, float32(h)/2
	for i := 0; i < 20; i++ {
		tt.Scroll(cx, cy, 0, -120)
	}
	for i := 0; i < 40; i++ {
		if r, ok := tt.Find(s); ok && inside(r) {
			return
		}
		tt.Scroll(cx, cy, 0, 120)
	}
	r, ok := tt.Find(s)
	t.Fatalf("%q did not scroll into the window: box %+v (found %v); texts %q", s, r, ok, tt.Texts())
}

// stepSlider focuses a slider where its knob is — the value it already holds,
// so the click changes nothing — and moves it by presses of the Right arrow.
func stepSlider(t *testing.T, tt *ui.Tester, name string, from, lo, hi float64, presses int) {
	t.Helper()
	reveal(t, tt, 1024, 720, name)
	info, ok := tt.AccessByRole(ui.RoleSlider, name)
	if !ok {
		t.Fatalf("the page draws no slider named %q; texts %q", name, tt.Texts())
	}
	x := info.Bounds.X + info.Bounds.W*float32((from-lo)/(hi-lo))
	tt.ClickAt(x, info.Bounds.Y+info.Bounds.H/2)
	tt.Frame()
	for i := 0; i < presses; i++ {
		tt.Key(0, ui.KeyRight)
		tt.Frame()
	}
}

// The queues' numbers reach a service without the service asking the window:
// the tray shows both. Grading a card moves the cards' count.
func TestCountsReachAService(t *testing.T) {
	a, _ := newTestApp(t)
	var got []Counts
	a.OnCounts(func(c Counts) { got = append(got, c) })
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallQueue(tt)
	if len(got) == 0 || got[len(got)-1].Cards != 4 {
		t.Fatalf("a service was told %v, want the queue's 4 cards", got)
	}
	if got[len(got)-1].Reading != 0 {
		t.Fatalf("a service was told %d notes to read, want none", got[len(got)-1].Reading)
	}

	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key3)
	if last := got[len(got)-1].Cards; last != 3 {
		t.Fatalf("after one grade the service was told %d, want 3", last)
	}
}

// The settings area is the app's, not the library's: a first run that has not
// made a library yet can still turn the reminder and update checks on.
func TestSettingsWithoutALibrary(t *testing.T) {
	data := t.TempDir()
	a := NewApp(Options{LibraryDir: filepath.Join(t.TempDir(), "Remit"), DataDir: data})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	if !tt.HasText("Start your library") {
		t.Fatalf("the window is not on the start view; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.KeyComma)
	if a.mode != modeSettings {
		t.Fatalf("Cmd+, opened %v with no library", a.mode)
	}
	settingsPage(t, tt, a, subSettingsBudget)
	reveal(t, tt, 1024, 720, "Daily reminder")
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
	settingsPage(t, tt, a, subSettingsUpdate)
	reveal(t, tt, 1024, 720, "Check at launch")
	if err := tt.Click("Check at launch"); err != nil {
		t.Fatal(err)
	}
	if !a.Settings().UpdateCheck {
		t.Error("the update switch did not write the setting without a library")
	}

	tt.Key(0, ui.KeyEscape)
	if !tt.HasText("Start your library") || a.area != areaToday {
		t.Fatalf("Esc did not go back to the start view; area %v, texts %q", a.area, tt.Texts())
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
	settle(tt)
	settingsPage(t, tt, a, subSettingsBudget)
	reveal(t, tt, 1024, 720, "Daily reminder")
	if err := tt.Click("Daily reminder"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Could not save the change.") {
		t.Fatalf("the failed write was not reported; texts %q", tt.Texts())
	}
	if a.prefs.Reminder || a.Settings().Notifications {
		t.Error("the switch stayed on after the write failed")
	}
}

// clickButton clicks the button named name, whichever legend happens to carry
// the same words above it.
func clickButton(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	reveal(t, tt, 1024, 720, name)
	info, ok := tt.AccessByRole(ui.RoleButton, name)
	if !ok {
		t.Fatalf("the page draws no button named %q; texts %q", name, tt.Texts())
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	tt.Frame()
}
