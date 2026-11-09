package settings_test

import (
	"strings"
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

// fake is the app the pages talk to: it records what each control asked, and
// writes it back into the state the pages read, the way the app's own writes
// retake their snapshot. No settings file, no library and no listener is behind
// it.
type fake struct {
	state *settings.State

	look       []string
	paperWidth []int
	system     int

	retention  []int
	interval   []int
	preview    []bool
	returned   []bool
	budget     []budgetChange
	reminder   []bool
	hotkey     []string
	agent      []bool
	rotated    int
	update     []bool
	checked    int
	reindexed  int
	exported   int
	licensed   int
	openedPage []string
}

type budgetChange struct{ cards, minutes int }

func (f *fake) deps() settings.Deps {
	return settings.Deps{
		SetAppearance: func(_ *ui.Context, look string) {
			f.look = append(f.look, look)
			f.state.Appearance = look
		},
		SetPaperWidth: func(_ *ui.Context, width int) {
			f.paperWidth = append(f.paperWidth, width)
			f.state.PaperWidth = width
		},
		OpenDisplaySettings: func(*ui.Context) { f.system++ },
		SetRetention: func(_ *ui.Context, percent int) {
			f.retention = append(f.retention, percent)
			f.state.RetentionPercent = percent
		},
		SetMaxInterval: func(_ *ui.Context, days int) {
			f.interval = append(f.interval, days)
			f.state.MaxIntervalDays = days
		},
		SetIntervalPreview: func(_ *ui.Context, on bool) {
			f.preview = append(f.preview, on)
			f.state.IntervalPreview = on
		},
		SetReturnAfterExcerpt: func(_ *ui.Context, on bool) {
			f.returned = append(f.returned, on)
			f.state.ReturnAfterExcerpt = on
		},
		SetBudget: func(_ *ui.Context, cards, minutes int) {
			f.budget = append(f.budget, budgetChange{cards, minutes})
			f.state.BudgetCards, f.state.BudgetMinutes = cards, minutes
		},
		SetNotifications: func(_ *ui.Context, on bool) {
			f.reminder = append(f.reminder, on)
			f.state.Reminder = on
		},
		ApplyHotkey: func(_ *ui.Context, accelerator string) {
			f.hotkey = append(f.hotkey, accelerator)
			f.state.Hotkey = accelerator
		},
		SetAgent: func(_ *ui.Context, on bool) {
			f.agent = append(f.agent, on)
			f.state.AgentOn = on
		},
		RotateToken: func(*ui.Context) { f.rotated++ },
		SetUpdateCheck: func(_ *ui.Context, on bool) {
			f.update = append(f.update, on)
			f.state.UpdateCheck = on
		},
		CheckNow:      func(*ui.Context) { f.checked++ },
		Reindex:       func(*ui.Context) { f.reindexed++ },
		ExportLibrary: func(*ui.Context) { f.exported++ },
		OpenLicense:   func(*ui.Context) { f.licensed++ },
		OpenPage: func(_ *ui.Context, plugin, _ string) {
			f.openedPage = append(f.openedPage, plugin)
		},
	}
}

// page is a state for one page, with the values a first-run app would hand it.
func page(p settings.Page) *settings.State {
	return &settings.State{
		Page:               p,
		HasLibrary:         true,
		Appearance:         appsettings.AppearanceSystem,
		PaperWidth:         appsettings.DefaultPaperWidth,
		RetentionPercent:   90,
		MaxIntervalDays:    180,
		IntervalPreview:    true,
		ReturnAfterExcerpt: true,
		BudgetCards:        20,
		BudgetMinutes:      30,
		Hotkey:             appsettings.DefaultHotkey,
		Version:            "0.1.0",
		FormatVersion:      "remit/3",
		AppName:            "Remit",
		LibraryDir:         "/home/ada/Remit",
		Notes:              12,
		Cards:              34,
		DueNow:             5,
		Agent:              settings.AgentStatus{Address: "127.0.0.1:51234", TokenPath: "/data/agent-token"},
		Pages: []settings.PluginPage{
			{Plugin: "acme.report", ID: "leech", Title: "Leech report"},
		},
	}
}

// at draws one page and lets it settle, with the fake app's writes landing in
// the state the next frame reads.
func at(t *testing.T, s *settings.State, f *fake, w, h int) *ui.Tester {
	t.Helper()
	if f.state == nil {
		f.state = s
	}
	tt := ui.NewTester(func(c *ui.Context) { settings.View(c, s, f.deps()) }, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// Each of the eight pages draws its own title and its own sentence, and the
// page is the only one up: the pages are one view, not one long form.
func TestSettingsPagesAreSeparate(t *testing.T) {
	msgs := locales.In("en")
	pages := []struct {
		page  settings.Page
		title string
		lead  string
	}{
		{settings.PageAppearance, msgs.AppearancePage(), msgs.SettingsAppearanceLead()},
		{settings.PageReview, msgs.ReviewPage(), msgs.SettingsReviewLead()},
		{settings.PageBudget, msgs.BudgetPage(), msgs.SettingsBudgetLead()},
		{settings.PageKeys, msgs.KeysPage(), msgs.SettingsKeysLead()},
		{settings.PagePlugins, msgs.SettingsPlugins(), msgs.SettingsPluginsBody()},
		{settings.PageAgent, msgs.AgentPage(), msgs.SettingsAgentBody()},
		{settings.PageUpdate, msgs.UpdatePage(), msgs.SettingsUpdatesBody()},
		{settings.PageAbout, msgs.AboutPage(), msgs.SettingsAboutLead()},
	}
	for _, size := range sizes {
		for _, p := range pages {
			t.Run(size.name+"/"+string(p.page), func(t *testing.T) {
				s := page(p.page)
				tt := at(t, s, &fake{}, size.w, size.h)
				if !tt.HasText(p.title) || !tt.HasText(p.lead) {
					t.Fatalf("the %s page is missing its own heading; texts %q", p.page, tt.Texts())
				}
				if tt.FramePending() {
					t.Error("the page still requests frames after it should settle")
				}
			})
		}
	}
}

// The appearance page is the look, the paper's measure and the desktop's text
// size: the first two are written, the third is read-only and says why, with the
// one door that reaches the setting that owns it.
func TestSettingsAppearanceWritesTheLookAndThePaper(t *testing.T) {
	msgs := locales.In("en")
	s := page(settings.PageAppearance)
	f := &fake{}
	tt := at(t, s, f, 1024, 720)

	// The look: the ink segment is one click, and the choice is the write.
	reveal(t, tt, 1024, 720, msgs.SettingsLookInk())
	if err := tt.Click(msgs.SettingsLookInk()); err != nil {
		t.Fatal(err)
	}
	if len(f.look) != 1 || f.look[0] != appsettings.AppearanceInk {
		t.Fatalf("the look reached the app as %v, want ink", f.look)
	}

	// The paper's measure steps by 20 from where the knob is.
	stepSlider(t, tt, msgs.SettingsPaperWidth(),
		appsettings.DefaultPaperWidth, appsettings.PaperWidthMin, appsettings.PaperWidthMax, 1)
	if len(f.paperWidth) != 1 || f.paperWidth[0] != appsettings.DefaultPaperWidth+appsettings.PaperWidthStep {
		t.Fatalf("the paper reached the app as %v, want one step wider", f.paperWidth)
	}

	// The text size is read-only, with the reason on the page and the door to
	// the desktop's own setting.
	reveal(t, tt, 1024, 720, msgs.SettingsTextSize())
	if err := tt.Click(msgs.SettingsOpenDisplay()); err != nil {
		t.Fatal(err)
	}
	if f.system != 1 {
		t.Fatalf("the system settings door opened %d times, want once", f.system)
	}
	if !tt.HasText(msgs.SettingsTextSizeBody()) {
		t.Fatalf("the page does not say why the size is read-only; texts %q", tt.Texts())
	}
}

// The review page writes the library's presets: the retention, the longest
// interval, whether a review shows the next one, and whether an excerpt comes
// back to the note.
func TestSettingsReviewWritesThePresets(t *testing.T) {
	msgs := locales.In("en")
	s := page(settings.PageReview)
	f := &fake{}
	tt := at(t, s, f, 1024, 720)

	stepSlider(t, tt, msgs.SettingsReviewRetention(), 90, 80, 97, 1)
	if len(f.retention) != 1 || f.retention[0] != 91 {
		t.Fatalf("the retention reached the app as %v, want 91", f.retention)
	}

	stepSlider(t, tt, msgs.SettingsReviewMaxInterval(), 180, 30, 365, 1)
	if len(f.interval) != 1 || f.interval[0] != 210 {
		t.Fatalf("the longest interval reached the app as %v, want 210", f.interval)
	}

	reveal(t, tt, 1024, 720, msgs.SettingsReviewPreview())
	if err := tt.Click(msgs.SettingsReviewPreview()); err != nil {
		t.Fatal(err)
	}
	if len(f.preview) != 1 || f.preview[0] {
		t.Fatalf("the interval preview reached the app as %v, want one off", f.preview)
	}

	reveal(t, tt, 1024, 720, msgs.SettingsReviewReturn())
	if err := tt.Click(msgs.SettingsReviewReturn()); err != nil {
		t.Fatal(err)
	}
	if len(f.returned) != 1 || f.returned[0] {
		t.Fatalf("return-after-excerpt reached the app as %v, want one off", f.returned)
	}
}

// The budget page draws today's queue at the presets' pace, the reminder, and
// the tray title as the tray would set it: the two numbers of the day and the
// lines it holds, from the state the app derived.
func TestSettingsBudgetShowsTheDayItWouldBuild(t *testing.T) {
	msgs := locales.In("en")
	s := page(settings.PageBudget)
	s.Budget = settings.BudgetPreview{Read: 18, Review: 12, Total: 30, Budget: 30, Lines: 4, Cards: 20}
	s.TrayCards, s.TrayReading = 20, 3
	tt := at(t, s, &fake{}, 1024, 720)

	for _, want := range []string{
		msgs.SettingsBudgetPreviewBody(18, 12, 30, 4),
		msgs.TrayTitleReading(20, 3),
		msgs.SettingsReminderLabel(),
	} {
		reveal(t, tt, 1024, 720, want)
		if !tt.HasText(want) {
			t.Fatalf("the budget page is missing %q; texts %q", want, tt.Texts())
		}
	}

	// The two sliders write the day's caps together.
	f := &fake{state: s}
	tt = at(t, s, f, 1024, 720)
	stepSlider(t, tt, msgs.SettingsBudgetCards(), 20, 5, 60, 1)
	if len(f.budget) != 1 || f.budget[0] != (budgetChange{25, 30}) {
		t.Fatalf("the cards cap reached the app as %v, want 25 of 30 minutes", f.budget)
	}

	// The reminder is the same switch the tray's pause item flips.
	reveal(t, tt, 1024, 720, msgs.SettingsReminderLabel())
	if err := tt.Click(msgs.SettingsReminderLabel()); err != nil {
		t.Fatal(err)
	}
	if len(f.reminder) != 1 || !f.reminder[0] {
		t.Fatalf("the reminder reached the app as %v, want one on", f.reminder)
	}
}

// The keyboard map is read-only, and says so where the change would be made:
// the button that would rebind keys is disabled, and the sentence beside it
// names the issue. The one key the user owns — the global review shortcut —
// stays a field, and applying it reaches the app.
func TestSettingsKeysAreReadOnlyWithAReason(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			s := page(settings.PageKeys)
			f := &fake{}
			tt := at(t, s, f, size.w, size.h)

			reveal(t, tt, size.w, size.h, msgs.SettingsKeysChange())
			info, ok := tt.Access(msgs.SettingsKeysChange())
			if !ok {
				t.Fatalf("the page does not draw the change-keys button; texts %q", tt.Texts())
			}
			if !info.Disabled {
				t.Error("the change-keys button is enabled with no key map to edit")
			}
			if !tt.HasText(msgs.SettingsKeysLocked()) {
				t.Fatalf("the map does not say why it cannot be changed; texts %q", tt.Texts())
			}

			// Every group and every row of the map is on the page, with its keys.
			for _, group := range []string{
				msgs.SettingsKeysGlobal(), msgs.SettingsKeysReading(),
				msgs.SettingsKeysReview(), msgs.SettingsKeysDocument(),
				msgs.SettingsKeysPalette(), msgs.SettingsKeysGrade(),
				msgs.SettingsKeysOutline(), msgs.SettingsKeysBacklinks(),
			} {
				reveal(t, tt, size.w, size.h, group)
				if !tt.HasText(group) {
					t.Fatalf("the keyboard map is missing %q; texts %q", group, tt.Texts())
				}
			}

			// The global accelerator is the one editable key.
			reveal(t, tt, size.w, size.h, msgs.SettingsShortcutLabel())
			if err := tt.Click(msgs.SettingsShortcutLabel()); err != nil {
				t.Fatal(err)
			}
			tt.Command("selectAll")
			tt.Type("CmdOrCtrl+Shift+J")
			tt.Frame()
			if err := tt.Click(msgs.ApplyShortcut()); err != nil {
				t.Fatal(err)
			}
			if len(f.hotkey) != 1 || f.hotkey[0] != "CmdOrCtrl+Shift+J" {
				t.Fatalf("Apply reached the app as %v, want the accelerator typed", f.hotkey)
			}
			tt.Advance(time.Second)
			if tt.FramePending() {
				t.Error("the page still requests frames after it should settle")
			}
		})
	}
}

