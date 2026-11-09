package ui

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// cardGroups lists the note's card-bearing paragraphs in document order, which
// is what the block menu counts before it deletes one.
func (a *App) cardGroups(note core.Note) []notes.CardGroup {
	var out []notes.CardGroup
	for i, block := range note.Blocks {
		var cards []core.Card
		for _, card := range a.data.cards {
			// A suspended card feeds nothing: the badge counts what the
			// paragraph still puts in the queue.
			if card.Ref.Block == block.ID && card.Status != core.StatusSuspended {
				cards = append(cards, card)
			}
		}
		if len(cards) == 0 {
			// A paragraph that feeds nothing has no badge to show.
			continue
		}
		out = append(out, notes.CardGroup{Paragraph: i + 1, Cards: cards})
	}
	return out
}

// reconcileCards applies the note's shape to its cards after a save. A paragraph
// that asks something makes the card it asks for — one per blank, both ways, or
// one — unless a card already holds that place; a card whose place the paragraph
// no longer asks for is suspended, out of the queue with its history kept.
// Nothing is deleted.
func (a *App) reconcileCards(c *ui.Context, note core.Note) {
	if a.lib == nil || note.ID == "" {
		return
	}
	wrote := false
	for _, block := range note.Blocks {
		asked, fromPlugin := a.deriveCards(block.Text)
		if !fromPlugin {
			// Without a plugin a paragraph makes no new card; the ones it
			// already has keep reading it.
			continue
		}
		type place struct {
			typ     core.ID
			ordinal int
		}
		wanted := make(map[place]bool, len(asked))
		for _, d := range asked {
			wanted[place{d.Type, d.Ordinal}] = true
		}
		held := map[place]bool{}
		for _, card := range a.data.cards {
			if card.Ref.Block != block.ID || card.Status == core.StatusSuspended {
				continue
			}
			place := place{card.Type, card.Ordinal}
			if !wanted[place] {
				// The paragraph stopped asking this: it leaves the queue.
				card.Status = core.StatusSuspended
				if err := a.lib.SaveCard(card); err != nil {
					a.report(c, "suspend card", err)
					return
				}
				wrote = true
				continue
			}
			held[place] = true
		}
		missing := 0
		for range wanted {
			missing++
		}
		if missing == len(held) && missing == len(wanted) {
			continue
		}
		if missing > 0 {
			// A card the note asks for and the library does not hold — a new
			// separator, a new blank — is made now.
			if _, err := a.lib.CreateCards(block.ID, asked); err != nil && !errors.Is(err, store.ErrNothingToAsk) {
				a.report(c, "make card", err)
				return
			}
			wrote = true
		}
	}
	if wrote {
		a.reload()
	}
}

// makeCard makes a card from one of the note's paragraphs.
func (a *App) makeCard(c *ui.Context, i int) {
	if i >= len(a.notes.Draft.Blocks) {
		return
	}
	block := a.notes.Draft.Blocks[i]
	derived, ok := a.deriveCards(block.Text)
	asks := false
	if ok {
		for _, card := range derived {
			for _, value := range card.Fields {
				if value != "" {
					asks = true
				}
			}
		}
	}
	if !asks {
		c.Toast(m(c).BlockEmpty())
		return
	}
	if block.ID == "" {
		// A block read from a hand-written note has no ID until the note is
		// saved; saving now mints one that the card can point at.
		saved, ok := a.saveNote(c, a.notes.Draft)
		if !ok {
			return
		}
		a.notes.Keep(saved)
		block = a.notes.Draft.Blocks[i]
	}
	if _, err := a.lib.CreateCards(block.ID, derived); err != nil {
		a.report(c, "make card", err)
		return
	}
	// A card made now is due now, so reloading puts it at the end of the queue
	// and the card the user is on does not move.
	a.reload()
	c.Toast(m(c).CardMade(i + 1))
}

// makeCardAtCaret makes a card from the paragraph the caret is in, which is what
// the palette's "Make card" does while card creation is a command rather than a
// gesture.
func (a *App) makeCardAtCaret(c *ui.Context) {
	a.notes.Follow(a.notesRows)
	at, ok := a.notes.ParagraphAtCaret()
	if !ok {
		return
	}
	a.makeCard(c, at)
}

