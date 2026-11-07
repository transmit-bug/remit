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

// A link may name one block of a note, which is how an excerpt points at the
// paragraph it was taken from rather than only at the note.
func TestLinksWithBlockAnchor(t *testing.T) {
	block := core.NewID("block")
	cases := []struct {
		name      string
		text      string
		title     string
		wantBlock core.ID
	}{
		{"a block", "from [[" + "Spacing" + "#^" + string(block) + "]]", "Spacing", block},
		{"a title with spaces", "[[ Spacing beats cramming #^" + string(block) + " ]]", "Spacing beats cramming", block},
		{"an anchor that is not a block stays in the title", "[[a#^nonsense]]", "a#^nonsense", ""},
		{"a card ID is not a block", "[[a#^" + string(core.NewID("card")) + "]]", "a#^" + string(core.NewID("card")), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := core.Links(c.text)
			if len(got) != 1 {
				t.Fatalf("found %d links, want 1: %v", len(got), got)
			}
			if got[0].Block != c.wantBlock {
				t.Fatalf("block = %q, want %q", got[0].Block, c.wantBlock)
			}
			if c.wantBlock == "" && got[0].Title == "" {
				t.Fatal("an anchor that is not a block lost the whole title")
			}
			if c.wantBlock != "" && got[0].Title != c.title {
				t.Errorf("title = %q, want %q", got[0].Title, c.title)
			}
		})
	}
}
