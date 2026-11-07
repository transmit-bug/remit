package exttest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext/exttest"
)

// plugin writes a plugin directory: the manifest, the entry, and the fixture
// files given by path relative to fixtures/.
func plugin(t *testing.T, manifest string, fixtures map[string]string) string {
	t.Helper()
	return pluginSource(t, manifest, `export function deriveBlock(block) {
  const text = (block && block.text) || "";
  const at = text.indexOf("::");
  if (at < 0) return [];
  return [{ typeId: "remit.basic", ordinal: 0, fields: { Front: text.slice(0, at).trim(), Back: text.slice(at + 2).trim() } }];
}
export function policy() { return { family: "fsrs", version: "1", retention: 0.9 }; }
`, fixtures)
}

func pluginSource(t *testing.T, manifest, source string, fixtures map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "extension.json"), manifest)
	write(t, filepath.Join(dir, "main.js"), source)
	for name, body := range fixtures {
		write(t, filepath.Join(dir, "fixtures", filepath.FromSlash(name)), body)
	}
	return dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const derived = `{
  "id": "acme.derived",
  "name": "derived",
  "version": "0.1.0",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js",
  "implements": { "deriveBlock": 1 }
}`

// problems joins what the run found, for an assertion that names the reason.
func problems(result exttest.Result) string {
	var lines []string
	for _, problem := range result.Problems {
		lines = append(lines, problem.Error())
	}
	return strings.Join(lines, "\n")
}

// A plugin whose hooks answer what its fixtures hold is trusted, and the run
// counts the comparisons it made.
func TestAPluginIsHeldToItsFixtures(t *testing.T) {
	dir := plugin(t, derived, map[string]string{
		"deriveBlock/one-card.in.json":  `{"args":[{"text":"spacing :: the gaps"}]}`,
		"deriveBlock/one-card.out.json": `[{"fields":{"Back":"the gaps","Front":"spacing"},"ordinal":0,"typeId":"remit.basic"}]`,
		"deriveBlock/prose.in.json":     `{"args":[{"text":"just prose"}]}`,
		"deriveBlock/prose.out.json":    `[]`,
	})

	result := exttest.Run(dir, exttest.Options{})
	if len(result.Problems) > 0 {
		t.Fatalf("a valid plugin has problems:\n%s", problems(result))
	}
	if result.Cases != 2 || len(result.Hooks) != 1 || result.Hooks[0] != "deriveBlock" {
		t.Fatalf("ran %d cases over %v", result.Cases, result.Hooks)
	}
}

// The fixture's whitespace is not part of the answer: both sides are compared
// as JSON, so a fixture may be written for a reader.
func TestFixtureWhitespaceIsNotTheAnswer(t *testing.T) {
	dir := plugin(t, derived, map[string]string{
		"deriveBlock/prose.out.json": `[

		]`,
	})
	result := exttest.Run(dir, exttest.Options{})
	if len(result.Problems) > 0 {
		t.Fatalf("problems:\n%s", problems(result))
	}
}

// An answer that differs names the fixture and holds both answers, so a plugin
// author sees what the hook said.
func TestAMismatchedAnswerIsReported(t *testing.T) {
	dir := plugin(t, derived, map[string]string{
		"deriveBlock/prose.in.json":  `{"args":[{"text":"just prose"}]}`,
		"deriveBlock/prose.out.json": `[{"typeId":"remit.basic"}]`,
	})
	result := exttest.Run(dir, exttest.Options{})
	if len(result.Problems) != 1 {
		t.Fatalf("want one problem, got:\n%s", problems(result))
	}
	got := result.Problems[0].Error()
	for _, want := range []string{"fixtures/deriveBlock/prose", "answered", "want"} {
		if !strings.Contains(got, want) {
			t.Errorf("the problem does not name %q:\n%s", want, got)
		}
	}
	if result.Cases != 0 {
		t.Errorf("a mismatch counted %d cases", result.Cases)
	}
}

// A hook the manifest declares and no fixture covers is a problem: a hook
// nobody compared is a hook nobody looked at.
func TestADeclaredHookNeedsACase(t *testing.T) {
	manifest := strings.Replace(derived, `"implements": { "deriveBlock": 1 }`, `"implements": { "deriveBlock": 1, "policy": 1 }`, 1)
	dir := plugin(t, manifest, map[string]string{
		"deriveBlock/prose.out.json": `[]`,
	})
	result := exttest.Run(dir, exttest.Options{})
	if !strings.Contains(problems(result), `implements "policy": fixtures/policy/ has no case`) {
		t.Fatalf("problems:\n%s", problems(result))
	}
}

// A fixture for a hook the plugin does not carry cannot be run: the host has
// nowhere to ask it.
func TestAFixtureForAnUndeclaredHookIsReported(t *testing.T) {
	dir := plugin(t, derived, map[string]string{
		"deriveBlock/prose.out.json": `[]`,
		"policy/answer.out.json":     `{"family":"fsrs","version":"1","retention":0.9}`,
	})
	result := exttest.Run(dir, exttest.Options{})
	if !strings.Contains(problems(result), "fixtures/policy: derived does not carry that hook") {
		t.Fatalf("problems:\n%s", problems(result))
	}
}

