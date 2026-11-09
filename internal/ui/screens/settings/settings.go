// Package settings is the app's settings: one area with eight pages — the look
// and the paper, the review and the budget presets, the keyboard map, the
// plugins, the agent API, the version and the index, and what this build is.
//
// Every page shows the app's state as a copy and reports each change through
// Deps; the app owns the writes — settings.json for what follows the machine,
// the library's presets for what follows the material — and nothing in a frame
// reads the store. Each page is built from MyGo's own form widgets, so the
// labels, the keyboard, the focus order and what a screen reader hears come
// from the widget rather than from this package.
package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// Deps is what the pages ask of the app: one function per thing the user can
// do, each run on the UI thread. Every read answers a snapshot the app retook
// when the page opened or after the last write, so a frame never reaches the
// store. A nil function leaves its control out rather than panicking; the pages
// say when this build has no service behind one.
type Deps struct {
	// SetAppearance chooses the look, and SetPaperWidth the document paper's
	// width in DIPs. Both are the app's preferences, not the library's.
	SetAppearance func(*ui.Context, string)
	SetPaperWidth func(*ui.Context, int)
	// OpenDisplaySettings opens the desktop's own display or text-size page:
	// the size the window draws at is the desktop's, so its setting is there.
	OpenDisplaySettings func(*ui.Context)

	// The review presets, written into the library's config. Retention is a
	// percentage, the override of the scheduling plugin's target.
	SetRetention          func(*ui.Context, int)
	SetMaxInterval        func(*ui.Context, int)
	SetIntervalPreview    func(*ui.Context, bool)
	SetReturnAfterExcerpt func(*ui.Context, bool)
	// SetBudget writes the day's cards and minutes into the library's presets.
	SetBudget func(*ui.Context, int, int)
	// SetNotifications remembers whether the daily reminder is on; it is the
	// same setting the tray's pause item flips.
	SetNotifications func(*ui.Context, bool)

	// ApplyHotkey binds the global review accelerator and remembers it.
	ApplyHotkey func(*ui.Context, string)

	// Plugins is what discovery found, as the app last saw it: the built-ins
	// and what this user installed. An empty list is stated on the page as
	// such, never as an error.
	Plugins func() []Plugin
	// SetEnabled approves a plugin for this library, or withdraws it. It is
	// the grant: the capabilities the plugin declares apply from the next
	// library open.
	SetEnabled func(*ui.Context, string, bool)
	// Roles is the plugin each role names in this library, the built-in when
	// the library names none.
	Roles func() map[string]string
	// SetRole chooses the plugin for a role; "" returns to the built-in.
	SetRole func(*ui.Context, string, string)
	// OpenPage opens a page a plugin contributed, by its manifest name and the
	// page's id. The page is a navigation, so it closes the settings area.
	OpenPage func(*ui.Context, string, string)
	// SetPluginSetting writes one value into the plugin's own store, under its
	// id: the host owns the write, and the plugin reads the value itself.
	SetPluginSetting func(*ui.Context, string, string, string)
	// PluginMessage resolves one of a plugin's own messages in the frame's
	// language, so a page follows the window's locale rather than a snapshot.
	PluginMessage func(*ui.Context, string, string) string
	// Reenable gives a plugin's failed contributions another try: the counts go
	// back to zero and the hidden entries show again.
	Reenable func(*ui.Context, string)

	// SetAgent starts or stops the agent API now, and remembers the choice.
	SetAgent func(*ui.Context, bool)
	// RotateToken withdraws the agent API's token, so the old one stops working
	// at once; the page shows the new one once.
	RotateToken func(*ui.Context)

	// SetUpdateCheck remembers whether Remit checks for updates at launch, and
	// CheckNow asks whether a newer version exists.
	SetUpdateCheck func(*ui.Context, bool)
	CheckNow       func(*ui.Context)
	// Reindex derives the library's index from the plaintext again.
	Reindex func(*ui.Context)
	// ExportLibrary writes the library into a .remitlib archive the user
	// chooses.
	ExportLibrary func(*ui.Context)

	// OpenLicense opens the license the app is under.
	OpenLicense func(*ui.Context)
}

// View draws the page the state names: the look, the review and budget presets,
// the keyboard map, the plugins, the agent API, the version or this build.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	page := s.page()

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.SettingsWidth).Gap(tk.SpaceL).Children(func() {
				ui.Text(c, pageTitle(msgs, page)).FontSize(tk.TypeH2).Bold()
				ui.Text(c, pageLead(msgs, page)).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
				ui.Form(c, func() {
					switch page {
					case PageReview:
						reviewPage(c, s, d, msgs, tk)
					case PageBudget:
						budgetPage(c, s, d, msgs, tk)
					case PageKeys:
						keysPage(c, s, d, msgs, tk)
					case PagePlugins:
						pluginsPage(c, s, d, msgs, tk)
					case PageAgent:
						agentPage(c, s, d, msgs, tk)
					case PageUpdate:
						updatePage(c, s, d, msgs, tk)
					case PageAbout:
						aboutPage(c, s, d, msgs, tk)
					default:
						appearancePage(c, s, d, msgs, tk)
					}
				})
			})
		})
	})
	// The details are an overlay over the page, at the window's root: the one
	// that was opened last takes the keyboard, and Escape closes it (S47).
	if page == PagePlugins {
		pluginDetailView(c, s, d, msgs, tk)
	}
}

// pageTitle is one page's name, the word the sub-navigation shows and the
// heading the page repeats.
func pageTitle(msgs locales.Messages, page Page) string {
	switch page {
	case PageReview:
		return msgs.ReviewPage()
	case PageBudget:
		return msgs.BudgetPage()
	case PageKeys:
		return msgs.KeysPage()
	case PagePlugins:
		return msgs.SettingsPlugins()
	case PageAgent:
		return msgs.AgentPage()
	case PageUpdate:
		return msgs.UpdatePage()
	case PageAbout:
		return msgs.AboutPage()
	}
	return msgs.AppearancePage()
}

// pageLead is the one sentence under a page's title: what the page is for, not
// a restatement of its name.
func pageLead(msgs locales.Messages, page Page) string {
	switch page {
	case PageReview:
		return msgs.SettingsReviewLead()
	case PageBudget:
		return msgs.SettingsBudgetLead()
	case PageKeys:
		return msgs.SettingsKeysLead()
	case PagePlugins:
		return msgs.SettingsPluginsBody()
	case PageAgent:
		return msgs.SettingsAgentBody()
	case PageUpdate:
		return msgs.SettingsUpdatesBody()
	case PageAbout:
		return msgs.SettingsAboutLead()
	}
	return msgs.SettingsAppearanceLead()
}

// muted is one quiet line, each page's way of stating what is not there.
func muted(c *ui.Context, tk design.Tokens, text string) {
	ui.Text(c, text).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
}

// notice is a line that reports a refusal or a state the user must act on.
func notice(c *ui.Context, tk design.Tokens, text string) {
	if text == "" {
		return
	}
	ui.Text(c, text).FontSize(tk.TypeUI).TextColor(tk.Danger)
}
