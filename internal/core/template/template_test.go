package template_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/core/template"
)

// mustCompile is a helper for render tests: syntax there is not under test.
func mustCompile(t *testing.T, src string) *template.Template {
	t.Helper()
	tpl, err := template.Compile(src)
	if err != nil {
		t.Fatalf("Compile(%q): %v", src, err)
	}
	return tpl
}

// wantKind asserts err is a *template.Error of the given sentinel kind.
func wantKind(t *testing.T, err error, kind error) {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error, want %v", kind)
	}
	if !errors.Is(err, kind) {
		t.Fatalf("error %v does not match %v", err, kind)
	}
	var terr *template.Error
	if !errors.As(err, &terr) {
		t.Fatalf("error %v is not a *template.Error", err)
	}
}

func TestCompileErrors(t *testing.T) {
	t.Helper()
	tests := []struct {
		name string
		src  string
		kind error
	}{
		{"unclosed tag", "hello {{Front", template.ErrSyntax},
		{"empty tag", "{{}}", template.ErrSyntax},
		{"section without name", "{{#}}x{{/}}", template.ErrSyntax},
		{"inverted without name", "{{^}}x{{/}}", template.ErrSyntax},
		{"close without name", "{{Front}}{{/}}", template.ErrSyntax},
		{"unclosed section", "{{#Front}}text", template.ErrSyntax},
		{"unclosed inverted section", "{{^Front}}text", template.ErrSyntax},
		{"mismatched close", "{{#Front}}x{{/Back}}", template.ErrSyntax},
		{"mismatched close nested", "{{#A}}{{#B}}x{{/A}}{{/B}}", template.ErrSyntax},
		{"unexpected close", "{{/Front}}", template.ErrSyntax},
		{"unexpected close after text", "plain {{/Front}}", template.ErrSyntax},
		{"unknown construct", "{{foo:Bar}}", template.ErrSyntax},
		{"cloze without field", "{{cloze:}}", template.ErrSyntax},
		{"hint without field", "{{hint:}}", template.ErrSyntax},
		{"type without field", "{{type:}}", template.ErrSyntax},
		{"unknown filter", "{{Front|upper}}", template.ErrUnknownFilter},
		{"unknown filter arbitrary name", "{{Front|nope}}", template.ErrUnknownFilter},
		{"filter on other tag", "{{cloze:Text|upper}}", template.ErrUnknownFilter},
		{"duplicate hint", "{{hint:Extra}} and {{hint:Extra}}", template.ErrSyntax},
		{"duplicate hint different fields", "{{hint:A}}{{hint:B}}", template.ErrSyntax},
		{"duplicate type", "{{type:Back}}{{type:Back}}", template.ErrSyntax},
		{"duplicate type different fields", "{{type:A}}{{type:B}}", template.ErrSyntax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			_, err := template.Compile(tt.src)
			wantKind(t, err, tt.kind)
		})
	}
}

func TestCompileErrorOffset(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		offAt int // the error offset must point at this substring
	}{
		{"unclosed tag points at it", "ab {{Front", 3},
		{"unknown filter points at tag", "x\n{{Front|upper}}", 2},
		{"unclosed section points at opener", "ok {{#Front}} body", 3},
		{"mismatched close points at close", "{{#A}}x{{/B}}", 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			_, err := template.Compile(tt.src)
			if err == nil {
				t.Fatalf("Compile(%q) succeeded, want error", tt.src)
			}
			var terr *template.Error
			if !errors.As(err, &terr) {
				t.Fatalf("error %v is not a *template.Error", err)
			}
			if terr.Offset != tt.offAt {
				t.Errorf("offset = %d, want %d (err: %v)", terr.Offset, tt.offAt, err)
			}
		})
	}
}

func TestCompileAccepts(t *testing.T) {
	t.Helper()
	srcs := []string{
		"",
		"plain text, no tags",
		"}}",
		"a }} b {{Front}} c }}",
		"{{Front}}",
		"{{#Front}}x{{/Front}}",
		"{{^Back}}y{{/Back}}",
		"{{#A}}{{#B}}{{/B}}{{/A}}",
		"{{^A}}{{#B}}{{/B}}{{/A}}",
		"{{cloze:Text}}",
		"{{hint:Extra}}",
		"{{type:Back}}",
		"{{hint:A}}{{type:B}}",
	}
	for _, src := range srcs {
		t.Run(src, func(t *testing.T) {
			t.Helper()
			if _, err := template.Compile(src); err != nil {
				t.Errorf("Compile(%q): %v", src, err)
			}
		})
	}
}

