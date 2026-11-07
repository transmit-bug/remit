// Package review is the review screen: one card at a time, its answer, and the
// four grades. It owns the session's queue order and draws what the queue says;
// the app owns the store, and Deps is the whole of what the screen asks of it.
//
// Nothing here reads the store, a clock or the disk, so the screen is driven in
// a test with fakes and one window size.
package review

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/internal/ui/maths"
	"github.com/transmit-bug/remit/locales"
)

// width caps the column: a comfortable measure, centred in the window with the
// notes list out of the way.
const width float32 = 680

// advance is how a graded card leaves and the next arrives: a short slide and
// fade, no celebration. The transition honours the desktop's reduce-motion
// setting by fading in place.
var advance = ui.ElementTransition{
	Duration: design.Grade.Duration,
	Ease:     design.Grade.Ease,
	Enter:    &ui.Motion{Y: design.GradeRise},
	Exit:     &ui.Motion{Y: -design.GradeRise},
}

// statusHeight is how tall the one line the screen keeps may be.
const statusHeight float32 = design.SpaceXL + design.SpaceXS

// Rating is one of the four answers a revealed card takes.
type Rating uint8

const (
	Again Rating = iota
	Hard
	Good
	Easy
)

// Schedule is the store's rating for this answer: the same four in the same
// order, counted from one.
func (r Rating) Schedule() schedule.Rating { return schedule.Rating(r) + 1 }

// Key is the keyboard key that grades this answer.
func (r Rating) Key() string { return [...]string{"1", "2", "3", "4"}[r] }

func (r Rating) label(msgs locales.Messages) string {
	switch r {
	case Again:
		return msgs.Again()
	case Hard:
		return msgs.Hard()
	case Good:
		return msgs.Good()
	default:
		return msgs.Easy()
	}
}

func (r Rating) colour(t *ui.Theme) ui.Color {
	return [...]ui.Color{t.Danger, t.Warning, t.Success, t.Accent}[r]
}

// Card is what the screen shows of a card: its sides as the library resolves
// them now, and how its source stands.
type Card struct {
	ID     core.ID
	Prompt string
	Answer string
	State  core.LinkState
	// Frozen is when a frozen card's snapshot was taken.
	Frozen time.Time
}

// Interval is what one answer would schedule: how long until the card comes
// back. The grade keys show it, so the cost of a grade is visible before it is
// taken.
type Interval struct {
	Rating Rating
	Wait   time.Duration
}

// Tone is how the completion summary carries itself: Quiet says it and stops;
// Celebrated arrives on the design language's panel motion, once.
type Tone uint8

const (
	Quiet Tone = iota
	Celebrated
)

// Summary is what a finished queue is worth saying. The numbers come from the
// app — today's answers, what tomorrow holds, how many were right — so that the
// screen words a decision it did not make.
type Summary struct {
	Reviewed    int
	DueTomorrow int
	Accuracy    float64
	Tone        Tone
	// Lines are what the scheduling plugin adds to the numbers, as markdown the
	// screen renders beneath its own wording.
	Lines []string
}

// Deps is what the screen asks the app for: the card to show, and what the user
// can do with it. Each runs on the UI thread; the app does the writing.
type Deps struct {
	Card func(core.ID) (Card, bool) // the card shown, false when it is gone
	// Preview is the four intervals for the card shown, when the schedule can
	// give them; false leaves the keys with their plain labels.
	Preview func() ([4]Interval, bool)
	// Summary is what a finished queue reports, when the app has a summary to
	// give; false falls back to the one quiet line.
	Summary func() (Summary, bool)
	Grade    func(core.ID, Rating) // record the answer and move on
	Undo     func()                // take the newest answer back
	Suspend  func(core.ID)         // out of the queue, history untouched
	Relink   func(core.ID)         // point a lost card at another block
	EditNote func(core.ID)         // open the note behind the card
	Home     func()                // leave for the notebook
}

// State is the queue and where the user is in it. What belongs in it is the
// store's answer (queue.Today); the order is the session's, so progress
// survives leaving review for notes, and undo can put the card just graded back
// at the head where the user left it.
type State struct {
	queue    []core.ID // the card shown, then those after it
	shown    core.ID   // the card revealed reports on
	revealed bool
	graded   []core.ID // cards graded this session, newest last
}

// Reset forgets the queue and the session's progress.
func (s *State) Reset() { s.queue, s.shown, s.revealed, s.graded = nil, "", false, nil }

