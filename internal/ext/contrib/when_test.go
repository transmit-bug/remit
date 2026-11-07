package contrib_test

import (
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// A when expression reads the frozen context keys, and only those: a key this
// host does not know hides the contribution rather than showing it everywhere.
func TestWhen(t *testing.T) {
	review := contrib.Context{Mode: "review", LibraryOpen: true, CardPresent: true, CardStatus: "review"}
	browsing := contrib.Context{Mode: "browse", LibraryOpen: true, BrowseSelection: true}
	empty := contrib.Context{}

	for _, tc := range []struct {
		name string
		when string
		ctx  contrib.Context
		want bool
	}{
		{"no expression", "", empty, true},
		{"a boolean key", "library.open", review, true},
		{"a false key", "library.open", empty, false},
		{"a bare word is a value", "mode == review", review, true},
		{"a bare word that differs", "mode == review", browsing, false},
		{"a quoted value", `mode == "browse"`, browsing, true},
		{"and", "library.open && mode == review", review, true},
		{"and fails", "library.open && mode == review", browsing, false},
		{"or in parentheses", "library.open && (mode == review || mode == browse)", browsing, true},
		{"not", "!card.present", browsing, true},
		{"not a key", "!nonsense.key", review, true},
		{"a card status", "card.status == suspended", review, false},
		{"selection", "browse.selection", browsing, true},
		{"inequality", `mode != review`, browsing, true},
		{"a page", `page.id == acme.leeches`, contrib.Context{PageID: "acme.leeches"}, true},
		{"an unknown key reads false", "newer.thing", review, false},
		{"an unknown key is not a value", "mode == newer.thing", review, false},
		{"a number", "mode == 3", review, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := contrib.When(tc.when, tc.ctx); got != tc.want {
				t.Errorf("When(%q) = %v, want %v", tc.when, got, tc.want)
			}
		})
	}
}

// An unknown key is a warning that keeps the expression, so the window logs it
// and the contribution stays hidden rather than failing to load.
func TestUnknownKeyWarns(t *testing.T) {
	expr, warnings, err := contrib.Compile("library.open && newer.thing && newer.thing")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %q, want one for the repeated key", warnings)
	}
	if !strings.Contains(warnings[0], "newer.thing") {
		t.Errorf("warning does not name the key: %q", warnings[0])
	}
	if expr.Eval(contrib.Context{LibraryOpen: true}) {
		t.Error("an unknown key read true; a contribution asking about a newer host must stay hidden")
	}
	if contrib.When("newer.thing", contrib.Context{}) {
		t.Error("an unknown key alone read true")
	}
}

// A typo is a validation error, not a contribution that silently hides: the
// plugin author is told what the expression got wrong.
func TestCompileRefusesBadSyntax(t *testing.T) {
	for _, src := range []string{
		"mode",                // a bare word is a value, so this compiles: checked below
		"mode ==",             // no right side
		"mode = review",       // one =
		"a & b",               // one &
		"a | b",               // one |
		"(mode == notes",      // no closing parenthesis
		"mode == review mode", // two expressions
	} {
		_, _, err := contrib.Compile(src)
		if src == "mode" {
			if err != nil {
				t.Errorf("Compile(%q): a bare word is a value and must compile: %v", src, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("Compile(%q) accepted a broken expression", src)
		}
	}
}
