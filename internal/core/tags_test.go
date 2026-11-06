package core_test

import (
	"testing"

	"github.com/transmit-bug/remit/internal/core"
)

func TestTags(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"none", "a paragraph with no tag", nil},
		{"one", "spacing :: the gaps #learning", []string{"learning"}},
		{"two on a line", "#spacing #learning", []string{"spacing", "learning"}},
		{"hyphen and underscore", "#spaced-repetition #recall_test", []string{"spaced-repetition", "recall_test"}},
		{"chinese", "记忆 #间隔重复", []string{"间隔重复"}},
		{"not mid word", "issue#42 stays text", nil},
		{"a bare hash is text", "# alone", nil},
		{"inline code is not a tag", "`#notatag` but #atag", []string{"atag"}},
		{"fenced code is not a tag", "```\n#comment\n```\n#real", []string{"real"}},
		{"two fenced blocks", "```\n#a\n```\n#b\n```\n#c\n```\n#d", []string{"b", "d"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := core.Tags(c.text)
			if len(got) != len(c.want) {
				t.Fatalf("found %d tags, want %d: %v", len(got), len(c.want), got)
			}
			for i, want := range c.want {
				if got[i] != want {
					t.Errorf("tag %d is %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
