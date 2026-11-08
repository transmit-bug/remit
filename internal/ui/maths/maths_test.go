package maths_test

import (
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ui/maths"
)

func TestRenderFoldsImagesIntoTheirAltText(t *testing.T) {
	got := maths.Render("look: ![a diagram](assets/abc.png) and $x^2$")
	if want := "look: a diagram and x²"; got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
	// A picture with no alt text folds to nothing, not to its path.
	if got := maths.Render("before ![](assets/abc.png) after"); got != "before  after" {
		t.Errorf("Render = %q, want %q", got, "before  after")
	}
	// Links keep their text and destination.
	if got := maths.Render("[notes](notes.md)"); got != "[notes](notes.md)" {
		t.Errorf("Render = %q, want the link kept", got)
	}
}

func TestTranscribe(t *testing.T) {
	cases := []struct{ name, tex, want string }{
		{"plain", "x + y", "x + y"},
		{"superscript", "x^2", "x²"},
		{"braced superscript", "x^{2n}", "x²ⁿ"},
		{"subscript", "x_1", "x₁"},
		{"greek and product", `\pi r^2`, "πr²"},
		{"operator", `a \cdot b`, "a · b"},
		{"fraction", `\frac{a}{b}`, "a/b"},
		{"nested fraction", `\frac{1}{2} x`, "1/2 x"},
		{"root", `\sqrt{2}`, "√2"},
		{"text keeps its spaces", `\text{per day}`, "per day"},
		{"comparison", `a \le b \ne c`, "a ≤ b ≠ c"},
		{"unknown command keeps its name", `\foobar x`, "foobar x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := maths.Transcribe(c.tex); got != c.want {
				t.Errorf("Transcribe(%q) = %q, want %q", c.tex, got, c.want)
			}
		})
	}
}

func TestSegments(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []maths.Segment
	}{
		{"prose only", "no formula here", []maths.Segment{{Text: "no formula here"}}},
		{"inline", "the area $\\pi r^2$ of a circle", []maths.Segment{
			{Text: "the area "}, {Text: "πr²", Math: true}, {Text: " of a circle"},
		}},
		{"display", "$$x^2$$", []maths.Segment{{Text: "x²", Math: true}}},
		{"unpaired dollar is prose", "costs $5 today", []maths.Segment{{Text: "costs $5 today"}}},
		{"two formulas", "$a$ and $b$", []maths.Segment{
			{Text: "a", Math: true}, {Text: " and "}, {Text: "b", Math: true},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := maths.Segments(c.text)
			if len(got) != len(c.want) {
				t.Fatalf("got %d segments, want %d: %+v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("segment %d is %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// No formula reaches the page as its own source.
func TestNoTeXSurvives(t *testing.T) {
	for _, tex := range []string{`\pi r^2`, `\frac{a}{b}`, `\sqrt{3}`, `\alpha`, `x_1^2`} {
		got := maths.Transcribe(tex)
		for _, bad := range []string{"\\", "^", "_", "{", "}"} {
			if strings.Contains(got, bad) {
				t.Errorf("%q kept %q: %q", tex, bad, got)
			}
		}
	}
}