// cardEditState is the card a dialog edits: the card itself, or the block a card
// would point at when it has none, with one value per field of its type. The
// fields follow the type's template, so a type a plugin supplies is edited with
// its own fields (ui-redesign-v2 §4.4 S18).
type cardEditState struct {
	open   bool
	block  core.ID     // the block the card points at
	card   core.ID     // the card open, "" while the block feeds none
	typ    core.ID     // the type whose template the fields follow
	types  []core.Type // every type the library holds, built-ins included
	choice string      // the type's name, as the Select binds it
	fields []string    // one value per field of the type, in its order
	focus  []ui.Handle // one handle per field, for the first one's focus
	frozen bool        // the freeze switch, as the dialog holds it
	asking bool        // the delete is waiting for its answer
}

// openCardEdit opens the field editor on a card, filled with what the card says
// now: its own edited value for a field, empty for one it takes from its
// paragraph.
func (a *App) openCardEdit(c *ui.Context, id core.ID) {
	card, ok := a.data.card(id)
	if !ok {
		return
	}
	a.openCardAt(c, card.Ref.Block, card.ID)
}

// openCardEditor opens the editor on a block: the card it feeds, or the fields a
// card of the chosen type starts with when it feeds none. It is what a block's
// badge and the block inspector open.
func (a *App) openCardEditor(c *ui.Context, block core.ID) { a.openCardAt(c, block, "") }

// openCardAt fills the editor from the card named — the one a badge points at,
// which a paragraph of two blanks has one of — and otherwise from the block's
// card. A paragraph that is not a card yet gets empty fields.
func (a *App) openCardAt(c *ui.Context, block, id core.ID) {
	if a.lib == nil || block == "" {
		return
	}
	edit := cardEditState{open: true, block: block, types: a.lib.Types(), typ: core.TypeBasic}
	card, has := a.data.card(id)
	if !has {
		card, has = a.blockCard(block)
	}
	if has {
		edit.card, edit.typ, edit.frozen = card.ID, card.Type, card.Ref.Frozen != nil
	}
	edit.useType(edit.typ)
	if typ, ok := findType(edit.types, edit.typ); ok && has {
		for i, name := range typ.Fields {
			edit.fields[i] = card.Fields[name]
		}
	}
	a.cardEdit = edit
	if len(edit.focus) > 0 {
		a.cardEdit.focus[0].Focus(c)
	}
}

// blockCard is the card a block feeds: the one still in the queue, when the
// block feeds several or its card was suspended.
func (a *App) blockCard(block core.ID) (core.Card, bool) {
	var held core.Card
	found := false
	for _, card := range a.data.cards {
		if card.Ref.Block != block {
			continue
		}
		if card.Status != core.StatusSuspended {
			return card, true
		}
		if !found {
			held, found = card, true
		}
	}
	return held, found
}

// useType fills the dialog's fields for one of the library's types: one empty
// value per field, in the type's order, keeping what the user typed for a field
// the new type has too.
func (e *cardEditState) useType(id core.ID) {
	held := map[string]string{}
	if current, ok := findType(e.types, e.typ); ok {
		for i, name := range current.Fields {
			if i < len(e.fields) {
				held[name] = e.fields[i]
			}
		}
	}
	typ, ok := findType(e.types, id)
	if !ok {
		return
	}
	e.typ, e.choice = typ.ID, typ.Name
	e.fields = make([]string, len(typ.Fields))
	e.focus = make([]ui.Handle, len(typ.Fields))
	for i, name := range typ.Fields {
		e.fields[i] = held[name]
	}
}

// typeNames is the type selector's options, in the order the library lists the
// types; the card's own type is always one of them.
func (e *cardEditState) typeNames() []string {
	names := make([]string, len(e.types))
	for i, typ := range e.types {
		names[i] = typ.Name
	}
	return names
}

// fieldName is the name the i-th input carries: the type names its own fields,
// so they are shown as the type spells them.
func (e *cardEditState) fieldName(i int) string {
	typ, ok := findType(e.types, e.typ)
	if !ok || i >= len(typ.Fields) {
		return ""
	}
	return typ.Fields[i]
}

// findType is the type with this id, and false for one the library dropped.
func findType(types []core.Type, id core.ID) (core.Type, bool) {
	for _, typ := range types {
		if typ.ID == id {
			return typ, true
		}
	}
	return core.Type{}, false
}

// typeNamed is the type a selector's answer names.
func typeNamed(types []core.Type, name string) (core.Type, bool) {
	for _, typ := range types {
		if typ.Name == name {
			return typ, true
		}
	}
	return core.Type{}, false
}