// The agent page states where the API listens, shows a token once, and lists
// the routes an agent can reach: the table is the page's promise of what the
// API does.
func TestSettingsAgentPageStatesTheSurface(t *testing.T) {
	msgs := locales.In("en")
	s := page(settings.PageAgent)
	s.Token = "0123456789abcdef"
	tt := at(t, s, &fake{}, 1024, 720)

	for _, want := range []string{
		msgs.SettingsAgentAddress("127.0.0.1:51234"),
		msgs.SettingsTokenPath("/data/agent-token"),
		s.Token,
		msgs.SettingsTokenOnce(),
		msgs.SettingsAgentRoutesBody(),
		msgs.SettingsRouteSchema(),
		msgs.SettingsRouteReview(),
		"/v1/cards/{id}/reviews",
	} {
		reveal(t, tt, 1024, 720, want)
		if !tt.HasText(want) {
			t.Fatalf("the agent page is missing %q; texts %q", want, tt.Texts())
		}
	}

	// The token is gone the moment the app forgets it: one showing, not a
	// second copy of a secret.
	s.Token = ""
	tt.Frame()
	if tt.HasText("0123456789abcdef") {
		t.Error("the page kept showing a token the app no longer hands it")
	}
}

// The update page states the two versions, and its two library actions say to
// open a library rather than pretending to rebuild or export nothing.
func TestSettingsUpdatePageWithoutALibrary(t *testing.T) {
	msgs := locales.In("en")
	s := page(settings.PageUpdate)
	s.HasLibrary = false
	f := &fake{}
	tt := at(t, s, f, 1024, 720)

	for _, want := range []string{
		msgs.SettingsVersion("0.1.0"), msgs.SettingsFormatVersion("remit/3"),
		msgs.SettingsUpdateNeedsLibrary(), msgs.SettingsReindexBody(),
	} {
		reveal(t, tt, 1024, 720, want)
		if !tt.HasText(want) {
			t.Fatalf("the update page is missing %q; texts %q", want, tt.Texts())
		}
	}
	// Both actions are refused with the reason in sight: a click on one runs
	// nothing, so the page cannot rebuild or export a library it does not have.
	reveal(t, tt, 1024, 720, msgs.SettingsReindex())
	if err := tt.Click(msgs.SettingsReindex()); err != nil {
		t.Fatal(err)
	}
	reveal(t, tt, 1024, 720, msgs.SettingsExport())
	if err := tt.Click(msgs.SettingsExport()); err != nil {
		t.Fatal(err)
	}
	if f.reindexed != 0 || f.exported != 0 {
		t.Errorf("a refused action ran: reindexed %d, exported %d", f.reindexed, f.exported)
	}
}

