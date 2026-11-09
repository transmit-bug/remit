package recall

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// EventKind is what a history event records: a session on a note, or an answer
// on a card. The timeline colours its node by kind, so the two activities stay
// apart although one log holds them.
type EventKind uint8

const (
	// EventReading is a reading session on a note.
	EventReading EventKind = iota
	// EventCard is a card answered.
	EventCard
)

// HistoryEvent is one review on the timeline: when it happened, what it was on,
// the answer, and the interval it moved.
type HistoryEvent struct {
	At     time.Time
	Kind   EventKind
	Title  string // the note read, or what the card asked
	Note   string // the note a card lives in, empty for a reading session
	Rating schedule.Rating
	From   int // the interval the previous answer set, in days
	To     int // the interval this answer set, in days
	NoteID core.ID
	CardID core.ID
}

// DayRow is one day of the week's table: the day, how many answers it holds,
// and how many of them were Again.
type DayRow struct {
	Day     time.Time
	Reviews int
	Again   int
}

// Accuracy is the share of the day's answers that were not Again, and whether
// the day has an answer to divide by.
func (d DayRow) Accuracy() (float64, bool) {
	if d.Reviews == 0 {
		return 0, false
	}
	return float64(d.Reviews-d.Again) / float64(d.Reviews), true
}

// HistoryState is the history page's data, derived from the live log by the app
// before the frame: today's events, their grade split, and the week's table.
type HistoryState struct {
	Events  []HistoryEvent
	Grades  [4]int
	Total   int
	Again   int
	Minutes int
	Week    []DayRow

	list     ui.ListState
	week     ui.ListState
	Selected int
}

// NewHistory returns the history page on no chosen event.
func NewHistory() HistoryState { return HistoryState{Selected: -1} }

// HistoryDeps is what the page asks of the app: the events it derived and the
// way back to what one of them recorded.
type HistoryDeps struct {
	// Open shows the note a reading session was on, or the note behind a card.
	Open func(HistoryEvent)
}

// HistoryView draws the history page: today's four numbers, the grade split,
// today's timeline, and the week's table.
func HistoryView(c *ui.Context, s *HistoryState, d HistoryDeps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	open := -1

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.ListWidth).Gap(tk.SpaceXL).Children(func() {
				pageHead(c, tk, msgs.RecallHistoryTitle(), msgs.RecallHistoryBody(), -1)
				if s.Total == 0 && len(s.Events) == 0 {
					emptyHistory(c, tk, msgs)
					return
				}
				numbers(c, s, tk, msgs)
				grades(c, s, tk, msgs)
				timeline(c, s, tk, msgs, &open)
				weekTable(c, s, tk, msgs)
			})
		})
	})

	if open >= 0 && open < len(s.Events) {
		d.Open(s.Events[open])
	}
}

// numbers is today at a glance: how many answers, how long they took, how many
// were Again, and the share that was right.
func numbers(c *ui.Context, s *HistoryState, tk design.Tokens, msgs locales.Messages) {
	accuracy := "—"
	if s.Total > 0 {
		accuracy = percent(float64(s.Total-s.Again) / float64(s.Total))
	}
	ui.Row(c).FillWidth().Gap(tk.SpaceXL).Children(func() {
		kit.StatDetail(c, strconv.Itoa(s.Total), msgs.RecallHistoryReviewed(), "").Grow(1)
		kit.StatDetail(c, msgs.IntervalMinutes(s.Minutes), msgs.RecallHistoryMinutes(), "").Grow(1)
		kit.StatDetail(c, strconv.Itoa(s.Again), msgs.RecallHistoryAgain(), "").Grow(1)
		kit.StatDetail(c, accuracy, msgs.RecallHistoryAccuracy(), "").Grow(1)
	})
}

// grades is the four-grade split: one bar per rating, its count, and the share
// of the day it holds.
func grades(c *ui.Context, s *HistoryState, tk design.Tokens, msgs locales.Messages) {
	most := 0
	for _, count := range s.Grades {
		most = max(most, count)
	}
	ratings := []struct {
		label  string
		colour ui.Color
	}{
		{msgs.Again(), tk.ReviewAgain},
		{msgs.Hard(), tk.ReviewHard},
		{msgs.Good(), tk.ReviewGood},
		{msgs.Easy(), tk.ReviewEasy},
	}
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionTitle(c, tk, msgs.RecallHistoryGrades())
		for i, rating := range ratings {
			count := s.Grades[i]
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceM).Children(func() {
				ui.Text(c, rating.label).FontSize(tk.TypeUI).Width(tk.CountColumnWidth)
				gradeBar(c, tk, count, most, rating.colour)
				ui.Text(c, strconv.Itoa(count)).FontSize(tk.TypeLabel).
					TextColor(tk.TextMuted).Width(tk.CountColumnWidth)
			})
		}
	})
}

// gradeBar is one rating's bar, its width the share of the busiest grade. It is
// a Row of boxes rather than ui.Meter (one value coloured by its own level) or
// ui/chart (a plot for four numbers), checked against MyGo v0.8.1.
func gradeBar(c *ui.Context, tk design.Tokens, count, most int, colour ui.Color) {
	track := ui.Row(c).Grow(1).Height(tk.SpaceS).Radius(tk.RadiusBar).
		Background(tk.Sunken).Clip()
	track.Children(func() {
		if count <= 0 || most <= 0 {
			return
		}
		ui.Box(c).Grow(float32(count)).FillHeight().Background(colour)
		if rest := most - count; rest > 0 {
			ui.Box(c).Grow(float32(rest)).FillHeight()
		}
	})
}

