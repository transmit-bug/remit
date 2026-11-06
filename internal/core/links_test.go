package core_test

import (
	"testing"

	"github.com/transmit-bug/remit/internal/core"
)

func TestLinks(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"none", "a paragraph with no link", nil},
		{"one", "see [[Spacing]] for more", []string{"Spacing"}},
		{"two, trimmed", "[[ Spacing ]] then [[Testing effect]]", []string{"Spacing", "Testing effect"}},
		{"unclosed is text", "a [[dangling link", nil},
		{"empty names nothing", "[[]] and [[   ]]", nil},
		{"a title may not hold an opening", "[[a [[b]]", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := core.Links(c.text)
			if len(got) != len(c.want) {
				t.Fatalf("found %d links, want %d: %v", len(got), len(c.want), got)
			}
			for i, want := range c.want {
				if got[i].Title != want {
					t.Errorf("link %d is %q, want %q", i, got[i].Title, want)
				}
			}
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"Spacing", "spacing"},
		{"  Spacing   beats ", "spacing beats"},
		{"A  daily   habit", "a daily habit"},
	} {
		if got, want := core.NormalizeTitle(c.a), core.NormalizeTitle(c.b); got != want {
			t.Errorf("%q normalises to %q, %q to %q", c.a, got, c.b, want)
		}
	}
	if core.NormalizeTitle("Spacing") == core.NormalizeTitle("Spaced") {
		t.Error("different titles normalise the same")
	}
}