// cardEditView draws the card editor over the window: one input per field of the
// card's type, the type selector, how the card's source stands, the freeze
// switch, and the three actions. A block that feeds no card yet shows empty
// fields, a delete it cannot do, and the sentence saying why (§4.13).
func (a *App) cardEditView(c *ui.Context, tk design.Tokens) {
	if !a.cardEdit.open {
		return
	}
	msgs := m(c)
	edit := &a.cardEdit
	freeze, submit := false, false

	ui.DialogBase(c, &edit.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.ArrivePanel(kit.StylePanel(c, panel.Width(tk.SummaryWidth).
			Padding(tk.SpaceL).Gap(tk.SpaceM)), 0)
		kit.ArriveScrim(backdrop, 0)

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, msgs.EditCard()).FontSize(tk.TypeH2).Bold().Grow(1)
				if card, ok := a.data.card(edit.card); ok {
					_, _, state := a.faces(card)
					linkBadge(c, tk, msgs, state)
				}
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { edit.open = false })
			})

			// The type decides the fields: choosing another one regenerates them
			// from that type's template, keeping what a shared field already
			// holds. Select reports a change in the following pass, so the fields
			// are rebuilt before they are drawn.
			if ui.Select(c, &edit.choice, edit.typeNames()).
				Label(msgs.CardType()).MinHeight(tk.Control).Changed() {
				if typ, ok := typeNamed(edit.types, edit.choice); ok {
					edit.useType(typ.ID)
				}
			}
			for i := range edit.fields {
				name := edit.fieldName(i)
				area := ui.TextArea(c, &edit.fields[i]).
					Label(name).Placeholder(name).MinHeight(tk.Control * 2).
					Bind(&edit.focus[i])
				if area.Submitted() {
					submit = true
				}
			}
			ui.Text(c, msgs.CardFieldsHint()).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted).MaxWidth(tk.SummaryWidth)

			// A field, so the label names the switch and a click on it reaches it.
			ui.Field(c, msgs.Frozen(), func() {
				if ui.Switch(c, &edit.frozen).Label(msgs.Frozen()).Changed() {
					freeze = true
				}
			}).Description(msgs.CardFreezeHint())

			ui.Row(c).Gap(tk.SpaceS).Children(func() {
				ui.PrimaryButton(c, msgs.Save()).MinHeight(tk.Control).
					OnClick(func() { a.writeCardEdit(c) })
				remove := ui.Button(c, msgs.CardDelete()).MinHeight(tk.Control)
				if edit.card == "" {
					// Nothing to delete, and the line below says why: a disabled
					// control shows no tooltip.
					remove.Disabled(true)
				} else {
					remove.OnClick(func() { edit.asking = true })
				}
				ui.Button(c, msgs.Cancel()).MinHeight(tk.Control).
					OnClick(func() { edit.open = false })
			})
			if edit.card == "" {
				ui.Text(c, msgs.CardNotACard()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			}
		})
	})

	// The confirmation is its own overlay over the editor: a delete asks first,
	// and Escape leaves the card where it is.
	if edit.asking {
		switch ui.AlertDialog(c, &edit.asking, msgs.CardDeleteTitle(), msgs.CardDeleteBody(),
			msgs.Cancel(), msgs.CardDeleteConfirm()) {
		case 1:
			a.deleteCardNow(c)
		}
	}
	if freeze {
		a.cardFreeze(c, edit.card, edit.frozen)
	}
	if submit {
		a.writeCardEdit(c)
	}
}

// linkBadge reports where a card's text comes from: the paragraph as it is, the
// snapshot it was frozen at, or nothing at all.
func linkBadge(c *ui.Context, tk design.Tokens, msgs locales.Messages, state core.LinkState) {
	switch state {
	case core.LinkFrozen:
		kit.StateBadge(c, "snowflake", msgs.Frozen(), tk.Warning)
	case core.LinkLost:
		kit.StateBadge(c, "unlink", msgs.Lost(), tk.Danger)
	default:
		kit.StateBadge(c, "link-2", msgs.CardLinkLive(), tk.TextMuted)
	}
}

