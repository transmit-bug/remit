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

// failedRowsVisible is how many of a summary's missed notes show before the rows
// scroll: enough to read the shape of the session, few enough to leave the one
// action on the card at the smallest window.
const failedRowsVisible = 4

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
// them now, what the type's template folds away, and how its source stands.
type Card struct {
	ID     core.ID
	Prompt string
	Answer string
	// Hint is the {{hint:}} the card's template folded into the front, shown
	// only when asked for.
	Hint string
	// TypeAnswer is the answer a {{type:}} tag asks the user to type. It is
	// empty on a card that is only flipped.
	TypeAnswer string
	State      core.LinkState
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
	// Failed are the cards the session did not remember, which the summary
	// groups by the note each came from: the way back into the material.
	Failed []FailedCard
}

// FailedCard is a card the session answered Again: where it came from, and the
// note's name, so the summary can gather a note's misses into one row without
// reading the library.
type FailedCard struct {
	Card  core.ID
	Note  core.ID
	Title string
}

// Source is where the card on screen came from: the note, the paragraph the
// card follows, and the cards the same paragraph asks. A card is never shown
// without a way back to what it asks about.
type Source struct {
	Note  core.ID
	Title string
	// Paragraph is the block's place in its note, counted from one, and Blocks
	// how many paragraphs the note holds.
	Paragraph, Blocks int
	// Siblings is how many cards the paragraph asks and Index which of them
	// this one is, both counted from one: the card's own badge reads "1 of 2".
	Siblings, Index int
	// Prev, Here and Next are the paragraphs around the card's, as plain text,
	// so that expanded context needs no second read.
	Prev, Here, Next string
}

// PluginItem is one of the actions a plugin contributed to the review session,
// named in this frame's language, with the command it runs.
type PluginItem struct {
	Label string
	Run   func()
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
	// Source is where the card on screen came from, when the card still has a
	// paragraph; false leaves the source line out.
	Source func(core.ID) (Source, bool)
	// OpenSource opens the note the card came from: the way back to the
	// material, from the source line and from the summary's misses.
	OpenSource func(core.ID)
	// Items are the review.card slot's actions for the card on screen, in the
	// order they draw.
	Items    func() []PluginItem
	Grade    func(core.ID, Rating) // record the answer and move on
	Undo     func()                // take the newest answer back
	Suspend  func(core.ID)         // out of the queue, history untouched
	Relink   func(core.ID)         // point a lost card at another block
	EditNote func(core.ID)         // open the note behind the card
	EditCard func(core.ID)         // edit the card's own fields
	Home     func()                // leave for the notebook
	// EmptyLabel is what the empty state says when the app has no summary to
	// give; nil leaves the plain "no cards" line.
	EmptyLabel func() string
}

// State is the queue and where the user is in it. What belongs in it is the
// store's answer (queue.Cards); the order is the session's, so progress
// survives leaving review for notes, and undo can put the card just graded back
// at the head where the user left it.
type State struct {
	queue    []core.ID // the card shown, then those after it
	shown    core.ID   // the card revealed reports on
	revealed bool
	graded   []core.ID // cards graded this session, newest last
	hintOpen bool      // the {{hint:}} fold is open
	// contextOpen shows the paragraphs around the card's, and the cards the
	// same paragraph asks: reading the card in its place rather than alone.
	contextOpen bool
	typed       string // what the user typed for the card on screen
	submitted   bool   // that typed answer has been checked
	verdict     []Span // how it compared with the card's answer
}

// Reset forgets the queue and the session's progress.
func (s *State) Reset() {
	s.queue, s.shown, s.revealed, s.graded = nil, "", false, nil
	s.forgetCard()
}

// Sync takes the library's answer to what is due now: a card still due keeps its
// place, one no longer due leaves, and one that became due — a card made just
// now, a card returned by undo — joins the end. Membership is the store's; the
// order the user is working through stays.
func (s *State) Sync(due []core.ID) {
	head, _ := s.Current()
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
	if now, ok := s.Current(); !ok || now != head {
		s.forgetCard()
	}
}

