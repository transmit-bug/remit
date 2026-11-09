// Package today draws the Today screen: one page that answers what to do today,
// how long it takes and where to start. The day's queue is a mix of the notes
// due to be read, the notes read but not yet excerpted and the cards due, in
// that order, priced against the day's budget; beside it sit the note the day
// is written in, the queues' counts, the sections plugins contribute and what
// tomorrow holds.
//
// It is a leaf: it reads State and Deps, draws, and reports what the user chose
// through callbacks (ADR 0004, ADR 0009). Nothing here reads the store, a clock
// or the disk — the app builds the day before the frame draws it — and nothing
// here imports package ui or a sibling screen.
package today

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

const (
	// rowHeight is a queue row: two lines of text beside the kind's mark, twice
	// a control tall so that the row is a comfortable target.
	rowHeight = design.ControlHeight * 2
	// excerptRows is how many read-but-not-excerpted lines the reminder lists:
	// enough to nag, few enough to stay a reminder.
	excerptRows = 3
	// skeletonRows is how many bars the loading page holds.
	skeletonRows = 3
	// budgetWarning and budgetDanger are where the day's meter changes tone:
	// four fifths of the budget, and the first minute past it. The danger line
	// sits just above a full budget so that a day spent exactly to its budget
	// still reads as a warning rather than as a spent one.
	budgetWarning = 0.8
	budgetDanger  = 1.01
)

// View draws the page: its head, the budget, the daily note, then whichever of
// the loading skeleton, the failure, the empty day or the queue the last load
// left, with the plugins' sections and tomorrow under it.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	day := d.Today()

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.PaperWidth).Gap(tk.SpaceXL).Children(func() {
				head(c, tk, msgs, s, day, d)
				budgetCard(c, tk, msgs, day)
				dailyRow(c, tk, msgs, s, d)
				switch {
				case s.Err != "":
					failed(c, tk, msgs, s.Err, d)
				case !s.Loaded:
					skeleton(c, tk, msgs)
				case len(day.Entries) == 0:
					empty(c, tk, msgs, d)
				default:
					counts(c, tk, msgs, s, day.Entries)
					queueSection(c, s, tk, msgs, day.Entries, d)
					excerptSection(c, tk, msgs, day.Entries, d)
				}
				// What does not depend on the day's own queue is still worth
				// drawing on an empty day, and never over a failed or unloaded
				// one, which would be a page pretending to have loaded.
				if s.Loaded && s.Err == "" {
					widgetSections(c, tk, d)
					tomorrowLine(c, tk, msgs, d)
				}
			})
		})
	})
}

// head is the page's name, the date and the day's budget, and the one command
// button the page carries: the palette, where the day's actions live.
func head(c *ui.Context, tk design.Tokens, msgs locales.Messages, s *State, day queue.Today, d Deps) {
	ui.Row(c).FillWidth().AlignItems(ui.End).Gap(tk.SpaceM).Children(func() {
		ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, msgs.TodayTitle()).FontSize(tk.TypeH1).Bold()
			ui.Text(c, msgs.TodayDay(s.Day, day.Budget)).
				FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		})
		ui.Button(c, msgs.TodayCommands()).MinHeight(tk.Control).OnClick(d.OpenPalette)
	})
}

// budgetCard is what the day costs before it is spent: the estimate in the
// display face, what it is made of, and a meter of it against the budget. It is
// a report, not a control, so it takes no click.
func budgetCard(c *ui.Context, tk design.Tokens, msgs locales.Messages, day queue.Today) {
	kit.Card(c, func() {
		ui.Column(c).FillWidth().Padding(tk.SpaceL).Gap(tk.SpaceM).Children(func() {
			ui.Text(c, msgs.TodayMinutes(day.TotalMinutes)).FontSize(tk.TypeH1).Bold()
			ui.Text(c, msgs.TodayBudgetCaption(day.ReadMinutes, day.ReviewMinutes)).
				FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			// A library with no budget shows what the day costs and no meter:
			// a bar needs a width to be measured against.
			if day.Budget > 0 {
				ui.Meter(c, float64(day.TotalMinutes)/float64(day.Budget), 0, 1,
					&ui.MeterLevels{Warning: budgetWarning, Critical: budgetDanger},
				).Label(msgs.TodayBudget())
			}
		})
	}).FillWidth()
}

