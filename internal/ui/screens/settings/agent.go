package settings

// AgentStatus is what the agent API says about itself: where it listens, and
// the file that holds the bearer token. Both are the files cmd writes beside
// the token, so a client finds the API without a fixed port.
type AgentStatus struct {
	Address   string // host:port, empty while nothing is served
	TokenPath string // the file holding the token, empty when unknown
}

// AgentControls is what the settings page needs from whoever serves the agent
// API (cmd): start or stop the listener now, rotate the token, and say where
// it listens. It is an interface so that a test drives all three without
// binding a port. A nil AgentControls leaves the page saying so.
type AgentControls interface {
	// SetEnabled starts the listener, or stops it, and reports why it could
	// not. Starting one that already runs changes nothing.
	SetEnabled(on bool) error
	// RotateToken writes a fresh token and restarts the listener with it, so
	// the old token stops working at once.
	RotateToken() error
	// Status is where it listens and which file holds the token now.
	Status() AgentStatus
}
