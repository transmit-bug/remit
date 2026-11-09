package notes

import (
	"math"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// Sort is the order the notes page lists the notes in. It is the screen's, not
// the store's: a reload keeps the order the user chose.
type Sort uint8

const (
	// SortUpdated lists the newest save first.
	SortUpdated Sort = iota
	// SortTitle lists by title.
	SortTitle
	// SortCards lists by how many cards a note feeds, most first.
	SortCards
)

// Filter is the notes page's three filters, which combine: a note the reader is
// in now and that feeds cards is what Reading and WithCards together keep.
type Filter struct {
	// Reading keeps the notes the reading queue holds now.
	Reading bool
	// HasCards keeps the notes at least one card renders from.
	HasCards bool
	// Excerpt keeps the notes that are excerpts.
	Excerpt bool
}

// Any reports whether a filter keeps fewer notes than the whole notebook.
func (f Filter) Any() bool { return f.Reading || f.HasCards || f.Excerpt }

// Row is one note as the notes page lists it: what it is called, the line that
// opens it, and where the reader left it. The app derives it from the store's
// query, so a page of rows costs no note read.
type Row struct {
	ID    core.ID
	Title string
	// Summary is the note's first paragraph pressed into one line.
	Summary string
	// Blocks is how many paragraphs the note holds.
	Blocks int
	// Cards is how many cards the note's blocks feed.
	Cards int
	// ReadingBlocks is the paragraph the reader reached, counted from one, and
	// zero for a note no session has read.
	ReadingBlocks int
	// Updated is when the note was last saved.
	Updated time.Time
	// Excerpt says the note is an excerpt: a block of it points into another
	// note.
	Excerpt bool
}

// ListView draws the notes page (S05): the library's notes, each with the line
// that opens it, the cards it feeds and where the reader left it, narrowed by a
// search, three filters, a tag and an order.
func ListView(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	rows, total := rowsOf(d)
	cards := 0
	for _, row := range rows {
		cards += row.Cards
	}

	reload := false
	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.ListWidth).Grow(1).Gap(tk.SpaceL).Children(func() {
			listHead(c, tk, msgs, total, cards, d)
			reload = listToolbar(c, s, tk, msgs)
			switch {
			case total == 0 && !s.narrowed():
				kit.Empty(c, "file-text", msgs.NoNotes(), func() {
					ui.Button(c, msgs.NewNote()).MinHeight(tk.Control).OnClick(func() { d.NewNote(c) })
				})
			case total == 0:
				kit.Empty(c, "search", msgs.WriteNoMatch(), func() {
					ui.Button(c, msgs.WriteClearFilters()).MinHeight(tk.Control).
						OnClick(func() { s.ClearFilters(); reload = true })
				})
			default:
				s.listRows(c, tk, msgs, rows, d)
			}
		})
	})
	// The query, a filter or an order changed: the rows are the store's answer,
	// so the app derives them again after the fields have taken the input.
	if reload && d.Reload != nil {
		d.Reload()
	}
}

// rowsOf is the page's rows and how many notes match; a screen drawn without an
// app behind it has neither.
func rowsOf(d Deps) ([]Row, int) {
	if d.Rows == nil {
		return nil, 0
	}
	return d.Rows()
}

// narrowed reports whether the page keeps fewer notes than the whole notebook.
func (s *State) narrowed() bool {
	return s.Query != "" || s.TagFilter != "" || s.Filter.Any()
}

// listHead is the page's name and what the list holds, and the one action that
// adds to it.
func listHead(c *ui.Context, tk design.Tokens, msgs locales.Messages, total, cards int, d Deps) {
	ui.Row(c).FillWidth().AlignItems(ui.End).Gap(tk.SpaceM).Children(func() {
		ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, msgs.ModeNotes()).FontSize(tk.TypeH1).Bold()
			ui.Text(c, msgs.WriteListCount(total, cards)+" · "+msgs.WritePlainText()).
				FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		})
		if d.NewNote != nil {
			ui.PrimaryButton(c, msgs.NewNote()).MinHeight(tk.Control).
				OnClick(func() { d.NewNote(c) })
		}
	})
}

// listToolbar is what narrows the list: the search, the three filters with
// their All, the tag the list is narrowed to, and the order. It reports whether
// the rows must be derived again.
func listToolbar(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages) bool {
	reload := false
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		field := ui.SearchField(c, &s.Query).
			Placeholder(msgs.NotesSearch()).Label(msgs.NotesSearch()).Grow(1)
		if field.Changed() {
			reload = true
		}
		ui.ToggleGroup(c, func() {
			all := !s.Filter.Any()
			if ui.Toggle(c, &all, msgs.WriteFilterAll()).MinHeight(tk.Control).Changed() && all {
				s.Filter = Filter{}
				reload = true
			}
			for _, toggle := range []struct {
				on    *bool
				label string
			}{
				{&s.Filter.Reading, msgs.WriteFilterReading()},
				{&s.Filter.HasCards, msgs.WriteFilterCards()},
				{&s.Filter.Excerpt, msgs.WriteFilterExcerpt()},
			} {
				if ui.Toggle(c, toggle.on, toggle.label).MinHeight(tk.Control).Changed() {
					reload = true
				}
			}
		}).Label(msgs.WriteFilterLabel())
		if s.TagFilter != "" {
			chip := ui.Button(c, "").Label(msgs.ClearFilter()).Tooltip(msgs.ClearFilter()).
				MinHeight(tk.Control - tk.SpaceXS)
			chip.Children(func() {
				ui.Text(c, "#"+s.TagFilter).FontSize(tk.TypeUI).SingleLine()
				tk.Icon(c, "x", design.IconSmall)
			})
			if chip.Clicked() {
				s.TagFilter = ""
				reload = true
			}
		}
		ui.PopoverMenuButton(c, msgs.WriteSort(), func(menu *ui.Menu) {
			for _, order := range []struct {
				sort  Sort
				label string
			}{
				{SortUpdated, msgs.WriteSortUpdated()},
				{SortTitle, msgs.WriteSortTitle()},
				{SortCards, msgs.WriteSortCards()},
			} {
				if menu.Item(order.label).Checked(s.Sort == order.sort).Chosen() {
					s.Sort = order.sort
					reload = true
				}
			}
		}).MinHeight(tk.Control).Tooltip(msgs.WriteSort())
	})
	return reload
}

