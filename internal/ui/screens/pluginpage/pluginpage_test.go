package pluginpage_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/screens/pluginpage"
)

// pageState is the screen with a tree the contract allows, as the app hands it
// over: the data command's answer, already parsed, and no error.
func pageState(t *testing.T, raw string) *pluginpage.State {
	t.Helper()
	tree, err := contrib.ParsePage([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &pluginpage.State{Title: "Leeches", Tree: tree}
}

// messages resolves a page's own strings as the app does: the plugin's catalog,
// in this frame's language.
func messages(c *ui.Context, raw string) string {
	catalog := contrib.Catalog{
		"en": {"page.title": "Leeches", "section.cards": "Cards", "tab.one": "One", "tab.two": "Two"},
	}
	return contrib.Message(raw).Text(catalog, c.Locale())
}

// deps is the page's app, as the screen reads it: what a test drove.
type deps struct {
	ran     []string
	retried int
}

func (d *deps) deps() pluginpage.Deps {
	return pluginpage.Deps{
		Message: messages,
		Run:     func(_ *ui.Context, command string) { d.ran = append(d.ran, command) },
		Retry:   func(*ui.Context) { d.retried++ },
	}
}

// Every layout node and every semantic leaf the contract names draws with the
// host's widgets: the tree is read, not painted, and the text a plugin wrote is
// what the page says back.
func TestEveryNodeTheHostKnowsDraws(t *testing.T) {
	raw := `{"kind":"page","children":[
	  {"kind":"section","title":"%section.cards%","description":"what a note makes","children":[
	    {"kind":"stack","direction":"row","gap":"m","children":[
	      {"kind":"stat","label":"notes","value":"3"},
	      {"kind":"text","text":"beside the number","tone":"warn"}
	    ]},
	    {"kind":"grid","columns":2,"children":[
	      {"kind":"markdown","text":"**bold** and plain"},
	      {"kind":"table","columns":["A","B"],"rows":[["a1","b1"],["a2","b2"]]}
	    ]},
	    {"kind":"split","ratio":2,"children":[
	      {"kind":"list","title":"rows","items":[{"label":"first","value":"1"}]},
	      {"kind":"tree","title":"outline","items":[{"label":"root","items":[{"label":"leaf"}]}]}
	    ]},
	    {"kind":"form","fields":[{"label":"Threshold","value":"8"}]},
	    {"kind":"chart","title":"lapses","points":[{"label":"mon","value":2},{"label":"tue","value":5}]},
	    {"kind":"callout","title":"A leech","text":"Make it smaller.","tone":"info"},
	    {"kind":"card","title":"Kept","children":[{"kind":"actions","items":[{"command":"acme.report.run","label":"Run it"}]}]},
	    {"kind":"tabs","tabs":[
	      {"label":"%tab.one%","children":[{"kind":"text","text":"the first tab"}]},
	      {"label":"%tab.two%","children":[{"kind":"text","text":"the second tab"}]}
	    ]}
	  ]}
	]}`
	s := pageState(t, raw)
	d := &deps{}
	tt := ui.NewTester(func(c *ui.Context) { pluginpage.View(c, s, d.deps()) }, 1024, 720)
	tt.Frame()

	for _, want := range []string{
		"Leeches", "Cards", "what a note makes", "3", "notes", "beside the number",
		"a1", "b1", "rows", "first", "outline", "root", "Threshold", "8",
		"A leech", "Make it smaller.", "Kept", "Run it", "One", "Two", "the first tab",
	} {
		if !tt.HasText(want) {
			t.Fatalf("the page is missing %q; texts %q", want, tt.Texts())
		}
	}
	// A chart is the host's plot: its axis labels are the points it was given.
	if !tt.HasText("mon") || !tt.HasText("tue") {
		t.Errorf("the chart did not draw its axis; texts %q", tt.Texts())
	}
	// A tree's branches start closed, as a file tree's do; its roots are rows.
	if err := tt.Click("root"); err != nil {
		t.Errorf("the tree's root: %v", err)
	}
	if !tt.HasText("bold") {
		t.Errorf("the markdown leaf did not render; texts %q", tt.Texts())
	}

	// The action is a button, and its command reaches the app.
	if err := tt.Click("Run it"); err != nil {
		t.Fatalf("the action: %v", err)
	}
	if len(d.ran) != 1 || d.ran[0] != "acme.report.run" {
		t.Errorf("the action ran %v", d.ran)
	}
}

// A page over the contract's limits is an error the page shows, and a retry is
// the one action it offers: never a blank page.
func TestFailedPageSaysWhyAndRetries(t *testing.T) {
	d := &deps{}
	s := &pluginpage.State{Title: "Leeches", Err: "page: the data command answered nothing"}
	tt := ui.NewTester(func(c *ui.Context) { pluginpage.View(c, s, d.deps()) }, 1024, 720)
	tt.Frame()

	if !tt.HasText("This page could not be loaded: page: the data command answered nothing") {
		t.Fatalf("the failed page says nothing; texts %q", tt.Texts())
	}
	if err := tt.Click("Retry"); err != nil {
		t.Fatalf("the retry: %v", err)
	}
	if d.retried != 1 {
		t.Errorf("the retry ran %d times", d.retried)
	}

	// The tree arriving is the page again.
	s.Err = ""
	s.Tree = pageState(t, `{"kind":"page","children":[{"kind":"text","text":"loaded"}]}`).Tree
	tt.Frame()
	if !tt.HasText("loaded") || tt.HasText("Retry") {
		t.Errorf("the page did not draw its tree; texts %q", tt.Texts())
	}
}

// A page without a tree yet is a skeleton, not an error: the title is up and the
// content says it is on its way.
func TestLoadingPageShowsASkeleton(t *testing.T) {
	s := &pluginpage.State{Title: "Leeches", Loading: true}
	tt := ui.NewTester(func(c *ui.Context) { pluginpage.View(c, s, pluginpage.Deps{Message: messages}) }, 1024, 720)
	tt.Frame()
	if !tt.HasText("Loading…") {
		t.Fatalf("the loading page says nothing; texts %q", tt.Texts())
	}
}

// An unknown node keeps its place: the page names the kind it cannot draw and
// renders everything around it.
func TestUnknownNodeDrawsInPlace(t *testing.T) {
	s := pageState(t, `{"kind":"page","children":[
	  {"kind":"text","text":"before"},
	  {"kind":"sparkline","children":[{"kind":"text","text":"inside"}]},
	  {"kind":"text","text":"after"}]}`)
	d := &deps{}
	tt := ui.NewTester(func(c *ui.Context) { pluginpage.View(c, s, d.deps()) }, 1024, 720)
	tt.Frame()

	if !tt.HasText("before") || !tt.HasText("after") {
		t.Fatalf("the unknown node cost its neighbours; texts %q", tt.Texts())
	}
	if !tt.HasText("sparkline is not something this version of Remit draws.") {
		t.Errorf("the unknown node drew nothing in place; texts %q", tt.Texts())
	}
}

// The narrow window is the host's business: a grid reads as one column and a
// split as one stack, so 720×520 has nothing to scroll sideways.
func TestNarrowPageCollapsesItsLayouts(t *testing.T) {
	raw := `{"kind":"page","children":[
	  {"kind":"grid","columns":2,"children":[
	    {"kind":"text","text":"left cell"},
	    {"kind":"text","text":"right cell"}
	  ]},
	  {"kind":"split","ratio":1,"children":[
	    {"kind":"text","text":"first pane"},
	    {"kind":"text","text":"second pane"}
	  ]}
	]}`
	s := pageState(t, raw)
	d := &deps{}
	for _, size := range []struct {
		name    string
		w, h    int
		stacked bool
	}{
		{"wide", 1024, 720, false},
		{"narrow", 720, 520, true},
	} {
		t.Run(size.name, func(t *testing.T) {
			tt := ui.NewTester(func(c *ui.Context) { pluginpage.View(c, s, d.deps()) }, size.w, size.h)
			tt.Frame()
			left, _ := tt.Find("left cell")
			right, _ := tt.Find("right cell")
			first, _ := tt.Find("first pane")
			second, _ := tt.Find("second pane")
			if size.stacked {
				if right.Y <= left.Y {
					t.Errorf("the narrow grid kept its columns: left y %.1f, right y %.1f", left.Y, right.Y)
				}
				if second.Y <= first.Y {
					t.Errorf("the narrow split kept its panes side by side: %.1f, %.1f", first.Y, second.Y)
				}
				return
			}
			if right.X <= left.X {
				t.Errorf("the wide grid stacked: left x %.1f, right x %.1f", left.X, right.X)
			}
			if second.X <= first.X {
				t.Errorf("the wide split stacked: first x %.1f, second x %.1f", first.X, second.X)
			}
		})
	}
}

// The tabs are a tab list: choosing one shows its content, and the labels are
// the plugin's, in the frame's language.
func TestTabsChooseTheirContent(t *testing.T) {
	s := pageState(t, `{"kind":"page","children":[{"kind":"tabs","tabs":[
	  {"label":"%tab.one%","children":[{"kind":"text","text":"one's content"}]},
	  {"label":"%tab.two%","children":[{"kind":"text","text":"two's content"}]}
	]}]}`)
	d := &deps{}
	tt := ui.NewTester(func(c *ui.Context) { pluginpage.View(c, s, d.deps()) }, 1024, 720)
	tt.Frame()
	if !tt.HasText("one's content") || tt.HasText("two's content") {
		t.Fatalf("the first tab is not the one shown; texts %q", tt.Texts())
	}
	if err := tt.Click("Two"); err != nil {
		t.Fatalf("the second tab: %v", err)
	}
	if !tt.HasText("two's content") || tt.HasText("one's content") {
		t.Errorf("choosing a tab did not show its content; texts %q", tt.Texts())
	}
	if strings.Join(tt.Texts(), ",") == "" {
		t.Error("the page drew nothing")
	}
}