func TestRenderField(t *testing.T) {
	t.Helper()
	tests := []struct {
		name string
		src  string
		in   template.Input
		want string
	}{
		{"plain substitution", "{{Front}}", in(template.SideFront, map[string]string{"Front": "hello"}, 0), "hello"},
		{"value rendered verbatim", "{{Front}}", in(template.SideFront, map[string]string{"Front": "*_markdown_* <b>"}, 0), "*_markdown_* <b>"},
		{"empty value renders empty", "a{{Front}}b", in(template.SideFront, map[string]string{"Front": ""}, 0), "ab"},
		{"text around tags", "Q: {{Front}}?", in(template.SideFront, map[string]string{"Front": "2+2"}, 0), "Q: 2+2?"},
		{"repeated field", "{{Front}}/{{Front}}", in(template.SideFront, map[string]string{"Front": "x"}, 0), "x/x"},
		{"multiple fields", "{{Front}}={{Back}}", in(template.SideBack, map[string]string{"Front": "a", "Back": "b"}, 0), "a=b"},
		{"lone close braces are literal", "a}}b", in(template.SideFront, map[string]string{}, 0), "a}}b"},
		{"trailing trim", "{{Front}}\n\n  ", in(template.SideFront, map[string]string{"Front": "v"}, 0), "v"},
		{"trailing trim mixed whitespace", "x {{Front}}\t\n", in(template.SideFront, map[string]string{"Front": "y"}, 0), "x y"},
		{"leading whitespace kept", "  {{Front}}", in(template.SideFront, map[string]string{"Front": "v"}, 0), "  v"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			out, err := mustCompile(t, tt.src).Render(tt.in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if out.Markdown != tt.want {
				t.Errorf("Markdown = %q, want %q", out.Markdown, tt.want)
			}
		})
	}
}

func TestRenderUnknownField(t *testing.T) {
	t.Helper()
	for _, side := range []template.Side{template.SideFront, template.SideBack} {
		srcs := []string{"{{Missing}}", "{{#Missing}}x{{/Missing}}", "{{^Missing}}x{{/Missing}}", "{{cloze:Missing}}", "{{hint:Missing}}", "{{type:Missing}}"}
		for _, src := range srcs {
			_, err := mustCompile(t, src).Render(in(side, map[string]string{"Front": "v"}, 1))
			wantKind(t, err, template.ErrUnknownField)
		}
	}
}

