package ext_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A hook is named in one place and one only: this package's table. Every caller
// above asks a capability — ext.HooksOf(runner).DeriveBlock, .Policy, .ImportFile
// — and the fixtures harness asks Invoke, which resolves the name against the same
// table.
//
// The regression this guards is the one P2 was cut for: a business call site that
// spells a hook again, so one hook's shapes live in five packages and a rename
// crosses all of them (docs/specs/plugin-system.md, P2). Reading the tree is the
// only way to see it — a call site compiles perfectly well against Call.
//
// Tests are out of scope. A test may stand in for a substrate and answer a hook's
// wire directly — including this package's own tests, which do exactly that — and
// a test that names a hook is checking the seam, not reaching past it.
func TestNoBusinessCodeCallsAHookByString(t *testing.T) {
	root := filepath.Join("..", "..")
	seam := filepath.Join(root, "internal", "ext") + string(filepath.Separator)

	// What a call site would write: the wire's own door, or the table's by name.
	named := []string{".Call(", "ext.Invoke("}
	var offenders []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.HasPrefix(path, seam) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, pattern := range named {
				if !strings.Contains(line, pattern) {
					continue
				}
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read the tree: %v", err)
	}
	for _, offender := range offenders {
		t.Errorf("%s\nwant no business code naming a hook: take the capability (ext.HooksOf) or the caller's typed method", offender)
	}
}
