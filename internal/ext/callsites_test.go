package ext_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext"
)

// A hook is named in one place and one only: this package's call table. Every
// caller above asks a capability — ext.HooksOf(runner).DeriveBlock, .Policy,
// .ImportFile — and the fixtures harness hands a case to ext.CheckFixture, which
// resolves the name against the same table and answers a verdict.
//
// The regression this guards is the one P2 was cut for: a business call site that
// spells a hook again, so one hook's shapes live in five packages and a rename
// crosses all of them (docs/specs/plugin-system.md, P2 ②). Reading the tree is the
// only way to see it — a call site compiles perfectly well against a by-name door.
//
// Tests are out of scope. A test may stand in for a substrate and answer a hook's
// wire directly — including this package's own tests, which do exactly that — and
// a test that names a hook is checking the seam, not reaching past it.
func TestNoBusinessCodeCallsAHookByString(t *testing.T) {
	root := filepath.Join("..", "..")
	seam := filepath.Join(root, "internal", "ext") + string(filepath.Separator)

	// What a call site would write: a by-name call, the fixture door an app has no
	// business opening, or the old spelling kept as a tripwire for its return.
	named := []string{".Call(", "ext.Invoke(", "ext.CheckFixture(", "ext.Fixture{"}
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

// The seam names a plugin and closes it, and that is all: no exported method of
// either runner is a by-name call, and Hook itself carries no such method.
//
// This is criterion 1 of #270 read off the exported surface rather than off a
// call site — restoring Call(method string, params, out any) on Hook or on either
// runner is red here, before any code could reach for it. The two substrates'
// own doors are unexported (JSRunner.call, Runner.call), and the fixture door
// answers a verdict, not a hook's answer (ext.CheckFixture).
func TestTheSeamCarriesNoExportedByNamedCall(t *testing.T) {
	hookType := reflect.TypeFor[ext.Hook]()
	var methods []string
	for i := range hookType.NumMethod() {
		method := hookType.Method(i)
		if callsAHook(method.Type) {
			t.Errorf("ext.Hook.%s is a by-name call: what an extension can do leaves the seam as a capability", method.Name)
		}
		methods = append(methods, method.Name)
	}
	if !slices.Equal(methods, []string{"Close", "Name"}) {
		t.Errorf("ext.Hook's methods are %v, want the handle's two: Name and Close", methods)
	}

	for _, runner := range []reflect.Type{
		reflect.TypeFor[*ext.JSRunner](),
		reflect.TypeFor[*ext.Runner](),
	} {
		for i := range runner.NumMethod() {
			method := runner.Method(i)
			if callsAHook(method.Type) {
				t.Errorf("%s.%s is a by-name call: a caller above takes a capability, and the wire stays in this package",
					runner.Elem().Name(), method.Name)
			}
		}
	}
}

// callsAHook reports a method shaped like a call by name: a hook's id and the
// wire's parameters in, an error out. Describe is not one — it is a typed method
// for a hook with no capability and no parameters of its own.
func callsAHook(sig reflect.Type) bool {
	if sig.NumOut() != 1 || sig.Out(0) != reflect.TypeFor[error]() {
		return false
	}
	if sig.NumIn() != 4 { // receiver, a hook's id, its parameters, the out
		return false
	}
	return sig.In(1).Kind() == reflect.String &&
		sig.In(2).Kind() == reflect.Interface &&
		sig.In(3).Kind() == reflect.Interface
}
