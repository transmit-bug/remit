// Package update asks whether a newer Remit exists, and does nothing unless
// the user turned update checks on (settings.UpdateCheck). Signature
// verification is MyGo's own: it checks the Ed25519 signature of the archive
// and of every delta against the key the app was built with, so this package
// never trusts a manifest on its own.
package update

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/settings"
)

// Timeout bounds one check: a slow network must not keep the app waiting.
const Timeout = 15 * time.Second

// Result is what a check found.
type Result struct {
	Version string
	Notes   string
}

// Check looks for a newer stable release, but only when settings allow it: off
// means the app sends nothing at all. A development build, or one packaged
// without update keys, reports ErrUpdatesDisabled from MyGo; callers treat it
// as "no update" and say nothing.
func Check(ctx context.Context, s settings.Settings) (Result, error) {
	if !s.UpdateCheck {
		return Result{}, nil
	}
	return check(ctx)
}

// check is the seam a test replaces to prove the gate without a network.
var check = func(ctx context.Context) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	u, err := mygo.Updater.Check(ctx)
	if err != nil {
		if errors.Is(err, mygo.ErrUpdatesDisabled) {
			return Result{}, nil
		}
		return Result{}, fmt.Errorf("update: %w", err)
	}
	if u == nil {
		return Result{}, nil
	}
	return Result{Version: u.Version, Notes: u.Notes}, nil
}
