package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
)

// reviewWidth caps the review column: a comfortable measure, centred in the
// window with the notes list out of the way.
const reviewWidth float32 = 680

// advance is how a graded card leaves and the next arrives: a short slide and
// fade, no celebration. Element.Transition honours the desktop's reduce-motion
// setting by fading in place.
var advance = ui.ElementTransition{
	Duration: 120 * time.Millisecond,
	Ease:     ui.EaseOut,
	Enter:    &ui.Motion{Y: 8},
	Exit:     &ui.Motion{Y: -8},
}

// rating is one of the four answers a revealed card takes.
type rating uint8

const (
	ratingAgain rating = iota
	ratingHard
	ratingGood
	ratingEasy
)

// schedule is the store's rating for this answer: the same four in the same
// order, counted from one.
func (g rating) schedule() schedule.Rating { return schedule.Rating(g) + 1 }

func (g rating) key() string {
	return [...]string{"1", "2", "3", "4"}[g]
}

func (g rating) label(c *ui.Context) string {
	switch g {
	case ratingAgain:
		return m(c).Again()
	case ratingHard:
		return m(c).Hard()
	case ratingGood:
		return m(c).Good()
	default:
		return m(c).Easy()
	}
}

func (g rating) colour(tk tokens) ui.Color {
	return [...]ui.Color{tk.reviewAgain, tk.reviewHard, tk.reviewGood, tk.reviewEasy}[g]
}

// reviewState is the queue and where the user is in it. What belongs in it is
// the store's answer (queue.Today); the order is the session's, so progress
// survives leaving review for notes, and undo can put the card just graded
// back at the head where the user left it.
type reviewState struct {
	queue    []core.ID // the card shown, then those after it
	shown    core.ID   // the card revealed reports on
	revealed bool
	graded   []core.ID // cards graded this session, newest last
}

// reset forgets the queue and the session's progress.
func (r *reviewState) reset() {
	r.queue, r.shown, r.revealed, r.graded = nil, "", false, nil
}

// sync takes the library's answer to what is due now: a card that is still due
// keeps its place, one that is no longer due leaves, and one that became due —
// a card made just now, a card returned by undo — joins the end. Membership is
// the store's; the order the user is working through stays.
func (r *reviewState) sync(due []core.ID) {
	live := make(map[core.ID]bool, len(due))
	for _, id := range due {
		live[id] = true
	}
	kept := make([]core.ID, 0, len(due))
	for _, id := range r.queue {
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
	r.queue = kept
}

// remaining is how many cards are still to grade.
func (r *reviewState) remaining() int { return len(r.queue) }

// current is the card on screen.
func (r *reviewState) current(d libraryData) (core.Card, bool) {
	if len(r.queue) == 0 {
		return core.Card{}, false
	}
	return d.card(r.queue[0])
}

// reveal shows the card's answer.
func (r *reviewState) reveal() {
	if len(r.queue) > 0 {
		r.shown, r.revealed = r.queue[0], true
	}
}

// answered reports whether the answer on screen is the card's.
func (r *reviewState) answered() bool {
	return r.revealed && len(r.queue) > 0 && r.shown == r.queue[0]
}

// advance moves past the card on screen, remembering it when it was graded
// rather than suspended.
func (r *reviewState) advance(graded bool) {
	if len(r.queue) == 0 {
		return
	}
	if graded {
		r.graded = append(r.graded, r.queue[0])
	}
	r.queue = r.queue[1:]
	r.shown, r.revealed = "", false
}

// last is the card the newest grade belongs to.
func (r *reviewState) last() (core.ID, bool) {
	if len(r.graded) == 0 {
		return "", false
	}
	return r.graded[len(r.graded)-1], true
}

// restore puts a card back on screen with its answer shown, the way it was
// when it was graded.
func (r *reviewState) restore(id core.ID) {
	r.queue = append([]core.ID{id}, r.queue...)
	r.shown, r.revealed = id, true
}

// show puts a card at the head of the queue with its answer still hidden, as a
// card that was just re-linked wants: the user sees the text it follows now,
// whether or not its due date is today.
func (r *reviewState) show(id core.ID) {
	r.queue = append([]core.ID{id}, r.queue...)
	r.shown, r.revealed = "", false
}

// forget drops the grade undo has just cancelled.
func (r *reviewState) forget() { r.graded = r.graded[:len(r.graded)-1] }

// reviewView draws review mode: one card, centred.
func (a *App) reviewView(c *ui.Context, tk tokens) {
	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.spaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(reviewWidth).Gap(tk.spaceL).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, m(c).ReviewTitle()).FontSize(tk.typeTitle).Bold()
				ui.Spacer(c)
				ui.Badge(c, m(c).Remaining(a.review.remaining()))
			})

			card, ok := a.review.current(a.data)
			if !ok {
				a.reviewEmpty(c, tk)
				return
			}
			prompt, answer, state := card.Sides(a.data.blocks)
			a.flipCard(c, tk, card, prompt, answer, state)
			if state != core.LinkLost && a.review.answered() {
				a.gradeRow(c, tk)
			}
			a.reviewFooter(c, tk, card, state)
		})
	})
}

