package core_test

import (
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
)

func TestClozeWrapsTheFirstHit(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		selected string
		want     string
	}{
		{"a word", "spacing beats cramming", "cramming", "spacing beats {{cramming}}"},
		{"only the first hit", "a b a", "a", "{{a}} b a"},
		{"at the start", "entropy grows", "entropy", "{{entropy}} grows"},
		{"at the end", "entropy grows", "grows", "entropy {{grows}}"},
		{"a phrase", "spacing beats cramming", "spacing beats", "{{spacing beats}} cramming"},
		{"a card separator", "spacing :: beats cramming", "spacing :: beats", "{{spacing :: beats}} cramming"},
		{"a multibyte selection", "日本語のテキスト", "語の", "日本{{語の}}テキスト"},
		{"a multibyte selection at the start", "日本語のテキスト", "日本", "{{日本}}語のテキスト"},
		{"an emoji", "recall 🧠 improves", "🧠", "recall {{🧠}} improves"},
		{
			"already clozed",
			"The {{first}} blank and the second one.",
			"first",
			"The {{first}} blank and the second one.",
		},
		{
			"a different blank is left alone",
			"The {{first}} blank and the second one.",
			"second",
			"The {{first}} blank and the {{second}} one.",
		},
		{"not in the text", "spacing beats cramming", "quantum", "spacing beats cramming"},
		{"an empty selection", "spacing beats cramming", "", "spacing beats cramming"},
		{"a blank selection", "spacing beats cramming", "  \t", "spacing beats cramming"},
		{"an empty text", "", "word", ""},
		{"nothing to wrap twice", "cramming", "cramming", "{{cramming}}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := core.Cloze(tt.text, tt.selected); got != tt.want {
				t.Errorf("Cloze(%q, %q) = %q, want %q", tt.text, tt.selected, got, tt.want)
			}
		})
	}
}

// TestClozeIsIdempotent is the guard against a second pass over a paragraph the
// editor already rewrote: the blank it made is not wrapped again, so the
// selection survives being applied twice — clicking the action twice, or
// re-applying it over a selection that now includes the braces.
func TestClozeIsIdempotent(t *testing.T) {
	const text = "spacing beats cramming"
	once := core.Cloze(text, "cramming")
	if twice := core.Cloze(once, "cramming"); twice != once {
		t.Errorf("second pass = %q, want the text unchanged at %q", twice, once)
	}
	// The same rule covers the selection with its braces already around it: a
	// user who selected "{{cramming}}" gets one blank, not two.
	if got := core.Cloze(once, "{{cramming}}"); got != once {
		t.Errorf("Cloze over its own blank = %q, want %q", got, once)
	}
}

// TestClozeCountsBlanks is what the cloze type leans on: one action makes one
// new blank, which is one new card ordinal.
func TestClozeCountsBlanks(t *testing.T) {
	got := core.Cloze("one two three", "two")
	if n := strings.Count(got, "{{"); n != 1 {
		t.Errorf("%q holds %d blanks, want 1", got, n)
	}
}
