package ui

import (
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/settings"
	"github.com/transmit-bug/remit/internal/ui/screens/recall"
	"github.com/transmit-bug/remit/internal/ui/screens/relink"
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
// what a card table row asks for, and what a link to a card opens. The session
// ends when the card is graded, as a queue of one does.
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

// reviewSelected enters the review session with exactly the rows the card table
// has chosen, in the order it lists them. A chosen card need not be due today —
// the user asked for these ones, and the session is the list rather than the
// day's queue.
func (a *App) reviewSelected(ids []core.ID) {
	if a.lib == nil || len(ids) == 0 {
		return
	}
	a.review.Reset()
	a.review.OnlyMany(ids)
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

// openLostCards shows the recall area's broken-links page, where a card whose
// paragraph is gone is pointed at another one, frozen or thawed (S35).
func (a *App) openLostCards() {
	a.setArea(areaRecall, subRecallRelink)
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

// The recall area's pages over the card library: the card table, the types, the
// presets, the history and the broken links. Each page's data is derived on a
// load path here — never in a frame — and every write goes through the store and
// ends in reload, which derives the pages again.

// recallStage draws the recall area's page that is up: the queue, the card
// table, the types, the presets, the history or the broken links. It is the
// area's one entry — the stage calls it and nothing else routes within Recall.
func (a *App) recallStage(c *ui.Context) {
	switch a.sub {
	case subRecallCards:
		a.cardTableView(c)
	case subRecallTypes:
		a.typesView(c)
	case subRecallPresets:
		a.presetsView(c)
	case subRecallHistory:
		a.historyView(c)
	case subRecallRelink:
		a.relinkPageView(c)
	default:
		a.recallView(c)
	}
}

// cardTableView draws the card table; the screen lives in its own package and
// this hands it the rows the app derived and the writes it owns.
func (a *App) cardTableView(c *ui.Context) {
	recall.CardsView(c, &a.cardTable, a.cardTableDeps(c))
}

// cardTableDeps is what the card table asks of the app: the rows, the facets and
// the writes, each run here on the UI thread.
func (a *App) cardTableDeps(c *ui.Context) recall.CardsDeps {
	return recall.CardsDeps{
		Rows:         func() []recall.CardRow { return a.cardRows },
		Total:        func() int { return a.cardTotal },
		Facets:       func() []recall.FacetGroup { return a.cardFacets },
		FacetLabel:   func(key string) string { return a.cardFacetLabel(c, key) },
		Reload:       a.loadCards,
		Sort:         func(ui.SortOrder) { a.loadCards() },
		ChooseFilter: func(f recall.CardFilter) { a.cardTable.CardFilter = f; a.loadCards() },
		Open:         a.openRow,
		Suspend: func(ids []core.ID) {
			result, err := a.lib.SuspendCards(ids, time.Now())
			a.bulkDone(c, result, err)
		},
		SetDue: func(ids []core.ID, days int) {
			result, err := a.lib.SetDue(ids, time.Now().AddDate(0, 0, days))
			a.bulkDone(c, result, err)
		},
		Freeze: func(ids []core.ID) {
			result, err := a.lib.FreezeCards(ids, time.Now())
			a.bulkDone(c, result, err)
		},
		AddTag: func(ids []core.ID, tag string) { a.addTagToCards(c, ids, tag) },
		ChangeType: func(ids []core.ID, typeID core.ID) {
			result, err := a.lib.ChangeCardTypes(ids, typeID)
			a.bulkDone(c, result, err)
		},
		Bury: func(ids []core.ID) {
			// Burying is for the rest of today: the paragraph's other cards come
			// back tomorrow, which is the day their sibling is learned by.
			until := dayStart(time.Now()).AddDate(0, 0, 1)
			result, err := a.lib.BuryCards(ids, until)
			a.bulkOutcome(c, result, err, m(c).RecallBulkBuried)
		},
		Delete: func(ids []core.ID) {
			result, err := a.lib.DeleteCards(ids)
			a.bulkOutcome(c, result, err, m(c).RecallBulkDeleted)
		},
		Review: a.reviewSelected,
		Types:  a.bulkTypeChoices,
		Items:  a.browseRowItems,
		PluginText: func(c *ui.Context, plugin, raw string) string {
			return a.pluginText(c, plugin, raw)
		},
		Run: func(c *ui.Context, ref contrib.CommandRef) { a.runContribution(c, ref) },
	}
}

// cardFacetLabel names one filter in the frame's language: a lifecycle state,
// a type by its name, or where a card's text comes from.
func (a *App) cardFacetLabel(c *ui.Context, key string) string {
	msgs := m(c)
	filter := recall.ParseFilterKey(key)
	switch {
	case filter.Status != nil:
		switch *filter.Status {
		case core.StatusNew:
			return msgs.RecallStatusNew()
		case core.StatusLearning:
			return msgs.RecallStatusLearning()
		case core.StatusReview:
			return msgs.RecallStatusReview()
		case core.StatusSuspended:
			return msgs.RecallStatusSuspended()
		}
	case filter.Type != "":
		return a.typeName(filter.Type)
	case filter.Ref != nil:
		switch *filter.Ref {
		case core.LinkLive:
			return msgs.RecallLinkLive()
		case core.LinkFrozen:
			return msgs.RecallLinkFrozen()
		case core.LinkLost:
			return msgs.Lost()
		}
	}
	return msgs.RecallCardsAll()
}

// addTagToCards tags the notes the chosen cards came from: the store writes the
// tag as one block at the end of each note, which is what typing it does.
func (a *App) addTagToCards(c *ui.Context, ids []core.ID, tag string) {
	seen := map[core.ID]bool{}
	var notes []core.ID
	for _, id := range ids {
		card, ok := a.data.card(id)
		if !ok {
			continue
		}
		note, ok := a.data.noteOfBlock(card.Ref.Block)
		if !ok || seen[note] {
			continue
		}
		seen[note] = true
		notes = append(notes, note)
	}
	if len(notes) == 0 {
		return
	}
	result, err := a.lib.AddTagToNotes(notes, tag)
	a.bulkDone(c, result, err)
}

// bulkTypeChoices are the types the bulk bar can move the chosen cards to, in
// catalog order: a type's name is library data, so it is shown as the library
// says it, the same words the type page and the table's column use.
func (a *App) bulkTypeChoices() []recall.TypeChoice {
	out := make([]recall.TypeChoice, 0, len(a.typeList))
	for _, typ := range a.typeList {
		out = append(out, recall.TypeChoice{ID: typ.ID, Name: typ.Name})
	}
	return out
}

// bulkDone reports a batch's outcome and derives the library again: a write
// that changed cards has changed the table, the queue and the counts.
func (a *App) bulkDone(c *ui.Context, result store.BulkResult, err error) {
	a.bulkOutcome(c, result, err, m(c).RecallBulkDone)
}

// bulkOutcome is bulkDone with the words the action reports: what a delete or a
// bury did is not what a suspend did, and calling a removed card "updated"
// would be a report the user cannot act on.
func (a *App) bulkOutcome(c *ui.Context, result store.BulkResult, err error, done func(int) string) {
	if err != nil {
		a.report(c, "bulk action", err)
		return
	}
	if failure := result.Err(); failure != nil {
		slog.Warn("remit: bulk action", "err", failure)
		c.Toast(m(c).RecallBulkPartial(result.Applied, len(result.Failed)))
	} else {
		// Nothing written is said too: a bury of a paragraph whose other cards
		// are already past tomorrow is a real answer, and silence reads as a
		// click that did nothing.
		c.Toast(done(result.Applied))
	}
	a.reload()
}

// loadCards derives the card table from one store query: the search syntax, the
// pane's filter and the column the user sorted by all become the query, so the
// table reads a page rather than filtering the library itself.
func (a *App) loadCards() {
	a.cardRows, a.cardTotal, a.cardFacets = nil, 0, nil
	if a.lib == nil {
		a.cardTable.Keep(0)
		return
	}
	query := a.cardQuery()
	rows, total, err := a.lib.QueryCards(query)
	if err != nil {
		slog.Error("remit: card table", "err", err)
		a.cardTable.Keep(0)
		return
	}
	if a.cardTable.Sort.Descending {
		// The store lists the soonest first; the column's second click asks for
		// the reverse, which is the page's own order.
		reverseCards(rows)
	}
	out := make([]recall.CardRow, 0, len(rows))
	for _, summary := range rows {
		out = append(out, a.cardRowOf(summary))
	}
	a.cardRows, a.cardTotal = out, total
	a.cardTable.Keep(len(out))
	a.cardFacets = a.cardFacetsOf()
}

// reverseCards turns a page around, for a column clicked a second time.
func reverseCards(rows []store.CardSummary) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

// cardQuery is what the card table asks the store: the typed syntax, the pane's
// filter laid over it, the sort the user chose and the library's own hard-card
// threshold for `is:hard`.
func (a *App) cardQuery() store.CardQuery {
	query := queue.ParseCardQuery(a.cardTable.Query)
	if filter := a.cardTable.CardFilter; filter.Status != nil || filter.Type != "" || filter.Ref != nil {
		if filter.Status != nil {
			query.Status = filter.Status
		}
		if filter.Type != "" {
			query.Type = filter.Type
		}
		if filter.Ref != nil {
			query.Ref = filter.Ref
		}
	}
	if query.MinLapses > 0 {
		// The parser knows `is:hard` is at least one lapse; the preset says how
		// many make a card hard in this library.
		query.MinLapses = max(query.MinLapses, a.lib.Presets().Hard.Lapses)
	}
	query.Sort = cardSort(a.cardTable.Sort.Column)
	return query
}

// cardSort is the store's order for a column. The three the schedule answers are
// the table's sortable columns; every other column keeps the due order, which is
// the order a review list is read in.
func cardSort(column string) store.CardSort {
	switch column {
	case "interval":
		return store.CardSortInterval
	case "lapses":
		return store.CardSortLapses
	case "note":
		return store.CardSortTitle
	}
	return store.CardSortDue
}

// cardRowOf reads one card summary into a row: what the card asks now, the note
// it came from, and the type it renders through.
func (a *App) cardRowOf(summary store.CardSummary) recall.CardRow {
	row := recall.CardRow{
		ID:       summary.ID,
		Status:   summary.Status,
		Interval: summary.IntervalDays,
		Lapses:   summary.Lapses,
		Due:      summary.Due,
		Lost:     summary.Ref == core.LinkLost,
		Type:     a.typeName(summary.Type),
	}
	if card, ok := a.data.card(summary.ID); ok {
		row.Prompt, _, _ = a.faces(card)
	}
	if summary.Note != "" {
		row.Note = a.data.titleOf(summary.Note)
	}
	return row
}

// cardFacetsOf is the filter pane's three groups with their counts. Each count
// is the store's answer to "what would the table hold if this facet were the
// filter", so the number beside a name is the number the user gets.
func (a *App) cardFacetsOf() []recall.FacetGroup {
	chosen := a.cardTable.CardFilter.Key()
	base := queue.ParseCardQuery(a.cardTable.Query)
	count := func(facet recall.CardFilter) int {
		query := base
		query.Status, query.Type, query.Ref = facet.Status, facet.Type, facet.Ref
		query.Limit = 1
		_, total, err := a.lib.QueryCards(query)
		if err != nil {
			return 0
		}
		return total
	}
	facet := func(key string) recall.Facet {
		filter := recall.ParseFilterKey(key)
		return recall.Facet{Key: key, Count: count(filter), Chosen: key == chosen}
	}

	statuses := []core.Status{core.StatusNew, core.StatusLearning, core.StatusReview, core.StatusSuspended}
	var statusGroup []recall.Facet
	for _, status := range statuses {
		status := status
		key := recall.CardFilter{Status: &status}.Key()
		statusGroup = append(statusGroup, facet(key))
	}
	var typeGroup []recall.Facet
	for _, typ := range a.lib.Types() {
		typeGroup = append(typeGroup, facet(recall.CardFilter{Type: typ.ID}.Key()))
	}
	var linkGroup []recall.Facet
	for _, ref := range []core.LinkState{core.LinkLive, core.LinkFrozen, core.LinkLost} {
		ref := ref
		linkGroup = append(linkGroup, facet(recall.CardFilter{Ref: &ref}.Key()))
	}
	return []recall.FacetGroup{
		{Key: "status", Items: statusGroup},
		{Key: "type", Items: typeGroup},
		{Key: "link", Items: linkGroup},
	}
}

// presetsView draws the library's presets page.
func (a *App) presetsView(c *ui.Context) {
	recall.PresetsView(c, &a.presets, recall.PresetsDeps{
		SetPresets:   a.setPresets,
		ResetPresets: a.resetPresets,
	})
}

// loadPresets copies the library's rhythm into the page and its snapshot.
func (a *App) loadPresets() {
	if a.lib == nil {
		a.presets.Sync(store.DefaultPresets())
		return
	}
	a.presets.Sync(a.lib.Presets())
}

// setPresets writes the rhythm the page holds into the library's remit.json and
// says where the write was made when it is refused.
func (a *App) setPresets(c *ui.Context, presets store.Presets) {
	a.presets.Err = ""
	if a.lib == nil {
		a.presets.Err = m(c).RecallPresetsNeedsLibrary()
		return
	}
	if err := a.lib.SetPresets(presets); err != nil {
		slog.Warn("remit: save presets", "err", err)
		a.presets.Err = m(c).RecallPresetsFailed(err.Error())
		a.loadPresets()
		return
	}
	a.presets.Sync(a.lib.Presets())
	// The limits pace the queue and the budget prices the day, so the reload
	// derives both again.
	a.reload()
}

// resetPresets writes the defaults back, behind the page's question.
func (a *App) resetPresets(c *ui.Context) {
	a.presets.Err = ""
	if a.lib == nil {
		a.presets.Err = m(c).RecallPresetsNeedsLibrary()
		return
	}
	if err := a.lib.SetPresets(store.DefaultPresets()); err != nil {
		slog.Warn("remit: reset presets", "err", err)
		a.presets.Err = m(c).RecallPresetsFailed(err.Error())
		a.loadPresets()
		return
	}
	a.presets.Sync(a.lib.Presets())
	a.reload()
	c.Toast(m(c).RecallPresetsResetDone())
}

// historyView draws the review history page.
func (a *App) historyView(c *ui.Context) {
	recall.HistoryView(c, &a.history, recall.HistoryDeps{Open: a.openHistoryEvent})
}

// loadHistory derives today's history and the week's table from the live log.
func (a *App) loadHistory() {
	a.history = recall.NewHistory()
	if a.lib == nil {
		return
	}
	now := time.Now()
	from, to := dayStart(now), dayStart(now).AddDate(0, 0, 1)
	today := stats.HistoryOf(a.lib, from, to)
	week := stats.HistoryOf(a.lib, weekStart(now), to)
	a.history.Total, a.history.Again = today.Total, today.Again
	a.history.Grades = today.Grades
	a.history.Minutes = stats.TimeShareOf(a.lib, from, to).Review
	a.history.Events = a.historyEvents(today.Events)
	a.history.Week = historyDays(week.Events, weekStart(now), from)
}

// historyEvents reads the log's events into what the timeline shows: a note's
// title for a session, the card's prompt and its note for an answer.
func (a *App) historyEvents(events []stats.HistoryEvent) []recall.HistoryEvent {
	out := make([]recall.HistoryEvent, 0, len(events))
	for _, event := range events {
		row := recall.HistoryEvent{
			At:     event.At,
			Rating: event.Rating,
			From:   event.IntervalFrom,
			To:     event.IntervalTo,
		}
		if event.Kind == stats.KindReading {
			row.Kind = recall.EventReading
			row.Title = event.NoteTitle
			row.NoteID = a.data.byTitle[core.NormalizeTitle(event.NoteTitle)]
			out = append(out, row)
			continue
		}
		row.Kind = recall.EventCard
		row.CardID = event.CardID
		if card, ok := a.data.card(event.CardID); ok {
			row.Title, _, _ = a.faces(card)
			if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
				row.Note = a.data.titleOf(note)
				row.NoteID = note
			}
		}
		out = append(out, row)
	}
	return out
}

// openHistoryEvent goes back to what an event recorded: the note a session was
// on, or the note behind an answered card.
func (a *App) openHistoryEvent(event recall.HistoryEvent) {
	switch {
	case event.NoteID != "":
		a.openNote(event.NoteID)
	case event.CardID != "":
		a.openRow(event.CardID)
	}
}

// relinkPageView draws the broken-links page.
func (a *App) relinkPageView(c *ui.Context) {
	relink.PageView(c, &a.relinkPage, a.relinkPageDeps(c))
}

// loadRelink derives the page's three lists: the lost cards, the frozen ones,
// and the blocks a lost card can be pointed at.
func (a *App) loadRelink() {
	a.lostCards, a.frozenCards = nil, nil
	if a.lib == nil {
		return
	}
	if lost, err := a.lib.LostCards(); err == nil {
		for _, card := range lost {
			row := relink.CardRow{ID: card.ID}
			if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
				row.Note = a.data.titleOf(note)
			}
			a.lostCards = append(a.lostCards, row)
		}
	} else {
		slog.Error("remit: lost cards", "err", err)
	}
	frozen := core.LinkFrozen
	summaries, _, err := a.lib.QueryCards(store.CardQuery{Ref: &frozen, Sort: store.CardSortDue})
	if err != nil {
		slog.Error("remit: frozen cards", "err", err)
	}
	for _, summary := range summaries {
		row := relink.CardRow{ID: summary.ID}
		if card, ok := a.data.card(summary.ID); ok {
			row.Prompt, _, _ = a.faces(card)
		}
		if summary.Note != "" {
			row.Note = a.data.titleOf(summary.Note)
		}
		a.frozenCards = append(a.frozenCards, row)
	}
	a.relinkQuery, a.relinkBlocks, a.relinkLoaded = "", nil, false
}

// relinkPageBlocks is the blocks a query finds, from the snapshot: the page asks
// for the same query every frame, so the answer is kept until the query moves.
func (a *App) relinkPageBlocks(query string) []relink.BlockRow {
	if a.relinkLoaded && query == a.relinkQuery {
		return a.relinkBlocks
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var rows []relink.BlockRow
	for _, note := range a.data.notes {
		for i, block := range note.Blocks {
			if block.ID == "" {
				continue
			}
			text := strings.Join(strings.Fields(block.Text), " ")
			if q != "" && !strings.Contains(strings.ToLower(note.Title+" "+text), q) {
				continue
			}
			rows = append(rows, relink.BlockRow{
				Block: block.ID, Note: note.ID, NoteTitle: note.Title,
				Paragraph: i + 1, Text: text,
			})
		}
	}
	a.relinkQuery, a.relinkBlocks, a.relinkLoaded = query, rows, true
	return rows
}

// relinkPageDeps is what the page asks of the app: the three lists and the four
// writes a lost or frozen card carries.
func (a *App) relinkPageDeps(c *ui.Context) relink.PageDeps {
	return relink.PageDeps{
		Lost:   func() []relink.CardRow { return a.lostCards },
		Frozen: func() []relink.CardRow { return a.frozenCards },
		Blocks: a.relinkPageBlocks,
		Relink: func(card, block core.ID) { a.relinkTo(c, card, block) },
		RelinkAll: func(block core.ID) {
			a.relinkEvery(c, block)
		},
		Freeze: func(card, block core.ID) { a.freezeAt(c, card, block) },
		Thaw:   func(card core.ID) { a.thaw(c, card) },
	}
}

// relinkTo points a lost card at a block, live again, and writes it through the
// store. Only the reference changes: the card keeps its status, its reviews and
// its due date.
func (a *App) relinkTo(c *ui.Context, cardID, blockID core.ID) {
	card, ok := a.data.card(cardID)
	if !ok || blockID == "" {
		return
	}
	card.Ref = core.Reference{Block: blockID}
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "re-link card", err)
		return
	}
	a.reload()
	c.Toast(m(c).Relinked())
}

