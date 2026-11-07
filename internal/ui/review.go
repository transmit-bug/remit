package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/review"
)

// reviewView draws review mode: the screen itself lives in its own package, and
// this only hands it the library and the app's own actions (ADR 0004).
func (a *App) reviewView(c *ui.Context, tk design.Tokens) {
	review.View(c, &a.review, a.reviewDeps(c))
}

// reviewDeps is what the screen asks of the app: the card to show, and what the
// user can do with it, each recorded by the app on the UI thread.
func (a *App) reviewDeps(c *ui.Context) review.Deps {
	return review.Deps{
		Card: a.reviewCard,
		Summary: func() (review.Summary, bool) {
			if !a.hasSummary {
				return review.Summary{}, false
			}
			out := review.Summary{
				Reviewed:    a.summary.Reviewed,
				DueTomorrow: a.summary.DueTomorrow,
				Accuracy:    a.summary.Accuracy,
				Tone:        review.Quiet,
			}
			// What the scheduling plugin adds to the numbers is asked once per
			// session, not once a frame: a plugin may spend a second on it.
			key := fmt.Sprintf("%d/%d/%.3f", out.Reviewed, out.DueTomorrow, out.Accuracy)
			if a.summaryWordFor != key {
				a.summaryWordFor = key
				a.summaryWord, _ = a.scheduler.SummaryOf(out.Reviewed, out.DueTomorrow, out.Accuracy)
			}
			if a.summaryWord.Tone == "celebrated" {
				out.Tone = review.Celebrated
			}
			out.Lines = a.summaryWord.Lines
			return out, true
		},
		Preview: func() ([4]review.Interval, bool) {
			if !a.hasPreview || len(a.preview) != 4 {
				return [4]review.Interval{}, false
			}
			var out [4]review.Interval
			for i, o := range a.preview {
				out[i] = review.Interval{Rating: review.Rating(i), Wait: o.Wait}
			}
			return out, true
		},
		Grade: func(id core.ID, r review.Rating) { a.grade(c, id, r) },
		Undo:  func() { a.undo(c) },
		Suspend: func(id core.ID) {
			a.suspend(c, id)
		},
		Relink: func(id core.ID) { a.openRelink(c, id) },
		EditNote: func(id core.ID) {
			if card, ok := a.data.card(id); ok {
				a.editNote(card)
			}
		},
		Home: func() { a.setMode(modeNotes) },
	}
}

// reviewCard is the card the screen shows, with its sides as the library
// resolves them now.
func (a *App) reviewCard(id core.ID) (review.Card, bool) {
	card, ok := a.data.card(id)
	if !ok {
		return review.Card{}, false
	}
	face := a.face(card)
	out := review.Card{
		ID:     card.ID,
		Prompt: face.Front,
		Answer: face.Back,
		Hint:   face.Hint,
		State:  face.State,
	}
	if card.Ref.Frozen != nil {
		out.Frozen = card.Ref.Frozen.At
	}
	return out, true
}

// grade rates the card shown, records the review in the store and moves on. A
// card is graded only once its answer is up, and a lost one has no sides to
// rate: it can be suspended instead.
func (a *App) grade(c *ui.Context, id core.ID, r review.Rating) {
	card, ok := a.data.card(id)
	if !ok || !a.review.Answered() {
		return
	}
	if _, _, state := a.faces(card); state == core.LinkLost {
		return
	}
	if err := a.lib.Review(card.ID, r.Schedule(), time.Now()); err != nil {
		a.report(c, "review", err)
		return
	}
	a.review.Advance(true)
	a.reload()
	// The change is visual first; say it too, so a screen reader hears that the
	// grade landed and how much is left.
	c.Announce(m(c).Remaining(a.review.Remaining()))
}

// undo cancels the newest review, puts its card back on screen with the answer
// shown and says so once.
func (a *App) undo(c *ui.Context) {
	id, ok := a.review.Last()
	if !ok {
		return
	}
	if err := a.lib.UndoLast(id); err != nil {
		a.report(c, "undo", err)
		return
	}
	a.review.Forget()
	a.review.Restore(id)
	a.reload()
	c.Toast(m(c).GradeUndone())
}

// suspend takes a card out of the queue without touching its history.
func (a *App) suspend(c *ui.Context, id core.ID) {
	card, ok := a.data.card(id)
	if !ok {
		return
	}
	card.Status = core.StatusSuspended
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "suspend", err)
		return
	}
	if current, ok := a.review.Current(); ok && current == id {
		a.review.Advance(false)
	}
	a.reload()
}

// suspendCurrent is the suspend key: it takes the card on screen out of the
// queue.
func (a *App) suspendCurrent(c *ui.Context) {
	if id, ok := a.review.Current(); ok {
		a.suspend(c, id)
	}
}

// editCurrent opens the note behind the card shown.
func (a *App) editCurrent() {
	if id, ok := a.review.Current(); ok {
		if card, ok := a.data.card(id); ok {
			a.editNote(card)
		}
	}
}

// gradeCurrent is the grade key: it rates the card on screen.
func (a *App) gradeCurrent(c *ui.Context, r review.Rating) {
	if id, ok := a.review.Current(); ok {
		a.grade(c, id, r)
	}
}