// writeCardEdit stores what the dialog holds: the type's fields on the card, or
// a card of the chosen type for a block that had none. An emptied field goes
// back to following its paragraph.
func (a *App) writeCardEdit(c *ui.Context) {
	edit := &a.cardEdit
	typ, ok := findType(edit.types, edit.typ)
	if !ok {
		edit.open = false
		return
	}
	fields := map[string]string{}
	for i, name := range typ.Fields {
		if i < len(edit.fields) && strings.TrimSpace(edit.fields[i]) != "" {
			fields[name] = edit.fields[i]
		}
	}
	if edit.card == "" {
		// The store makes the card from its type; the values typed here are the
		// card's own, so they are written straight after.
		made, err := a.lib.CreateCards(edit.block, []core.Derived{{Type: typ.ID}})
		if err != nil && !errors.Is(err, store.ErrNothingToAsk) {
			a.report(c, "make card", err)
			return
		}
		if len(made) > 0 && len(fields) > 0 {
			card := made[0]
			card.Fields, card.Edited = fields, true
			if err := a.lib.SaveCard(card); err != nil {
				a.report(c, "edit card", err)
				return
			}
		}
		edit.open = false
		a.reload()
		return
	}
	card, ok := a.data.card(edit.card)
	if !ok {
		edit.open = false
		return
	}
	card.Type, card.Fields, card.Edited = typ.ID, fields, len(fields) > 0
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "edit card", err)
		return
	}
	edit.open = false
	a.reload()
}

// cardFreeze stops a card following its paragraph, or lets it follow again: the
// frozen card keeps the words it was made from, the thawed one reads the
// paragraph as it is now.
func (a *App) cardFreeze(c *ui.Context, id core.ID, frozen bool) {
	card, ok := a.data.card(id)
	if !ok || frozen == (card.Ref.Frozen != nil) {
		return
	}
	if frozen {
		card.Ref = card.Ref.Freeze(a.data.blocks, time.Now())
	} else {
		card.Ref = card.Ref.Thaw()
	}
	if err := a.lib.SaveCard(card); err != nil {
		a.report(c, "freeze card", err)
		return
	}
	a.reload()
}

// deleteCardNow removes the card the editor is on. The paragraph stays, and the
// card's history goes with the card: this is the one destructive control the
// window has over a review record, which is why it asks first.
func (a *App) deleteCardNow(c *ui.Context) {
	edit := &a.cardEdit
	if edit.card == "" {
		return
	}
	if err := a.lib.DeleteCard(edit.card); err != nil {
		a.report(c, "delete card", err)
		return
	}
	edit.open, edit.card = false, ""
	a.reload()
}

// cardDetailState is the card the detail dialog shows: the card, and its
// schedule history as the store answered when the dialog opened. The rows are
// derived once, off the frame, so a frame only prints them.
type cardDetailState struct {
	open bool
	card core.ID
	rows []cardHistoryRow
}

// cardHistoryRow is one review of a card: when it was answered, with what, and
// how long the answer scheduled it for.
type cardHistoryRow struct {
	At       time.Time
	Rating   schedule.Rating
	Interval int // days until the card is due again
}

// openCardDetail shows one card's source and its schedule history: what a card
// table row and a block inspector's card row open (ui-redesign-v2 §4.4 S19).
func (a *App) openCardDetail(c *ui.Context, id core.ID) {
	card, ok := a.data.card(id)
	if !ok {
		return
	}
	detail := cardDetailState{open: true, card: id}
	history, err := a.lib.History(id)
	if err != nil {
		a.report(c, "read card history", err)
		return
	}
	// Each row is the schedule after that one review: the applier is fed the
	// card's own events in order, so a row says what the answer was worth at the
	// moment it was given.
	applier := schedule.NewApplier(a.lib.ScheduleParams())
	for _, event := range history {
		if err := applier.Apply(event); err != nil {
			break
		}
		state := applier.States()[card.ID]
		detail.rows = append(detail.rows, cardHistoryRow{
			At:       event.At,
			Rating:   event.Rating,
			Interval: int(math.Round(state.Due.Sub(event.At).Hours() / 24)),
		})
	}
	a.cardDetail = detail
}

