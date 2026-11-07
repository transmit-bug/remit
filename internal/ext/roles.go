package ext

import "github.com/transmit-bug/remit/internal/core"

// roleHooks is the hook a plugin must implement to play each role. A role is a
// job with one plugin per library (ADR 0007), and the manifest's implements is
// where a plugin says it can do it — before any code runs.
var roleHooks = map[string]string{
	core.RoleCardsDerive:    "deriveBlock",
	core.RoleReviewSchedule: "policy",
}

// Plays reports whether e implements the hook role asks for. A role the host
// does not know is played by nobody.
func Plays(e Extension, role string) bool {
	hook, ok := roleHooks[role]
	if !ok {
		return false
	}
	_, ok = e.Manifest.Implements[hook]
	return ok
}

// ForRole answers the first extension that plays role, in the order given: the
// app resolves a library's role to the plugin its remit.json names.
func ForRole(exts []Extension, role string) (Extension, bool) {
	for _, e := range exts {
		if Plays(e, role) {
			return e, true
		}
	}
	return Extension{}, false
}
