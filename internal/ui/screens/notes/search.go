package notes

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// HitKind is what a search result is: a note, or a card a note's paragraph
// renders.
type HitKind uint8

const (
	// HitNote is a note the query matched, by its title or by a paragraph.
	HitNote HitKind = iota
	// HitCard is a card whose prompt or answer the query matched.
	HitCard
)

// Hit is one search result: what matched, and the line around the match, split
// where the match is so that the view marks it without parsing it back.
type Hit struct {
	Kind HitKind
	// ID is the note, or the card.
	ID core.ID
	// Note is the note the result belongs to: the note itself, or the note the
	// card renders from.
	Note core.ID
	// Title names the note the result belongs to.
	Title string
	// Before, Match and After are the hit's line around the match: the words
	// either side of it, and the matched text itself.
	Before, Match, After string
}

// SearchView draws the search page (S06): the notes and the cards the query
// matched, in two groups, each row opening what it names.
func SearchView(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	hits, cards, total := searchResults(d)
	query := s.SearchQuery

	reload := false
	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.ListWidth).Grow(1).Gap(tk.SpaceL).Children(func() {
			ui.Column(c).FillWidth().Gap(tk.SpaceXS).Children(func() {
				ui.Text(c, msgs.WriteSearch()).FontSize(tk.TypeH1).Bold()
				subtitle := msgs.WriteSearchScope()
				if query != "" {
					subtitle = msgs.WriteSearchCount(total, query)
				}
				ui.Text(c, subtitle).FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
			})
			field := ui.SearchField(c, &s.SearchQuery).
				Placeholder(msgs.WriteSearchPlaceholder()).
				Label(msgs.WriteSearchPlaceholder()).Bind(&s.search)
			if field.Changed() {
				reload = true
			}
			switch {
			case query == "":
				// Nothing typed yet: the page says what it looks at rather than
				// that nothing was found.
				kit.Empty(c, "search", msgs.WriteSearchScope(), nil)
			case total == 0:
				hitGroup(c, tk, msgs.WriteSearchNotes(), nil, &s.searchNotes, d)
				hitGroup(c, tk, msgs.WriteSearchCards(), nil, &s.searchCards, d)
				ui.Text(c, msgs.WriteSearchNone()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			default:
				hitGroup(c, tk, msgs.WriteSearchNotes(), hits, &s.searchNotes, d)
				hitGroup(c, tk, msgs.WriteSearchCards(), cards, &s.searchCards, d)
			}
		})
	})
	if reload && d.Reload != nil {
		d.Reload()
	}
}

// searchResults is the app's answer; a screen without one behind it found
// nothing.
func searchResults(d Deps) (hits, cards []Hit, total int) {
	if d.SearchResults == nil {
		return nil, nil, 0
	}
	return d.SearchResults()
}

// hitGroup is one group of results — the notes or the cards — with how many it
// holds. A group with none still shows its heading and its zero: the two zeros
// are what tell a reader the search ran and looked at both.
func hitGroup(c *ui.Context, tk design.Tokens, title string, hits []Hit, list *ui.ListState, d Deps) {
	ui.Column(c).FillWidth().Gap(tk.SpaceS).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
			ui.Text(c, title).FontSize(tk.TypeUI).Bold().TextColor(tk.TextMuted)
			ui.Badge(c, itoa(len(hits)))
		})
		if len(hits) == 0 {
			return
		}
		list.Key = func(row int) any { return hits[row].ID }
		list.Label = func(row int) string { return hits[row].Title }
		ui.List(c, list, len(hits), func(row int) {
			hitRow(c, tk, title, hits[row], d)
		}).MaxHeight(tk.Hit * 8).Label(title)
	})
}

// hitRow is one result: the note it belongs to, the line that matched with the
// match marked, and whether it was the note or one of its cards.
func hitRow(c *ui.Context, tk design.Tokens, group string, hit Hit, d Deps) {
	// A card hit and the note it belongs to can be called the same thing: the
	// name a screen reader reads says which group the row is in.
	name := hit.Title
	if hit.Kind == HitCard {
		name = group + " · " + hit.Title
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
		Padding(tk.SpaceS, tk.SpaceM).Radius(tk.RadiusControl).Children(func() {
		open := ui.Button(c, "").Label(name).Grow(1).MinWidth(0).
			MinHeight(tk.Hit).Padding(0).Radius(0).
			Background(ui.Color{}).Border(0, tk.Border)
		if open.Clicked() {
			openHit(hit, d)
		}
		open.Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Start).
				Gap(tk.SpaceXS).Children(func() {
				ui.Text(c, hit.Title).FontSize(tk.TypeUI).SingleLine()
				snippet(c, tk, hit)
			})
		})
	})
}

// snippet is the line a hit matched on, with the match marked as a search
// result is: the text either side stays in the quiet ink.
func snippet(c *ui.Context, tk design.Tokens, hit Hit) ui.Element {
	if hit.Before == "" && hit.Match == "" && hit.After == "" {
		return ui.Element{}
	}
	quiet := ui.Span{Size: tk.TypeLabel, Color: tk.TextMuted}
	before, after := quiet, quiet
	before.Text, after.Text = hit.Before, hit.After
	match := ui.Span{
		Text:       hit.Match,
		Size:       tk.TypeLabel,
		Color:      tk.Text,
		Weight:     600,
		Background: tk.Accent.Alpha(0.16),
		Radius:     tk.RadiusBar,
	}
	return ui.RichText(c, before, match, after).SingleLine()
}

// openHit shows what a result names: the note itself, or the card.
func openHit(hit Hit, d Deps) {
	if hit.Kind == HitCard {
		if d.OpenCard != nil {
			d.OpenCard(hit.ID)
		}
		return
	}
	if d.OpenNote != nil {
		d.OpenNote(hit.Note)
	}
}

// itoa is a number a badge shows.
func itoa(n int) string { return strconv.Itoa(n) }