// dailyRow is today's note: its date, what it last said, and the way in. It is
// a row in the day rather than a page of its own, so it is a button the width of
// the paper.
func dailyRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, s *State, d Deps) {
	title := msgs.TodayDaily(s.Day)
	summary := msgs.TodayDailyNone()
	if note, ok := d.Daily(); ok {
		summary = msgs.TodayDailyEmpty()
		if text := firstLine(note); text != "" {
			summary = text
		}
	}
	ui.Button(c, "").FillWidth().MinHeight(rowHeight).Padding(tk.SpaceS, tk.SpaceM).
		Label(title).Radius(tk.RadiusCard).Border(tk.Hairline, tk.Border).
		Children(func() {
			ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(tk.SpaceM).Children(func() {
				tk.Icon(c, "notebook-pen", design.IconSmall).TextColor(tk.TextMuted)
				ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
					ui.Text(c, title).FontSize(tk.TypeUI).SingleLine()
					ui.Text(c, summary).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
				})
				ui.Text(c, msgs.TodayOpen()).FontSize(tk.TypeUI).TextColor(tk.Accent)
			})
		}).OnClick(d.OpenDaily)
}

// counts is the day in three numbers: the notes to read, the cards to recall,
// and how many were recalled today. They are the queues' own numbers, so they
// move with the rail's badges and with the day's work.
func counts(c *ui.Context, tk design.Tokens, msgs locales.Messages, s *State, entries []queue.Entry) {
	reading, recall := 0, 0
	for _, entry := range entries {
		switch entry.Kind {
		case queue.EntryRead:
			reading++
		case queue.EntryReview:
			recall = entry.Cards
		}
	}
	ui.Row(c).FillWidth().Gap(tk.SpaceXL).Children(func() {
		stat(c, tk, msgs.TodayStatReading(), reading)
		stat(c, tk, msgs.TodayStatRecall(), recall)
		stat(c, tk, msgs.TodayStatRecalled(), s.Recalled)
	})
}

// stat is one of the three, spread across the paper.
func stat(c *ui.Context, tk design.Tokens, label string, n int) {
	ui.Column(c).Grow(1).Children(func() { kit.Stat(c, strconv.Itoa(n), label) })
}

// queueSection is the day itself, one line per sitting, in the order they are
// meant to be worked: reading before recall. Its Start button is the first line.
func queueSection(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, entries []queue.Entry, d Deps) {
	s.list.Key = entryKey(entries)
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		kit.SectionHead(c, tk, msgs.TodayQueue(), msgs.TodayQueueHint(), func() {
			ui.Button(c, msgs.TodayStart()).MinHeight(tk.Control).
				OnClick(func() { open(entries[0], d) })
		})
		ui.Divider(c)
		ui.List(c, &s.list, len(entries), func(row int) {
			rowView(c, tk, msgs, entries[row], d)
		}).Label(msgs.TodayQueue())
	})
}

// rowView is one line of the queue: what kind of sitting it is, what it is
// called, how far into it the reader is, what it costs, and — past the day's
// budget — that it is being postponed rather than dropped.
func rowView(c *ui.Context, tk design.Tokens, msgs locales.Messages, entry queue.Entry, d Deps) {
	title := entryTitle(msgs, entry)
	ui.Button(c, "").FillWidth().MinHeight(rowHeight).
		Padding(tk.SpaceM-tk.SpaceXS, tk.SpaceS).Radius(0).
		BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border).
		Label(title).Children(func() {
		ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(tk.SpaceM).Children(func() {
			kindMark(c, tk, entry.Kind)
			ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
					ui.Text(c, title).FontSize(tk.TypeUI).Bold().SingleLine()
					if entry.Over {
						kit.StateBadge(c, "clock", msgs.TodayOverBudget(), tk.Warning)
					}
				})
				ui.Text(c, entryMeta(msgs, entry)).
					FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
			})
			ui.Text(c, msgs.TodayMinutesTail(entry.Minutes)).
				FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		})
	}).OnClick(func() { open(entry, d) })
}

