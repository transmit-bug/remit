package ext_test

import (
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext"
)

// jsManifest is a well-formed js manifest; each case below breaks one thing.
func jsManifest() ext.Manifest {
	return ext.Manifest{
		ID:           "test.cards",
		Name:         "cards",
		Version:      "1.0.0",
		APIVersion:   1,
		Runtime:      string(ext.RuntimeJS),
		Entry:        "main.js",
		Capabilities: []string{},
		Implements:   map[string]int{"deriveBlock": 1},
	}
}

// Every way a js manifest can be wrong is named, so a plugin author is told
// which line to fix rather than that something is wrong.
func TestValidateJSManifestRefusals(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ext.Manifest)
		refusal string
	}{
		{
			name:    "no id",
			mutate:  func(m *ext.Manifest) { m.ID = "" },
			refusal: "needs an id",
		},
		{
			name:    "apiVersion below one",
			mutate:  func(m *ext.Manifest) { m.APIVersion = 0 },
			refusal: "apiVersion 0, this app speaks 1 through 1",
		},
		{
			name:    "apiVersion above the host",
			mutate:  func(m *ext.Manifest) { m.APIVersion = ext.HostAPIVersion + 1 },
			refusal: "apiVersion 2, this app speaks 1 through 1",
		},
		{
			name:    "still carries protocol",
			mutate:  func(m *ext.Manifest) { m.Protocol = 1 },
			refusal: "takes no protocol field",
		},
		{
			name:    "unknown hook",
			mutate:  func(m *ext.Manifest) { m.Implements["cards.cook"] = 1 },
			refusal: `implements "cards.cook": no such hook`,
		},
		{
			name:    "hook contract above the host's",
			mutate:  func(m *ext.Manifest) { m.Implements["deriveBlock"] = 2 },
			refusal: `implements "deriveBlock": version 2, this app speaks 1`,
		},
		{
			name:    "hook contract below one",
			mutate:  func(m *ext.Manifest) { m.Implements["deriveBlock"] = 0 },
			refusal: `implements "deriveBlock": version 0, contracts start at 1`,
		},
		{
			name:    "no entry",
			mutate:  func(m *ext.Manifest) { m.Entry = "" },
			refusal: "needs an entry",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := jsManifest()
			tc.mutate(&m)
			err := ext.Validate(m)
			if err == nil {
				t.Fatalf("a manifest that %s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.refusal) {
				t.Errorf("refused with %q, want it to name %q", err, tc.refusal)
			}
		})
	}
}

// Two wrong implements claims are both listed: one refusal, whole picture.
func TestValidateJSManifestListsEveryWrongClaim(t *testing.T) {
	m := jsManifest()
	m.Implements = map[string]int{"cards.cook": 1, "deriveBlock": 9}
	err := ext.Validate(m)
	if err == nil {
		t.Fatal("wrong implements claims were accepted")
	}
	for _, want := range []string{`implements "cards.cook": no such hook`, `implements "deriveBlock": version 9`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q misses %q", err, want)
		}
	}
}

// A manifest that only names what the host still speaks loads, and the
// vocabulary it is checked against is the one the host exports.
func TestValidateJSManifestAcceptsTheCurrentContract(t *testing.T) {
	m := jsManifest()
	if err := ext.Validate(m); err != nil {
		t.Fatalf("a well-formed js manifest was refused: %v", err)
	}
	if ext.Hooks["deriveBlock"] != 1 || ext.Hooks["on_event"] != 1 {
		t.Errorf("the hook vocabulary drifted: %+v", ext.Hooks)
	}
}

// An exec manifest keeps its own rules: the wire protocol, an entrypoint —
// and none of the js fields are demanded of it.
func TestValidateExecManifestRules(t *testing.T) {
	m := ext.Manifest{Name: "fake", Version: "1", Protocol: ext.Protocol, Entrypoint: []string{"fakeext"}}
	if err := ext.Validate(m); err != nil {
		t.Fatalf("a well-formed exec manifest was refused: %v", err)
	}
	m.Protocol = ext.Protocol + 1
	if err := ext.Validate(m); err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("a wrong wire protocol was accepted: %v", err)
	}
}
