package ui

import (
	"testing"

	"github.com/transmit-bug/remit/locales"
)

// TestEveryPageIsDeclaredOnce is the table's own contract. A page the enum names
// but the table leaves out has no name and no label: a link that spells it lands
// on the area's first page, and the breadcrumb shows a blank segment. A name
// repeated inside one area sends that link to whichever row the table reaches
// first.
func TestEveryPageIsDeclaredOnce(t *testing.T) {
	if got, want := len(pageTable), 26; got != want {
		t.Fatalf("the table declares %d pages, want %d", got, want)
	}
	type key struct {
		area areaID
		sub  subID
	}
	places := map[key]bool{}
	for _, p := range pageTable {
		if p.name == "" {
			t.Errorf("%v has no stable name", p.sub)
		}
		if p.label == nil {
			t.Errorf("%s has no label", p.name)
		}
		if p.view == nil {
			t.Errorf("%s has no view", p.name)
		}
		switch p.form {
		case formPaper, formWorkspace, formSplit:
		default:
			t.Errorf("%s has form %v, which the stage has no arrangement for", p.name, p.form)
		}
		if places[key{p.area, p.sub}] {
			t.Errorf("%s appears twice in %v", p.name, areaName(p.area))
		}
		places[key{p.area, p.sub}] = true
	}
	// A name is unique inside one area: two areas share "queue" and "review".
	byName := map[areaID]map[string]bool{}
	for _, p := range pageTable {
		if byName[p.area] == nil {
			byName[p.area] = map[string]bool{}
		}
		if byName[p.area][p.name] {
			t.Errorf("%v has two pages named %q", areaName(p.area), p.name)
		}
		byName[p.area][p.name] = true
	}
}

// TestThePageNamesAreTheContract pins the vocabulary a link and a plugin's when
// expression spell: a deep link saved on someone's disk is #/area/sub, and the
// settings area keys its own pages by name. Renaming a page silently breaks both,
// and this is the test that says so.
func TestThePageNamesAreTheContract(t *testing.T) {
	want := map[areaID][]string{
		areaWrite:    {"notes", "list", "search", "tags", "assets", "templates", "tree"},
		areaRead:     {"queue", "archive"},
		areaRecall:   {"queue", "cards", "types", "presets", "history", "relink"},
		areaStats:    {"overview", "heatmap", "forecast"},
		areaSettings: {"appearance", "review", "budget", "keys", "plugins", "agent", "update", "about"},
	}
	for area, names := range want {
		for _, name := range names {
			p, ok := pageOfName(area, name)
			if !ok {
				t.Errorf("%v has no page named %q", areaName(area), name)
				continue
			}
			if got := subOfName(area, name); got != p.sub {
				t.Errorf("%v/%s resolves to %v, want %v", areaName(area), name, got, p.sub)
			}
			if got := p.name; got != name {
				t.Errorf("%v's page %v spells itself %q, want %q", areaName(area), p.sub, got, name)
			}
			if got := p.label(locales.In("en")); got == "" {
				t.Errorf("%v/%s shows no word", areaName(area), name)
			}
		}
	}
	// A name the area does not have lands on its first page rather than nowhere.
	if got := subOfName(areaRecall, "no-such-page"); got != subRecallQueue {
		t.Errorf("an unknown page lands on %v, want the area's first", got)
	}
}

// TestEveryPageIsOneClickAway: each page's own word in the sub-navigation goes
// there, and the area's first page brings the window back — at the size the
// window opens at and at its minimum. A row drawn from a list of its own sends
// the click to the wrong place or nowhere, and a page the row cannot leave
// strands the user in it.
func TestEveryPageIsOneClickAway(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _, tt := openApp(t)
			tt.SetSize(size.w, size.h)
			settle(tt)
			for _, area := range []areaID{areaWrite, areaRead, areaRecall, areaStats, areaSettings} {
				pages := pagesOf(area)
				first := pages[0]
				a.setArea(area, first.sub)
				settle(tt)
				for _, p := range pages[1:] {
					if err := tt.Click(p.label(msgs)); err != nil {
						t.Fatalf("%v's row has no %q page: %v", areaName(area), p.name, err)
					}
					settle(tt)
					if a.area != p.area || a.sub != p.sub {
						t.Fatalf("%v's %q page clicked to %v/%v", areaName(area), p.name, a.area, a.sub)
					}
					if err := tt.Click(first.label(msgs)); err != nil {
						t.Fatalf("%v/%s leaves no word to click back: %v", areaName(area), p.name, err)
					}
					settle(tt)
					if a.area != first.area || a.sub != first.sub {
						t.Fatalf("%v/%s was left for %v/%v, want %s", areaName(area), p.name, a.area, a.sub, first.name)
					}
				}
			}
		})
	}
}

// TestEveryPageDrawsAtBothSizes walks the whole table at the two sizes the
// window is judged at (docs/ui-layout-rules.md §8). A page whose view reads what
// no load path derived panics on the way in, and a page that draws nothing at
// 720×520 is one a user cannot use — neither is visible to the tests that visit
// only the pages they open themselves.
func TestEveryPageDrawsAtBothSizes(t *testing.T) {
	for _, size := range []struct {
		name string
		w, h int
	}{{"the size the window opens at", 1024, 720}, {"the minimum", 720, 520}} {
		a, _, tt := openApp(t)
		tt.SetSize(size.w, size.h)
		settle(tt)
		for _, p := range pageTable {
			a.setArea(p.area, p.sub)
			settle(tt)
			if tt.FramePending() {
				t.Errorf("%s never settles at %s", p.name, size.name)
			}
			label := p.label(locales.In("en"))
			if label == "" {
				t.Fatalf("%s has no word to show", p.name)
			}
			assertControl(t, tt, size.w, size.h, label)
		}
	}
}