// cardDetailView draws the card's detail over the window: the paragraph it reads,
// the way back to it, its schedule history, and the two actions a card carries —
// freeze or thaw, and edit.
func (a *App) cardDetailView(c *ui.Context, tk design.Tokens) {
	if !a.cardDetail.open {
		return
	}
	msgs := m(c)
	detail := &a.cardDetail
	card, has := a.data.card(detail.card)

	// Every action here is a handler, which runs after the pass: it closes the
	// dialog and does its work there, so the frame that follows draws the result.

	ui.DialogBase(c, &detail.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.ArrivePanel(kit.StylePanel(c, panel.Width(tk.SummaryWidth).
			Padding(tk.SpaceL).Gap(tk.SpaceM)), 0)
		kit.ArriveScrim(backdrop, 0)

		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, msgs.CardDetail()).FontSize(tk.TypeH2).Bold().Grow(1)
				if has {
					_, _, state := a.faces(card)
					linkBadge(c, tk, msgs, state)
				}
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { detail.open = false })
			})
			if has {
				prompt, _, _ := a.faces(card)
				ui.Text(c, prompt).FontSize(tk.TypeUI).TextColor(tk.TextMuted).MaxWidth(tk.SummaryWidth)
			}

			// The source, read before leaving for it: a card is what its
			// paragraph says, so the dialog shows both.
			if note, block, ok := a.cardOrigin(card); ok {
				ui.Column(c).FillWidth().Gap(tk.SpaceXS).Padding(tk.SpaceM).
					Background(tk.Sunken).Radius(tk.RadiusControl).Children(func() {
					ui.Text(c, msgs.CardSource()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
					if title, text := a.sourceText(note, block); text != "" {
						ui.Text(c, title).FontSize(tk.TypeLabel).TextColor(tk.TextMuted).SingleLine()
						ui.Text(c, text).FontSize(tk.TypeUI).MaxWidth(tk.SummaryWidth)
					} else {
						ui.Text(c, msgs.LostBody()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
					}
				})
				ui.Button(c, "").Label(msgs.CardBackToSource()).
					Children(func() { tk.IconText(c, "arrow-left", msgs.CardBackToSource(), design.IconSmall) }).
					MinHeight(tk.Control).Justify(ui.Start).
					OnClick(func() {
						detail.open = false
						a.openNoteAt(c, note, block)
					})
			}

			ui.Text(c, msgs.CardHistory()).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			if len(detail.rows) == 0 {
				ui.Text(c, msgs.CardHistoryEmpty()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			} else {
				ui.List(c, nil, len(detail.rows), func(i int) { a.historyRow(c, tk, detail.rows[i]) }).
					MaxHeight(tk.Control * 6).Label(msgs.CardHistory())
			}

			ui.Row(c).Gap(tk.SpaceS).Children(func() {
				ui.Button(c, msgs.EditCard()).MinHeight(tk.Control).OnClick(func() {
					detail.open = false
					a.openCardEdit(c, detail.card)
				})
				if !has {
					return
				}
				label, frozen := msgs.CardFreeze(), card.Ref.Frozen == nil
				if !frozen {
					label = msgs.CardThaw()
				}
				ui.Button(c, label).MinHeight(tk.Control).OnClick(func() {
					a.cardFreeze(c, detail.card, frozen)
				})
			})
		})
	})
}

// historyRow is one review as the list shows it: when, with what, and how long
// the answer scheduled it for.
func (a *App) historyRow(c *ui.Context, tk design.Tokens, row cardHistoryRow) {
	msgs := m(c)
	ui.Row(c).FillWidth().Gap(tk.SpaceS).AlignItems(ui.Center).
		Padding(tk.SpaceXS, 0).Children(func() {
		ui.Text(c, i18n.FormatRelativeTime(c.Locale(), relativeDays(c.Now(), row.At), i18n.RelativeDay,
			i18n.RelativeTimeOptions{Auto: true})).
			FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		ui.Text(c, ratingLabel(msgs, row.Rating)).FontSize(tk.TypeUI).
			TextColor(ratingColour(tk, row.Rating))
		ui.Spacer(c)
		ui.Text(c, msgs.CardHistoryInterval(row.Interval)).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted)
	})
}

// relativeDays is how many days before now t was, as FormatRelativeTime counts
// them: negative for the past, rounded to the day the reader counts in.
func relativeDays(now, t time.Time) float64 {
	return math.Round(t.Sub(now).Hours() / 24)
}

// ratingLabel names a grade as the review screen does.
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

// ratingColour is the grade's colour, as the review buttons carry it.
func ratingColour(tk design.Tokens, rating schedule.Rating) ui.Color {
	switch rating {
	case schedule.Again:
		return tk.ReviewAgain
	case schedule.Hard:
		return tk.ReviewHard
	case schedule.Good:
		return tk.ReviewGood
	case schedule.Easy:
		return tk.ReviewEasy
	}
	return tk.TextMuted
}
