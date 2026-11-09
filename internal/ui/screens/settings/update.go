package settings

import (
	"context"
	"errors"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	appsettings "github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/update"
	"github.com/transmit-bug/remit/locales"
)

// ErrCannotUpdate reports a build without update keys: MyGo's updater is
// disabled, so there is nothing to ask and nowhere to install from.
var ErrCannotUpdate = errors.New("this build cannot update itself")

// CheckForUpdate is the check the app runs: MyGo's, and a build that cannot
// update itself says so instead of reporting silence as being up to date.
func CheckForUpdate(ctx context.Context, s appsettings.Settings) (update.Result, error) {
	if !mygo.Updater.Enabled() {
		return update.Result{}, ErrCannotUpdate
	}
	return update.Check(ctx, s)
}

// updatePage is what this build is and what can be done to the library it
// opened: the two versions, the launch check, the index rebuild and the archive
// export. The last two work on a library, so with none open they say which one
// to open instead of offering a control that cannot act.
func updatePage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	var toggled, checked bool

	// The page's heading names the build, so its two versions are two lines in
	// the form rather than a group named after the page again.
	ui.Text(c, msgs.SettingsVersion(s.Version)).FontSize(tk.TypeUI).TextColor(tk.Text)
	ui.Text(c, msgs.SettingsFormatVersion(s.FormatVersion)).
		Font("monospace").FontSize(tk.TypeMono).TextColor(tk.TextMuted)

	ui.Fieldset(c, msgs.SettingsUpdates(), func() {
		ui.Field(c, msgs.SettingsCheckAtLaunch(), func() {
			toggled = ui.Switch(c, &s.UpdateCheck).Changed()
		}).Description(msgs.SettingsUpdatesBody())
		ui.Row(c).Gap(tk.SpaceM).AlignItems(ui.Center).Children(func() {
			checked = ui.Button(c, msgs.CheckNow()).MinHeight(tk.Control).Clicked()
			if s.UpdateMsg != "" {
				ui.Text(c, s.UpdateMsg).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			}
		})
	})

	ui.Fieldset(c, msgs.SettingsReindex(), func() {
		muted(c, tk, msgs.SettingsReindexBody())
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			ui.Button(c, msgs.SettingsReindex()).MinHeight(tk.Control).
				Disabled(!s.HasLibrary).
				OnClick(func() { d.reindex(c) })
			if !s.HasLibrary {
				muted(c, tk, msgs.SettingsUpdateNeedsLibrary())
			}
		})
	})

	ui.Fieldset(c, msgs.SettingsExport(), func() {
		muted(c, tk, msgs.SettingsExportBody())
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			ui.Button(c, msgs.SettingsExport()).MinHeight(tk.Control).
				Disabled(!s.HasLibrary).
				OnClick(func() { d.exportLibrary(c) })
			if !s.HasLibrary {
				muted(c, tk, msgs.SettingsUpdateNeedsLibrary())
			}
		})
	})

	if toggled {
		d.setUpdateCheck(c, s.UpdateCheck)
	}
	if checked {
		d.checkNow(c)
	}
}

func (d Deps) setUpdateCheck(c *ui.Context, on bool) {
	if d.SetUpdateCheck != nil {
		d.SetUpdateCheck(c, on)
	}
}

func (d Deps) checkNow(c *ui.Context) {
	if d.CheckNow != nil {
		d.CheckNow(c)
	}
}

func (d Deps) reindex(c *ui.Context) {
	if d.Reindex != nil {
		d.Reindex(c)
	}
}

func (d Deps) exportLibrary(c *ui.Context) {
	if d.ExportLibrary != nil {
		d.ExportLibrary(c)
	}
}
