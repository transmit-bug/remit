package ts_test

import (
	"os"
	"path/filepath"
	"strconv"
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

// The artifact's context goes key by key as the host's table does: the type a
// key answers is the type the table names, and an enum's own type lists the
// table's values in its order. A key typed `string | boolean` — or as a
// vocabulary the host does not read — is a plugin author's type error, caught
// here rather than in an editor.
func TestTheArtifactTypesEveryContextKey(t *testing.T) {
	generated := string(ts.Render())
	for _, entry := range contrib.ContextTable {
		if line := `"` + entry.Key + `": ` + entry.Type + `;`; !strings.Contains(generated, line) {
			t.Errorf("context key %q is not %s in the rendered contract", entry.Key, line)
		}
		if entry.Kind != contrib.ContextEnum {
			continue
		}
		values := make([]string, len(entry.Values))
		for i, value := range entry.Values {
			values[i] = strconv.Quote(value)
		}
		if line := "export type " + entry.Type + " = " + strings.Join(values, " | ") + ";"; !strings.Contains(generated, line) {
			t.Errorf("context key %q does not render its values: %s", entry.Key, line)
		}
	}
}