// flipCard draws the review card: its link state, the prompt, and the answer
// once revealed. It is a wrapper, not a copied widget: see RECONCILE.md.
//
// The lost branch is defensive: today's queue leaves a lost card out, because a
// queue promises cards that can be reviewed now. It holds if a card's source
// disappears between the queue being derived and the card being drawn, and a
// card opened from browse lands here too. Re-linking is the repair, and
// suspending — the one action that never touches history — is the way out.
func (a *App) flipCard(c *ui.Context, tk tokens, card core.Card, prompt, answer string, state core.LinkState) {
	kit.StyleCard(c, ui.Column(c).Key(string(card.ID)).
		Padding(tk.spaceXL).Gap(tk.spaceL)).Transition(advance).
		Children(func() {
			switch state {
			case core.LinkLost:
				ui.Badge(c, "").AlignSelf(ui.Start).
					Children(func() { tk.iconText(c, "unlink", m(c).Lost(), iconSmall) }).
					Background(tk.linkFill(tk.Danger)).TextColor(tk.linkInk(tk.Danger))
				ui.Text(c, m(c).LostBody()).
					FontSize(tk.typeBody).TextColor(tk.TextMuted)
				ui.Row(c).Gap(tk.spaceS).AlignSelf(ui.Start).Children(func() {
					ui.PrimaryButton(c, m(c).Relink()).MinHeight(tk.control).
						OnClick(func() { a.openRelink(c, card.ID) })
					ui.Button(c, m(c).SuspendCard()).MinHeight(tk.control).
						OnClick(func() { a.suspend(c) })
				})
				return
			case core.LinkFrozen:
				ui.Row(c).AlignSelf(ui.Start).AlignItems(ui.Center).Gap(tk.spaceS).Children(func() {
					ui.Badge(c, "").
						Children(func() { tk.iconText(c, "snowflake", m(c).Frozen(), iconSmall) }).
						Background(tk.linkFill(tk.Warning)).TextColor(tk.linkInk(tk.Warning))
					ui.Text(c, m(c).SnapshotTaken(card.Ref.Frozen.At.Format("2 January 2006"))).
						FontSize(tk.typeSmall).TextColor(tk.TextMuted)
				})
			}

			ui.Text(c, prompt).FontSize(tk.typeHeading).Bold()
			ui.Divider(c)
			if !a.review.answered() {
				ui.Text(c, m(c).RevealHint()).
					FontSize(tk.typeSmall).TextColor(tk.TextMuted)
				ui.PrimaryButton(c, m(c).ShowAnswer()).AlignSelf(ui.Start).
					PaddingX(tk.spaceL).MinHeight(tk.control).
					OnClick(func() { a.review.reveal() })
				return
			}
			if answer == "" {
				ui.Text(c, m(c).EmptyAnswer()).
					FontSize(tk.typeBody).TextColor(tk.TextMuted)
				return
			}
			ui.Text(c, answer).FontSize(tk.typeBody).TextColor(tk.Text)
		})
}

