package notes

import (
	"math"
	"strings"
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
	// The row menu's two writes: a rename typed in, and a delete that says what
	// it costs before it does it. Both are drawn over the page, so a row's menu
	// closes and its decision is asked here.
	s.renameDialog(c, tk, msgs, d)
	s.deleteDialog(c, msgs, d)

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
	ui.Divider(c)
	ui.List(c, &s.list, len(rows), func(row int) {
		listRow(c, s, tk, msgs, rows[row], d)
	}).Grow(1).Label(msgs.ModeNotes())
}

// listRow is one note: its title with the excerpt badge, the line that opens it,
// and — on the quiet end of the row — where the reader left it, the cards it
// feeds, when it was last written, and the two actions that ask for more than
// opening it.
//
// The two actions stay drawn, quiet until the row is under the pointer: a
// control that exists only under the pointer is out of the keyboard's reach.
func listRow(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, row Row, d Deps) {
	func() {
		// A row is a line of the list, not a card: the hairline is what
		// separates it from its neighbours, and the hover only tints the line.
		face := ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
			Padding(tk.SpaceM, tk.SpaceS).
			BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border)
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
			read := ui.Button(c, "").Label(msgs.WriteRowRead()+" "+row.Title).
				MinHeight(tk.Control).Tooltip(msgs.WriteRowRead()).
				Border(0, tk.Border).TextColor(actionInk(face, tk))
			read.Children(func() { tk.Icon(c, "book-open", design.IconSmall) })
			if read.Clicked() && d.OpenNoteRead != nil {
				d.OpenNoteRead(row.ID)
			}
			menu := ui.ButtonBase(c).Label(msgs.WriteRowActions() + " " + row.Title).
				MinHeight(tk.Control).MinWidth(tk.Control).Justify(ui.Center).
				Radius(tk.RadiusControl).TextColor(actionInk(face, tk))
			menu.Children(func() { tk.Icon(c, "ellipsis", design.IconSmall) })
			menu.PopoverMenuButton(func(m *ui.Menu) {
				if m.Item(msgs.WriteRowRead()).Chosen() && d.OpenNoteRead != nil {
					d.OpenNoteRead(row.ID)
				}
				if m.Item(msgs.WriteRowQueue()).Chosen() && d.QueueReading != nil {
					d.QueueReading(c, row.ID)
				}
				m.Separator()
				if m.Item(msgs.WriteRowRename()).Chosen() {
					s.beginRename(c, row)
				}
				if m.Item(msgs.WriteRowDelete()).Danger().Chosen() {
					s.deleteNote, s.deleteCards = row.ID, row.Cards
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

// actionInk is the colour of a row's own controls: drawn at all times, so the
// keyboard can reach them, but quiet enough that the row's title is what reads
// until the pointer is on it.
func actionInk(face ui.Element, tk design.Tokens) ui.Color {
	if face.Hovered() {
		return tk.TextMuted
	}
	return tk.TextFaint
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

// beginRename asks for the note's title to be typed again, and points the
// dialog's field at the next frame. The title lives in the note rather than in
// its row — the row's name is drawn from it — so the edit is a dialog over the
// list, not a field in the row. ui.EditableText renames a row's own label, and
// a note's title is not one: it is the note's first line.
func (s *State) beginRename(c *ui.Context, row Row) {
	s.renameNote, s.renameText = row.ID, row.Title
	s.renameField.Focus(c)
}

// renameDialog is the note's new title, asked for over the list. Enter keeps it,
// Escape and the backdrop leave the note as it was.
func (s *State) renameDialog(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps) {
	if s.renameNote == "" {
		return
	}
	open := true
	renamed := false
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.Width(tk.SummaryWidth).Padding(tk.SpaceL).Gap(tk.SpaceM))
		panel.Label(msgs.RenameNoteTitle())
		panel.Children(func() {
			ui.Text(c, msgs.RenameNoteTitle()).FontSize(tk.TypeH2).Bold()
			field := ui.TextInput(c, &s.renameText).Label(msgs.RenameNoteField()).
				Bind(&s.renameField)
			// A note with no title is not a name: the action waits, and the line
			// beside it says what it waits for.
			empty := strings.TrimSpace(s.renameText) == ""
			ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
				if empty {
					ui.Text(c, msgs.RenameNoteEmpty()).FontSize(tk.TypeLabel).
						TextColor(tk.TextMuted).Grow(1)
				} else {
					ui.Spacer(c)
				}
				ui.Button(c, msgs.Cancel()).MinHeight(tk.Control).
					OnClick(func() { open = false })
				ui.PrimaryButton(c, msgs.RenameNoteSave()).MinHeight(tk.Control).
					Disabled(empty).
					OnClick(func() {
						renamed = true
						open = false
					})
			})
			if field.Submitted() && !empty {
				renamed = true
				open = false
			}
		})
	})
	if renamed && d.RenameNote != nil {
		d.RenameNote(c, s.renameNote, s.renameText)
	}
	if !open || renamed {
		s.renameNote, s.renameText = "", ""
	}
}

// deleteDialog asks before a note goes, and names what it costs: the cards the
// note fed become lost, and their reviews stay in the log. The count is the
// row's own, so the question is about the note the user chose.
func (s *State) deleteDialog(c *ui.Context, msgs locales.Messages, d Deps) {
	if s.deleteNote == "" {
		return
	}
	open := true
	switch ui.AlertDialog(c, &open, msgs.DeleteNoteTitle(), msgs.DeleteNoteWarning(s.deleteCards),
		msgs.Cancel(), msgs.WriteRowDelete()) {
	case 0, ui.AlertDismissed:
		s.deleteNote, s.deleteCards = "", 0
	case 1:
		id := s.deleteNote
		s.deleteNote, s.deleteCards = "", 0
		if d.DeleteNote != nil {
			d.DeleteNote(c, id)
		}
	}
}
