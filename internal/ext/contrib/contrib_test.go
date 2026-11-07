package contrib_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// leeches is the contract's example manifest, with the menu item and the page's
// data command matching the commands it declares.
const leeches = `{
  "commands": [
    {
      "id": "acme.leeches.show",
      "title": "%command.show%",
      "category": "%category.review%",
      "icon": "bug",
      "keybinding": "mod+shift+l",
      "when": "library.open && (mode == review || mode == browse)",
      "action": "command:acme.showLeeches"
    }
  ],
  "menus": {
    "review.card": [
      {"command": "acme.leeches.show", "group": "plugin", "order": 100, "when": "card.present"}
    ]
  },
  "pages": [
    {
      "id": "acme.leeches",
      "title": "%page.leeches%",
      "icon": "bug",
      "nav": "plugins",
      "when": "library.open",
      "data": "command:acme.leechesData"
    }
  ],
  "settings": [
    {"key": "threshold", "type": "number", "title": "%setting.threshold%", "default": 8}
  ],
  "locales": {
    "en": {
      "command.show": "Leech report",
      "category.review": "Review",
      "page.leeches": "Leeches",
      "setting.threshold": "Leech threshold"
    },
    "zh-Hans": {
      "command.show": "水蛭报告",
      "page.leeches": "水蛭",
      "setting.threshold": "水蛭阈值"
    }
  }
}`

// icons is a host icon set with the one icon these manifests name.
func icons(name string) bool { return name == "bug" }

func extension(id, name, contributes string) ext.Extension {
	return ext.Extension{Manifest: ext.Manifest{ID: id, Name: name, Contributes: json.RawMessage(contributes)}}
}

// The example manifest indexes into what the window asks for: one command, one
// menu item, one page, one setting and the plugin's own strings.
func TestParseTheContractExample(t *testing.T) {
	got, problems := contrib.Parse(extension("acme.leeches", "leeches", leeches), contrib.Options{Icon: icons})
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	if got.Plugin != "acme.leeches" || got.Name != "leeches" {
		t.Errorf("identity = %q/%q", got.Plugin, got.Name)
	}
	if len(got.Commands) != 1 {
		t.Fatalf("commands = %+v", got.Commands)
	}
	command := got.Commands[0]
	if command.ID != "acme.leeches.show" || command.Icon != "bug" || command.Keybinding != "mod+shift+l" {
		t.Errorf("command = %+v", command)
	}
	if command.Action.Name != "acme.showLeeches" || command.Action.Host != "" {
		t.Errorf("action = %+v", command.Action)
	}
	if !command.When.Eval(contrib.Context{Mode: "review", LibraryOpen: true}) {
		t.Error("the command does not show in review")
	}
	if command.When.Eval(contrib.Context{Mode: "notes", LibraryOpen: true}) {
		t.Error("the command shows in notes, where its when says no")
	}
	if text := command.Title.Text(got.Catalog, "zh-Hans"); text != "水蛭报告" {
		t.Errorf("zh-Hans title = %q", text)
	}
	if text := command.Category.Text(got.Catalog, "zh-Hans"); text != "Review" {
		t.Errorf("a key only English holds falls back to it, got %q", text)
	}
	if text := (contrib.Message("%nowhere%")).Text(got.Catalog, "zh-Hans"); text != "nowhere" {
		t.Errorf("an untranslated key shows the key, got %q", text)
	}
	if items := got.Menus["review.card"]; len(items) != 1 || items[0].Command != "acme.leeches.show" {
		t.Errorf("menu = %+v", got.Menus)
	}
	if len(got.Pages) != 1 || got.Pages[0].Data != "acme.leechesData" || got.Pages[0].Nav != contrib.NavPlugins {
		t.Errorf("pages = %+v", got.Pages)
	}
	if len(got.Settings) != 1 || got.Settings[0].Default != float64(8) || got.Settings[0].Type != contrib.SettingNumber {
		t.Errorf("settings = %+v", got.Settings)
	}
}

