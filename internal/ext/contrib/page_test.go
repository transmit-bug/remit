package contrib_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/transmit-bug/remit/internal/ext/contrib"
)

// leechesPage is the page tree the contract's example answers: layout nodes and
// semantic leaves, with no pixel in sight.
const leechesPage = `{
  "kind": "page",
  "children": [
    {
      "kind": "section",
      "title": "%section.overview%",
      "children": [
        {
          "kind": "stack",
          "direction": "row",
          "gap": "m",
          "children": [
            {"kind": "stat", "label": "%stat.leeches%", "value": "12"},
            {"kind": "stat", "label": "%stat.suspended%", "value": "3"}
          ]
        },
        {
          "kind": "grid",
          "columns": 2,
          "children": [
            {"kind": "table", "columns": ["%col.card%", "%col.lapses%"], "rows": [["…", "9"]]},
            {"kind": "markdown", "text": "…"}
          ]
        }
      ]
    }
  ]
}`

// A page is a tree of the contract's layout nodes and semantic leaves, read
// with the tokens it may name and nothing to style with.
func TestParsePage(t *testing.T) {
	page, err := contrib.ParsePage([]byte(leechesPage))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if page.Kind != contrib.KindPage || len(page.Children) != 1 {
		t.Fatalf("root = %q with %d children", page.Kind, len(page.Children))
	}
	section := page.Children[0]
	if section.Kind != contrib.KindSection || section.Title != "%section.overview%" {
		t.Errorf("section = %+v", section)
	}
	stack := section.Children[0]
	if stack.Direction != contrib.DirectionRow || stack.Gap != contrib.GapM {
		t.Errorf("stack direction %q gap %q", stack.Direction, stack.Gap)
	}
	if stack.Children[0].Kind != contrib.KindStat || stack.Children[0].Value != "12" {
		t.Errorf("stat = %+v", stack.Children[0])
	}
	grid := section.Children[1]
	if grid.Columns != 2 {
		t.Errorf("grid columns = %d", grid.Columns)
	}
	table := grid.Children[0]
	if len(table.Headings) != 2 || len(table.Rows) != 1 || table.Rows[0][1] != "9" {
		t.Errorf("table = %+v", table)
	}
	if grid.Children[1].Kind != contrib.KindMarkdown || grid.Children[1].Text != "…" {
		t.Errorf("markdown = %+v", grid.Children[1])
	}
}