// Sync takes the library's answer to what is due now: a card still due keeps its
// place, one no longer due leaves, and one that became due — a card made just
// now, a card returned by undo — joins the end. Membership is the store's; the
// order the user is working through stays.
func (s *State) Sync(due []core.ID) {
	live := make(map[core.ID]bool, len(due))
	for _, id := range due {
		live[id] = true
	}
	kept := make([]core.ID, 0, len(due))
	for _, id := range s.queue {
		if live[id] {
			kept = append(kept, id)
			delete(live, id)
		}
	}
	for _, id := range due {
		if live[id] {
			kept = append(kept, id)
			delete(live, id)
		}
	}
	s.queue = kept
}

// Progress is how many cards this session has graded, and how many it started
// with: what the status bar draws.
func (s *State) Progress() (done, total int) {
	done = len(s.graded)
	return done, done + len(s.queue)
}

// Remaining is how many cards are still to grade.
func (s *State) Remaining() int { return len(s.queue) }

// Current is the card on screen, if there is one.
func (s *State) Current() (core.ID, bool) {
	if len(s.queue) == 0 {
		return "", false
	}
	return s.queue[0], true
}

// Only makes id the whole queue, as opening one card from browse does.
func (s *State) Only(id core.ID) {
	s.queue = []core.ID{id}
}

// Reveal shows the card's answer.
func (s *State) Reveal() {
	if len(s.queue) > 0 {
		s.shown, s.revealed = s.queue[0], true
	}
}

// Answered reports whether the answer on screen is the card's.
func (s *State) Answered() bool {
	return s.revealed && len(s.queue) > 0 && s.shown == s.queue[0]
}

// Advance moves past the card on screen, remembering it when it was graded
// rather than suspended.
func (s *State) Advance(graded bool) {
	if len(s.queue) == 0 {
		return
	}
	if graded {
		s.graded = append(s.graded, s.queue[0])
	}
	s.queue = s.queue[1:]
	s.shown, s.revealed = "", false
}

// Last is the card the newest grade belongs to.
func (s *State) Last() (core.ID, bool) {
	if len(s.graded) == 0 {
		return "", false
	}
	return s.graded[len(s.graded)-1], true
}

// Restore puts a card back on screen with its answer shown, the way it was when
// it was graded.
func (s *State) Restore(id core.ID) {
	s.queue = append([]core.ID{id}, s.queue...)
	s.shown, s.revealed = id, true
}

// Show puts a card at the head of the queue with its answer still hidden, as a
// card that was just re-linked wants: the user sees the text it follows now,
// whether or not its due date is today.
func (s *State) Show(id core.ID) {
	s.queue = append([]core.ID{id}, s.queue...)
	s.shown, s.revealed = "", false
}

// Forget drops the grade undo has just cancelled.
func (s *State) Forget() {
	if len(s.graded) > 0 {
		s.graded = s.graded[:len(s.graded)-1]
	}
}

// View draws the screen: one card, centred.
func View(c *ui.Context, s *State, d Deps) {
	ui.Column(c).Fill().Children(func() {
		statusBar(c, s)
		// The card sits in the middle of what the status bar leaves, on the
		// smallest window with a tighter margin so the card keeps its room.
		pad := float32(design.SpaceXXL)
		if w, _ := c.Size(); w < 800 {
			pad = design.SpaceXL
		}
		ui.Column(c).Grow(1).Center().Padding(pad).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(width).Gap(design.SpaceL).Children(func() {
				id, ok := s.Current()
				var card Card
				if ok {
					card, ok = d.Card(id)
				}
				if !ok {
					empty(c, d)
					return
				}
				flipCard(c, s, d, card)
				if card.State != core.LinkLost && s.Answered() {
					grades(c, d, card.ID)
				}
				footer(c, s, d, card)
			})
		})
	})
}

// statusBar is the one line the taking-over screen keeps: how much is left, how
// far along the session is, and the keys that move it. It is never taller than
// statusHeight.
func statusBar(c *ui.Context, s *State) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	done, total := s.Progress()
	ui.Row(c).Height(statusHeight).Padding(design.SpaceXS, design.SpaceL).
		AlignItems(ui.Center).Gap(design.SpaceS).Children(func() {
		ui.Text(c, msgs.Remaining(s.Remaining())).
			FontSize(design.TypeSmall).TextColor(t.TextMuted)
		if total > 0 {
			ui.Progress(c, float64(done)/float64(total)).Grow(1)
		}
		ui.Spacer(c)
		ui.Row(c).Gap(design.SpaceXS).AlignItems(ui.Center).Children(func() {
			keys := []struct{ key, label string }{
				{"Space", msgs.ShowAnswer()},
				{"U", msgs.Undo()},
				{"S", msgs.Suspend()},
				{"E", msgs.EditNote()},
				{"Esc", msgs.ModeNotes()},
			}
			for _, k := range keys {
				kit.Keycap(c, k.key).Tooltip(k.label)
			}
		})
	})
}