// gradeRow is the four ratings, each in its own colour.
func (a *App) gradeRow(c *ui.Context, tk tokens) {
	ui.Row(c).Gap(tk.spaceM).Children(func() {
		for g := ratingAgain; g <= ratingEasy; g++ {
			face := g.colour(tk)
			key := ui.ButtonBase(c).Grow(1).MinHeight(tk.control).
				Gap(tk.spaceS).Padding(tk.spaceXS, tk.spaceS).
				Background(tk.ratingFill(face)).Border(tk.hairline, tk.ratingEdge(face)).
				Radius(tk.radiusControl).Label(g.label(c)).
				OnClick(func() { a.grade(c, g) }).
				Children(func() {
					kit.Keycap(c, g.key())
					ui.Text(c, g.label(c)).FontSize(tk.typeBody).TextColor(tk.Text)
				})
			// The face eases under the pointer, and a press shrinks the key
			// without changing the room it takes.
			if key.Hovered() {
				key.Background(tk.ratingFill(face).Mix(face, 0.08)).
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

// reviewFooter holds the actions beside the card's grades.
func (a *App) reviewFooter(c *ui.Context, tk tokens, card core.Card, state core.LinkState) {
	ui.Row(c).Gap(tk.spaceS).AlignItems(ui.Center).Children(func() {
		_, canUndo := a.review.last()
		ui.Button(c, m(c).Undo()).MinHeight(tk.control).Disabled(!canUndo).
			OnClick(func() { a.undo(c) })

		if state == core.LinkLost {
			return
		}
		ui.Button(c, m(c).Suspend()).MinHeight(tk.control).OnClick(func() { a.suspend(c) })
		ui.Button(c, m(c).EditNote()).MinHeight(tk.control).OnClick(func() { a.editNote(card) })
	})
}

// reviewEmpty is the state after the last card.
func (a *App) reviewEmpty(c *ui.Context, tk tokens) {
	kit.Card(c, func() {
		kit.Empty(c, "coffee", m(c).NoCards(), func() {
			ui.Button(c, m(c).BackToNotes()).MinHeight(tk.control).
				OnClick(func() { a.setMode(modeNotes) })
		})
	})
}

// grade rates the card shown, records the review in the store and moves on. A
// card is graded only once its answer is up, and a lost one has no sides to
// rate: it can be suspended instead.
func (a *App) grade(c *ui.Context, g rating) {
	card, ok := a.review.current(a.data)
	if !ok || !a.review.answered() {
		return
	}
	if _, _, state := card.Sides(a.data.blocks); state == core.LinkLost {
		return
	}
	if err := a.lib.Review(card.ID, g.schedule(), time.Now()); err != nil {
		a.report(c, "review", err)
		return
	}
	a.review.advance(true)
	a.reload()
	// The change is visual first; say it too, so a screen reader hears that
	// the grade landed and how much is left.
	c.Announce(m(c).Remaining(a.review.remaining()))
}

// undo cancels the newest review, puts its card back on screen with the answer
// shown and says so once.
func (a *App) undo(c *ui.Context) {
	id, ok := a.review.last()
	if !ok {
		return
	}
	if err := a.lib.UndoLast(id); err != nil {
		a.report(c, "undo", err)
		return
	}
	a.review.forget()
	a.review.restore(id)
	a.reload()
	c.Toast(m(c).GradeUndone())
}

// suspend takes the card shown out of the queue without touching its history.
func (a *App) suspend(c *ui.Context) {
	card, ok := a.review.current(a.data)
	if !ok {
		return
	}
	card.Status = core.StatusSuspended
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "suspend", err)
		return
	}
	a.review.advance(false)
	a.reload()
}

// editCurrent opens the note behind the card shown.
func (a *App) editCurrent() {
	card, ok := a.review.current(a.data)
	if !ok {
		return
	}
	a.editNote(card)
}
