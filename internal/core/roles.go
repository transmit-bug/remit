package core

// The plugin roles a library chooses one plugin for (ADR 0007: roles, not
// ordering). A role names a job, never a plugin: the library's remit.json says
// which plugin plays it, and the built-in plays it while the library names
// none.
const (
	// RoleCardsDerive turns a block's text into typed cards.
	RoleCardsDerive = "cards.derive"
	// RoleReviewSchedule sets the scheduling policy and what a finished
	// session is worth saying.
	RoleReviewSchedule = "review.schedule"
)

// Roles lists every role, in the order the settings page shows them.
func Roles() []string {
	return []string{RoleCardsDerive, RoleReviewSchedule}
}
