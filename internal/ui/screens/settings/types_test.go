package settings_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/settings"
	"github.com/transmit-bug/remit/locales"
)

// The type editor shows the chosen type, and its preview is the host engine's
// own answer: retyping a template changes the preview on the next frame, with
// nothing written until Save.
func TestTypeEditingAndPreview(t *testing.T) {
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			w := newWorld()
			w.state = &settings.State{HasLibrary: true}
			tt := atWorld(t, w, size.w, size.h)

			// The first type is loaded: Basic's front template renders "Front!".
			reveal(t, tt, size.w, size.h, en.TypePreviewFront())
			if !tt.HasText("Front!") {
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
			if len(w.saves) != 0 {
				t.Fatalf("typing wrote the type: %+v", w.saves)
			}

			reveal(t, tt, size.w, size.h, en.TypeSave())
			if err := tt.Click(en.TypeSave()); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(w.saves) != 1 {
				t.Fatalf("Save wrote %d types, want 1", len(w.saves))
			}
			saved := w.saves[0]
			if saved.ID != core.TypeBasic || saved.Front != "{{Back}}?" {
				t.Fatalf("saved %+v, want remit.basic with the new front template", saved)
			}
		})
	}
}

// A new type starts from a draft the user shapes, and Save is what writes it:
// the store mints its ID and the list shows it.
func TestTypeCreation(t *testing.T) {
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			w := newWorld()
			w.state = &settings.State{HasLibrary: true}
			tt := atWorld(t, w, size.w, size.h)

			reveal(t, tt, size.w, size.h, en.TypeNew())
			if err := tt.Click(en.TypeNew()); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText(en.TypeNew()) {
				t.Fatalf("the new draft is not on the page; texts %q", tt.Texts())
			}

			reveal(t, tt, size.w, size.h, en.TypeFieldName(1))
			if err := tt.Click(en.TypeFieldName(1)); err != nil {
				t.Fatal(err)
			}
			tt.Command("selectAll")
			tt.Type("Prompt")
			tt.Frame()

			reveal(t, tt, size.w, size.h, en.TypeSave())
			if err := tt.Click(en.TypeSave()); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(w.saves) != 1 {
				t.Fatalf("Save wrote %d types, want the new one", len(w.saves))
			}
			saved := w.saves[0]
			if saved.ID.Kind() != "type" || !saved.ID.Valid() {
				t.Errorf("the new type's ID is %q, want one minted", saved.ID)
			}
			if saved.Name != en.TypeNew() || len(saved.Fields) != 2 ||
				saved.Fields[0] != "Prompt" || saved.Fields[1] != "Back" {
				t.Errorf("saved %+v, want the edited draft", saved)
			}
			if len(w.types) != 3 {
				t.Errorf("the list holds %d types, want the new one added", len(w.types))
			}
		})
	}
}

// Removing a field a card holds a value in is refused with an explanation, and
// nothing is written; a type cards reference cannot be deleted either.
func TestTypeWriteRefusals(t *testing.T) {
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			w := newWorld()
			w.state = &settings.State{HasLibrary: true, TypeRow: 1} // Vocabulary
			tt := atWorld(t, w, size.w, size.h)

			// Field 3 is Extra: focus it, Tab to its Remove button, press Enter.
			reveal(t, tt, size.w, size.h, en.TypeFieldName(3))
			if err := tt.Click(en.TypeFieldName(3)); err != nil {
				t.Fatal(err)
			}
			tt.Key(0, ui.KeyTab)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()

			reveal(t, tt, size.w, size.h, en.TypeSave())
			if err := tt.Click(en.TypeSave()); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(w.saves) != 0 {
				t.Fatalf("the refused save wrote %+v", w.saves)
			}
			held := en.TypeFieldHeld("Extra", 1)
			reveal(t, tt, size.w, size.h, held)
			if !tt.HasText(held) {
				t.Fatalf("the page does not explain the refusal %q; texts %q", held, tt.Texts())
			}

			reveal(t, tt, size.w, size.h, en.TypeDelete())
			if err := tt.Click(en.TypeDelete()); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(w.deletes) != 0 {
				t.Fatalf("a referenced type was deleted: %v", w.deletes)
			}
			inUse := en.TypeInUse(1)
			reveal(t, tt, size.w, size.h, inUse)
			if !tt.HasText(inUse) {
				t.Fatalf("the page does not explain why the type stays %q; texts %q", inUse, tt.Texts())
			}
		})
	}
}

// In Chinese at the smallest window the sections still work: approval, a role
// change and a template edit that the preview follows at once.
func TestPluginsAndTypesInChinese(t *testing.T) {
	zh := locales.In("zh-Hans")
	w := newWorld()
	w.plugins[3].Enabled = true // acme.sched is approved
	w.state = &settings.State{HasLibrary: true}
	tt := ui.NewTester(func(c *ui.Context) { settings.View(c, w.state, w.deps()) }, 720, 520)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	reveal(t, tt, 720, 520, zh.SettingsPlugins())
	if !tt.HasText(zh.SettingsPlugins()) || !tt.HasText(zh.SettingsTypes()) {
		t.Fatalf("the Chinese page is missing its sections; texts %q", tt.Texts())
	}

	reveal(t, tt, 720, 520, zh.PluginEnable("acme.cards"))
	if err := tt.Click(zh.PluginEnable("acme.cards")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(w.enabled) != 1 || w.enabled[0] != (pluginChange{"acme.cards", true}) {
		t.Fatalf("the approval reached the app as %v", w.enabled)
	}

	reveal(t, tt, 720, 520, "scheduler 1.0.0")
	if err := tt.Click("scheduler 1.0.0"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if len(w.roleWrites) != 1 || w.roleWrites[0] != (roleChange{core.RoleReviewSchedule, "acme.sched"}) {
		t.Fatalf("the role change reached the app as %v", w.roleWrites)
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
	if tt.HasText(en.SettingsTitle()) {
		t.Error("the page kept its English title")
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
