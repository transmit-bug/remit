package cardtypes_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/cardtypes"
	"github.com/transmit-bug/remit/locales"
)

var en = locales.In("en")

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// fake is the app the types page talks to: the types it lists and the two
// writes it was asked for. A refusal is written into the page's own state, as
// the app does, so the page's sentence is the one the store's error becomes.
type fake struct {
	types   []cardtypes.Type
	saves   []core.Type
	deletes []core.ID
	state   *cardtypes.State
}

// builtins is a library's three seeded types with a type of the user's own.
func builtins() []cardtypes.Type {
	var out []cardtypes.Type
	for i, typ := range core.BuiltinTypes() {
		out = append(out, cardtypes.Type{Type: typ, Cards: i, Builtin: true})
	}
	out = append(out, cardtypes.Type{
		Type: core.Type{
			ID: "type_vocab", Name: "Vocabulary",
			Fields: []string{"Term", "Meaning", "Extra"},
			Front:  "{{Term}}", Back: "{{Meaning}}", Version: 1,
		},
		Cards: 1,
	})
	return out
}

func (f *fake) deps() cardtypes.Deps {
	return cardtypes.Deps{
		Types: func() []cardtypes.Type { return f.types },
		SaveType: func(_ *ui.Context, typ core.Type) core.ID {
			if typ.ID == "" {
				typ.ID = core.NewID("type")
			}
			f.saves = append(f.saves, typ)
			return typ.ID
		},
		DeleteType: func(_ *ui.Context, id core.ID) {
			for _, typ := range core.BuiltinTypes() {
				if typ.ID == id {
					if f.state != nil {
						f.state.Err = en.TypeBuiltinDelete()
					}
					return
				}
			}
			f.deletes = append(f.deletes, id)
		},
	}
}

// shown draws the page over the fake, at a size and in a language.
func shown(t *testing.T, f *fake, w, h int, locale string) *ui.Tester {
	t.Helper()
	f.state = &cardtypes.State{}
	s := cardtypes.New()
	f.state = &s
	tt := ui.NewTester(func(c *ui.Context) { cardtypes.View(c, f.state, f.deps()) }, w, h)
	if locale != "" {
		tt.SetLocale(locale)
	}
	tt.Frame()
	return tt
}

// reveal scrolls s into the window, as a page taller than it needs.
func reveal(t *testing.T, tt *ui.Tester, w, h int, s string) {
	t.Helper()
	inside := func(r ui.Rect) bool {
		return r.W > 0 && r.H > 0 && r.X >= 0 && r.Y >= 0 &&
			r.X+r.W <= float32(w) && r.Y+r.H <= float32(h)
	}
	for i := 0; i < 20; i++ {
		if r, ok := tt.Find(s); ok && inside(r) {
			return
		}
		tt.Scroll(float32(w)/2, float32(h)/2, 0, 120)
	}
	r, ok := tt.Find(s)
	t.Fatalf("%q did not scroll into the window: box %+v (found %v); texts %q", s, r, ok, tt.Texts())
}

// The editor's preview is the host engine's own answer: retyping a template
// changes the preview on the next frame, and nothing is written until Save.
func TestTypesPagePreviewsTheTemplate(t *testing.T) {
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f := &fake{types: builtins()}
			tt := shown(t, f, size.w, size.h, "")
			if !tt.HasText(en.TypesTitle()) {
				t.Fatalf("the page has no title; texts %q", tt.Texts())
			}

			// The first type is loaded: Basic's front template renders "Front".
			reveal(t, tt, size.w, size.h, en.TypePreviewFront())
			if !tt.HasText("Front") {
				t.Fatalf("the preview did not render the draft; texts %q", tt.Texts())
			}

			reveal(t, tt, size.w, size.h, en.TypeFrontTemplate())
			if err := tt.Click(en.TypeFrontTemplate()); err != nil {
				t.Fatal(err)
			}
			tt.Command("selectAll")
			tt.Type("{{Back}}?")
			tt.Frame()
			reveal(t, tt, size.w, size.h, "Back?")
			if !tt.HasText("Back?") {
				t.Fatalf("the preview did not follow the edit; texts %q", tt.Texts())
			}
			if len(f.saves) != 0 {
				t.Fatalf("typing wrote the type: %+v", f.saves)
			}

			reveal(t, tt, size.w, size.h, en.TypeSave())
			if err := tt.Click(en.TypeSave()); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(f.saves) != 1 {
				t.Fatalf("Save wrote %d types, want 1", len(f.saves))
			}
			if saved := f.saves[0]; saved.ID != core.TypeBasic || saved.Front != "{{Back}}?" {
				t.Fatalf("saved %+v, want remit.basic with the new front template", saved)
			}
		})
	}
}