// One illegal entry costs only itself: the plugin still enables, and the window
// still shows everything its manifest got right.
func TestOneBadEntryCostsOnlyItself(t *testing.T) {
	raw := `{
	  "commands": [
	    {"id": "acme.good", "title": "%a%", "action": "command:good"},
	    {"id": "acme.bad", "title": "%a%", "icon": "no-such-icon", "action": "command:bad"},
	    {"id": "acme.also-good", "title": "%a%", "action": "command:also"}
	  ],
	  "pages": [
	    {"id": "acme.page", "title": "%a%", "data": "command:page"},
	    {"id": "acme.bad-page", "title": "%a%", "nav": "top", "data": "command:page"}
	  ],
	  "locales": {"en": {"a": "A"}}
	}`
	got, problems := contrib.Parse(extension("acme.x", "x", raw), contrib.Options{Icon: icons})
	if len(got.Commands) != 2 {
		t.Errorf("commands = %+v, want the two legal ones", got.Commands)
	}
	if len(got.Pages) != 1 {
		t.Errorf("pages = %+v, want the one legal one", got.Pages)
	}
	if len(problems) != 2 {
		t.Fatalf("problems = %v, want one per dropped contribution", problems)
	}
	for _, want := range []string{"no-such-icon", "nav"} {
		found := false
		for _, problem := range problems {
			if strings.Contains(problem.Error(), want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem names %q: %v", want, problems)
		}
	}
}

// A menu slot is the host's: an unknown one is a validation error, because the
// host has nowhere to put it, while the slots it knows still stand.
func TestMenusAreTheHosts(t *testing.T) {
	raw := `{
	  "commands": [{"id": "acme.go", "title": "%a%", "action": "command:go"}],
	  "menus": {
	    "review.card": [{"command": "acme.go"}],
	    "toolbar.top": [{"command": "acme.go"}],
	    "note.block": [{"command": "acme.missing"}]
	  },
	  "locales": {"en": {"a": "A"}}
	}`
	got, problems := contrib.Parse(extension("acme.x", "x", raw), contrib.Options{})
	if len(got.Menus["review.card"]) != 1 {
		t.Errorf("the known slot lost its item: %+v", got.Menus)
	}
	if len(got.Menus["note.block"]) != 0 {
		t.Errorf("a menu item pointing at a command that is not declared was kept")
	}
	var says []string
	for _, problem := range problems {
		says = append(says, problem.Error())
	}
	joined := strings.Join(says, "\n")
	for _, want := range []string{"toolbar.top", "acme.missing"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no problem names %q:\n%s", want, joined)
		}
	}
}

// The manifest's rules, one refusal each.
func TestContributionRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		says string
	}{
		{"a literal title", `{"commands":[{"id":"a.b","title":"Show","action":"command:x"}]}`, "literal"},
		{"a reserved id", `{"commands":[{"id":"remit.b","title":"%a%","action":"command:x"}],"locales":{"en":{"a":"A"}}}`, "remit."},
		{"an unknown host action", `{"commands":[{"id":"a.b","title":"%a%","action":"remit.command.launchMissiles"}],"locales":{"en":{"a":"A"}}}`, "launchMissiles"},
		{"a keybinding that is not one", `{"commands":[{"id":"a.b","title":"%a%","keybinding":"hyper+l","action":"command:x"}],"locales":{"en":{"a":"A"}}}`, "hyper"},
		{"an action that names nothing", `{"commands":[{"id":"a.b","title":"%a%","action":"command:"}],"locales":{"en":{"a":"A"}}}`, "nothing"},
		{"a setting type the host has not", `{"settings":[{"key":"k","type":"date","title":"%a%"}],"locales":{"en":{"a":"A"}}}`, "date"},
		{"a setting default of another type", `{"settings":[{"key":"k","type":"number","title":"%a%","default":"eight"}],"locales":{"en":{"a":"A"}}}`, "number"},
		{"an untranslated key", `{"commands":[{"id":"a.b","title":"%missing%","action":"command:x"}]}`, "missing"},
		{"a kind the host has not", `{"widgets":[{"id":"w"}],"locales":{}}`, "widgets"},
		{"a field the contract has not", `{"commands":[{"id":"a.b","title":"%a%","action":"command:x","width":12}],"locales":{"en":{"a":"A"}}}`, "width"},
		{"two commands with one id", `{"commands":[{"id":"a.b","title":"%a%","action":"command:x"},{"id":"a.b","title":"%a%","action":"command:y"}],"locales":{"en":{"a":"A"}}}`, "twice"},
		{"a page with no data", `{"pages":[{"id":"p","title":"%a%"}],"locales":{"en":{"a":"A"}}}`, "data"},
		{"a bad when", `{"commands":[{"id":"a.b","title":"%a%","when":"mode ==","action":"command:x"}],"locales":{"en":{"a":"A"}}}`, "when"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := contrib.Parse(extension("acme.x", "x", tc.raw), contrib.Options{Icon: icons})
			var says []string
			for _, problem := range problems {
				says = append(says, problem.Error())
			}
			joined := strings.Join(says, "\n")
			if !strings.Contains(joined, tc.says) {
				t.Fatalf("no problem names %q:\n%s", tc.says, joined)
			}
		})
	}
}

