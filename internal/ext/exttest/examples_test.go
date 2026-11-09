package exttest_test

import (
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/ext/exttest"
)

// The plugins the guide walks a reader through really load and answer: each is
// run here through the same harness `remit ext test` is, against the fixtures it
// ships, so the examples and the guide cannot rot.
func TestTheExamplePluginsPass(t *testing.T) {
	for _, dir := range []string{"cards-mini", "leeches", "whiteboard"} {
		dir := filepath.Join("..", "..", "..", "examples", "plugins", dir)
		result := exttest.Run(dir, exttest.Options{})
		if len(result.Problems) > 0 {
			t.Errorf("%s:\n%s", dir, problems(result))
			continue
		}
		if result.Cases == 0 {
			t.Errorf("%s: the run compared nothing", dir)
		}
	}
}
