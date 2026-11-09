package store_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// writeConfig puts a remit.json in dir as a test writes it by hand.
func writeConfig(t *testing.T, dir string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "remit.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// samePresets compares two presets as the file holds them, so a retention
// pointer is compared by the number it names.
func samePresets(t *testing.T, got, want store.Presets) {
	t.Helper()
	a, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("presets = %s, want %s", a, b)
	}
}

// customPresets is a rhythm that differs from the defaults in every section, so
// a value that does not survive a write is visible.
func customPresets() store.Presets {
	retention := 0.85
	return store.Presets{
		Budget:  store.BudgetPreset{Cards: 7, Minutes: 45, ExcerptSlots: 1},
		Review:  store.ReviewPreset{NewPerDay: 5, MaxIntervalDays: 90, Retention: &retention},
		Reading: store.ReadingPreset{NewPerDay: 2},
		Hard:    store.HardPreset{Lapses: 4},
		Broken:  store.BrokenPreset{Policy: store.BrokenSnapshot},
	}
}

// The defaults are the spec's, which is what a new library starts with.
func TestDefaultPresets(t *testing.T) {
	want := store.Presets{
		Budget:  store.BudgetPreset{Cards: 20, Minutes: 30, ExcerptSlots: 2},
		Review:  store.ReviewPreset{NewPerDay: 10, MaxIntervalDays: 180, Preview: true},
		Reading: store.ReadingPreset{NewPerDay: 3, ReturnAfterExcerpt: true},
		Hard:    store.HardPreset{Lapses: 3},
		Broken:  store.BrokenPreset{Policy: store.BrokenKeep},
	}
	samePresets(t, store.DefaultPresets(), want)
}

// A new library writes the defaults into remit.json, and reads them back: the
// rhythm travels with the library rather than living in the app's settings.
func TestNewLibraryWritesTheDefaultPresets(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	samePresets(t, lib.Presets(), store.DefaultPresets())

	config := remitJSON(t, dir)
	if config["format"] != "remit/3" {
		t.Errorf("format = %v, want remit/3", config["format"])
	}
	presets, ok := config["presets"].(map[string]any)
	if !ok {
		t.Fatalf("remit.json has no presets object: %v", config["presets"])
	}
	budget, _ := presets["budget"].(map[string]any)
	if budget["minutes"] != 30.0 || budget["excerptSlots"] != 2.0 {
		t.Errorf("budget = %v, want 20/30/2", budget)
	}
	review, _ := presets["review"].(map[string]any)
	if retention, present := review["retention"]; !present || retention != nil {
		t.Errorf("review.retention = %v (present %v), want null so the plugin's policy holds", retention, present)
	}

	reopened := reopen(t, lib, dir, clk)
	samePresets(t, reopened.Presets(), store.DefaultPresets())
}

// Every section round-trips through the file, a retention override included.
func TestPresetsRoundTrip(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	if err := lib.SetPresets(customPresets()); err != nil {
		t.Fatalf("SetPresets: %v", err)
	}
	samePresets(t, lib.Presets(), customPresets())

	presets, _ := remitJSON(t, dir)["presets"].(map[string]any)
	hard, _ := presets["hard"].(map[string]any)
	if hard["lapses"] != 4.0 {
		t.Errorf("hard.lapses = %v, want 4", hard["lapses"])
	}
	broken, _ := presets["broken"].(map[string]any)
	if broken["policy"] != store.BrokenSnapshot {
		t.Errorf("broken.policy = %v, want %q", broken["policy"], store.BrokenSnapshot)
	}

	reopened := reopen(t, lib, dir, clk)
	samePresets(t, reopened.Presets(), customPresets())
}

// Zero is a cap of none, not a value to be filled in: a library that turns a
// limit off keeps it off across a reopen.
func TestZeroLimitIsNoCap(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	open := store.DefaultPresets()
	open.Budget = store.BudgetPreset{}
	open.Review.NewPerDay, open.Review.MaxIntervalDays = 0, 0
	open.Reading.NewPerDay = 0
	open.Hard.Lapses = 0
	if err := lib.SetPresets(open); err != nil {
		t.Fatalf("SetPresets: %v", err)
	}
	reopened := reopen(t, lib, dir, clk)
	samePresets(t, reopened.Presets(), open)
}