func TestRenderSections(t *testing.T) {
	t.Helper()
	tests := []struct {
		name string
		src  string
		in   template.Input
		want string
	}{
		{"truthy renders", "{{#Front}}has{{/Front}}", in(template.SideFront, map[string]string{"Front": "v"}, 0), "has"},
		{"whitespace-only value counts as empty", "{{#Front}}has{{/Front}}", in(template.SideFront, map[string]string{"Front": "  \n "}, 0), ""},
		{"empty value skips", "{{#Front}}has{{/Front}}", in(template.SideFront, map[string]string{"Front": ""}, 0), ""},
		{"inverted renders when empty", "{{^Front}}none{{/Front}}", in(template.SideFront, map[string]string{"Front": ""}, 0), "none"},
		{"inverted skips when set", "{{^Front}}none{{/Front}}", in(template.SideFront, map[string]string{"Front": "v"}, 0), ""},
		{"inverted renders when whitespace-only", "{{^Front}}none{{/Front}}", in(template.SideFront, map[string]string{"Front": "  "}, 0), "none"},
		{"section body references the field", "{{#Front}}[{{Front}}]{{/Front}}", in(template.SideFront, map[string]string{"Front": "v"}, 0), "[v]"},
		{"nested truthy", "{{#A}}a{{#B}}b{{/B}}{{/A}}", in(template.SideFront, map[string]string{"A": "1", "B": "2"}, 0), "ab"},
		{"nested inner empty", "{{#A}}a{{#B}}b{{/B}}{{/A}}", in(template.SideFront, map[string]string{"A": "1", "B": ""}, 0), "a"},
		{"nested outer empty skips inner", "{{#A}}a{{#B}}b{{/B}}{{/A}}", in(template.SideFront, map[string]string{"A": "", "B": "2"}, 0), ""},
		{"nested inverted inside truthy", "{{#A}}{{^B}}nb{{/B}}{{/A}}", in(template.SideFront, map[string]string{"A": "1", "B": ""}, 0), "nb"},
		{"same-field nesting", "{{#A}}x{{#A}}y{{/A}}{{/A}}", in(template.SideFront, map[string]string{"A": "1"}, 0), "xy"},
		{"sibling sections", "{{#A}}a{{/A}}{{^B}}!{{/B}}", in(template.SideFront, map[string]string{"A": "1", "B": ""}, 0), "a!"},
		{"back side sections", "{{^Extra}}no extra{{/Extra}}", in(template.SideBack, map[string]string{"Extra": ""}, 0), "no extra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			out, err := mustCompile(t, tt.src).Render(tt.in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if out.Markdown != tt.want {
				t.Errorf("Markdown = %q, want %q", out.Markdown, tt.want)
			}
		})
	}
}
func TestRenderCloze(t *testing.T) {
	t.Helper()
	twoBlanks := map[string]string{"Text": "In {{1969}} humans landed on the {{Moon}}"}
	tests := []struct {
		name string
		src  string
		in   template.Input
		want string
	}{
		{"front ordinal 0 masks first blank", "{{cloze:Text}}", in(template.SideFront, twoBlanks, 0), "In […] humans landed on the Moon"},
		{"front ordinal 1 masks second blank", "{{cloze:Text}}", in(template.SideFront, twoBlanks, 1), "In 1969 humans landed on the […]"},
		{"back reveals all", "{{cloze:Text}}", in(template.SideBack, twoBlanks, 0), "In 1969 humans landed on the Moon"},
		{"back other ordinal same output", "{{cloze:Text}}", in(template.SideBack, twoBlanks, 1), "In 1969 humans landed on the Moon"},
		{"single blank ordinal 0", "{{cloze:Text}}", in(template.SideFront, map[string]string{"Text": "{{answer}}"}, 0), "[…]"},
		{"adjacent blanks", "{{cloze:Text}}", in(template.SideFront, map[string]string{"Text": "{{a}}{{b}}"}, 1), "a[…]"},
		{"adjacent blanks back", "{{cloze:Text}}", in(template.SideBack, map[string]string{"Text": "{{a}}{{b}}"}, 0), "ab"},
		{"no blanks front asks none", "{{cloze:Text}}", in(template.SideFront, map[string]string{"Text": "no blanks here"}, 0), "no blanks here"},
		{"value with unclosed braces stays literal", "{{cloze:Text}}", in(template.SideFront, map[string]string{"Text": "a {{ b"}, 0), "a {{ b"},
		{"no blanks back asks none", "{{cloze:Text}}", in(template.SideBack, map[string]string{"Text": "no blanks here"}, 0), "no blanks here"},
		{"ordinal negative reveals all", "{{cloze:Text}}", in(template.SideFront, twoBlanks, -1), "In 1969 humans landed on the Moon"},
		{"ordinal beyond blanks reveals all", "{{cloze:Text}}", in(template.SideFront, twoBlanks, 2), "In 1969 humans landed on the Moon"},
		{"empty value renders empty", "{{cloze:Text}}", in(template.SideFront, map[string]string{"Text": ""}, 0), ""},
		{"text around cloze", "Q: {{cloze:Text}}.", in(template.SideFront, twoBlanks, 0), "Q: In […] humans landed on the Moon."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			out, err := mustCompile(t, tt.src).Render(tt.in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if out.Markdown != tt.want {
				t.Errorf("Markdown = %q, want %q", out.Markdown, tt.want)
			}
		})
	}
}

