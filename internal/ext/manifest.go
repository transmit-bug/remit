package ext

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// HostAPIVersion is the version of the host API (the JavaScript face) this
// build speaks. A manifest naming a lower apiVersion still loads: the host
// accepts by minimum-version semantics, one breaking release back.
const HostAPIVersion = 1

// Hooks is the host's hook vocabulary: every hook id an extension may put in
// its manifest's implements, with the contract version the host records for
// it. It is the single source the host checks against, before any code runs.
//
// Contract v2 (#140): derive became deriveBlock, whose answer is typed
// (typeId, ordinal, fields) rather than shaped sides, and faces/render are
// gone — card faces are rendered by the host template engine from the card
// type's own templates, not by a plugin.
var Hooks = map[string]int{
	"describe":    1,
	"deriveBlock": 1,
	"policy":      1,
	"identity":    1,
	"summary":     1,
	"on_event":    1,
}

// Manifest is extension.json: what the extension is, what it runs on, and what
// the host may offer it.
type Manifest struct {
	// ID is the extension's stable identity, minted once ("remit.cards.standard").
	ID string `json:"id"`
	// APIVersion is the host API the extension was written for. The host loads
	// anything from 1 up to HostAPIVersion.
	APIVersion int `json:"apiVersion"`
	// Implements names the hooks the extension carries, each with the contract
	// version it speaks. The host can show and index these without loading
	// the extension.
	Implements map[string]int `json:"implements"`
	// Activation is when the extension wants to be loaded. It decides when,
	// never who is chosen for a role.
	Activation []string `json:"activation"`
	// Produces are the card types a derive extension answers.
	Produces []string `json:"produces"`

	Name    string `json:"name"`
	Version string `json:"version"`
	// Protocol is the exec wire protocol version; only an exec manifest may
	// carry it. A js manifest that does is refused: the two versionings are
	// separate (js is versioned by apiVersion).
	Protocol int `json:"protocol"`
	// Runtime is "exec" (the default, a process speaking the protocol) or "js"
	// (JavaScript in this process).
	Runtime string `json:"runtime"`
	// Entrypoint starts an exec extension; Entry is a js extension's file, in
	// its own directory.
	Entrypoint []string `json:"entrypoint"`
	Entry      string   `json:"entry"`
	// Capabilities are what the host offers the extension, by name; an
	// extension gets nothing it does not declare.
	Capabilities []string `json:"capabilities"`
}

// Validate checks a manifest against what this build can run. The two runtimes
// are checked by their own rules: an exec extension speaks the wire protocol
// and names an entrypoint, a js extension carries an id, an apiVersion the host
// knows and implements entries the vocabulary holds.
func Validate(m Manifest) error {
	if m.Name == "" {
		return errors.New("manifest needs a name")
	}
	if Runtime(m.Runtime) == RuntimeJS {
		return validateJS(m)
	}
	return validateExec(m)
}

func validateExec(m Manifest) error {
	if len(m.Entrypoint) == 0 {
		return errors.New("manifest needs an entrypoint")
	}
	if m.Protocol != Protocol {
		return fmt.Errorf("protocol %d, this app speaks %d", m.Protocol, Protocol)
	}
	return nil
}

func validateJS(m Manifest) error {
	if m.Protocol != 0 {
		return fmt.Errorf("a js extension takes no protocol field (found %d); it is versioned by apiVersion", m.Protocol)
	}
	if m.ID == "" {
		return errors.New("a js extension needs an id")
	}
	if m.APIVersion < 1 || m.APIVersion > HostAPIVersion {
		return fmt.Errorf("apiVersion %d, this app speaks 1 through %d", m.APIVersion, HostAPIVersion)
	}
	if m.Entry == "" {
		return errors.New("a js extension needs an entry")
	}
	if filepath.IsAbs(m.Entry) || strings.Contains(m.Entry, "..") {
		return fmt.Errorf("entry %q is outside the extension", m.Entry)
	}
	return validateImplements(m)
}

// validateImplements checks what the extension claims to carry against the
// host's vocabulary. Every wrong claim is listed, so a plugin author sees the
// whole picture in one refusal.
func validateImplements(m Manifest) error {
	var problems []error
	for _, hook := range sortedKeys(m.Implements) {
		contract, ok := Hooks[hook]
		switch {
		case !ok:
			problems = append(problems, fmt.Errorf("implements %q: no such hook", hook))
		case m.Implements[hook] < 1:
			problems = append(problems, fmt.Errorf("implements %q: version %d, contracts start at 1", hook, m.Implements[hook]))
		case m.Implements[hook] > contract:
			problems = append(problems, fmt.Errorf("implements %q: version %d, this app speaks %d", hook, m.Implements[hook], contract))
		}
	}
	return errors.Join(problems...)
}

// sortedKeys reads a small map in a stable order, for stable refusals.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// insertion sort: the vocabulary is small, and so is a manifest
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