// A built-in type cannot be deleted: the page asks, the store refuses, and the
// page says why while the type stays in the list.
func TestTypesPageKeepsBuiltinsUndeletable(t *testing.T) {
	f := &fake{types: builtins()}
	tt := shown(t, f, 1024, 720, "")

	reveal(t, tt, 1024, 720, en.TypeDelete())
	if err := tt.Click(en.TypeDelete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(f.deletes) != 0 {
		t.Fatalf("a built-in type was deleted: %v", f.deletes)
	}
	reveal(t, tt, 1024, 720, en.TypeBuiltinDelete())
	if !tt.HasText(en.TypeBuiltinDelete()) {
		t.Fatalf("the page does not say why the type stays; texts %q", tt.Texts())
	}
	if !tt.HasText(core.BuiltinTypes()[0].Name) {
		t.Errorf("the built-in type left the list; texts %q", tt.Texts())
	}
}

// A new type starts from a draft the user shapes, and Save is what writes it:
// the store mints its ID and the list shows it.
func TestTypesPageCreatesAType(t *testing.T) {
	f := &fake{types: builtins()}
	tt := shown(t, f, 1024, 720, "")

	if err := tt.Click(en.TypeNew()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	reveal(t, tt, 1024, 720, en.TypeFieldName(1))
	if err := tt.Click(en.TypeFieldName(1)); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("Prompt")
	tt.Frame()

	reveal(t, tt, 1024, 720, en.TypeSave())
	if err := tt.Click(en.TypeSave()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(f.saves) != 1 {
		t.Fatalf("Save wrote %d types, want the new one", len(f.saves))
	}
	saved := f.saves[0]
	if saved.ID.Kind() != "type" || !saved.ID.Valid() {
		t.Errorf("the new type's ID is %q, want one minted", saved.ID)
	}
	if saved.Name != en.TypeNew() || len(saved.Fields) != 2 ||
		saved.Fields[0] != "Prompt" || saved.Fields[1] != "Back" {
		t.Errorf("saved %+v, want the edited draft", saved)
	}
}

// The page holds the smallest window in Chinese: the list, the editor and the
// preview all keep their controls inside it.
func TestTypesPageInChineseAtMinimum(t *testing.T) {
	zh := locales.In("zh-Hans")
	f := &fake{types: builtins()}
	tt := shown(t, f, 720, 520, "zh-Hans")

	for _, want := range []string{zh.TypesTitle(), zh.TypesBuiltin()} {
		reveal(t, tt, 720, 520, want)
		if !tt.HasText(want) {
			t.Fatalf("the Chinese page is missing %q; texts %q", want, tt.Texts())
		}
	}
	reveal(t, tt, 720, 520, zh.TypeFrontTemplate())
	if err := tt.Click(zh.TypeFrontTemplate()); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("{{Back}}?")
	tt.Frame()
	reveal(t, tt, 720, 520, "Back?")
	if !tt.HasText("Back?") {
		t.Fatalf("the Chinese page's preview did not follow the edit; texts %q", tt.Texts())
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