func TestRenderHintAndType(t *testing.T) {
	t.Helper()
	tests := []struct {
		name       string
		src        string
		in         template.Input
		wantMD     string
		wantHint   string
		wantAnswer string
	}{
		{"hint captured", "Guess: {{Front}} {{hint:Extra}}", in(template.SideFront, map[string]string{"Front": "q", "Extra": "e"}, 0), "Guess: q", "e", ""},
		{"hint renders empty inline even with value", "{{hint:Extra}}x", in(template.SideFront, map[string]string{"Extra": "tip"}, 0), "x", "tip", ""},
		{"type captured", "{{Front}} {{type:Back}}", in(template.SideFront, map[string]string{"Front": "q", "Back": "a"}, 0), "q", "", "a"},
		{"hint and type together", "{{Front}}{{hint:H}}{{type:A}}", in(template.SideFront, map[string]string{"Front": "q", "H": "h", "A": "a"}, 0), "q", "h", "a"},
		{"hint on back side too", "{{Back}}{{hint:H}}", in(template.SideBack, map[string]string{"Back": "b", "H": "h"}, 0), "b", "h", ""},
		{"empty hint value captured as empty", "{{hint:H}}", in(template.SideFront, map[string]string{"H": ""}, 0), "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			out, err := mustCompile(t, tt.src).Render(tt.in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if out.Markdown != tt.wantMD {
				t.Errorf("Markdown = %q, want %q", out.Markdown, tt.wantMD)
			}
			if out.Hint != tt.wantHint {
				t.Errorf("Hint = %q, want %q", out.Hint, tt.wantHint)
			}
			if out.TypeAnswer != tt.wantAnswer {
				t.Errorf("TypeAnswer = %q, want %q", out.TypeAnswer, tt.wantAnswer)
			}
		})
	}
}

func TestRenderErrorOffset(t *testing.T) {
	t.Helper()
	tpl := mustCompile(t, "ok {{Missing}} tail")
	_, err := tpl.Render(in(template.SideFront, map[string]string{"A": "1"}, 0))
	wantKind(t, err, template.ErrUnknownField)
	var terr *template.Error
	if !errors.As(err, &terr) {
		t.Fatalf("not a *template.Error")
	}
	if terr.Offset != 3 {
		t.Errorf("offset = %d, want 3", terr.Offset)
	}
}

func TestCompileOnceRenderMany(t *testing.T) {
	t.Helper()
	tpl := mustCompile(t, "{{cloze:Text}}\n{{hint:Extra}}")
	for ordinal := range 3 {
		out, err := tpl.Render(in(template.SideFront, map[string]string{"Text": "{{a}} {{b}} {{c}}", "Extra": "h"}, ordinal))
		if err != nil {
			t.Fatalf("Render ordinal %d: %v", ordinal, err)
		}
		want := "[…]"
		switch ordinal {
		case 0:
			want = "[…] b c"
		case 1:
			want = "a […] c"
		case 2:
			want = "a b […]"
		}
		if out.Markdown != want {
			t.Errorf("ordinal %d: Markdown = %q, want %q", ordinal, out.Markdown, want)
		}
		if out.Hint != "h" {
			t.Errorf("ordinal %d: Hint = %q, want %q", ordinal, out.Hint, "h")
		}
	}
	// Back side with the same compiled template.
	out, err := tpl.Render(in(template.SideBack, map[string]string{"Text": "{{a}} {{b}}", "Extra": ""}, 1))
	if err != nil {
		t.Fatalf("Render back: %v", err)
	}
	if out.Markdown != "a b" {
		t.Errorf("back: Markdown = %q, want %q", out.Markdown, "a b")
	}
	// A value without blanks asks none of them and renders as it is.
	out, err = tpl.Render(in(template.SideFront, map[string]string{"Text": "no blanks", "Extra": ""}, 0))
	if err != nil || out.Markdown != "no blanks" {
		t.Errorf("no blanks: (%q, %v), want (no blanks, nil)", out.Markdown, err)
	}
}

func TestFallback(t *testing.T) {
	t.Helper()
	tests := []struct {
		name   string
		fields map[string]string
		want   string
	}{
		{"ordered list", map[string]string{"Back": "b", "Front": "a"}, "- Back: b\n- Front: a"},
		{"single field", map[string]string{"Text": "hello"}, "- Text: hello"},
		{"empty value still listed", map[string]string{"Front": ""}, "- Front: "},
		{"empty map still non-blank", map[string]string{}, "- (no fields)"},
		{"nil map still non-blank", nil, "- (no fields)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			got := template.Fallback(tt.fields)
			if got != tt.want {
				t.Errorf("Fallback = %q, want %q", got, tt.want)
			}
			if strings.TrimSpace(got) == "" {
				t.Errorf("Fallback returned blank output")
			}
		})
	}
}

func in(side template.Side, fields map[string]string, ordinal int) template.Input {
	return template.Input{Fields: fields, Side: side, Ordinal: ordinal}
}