// A node kind this host does not know keeps its place and its name, so the page
// shows where the newer plugin's node goes and draws the rest around it.
func TestUnknownKindDegradesInPlace(t *testing.T) {
	page, err := contrib.ParsePage([]byte(`{"kind":"page","children":[
		{"kind":"text","text":"before"},
		{"kind":"sparkline","children":[{"kind":"text","text":"inside"}]},
		{"kind":"text","text":"after"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(page.Children) != 3 {
		t.Fatalf("the unknown node was dropped: %+v", page.Children)
	}
	unknown := page.Children[1]
	if unknown.Kind != "sparkline" || unknown.Source != "sparkline" {
		t.Errorf("unknown node = %+v, want its kind kept", unknown)
	}
	if len(unknown.Children) != 1 || unknown.Children[0].Text != "inside" {
		t.Errorf("the unknown node lost its children: %+v", unknown.Children)
	}
}

// The contract's whole-page mistakes are errors the page shows in place of a
// tree, each naming the node that broke it.
func TestParsePageRefuses(t *testing.T) {
	deep := `{"kind":"page"`
	for i := 0; i < contrib.MaxDepth; i++ {
		deep += `,"children":[{"kind":"stack"`
	}
	deep += strings.Repeat("}]", contrib.MaxDepth+1)

	wide := `{"kind":"page","children":[`
	for i := 0; i < contrib.MaxNodes; i++ {
		if i > 0 {
			wide += ","
		}
		wide += `{"kind":"text","text":"x"}`
	}
	wide += `]}`

	manyRows := strings.Builder{}
	manyRows.WriteString(`{"kind":"page","children":[{"kind":"table","columns":["a"],"rows":[`)
	for i := 0; i <= contrib.MaxTableRows; i++ {
		if i > 0 {
			manyRows.WriteString(",")
		}
		manyRows.WriteString(`["x"]`)
	}
	manyRows.WriteString(`]}]}`)

	huge := `{"kind":"page","children":[{"kind":"text","text":"` +
		strings.Repeat("x", contrib.MaxPageBytes) + `"}]}`

	for _, tc := range []struct {
		name string
		raw  string
		says string
	}{
		{"depth", deep, "deep"},
		{"node count", wide, "nodes"},
		{"table rows", manyRows.String(), "rows"},
		{"bytes", huge, "bytes"},
		{"a split of three", `{"kind":"page","children":[{"kind":"split","ratio":1,"children":[{"kind":"text","text":"a"},{"kind":"text","text":"b"},{"kind":"text","text":"c"}]}]}`, "two panes"},
		{"a split of one", `{"kind":"page","children":[{"kind":"split","ratio":1,"children":[{"kind":"text","text":"a"}]}]}`, "two panes"},
		{"a split ratio", `{"kind":"page","children":[{"kind":"split","ratio":3,"children":[{"kind":"text","text":"a"},{"kind":"text","text":"b"}]}]}`, "ratio"},
		{"a section without a title", `{"kind":"page","children":[{"kind":"section","children":[]}]}`, "title"},
		{"a tab without a label", `{"kind":"page","children":[{"kind":"tabs","tabs":[{"label":"","children":[]}]}]}`, "label"},
		{"no tabs", `{"kind":"page","children":[{"kind":"tabs"}]}`, "no tab"},
		{"grid columns", `{"kind":"page","children":[{"kind":"grid","columns":5,"children":[]}]}`, "columns"},
		{"a gap token", `{"kind":"page","children":[{"kind":"stack","gap":"huge","children":[]}]}`, "gap"},
		{"a tone token", `{"kind":"page","children":[{"kind":"callout","tone":"purple","text":"x"}]}`, "tone"},
		{"a table row of the wrong width", `{"kind":"page","children":[{"kind":"table","columns":["a","b"],"rows":[["only one"]]}]}`, "cells"},
		{"an action without a command", `{"kind":"page","children":[{"kind":"actions","items":[{"label":"Go"}]}]}`, "command"},
		{"a node without a kind", `{"kind":"page","children":[{"text":"x"}]}`, "kind"},
		{"nothing", ``, "nothing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := contrib.ParsePage([]byte(tc.raw))
			if err == nil {
				t.Fatalf("the host accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q does not say %q", err, tc.says)
			}
		})
	}
}

// The contract bans styling, not just ignores it: a field that would name a
// width, a colour or a size is refused by name, so a plugin author is told.
func TestPageRefusesStyleFields(t *testing.T) {
	for _, field := range []string{"width", "height", "color", "font", "margin", "padding", "position"} {
		raw := fmt.Sprintf(`{"kind":"page","children":[{"kind":"text","text":"x","%s":8}]}`, field)
		_, err := contrib.ParsePage([]byte(raw))
		if err == nil {
			t.Fatalf("a page naming %s was accepted", field)
		}
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error %q does not name %s", err, field)
		}
	}
}

// Tabs and a tree are the two nodes whose children are not plain children; both
// read and both count against the depth.
func TestParsePageReadsTabsAndTrees(t *testing.T) {
	raw := `{"kind":"page","children":[
		{"kind":"tabs","tabs":[
			{"label":"%tab.one%","children":[{"kind":"text","text":"one"}]},
			{"label":"%tab.two%","children":[{"kind":"list","items":[{"label":"row","value":"2"}]}]}
		]},
		{"kind":"tree","items":[{"label":"root","items":[{"label":"leaf"}]}]},
		{"kind":"chart","points":[{"label":"a","value":3},{"label":"b","value":5}]},
		{"kind":"card","title":"%card.title%","children":[]},
		{"kind":"callout","title":"%callout.title%","text":"body","tone":"warn"},
		{"kind":"form","fields":[{"label":"%form.field%","value":"v"}]}
	]}`
	page, err := contrib.ParsePage([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tabs := page.Children[0]
	if len(tabs.Tabs) != 2 || tabs.Tabs[0].Label != "%tab.one%" || len(tabs.Tabs[1].Children) != 1 {
		t.Fatalf("tabs = %+v", tabs.Tabs)
	}
	tree := page.Children[1]
	if len(tree.Items) != 1 || len(tree.Items[0].Items) != 1 {
		t.Errorf("tree items = %+v", tree.Items)
	}
	chart := page.Children[2]
	if len(chart.Points) != 2 || chart.Points[1].Value != 5 {
		t.Errorf("chart points = %+v", chart.Points)
	}
	if page.Children[4].Tone != contrib.ToneWarn {
		t.Errorf("callout tone = %q", page.Children[4].Tone)
	}
	if page.Children[5].Fields[0].Value != "v" {
		t.Errorf("form fields = %+v", page.Children[5].Fields)
	}
}

// A page's answer as a plugin sends it is JSON; the parser is the only thing
// between that and the widgets, so it is tested with the decoder's own rules.
func TestParsePageNeedsJSON(t *testing.T) {
	var page contrib.Node
	if err := json.Unmarshal([]byte(leechesPage), &page); err == nil {
		t.Error("a contrib.Node accepted a plugin's raw JSON; the host parses it instead")
	}
}