// relinkEvery points every lost card at one block. It is the page's bulk
// repair, asked for behind a question because the library keeps no undo for it.
func (a *App) relinkEvery(c *ui.Context, blockID core.ID) {
	if blockID == "" {
		return
	}
	applied, failed := 0, 0
	for _, row := range a.lostCards {
		card, ok := a.data.card(row.ID)
		if !ok {
			continue
		}
		card.Ref = core.Reference{Block: blockID}
		if err := a.lib.SaveCard(card); err != nil {
			slog.Warn("remit: re-link card", "card", row.ID, "err", err)
			failed++
			continue
		}
		applied++
	}
	a.reload()
	if failed > 0 {
		c.Toast(m(c).RecallBulkPartial(applied, failed))
		return
	}
	if applied > 0 {
		c.Toast(m(c).RecallRelinkAllDone(applied))
	}
}

// freezeAt points a card at a block and keeps what that block says now, so the
// card stops following the text it was repaired with.
func (a *App) freezeAt(c *ui.Context, cardID, blockID core.ID) {
	card, ok := a.data.card(cardID)
	if !ok || blockID == "" {
		return
	}
	card.Ref = core.Reference{Block: blockID}.Freeze(a.data.blocks, time.Now())
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "freeze card", err)
		return
	}
	a.reload()
	c.Toast(m(c).RecallRelinkFrozenDone())
}