// A value a preset cannot mean is refused, and the file the library holds stays
// exactly as it was.
func TestSetPresetsRefusesWhatALimitCannotMean(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	if err := lib.SetPresets(customPresets()); err != nil {
		t.Fatalf("SetPresets: %v", err)
	}
	before := read(t, filepath.Join(dir, "remit.json"))

	above := 1.5
	cases := []struct {
		name   string
		change func(*store.Presets)
	}{
		{"negative card cap", func(p *store.Presets) { p.Budget.Cards = -1 }},
		{"negative minutes", func(p *store.Presets) { p.Budget.Minutes = -30 }},
		{"negative excerpt slots", func(p *store.Presets) { p.Budget.ExcerptSlots = -2 }},
		{"negative new cards", func(p *store.Presets) { p.Review.NewPerDay = -5 }},
		{"negative max interval", func(p *store.Presets) { p.Review.MaxIntervalDays = -90 }},
		{"negative new reading", func(p *store.Presets) { p.Reading.NewPerDay = -2 }},
		{"negative hard threshold", func(p *store.Presets) { p.Hard.Lapses = -3 }},
		{"retention above one", func(p *store.Presets) { p.Review.Retention = &above }},
		{"zero retention", func(p *store.Presets) { p.Review.Retention = new(float64) }},
		{"unknown broken-link policy", func(p *store.Presets) { p.Broken.Policy = "guess" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			presets := customPresets()
			tc.change(&presets)
			if err := lib.SetPresets(presets); !errors.Is(err, store.ErrInvalidPresets) {
				t.Fatalf("SetPresets = %v, want ErrInvalidPresets", err)
			}
			samePresets(t, lib.Presets(), customPresets())
			if after := read(t, filepath.Join(dir, "remit.json")); !bytes.Equal(after, before) {
				t.Error("a refused preset write changed remit.json")
			}
		})
	}
}

// A library is not created from presets it could never write: the refused
// rhythm leaves no remit.json behind.
func TestCreateRefusesInvalidPresets(t *testing.T) {
	dir := t.TempDir()
	presets := store.DefaultPresets()
	presets.Budget.Cards = -1
	if _, err := store.Create(dir, store.Options{Presets: presets}); !errors.Is(err, store.ErrInvalidPresets) {
		t.Fatalf("Create = %v, want ErrInvalidPresets", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "remit.json")); !os.IsNotExist(err) {
		t.Errorf("a refused create wrote remit.json: %v", err)
	}
}

