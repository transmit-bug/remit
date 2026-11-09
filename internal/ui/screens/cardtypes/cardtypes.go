// Package cardtypes is the library's card types: the list of types on the left
// and the chosen one's fields, templates and live preview on the right. The page
// shows what the app's snapshot holds and reports every write through Deps; the
// store is the authority on what a type may be, so a refusal is shown where the
// action was taken rather than guessed at here.
package cardtypes

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/template"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// templateHeight is one template's room: tall enough for the two sides of a
// built-in type, short enough that both fit above the preview.
const templateHeight float32 = 96

// Type is a card type as the page lists it: the type itself, how many cards
// reference it, and whether the library seeds it on every open.
type Type struct {
	core.Type
	Cards   int
	Builtin bool
}

// State is the types page: the row the user chose, the draft the editor holds,
// and what the last write answered. Nothing typed here is written until Save.
type State struct {
	List ui.ListState
	Row  int
	// Draft is the type being edited: the chosen type as the editor holds it,
	// or a new one with no ID yet.
	Draft core.Type
	// Err is why the last write was refused, "" when it was taken.
	Err string
	// fresh says the draft is a new type the user started, which no row names.
	fresh bool

	// split is the list's width until its divider moves.
	split float32
	// PreviewFront and PreviewBack keep the parsed previews between frames, so
	// the markdown view reparses only what changed. Pointers because the
	// options carry a cache the frame must not copy.
	PreviewFront, PreviewBack *markdown.Options
}

// New returns the types page on no chosen type.
func New() State { return State{Row: -1, split: design.NotesColumnWidth} }

// Deps is what the page asks of the app: the types it lists, and the two writes
// the store owns. Each runs on the UI thread.
type Deps struct {
	// Types is this library's card types, with how many cards reference each.
	Types func() []Type
	// SaveType writes a type, minting the ID of a new one and rewriting the
	// references a field rename touches. It answers the type's ID, "" when the
	// app refused the write and said why in the page's State.
	SaveType func(*ui.Context, core.Type) core.ID
	// DeleteType removes a type no card references, and says why in the page's
	// State when one does.
	DeleteType func(*ui.Context, core.ID)
}

// View draws the page: the type list beside the chosen type's editor, its two
// templates and the preview the host engine renders them with.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	types := d.Types()
	if s.Row >= len(types) {
		s.Row = len(types) - 1
	}
	if s.split <= 0 {
		s.split = tk.NotesColumnWidth
	}
	if s.Row < 0 && s.Draft.ID == "" && !s.fresh && len(types) > 0 {
		// The page opens on the first type: an empty editor beside a full list
		// reads as a type that failed to load.
		s.Row = 0
	}

	ui.Column(c).Grow(1).FillWidth().Padding(tk.SpaceXL).Gap(tk.SpaceL).Children(func() {
		pageHead(c, tk, msgs.TypesTitle(), msgs.TypesBody())
		if len(types) == 0 {
			emptyTypes(c, tk, msgs)
			return
		}
		ui.Split(c, &s.split, func() {
			typeList(c, s, tk, msgs, types)
		}, func() {
			detail(c, s, d, tk, msgs, types)
		}).Grow(1).Label(msgs.TypesTitle())
	})
}

// pageHead is the page's title and the one line that says what a type is.
func pageHead(c *ui.Context, tk design.Tokens, title, body string) {
	ui.Column(c).Gap(tk.SpaceXS).Children(func() {
		ui.Text(c, title).FontSize(tk.TypeH1).Bold()
		if body != "" {
			ui.Text(c, body).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
		}
	})
}

// typeList is the master list: every type by name, with the cards that
// reference it and whether the library seeds it.
func typeList(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, types []Type) {
	s.List.Selected = &s.Row
	s.List.Key = func(row int) any { return types[row].ID }
	s.List.Label = func(row int) string { return types[row].Name }
	ui.List(c, &s.List, len(types), func(i int) {
		typ := types[i]
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
			Padding(tk.SpaceXS, tk.SpaceS).Children(func() {
			ui.Text(c, typ.Name).FontSize(tk.TypeUI).SingleLine().Grow(1)
			if typ.Builtin {
				ui.Badge(c, msgs.TypesBuiltin())
			}
			ui.Text(c, strconv.Itoa(typ.Cards)).FontSize(tk.TypeLabel).
				TextColor(tk.TextMuted)
		})
	}).Label(msgs.TypesList())
	// The chosen row is what the editor shows: a type the user just picked
	// replaces a draft of another type, but a new draft with no ID stays.
	if s.Row >= 0 && s.Row < len(types) && s.Draft.ID != types[s.Row].ID {
		s.Draft = types[s.Row].Type
		s.Err, s.fresh = "", false
	}
}

// detail is the chosen type's editor: its name and fields, its two templates,
// the preview, and the two writes.
func detail(c *ui.Context, s *State, d Deps, tk design.Tokens, msgs locales.Messages, types []Type) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(tk.SpaceL).Gap(tk.SpaceM).Children(func() {
			header(c, s, tk, msgs, types)
			if s.Draft.Name == "" {
				ui.Text(c, msgs.TypeChoose()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
				return
			}
			ui.Form(c, func() {
				editor(c, s, tk, msgs)
			})
			actions(c, s, d, tk, msgs)
			preview(c, s, tk, msgs)
			ui.Text(c, msgs.TypesFallback()).FontSize(tk.TypeLabel).
				TextColor(tk.TextFaint)
		})
	})
}