// forgetCard drops what belonged to the card on screen: what is open, what was
// typed, and how it compared, is the card's, not the session's.
func (s *State) forgetCard() {
	s.hintOpen, s.contextOpen = false, false
	s.typed, s.submitted, s.verdict = "", false, nil
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

// Only makes id the whole queue, as opening one card from the card table does.
func (s *State) Only(id core.ID) {
	s.queue = []core.ID{id}
	s.forgetCard()
}

// Reveal shows the card's answer.
func (s *State) Reveal() {
	if len(s.queue) > 0 {
		s.shown, s.revealed = s.queue[0], true
	}
}

// ToggleHint opens or closes the card's {{hint:}} fold.
func (s *State) ToggleHint() { s.hintOpen = !s.hintOpen }

// Typing reports whether the card on screen takes a typed answer that has not
// been checked yet: the screen collects one instead of revealing the answer.
func (s *State) Typing(card Card) bool {
	return card.TypeAnswer != "" && len(s.queue) > 0 && !s.Answered()
}

// Submit checks the typed answer against the card's and shows the answer. The
// comparison is kept, so the screen can draw how the two spellings differ.
func (s *State) Submit(want string) {
	if len(s.queue) == 0 || s.submitted || s.Answered() {
		return
	}
	s.verdict = Compare(want, s.typed)
	s.submitted = true
	s.shown, s.revealed = s.queue[0], true
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
	s.forgetCard()
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
	s.forgetCard()
}

// Show puts a card at the head of the queue with its answer still hidden, as a
// card that was just re-linked wants: the user sees the text it follows now,
// whether or not its due date is today.
func (s *State) Show(id core.ID) {
	s.queue = append([]core.ID{id}, s.queue...)
	s.shown, s.revealed = "", false
	s.forgetCard()
}

// Forget drops the grade undo has just cancelled.
func (s *State) Forget() {
	if len(s.graded) > 0 {
		s.graded = s.graded[:len(s.graded)-1]
	}
}

// View draws the screen: one card, centred.
func View(c *ui.Context, s *State, d Deps) {
	id, hasCard := s.Current()
	var card Card
	if hasCard {
		card, hasCard = d.Card(id)
	}
	ui.Column(c).Fill().Children(func() {
		statusBar(c, s, card)
		// The card sits in the middle of what the status bar leaves, on the
		// smallest window with a tighter margin so the card keeps its room.
		pad := float32(design.SpaceXXL)
		if w, h := c.Size(); w < 800 || h < 600 {
			pad = design.SpaceXL
		}
		if _, h := c.Size(); h < 600 {
			pad = design.SpaceL
		}
		ui.Column(c).Grow(1).Center().Padding(pad).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(design.ReviewWidth).Gap(design.SpaceL).Children(func() {
				if !hasCard {
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
func statusBar(c *ui.Context, s *State, card Card) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	done, total := s.Progress()
	ui.Row(c).Justify(ui.Center).Children(func() {
		ui.Row(c).Grow(1).MaxWidth(design.ReviewWidth).Height(statusHeight).
			Padding(design.SpaceXS, design.SpaceL).AlignItems(ui.Center).
			Gap(design.SpaceS).Children(func() {
			ui.Text(c, msgs.Remaining(s.Remaining())).
				FontSize(design.TypeUI).TextColor(t.TextMuted)
			if total > 0 {
				ui.Progress(c, float64(done)/float64(total)).Grow(1)
			}
			ui.Spacer(c)
			ui.Row(c).Gap(design.SpaceXS).AlignItems(ui.Center).Children(func() {
				// A card that takes a typed answer is checked from its input, so the
				// key the status bar names is Enter, not the reveal only flips.
				revealKey, revealLabel := "Space", msgs.ShowAnswer()
				if s.Typing(card) {
					revealKey, revealLabel = "Enter", msgs.CheckAnswer()
				}
				keys := []struct{ key, label string }{
					{revealKey, revealLabel},
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
	})
}

// flipCard draws the card: its link state, the prompt, and the answer once
// revealed. It is a wrapper, not a copied widget: see RECONCILE.md.
//
// The lost branch is defensive: today's queue leaves a lost card out, because a
// queue promises cards that can be reviewed now. It holds if a card's source
// disappears between the queue being derived and the card being drawn, and a
// card opened from the card table lands here too. Re-linking is the repair,
// and
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
					FontSize(design.TypeUI).TextColor(c.Theme().TextMuted)
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
						FontSize(design.TypeUI).TextColor(c.Theme().TextMuted)
				})
			}

			// A card's faces arrive already rendered: the type's templates
			// turned its fields into markdown, the host turns markdown into text.
			front, back := card.Prompt, card.Answer
			ui.Text(c, maths.Render(front)).FontSize(design.TypeH1).Bold()
			sourceLine(c, s, d, card)
			ui.Divider(c)
			if card.Hint != "" && !s.Answered() {
				hintFold(c, s, card)
			}
			if s.Typing(card) {
				typing(c, s, card)
				return
			}
			if !s.Answered() {
				ui.Text(c, msgs.RevealHint()).
					FontSize(design.TypeUI).TextColor(c.Theme().TextMuted)
				ui.PrimaryButton(c, msgs.ShowAnswer()).AlignSelf(ui.Start).
					PaddingX(design.SpaceL).MinHeight(design.ControlHeight).
					OnClick(s.Reveal)
				return
			}
			if s.submitted {
				comparison(c, s)
			}
			if card.Answer == "" {
				ui.Text(c, msgs.EmptyAnswer()).
					FontSize(design.TypeUI).TextColor(c.Theme().TextMuted)
				return
			}
			// The answer's formulas are set for reading; the note keeps the
			// source. It unfolds where it appears: the card was there the frame
			// before, so the reveal plays once, on this answer.
			kit.Reveal(c, ui.Text(c, maths.Render(back)).
				FontSize(design.TypeBody).TextColor(c.Theme().Text))
		})
}

// grades is the four ratings, each in its own colour. Below BreakpointSmall
// they take two columns instead of one row, so a narrow window still shows all
// four without scrolling the card (spec §2.5 ④).
func grades(c *ui.Context, d Deps, id core.ID) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	intervals, hasPreview := [4]Interval{}, false
	if d.Preview != nil {
		intervals, hasPreview = d.Preview()
	}
	columns := 4
	if w, _ := c.Size(); w <= design.BreakpointSmall {
		columns = 2
	}
	ui.Grid(c).FillWidth().Columns(columns).Gap(design.SpaceM).Children(func() {
		for r := Again; r <= Easy; r++ {
			face := r.colour(t)
			label := r.label(msgs)
			// A grid cell measures its own content, so the key takes the track
			// it is given rather than pushing the grid to one column.
			key := ui.ButtonBase(c).FillWidth().MinWidth(0).MinHeight(design.ControlHeight).
				Gap(design.SpaceS).Padding(design.SpaceXS, design.SpaceS).
				Background(t.Surface.Mix(face, 0.16)).Border(design.Hairline, t.Border.Mix(face, 0.5)).
				Radius(design.RadiusControl).Label(label).
				OnClick(func() { d.Grade(id, r) }).
				Children(func() {
					kit.Keycap(c, r.Key())
					ui.Text(c, label).FontSize(design.TypeUI).TextColor(t.Text)
					if hasPreview {
						if wait := formatWait(msgs, intervals[r].Wait); wait != "" {
							ui.Spacer(c)
							ui.Text(c, wait).FontSize(design.TypeUI).TextColor(t.TextMuted)
						}
					}
				})
			// The face eases under the pointer, and a press shrinks the key
			// without changing the room it takes.
			key = kit.HoverFace(c, key)
			if key.Hovered() {
				key.Background(t.Surface.Mix(face, 0.24))
			}
			kit.Press(key, key.Pressed())
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

// footer holds the actions beside the card's grades: what to do with the card
// itself, and what the plugins contribute to this session.
func footer(c *ui.Context, s *State, d Deps, card Card) {
	msgs := locales.In(c.Locale())
	ui.Row(c).Gap(design.SpaceS).AlignItems(ui.Center).Wrap().Children(func() {
		_, canUndo := s.Last()
		ui.Button(c, msgs.Undo()).MinHeight(design.ControlHeight).Disabled(!canUndo).
			OnClick(d.Undo)

		if card.State == core.LinkLost {
			return
		}
		ui.Button(c, msgs.Suspend()).MinHeight(design.ControlHeight).
			OnClick(func() { d.Suspend(card.ID) })
		if d.EditCard != nil {
			ui.Button(c, msgs.ReviewEditCard()).MinHeight(design.ControlHeight).
				OnClick(func() { d.EditCard(card.ID) })
		}
		ui.Button(c, msgs.EditNote()).MinHeight(design.ControlHeight).
			OnClick(func() { d.EditNote(card.ID) })
		if d.Items == nil {
			return
		}
		for _, item := range d.Items() {
			item := item
			ui.Button(c, item.Label).MinHeight(design.ControlHeight).
				OnClick(item.Run)
		}
	})
}

// sourceLine is where the card came from, under the question it asks: the note,
// the paragraph, the cards the same paragraph asks, the way back to the note, and
// the fold that shows the card in its place. A card without a paragraph — a
// lost one — has nothing to say and draws no line.
func sourceLine(c *ui.Context, s *State, d Deps, card Card) {
	if d.Source == nil {
		return
	}
	source, ok := d.Source(card.ID)
	if !ok {
		return
	}
	t := c.Theme()
	msgs := locales.In(c.Locale())
	ui.Row(c).AlignItems(ui.Center).Gap(design.SpaceS).Wrap().Children(func() {
		ui.Text(c, msgs.ReviewSourceLine(source.Title, source.Paragraph, source.Blocks, source.Siblings)).
			FontSize(design.TypeUI).TextColor(t.TextMuted).Grow(1)
		if d.OpenSource != nil {
			ui.Button(c, msgs.ReviewBackToSource()).MinHeight(design.ControlHeight).
				OnClick(func() { d.OpenSource(source.Note) })
		}
		label := msgs.ReviewShowContext()
		if s.contextOpen {
			label = msgs.ReviewHideContext()
		}
		ui.Button(c, label).MinHeight(design.ControlHeight).
			OnClick(func() { s.contextOpen = !s.contextOpen })
	})
	if s.contextOpen {
		contextBlock(c, source)
	}
}

// contextBlock is the card in its place: the paragraph before it, the one it
// asks, the one after it, and the other cards that paragraph asks — so a card
// read out of the queue is still read in its context. The quiet face is a Column
// in the page's surface: MyGo v0.8.1 has no callout (mygo-components records the
// gap), and a notice's meaning here is the paragraph's own place.
func contextBlock(c *ui.Context, source Source) {
	t := c.Theme()
	msgs := locales.In(c.Locale())
	block := ui.Column(c).FillWidth().Padding(design.SpaceM).Gap(design.SpaceXS).
		Background(t.Surface).Border(design.Hairline, t.Border).
		Radius(design.RadiusControl).Label(msgs.ReviewContext()).Role(ui.RoleGroup)
	block.Children(func() {
		contextLine(c, msgs.ReviewContextBefore(), source.Prev, t.TextMuted)
		contextLine(c, msgs.ReviewContextHere(), source.Here, t.Text)
		contextLine(c, msgs.ReviewContextAfter(), source.Next, t.TextMuted)
		if source.Siblings > 1 {
			ui.Row(c).Gap(design.SpaceXS).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Text(c, msgs.ReviewSiblings()).FontSize(design.TypeCaption).
					TextColor(t.TextMuted)
				for i := 1; i <= source.Siblings; i++ {
					badge := ui.Badge(c, msgs.ReviewSibling(i, source.Siblings))
					if i == source.Index {
						badge.Background(t.Accent.Alpha(0.12)).TextColor(t.Accent)
					}
				}
			})
		}
	})
}

// contextLine is one paragraph of the context: its name, and its text as the
// card's own sides are set. An empty paragraph draws nothing, so a card at
// either end of its note has no blank line where the note stops.
func contextLine(c *ui.Context, name, text string, ink ui.Color) {
	if text == "" {
		return
	}
	ui.Row(c).AlignItems(ui.Start).Gap(design.SpaceS).Children(func() {
		ui.Text(c, name).FontSize(design.TypeCaption).TextColor(ink).
			MinWidth(design.SourceLabelWidth)
		ui.Text(c, maths.Render(text)).FontSize(design.TypeUI).TextColor(ink).Grow(1)
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

	// The tone is what the screen does: a quiet summary unfolds into the room
	// the last card left, a celebrated one arrives once and rests. Both follow
	// the desktop's reduce-motion setting, which snaps the frames to their end.
	face := kit.Reveal(c, ui.Column(c))
	if hasSummary && summary.Tone == Celebrated {
		rise := face.Keyframes("celebrate", design.EnterFrames)
		face.Opacity(rise).Translate(0, design.Rise*(1-rise))
	}
	face.Children(func() {
		t := c.Theme()
		if !hasSummary {
			line := msgs.NoCards()
			if d.EmptyLabel != nil {
				line = d.EmptyLabel()
			}
			kit.Card(c, func() {
				kit.Empty(c, "coffee", line, func() {
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
					ui.Text(c, msgs.SummaryTitle()).FontSize(design.TypeH2).Bold()
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
						FontSize(design.TypeUI).TextColor(t.TextMuted)
				}
				if len(summary.Failed) > 0 {
					failedBlock(c, summary.Failed, d)
				}
				// Leaving is never blocked, whatever the tone, and it is the one
				// filled button on the screen.
				ui.PrimaryButton(c, msgs.BackToNotes()).AlignSelf(ui.Start).
					MinHeight(design.ControlHeight).OnClick(d.Home)
			})
		})
		card.FillWidth().MaxWidth(design.SummaryWidth).AlignSelf(ui.Center).Padding(design.SpaceXXL).
			Label(msgs.SummaryTitle()).Role(ui.RoleGroup)
	})
}

// FailedGroup is the cards one note did not give up, gathered into the row the
// summary draws: the note's name and how many of its cards were missed.
type FailedGroup struct {
	Note  core.ID
	Title string
	Count int
}

// GroupFailed gathers the cards a session did not remember by the note each came
// from, in the order the session met them. A session that missed nothing has no
// groups, which is what keeps the heading off the summary.
func GroupFailed(failed []FailedCard) []FailedGroup {
	at := map[core.ID]int{}
	groups := make([]FailedGroup, 0, len(failed))
	for _, card := range failed {
		if i, seen := at[card.Note]; seen {
			groups[i].Count++
			continue
		}
		at[card.Note] = len(groups)
		groups = append(groups, FailedGroup{Note: card.Note, Title: card.Title, Count: 1})
	}
	return groups
}

// failedBlock is what the session did not remember, by the note it came from:
// one row per note, each a click back into the material, so a miss is the way
// back to the paragraph rather than the end of the sitting.
func failedBlock(c *ui.Context, failed []FailedCard, d Deps) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	groups := GroupFailed(failed)
	ui.Column(c).FillWidth().Gap(design.SpaceS).Children(func() {
		ui.Text(c, msgs.SummaryFailed()).FontSize(design.TypeH2).Bold()
		ui.Text(c, msgs.SummaryFailedHint()).FontSize(design.TypeLabel).
			TextColor(t.TextMuted)
		// The rows scroll rather than push the one action off the card: the
		// smallest window holds four of them, and the rest stay reachable.
		ui.Scroll(c).MaxHeight(design.ControlHeight * failedRowsVisible).
			Children(func() {
				ui.Column(c).Gap(design.SpaceXS).Children(func() {
					for _, group := range groups {
						group := group
						row := ui.Button(c, "").FillWidth().MinHeight(design.ControlHeight).
							Padding(design.SpaceXS, design.SpaceS).
							Label(msgs.SummaryFailedOpen(group.Title))
						row.Children(func() {
							ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(design.SpaceS).Children(func() {
								ui.Column(c).Grow(1).Gap(design.SpaceXS).Children(func() {
									ui.Text(c, group.Title).FontSize(design.TypeUI).SingleLine()
									ui.Text(c, msgs.SummaryFailedRow(group.Count)).
										FontSize(design.TypeLabel).TextColor(t.TextMuted).SingleLine()
								})
								ui.Text(c, msgs.ReviewBackToSource()).FontSize(design.TypeLabel).
									TextColor(t.Accent)
							})
						})
						if d.OpenSource != nil {
							row.OnClick(func() { d.OpenSource(group.Note) })
						}
					}
				})
			})
	})
}