// The about page names the build and the library: its version, the format it
// writes, the plaintext directory, what the library holds, and the license.
func TestSettingsAboutPageStatesTheBuild(t *testing.T) {
	msgs := locales.In("en")
	s := page(settings.PageAbout)
	f := &fake{}
	tt := at(t, s, f, 1024, 720)

	for _, want := range []string{
		"Remit", msgs.SettingsVersion("0.1.0"), msgs.SettingsAboutTagline(),
		"/home/ada/Remit", msgs.SettingsAboutNotes(), "12", msgs.SettingsAboutCards(), "34",
		msgs.SettingsAboutQueue(), "5", msgs.SettingsCreditsBody(),
	} {
		reveal(t, tt, 1024, 720, want)
		if !tt.HasText(want) {
			t.Fatalf("the about page is missing %q; texts %q", want, tt.Texts())
		}
	}

	reveal(t, tt, 1024, 720, msgs.LicenseLink())
	if err := tt.Click(msgs.LicenseLink()); err != nil {
		t.Fatal(err)
	}
	if f.licensed != 1 {
		t.Fatalf("the license opened %d times, want once", f.licensed)
	}
}

// Every page holds at the smallest window in Chinese: each one keeps its own
// heading and its own first field inside the window.
func TestSettingsPagesAtMinimumInChinese(t *testing.T) {
	msgs := locales.In("zh-Hans")
	pages := []struct {
		page  settings.Page
		title string
		field string
	}{
		{settings.PageAppearance, msgs.AppearancePage(), msgs.SettingsLook()},
		{settings.PageReview, msgs.ReviewPage(), msgs.SettingsReviewRetention()},
		{settings.PageBudget, msgs.BudgetPage(), msgs.SettingsBudgetCards()},
		{settings.PageKeys, msgs.KeysPage(), msgs.SettingsKeysChange()},
		{settings.PagePlugins, msgs.SettingsPlugins(), msgs.SettingsPluginsBody()},
		{settings.PageAgent, msgs.AgentPage(), msgs.SettingsAgentServe()},
		{settings.PageUpdate, msgs.UpdatePage(), msgs.SettingsCheckAtLaunch()},
		{settings.PageAbout, msgs.AboutPage(), msgs.SettingsAboutNotes()},
	}
	for _, p := range pages {
		t.Run(string(p.page), func(t *testing.T) {
			s := page(p.page)
			tt := ui.NewTester(func(c *ui.Context) { settings.View(c, s, (&fake{state: s}).deps()) }, 720, 520)
			tt.SetLocale("zh-Hans")
			tt.Frame()

			for _, want := range []string{p.title, p.field} {
				reveal(t, tt, 720, 520, want)
				if !tt.HasText(want) {
					t.Fatalf("the page is missing %q in Chinese; texts %q", want, tt.Texts())
				}
			}
			if tt.HasText(locales.In("en").SettingsLook()) && p.page == settings.PageAppearance {
				t.Error("the page kept its English words")
			}
			for _, text := range tt.Texts() {
				if strings.HasPrefix(text, "settings") {
					t.Fatalf("a catalog key reached the page: %q", text)
				}
			}
			tt.Advance(time.Second)
			if tt.FramePending() {
				t.Error("the page still requests frames after the locale switch")
			}
		})
	}
}

// reveal scrolls the page until s has a box inside the window, which is what
// makes a control below or above the fold visible and clickable.
func reveal(t *testing.T, tt *ui.Tester, w, h int, s string) {
	t.Helper()
	inside := func(r ui.Rect) bool {
		return r.W > 0 && r.H > 0 && r.X >= 0 && r.Y >= 0 &&
			r.X+r.W <= float32(w) && r.Y+r.H <= float32(h)
	}
	// A box the scroll has culled reads as zero-sized, so the target's side of
	// the window cannot be told from it: go back to the top first, then down.
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
