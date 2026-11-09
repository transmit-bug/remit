package notes

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// TagRow is one tag as the tags page shows it: how many notes carry it, and a
// couple of titles so that the row says what the tag is about.
type TagRow struct {
	Tag string
	// Count is how many notes carry the tag.
	Count int
	// Sample is the titles of up to two of them.
	Sample []string
}

// TagsView draws the tags page (S07): every tag the library carries, how many
// notes hold it, and what they are. Choosing one narrows the notes page to it.
func TagsView(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	var tags []TagRow
	if d.TagRows != nil {
		tags = d.TagRows()
	}

	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.ListWidth).Grow(1).Gap(tk.SpaceL).Children(func() {
			ui.Column(c).FillWidth().Gap(tk.SpaceXS).Children(func() {
				ui.Text(c, msgs.WriteTagsTitle()).FontSize(tk.TypeH1).Bold()
				ui.Text(c, msgs.WriteTagsHint("#tag")).
					FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			})
			if len(tags) == 0 {
				kit.Empty(c, "quote", msgs.WriteTagsEmpty(), func() {
					ui.Button(c, msgs.NewNote()).MinHeight(tk.Control).
						OnClick(func() { d.NewNote(c) })
				})
				return
			}
			s.tagList.Key = func(row int) any { return tags[row].Tag }
			s.tagList.Label = func(row int) string { return "#" + tags[row].Tag }
			ui.Divider(c)
			ui.List(c, &s.tagList, len(tags), func(row int) {
				tagRow(c, tk, msgs, tags[row], d)
			}).Grow(1).Label(msgs.WriteTagsTitle())
		})
	})
}

// tagRow is one tag: its name, the notes that carry it as its examples, and how
// many there are. The whole row opens the notes page narrowed to it.
func tagRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, tag TagRow, d Deps) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
		Padding(tk.SpaceM, tk.SpaceS).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border).Children(func() {
		open := ui.Button(c, "").Label("#"+tag.Tag).Grow(1).MinWidth(0).
			MinHeight(tk.Hit).Padding(0).Radius(0).
			Background(ui.Color{}).Border(0, tk.Border)
		if open.Clicked() && d.OpenTag != nil {
			d.OpenTag(tag.Tag)
		}
		open.Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).
				Gap(tk.SpaceM).Children(func() {
				ui.Text(c, "#").FontSize(tk.TypeUI).Bold().TextColor(tk.Accent)
				ui.Column(c).Grow(1).MinWidth(0).Gap(tk.SpaceXS).Children(func() {
					ui.Text(c, tag.Tag).FontSize(tk.TypeUI).SingleLine()
					if names := strings.Join(tag.Sample, " · "); names != "" {
						ui.Text(c, names).FontSize(tk.TypeLabel).
							TextColor(tk.TextMuted).SingleLine()
					}
				})
			})
		})
		ui.Text(c, msgs.WriteTagNotes(tag.Count)).
			FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
	})
}
