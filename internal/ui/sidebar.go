package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
)

// tagFilterBar is the row above the notes: every tag the library carries, with
// how many notes hold it. Choosing one filters the list; choosing it again — or
// the × — puts the whole library back.
func (a *App) tagFilterBar(c *ui.Context, tk tokens) {
	if len(a.data.tags) == 0 {
		return
	}
	tags := make([]string, 0, len(a.data.tags))
	for tag := range a.data.tags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	ui.ScrollHorizontal(c).Children(func() {
		ui.Row(c).Gap(tk.spaceXS).Padding(tk.spaceXS, 0).Children(func() {
			if a.tagFilter != "" {
				ui.Button(c, m(c).ClearFilter()).MinHeight(tk.control - tk.spaceS).
					OnClick(func() { a.tagFilter = "" })
			}
			for _, tag := range tags {
				chosen := a.tagFilter == tag
				label := "#" + tag
				chip := ui.ButtonBase(c.Key(label)).
					Padding(tk.spaceXS, tk.spaceS).MinHeight(tk.control-tk.spaceS).
					Radius(tk.radiusControl).Label(label).
					Border(tk.hairline, tk.Border)
				if chosen {
					chip.Background(tk.Surface.Mix(tk.Accent, 0.16)).
						Border(tk.hairline, tk.Accent)
				}
				chip.OnClick(func() {
					if a.tagFilter == tag {
						a.tagFilter = ""
						return
					}
					a.tagFilter = tag
				})
				chip.Children(func() {
					ui.Text(c, label).FontSize(tk.typeSmall).TextColor(tk.Text)
					if n := a.data.tags[tag]; n > 1 {
						ui.Text(c, fmt.Sprint(n)).FontSize(tk.typeSmall).TextColor(tk.TextMuted)
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
func (a *App) notesList(c *ui.Context, tk tokens) {
	msgs := m(c)
	ui.Column(c).Grow(1).MinWidth(0).Gap(tk.spaceS).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.spaceS).Padding(tk.spaceS, tk.spaceM).Children(func() {
			ui.SearchField(c, &a.notesQuery).Placeholder(msgs.NotesSearch()).Label(msgs.NotesSearch()).Grow(1)
			ui.Button(c, "").Label(msgs.NewNote()).Tooltip(msgs.NewNote()).
				Children(func() { tk.icon(c, "plus", iconSmall) }).
				OnClick(func() { a.newNote(c) })
		})
		// The tags are their own row: inside a Sidebar a click belongs to the
		// list's selection, not to a button.
		a.tagFilterBar(c, tk)
		ui.Sidebar(c, &a.editor.selected, func() {
			ui.SidebarItem(c, "today", design.Icon("layers"), msgs.Today()).
				Children(func() { ui.Badge(c, fmt.Sprint(a.review.Remaining())) }).
				OnClick(func() { a.setMode(modeReview) })
			ui.SidebarSection(c, msgs.Notebook(), nil, func() {
				for _, n := range a.data.notes {
					if a.tagFilter != "" && !hasTag(a.data.noteTags[n.ID], a.tagFilter) {
						continue
					}
					if !a.noteMatches(n) {
						continue
					}
					item := ui.SidebarItem(c, string(n.ID), design.Icon("file-text"), n.Title)
					if cards := a.data.cardsInNote(n); cards > 0 {
						item.Children(func() { ui.Badge(c, fmt.Sprint(cards)) })
					}
				}
			})
		}).Grow(1)
	})
}

// noteMatches reports whether the sidebar's query keeps a note: its title or
// the first line of its text, as the list reads it.
func (a *App) noteMatches(n core.Note) bool {
	query := strings.ToLower(strings.TrimSpace(a.notesQuery))
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
