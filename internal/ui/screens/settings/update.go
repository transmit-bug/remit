package settings

import (
	"context"
	"errors"

	"github.com/egoist/mygo"

	appsettings "github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/update"
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