// listRows is the list itself: one row per note, each opening the note, read
// whole, or added to the reading queue from the row's own controls.
func (s *State) listRows(c *ui.Context, tk design.Tokens, msgs locales.Messages, rows []Row, d Deps) {
	s.list.Key = func(row int) any { return rows[row].ID }
	s.list.Label = func(row int) string { return rows[row].Title }
	ui.List(c, &s.list, len(rows), func(row int) {
		listRow(c, tk, msgs, rows[row], d)
	}).Grow(1).Label(msgs.ModeNotes())
}

// listRow is one note: its title with the excerpt badge, the line that opens it,
// and — on the quiet end of the row — where the reader left it, the cards it
// feeds, when it was last written, and the two actions that ask for more than
// opening it.
//
// The two actions stay drawn, quiet until the row is under the pointer: a
// control that exists only under the pointer is out of the keyboard's reach.
func listRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, row Row, d Deps) {
	func() {
		face := ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
			Padding(tk.SpaceS, tk.SpaceM).Radius(tk.RadiusControl)
		if face.Hovered() {
			face.Background(tk.SurfaceHover)
		}
		face.Children(func() {
			open := ui.Button(c, "").Label(row.Title).Grow(1).MinWidth(0).
				MinHeight(tk.Hit).Padding(0).Radius(0).
				Background(ui.Color{}).Border(0, tk.Border)
			if open.Clicked() && d.OpenNote != nil {
				d.OpenNote(row.ID)
			}
			open.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Start).
					Gap(tk.SpaceXS).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
						ui.Text(c, row.Title).FontSize(tk.TypeUI).SingleLine()
						if row.Excerpt {
							ui.Badge(c, msgs.WriteExcerpt())
						}
					})
					ui.Text(c, summaryOf(msgs, row)).
						FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
				})
			})
			if row.ReadingBlocks > 0 {
				ui.Badge(c, msgs.WriteRowProgress(row.ReadingBlocks, row.Blocks)).
					Label(msgs.WriteRowProgress(row.ReadingBlocks, row.Blocks))
			}
			if row.Cards > 0 {
				ui.Badge(c, itoa(row.Cards)).Label(msgs.WriteCardsCount(row.Cards))
			}
			ui.Text(c, when(c, row.Updated, nowOf(d))).
				FontSize(tk.TypeLabel).TextColor(tk.TextFaint).SingleLine()
			read := ui.Button(c, "").Label(msgs.WriteRowRead() + " " + row.Title).
				MinHeight(tk.Control).Tooltip(msgs.WriteRowRead()).TextColor(tk.TextMuted)
			read.Children(func() { tk.Icon(c, "book-open", design.IconSmall) })
			if read.Clicked() && d.OpenNoteRead != nil {
				d.OpenNoteRead(row.ID)
			}
			menu := ui.ButtonBase(c).Label(msgs.WriteRowActions() + " " + row.Title).
				MinHeight(tk.Control).MinWidth(tk.Control).Justify(ui.Center).
				Radius(tk.RadiusControl).TextColor(tk.TextMuted)
			menu.Children(func() { tk.Icon(c, "ellipsis", design.IconSmall) })
			menu.PopoverMenuButton(func(m *ui.Menu) {
				if m.Item(msgs.WriteRowRead()).Chosen() && d.OpenNoteRead != nil {
					d.OpenNoteRead(row.ID)
				}
				if m.Item(msgs.WriteRowQueue()).Chosen() && d.QueueReading != nil {
					d.QueueReading(c, row.ID)
				}
				// A plugin's actions for a note close the menu, under the host's,
				// which is where the sidebar.note slot puts them (ui.md §4).
				items := rowItems(d, c)
				if len(items) == 0 {
					return
				}
				m.Separator()
				for _, item := range items {
					if m.Item(item.Label).Chosen() {
						item.Run(c)
					}
				}
			})
		})
	}()
}

// rowItems are the actions plugins contributed to a note row's menu. A screen
// drawn without an app behind it has none.
func rowItems(d Deps, c *ui.Context) []BlockItem {
	if d.RowItems == nil {
		return nil
	}
	return d.RowItems(c)
}

// summaryOf is the line under a row's title: the note's first paragraph, or the
// line that says it has none yet.
func summaryOf(msgs locales.Messages, row Row) string {
	if row.Summary == "" {
		return msgs.WriteRowNoSummary()
	}
	return row.Summary
}

// nowOf is the clock the rows' times are measured against.
func nowOf(d Deps) time.Time {
	if d.Now == nil {
		return time.Time{}
	}
	return d.Now()
}

// when is a note's last save as a reader reads it: today and yesterday by their
// words, anything older as a count of days, in the frame's language.
func when(c *ui.Context, at, now time.Time) string {
	if at.IsZero() || now.IsZero() {
		return ""
	}
	days := int(math.Round(now.Sub(at).Hours() / 24))
	if days < 0 {
		days = 0
	}
	return i18n.FormatRelativeTime(c.Locale(), float64(-days), i18n.RelativeDay,
		i18n.RelativeTimeOptions{Auto: true})
}
