package importanki

import (
	"reflect"
	"testing"
)

func TestFrontFieldName(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		want string
	}{
		{"plain", "{{Front}}", "Front"},
		{"filter", "{{cloze:Text}}", "Text"},
		{"text filter", "{{text:Back}}", "Back"},
		{"conditional section", "{{#Add Reverse}}{{Back}}{{/Add Reverse}}", "Back"},
		{"inverted section", "{{^Empty}}{{First}}{{/Empty}}", "First"},
		{"front side is not a field", "{{FrontSide}}{{Back}}", "Back"},
		{"comment", "{{! note }}{{Front}}", "Front"},
		{"none", "<b>static</b>", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := frontFieldName(tt.tmpl); got != tt.want {
				t.Errorf("frontFieldName(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	values := map[string]string{
		"front": "question",
		"back":  "answer",
		"empty": "",
	}
	tests := []struct {
		name string
		tmpl string
		want string
	}{
		{"fields", "{{Front}} / {{Back}}", "question / answer"},
		{"front side", "{{FrontSide}}<hr>{{Back}}", "<hr>answer"},
		{"present section", "{{#Back}}{{Back}}{{/Back}}", "answer"},
		{"missing section", "{{#Empty}}hidden{{/Empty}}", ""},
		{"inverted section", "{{^Empty}}shown{{/Empty}}", "shown"},
		{"nested sections", "{{#Back}}{{#Empty}}a{{/Empty}}{{Back}}{{/Back}}", "answer"},
		{"unknown field", "{{Missing}}", ""},
		{"filter", "{{text:Back}}", "answer"},
		{"comment", "a{{! ignore }}b", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderTemplate(tt.tmpl, values); got != tt.want {
				t.Errorf("renderTemplate(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestPlainText(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      string
		wantMedia []string
	}{
		{
			name: "tags and entities",
			raw:  "<b>Q</b>&nbsp;one &amp; two",
			want: "Q one & two",
		},
		{
			name: "block tags break lines",
			raw:  "<div>one</div><div>two</div>",
			want: "one\ntwo",
		},
		{
			name:      "image keeps its name",
			raw:       `a <img src="diagram.png"> b`,
			want:      "a [img: diagram.png] b",
			wantMedia: []string{"diagram.png"},
		},
		{
			name:      "sound keeps its name",
			raw:       "a[sound:foo.mp3]b",
			want:      "a[sound: foo.mp3]b",
			wantMedia: []string{"foo.mp3"},
		},
		{
			name: "style is not text",
			raw:  "<style>.card { color: red }</style>hello",
			want: "hello",
		},
		{
			name: "comment disappears",
			raw:  "a<!-- hidden -->b",
			want: "ab",
		},
		{
			name: "escaped markup stays literal",
			raw:  "&lt;b&gt;not bold&lt;/b&gt;",
			want: "<b>not bold</b>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var media []string
			if got := normalizeLines(plainText(tt.raw, &media)); got != tt.want {
				t.Errorf("plainText(%q) = %q, want %q", tt.raw, got, tt.want)
			}
			if !reflect.DeepEqual(media, tt.wantMedia) {
				t.Errorf("media = %v, want %v", media, tt.wantMedia)
			}
		})
	}
}

func TestClozeMarkup(t *testing.T) {
	const text = "{{c1::Paris::city}} is in {{c2::France}}"
	if got := clozeUnwrap(text); got != "Paris is in France" {
		t.Errorf("clozeUnwrap = %q", got)
	}
	tests := []struct {
		ord  int
		want string
	}{
		{1, "[...] is in France"},
		{2, "Paris is in [...]"},
		{3, "Paris is in France"},
	}
	for _, tt := range tests {
		if got := maskCloze(text, tt.ord); got != tt.want {
			t.Errorf("maskCloze(c%d) = %q, want %q", tt.ord, got, tt.want)
		}
	}
}
