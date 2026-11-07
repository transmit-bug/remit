package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ext/exttest"
	"github.com/transmit-bug/remit/internal/ext/ts"
	"github.com/transmit-bug/remit/internal/ui/design"
)

// The exit codes an ext command answers: 0 the plugin is one this build can
// run, 1 a problem its author has to fix, 2 the command line itself.
const (
	extOK       = 0
	extProblems = 1
	extUsage    = 2
)

const extHelp = `usage: remit ext <command>

  test <dir>    load a plugin, validate what it declares, and run every hook it
                carries against the fixtures in <dir>/fixtures
  check <dir>   check a plugin's manifest and what it contributes to the window
  types         write the plugin contract (remit.d.ts) to stdout; -o <file>
                writes it in place

A plugin directory holds extension.json and its entry file; see
docs/specs/plugins/writing-a-plugin.md.
`

// runExt runs one "remit ext" command and reports whether the command line was
// one. It runs before the window exists: no library, no data directory.
func runExt(args []string, stdout, stderr io.Writer) (int, bool) {
	if len(args) == 0 || args[0] != "ext" {
		return extOK, false
	}
	if len(args) < 2 {
		fmt.Fprint(stderr, extHelp)
		return extUsage, true
	}
	for _, arg := range args[1:] {
		if arg == "-h" || arg == "--help" || arg == "help" {
			fmt.Fprint(stdout, extHelp)
			return extOK, true
		}
	}
	switch args[1] {
	case "test":
		dir, code := extDir(args[2:], "test", stderr)
		if code != extOK {
			return code, true
		}
		return extTest(dir, stdout), true
	case "check":
		dir, code := extDir(args[2:], "check", stderr)
		if code != extOK {
			return code, true
		}
		return extCheck(dir, stdout), true
	case "types":
		return extTypes(args[2:], stdout, stderr), true
	}
	fmt.Fprintf(stderr, "remit ext: no such command %q\n\n%s", args[1], extHelp)
	return extUsage, true
}

// extDir reads the one directory a test or check is about.
func extDir(args []string, command string, stderr io.Writer) (string, int) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintf(stderr, "remit ext %s: one plugin directory\n\n%s", command, extHelp)
		return "", extUsage
	}
	return args[0], extOK
}

// extTest runs a plugin's hooks against its own fixtures and prints what it
// found. Every problem is one line, so a CI log names the fixture and the
// answer that differed.
func extTest(dir string, out io.Writer) int {
	result := exttest.Run(dir, exttest.Options{Icon: design.HasIcon})
	for _, problem := range result.Problems {
		fmt.Fprintf(out, "%s: %v\n", dir, problem)
	}
	if len(result.Problems) > 0 {
		return extProblems
	}
	fmt.Fprintf(out, "%s: ok (%d cases: %s)\n", dir, result.Cases, strings.Join(result.Hooks, ", "))
	return extOK
}

// extCheck checks a plugin's manifest and contributions without running any of
// its code: the same parse the window runs when it reads a library's plugins.
func extCheck(dir string, out io.Writer) int {
	problems := contrib.Check(dir, contrib.Options{Icon: design.HasIcon})
	if len(problems) == 0 {
		fmt.Fprintf(out, "%s: ok\n", dir)
		return extOK
	}
	for _, problem := range problems {
		fmt.Fprintf(out, "%s: %v\n", dir, problem)
	}
	return extProblems
}

// extTypes writes the plugin contract: stdout by default, or the file -o
// names, which is how the checked-in remit.d.ts is regenerated.
func extTypes(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("remit ext types", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("o", "", "write the contract to this file instead of stdout")
	if err := flags.Parse(args); err != nil {
		return extUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "remit ext types: no arguments\n\n%s", extHelp)
		return extUsage
	}
	if *out == "" {
		if _, err := stdout.Write(ts.Render()); err != nil {
			fmt.Fprintf(stderr, "remit ext types: %v\n", err)
			return extProblems
		}
		return extOK
	}
	if err := os.WriteFile(*out, ts.Render(), 0o644); err != nil {
		fmt.Fprintf(stderr, "remit ext types: %v\n", err)
		return extProblems
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return extOK
}