// flipCard draws the card: its link state, the prompt, and the answer once
// revealed. It is a wrapper, not a copied widget: see RECONCILE.md.
//
// The lost branch is defensive: today's queue leaves a lost card out, because a
// queue promises cards that can be reviewed now. It holds if a card's source
// disappears between the queue being derived and the card being drawn, and a
// card opened from browse lands here too. Re-linking is the repair, and
// suspending — the one action that never touches history — is the way out.
func flipCard(c *ui.Context, s *State, d Deps, card Card) {
	msgs := locales.In(c.Locale())
	kit.StyleCard(c, ui.Column(c.Key(string(card.ID))).
		Padding(design.SpaceXL).Gap(design.SpaceL)).Transition(advance).
		Children(func() {
			switch card.State {
			case core.LinkLost:
				kit.StateBadge(c, "unlink", msgs.Lost(), c.Theme().Danger).AlignSelf(ui.Start)
				ui.Text(c, msgs.LostBody()).
					FontSize(design.TypeBody).TextColor(c.Theme().TextMuted)
				ui.Row(c).Gap(design.SpaceS).AlignSelf(ui.Start).Children(func() {
					ui.PrimaryButton(c, msgs.Relink()).MinHeight(design.ControlHeight).
						OnClick(func() { d.Relink(card.ID) })
					ui.Button(c, msgs.SuspendCard()).MinHeight(design.ControlHeight).
						OnClick(func() { d.Suspend(card.ID) })
				})
				return
			case core.LinkFrozen:
				ui.Row(c).AlignSelf(ui.Start).AlignItems(ui.Center).Gap(design.SpaceS).Children(func() {
					kit.StateBadge(c, "snowflake", msgs.Frozen(), c.Theme().Warning)
					ui.Text(c, msgs.SnapshotTaken(card.Frozen.Format("2 January 2006"))).
						FontSize(design.TypeSmall).TextColor(c.Theme().TextMuted)
				})
			}

			// A card's faces arrive already rendered: the type's templates
			// turned its fields into markdown, the host turns markdown into text.
			front, back := card.Prompt, card.Answer
			ui.Text(c, maths.Render(front)).FontSize(design.TypeHeading).Bold()
			ui.Divider(c)
			if !s.Answered() {
				ui.Text(c, msgs.RevealHint()).
					FontSize(design.TypeSmall).TextColor(c.Theme().TextMuted)
				ui.PrimaryButton(c, msgs.ShowAnswer()).AlignSelf(ui.Start).
					PaddingX(design.SpaceL).MinHeight(design.ControlHeight).
					OnClick(s.Reveal)
				return
			}
			if card.Answer == "" {
				ui.Text(c, msgs.EmptyAnswer()).
					FontSize(design.TypeBody).TextColor(c.Theme().TextMuted)
				return
			}
			// The answer's formulas are set for reading; the note keeps the
			// source.
			ui.Text(c, maths.Render(back)).FontSize(design.TypeBody).TextColor(c.Theme().Text)
		})
}

// grades is the four ratings, each in its own colour.
func grades(c *ui.Context, d Deps, id core.ID) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	intervals, hasPreview := [4]Interval{}, false
	if d.Preview != nil {
		intervals, hasPreview = d.Preview()
	}
	ui.Row(c).Gap(design.SpaceM).Children(func() {
		for r := Again; r <= Easy; r++ {
			face := r.colour(t)
			label := r.label(msgs)
			key := ui.ButtonBase(c).Grow(1).MinHeight(design.ControlHeight).
				Gap(design.SpaceS).Padding(design.SpaceXS, design.SpaceS).
				Background(t.Surface.Mix(face, 0.16)).Border(design.Hairline, t.Border.Mix(face, 0.5)).
				Radius(design.RadiusControl).Label(label).
				OnClick(func() { d.Grade(id, r) }).
				Children(func() {
					kit.Keycap(c, r.Key())
					ui.Text(c, label).FontSize(design.TypeBody).TextColor(t.Text)
					if hasPreview {
						if wait := formatWait(msgs, intervals[r].Wait); wait != "" {
							ui.Spacer(c)
							ui.Text(c, wait).FontSize(design.TypeSmall).TextColor(t.TextMuted)
						}
					}
				})
			// The face eases under the pointer, and a press shrinks the key
			// without changing the room it takes.
			if key.Hovered() {
				key.Background(t.Surface.Mix(face, 0.24)).
					Transition(ui.ElementTransition{Colors: true, Animation: &design.Hover})
			}
			pose := ui.Pose{Scale: 1}
			if key.Pressed() {
				pose.Scale = design.PressScale
			}
			key.AnimatePose(pose, design.Press)
		}
	})
}