// kindMark is what a line is: a square naming the sitting — reading, an excerpt
// owed, or recall. The excerpt's is dashed, as the thing not yet done.
func kindMark(c *ui.Context, tk design.Tokens, kind queue.EntryKind) {
	icon, ink := "book-open", tk.Accent
	mark := ui.Row(c).Width(tk.Control).Height(tk.Control).Center().Shrink(0).
		Radius(tk.RadiusControl)
	switch kind {
	case queue.EntryRead:
		mark.Background(tk.Accent.Alpha(0.12)).Border(tk.Hairline, tk.Accent.Alpha(0.35))
	case queue.EntryExcerpt:
		ink = tk.TextMuted
		mark.Border(tk.Hairline, tk.Border).BorderStyle(ui.BorderDashed)
	default:
		icon, ink = "layers", tk.TextMuted
		mark.Background(tk.Sunken)
	}
	mark.Children(func() { tk.Icon(c, icon, design.IconSmall).TextColor(ink) })
}

// excerptSection is what was read and not excerpted, as a reminder of its own:
// the queue's own excerpt lines, at most excerptRows of them, and nothing at all
// when the queue owes no excerpt.
func excerptSection(c *ui.Context, tk design.Tokens, msgs locales.Messages, entries []queue.Entry, d Deps) {
	var owed []queue.Entry
	for _, entry := range entries {
		if entry.Kind != queue.EntryExcerpt {
			continue
		}
		owed = append(owed, entry)
		if len(owed) == excerptRows {
			break
		}
	}
	if len(owed) == 0 {
		return
	}
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		kit.SectionHead(c, tk, msgs.TodayExcerpts(), msgs.TodayExcerptsHint(), nil)
		ui.Divider(c)
		ui.Column(c).Children(func() {
			for _, entry := range owed {
				entry := entry
				ui.Button(c, "").FillWidth().MinHeight(tk.Control*1.5).
					Padding(tk.SpaceXS, tk.SpaceS).Radius(0).
					BorderWidth(0, 0, tk.Hairline, 0).BorderColor(tk.Border).
					Label(entryTitle(msgs, entry)).
					Children(func() {
						ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(tk.SpaceM).Children(func() {
							tk.Icon(c, "quote", design.IconSmall).TextColor(tk.TextMuted)
							ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
								ui.Text(c, entryTitle(msgs, entry)).
									FontSize(tk.TypeUI).SingleLine()
								ui.Text(c, entryMeta(msgs, entry)).
									FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
							})
							ui.Text(c, msgs.TodayOpen()).
								FontSize(tk.TypeLabel).TextColor(tk.Accent)
						})
					}).OnClick(func() { open(entry, d) })
			}
		})
	})
}

// widgetSections is what the plugins contribute to the day: one section per
// plugin with items in the today.widget slot, each a card of its commands. A
// slot with no items draws neither a heading nor a button.
func widgetSections(c *ui.Context, tk design.Tokens, d Deps) {
	items := d.Widgets()
	if len(items) == 0 {
		return
	}
	var plugins []string
	byPlugin := make(map[string][]contrib.CommandRef, len(items))
	for _, item := range items {
		if _, seen := byPlugin[item.Plugin]; !seen {
			plugins = append(plugins, item.Plugin)
		}
		byPlugin[item.Plugin] = append(byPlugin[item.Plugin], item.CommandRef)
	}
	for _, plugin := range plugins {
		refs := byPlugin[plugin]
		ui.Column(c).Gap(tk.SpaceS).Children(func() {
			kit.SectionHead(c, tk, d.WidgetText(plugin, refs[0].Name), "", nil)
			kit.Card(c, func() {
				ui.Row(c).FillWidth().Padding(tk.SpaceL).Gap(tk.SpaceS).Children(func() {
					for _, ref := range refs {
						ref := ref
						ui.Button(c, d.WidgetText(ref.Plugin, string(ref.Command.Title))).
							MinHeight(tk.Control).
							OnClick(func() { d.RunWidget(ref) })
					}
				})
			}).FillWidth()
		})
	}
}

