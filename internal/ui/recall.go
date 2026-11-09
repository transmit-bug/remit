package ui

import (
	"log/slog"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/screens/browse"
	"github.com/transmit-bug/remit/internal/ui/screens/recall"
	"github.com/transmit-bug/remit/internal/ui/screens/review"
)

// recallView draws the recall area's queue: the page itself lives in its own
// package, and this hands it the queue the app derived and the app's own
// actions (ADR 0004).
func (a *App) recallView(c *ui.Context) {
	recall.View(c, &a.recall, a.recallDeps(c))
}

// recallScopeView draws the review scope over the window, which the queue page's
// scope button opens. It is an overlay, so it is built at the window's root with
// the other overlays rather than inside the page's column.
func (a *App) recallScopeView(c *ui.Context) {
	if a.session != sessionNone || a.area != areaRecall || a.sub != subRecallQueue {
		return
	}
	recall.ScopeDialog(c, &a.recall, a.recallDeps(c))
}

// recallDeps is what the queue page asks of the app: what the last load derived,
// and one function per thing the user can do, each run here on the UI thread.
func (a *App) recallDeps(c *ui.Context) recall.Deps {
	return recall.Deps{
		Rows:        func() []recall.NoteRow { return a.recallRows },
		Counts:      func() queue.Counts { return a.recallCounts },
		Scope:       func() queue.Scope { return a.reviewScope },
		ScopeNotes:  a.scopeNoteChoices,
		ScopeTags:   a.scopeTags,
		ScopeCounts: func() recall.ScopeCounts { return a.recallScope },
		History:     func() []stats.DayCount { return a.recallHistory },
		Tomorrow:    func() int { return a.recallTomorrow },
		Lost:        func() int { return a.recallLost },
		SavedFilters: func() []recall.Filter {
			return recall.Filters(a.settings.SavedFilters)
		},
		Start:        a.startReview,
		ReviewNote:   func(note core.ID) { a.reviewNote(c, note) },
		SetFilter:    func(f recall.Filter) { a.setRecallFilter(c, f) },
		SaveFilter:   func(f recall.Filter) { a.saveRecallFilter(c, f) },
		RemoveFilter: func(f recall.Filter) { a.removeRecallFilter(c, f) },
		ApplyScope:   func(scope queue.Scope) { a.applyScope(c, scope) },
		Relink:       a.openLostCards,
		Reload: func() {
			a.loadRecall()
			c.Invalidate()
		},
	}
}

// loadRecall derives what the queue page reads: the session's counts, the notes
// the rows gather, each scope's live count, the half year's reviews and what
// tomorrow holds. It is a load path — the page reads what it leaves, so no frame
// asks the store — and it runs on entering Recall and after every write.
func (a *App) loadRecall() {
	a.recall.Loaded, a.recall.Err = false, ""
	a.recallRows, a.recallHistory = nil, nil
	a.recallScope = recall.ScopeCounts{}
	a.recallCounts, a.recallTomorrow, a.recallLost = queue.Counts{}, 0, 0
	if a.lib == nil {
		return
	}
	now := time.Now()
	fail := func(what string, err error) {
		slog.Error("remit: "+what, "err", err)
		a.recall.Err = err.Error()
	}
	counts, err := queue.CountsOf(a.lib, now, queue.Options{Scope: a.reviewScope})
	if err != nil {
		fail("recall counts", err)
		return
	}
	a.recallCounts = counts
	cards, err := queue.Cards(a.lib, now, queue.Options{Scope: a.reviewScope})
	if err != nil {
		fail("recall queue", err)
		return
	}
	// The whole library's queue is what the scope dialog counts: a choice is
	// taken before the session narrows to it.
	all, err := queue.Cards(a.lib, now, queue.Options{})
	if err != nil {
		fail("recall scopes", err)
		return
	}
	states, err := a.scheduledStates()
	if err != nil {
		fail("recall schedules", err)
		return
	}
	lost, err := a.lib.LostCards()
	if err != nil {
		fail("recall lost cards", err)
		return
	}
	presets := a.lib.Presets()
	a.recallScope = a.scopeCountsOf(all)
	a.recallRows = a.recallRowsOf(cards, states, presets.Hard.Lapses)
	a.recallLost = len(lost)
	from, to := recall.HeatRange(now)
	a.recallHistory = stats.ReviewsPerDay(a.lib, from, to)
	a.recallTomorrow = a.tomorrowCount(now)
	a.recall.Day, a.recall.Loaded = now, true
}

// scheduledStates is every queue-eligible card's schedule, by card: what the
// page says about a card — new, learning, due, hard — is read from here, so a
// row's count is the answer the session would take.
func (a *App) scheduledStates() (map[core.ID]schedule.State, error) {
	scheduled, err := a.lib.Scheduled()
	if err != nil {
		return nil, err
	}
	states := make(map[core.ID]schedule.State, len(scheduled))
	for _, entry := range scheduled {
		states[entry.Card.ID] = entry.State
	}
	return states, nil
}

