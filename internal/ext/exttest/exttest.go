// Package exttest runs a plugin's hooks against its own fixtures, without the
// app: what `remit ext test` reports, and what the suite holds the example
// plugins to.
//
// A plugin keeps its fixtures in its own directory:
//
//	fixtures/host.json                what the plugin may reach (optional)
//	fixtures/<hook>/<case>.in.json    the arguments (optional)
//	fixtures/<hook>/<case>.out.json   the answer the hook must give
//
// The comparison is JSON for JSON: both sides are read and written back with
// sorted keys and two-space indents, so a fixture's whitespace is not part of
// the answer. Every hook the manifest's implements names gets at least one case
// — a hook nobody compared is a hook nobody looked at — and a plugin that
// contributes a command callback gets fixtures/command/ too.
//
// A case is a pure call: the fixture host (host.go) answers reads from
// host.json and refuses writes, so an answer that reads the library still
// reproduces on every run.
package exttest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// The fixture layout's names, in one place so a command, a doc and a test say
// the same thing.
const (
	FixturesDir  = "fixtures"
	HostFile     = "host.json"
	InputSuffix  = ".in.json"
	OutputSuffix = ".out.json"
)

// Options is what the harness needs to know about the host it stands in for.
type Options struct {
	// Icon reports whether the host's icon set carries a name. nil accepts every
	// name, which only a test does — the app and the CLI pass the window's set.
	Icon func(name string) bool
}

// Result is what running one plugin's fixtures found. A plugin is trusted when
// Problems is empty: its manifest and contributions load, every hook it
// declares was compared, and every answer was the one its fixture holds.
type Result struct {
	// Dir is the plugin directory the result is about.
	Dir string
	// Name is the manifest name, empty when the manifest did not load.
	Name string
	// Hooks are the hooks that were compared, in the order they were asked.
	Hooks []string
	// Cases is how many answers were compared.
	Cases int
	// Problems is everything that keeps the plugin from being trusted.
	Problems []error
}

// Run loads the plugin in dir, validates what it declares and runs every hook it
// carries against the fixtures in its own directory. It starts nothing but the
// plugin itself, and it never reads a library.
func Run(dir string, opts Options) Result {
	result := Result{Dir: dir}
	e, err := ext.Load(dir)
	if err != nil {
		result.Problems = append(result.Problems, err)
		return result
	}
	result.Name = e.Manifest.Name

	// One validator: the same parse the window runs when it reads a library's
	// plugins, and the one `ext check` reports.
	declared, problems := contrib.Parse(e, contrib.Options{Icon: opts.Icon})
	result.Problems = append(result.Problems, problems...)

	fixtures, problems := readFixtures(filepath.Join(dir, FixturesDir))
	result.Problems = append(result.Problems, problems...)

	hooks, problems := planHooks(e.Manifest, declared, fixtures)
	result.Problems = append(result.Problems, problems...)
	if len(hooks) == 0 {
		return result
	}

	runner, err := ext.Open(e, fixtures.host)
	if err != nil {
		result.Problems = append(result.Problems, err)
		return result
	}
	defer runner.Close()

	for _, hook := range hooks {
		for _, one := range fixtures.cases[hook] {
			if err := compare(runner, one); err != nil {
				result.Problems = append(result.Problems, err)
				continue
			}
			result.Cases++
		}
	}
	result.Hooks = hooks
	return result
}

// planHooks is which hooks run and in what order: every hook implements names,
// then the command hook a contribution answers. A hook without a case, or a
// fixture for a hook the plugin does not carry, is a problem.
func planHooks(m ext.Manifest, c contrib.Contributions, f fixtures) ([]string, []error) {
	var problems []error
	allowed := make(map[string]bool, len(m.Implements)+1)
	for hook := range m.Implements {
		allowed[hook] = true
	}
	if contributesCallbacks(c) {
		allowed[ext.CommandHook] = true
	}

	for _, hook := range f.hooks {
		if !allowed[hook] {
			problems = append(problems, fmt.Errorf("fixtures/%s: %s does not carry that hook", hook, m.Name))
		}
	}

	var hooks []string
	for _, hook := range sortedHooks(m.Implements) {
		if len(f.cases[hook]) == 0 {
			problems = append(problems, fmt.Errorf(
				"implements %q: fixtures/%s/ has no case; every hook a plugin declares is compared", hook, hook))
			continue
		}
		hooks = append(hooks, hook)
	}
	if contributesCallbacks(c) {
		if len(f.cases[ext.CommandHook]) == 0 {
			problems = append(problems, fmt.Errorf(
				"contributes: fixtures/%s/ has no case for the command hook", ext.CommandHook))
		} else {
			hooks = append(hooks, ext.CommandHook)
		}
	}
	if len(hooks) == 0 && len(problems) == 0 {
		problems = append(problems, fmt.Errorf("%s declares no hook and contributes no command: there is nothing to run", m.Name))
	}
	return hooks, problems
}

// contributesCallbacks reports whether the window calls the plugin's command
// hook: a command with a callback, or a page, whose data is one.
func contributesCallbacks(c contrib.Contributions) bool {
	if len(c.Pages) > 0 {
		return true
	}
	for _, command := range c.Commands {
		if command.Action.Name != "" {
			return true
		}
	}
	return false
}