// An input without an answer is a case the harness cannot compare, and saying
// so beats silently skipping it.
func TestAnInputWithoutAnAnswerIsReported(t *testing.T) {
	dir := plugin(t, derived, map[string]string{
		"deriveBlock/prose.in.json":  `{"args":[{"text":"just prose"}]}`,
		"deriveBlock/prose.out.json": `[]`,
		"deriveBlock/typo.in.json":   `{"args":[{"text":"x"}]}`,
	})
	result := exttest.Run(dir, exttest.Options{})
	if !strings.Contains(problems(result), "deriveBlock/typo.in.json: no answer to compare") {
		t.Fatalf("problems:\n%s", problems(result))
	}
}

// A manifest this build cannot load is reported by the same loader the app
// uses, and no hook is run.
func TestAnInvalidManifestIsReported(t *testing.T) {
	manifest := strings.Replace(derived, `"apiVersion": 1`, `"apiVersion": 2`, 1)
	dir := plugin(t, manifest, map[string]string{"deriveBlock/prose.out.json": `[]`})
	result := exttest.Run(dir, exttest.Options{})
	if !strings.Contains(problems(result), "apiVersion 2, this app speaks 1 through 1") {
		t.Fatalf("problems:\n%s", problems(result))
	}
}

// What contributes declares is held to the host's schema by the same parse the
// window runs, before any hook runs.
func TestAnInvalidContributionIsReported(t *testing.T) {
	manifest := `{
  "id": "acme.report",
  "name": "report",
  "version": "0.1.0",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js",
  "implements": { "deriveBlock": 1 },
  "contributes": { "commands": [
    { "id": "acme.report.run", "title": "%run%", "icon": "no-such-icon", "action": "command:run" }
  ], "locales": { "en": { "run": "Run it" } } }
}`
	dir := plugin(t, manifest, map[string]string{"deriveBlock/prose.out.json": `[]`})
	icons := func(name string) bool { return name == "leaf" }
	result := exttest.Run(dir, exttest.Options{Icon: icons})
	if !strings.Contains(problems(result), `icon "no-such-icon": the host has no such icon`) {
		t.Fatalf("problems:\n%s", problems(result))
	}
}

// A plugin that contributes a command callback is asked its command hook, and a
// contribution with no case is a problem: the command is the plugin's other
// surface.
func TestAContributedCommandNeedsACase(t *testing.T) {
	manifest := strings.Replace(derived, `"implements": { "deriveBlock": 1 }`,
		`"implements": { "deriveBlock": 1 },
  "contributes": { "commands": [{ "id": "acme.report.run", "title": "%run%", "action": "command:run" }], "locales": { "en": { "run": "Run it" } } }`, 1)
	dir := plugin(t, manifest, map[string]string{"deriveBlock/prose.out.json": `[]`})
	result := exttest.Run(dir, exttest.Options{})
	if !strings.Contains(problems(result), "contributes: fixtures/command/ has no case") {
		t.Fatalf("problems:\n%s", problems(result))
	}
}

// A plugin that carries no hook and contributes no command has nothing to
// compare, and saying so beats a silent pass.
func TestAPluginWithNothingToRunIsReported(t *testing.T) {
	manifest := `{
  "id": "acme.empty",
  "name": "empty",
  "version": "0.1.0",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js"
}`
	dir := plugin(t, manifest, nil)
	result := exttest.Run(dir, exttest.Options{})
	if !strings.Contains(problems(result), "nothing to run") {
		t.Fatalf("problems:\n%s", problems(result))
	}
}

// The fixture host answers reads from host.json and refuses writes, so a case
// that reads the library still reproduces.
func TestTheFixtureHostAnswersWhatHostJSONHolds(t *testing.T) {
	manifest := `{
  "id": "acme.reader",
  "name": "reader",
  "version": "0.1.0",
  "apiVersion": 1,
  "runtime": "js",
  "entry": "main.js",
  "capabilities": ["notes:read", "store:own"],
  "implements": { "deriveBlock": 1 }
}`
	source := `export function deriveBlock() {
  return [{ typeId: remit.notes.list()[0].title, ordinal: 0, fields: { stored: String(remit.store.get()) } }];
}
`
	dir := pluginSource(t, manifest, source, map[string]string{
		"host.json":                 `{"notes":[{"id":"note_1","title":"Memory","blocks":[{"id":"block_1","text":"a :: b"}]}],"pluginData":"kept"}`,
		"deriveBlock/read.out.json": `[{"fields":{"stored":"kept"},"ordinal":0,"typeId":"Memory"}]`,
	})
	result := exttest.Run(dir, exttest.Options{})
	if len(result.Problems) > 0 {
		t.Fatalf("the host did not answer:\n%s", problems(result))
	}
	if result.Cases != 1 {
		t.Fatalf("ran %d cases", result.Cases)
	}
}
