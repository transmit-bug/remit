package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/internal/ui/screens/review"
	"github.com/transmit-bug/remit/locales"
)

// reviewView draws review mode: the screen itself lives in its own package, and
// this only hands it the library and the app's own actions (ADR 0004).
func (a *App) reviewView(c *ui.Context, tk design.Tokens) {
	review.View(c, &a.review, a.reviewDeps(c))
}

// reviewScopeMenu is the scope picker beside the mode switch: the whole
// library, one note, or one tag. The choice lives in the session only — it is
// app state, never written to the library — and switching re-derives the queue,
// which is what Sync then hands the screen. Widgets checked against the
// inventory (mygo-components, v0.8.1): MenuButton + Submenu + Radio, the same
// shape pluginPagesMenu uses; Select would be a second control style beside
// the mode switch it sits with.
func (a *App) reviewScopeMenu(c *ui.Context, msgs locales.Messages) {
	ui.MenuButton(c, a.scopeLabel(msgs), func(menu *ui.Menu) {
		menu.MaxWidth(360)
		if menu.Item(msgs.ReviewScope()).Radio(a.reviewScope.Kind == queue.ScopeAll).Chosen() {
			a.setReviewScope(c, queue.Scope{})
		}
		notes := a.scopeNotes()
		if len(notes) > 0 {
			menu.Submenu(msgs.ReviewScopeNotes(), func(sub *ui.Menu) {
				sub.MaxWidth(360)
				for _, note := range notes {
					note := note
					if sub.Item(note.Title).Radio(a.reviewScope.Kind == queue.ScopeNote && a.reviewScope.Note == note.ID).Chosen() {
						a.setReviewScope(c, queue.Scope{Kind: queue.ScopeNote, Note: note.ID})
					}
				}
			})
		}
		tags := a.scopeTags()
		if len(tags) > 0 {
			menu.Submenu(msgs.ReviewScopeTags(), func(sub *ui.Menu) {
				sub.MaxWidth(360)
				for _, tag := range tags {
					tag := tag
					if sub.Item(tag).Radio(a.reviewScope.Kind == queue.ScopeTag && a.reviewScope.Tag == tag).Chosen() {
						a.setReviewScope(c, queue.Scope{Kind: queue.ScopeTag, Tag: tag})
					}
				}
			})
		}
	}).Label(msgs.ReviewScopeMenu())
}

// scopeLabel names the scope the session is under: the whole library, or the
// note or tag it draws from.
func (a *App) scopeLabel(msgs locales.Messages) string {
	switch a.reviewScope.Kind {
	case queue.ScopeNote:
		if _, ok := a.data.note(a.reviewScope.Note); ok {
			return a.data.titleOf(a.reviewScope.Note)
		}
	case queue.ScopeTag:
		return "#" + a.reviewScope.Tag
	}
	return msgs.ReviewScope()
}

// scopeNotes lists the notes a scope can name: the snapshot's rows, in the
// notebook's order, without touching the store in the frame.
func (a *App) scopeNotes() []notes.Note {
	if a.lib == nil {
		return nil
	}
	return a.notesRows
}

// scopeTags lists the tags a scope can name, most notes first, from the
// snapshot the last reload derived.
func (a *App) scopeTags() []string {
	counts := make([]string, 0, len(a.data.tags))
	for tag := range a.data.tags {
		counts = append(counts, tag)
	}
	sort.Slice(counts, func(i, j int) bool {
		if a.data.tags[counts[i]] != a.data.tags[counts[j]] {
			return a.data.tags[counts[i]] > a.data.tags[counts[j]]
		}
		return counts[i] < counts[j]
	})
	return counts
}

// setReviewScope puts the session under a scope and re-derives the queue: the
// store's answer is the scope's, and Sync keeps the cards the new scope still
// holds. A scope with no cards shows the screen's empty state, which says so.
func (a *App) setReviewScope(c *ui.Context, scope queue.Scope) {
	if scope == a.reviewScope {
		return
	}
	a.reviewScope = scope
	a.reload()
	c.Announce(a.scopeLabel(m(c)))
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
		EmptyLabel: func() string {
			if a.reviewScope.Kind != queue.ScopeAll && a.reviewScopeEmpty() {
				return m(c).ReviewScopeEmpty()
			}
			return m(c).NoCards()
		},
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
		ID:         card.ID,
		Prompt:     face.Front,
		Answer:     face.Back,
		Hint:       face.Hint,
		TypeAnswer: face.TypeAnswer,
		State:      face.State,
	}
	if card.Ref.Frozen != nil {
		out.Frozen = card.Ref.Frozen.At
	}
	return out, true
}

// typing reports whether the card on screen is taking a typed answer, so the
// window's review keys leave the keyboard to its input.
func (a *App) typing() bool {
	id, ok := a.review.Current()
	if !ok {
		return false
	}
	card, ok := a.reviewCard(id)
	return ok && a.review.Typing(card)
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
