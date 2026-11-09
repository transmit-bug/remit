package ui

import (
	"errors"
	"log/slog"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ui/screens/cardtypes"
)

// The library's card types, which the recall area's types page shows: the list
// is a snapshot retaken with the library, and the two writes go through the
// store, which owns what a type may be.

// typesView draws the card types page; the screen lives in its own package and
// this hands it the snapshot and the writes.
func (a *App) typesView(c *ui.Context) {
	cardtypes.View(c, &a.types, cardtypes.Deps{
		Types:      func() []cardtypes.Type { return a.typeList },
		SaveType:   a.saveType,
		DeleteType: a.deleteType,
	})
}

// loadTypes retakes the type list with the cards that reference each: the page
// lists the counts and a delete is refused on them.
func (a *App) loadTypes() {
	a.typeList = nil
	if a.lib == nil {
		return
	}
	used := make(map[core.ID]int, 8)
	for _, card := range a.data.cards {
		used[card.Type]++
	}
	builtin := make(map[core.ID]bool, len(core.BuiltinTypes()))
	for _, typ := range core.BuiltinTypes() {
		builtin[typ.ID] = true
	}
	for _, typ := range a.lib.Types() {
		a.typeList = append(a.typeList, cardtypes.Type{
			Type:    typ,
			Cards:   used[typ.ID],
			Builtin: builtin[typ.ID],
		})
	}
}

// saveType writes a type through the store, which mints a new ID and carries a
// field rename through every card. The library is reloaded so the cards on
// screen render through the type as it now is.
func (a *App) saveType(c *ui.Context, typ core.Type) core.ID {
	a.types.Err = ""
	if a.lib == nil {
		a.types.Err = m(c).TypesNeedsLibrary()
		return ""
	}
	id, err := a.lib.SaveType(typ)
	if err != nil {
		slog.Warn("remit: save type", "type", typ.ID, "err", err)
		a.types.Err = a.typeWriteError(c, err)
		a.loadTypes()
		return ""
	}
	a.reload()
	c.Toast(m(c).TypeSaved())
	return id
}

// deleteType removes a type no card references, and says why when one does.
func (a *App) deleteType(c *ui.Context, id core.ID) {
	a.types.Err = ""
	if a.lib == nil {
		a.types.Err = m(c).TypesNeedsLibrary()
		return
	}
	if err := a.lib.DeleteType(id); err != nil {
		slog.Warn("remit: delete type", "type", id, "err", err)
		a.types.Err = a.typeWriteError(c, err)
		a.loadTypes()
		return
	}
	a.reload()
	c.Toast(m(c).TypeDeleted())
}

// typeWriteError is the page's sentence for a refusal: which field holds the
// values, how many cards keep the type, or what the store would not take.
func (a *App) typeWriteError(c *ui.Context, err error) string {
	msgs := m(c)
	var held *store.FieldInUseError
	if errors.As(err, &held) {
		return msgs.TypeFieldHeld(held.Field, held.Cards)
	}
	var used *store.TypeInUseError
	if errors.As(err, &used) {
		return msgs.TypeInUse(used.Cards)
	}
	switch {
	case errors.Is(err, store.ErrBuiltinType):
		return msgs.TypeBuiltinDelete()
	case errors.Is(err, store.ErrNoType):
		return msgs.TypeUnknown()
	}
	return msgs.TypeBad(err.Error())
}