// A remit/2 library opens as remit/3: its fsrs, extensions and roles come
// across, its presets are the defaults, and its hand-added keys are still
// there. The original is kept beside it for a rollback.
func TestLegacyConfigMigratesToPresets(t *testing.T) {
	dir := t.TempDir()
	original := []byte(`{
  "format": "remit/2",
  "fsrs": {"desired_retention": 0.8, "scheduler": "fsrs-6"},
  "extensions": ["acme.cards"],
  "roles": {"cards.derive": "acme.cards"},
  "mine": {"keep": [1, 2, 3]}
}
`)
	writeConfig(t, dir, original)

	// The app resolves roles before it opens the library: a legacy file must
	// answer them too, or the first open would play the built-ins.
	if got := store.ReadRoles(dir)[core.RoleCardsDerive]; got != "acme.cards" {
		t.Errorf("ReadRoles before the migration = %q, want acme.cards", got)
	}

	lib, err := store.Open(dir, store.Options{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	if params := lib.ScheduleParams(); params.DesiredRetention != 0.8 || params.Scheduler != "fsrs-6" {
		t.Errorf("schedule = %+v, want retention 0.8 and fsrs-6", params)
	}
	samePresets(t, lib.Presets(), store.DefaultPresets())
	if got := lib.EnabledExtensions(); len(got) != 1 || got[0] != "acme.cards" {
		t.Errorf("extensions = %v, want acme.cards", got)
	}
	if got := lib.Roles()[core.RoleCardsDerive]; got != "acme.cards" {
		t.Errorf("cards.derive = %q, want acme.cards", got)
	}

	config := remitJSON(t, dir)
	if config["format"] != "remit/3" {
		t.Errorf("format = %v, want the migrated remit/3", config["format"])
	}
	if _, ok := config["presets"].(map[string]any); !ok {
		t.Error("the migrated file declares no presets")
	}
	mine, err := json.Marshal(config["mine"])
	if err != nil {
		t.Fatal(err)
	}
	if string(mine) != `{"keep":[1,2,3]}` {
		t.Errorf("the hand-added key reads %s, want it kept verbatim", mine)
	}
	fsrs, _ := config["fsrs"].(map[string]any)
	if fsrs["desired_retention"] != 0.8 {
		t.Errorf("desired retention = %v, want the file's 0.8", fsrs["desired_retention"])
	}
	if bak := read(t, filepath.Join(dir, "remit.json.bak")); !bytes.Equal(bak, original) {
		t.Errorf("remit.json.bak does not hold the original file:\n%s", bak)
	}

	// The migration happens once: a reopen of the migrated file leaves the
	// backup alone.
	if err := os.WriteFile(filepath.Join(dir, "remit.json.bak"), []byte("older"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	again, err := store.Open(dir, store.Options{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { again.Close() })
	samePresets(t, again.Presets(), store.DefaultPresets())
	if bak := read(t, filepath.Join(dir, "remit.json.bak")); string(bak) != "older" {
		t.Errorf("a reopen migrated again: remit.json.bak is %q", bak)
	}
}

// A remit/3 file that declares no presets reads as the defaults and is left as
// it is: there is nothing to migrate, and an open is not a write.
func TestConfigWithoutPresetsReadsTheDefaults(t *testing.T) {
	dir := t.TempDir()
	original := []byte(`{"format": "remit/3", "fsrs": {"desired_retention": 0.85}}`)
	writeConfig(t, dir, original)

	lib, err := store.Open(dir, store.Options{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer lib.Close()

	samePresets(t, lib.Presets(), store.DefaultPresets())
	if got := lib.ScheduleParams().DesiredRetention; got != 0.85 {
		t.Errorf("retention = %v, want the file's 0.85", got)
	}
	if after := read(t, filepath.Join(dir, "remit.json")); !bytes.Equal(after, original) {
		t.Errorf("an open rewrote a current file:\n%s", after)
	}
}

// A migration that cannot write its backup leaves both files alone: the library
// opens with what it read, the defaults fill the presets, and one line says why
// the file was not migrated.
func TestFailedMigrationLeavesTheFile(t *testing.T) {
	dir := t.TempDir()
	original := []byte(`{"format": "remit/2", "fsrs": {"desired_retention": 0.8}}`)
	writeConfig(t, dir, original)
	// A directory where the backup belongs is a backup that cannot be written.
	if err := os.Mkdir(filepath.Join(dir, "remit.json.bak"), 0o755); err != nil {
		t.Fatal(err)
	}
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	lib, err := store.Open(dir, store.Options{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer lib.Close()

	if after := read(t, filepath.Join(dir, "remit.json")); !bytes.Equal(after, original) {
		t.Errorf("a failed migration changed remit.json:\n%s", after)
	}
	samePresets(t, lib.Presets(), store.DefaultPresets())
	if got := lib.ScheduleParams().DesiredRetention; got != 0.8 {
		t.Errorf("retention = %v, want the 0.8 the file declares", got)
	}
	if !strings.Contains(logged.String(), "not migrated") {
		t.Errorf("a failed migration logged %q, want one line saying why", logged.String())
	}
}

// A file this build does not know is refused with the version it declares, and
// not rewritten: a newer Remit wrote it and only that build reads it.
func TestNewerConfigIsRefused(t *testing.T) {
	dir := t.TempDir()
	original := []byte(`{"format": "remit/4"}`)
	writeConfig(t, dir, original)

	_, err := store.Open(dir, store.Options{})
	if err == nil {
		t.Fatal("Open of a newer library succeeded")
	}
	if !strings.Contains(err.Error(), "remit/4") {
		t.Errorf("Open = %v, want the version named", err)
	}
	if after := read(t, filepath.Join(dir, "remit.json")); !bytes.Equal(after, original) {
		t.Errorf("a refused open changed remit.json:\n%s", after)
	}
	if _, err := os.Stat(filepath.Join(dir, "remit.json.bak")); !os.IsNotExist(err) {
		t.Errorf("a refused open wrote a backup: %v", err)
	}
}

// A key a person added survives the writes the library makes for something
// else: approving a plugin, choosing its role, and setting the rhythm.
func TestUnknownConfigKeysSurviveAWrites(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	config := remitJSON(t, dir)
	config["mine"] = map[string]any{"rate": 1.5}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "remit.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	lib = reopen(t, lib, dir, clk) // the handle reads the hand-added key

	if err := lib.SetEnabledExtensions([]string{"acme.cards"}); err != nil {
		t.Fatalf("SetEnabledExtensions: %v", err)
	}
	if err := lib.SetRole(core.RoleCardsDerive, "acme.cards"); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	if err := lib.SetPresets(customPresets()); err != nil {
		t.Fatalf("SetPresets: %v", err)
	}

	mine, err := json.Marshal(remitJSON(t, dir)["mine"])
	if err != nil {
		t.Fatal(err)
	}
	if string(mine) != `{"rate":1.5}` {
		t.Errorf("the hand-added key reads %s after the library's writes", mine)
	}
}

// The rhythm is configuration, not derived state: an index rebuilt from the
// text keeps what the library's file says.
func TestPresetsSurviveAnIndexRebuild(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	if err := lib.SetPresets(customPresets()); err != nil {
		t.Fatalf("SetPresets: %v", err)
	}
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	samePresets(t, lib.Presets(), customPresets())

	lib = reopen(t, lib, dir, clk)
	if err := os.RemoveAll(filepath.Join(dir, ".remit")); err != nil {
		t.Fatal(err)
	}
	lib = reopen(t, lib, dir, clk)
	samePresets(t, lib.Presets(), customPresets())
}

// A preset's retention replaces the scheduling plugin's target for this
// library, and clearing it gives the target back: the fsrs section is the
// plugin's value throughout.
func TestRetentionOverrideSchedulesTheLibrary(t *testing.T) {
	dir := t.TempDir()
	clk := &clock{at: t0}
	presets := store.DefaultPresets()
	retention := 0.8
	presets.Review.Retention = &retention
	lib, err := store.Create(dir, store.Options{Now: clk.Now, Presets: presets})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	if got := lib.ScheduleParams().DesiredRetention; got != 0.8 {
		t.Errorf("retention = %v, want the preset's 0.8", got)
	}
	fsrs, _ := remitJSON(t, dir)["fsrs"].(map[string]any)
	if fsrs["desired_retention"] != schedule.DefaultRetention {
		t.Errorf("fsrs.desired_retention = %v, want the plugin's %v kept", fsrs["desired_retention"], schedule.DefaultRetention)
	}

	if err := lib.SetPresets(store.DefaultPresets()); err != nil {
		t.Fatalf("SetPresets: %v", err)
	}
	if got := lib.ScheduleParams().DesiredRetention; got != schedule.DefaultRetention {
		t.Errorf("retention = %v, want the plugin's %v back", got, schedule.DefaultRetention)
	}
	lib = reopen(t, lib, dir, clk)
	if got := lib.ScheduleParams().DesiredRetention; got != schedule.DefaultRetention {
		t.Errorf("a reopen kept %v, want the plugin's %v", got, schedule.DefaultRetention)
	}
}
