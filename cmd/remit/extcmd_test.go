package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext/ts"
)

// extPlugin writes a plugin directory for the ext commands to read: the
// manifest, the entry, and any fixture file by its path under fixtures/.
func extPlugin(t *testing.T, manifest string, fixtures map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	source := `export function deriveBlock(block) {
  return [{ typeId: "remit.basic", ordinal: 0, fields: { Front: (block && block.text) || "" } }];
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(source), 0o644); err != nil {
		t.Fatalf("entry: %v", err)
	}
	for name, body := range fixtures {
		path := filepath.Join(dir, "fixtures", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("fixtures: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("fixture %s: %v", name, err)
		}
	}
	return dir
}

const extManifest = `{
  "id": "acme.ext",
  "name": "acme",
  "version": "0.1.0",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js",
  "implements": { "deriveBlock": 1 }
}`

// ext writes the fixture the manifest's one hook is compared against.
var extFixtures = map[string]string{
	"deriveBlock/answer.in.json":  `{"args":[{"text":"a paragraph"}]}`,
	"deriveBlock/answer.out.json": `[{"fields":{"Front":"a paragraph"},"ordinal":0,"typeId":"remit.basic"}]`,
}

// run is one command line as the binary runs it: the output, and the exit code.
func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code, handled := runExt(args, &stdout, &stderr)
	if !handled {
		t.Fatalf("remit %v was not an ext command", args)
	}
	return code, stdout.String(), stderr.String()
}

// A plugin whose fixtures its hooks answer is one this build can run: exit 0,
// and the run says how many answers it compared.
func TestExtTestAcceptsAPluginItsFixturesHold(t *testing.T) {
	dir := extPlugin(t, extManifest, extFixtures)
	code, stdout, stderr := run(t, "ext", "test", dir)
	if code != extOK {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, dir+": ok (1 cases: deriveBlock)") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

// A manifest the loader refuses is a problem the author has to fix: exit 1, the
// loader's reason on stdout, and no hook run.
func TestExtTestRefusesAnInvalidManifest(t *testing.T) {
	manifest := strings.Replace(extManifest, `"name": "acme",`, "", 1)
	dir := extPlugin(t, manifest, extFixtures)
	code, stdout, _ := run(t, "ext", "test", dir)
	if code != extProblems {
		t.Fatalf("exit %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "manifest needs a name") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

// What contributes declares is held to the host's schema: an icon the window
// does not have is one problem, named by the same parse the window runs.
func TestExtTestRefusesAnInvalidContribution(t *testing.T) {
	manifest := strings.Replace(extManifest, `"implements": { "deriveBlock": 1 }`,
		`"implements": { "deriveBlock": 1 },
  "contributes": { "commands": [{ "id": "acme.ext.run", "title": "%run%", "icon": "no-such-icon", "action": "command:run" }], "locales": { "en": { "run": "Run it" } } }`, 1)
	dir := extPlugin(t, manifest, extFixtures)
	code, stdout, _ := run(t, "ext", "test", dir)
	if code != extProblems {
		t.Fatalf("exit %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, `icon "no-such-icon": the host has no such icon`) {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

// A plugin whose hook answers something else names the fixture and both
// answers, which is what a plugin author pastes back.
func TestExtTestReportsAMismatch(t *testing.T) {
	dir := extPlugin(t, extManifest, map[string]string{
		"deriveBlock/answer.out.json": `[]`,
	})
	code, stdout, _ := run(t, "ext", "test", dir)
	if code != extProblems {
		t.Fatalf("exit %d\n%s", code, stdout)
	}
	for _, want := range []string{"fixtures/deriveBlock/answer", "answered", "want"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not name %q:\n%s", want, stdout)
		}
	}
}

// check answers the manifest and contributions alone, without running a hook:
// a plugin with no fixtures is still checkable.
func TestExtCheckReadsTheManifestOnly(t *testing.T) {
	dir := extPlugin(t, extManifest, nil)
	code, stdout, _ := run(t, "ext", "check", dir)
	if code != extOK || !strings.Contains(stdout, dir+": ok") {
		t.Fatalf("exit %d\n%s", code, stdout)
	}

	broken := extPlugin(t, strings.Replace(extManifest, `"apiVersion": 1`, `"apiVersion": 7`, 1), nil)
	code, stdout, _ = run(t, "ext", "check", broken)
	if code != extProblems || !strings.Contains(stdout, "apiVersion 7, this app speaks 1 through 1") {
		t.Fatalf("exit %d\n%s", code, stdout)
	}
}

// The command line itself is a third outcome: usage on stderr, exit 2.
func TestExtUsage(t *testing.T) {
	for _, args := range [][]string{
		{"ext"},
		{"ext", "nonsense"},
		{"ext", "test"},
		{"ext", "test", "one", "two"},
		{"ext", "check"},
		{"ext", "types", "extra"},
	} {
		code, _, stderr := run(t, args...)
		if code != extUsage {
			t.Errorf("remit %v: exit %d", args, code)
			continue
		}
		if !strings.Contains(stderr, "usage: remit ext") {
			t.Errorf("remit %v: stderr:\n%s", args, stderr)
		}
	}
}

// Asking for help is not a problem with the command line: usage on stdout and
// exit 0, however deep the help was asked for.
func TestExtHelp(t *testing.T) {
	for _, args := range [][]string{
		{"ext", "help"},
		{"ext", "-h"},
		{"ext", "test", "--help"},
		{"ext", "types", "-h"},
	} {
		code, stdout, _ := run(t, args...)
		if code != extOK || !strings.Contains(stdout, "usage: remit ext") {
			t.Errorf("remit %v: exit %d, stdout:\n%s", args, code, stdout)
		}
	}
}

// remit ext types is how the checked-in contract is regenerated: stdout by
// default, the file -o names otherwise.
func TestExtTypesWritesTheContract(t *testing.T) {
	code, stdout, _ := run(t, "ext", "types")
	if code != extOK {
		t.Fatalf("exit %d", code)
	}
	if stdout != string(ts.Render()) {
		t.Fatal("stdout is not the rendered contract")
	}
	if !strings.Contains(stdout, "export declare const apiVersion: 1;") {
		t.Fatalf("stdout:\n%s", stdout[:200])
	}

	path := filepath.Join(t.TempDir(), ts.FileName)
	code, _, stderr := run(t, "ext", "types", "-o", path)
	if code != extOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(written, ts.Render()) {
		t.Fatal("the file is not the rendered contract")
	}
}

// The flag this command had before it was a subcommand still checks a plugin:
// a caller that has not moved yet is not broken by the move.
func TestTheLegacyCheckFlag(t *testing.T) {
	var out bytes.Buffer
	if _, ran := runCheckExtension("", &out); ran {
		t.Fatal("an empty flag ran the checker")
	}
	dir := extPlugin(t, extManifest, nil)
	code, ran := runCheckExtension(dir, &out)
	if !ran || code != extOK {
		t.Fatalf("ran %v, exit %d", ran, code)
	}
	if !strings.Contains(out.String(), dir+": ok") {
		t.Fatalf("stdout:\n%s", out.String())
	}
}
