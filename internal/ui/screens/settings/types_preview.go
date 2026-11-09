package settings

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/transmit-bug/remit/internal/core/template"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// typePreview renders both faces of the draft through the host template engine:
// the same Compile and Render the review path calls, so what the page shows is
// what the next card will. Every field stands for its own name as the sample
// value, so the preview says what the template does rather than inventing
// content.
func typePreview(c *ui.Context, s *State, msgs locales.Messages, tk design.Tokens) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, msgs.TypePreview()).FontSize(tk.TypeUI).Bold()
		previewFace(c, s, msgs, tk, msgs.TypePreviewFront(), s.Draft.Front, template.SideFront, &s.PreviewFront)
		previewFace(c, s, msgs, tk, msgs.TypePreviewBack(), s.Draft.Back, template.SideBack, &s.PreviewBack)
	})
}

// previewFace is one side of the preview: the face's label and the rendered
// markdown, or the dialect's refusal with the offset it points at.
func previewFace(c *ui.Context, s *State, msgs locales.Messages, tk design.Tokens, label, src string, side template.Side, opts **markdown.Options) {
	ui.Text(c, label).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	text, err := renderPreview(src, s.Draft.Fields, side)
	if err != nil {
		ui.Text(c, msgs.TypeTemplateBroken(err.Error())).FontSize(tk.TypeUI).TextColor(tk.Danger)
		return
	}
	if strings.TrimSpace(text) == "" {
		muted(c, tk, "—")
		return
	}
	if *opts == nil {
		*opts = &markdown.Options{}
	}
	markdown.View(c, text, *opts)
}

// renderPreview renders one template with the field list as its sample values.
func renderPreview(src string, fields []string, side template.Side) (string, error) {
	tpl, err := template.Compile(src)
	if err != nil {
		return "", err
	}
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			values[field] = field
		}
	}
	out, err := tpl.Render(template.Input{Fields: values, Side: side})
	if err != nil {
		return "", err
	}
	return out.Markdown, nil
}
