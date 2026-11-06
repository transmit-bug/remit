package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
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

// gradeRecord is what undo needs to put a graded card back as it was.
type gradeRecord struct {
	card   core.ID
	status core.Status
}

// reviewState is the queue and where the user is in it. Progress survives
// leaving review for notes, so coming back does not start over.
type reviewState struct {
	started  bool
	queue    []core.ID // the card shown, then those after it
	revealed bool
	history  []gradeRecord
}

// begin seeds the queue the first time review opens.
func (r *reviewState) begin(lib *demoLibrary) {
	if r.started {
		return
	}
	r.started = true
	for _, c := range lib.cards {
		if c.Status.Scheduled() {
			r.queue = append(r.queue, c.ID)
		}
	}
}

// remaining is how many cards are still to grade.
func (r *reviewState) remaining() int { return len(r.queue) }

// current is the card on screen.
func (r *reviewState) current(lib *demoLibrary) (core.Card, bool) {
	if len(r.queue) == 0 {
		return core.Card{}, false
	}
	c, ok := lib.card(r.queue[0])
	if !ok {
		return core.Card{}, false
	}
	return *c, true
}

// reveal shows the card's answer.
func (r *reviewState) reveal() {
	if len(r.queue) > 0 {
		r.revealed = true
	}
}

// grade records the rating, advances and hides the next card's answer.
func (r *reviewState) grade(lib *demoLibrary, g rating) {
	if !r.revealed || len(r.queue) == 0 {
		return
	}
	c, ok := lib.card(r.queue[0])
	if !ok {
		r.queue = r.queue[1:]
		return
	}
	r.history = append(r.history, gradeRecord{card: c.ID, status: c.Status})
	if g == ratingAgain || g == ratingHard {
		c.Status = core.StatusLearning
	} else {
		c.Status = core.StatusReview
	}
	r.queue = r.queue[1:]
	r.revealed = false
}

// undo puts the last graded card back, answer shown, and restores its status.
func (r *reviewState) undo(lib *demoLibrary) bool {
	if len(r.history) == 0 {
		return false
	}
	last := r.history[len(r.history)-1]
	r.history = r.history[:len(r.history)-1]
	if c, ok := lib.card(last.card); ok {
		c.Status = last.status
	}
	r.queue = append([]core.ID{last.card}, r.queue...)
	r.revealed = true
	return true
}

// suspend takes the card shown out of the queue without touching its history.
func (r *reviewState) suspend(lib *demoLibrary) bool {
	if len(r.queue) == 0 {
		return false
	}
	if c, ok := lib.card(r.queue[0]); ok {
		c.Status = core.StatusSuspended
	}
	r.queue = r.queue[1:]
	r.revealed = false
	return true
}

// reviewView draws review mode: one card, centred.
func (a *App) reviewView(c *ui.Context, tk tokens) {
	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.spaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(reviewWidth).Gap(tk.spaceL).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, m(c).ReviewTitle()).FontSize(tk.typeTitle).Bold()
				ui.Spacer(c)
				ui.Badge(c, m(c).Remaining(a.review.remaining()))
			})

			card, ok := a.review.current(a.library)
			if !ok {
				a.reviewEmpty(c, tk)
				return
			}
			prompt, answer, state := card.Sides(a.library.blocks())
			a.flipCard(c, tk, card, prompt, answer, state)
			if state != core.LinkLost && a.review.revealed {
				a.gradeRow(c, tk)
			}
			a.reviewFooter(c, tk, card, state)
		})
	})
}