// fixtureCase is one hook call and the answer it must give.
type fixtureCase struct {
	hook string
	name string
	// args are the call's parameters, spread one per hook parameter; nil when
	// the fixture gives none.
	args []any
	// want is the answer, as canonical JSON.
	want []byte
}

// fixtures is one plugin's own test data.
type fixtures struct {
	host  *fixtureHost
	hooks []string
	cases map[string][]fixtureCase
}

// readFixtures reads fixtures/ — a directory per hook, and the host's answers.
// A plugin with no fixtures/ is not an error here: planHooks reports it as one
// problem per hook the plugin declares.
func readFixtures(dir string) (fixtures, []error) {
	out := fixtures{host: defaultHost(), cases: map[string][]fixtureCase{}}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, []error{fmt.Errorf("%s: %w", FixturesDir, err)}
	}

	var problems []error
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !entry.IsDir() {
			if name != HostFile {
				problems = append(problems, fmt.Errorf("%s/%s: fixtures live in %s/ and %s", FixturesDir, name, FixturesDir, HostFile))
				continue
			}
			h, err := readHost(filepath.Join(dir, name))
			if err != nil {
				problems = append(problems, err)
				continue
			}
			out.host = h
			continue
		}
		cases, errs := readHookDir(filepath.Join(dir, name), name)
		problems = append(problems, errs...)
		out.hooks = append(out.hooks, name)
		out.cases[name] = cases
	}
	sort.Strings(out.hooks)
	return out, problems
}

// readHookDir reads one hook's cases: every <case>.out.json is one comparison,
// and <case>.in.json, when it is there, is what the hook is called with.
func readHookDir(dir, hook string) ([]fixtureCase, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("%s/%s: %w", FixturesDir, hook, err)}
	}

	var problems []error
	answered := map[string]bool{}
	asked := map[string]bool{}
	var out []fixtureCase
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		where := FixturesDir + "/" + hook + "/" + name
		if entry.IsDir() {
			problems = append(problems, fmt.Errorf("%s: a hook's cases are files, not directories", where))
			continue
		}
		switch {
		case strings.HasSuffix(name, OutputSuffix):
			stem := strings.TrimSuffix(name, OutputSuffix)
			want, err := readCanonical(filepath.Join(dir, name))
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", where, err))
				continue
			}
			args, err := readArgs(filepath.Join(dir, stem+InputSuffix))
			if err != nil {
				problems = append(problems, fmt.Errorf("%s%s: %w", FixturesDir+"/"+hook+"/"+stem, InputSuffix, err))
				continue
			}
			answered[stem] = true
			out = append(out, fixtureCase{hook: hook, name: stem, args: args, want: want})
		case strings.HasSuffix(name, InputSuffix):
			asked[strings.TrimSuffix(name, InputSuffix)] = true
		default:
			problems = append(problems, fmt.Errorf("%s: a case is <name>%s and <name>%s", where, InputSuffix, OutputSuffix))
		}
	}

	var unanswered []string
	for stem := range asked {
		if !answered[stem] {
			unanswered = append(unanswered, stem)
		}
	}
	sort.Strings(unanswered)
	for _, stem := range unanswered {
		problems = append(problems, fmt.Errorf("%s/%s/%s%s: no answer to compare; a case needs %s",
			FixturesDir, hook, stem, InputSuffix, stem+OutputSuffix))
	}

	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, problems
}

// compare runs one case and holds the hook to its fixture.
func compare(runner ext.Hook, one fixtureCase) error {
	where := fmt.Sprintf("%s/%s/%s", FixturesDir, one.hook, one.name)
	var answer json.RawMessage
	if err := runner.Call(one.hook, one.args, &answer); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	got, err := canonical(answer)
	if err != nil {
		return fmt.Errorf("%s: the answer is not JSON: %w", where, err)
	}
	if !bytes.Equal(got, one.want) {
		return fmt.Errorf("%s: the hook answered\n%s\nwant\n%s", where, got, one.want)
	}
	return nil
}

// readCanonical reads a fixture and writes it back as the comparison holds it.
func readCanonical(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	got, err := canonical(data)
	if err != nil {
		return nil, fmt.Errorf("not JSON: %w", err)
	}
	return got, nil
}

// canonical is one JSON value as the comparison reads it: decoded and written
// back with sorted keys and two-space indents.
func canonical(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("null")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.MarshalIndent(value, "", "  ")
}

// readArgs reads a case's arguments: one array member per hook parameter, which
// is how Call spreads a []any. A case without an .in.json is called with none.
func readArgs(path string) ([]any, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var in struct {
		Args []any `json:"args"`
	}
	if err := decode(path, &in); err != nil {
		return nil, err
	}
	return in.Args, nil
}

// decode reads one JSON file strictly: a misspelled field is a problem naming
// the file, not a fixture that quietly says nothing.
func decode(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data after the JSON value")
	}
	return nil
}

// sortedHooks is a manifest's implements in a stable order, so a report reads
// the same way twice.
func sortedHooks(implements map[string]int) []string {
	hooks := make([]string, 0, len(implements))
	for hook := range implements {
		hooks = append(hooks, hook)
	}
	sort.Strings(hooks)
	return hooks
}