// thaw makes a frozen card follow its block again.
func (a *App) thaw(c *ui.Context, cardID core.ID) {
	card, ok := a.data.card(cardID)
	if !ok {
		return
	}
	card.Ref = card.Ref.Thaw()
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "thaw card", err)
		return
	}
	a.reload()
	c.Toast(m(c).RecallRelinkThawed())
}

// typeName is a card type's name, by its ID, as the snapshot's catalog holds it.
func (a *App) typeName(id core.ID) string {
	for _, typ := range a.typeList {
		if typ.ID == id {
			return typ.Name
		}
	}
	return string(id)
}

// weekStart is midnight of the Monday of t's week.
func weekStart(t time.Time) time.Time {
	t = dayStart(t)
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}

// historyDays groups the week's events into one row a day, from the week's
// Monday to today, so the table keeps the days that recorded nothing.
func historyDays(events []stats.HistoryEvent, from, today time.Time) []recall.DayRow {
	var rows []recall.DayRow
	index := map[string]int{}
	for day := from; !day.After(today); day = day.AddDate(0, 0, 1) {
		index[day.Format("2006-01-02")] = len(rows)
		rows = append(rows, recall.DayRow{Day: day})
	}
	for _, event := range events {
		key := dayStart(event.At.In(today.Location())).Format("2006-01-02")
		i, ok := index[key]
		if !ok {
			continue
		}
		rows[i].Reviews++
		if event.Rating == schedule.Again {
			rows[i].Again++
		}
	}
	return rows
}

// openRow shows the row chosen in the card table or the history: its note in
// the notebook, or the card in review when the paragraph it came from is gone —
// where a lost card can still be suspended.
func (a *App) openRow(id core.ID) {
	card, ok := a.data.card(id)
	if !ok {
		return
	}
	if note, ok := a.data.noteOfBlock(card.Ref.Block); ok {
		a.notes.Selected = string(note)
		a.setMode(modeNotes)
		return
	}
	a.showCard(card.ID)
}