// flipCard draws the review card: its link state, the prompt, and the answer
// once revealed. It is a wrapper, not a copied widget: see RECONCILE.md.
func (a *App) flipCard(c *ui.Context, tk tokens, card core.Card, prompt, answer string, state core.LinkState) {
	ui.Column(c).Key(string(card.ID)).
		Padding(tk.spaceXL).Gap(tk.spaceL).
		Background(tk.surfaceRaised).Border(tk.hairline, tk.border).
		Radius(tk.radiusCard).Transition(advance).
		Children(func() {
			switch state {
			case core.LinkLost:
				ui.Badge(c, m(c).Lost()).AlignSelf(ui.Start).
					Background(tk.linkFill(tk.danger)).TextColor(tk.linkInk(tk.danger))
				ui.Text(c, m(c).LostBody()).
					FontSize(tk.typeBody).TextColor(tk.textMuted)
				ui.Button(c, m(c).SuspendCard()).AlignSelf(ui.Start).
					PaddingX(tk.spaceL).MinHeight(tk.control).
					OnClick(a.suspend)
				return
			case core.LinkFrozen:
				ui.Row(c).AlignSelf(ui.Start).AlignItems(ui.Center).Gap(tk.spaceS).Children(func() {
					ui.Badge(c, m(c).Frozen()).
						Background(tk.linkFill(tk.warning)).TextColor(tk.linkInk(tk.warning))
					ui.Text(c, m(c).SnapshotTaken(card.Ref.Frozen.At.Format("2 January 2006"))).
						FontSize(tk.typeSmall).TextColor(tk.textMuted)
				})
			}

			ui.Text(c, prompt).FontSize(tk.typeHeading).Bold()
			ui.Divider(c)
			if !a.review.revealed {
				ui.Text(c, m(c).RevealHint()).
					FontSize(tk.typeSmall).TextColor(tk.textMuted)
				ui.PrimaryButton(c, m(c).ShowAnswer()).AlignSelf(ui.Start).
					PaddingX(tk.spaceL).MinHeight(tk.control).
					OnClick(func() { a.review.reveal() })
				return
			}
			if answer == "" {
				ui.Text(c, m(c).EmptyAnswer()).
					FontSize(tk.typeBody).TextColor(tk.textMuted)
				return
			}
			ui.Text(c, answer).FontSize(tk.typeBody).TextColor(tk.text)
		})
}

// gradeRow is the four ratings, each in its own colour.
func (a *App) gradeRow(c *ui.Context, tk tokens) {
	ui.Row(c).Gap(tk.spaceM).Children(func() {
		for g := ratingAgain; g <= ratingEasy; g++ {
			face := g.colour(tk)
			ui.ButtonBase(c).Grow(1).MinHeight(tk.control).
				Gap(tk.spaceS).Padding(tk.spaceXS, tk.spaceS).
				Background(tk.ratingFill(face)).Border(tk.hairline, tk.ratingEdge(face)).
				Radius(tk.radiusControl).Label(g.label(c)).
				OnClick(func() { a.grade(g) }).
				Children(func() {
					ui.Text(c, g.key()).FontSize(tk.typeSmall).TextColor(tk.textMuted)
					ui.Text(c, g.label(c)).FontSize(tk.typeBody).TextColor(tk.text)
				})
		}
	})
}

// reviewFooter holds the actions beside the card's grades.
func (a *App) reviewFooter(c *ui.Context, tk tokens, card core.Card, state core.LinkState) {
	ui.Row(c).Gap(tk.spaceS).AlignItems(ui.Center).Children(func() {
		undo := ui.Button(c, m(c).Undo()).MinHeight(tk.control)
		undo.Disabled(len(a.review.history) == 0)
		undo.OnClick(func() { a.undo(c) })

		if state == core.LinkLost {
			return
		}
		ui.Button(c, m(c).Suspend()).MinHeight(tk.control).OnClick(a.suspend)
		ui.Button(c, m(c).EditNote()).MinHeight(tk.control).OnClick(func() { a.editNote(card) })
	})
}

// reviewEmpty is the state after the last card.
func (a *App) reviewEmpty(c *ui.Context, tk tokens) {
	ui.Column(c).Padding(tk.spaceXL).Gap(tk.spaceM).
		Background(tk.surfaceRaised).Border(tk.hairline, tk.border).
		Radius(tk.radiusCard).Children(func() {
		ui.Text(c, m(c).NoCards()).FontSize(tk.typeHeading).TextColor(tk.textMuted)
		ui.Button(c, m(c).BackToNotes()).MinHeight(tk.control).
			OnClick(func() { a.setMode(modeNotes) })
	})
}

// grade rates the card shown and moves to the next. A lost card has no sides
// to rate; it can be suspended instead.
func (a *App) grade(g rating) {
	card, ok := a.review.current(a.library)
	if !ok {
		return
	}
	if _, _, state := card.Sides(a.library.blocks()); state == core.LinkLost {
		return
	}
	a.review.grade(a.library, g)
}

// undo puts the last graded card back and says so once.
func (a *App) undo(c *ui.Context) {
	if a.review.undo(a.library) {
		c.Toast(m(c).GradeUndone())
	}
}

// suspend takes the card shown out of the queue.
func (a *App) suspend() {
	a.review.suspend(a.library)
}

// editCurrent opens the note behind the card shown.
func (a *App) editCurrent() {
	card, ok := a.review.current(a.library)
	if !ok {
		return
	}
	a.editNote(card)
}
