// Package start is the window before a library is open: the three ways in — a
// new library, one already on disk, and an import — the libraries the app
// opened before, and why one of them will not open. It reads no store and no
// settings — the app names the place, keeps the list and owns every action — so
// nothing is created until the user says so.
package start

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// State is what the screen shows: the libraries opened before, and the one that
// would not open. The folder a new library would go in is the app's answer, so
// it is asked rather than stored: a frame draws the place the app names.
type State struct {
	// Recent are the libraries the app opened before, newest first. The screen
	// draws the section only when there is one.
	Recent []Recent
	// Failed is the library the user just asked for and why it did not open.
	// It is a callout inside the card, not a page of its own: the three doors
	// stay where they were.
	Failed *Failure

	// list is the recent list's own state: the row chosen and its scroll.
	list ui.ListState
}

// Recent is one library the app opened before: where it is, and whether the
// folder is still there. A row whose folder is gone still opens, so a disk that
// comes back brings the library with it.
type Recent struct {
	Dir string
	// Missing says the folder is not on disk any more.
	Missing bool
}

// Failure is the library that would not open: where it is, and what the store
// said.
type Failure struct {
	Dir string
	Err string
}

// Deps is what the screen asks of the app. Every action runs on the UI thread;
// writing the library, the system's dialog and the import stay in the app, so
// the screen only reports that the user clicked.
type Deps struct {
	// Dir is the place a library would go, asked when State names none.
	Dir func() string
	// Create makes a library where the screen names it, opens it and remembers
	// it for the next start.
	Create func(*ui.Context)
	// Choose asks the system for a folder and opens the library in it, making
	// one when the folder holds none.
	Choose func(*ui.Context)
	// Import brings a `.remitlib` archive or a folder of Markdown in: the app
	// makes the library that receives it, then opens the wizard.
	Import func(*ui.Context)
	// OpenAt opens the library at a remembered path, which is a recent row.
	OpenAt func(*ui.Context, string)
}

// View draws the screen: the three ways to begin, the libraries opened before,
// and what a library that would not open said.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())

	dir := ""
	if d.Dir != nil {
		dir = d.Dir()
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.WelcomeWidth).Children(func() {
				card := kit.Card(c, func() {
					ui.Column(c).Gap(tk.SpaceL).Padding(tk.SpaceXL).Children(func() {
						brand(c, tk, msgs)
						ui.Column(c).Gap(tk.SpaceXS).Children(func() {
							ui.Text(c, msgs.StartTitle()).FontSize(tk.TypeH1).Bold()
							ui.Text(c, msgs.StartBody()).
								FontSize(tk.TypeUI).TextColor(tk.TextMuted)
							if dir != "" {
								ui.Text(c, dir).FontSize(tk.TypeMono).TextColor(tk.TextFaint).
									SingleLine()
							}
						})
						if s.Failed != nil {
							kit.Notice(c, "circle-alert",
								msgs.StartFailed(s.Failed.Dir, s.Failed.Err), tk.Danger, func() {
									ui.Button(c, msgs.StartChooseAnother()).MinHeight(tk.Control).
										OnClick(func() {
											if d.Choose != nil {
												d.Choose(c)
											}
										})
								})
						}
						doors(c, tk, msgs, d)
						if len(s.Recent) > 0 {
							ui.Divider(c)
							recent(c, s, tk, msgs, d)
						}
						ui.Text(c, msgs.StartTelemetry()).
							FontSize(tk.TypeCaption).TextColor(tk.TextFaint)
					})
				})
				card.FillWidth()
			})
		})
	})
}

// brand is what the window is, above what it asks: the mark the rail carries,
// the name, and the one line that says what a library is.
func brand(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceM).Children(func() {
		ui.Box(c).Size(tk.SpaceXXL+tk.SpaceS, tk.SpaceXXL+tk.SpaceS).
			Radius(tk.SpaceM).Background(tk.Sunken).Center().Children(func() {
			tk.Icon(c, "book-open", design.IconMedium).TextColor(tk.TextMuted)
		})
		ui.Column(c).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, msgs.StartBrand()).FontSize(tk.TypeH2).Bold()
			ui.Text(c, msgs.StartTagline()).
				FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		})
	})
}

// doors are the three ways in, each one button: a name, the line that says
// where it leads, and the icon of what it does. One click is one decision, so
// nothing here is a menu of further questions.
func doors(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		door(c, tk, "door-create", "plus", msgs.CreateLibrary(), msgs.StartCreateHint(), func() {
			if d.Create != nil {
				d.Create(c)
			}
		})
		door(c, tk, "door-open", "book-open", msgs.OpenLibrary(), msgs.StartOpenHint(), func() {
			if d.Choose != nil {
				d.Choose(c)
			}
		})
		door(c, tk, "door-import", "inbox", msgs.StartImport(), msgs.StartImportHint(), func() {
			if d.Import != nil {
				d.Import(c)
			}
		})
	})
}

// door is one way into the library. It is a button rather than a box with a
// click, so the pointer, the tab key and a screen reader reach the same place.
func door(c *ui.Context, tk design.Tokens, key, icon, title, hint string, run func()) {
	face := ui.ButtonBase(c.Key(key)).FillWidth().MinWidth(0).
		Justify(ui.Start).AlignItems(ui.Center).Gap(tk.SpaceM).
		Padding(tk.SpaceM, tk.SpaceM).MinHeight(tk.Door).
		Radius(tk.RadiusControl).Border(tk.Hairline, tk.Border).Label(title)
	face.OnClick(run)
	face.Children(func() {
		tk.Icon(c, icon, design.IconMedium).TextColor(tk.TextMuted)
		ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Start).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, title).FontSize(tk.TypeUI).FontWeight(600)
			ui.Text(c, hint).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
		})
	})
}

// recent is the libraries the app opened before: a row each, the path it lives
// at, and the reason a folder that is gone will not open.
func recent(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, d Deps) {
	s.list.Key = func(row int) any { return s.Recent[row].Dir }
	s.list.Label = func(row int) string { return s.Recent[row].Dir }
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, msgs.StartRecent()).FontSize(tk.TypeLabel).FontWeight(600).
			TextColor(tk.TextMuted)
		ui.List(c, &s.list, len(s.Recent), func(row int) {
			recentRow(c, tk, msgs, s.Recent[row], d)
		}).MaxHeight(tk.Door * float32(len(s.Recent))).Label(msgs.StartRecent())
	})
}

// recentRow is one library opened before: its path, the reason when the folder
// is gone, and the click that opens it again.
func recentRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, item Recent, d Deps) {
	face := ui.ButtonBase(c).FillWidth().MinWidth(0).
		Justify(ui.Start).AlignItems(ui.Center).Gap(tk.SpaceS).
		Padding(tk.SpaceS, tk.SpaceS).MinHeight(tk.Hit).
		Radius(tk.RadiusControl).Label(msgs.StartRecentOpen(item.Dir))
	face.OnClick(func() {
		if d.OpenAt != nil {
			d.OpenAt(c, item.Dir)
		}
	})
	face.Children(func() {
		tk.Icon(c, "clock", design.IconSmall).TextColor(tk.TextFaint)
		ui.Text(c, item.Dir).FontSize(tk.TypeUI).TextColor(tk.Text).SingleLine().Grow(1)
		if item.Missing {
			// The reason is the row's own: the folder is not there, so opening
			// it will fail, and the click still says so where it happened.
			ui.Badge(c, msgs.StartRecentMissing()).
				Background(tk.Danger.Alpha(0.12)).TextColor(tk.Danger.Mix(tk.Text, 0.35))
		}
	})
}