// recallRowsOf groups the queue's cards by the note they came from, keeping the
// notes in the order the session would meet them and counting each one's
// paragraphs, cards, hard cards and what the page's filter keeps.
//
// A filter is a view of the queue, not a second queue: the cards are the ones
// the session would take, and the card's own schedule says which kind it is —
// never reviewed is new, answered Again is learning, the rest is review.
func (a *App) recallRowsOf(cards []core.Card, states map[core.ID]schedule.State, threshold int) []recall.NoteRow {
	if threshold <= 0 {
		threshold = 1
	}
	hard := a.hardCardsPerNote(states, threshold)
	rows := make([]recall.NoteRow, 0, len(a.data.notes))
	at := make(map[core.ID]int, len(a.data.notes))
	add := func(card core.Card) {
		note, ok := a.data.noteOfBlock(card.Ref.Block)
		if !ok {
			return
		}
		i, seen := at[note]
		if !seen {
			document, ok := a.data.note(note)
			if !ok {
				return
			}
			i = len(rows)
			at[note] = i
			rows = append(rows, recall.NoteRow{
				Note: note, Title: document.Title, Blocks: len(document.Blocks),
				Cards: a.data.cardsInNote(document), Hard: hard[note],
			})
		}
		rows[i].Due++
	}
	for _, card := range cards {
		if !a.recallKeeps(states[card.ID], threshold) {
			continue
		}
		add(card)
	}
	return rows
}

// recallKeeps reports whether the page's filter keeps a card, read from its
// schedule: hard is the lapse count the library counts from, and a card with no
// review at all is new rather than due.
func (a *App) recallKeeps(state schedule.State, threshold int) bool {
	switch a.recall.Filter {
	case recall.FilterDue:
		return state.Reviews > 0 && state.LastRating != schedule.Again
	case recall.FilterNew:
		return state.Reviews == 0
	case recall.FilterLearning:
		return state.Reviews > 0 && state.LastRating == schedule.Again
	case recall.FilterHard:
		return state.Lapses >= threshold
	}
	return true
}

// hardCardsPerNote counts the cards of each note that keep coming back wrong:
// the row's "3 missed" is the note's own record, not only today's.
func (a *App) hardCardsPerNote(states map[core.ID]schedule.State, threshold int) map[core.ID]int {
	out := map[core.ID]int{}
	for _, card := range a.data.cards {
		state, ok := states[card.ID]
		if !ok || state.Lapses < threshold {
			continue
		}
		if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
			out[note]++
		}
	}
	return out
}

// scopeCountsOf counts what each scope would take from the whole library's
// queue: the dialog's live number, derived once so the dialog reads it.
func (a *App) scopeCountsOf(cards []core.Card) recall.ScopeCounts {
	counts := recall.ScopeCounts{
		All:   len(cards),
		Notes: make(map[core.ID]int, len(a.data.notes)),
		Tags:  make(map[string]int, len(a.data.tags)),
	}
	for _, card := range cards {
		if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
			counts.Notes[note]++
		}
		block, ok := a.data.blocks[card.Ref.Block]
		if !ok {
			continue
		}
		for _, tag := range core.Tags(block.Text) {
			counts.Tags[tag]++
		}
	}
	return counts
}