// header is the detail's title row: the type's name and what kind it is, with
// the one action that makes a new type.
func header(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, types []Type) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		name := s.Draft.Name
		if name == "" {
			name = msgs.TypesTitle()
		}
		ui.Text(c, name).FontSize(tk.TypeH2).Bold().SingleLine().Grow(1)
		if builtinType(s, types) {
			ui.Badge(c, msgs.TypesBuiltin())
		}
		ui.Button(c, msgs.TypeNew()).MinHeight(tk.Control).OnClick(func() { s.start(msgs) })
	})
}

// builtinType reports whether the chosen row is one the library seeds.
func builtinType(s *State, types []Type) bool {
	return s.Row >= 0 && s.Row < len(types) && types[s.Row].Builtin
}

// editor is the draft's name, its fields and its two templates.
func editor(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages) {
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
		ui.TextArea(c, &s.Draft.Front).Height(templateHeight).FontSize(tk.TypeMono).
			Label(msgs.TypeFrontTemplate())
	})
	ui.Field(c, msgs.TypeBackTemplate(), func() {
		ui.TextArea(c, &s.Draft.Back).Height(templateHeight).FontSize(tk.TypeMono).
			Label(msgs.TypeBackTemplate())
	})
}

// actions is Save and Delete, with the reason the last write was refused where
// the write was taken.
func actions(c *ui.Context, s *State, d Deps, tk design.Tokens, msgs locales.Messages) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
		ui.PrimaryButton(c, msgs.TypeSave()).MinHeight(tk.Control).OnClick(func() {
			if id := d.SaveType(c, s.Draft); id != "" {
				s.afterSave(d.Types(), id)
			}
		})
		ui.Button(c, msgs.TypeDelete()).MinHeight(tk.Control).OnClick(func() {
			if s.Draft.ID == "" {
				return
			}
			id := s.Draft.ID
			d.DeleteType(c, id)
			remaining := d.Types()
			if !holdsType(remaining, id) {
				s.afterDelete(remaining)
			}
		})
		if s.Err != "" {
			ui.Text(c, s.Err).FontSize(tk.TypeUI).TextColor(tk.Danger).Grow(1)
		}
	})
}

// preview renders both faces of the draft through the host template engine:
// the same Compile and Render the review path calls, so what the page shows is
// what the next card will. Every field stands for its own name as the sample
// value, so the preview says what the template does rather than inventing
// content.
func preview(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages) {
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, msgs.TypePreview()).FontSize(tk.TypeUI).Bold()
		previewFace(c, s, tk, msgs, msgs.TypePreviewFront(), s.Draft.Front, template.SideFront, &s.PreviewFront)
		previewFace(c, s, tk, msgs, msgs.TypePreviewBack(), s.Draft.Back, template.SideBack, &s.PreviewBack)
	})
}

// previewFace is one side of the preview: the face's label and the rendered
// markdown, or the dialect's refusal with the reason it points at.
func previewFace(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, label, src string, side template.Side, opts **markdown.Options) {
	ui.Text(c, label).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	text, err := renderPreview(src, s.Draft.Fields, side)
	if err != nil {
		ui.Text(c, msgs.TypeTemplateBroken(err.Error())).FontSize(tk.TypeUI).TextColor(tk.Danger)
		return
	}
	if strings.TrimSpace(text) == "" {
		ui.Text(c, "—").FontSize(tk.TypeUI).TextColor(tk.TextMuted)
		return
	}
	if *opts == nil {
		*opts = &markdown.Options{}
	}
	markdown.View(c, text, *opts)
}

// renderPreview renders one template with the field list as its sample values.
func renderPreview(src string, fields []string, side template.Side) (string, error) {
	tpl, err := template.Compile(src)
	if err != nil {
		return "", err
	}
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			values[field] = field
		}
	}
	out, err := tpl.Render(template.Input{Fields: values, Side: side})
	if err != nil {
		return "", err
	}
	return out.Markdown, nil
}

// emptyTypes is a library the store seeded nothing into: the list says so and
// the one action that starts a type is on the page.
func emptyTypes(c *ui.Context, tk design.Tokens, msgs locales.Messages) {
	ui.Column(c).Grow(1).Center().Gap(tk.SpaceM).Children(func() {
		ui.Text(c, msgs.TypeEmpty()).FontSize(tk.TypeH2).TextColor(tk.TextMuted)
	})
}

// start puts a new type in the editor: the two fields a `::` paragraph asks
// for, as a starting point rather than a schema the user cannot change.
func (s *State) start(msgs locales.Messages) {
	s.Row = -1
	s.List.Selected = nil
	s.fresh = true
	s.Draft = core.Type{
		Name:   msgs.TypeNew(),
		Fields: []string{"Front", "Back"},
		Front:  "{{Front}}",
		Back:   "{{Back}}",
	}
	s.Err = ""
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
	s.Err, s.fresh = "", false
	for i, typ := range types {
		if typ.ID == id {
			s.Row = i
			s.Draft = typ.Type
			return
		}
	}
}

// afterDelete chooses what is left after a type went: the first type, or
// nothing when the library has none.
func (s *State) afterDelete(types []Type) {
	s.Err, s.fresh = "", false
	s.Draft = core.Type{}
	s.Row = -1
	if len(types) > 0 {
		s.Row = 0
		return
	}
	s.List.Selected = nil
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
