package contrib_test

import (
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// The two slots the window grew — the today screen's plugin block and the stats
// overview's — are the host's like the rest: their items parse, keep the when
// they were declared with, and reach the index in drawing order.
func TestTheTodayAndStatsSlots(t *testing.T) {
	raw := `{
	  "commands": [
	    {"id": "p.today", "title": "%today%", "action": "command:today"},
	    {"id": "p.stats", "title": "%stats%", "action": "command:stats"}
	  ],
	  "menus": {
	    "today.widget": [{"command": "p.today", "group": "plugin", "when": "area == today && library.open"}],
	    "stats.block": [{"command": "p.stats", "group": "plugin", "when": "area == stats && sub == overview"}]
	  },
	  "locales": {"en": {"today": "Today's cards", "stats": "Lapses by week"}}
	}`
	got, problems := contrib.Parse(extension("p.x", "x", raw), contrib.Options{})
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	index, problems := contrib.BuildIndex([]contrib.Contributions{got})
	if len(problems) != 0 {
		t.Fatalf("index: %v", problems)
	}

	today := index.Menu(contrib.SlotTodayWidget)
	if len(today) != 1 || today[0].Command.ID != "p.today" {
		t.Fatalf("today.widget = %+v", today)
	}
	if !today[0].When.Eval(contrib.Context{Area: "today", LibraryOpen: true}) {
		t.Error("the today item hides on the today screen")
	}
	if today[0].When.Eval(contrib.Context{Area: "write", LibraryOpen: true}) {
		t.Error("the today item shows outside its area")
	}

	stats := index.Menu(contrib.SlotStatsBlock)
	if len(stats) != 1 || stats[0].Command.ID != "p.stats" {
		t.Fatalf("stats.block = %+v", stats)
	}
	if !stats[0].When.Eval(contrib.Context{Area: "stats", Sub: "overview"}) {
		t.Error("the stats item hides on the overview")
	}
	if stats[0].When.Eval(contrib.Context{Area: "stats", Sub: "forecast"}) {
		t.Error("the stats item shows outside its sub-page")
	}

	if !known(contrib.Slots, contrib.SlotTodayWidget) || !known(contrib.Slots, contrib.SlotStatsBlock) {
		t.Errorf("the new slots are not the host's: %v", contrib.Slots)
	}
}

// A slot name the host does not have is still a validation error next to the new
// ones: the window has nowhere to put an item, so the author is told.
func TestAnUnknownSlotNextToTheNewOnes(t *testing.T) {
	raw := `{
	  "commands": [{"id": "p.go", "title": "%a%", "action": "command:go"}],
	  "menus": {
	    "today.widgets": [{"command": "p.go"}],
	    "stats.block": [{"command": "p.go"}]
	  },
	  "locales": {"en": {"a": "A"}}
	}`
	got, problems := contrib.Parse(extension("p.x", "x", raw), contrib.Options{})
	if len(problems) != 1 || !strings.Contains(problems[0].Error(), "today.widgets") {
		t.Fatalf("problems = %v, want the typo alone", problems)
	}
	if len(got.Menus[contrib.SlotStatsBlock]) != 1 {
		t.Errorf("the known slot was dropped with the typo: %+v", got.Menus)
	}
}

// status.item is reserved: a manifest declaring it validates and keeps its items,
// because a later apiVersion draws them, but this host shows nothing there — so
// the declaration is a problem the settings page lists, not a silent no-op.
func TestReservedSlotIsKeptAndReported(t *testing.T) {
	raw := `{
	  "commands": [{"id": "p.status", "title": "%a%", "action": "command:status"}],
	  "menus": {"status.item": [{"command": "p.status"}]},
	  "locales": {"en": {"a": "A"}}
	}`
	got, problems := contrib.Parse(extension("p.x", "x", raw), contrib.Options{})
	if len(got.Menus[contrib.SlotStatusItem]) != 1 {
		t.Errorf("the reserved slot's item was dropped: %+v", got.Menus)
	}
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want one saying the slot is reserved", problems)
	}
	says := problems[0].Error()
	if !strings.Contains(says, contrib.SlotStatusItem) || !strings.Contains(says, "reserved") {
		t.Errorf("the problem does not say what is reserved: %q", says)
	}

	if !contrib.SlotReserved(contrib.SlotStatusItem) {
		t.Error("status.item is not exposed as reserved")
	}
	if contrib.SlotReserved(contrib.SlotTodayWidget) {
		t.Error("a slot the host draws is reported as reserved")
	}
	// The two sets are separate: a slot is either drawn or reserved, never both.
	for _, reserved := range contrib.ReservedSlots {
		if known(contrib.Slots, reserved) {
			t.Errorf("%q is both drawn and reserved", reserved)
		}
	}
}

// The contract's area values are the ones the host reads, and an area is not a
// mode: a when expression written against either keeps its meaning.
func TestAreasAreTheContracts(t *testing.T) {
	want := []string{"today", "write", "read", "recall", "stats", "settings", "plugin"}
	if len(contrib.Areas) != len(want) {
		t.Fatalf("areas = %v, want %v", contrib.Areas, want)
	}
	for i, area := range want {
		if contrib.Areas[i] != area {
			t.Fatalf("areas = %v, want %v", contrib.Areas, want)
		}
		if !contrib.When("area == "+area, contrib.Context{Area: area}) {
			t.Errorf("area == %s does not read the area", area)
		}
	}
}

// known reports whether list holds value, so a test names the list it checks.
func known(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