// A contribution a plugin did not declare is not one the window can reach: the
// index answers only what the manifest holds, and an empty manifest holds
// nothing.
func TestUndeclaredContributionsAreNotThere(t *testing.T) {
	got, problems := contrib.Parse(extension("acme.x", "x", ""), contrib.Options{})
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	index, problems := contrib.BuildIndex([]contrib.Contributions{got})
	if len(problems) != 0 {
		t.Fatalf("index: %v", problems)
	}
	if len(index.Commands()) != 0 || len(index.Pages()) != 0 || len(index.Menu("review.card")) != 0 {
		t.Errorf("a manifest with no contributes granted %+v", index.Commands())
	}
	if _, ok := index.Command("acme.hidden"); ok {
		t.Error("the index answered a command no manifest declared")
	}
}

// A catalog a manifest does not inline is the plugin's own file, read from its
// directory; the inline entry wins where both hold a key.
func TestCatalogFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "locales"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "locales", "fr.json"), []byte(`{"a":"Un","b":"B"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	e := extension("acme.x", "x", `{
	  "commands": [{"id":"a.b","title":"%a%","action":"command:x"},{"id":"a.c","title":"%b%","action":"command:y"}],
	  "locales": {"fr": {"a": "In French"}, "en": {"a": "A", "b": "B"}}
	}`)
	e.Dir = dir
	got, problems := contrib.Parse(e, contrib.Options{})
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	if text := got.Commands[0].Title.Text(got.Catalog, "fr"); text != "In French" {
		t.Errorf("the inline catalog did not win: %q", text)
	}
	if text := got.Commands[1].Title.Text(got.Catalog, "fr"); text != "B" {
		t.Errorf("the file catalog was not read: %q", text)
	}
}

// One id, one keybinding: the later plugin loses the claim, and the problem
// names both sides so the author knows who holds it.
func TestIndexRefusesTwoClaimants(t *testing.T) {
	plugin := func(id, name, command, binding string) contrib.Contributions {
		return contrib.Contributions{
			Plugin: id,
			Name:   name,
			Commands: []contrib.Command{{
				ID:         command,
				Title:      "%a%",
				Keybinding: binding,
				Action:     contrib.Action{Name: "run"},
			}},
			Menus: map[string][]contrib.MenuItem{
				"review.card": {{Command: command, Group: "plugin"}},
			},
		}
	}
	index, problems := contrib.BuildIndex([]contrib.Contributions{
		plugin("b.plugin", "b", "b.command", "mod+k"),
		plugin("a.plugin", "a", "a.command", "mod+k"),
	})
	if len(problems) != 1 || !strings.Contains(problems[0].Error(), "mod+k") {
		t.Fatalf("problems = %v, want the key claimed twice", problems)
	}
	b, ok := index.Plugin("b.plugin")
	if !ok || len(b.Commands) != 1 {
		t.Fatalf("the later plugin lost its command as well as its key: %+v", b)
	}
	if b.Commands[0].Keybinding != "" {
		t.Errorf("the later plugin kept a key %q already bound", b.Commands[0].Keybinding)
	}

	// One id: the later plugin loses the command, and the menu item that pointed
	// at it goes with it.
	index, problems = contrib.BuildIndex([]contrib.Contributions{
		plugin("b.plugin", "b", "shared.command", ""),
		plugin("a.plugin", "a", "shared.command", ""),
	})
	if len(problems) != 1 || !strings.Contains(problems[0].Error(), "shared.command") {
		t.Fatalf("problems = %v, want the id claimed twice", problems)
	}
	if got := index.Commands(); len(got) != 1 || got[0].Name != "a" {
		t.Errorf("commands = %+v, want the plugin named first to keep it", got)
	}
	if got := index.Menu("review.card"); len(got) != 1 || got[0].Name != "a" {
		t.Errorf("the menu kept an item whose command is gone: %+v", got)
	}
}

// Check reads one directory and answers what the host cannot use: the answer
// `ext check` prints, without opening a window.
func TestCheck(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"id":"acme.x","name":"x","version":"1","apiVersion":1,"runtime":"js","entry":"main.js",
	  "contributes":{"commands":[{"id":"a.b","title":"%nope%","action":"command:x"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	problems := contrib.Check(dir, contrib.Options{})
	if len(problems) != 1 || !strings.Contains(problems[0].Error(), "nope") {
		t.Fatalf("problems = %v, want the untranslated key", problems)
	}

	// A directory whose manifest does not load still answers with that reason.
	if problems := contrib.Check(t.TempDir(), contrib.Options{}); len(problems) == 0 {
		t.Error("a directory without a manifest reported nothing")
	}
}
