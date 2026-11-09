package ui

import (
	"log/slog"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/screens/recall"
	"github.com/transmit-bug/remit/internal/ui/screens/relink"
)

// The recall area's pages over the card library: the card table, the presets,
// the history and the broken links. Each page's data is derived on a load path
// here — never in a frame — and every write goes through the store and ends in
// reload, which derives the pages again.

// recallStage draws the recall area's page that is up: the queue, the card
// table, the types, the presets, the history or the broken links.
func (a *App) recallStage(c *ui.Context, tk design.Tokens) {
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
		a.reviewView(c, tk)
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
		ChooseFilter: func(f recall.Filter) { a.cardTable.Filter = f; a.loadCards() },
		Open:         a.openBrowseRow,
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
		Items:  a.browseRowItems(),
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

// bulkDone reports a batch's outcome and derives the library again: a write
// that changed cards has changed the table, the queue and the counts.
func (a *App) bulkDone(c *ui.Context, result store.BulkResult, err error) {
	if err != nil {
		a.report(c, "bulk action", err)
		return
	}
	if failure := result.Err(); failure != nil {
		slog.Warn("remit: bulk action", "err", failure)
		c.Toast(m(c).RecallBulkPartial(result.Applied, len(result.Failed)))
	} else if result.Applied > 0 {
		c.Toast(m(c).RecallBulkDone(result.Applied))
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
	if filter := a.cardTable.Filter; filter.Status != nil || filter.Type != "" || filter.Ref != nil {
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
	chosen := a.cardTable.Filter.Key()
	base := queue.ParseCardQuery(a.cardTable.Query)
	count := func(facet recall.Filter) int {
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
		key := recall.Filter{Status: &status}.Key()
		statusGroup = append(statusGroup, facet(key))
	}
	var typeGroup []recall.Facet
	for _, typ := range a.lib.Types() {
		typeGroup = append(typeGroup, facet(recall.Filter{Type: typ.ID}.Key()))
	}
	var linkGroup []recall.Facet
	for _, ref := range []core.LinkState{core.LinkLive, core.LinkFrozen, core.LinkLost} {
		ref := ref
		linkGroup = append(linkGroup, facet(recall.Filter{Ref: &ref}.Key()))
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
		a.openBrowseRow(event.CardID)
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
