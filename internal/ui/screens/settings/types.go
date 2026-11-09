package settings

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// Type is a card type as the page lists it: the type itself, and how many
// cards reference it, which is what a delete is refused on.
type Type struct {
	core.Type
	Cards int
}

const (
	// typeTableHeight bounds the type list. The page scrolls, and a table
	// nested in a scroll region needs a height of its own: this is a header
	// and five rows, enough to see the built-ins and a couple of one's own.
	typeTableHeight float32 = 200
	// templateHeight is one template's room: tall enough for the two sides of
	// a built-in type, short enough that both fit above the preview.
	templateHeight float32 = 96
)

// typesFieldset is the library's card types: the list, the chosen type's fields
// and templates, and a preview rendered by the same engine the review uses, so
// a template edit shows what it does before the next card does.
func typesFieldset(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	types := d.typeList()
	ui.Fieldset(c, msgs.SettingsTypes(), func() {
		if !s.HasLibrary {
			muted(c, tk, msgs.SettingsTypesNeedsLibrary())
			return
		}
		muted(c, tk, msgs.SettingsTypesBody())
		if len(types) == 0 {
			muted(c, tk, msgs.TypeEmpty())
			return
		}
		typeTable(c, s, msgs, tk, types)
		if s.TypeRow >= 0 && s.TypeRow < len(types) && s.Draft.ID != types[s.TypeRow].ID {
			// The chosen row is not what the editor shows: it is the type the
			// user just picked, so the draft follows it.
			s.Draft = types[s.TypeRow].Type
			s.TypeErr = ""
		}
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			ui.Button(c, msgs.TypeNew()).MinHeight(tk.Control).OnClick(func() { s.startType(msgs) })
			ui.Button(c, msgs.TypeDelete()).MinHeight(tk.Control).OnClick(func() {
				if s.Draft.ID == "" {
					return
				}
				id := s.Draft.ID
				d.removeType(c, id)
				remaining := d.typeList()
				if !holdsType(remaining, id) {
					s.afterDelete(remaining)
				}
			})
		})
		if s.Draft.Name == "" {
			muted(c, tk, msgs.TypeChoose())
			return
		}
		typeEditor(c, s, msgs, tk)
		ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
			ui.Button(c, msgs.TypeSave()).MinHeight(tk.Control).OnClick(func() {
				if id := d.save(c, s.Draft); id != "" {
					s.afterSave(d.typeList(), id)
				}
			})
			if s.TypeErr != "" {
				ui.Text(c, s.TypeErr).FontSize(tk.TypeUI).TextColor(tk.Danger).Grow(1)
			}
		})
		typePreview(c, s, msgs, tk)
	})
}

// typeTable is the type list: the name, the field count and the cards that
// reference it. The row chosen is the type the editor shows.
func typeTable(c *ui.Context, s *State, msgs locales.Messages, tk design.Tokens, types []Type) {
	if s.TypeRow >= len(types) {
		s.TypeRow = len(types) - 1
	}
	s.TypeList.Selected = &s.TypeRow
	columns := []ui.TableColumn{
		{Title: msgs.TypeColumnName()},
		{Title: msgs.TypeColumnFields(), Width: 72, Align: ui.End},
		{Title: msgs.TypeColumnCards(), Width: 72, Align: ui.End},
	}
	ui.Table(c, &s.TypeList, columns, len(types), func(row, col int) {
		typ := types[row]
		switch col {
		case 0:
			ui.Text(c, typ.Name).SingleLine()
		case 1:
			ui.Text(c, strconv.Itoa(len(typ.Fields))).TextColor(tk.TextMuted)
		case 2:
			ui.Text(c, strconv.Itoa(typ.Cards)).TextColor(tk.TextMuted)
		}
	}).Height(typeTableHeight).Label(msgs.TypeColumnName())
}