// timeline is today's events in the order they happened: the time, a node
// coloured by kind, what was read or answered, and the schedule change.
func timeline(c *ui.Context, s *HistoryState, tk design.Tokens, msgs locales.Messages, open *int) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionTitle(c, tk, msgs.RecallHistoryTimeline())
		s.list.Selected = &s.Selected
		s.list.Key = func(row int) any {
			event := s.Events[row]
			return strconv.FormatInt(event.At.UnixNano(), 10) + string(event.CardID) + string(event.NoteID)
		}
		s.list.Label = func(row int) string { return eventLabel(s.Events[row], msgs) }
		list := ui.List(c, &s.list, len(s.Events), func(i int) {
			eventRow(c, tk, msgs, s.Events[i])
		}).Label(msgs.RecallHistoryTimeline())
		if list.Submitted() {
			*open = s.Selected
		}
	})
}

// eventRow is one event: the time it happened, the node that says what kind it
// was, what it was on, and the interval a card answer moved.
func eventRow(c *ui.Context, tk design.Tokens, msgs locales.Messages, event HistoryEvent) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceM).Padding(tk.SpaceXS, 0).Children(func() {
		ui.Text(c, msgs.RecallHistoryAt(event.At)).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted).Width(tk.CountColumnWidth)
		colour := tk.Accent
		if event.Kind == EventCard {
			colour = tk.ReviewGood
		}
		ui.Box(c).Width(tk.SpaceS).Height(tk.SpaceS).Radius(tk.SpaceS / 2).Background(colour)
		ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, event.Title).FontSize(tk.TypeUI).SingleLine()
			if event.Note != "" {
				ui.Text(c, event.Note).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
			}
		})
		if event.Kind == EventCard {
			ui.Text(c, ratingLabel(msgs, event.Rating)).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted)
			if event.To > 0 {
				ui.Text(c, msgs.RecallHistoryInterval(event.From, event.To)).
					FontSize(tk.TypeLabel).TextColor(tk.TextFaint)
			}
		}
	})
}

// ratingLabel names one grade.
func ratingLabel(msgs locales.Messages, rating schedule.Rating) string {
	switch rating {
	case schedule.Again:
		return msgs.Again()
	case schedule.Hard:
		return msgs.Hard()
	case schedule.Good:
		return msgs.Good()
	case schedule.Easy:
		return msgs.Easy()
	}
	return ""
}

// eventLabel names an event for the list's keyboard and screen reader.
func eventLabel(event HistoryEvent, msgs locales.Messages) string {
	if event.Kind == EventReading {
		return msgs.RecallHistoryReadLabel(event.Title)
	}
	return msgs.RecallHistoryCardLabel(event.Title, ratingLabel(msgs, event.Rating))
}

// weekTable is the week so far: one row a day, its answers and its accuracy.
func weekTable(c *ui.Context, s *HistoryState, tk design.Tokens, msgs locales.Messages) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		sectionTitle(c, tk, msgs.RecallHistoryWeek())
		columns := []ui.TableColumn{
			{Title: msgs.RecallHistoryDay(), ID: "day"},
			{Title: msgs.RecallHistoryReviewed(), ID: "reviews", Width: tk.CountColumnWidth, Align: ui.End},
			{Title: msgs.RecallHistoryAgain(), ID: "again", Width: tk.CountColumnWidth, Align: ui.End},
			{Title: msgs.RecallHistoryAccuracy(), ID: "accuracy", Width: tk.CountColumnWidth, Align: ui.End},
		}
		ui.Table(c, &s.week, columns, len(s.Week), func(row, col int) {
			if row < 0 || row >= len(s.Week) {
				return
			}
			day := s.Week[row]
			switch col {
			case 0:
				ui.Text(c, msgs.RecallHistoryWeekday(day.Day)).SingleLine()
			case 1:
				ui.Text(c, strconv.Itoa(day.Reviews)).TextColor(tk.TextMuted)
			case 2:
				ui.Text(c, strconv.Itoa(day.Again)).TextColor(tk.TextMuted)
			case 3:
				accuracy := "—"
				if rate, ok := day.Accuracy(); ok {
					accuracy = percent(rate)
				}
				ui.Text(c, accuracy).TextColor(tk.TextMuted)
			}
		}).Label(msgs.RecallHistoryWeek())
	})
}

// sectionTitle is one section's heading on the page.
func sectionTitle(c *ui.Context, tk design.Tokens, title string) {
	kit.SectionHead(c, tk, title, "", nil)
}

// emptyHistory is a day with nothing recorded: one sentence and what fills it.
func emptyHistory(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	kit.Empty(c, "history", msgs.RecallHistoryEmpty(), func() {
		ui.Text(c, msgs.RecallHistoryEmptyBody()).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted).MaxWidth(tk.SummaryWidth)
	})
}

// percent prints a share as a whole percentage.
func percent(rate float64) string {
	return strconv.Itoa(int(rate*100+0.5)) + "%"
}
