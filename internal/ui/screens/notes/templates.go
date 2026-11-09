package notes

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// TemplatesView draws the templates page (S09): the notes tagged as templates,
// each one the start of a new note. A template is a note, and a copy strips the
// marker and mints new block IDs, so one template starts as many notes as it is
// asked for (#154).
func TemplatesView(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	var templates []core.Note
	if d.Templates != nil {
		templates = d.Templates()
	}

	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.ListWidth).Grow(1).Gap(tk.SpaceL).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.End).Gap(tk.SpaceM).Children(func() {
				ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
					ui.Text(c, msgs.WriteTemplatesTitle()).FontSize(tk.TypeH1).Bold()
					ui.Text(c, msgs.WriteTemplatesHint("#"+core.TemplateTag)).
						FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
				})
				if d.NewNote != nil {
					ui.Button(c, msgs.WriteTemplateBlank()).MinHeight(tk.Control).
						OnClick(func() { d.NewNote(c) })
				}
			})
			if len(templates) == 0 {
				kit.Empty(c, "file-text", msgs.WriteTemplatesEmpty(), func() {
					if d.NewNote == nil {
						return
					}
					ui.Button(c, msgs.NewNote()).MinHeight(tk.Control).
						OnClick(func() { d.NewNote(c) })
				})
				return
			}
			s.templateList.Key = func(row int) any { return templates[row].ID }
			s.templateList.Label = func(row int) string { return templates[row].Title }
			ui.Divider(c)
			ui.List(c, &s.templateList, len(templates), func(row int) {
				templateRow(c, tk, msgs, templates[row], d)
			}).Grow(1).Label(msgs.WriteTemplatesTitle())
		})
	})
}

// templateRow is one template: what it is called, how much it holds and the line
// it opens with, and the button that starts a note from it.
func templateRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, template core.Note, d Deps) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
		Padding(tk.SpaceM, tk.SpaceS).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border).Children(func() {
		open := ui.Button(c, "").Label(msgs.WriteTemplateNew()+" "+template.Title).
			Grow(1).MinHeight(tk.Hit).Padding(tk.SpaceS, tk.SpaceM)
		if open.Clicked() && d.NewFromTemplate != nil {
			d.NewFromTemplate(c, template.ID)
		}
		open.Children(func() {
			ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(tk.SpaceM).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Start).
					Gap(tk.SpaceXS).Children(func() {
					ui.Text(c, template.Title).FontSize(tk.TypeUI).SingleLine()
					ui.Text(c, templateLine(msgs, template)).FontSize(tk.TypeLabel).
						TextColor(tk.TextMuted).SingleLine()
				})
				ui.Text(c, msgs.WriteTemplateNew()).FontSize(tk.TypeUI).
					TextColor(tk.Accent).SingleLine()
			})
		})
	})
}

// templateLine is a template's row's second line: how many paragraphs it holds
// and the first one that says something, the marker block aside.
func templateLine(msgs locales.Messages, template core.Note) string {
	line := msgs.WriteTemplateBlocks(len(template.Blocks))
	for _, block := range template.Blocks {
		if text := strings.Join(strings.Fields(block.Text), " "); text != "" &&
			!strings.Contains(text, "#"+core.TemplateTag) {
			return line + " · " + text
		}
	}
	return line
}