// tomorrowLine is what the day after holds, from the same forecast the
// statistics page plots: today's leftover is not a debt, it is tomorrow's work.
func tomorrowLine(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		kit.SectionHead(c, tk, msgs.TodayTomorrowTitle(), msgs.TodayTomorrow(d.Tomorrow()), nil)
	})
}

// failed is a day whose queue could not be built: what went wrong, and the one
// action that asks again. MyGo has no callout at v0.8.1 (the catalogue has no
// notice, banner or callout), so it is an icon and a line in the danger colour,
// as the plugin page and the window's library error draw the same thing.
func failed(c *ui.Context, tk design.Tokens, msgs locales.Messages, reason string, d Deps) {
	kit.Notice(c, "circle-alert", msgs.TodayLoadFailed(reason), tk.Danger, func() {
		ui.Button(c, msgs.TodayRetry()).MinHeight(tk.Control).OnClick(d.Reload)
	})
}

// skeleton is the page before its queue: three bars where the lines will be, so
// that a slow load is not read as an empty day.
func skeleton(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	ui.Column(c).Gap(tk.SpaceS).Label(msgs.TodayLoading()).Children(func() {
		for range skeletonRows {
			ui.Box(c).FillWidth().Height(rowHeight).
				Radius(tk.RadiusCard).Background(tk.Sunken)
		}
	})
}

// empty is a day with nothing to read and nothing due: one sentence, and the
// one action that gives the day something in it.
func empty(c *ui.Context, tk design.Tokens, msgs locales.Messages, d Deps) {
	kit.Empty(c, "inbox", msgs.TodayEmpty(), func() {
		ui.Button(c, msgs.TodayEmptyAction()).MinHeight(tk.Control).OnClick(d.NewNote)
	})
}

// open goes where a line says the work is: into the note at the paragraph the
// reader left, into an excerpt still owed, or into the recall queue.
func open(entry queue.Entry, d Deps) {
	switch entry.Kind {
	case queue.EntryRead:
		if entry.Position != "" {
			d.OpenReadingAt(entry.Note, entry.Position)
			return
		}
		d.OpenReading(entry.Note)
	case queue.EntryExcerpt:
		d.OpenReading(entry.Note)
	case queue.EntryReview:
		d.StartReview()
	}
}

// entryKey is a line's identity in the list, so that its focus and the list's
// place follow the note rather than the row's number.
func entryKey(entries []queue.Entry) func(int) any {
	return func(row int) any {
		if note := entries[row].Note; note != "" {
			return string(note)
		}
		return "review"
	}
}

// entryTitle is what a line is called: the note's title, the review's own name,
// or — for a note a reading entry outlived — its placeholder.
func entryTitle(msgs locales.Messages, entry queue.Entry) string {
	if entry.Kind == queue.EntryReview {
		return msgs.TodayReviewTitle()
	}
	if entry.Title == "" {
		return msgs.UntitledNote()
	}
	return entry.Title
}

// entryMeta is where the line stands: how far into the note the reader got, how
// many cards wait and how many of them are new.
func entryMeta(msgs locales.Messages, entry queue.Entry) string {
	switch entry.Kind {
	case queue.EntryRead:
		if entry.Read > 0 {
			return msgs.TodayParagraph(entry.Read, entry.Blocks)
		}
		return msgs.TodayNotStarted(entry.Blocks)
	case queue.EntryExcerpt:
		if entry.Read > 0 {
			return msgs.TodayExcerptMeta(entry.Read, entry.Blocks)
		}
		return msgs.TodayNotStarted(entry.Blocks)
	}
	return msgs.TodayReviewMeta(entry.Cards, entry.New)
}

// firstLine is a note's first words on one line, which is what the daily row
// says it last held.
func firstLine(note core.Note) string {
	for _, block := range note.Blocks {
		if text := strings.Join(strings.Fields(block.Text), " "); text != "" {
			return text
		}
	}
	return ""
}