// formatWait is how long an interval reads as on a key: minutes below an hour,
// hours below a day, days after that. A null wait reads as nothing, so the key
// falls back to its plain label rather than showing a placeholder.
func formatWait(msgs locales.Messages, wait time.Duration) string {
	switch {
	case wait <= 0:
		return ""
	case wait < time.Hour:
		return msgs.IntervalMinutes(max(1, int(wait.Round(time.Minute)/time.Minute)))
	case wait < 24*time.Hour:
		return msgs.IntervalHours(max(1, int(wait.Round(time.Hour)/time.Hour)))
	default:
		return msgs.IntervalDays(max(1, int(wait.Round(24*time.Hour)/(24*time.Hour))))
	}
}

// footer holds the actions beside the card's grades.
func footer(c *ui.Context, s *State, d Deps, card Card) {
	msgs := locales.In(c.Locale())
	ui.Row(c).Gap(design.SpaceS).AlignItems(ui.Center).Children(func() {
		_, canUndo := s.Last()
		ui.Button(c, msgs.Undo()).MinHeight(design.ControlHeight).Disabled(!canUndo).
			OnClick(d.Undo)

		if card.State == core.LinkLost {
			return
		}
		ui.Button(c, msgs.Suspend()).MinHeight(design.ControlHeight).
			OnClick(func() { d.Suspend(card.ID) })
		ui.Button(c, msgs.EditNote()).MinHeight(design.ControlHeight).
			OnClick(func() { d.EditNote(card.ID) })
	})
}

// empty is the state after the last card: the session's summary when the app
// has one, and either way the way back.
func empty(c *ui.Context, d Deps) {
	msgs := locales.In(c.Locale())
	summary, hasSummary := Summary{}, false
	if d.Summary != nil {
		summary, hasSummary = d.Summary()
	}

	// The tone is what the screen does: a quiet summary is simply there, a
	// celebrated one arrives once and then rests. Both follow the desktop's
	// reduce-motion setting, which snaps the frames to their end.
	face := ui.Column(c)
	if hasSummary && summary.Tone == Celebrated {
		rise := face.Keyframes("celebrate", design.EnterFrames)
		face.Opacity(rise).Translate(0, design.Rise*(1-rise))
	}
	face.Children(func() {
		t := c.Theme()
		if !hasSummary {
			kit.Card(c, func() {
				kit.Empty(c, "coffee", msgs.NoCards(), func() {
					ui.Button(c, msgs.BackToNotes()).MinHeight(design.ControlHeight).
						OnClick(d.Home)
				})
			})
			return
		}
		// A session's worth is read as three numbers first, then in words: the
		// same card the queue used, wider, with the one action under it.
		card := kit.Card(c, func() {
			ui.Column(c).Gap(design.SpaceXL).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(design.SpaceS).Children(func() {
					ui.Icon(c, design.Icon("coffee")).
						Width(design.IconMedium).Height(design.IconMedium).
						TextColor(t.Accent)
					ui.Text(c, msgs.SummaryTitle()).FontSize(design.TypeTitle).Bold()
				})
				ui.Row(c).Gap(design.SpaceXXL).Children(func() {
					kit.Stat(c, strconv.Itoa(summary.Reviewed), msgs.StatReviewed())
					kit.Stat(c, strconv.Itoa(summary.DueTomorrow), msgs.StatDueTomorrow())
					kit.Stat(c, fmt.Sprintf("%d%%", int(summary.Accuracy*100)), msgs.StatRemembered())
				})
				for _, line := range summary.Lines {
					if strings.TrimSpace(line) == "" {
						continue
					}
					// A plugin's line is markdown; the screen renders it as one
					// line of text with its formulas set.
					ui.Text(c, maths.Render(line)).
						FontSize(design.TypeBody).TextColor(t.TextMuted)
				}
				// Leaving is never blocked, whatever the tone, and it is the one
				// filled button on the screen.
				ui.PrimaryButton(c, msgs.BackToNotes()).AlignSelf(ui.Start).
					MinHeight(design.ControlHeight).OnClick(d.Home)
			})
		})
		card.Width(520).MaxWidth(520).Padding(design.SpaceXXL)
	})
}