// typeEditor is the chosen type's name, its fields and its two templates.
// A field renamed here is rewritten across the library by the app's write, and
// a field removed is refused while a card holds a value in it; the page only
// says what the user typed.
func typeEditor(c *ui.Context, s *State, msgs locales.Messages, tk design.Tokens) {
	ui.Field(c, msgs.TypeColumnName(), func() {
		ui.TextInput(c, &s.Draft.Name).MinHeight(tk.Control).Label(msgs.TypeColumnName())
	})
	ui.Field(c, msgs.TypeColumnFields(), func() {
		ui.Column(c).Gap(tk.SpaceS).Children(func() {
			for i := range s.Draft.Fields {
				i := i
				ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &s.Draft.Fields[i]).MinHeight(tk.Control).Grow(1).
						Label(msgs.TypeFieldName(i + 1))
					ui.Button(c, msgs.TypeRemoveField()).MinHeight(tk.Control).
						OnClick(func() { s.removeField(i) })
				})
			}
			ui.Button(c, msgs.TypeAddField()).MinHeight(tk.Control).AlignSelf(ui.Start).
				OnClick(func() { s.Draft.Fields = append(s.Draft.Fields, "") })
		})
	})
	ui.Field(c, msgs.TypeFrontTemplate(), func() {
		ui.TextArea(c, &s.Draft.Front).Height(templateHeight).Label(msgs.TypeFrontTemplate())
	})
	ui.Field(c, msgs.TypeBackTemplate(), func() {
		ui.TextArea(c, &s.Draft.Back).Height(templateHeight).Label(msgs.TypeBackTemplate())
	})
}

// startType puts a new type in the editor: the two fields a `::` paragraph
// asks for, as a starting point rather than a schema the user cannot change.
func (s *State) startType(msgs locales.Messages) {
	s.TypeRow = -1
	s.TypeList.Selected = nil
	s.Draft = core.Type{
		Name:   msgs.TypeNew(),
		Fields: []string{"Front", "Back"},
		Front:  "{{Front}}",
		Back:   "{{Back}}",
	}
	s.TypeErr = ""
}

// removeField drops one field from the draft. The store refuses the save when
// a card still holds a value under the name, and says so on the page.
func (s *State) removeField(i int) {
	if i < 0 || i >= len(s.Draft.Fields) {
		return
	}
	fields := make([]string, 0, len(s.Draft.Fields)-1)
	fields = append(fields, s.Draft.Fields[:i]...)
	s.Draft.Fields = append(fields, s.Draft.Fields[i+1:]...)
}

// afterSave shows the type just written: its row, its stored shape and the
// counts the app re-read after the write.
func (s *State) afterSave(types []Type, id core.ID) {
	s.TypeErr = ""
	for i, typ := range types {
		if typ.ID == id {
			s.TypeRow = i
			s.Draft = typ.Type
			return
		}
	}
}

// afterDelete chooses what is left after a type went: the first type, or
// nothing when the library has none.
func (s *State) afterDelete(types []Type) {
	s.TypeErr = ""
	s.Draft = core.Type{}
	s.TypeRow = -1
	if len(types) > 0 {
		s.TypeRow = 0
		return
	}
	s.TypeList.Selected = nil
}

// The type page's reads and writes, in the same shape as every other page's: a
// nil function is a build without the service, and the page says so instead of
// panicking in a frame.

func (d Deps) typeList() []Type {
	if d.Types == nil {
		return nil
	}
	return d.Types()
}

func (d Deps) save(c *ui.Context, typ core.Type) core.ID {
	if d.SaveType == nil {
		return ""
	}
	return d.SaveType(c, typ)
}

func (d Deps) removeType(c *ui.Context, id core.ID) {
	if d.DeleteType != nil {
		d.DeleteType(c, id)
	}
}

// holdsType reports whether a type is among those the page lists.
func holdsType(types []Type, id core.ID) bool {
	for _, typ := range types {
		if typ.ID == id {
			return true
		}
	}
	return false
}
