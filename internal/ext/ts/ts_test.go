package ts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ext/ts"
)

// remit.d.ts is generated from the host's own tables. A hook, a capability, a
// slot, a context key or a node kind the code adds and the artifact does not
// (or the reverse) fails here, not in a plugin author's editor.
func TestRemitDtsMatchesTheHost(t *testing.T) {
	path := filepath.Join("..", "..", "..", ts.FileName)
	checked, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (regenerate: go run ./cmd/remit ext types -o %s)", ts.FileName, err, ts.FileName)
	}
	generated := string(ts.Render())
	if string(checked) == generated {
		return
	}

	have := strings.Split(string(checked), "\n")
	want := strings.Split(generated, "\n")
	for i := 0; i < len(have) && i < len(want); i++ {
		if have[i] != want[i] {
			t.Fatalf("%s does not match the code at line %d\nhave: %s\nwant: %s\n"+
				"regenerate: go run ./cmd/remit ext types -o %s",
				ts.FileName, i+1, have[i], want[i], ts.FileName)
		}
	}
	t.Fatalf("%s is %d lines, the code renders %d; regenerate: go run ./cmd/remit ext types -o %s",
		ts.FileName, len(have), len(want), ts.FileName)
}

// The artifact types a value for every context key the window exports: a key
// added to the contract without a type would otherwise read as a union of the
// two kinds, and a plugin author would have to guess.
func TestTheArtifactTypesEveryContextKey(t *testing.T) {
	generated := string(ts.Render())
	for _, key := range contrib.ContextKeys {
		if !strings.Contains(generated, `"`+key+`":`) {
			t.Errorf("context key %q is not in the rendered contract", key)
		}
	}
	if strings.Contains(generated, "string | boolean;") {
		t.Error("a context key has no type of its own in the rendered contract")
	}
}
