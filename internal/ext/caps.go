package ext

import "fmt"

// The capabilities an extension may declare in its manifest (ADR 0007). A
// capability is what the host offers, not a permission prompt: a name the host
// does not know is a manifest error, and a call to a host function whose
// capability the manifest left out is a CapabilityError.
const (
	// CapNotesRead lets an extension list, read and search notes.
	CapNotesRead = "notes:read"
	// CapCardsWrite lets it make, change and remove cards.
	CapCardsWrite = "cards:write"
	// CapReviewRead lets it read a card's review history and the queue's length.
	CapReviewRead = "review:read"
	// CapReviewWrite lets it record a review.
	CapReviewWrite = "review:write"
	// CapScheduleFSRS lets it advance a card's FSRS state.
	CapScheduleFSRS = "schedule:fsrs"
	// CapStoreOwn lets it keep a value of its own, under its own id.
	CapStoreOwn = "store:own"
)

// Capabilities are every name a manifest may declare, in the order the contract
// lists them: the six plugin capabilities, then the two host functions a plugin
// uses to test itself. This is the one list the host validates against and the
// one the contract artifact remit.d.ts is generated from.
var Capabilities = []string{
	CapNotesRead, CapCardsWrite, CapReviewRead, CapReviewWrite, CapScheduleFSRS, CapStoreOwn,
	CapEcho, CapNow,
}

// CapabilityError is a call to a remit.* function the extension did not
// declare. It names the extension and the capability, so the plugin log says
// which plugin asked for what.
type CapabilityError struct {
	Extension  string
	Capability string
}

// Error names the plugin id and the capability it did not declare.
func (e *CapabilityError) Error() string {
	return fmt.Sprintf("ext: %s did not declare capability %s", e.Extension, e.Capability)
}

// knownCapability reports whether a manifest may declare name. The six in
// caps.go are the plugin capabilities; remit.echo and remit.now are the host
// functions the app offers for a plugin's own tests (js.go).
func knownCapability(name string) bool {
	for _, known := range Capabilities {
		if known == name {
			return true
		}
	}
	return false
}