// scopeNoteChoices are the notes the scope dialog can name, in the notebook's
// order: the snapshot's list, without touching the store in the frame.
func (a *App) scopeNoteChoices() []recall.Note {
	out := make([]recall.Note, 0, len(a.data.notes))
	for _, note := range a.data.notes {
		out = append(out, recall.Note{ID: note.ID, Title: note.Title})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// startReview enters the review session with what the session's scope holds:
// the queue page stays behind it, and Esc comes back to it.
func (a *App) startReview() {
	if a.lib == nil {
		return
	}
	a.review.Reset()
	a.review.Sync(a.due)
	a.failed = nil
	if a.review.Remaining() == 0 {
		return
	}
	a.session = sessionReview
	a.syncMode()
	a.loadCurrent()
}

// showCard enters the review session on one card, which is the whole queue:
// what a browse row asks for, and what a link to a card opens. The session ends
// when the card is graded, as a queue of one does.
func (a *App) showCard(id core.ID) {
	if a.lib == nil {
		return
	}
	// The card opens over the queue it belongs to, so leaving the session lands
	// on the page that holds it.
	a.setArea(areaRecall, subRecallQueue)
	a.review.Reset()
	a.review.Only(id)
	a.failed = nil
	a.session = sessionReview
	a.syncMode()
	a.loadCurrent()
}

// reviewNote narrows the session to one note and enters it: what a queue row
// does, so a note is reviewed without opening the scope dialog.
func (a *App) reviewNote(c *ui.Context, note core.ID) {
	a.setReviewScope(c, queue.Scope{Kind: queue.ScopeNote, Note: note})
	a.startReview()
}

// applyScope takes the scope the dialog showed and enters the session with it.
func (a *App) applyScope(c *ui.Context, scope queue.Scope) {
	a.setReviewScope(c, scope)
	a.startReview()
}

// openLostCards shows the cards whose paragraph is gone, in the card table's Lost
// filter: each row opens the card in review, where it is pointed at another
// paragraph or suspended.
func (a *App) openLostCards() {
	a.browse.Filter, a.browse.Selected = browse.Lost, -1
	a.setArea(areaRecall, subRecallCards)
}

// setRecallFilter narrows the page's rows. The filter is the page's, not the
// session's: what Start takes is still the scope's, so a filter never silently
// shrinks a sitting.
func (a *App) setRecallFilter(c *ui.Context, filter recall.Filter) {
	if a.recall.Filter == filter {
		return
	}
	a.recall.Filter = filter
	a.loadRecall()
	c.Invalidate()
}

// saveRecallFilter keeps the page's filter as a chip in settings.json. A filter
// already kept is kept once, and the choice stays in the session's language: the
// file holds the filter's id, so it reads the same in every language.
func (a *App) saveRecallFilter(c *ui.Context, filter recall.Filter) {
	for _, id := range a.settings.SavedFilters {
		if recall.FilterOf(id) == filter {
			c.Toast(m(c).RecallFilterSaved())
			return
		}
	}
	err := a.UpdateSettings(func(s *settings.Settings) {
		s.SavedFilters = append(s.SavedFilters, filter.Name())
	})
	if err != nil {
		a.report(c, "save filter", err)
		return
	}
	c.Toast(m(c).RecallFilterSaved())
}

// removeRecallFilter takes a kept filter out of settings.json: the chip is the
// only place it lives.
func (a *App) removeRecallFilter(c *ui.Context, filter recall.Filter) {
	err := a.UpdateSettings(func(s *settings.Settings) {
		kept := make([]string, 0, len(s.SavedFilters))
		for _, id := range s.SavedFilters {
			if recall.FilterOf(id) != filter {
				kept = append(kept, id)
			}
		}
		s.SavedFilters = kept
	})
	if err != nil {
		a.report(c, "remove filter", err)
		return
	}
	c.Toast(m(c).RecallFilterRemoved())
}

// cardsByBlock groups the snapshot's cards by the paragraph they follow, each
// group in ordinal order: the review session's context names the other cards of
// the paragraph, and the source line says which of them this one is.
func (a *App) cardsByBlock() map[core.ID][]core.Card {
	out := make(map[core.ID][]core.Card, len(a.data.cards))
	for _, card := range a.data.cards {
		out[card.Ref.Block] = append(out[card.Ref.Block], card)
	}
	for block := range out {
		group := out[block]
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Ordinal != group[j].Ordinal {
				return group[i].Ordinal < group[j].Ordinal
			}
			return group[i].ID < group[j].ID
		})
	}
	return out
}

// cardSource is where the card on screen came from, as the session's source line
// names it: its note, the paragraph, the cards the same paragraph asks, and the
// paragraphs on either side. A card whose note or paragraph is gone has no
// source to show.
func (a *App) cardSource(id core.ID) (review.Source, bool) {
	card, ok := a.data.card(id)
	if !ok {
		return review.Source{}, false
	}
	noteID, ok := a.data.noteOfBlock(card.Ref.Block)
	if !ok {
		return review.Source{}, false
	}
	note, ok := a.data.note(noteID)
	if !ok {
		return review.Source{}, false
	}
	at := -1
	for i, block := range note.Blocks {
		if block.ID == card.Ref.Block {
			at = i
			break
		}
	}
	if at < 0 {
		return review.Source{}, false
	}
	source := review.Source{
		Note: noteID, Title: note.Title,
		Paragraph: at + 1, Blocks: len(note.Blocks),
		Here: note.Blocks[at].Text,
	}
	if at > 0 {
		source.Prev = note.Blocks[at-1].Text
	}
	if at+1 < len(note.Blocks) {
		source.Next = note.Blocks[at+1].Text
	}
	group := a.blockCards[card.Ref.Block]
	source.Siblings = len(group)
	for i, sibling := range group {
		if sibling.ID == id {
			source.Index = i + 1
			break
		}
	}
	return source, true
}

// failedCardsOf is the cards this session did not remember, as the summary
// groups them: each one with the note it came from, gathered at the moment it
// was graded so the summary needs no second read.
func (a *App) failedCardsOf() []review.FailedCard {
	out := make([]review.FailedCard, 0, len(a.failed))
	for _, id := range a.failed {
		card, ok := a.data.card(id)
		if !ok {
			continue
		}
		note, ok := a.data.noteOfBlock(card.Ref.Block)
		if !ok {
			continue
		}
		out = append(out, review.FailedCard{Card: id, Note: note, Title: a.data.titleOf(note)})
	}
	return out
}
