package notes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// tagFilterBar is the row above the notes: every tag the library carries, with
// how many notes hold it. Choosing one filters the list; choosing it again — or
// the × — puts the whole library back.
func (s *State) tagFilterBar(c *ui.Context, tk design.Tokens, d Deps) {
	tags := d.Tags()
	if len(tags) == 0 {
		return
	}
	msgs := locales.In(c.Locale())
	names := make([]string, 0, len(tags))
	for tag := range tags {
		names = append(names, tag)
	}
	sort.Strings(names)

	ui.ScrollHorizontal(c).Children(func() {
		ui.Row(c).Gap(tk.SpaceXS).Padding(tk.SpaceXS, 0).Children(func() {
			if s.TagFilter != "" {
				ui.Button(c, msgs.ClearFilter()).MinHeight(tk.Control - tk.SpaceS).
					OnClick(func() { s.TagFilter = "" })
			}
			for _, tag := range names {
				chosen := s.TagFilter == tag
				label := "#" + tag
				chip := ui.ButtonBase(c.Key(label)).
					Padding(tk.SpaceXS, tk.SpaceS).MinHeight(tk.Control-tk.SpaceS).
					Radius(tk.RadiusControl).Label(label).
					Border(tk.Hairline, tk.Border)
				if chosen {
					chip.Background(tk.Surface.Mix(tk.Accent, 0.16)).
						Border(tk.Hairline, tk.Accent)
				}
				chip.OnClick(func() {
					if s.TagFilter == tag {
						s.TagFilter = ""
						return
					}
					s.TagFilter = tag
				})
				chip.Children(func() {
					ui.Text(c, label).FontSize(tk.TypeSmall).TextColor(tk.Text)
					if n := tags[tag]; n > 1 {
						ui.Text(c, fmt.Sprint(n)).FontSize(tk.TypeSmall).TextColor(tk.TextMuted)
					}
				})
			}
		})
	})
}

// hasTag reports whether a note carries one of the tags.
func hasTag(tags []string, tag string) bool {
	for _, got := range tags {
		if got == tag {
			return true
		}
	}
	return false
}

// notesList is the source list of notes: today at the top, then every note
// newest first, with how many cards each one feeds.
func (s *State) notesList(c *ui.Context, tk design.Tokens, d Deps) {
	msgs := locales.In(c.Locale())
	library := d.Notes()
	ui.Column(c).Grow(1).MinWidth(0).Gap(tk.SpaceS).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Padding(tk.SpaceS, tk.SpaceM).Children(func() {
			ui.SearchField(c, &s.Query).Placeholder(msgs.NotesSearch()).Label(msgs.NotesSearch()).Grow(1)
			// A template only shows its door when there is one to start from:
			// with no templates in the library the row is the plain new-note
			// button, exactly as before (#154). Templates is a test-only nil
			// when the harness supplies no library at all.
			templates := []core.Note(nil)
			if d.Templates != nil {
				templates = d.Templates()
			}
			if len(templates) > 0 {
				ui.MenuButton(c, msgs.NewFromTemplate(), func(menu *ui.Menu) {
					menu.MaxWidth(360)
					for _, template := range templates {
						template := template
						if menu.Item(template.Title).Chosen() {
							d.NewFromTemplate(c, template.ID)
						}
					}
				}).Label(msgs.NewFromTemplate()).Tooltip(msgs.NewFromTemplate())
			}
			ui.Button(c, "").Label(msgs.NewNote()).Tooltip(msgs.NewNote()).
				Children(func() { tk.Icon(c, "plus", design.IconSmall) }).
				OnClick(func() { d.NewNote(c) })
		})
		// The tags are their own row: inside a Sidebar a click belongs to the
		// list's selection, not to a button.
		s.tagFilterBar(c, tk, d)
		ui.Sidebar(c, &s.Selected, func() {
			ui.SidebarItem(c, "today", design.Icon("layers"), msgs.Today()).
				Children(func() { ui.Badge(c, fmt.Sprint(d.Due())) }).
				OnClick(func() { d.OpenReview() })
			ui.SidebarSection(c, msgs.Notebook(), nil, func() {
				for _, n := range library {
					if s.TagFilter != "" && !hasTag(n.Tags, s.TagFilter) {
						continue
					}
					if !s.noteMatches(n.Note) {
						continue
					}
					item := ui.SidebarItem(c, string(n.ID), design.Icon("file-text"), n.Title)
					if n.Cards > 0 {
						item.Children(func() { ui.Badge(c, fmt.Sprint(n.Cards)) })
					}
				}
			})
		}).Grow(1)
	})
}

// noteMatches reports whether the sidebar's query keeps a note: its title or
// the first line of its text, as the list reads it.
func (s *State) noteMatches(n core.Note) bool {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(n.Title), query) {
		return true
	}
	return strings.Contains(strings.ToLower(noteSummary(n)), query)
}

// noteSummary is the line under a note's title: its first paragraph, flattened
// to one line.
func noteSummary(n core.Note) string {
	for _, block := range n.Blocks {
		text := strings.Join(strings.Fields(block.Text), " ")
		if text != "" {
			return text
		}
	}
	return ""
}
