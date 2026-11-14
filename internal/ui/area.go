package ui

import (
	"github.com/transmit-bug/remit/internal/ui/shell"
	"github.com/transmit-bug/remit/locales"
)

// areaID is which of the workbench's areas the window shows: what kind of work
// the user came to do. It is the primary navigation and the rail's subject.
type areaID uint8

const (
	areaToday areaID = iota
	areaWrite
	areaRead
	areaRecall
	areaStats
	areaSettings
	// areaPlugin is a page a plugin contributed. It is a rail item of its own,
	// under the plugin pages group.
	areaPlugin
)

// subID is a page within an area. Each area numbers its own pages, so a sub id
// means nothing outside the area that owns it.
type subID uint8

const (
	subNone subID = iota
	// subNotes is the document: a note open, written in or read whole. It is a
	// detail state of the write area rather than one of its pages — a note is
	// entered from the notes list and left by the breadcrumb — so the
	// sub-navigation does not show it.
	subNotes
	// The write area's pages, which its sub-navigation shows.
	subNoteList
	subSearch
	subTags
	subAssets
	subTemplates
	subTree
	// The read area.
	subReadQueue
	subReadArchive
	// The recall area: the queue, the card table, the types, the presets, the
	// history and the broken links.
	subRecallQueue
	subRecallCards
	subRecallTypes
	subRecallPresets
	subRecallHistory
	subRecallRelink
	// The statistics area: the summary, the calendar and the forecast, each a
	// page of its own.
	subStatsOverview
	subStatsHeatmap
	subStatsForecast
	// The settings area, one page until its workflow splits it.

	subSettingsAppearance
	subSettingsReview
	subSettingsBudget
	subSettingsKeys
	subSettingsPlugins
	subSettingsAgent
	subSettingsUpdate
	subSettingsAbout
)

// sessionKind is a full-window session over the stage: the review or the reading
// session, which leave the area and page they were entered from behind them.
// W1 keeps the state and the frame's answer to it; the screens that enter a
// session — the recall queue's Start and the reading queue's Start (S23, S27) —
// arrive with their own slices, and every navigation clears the session.
type sessionKind uint8

const (
	sessionNone sessionKind = iota
	sessionReview
	sessionReading
)

// subsOf is the pages an area has, in the order its sub-navigation shows them.
// An empty list hides the row and gives its height back to the stage. A detail
// state — the document of a note — is not one of them: what is entered from a
// list is left the way it was entered.
func subsOf(area areaID) []subID {
	listed := pagesOf(area)
	out := make([]subID, 0, len(listed))
	for _, p := range listed {
		out = append(out, p.sub)
	}
	return out
}

// defaultSub is the page an area opens on: its first, which is never a place
// entered from another page.
func defaultSub(area areaID) subID {
	if listed := pagesOf(area); len(listed) > 0 {
		return listed[0].sub
	}
	return subNone
}

// hasSub reports whether area shows sub; a request for a page it does not have
// opens the area at its first page instead of nowhere. The document is the write
// area's detail state: a note enters it and the breadcrumb leaves it, so it is
// not a page of the sub-navigation but it is a place the area can be.
func hasSub(area areaID, sub subID) bool {
	_, ok := pageOf(area, sub)
	return ok
}

// documentOpen reports whether the window is on a note: the write area's detail
// state, which hides the area's pages and gives the stage their height.
func (a *App) documentOpen() bool { return a.area == areaWrite && a.sub == subNotes }

// areaName is the area as the plugin contract and the rail read it: a stable,
// lowercase id.
func areaName(area areaID) shell.Area {
	switch area {
	case areaToday:
		return shell.Today
	case areaWrite:
		return shell.Write
	case areaRead:
		return shell.Read
	case areaRecall:
		return shell.Recall
	case areaStats:
		return shell.Stats
	case areaSettings:
		return shell.Settings
	}
	return shell.Area("plugin")
}

// areaLabel is the area's name in this frame's language: the rail's word and
// the breadcrumb's segment.
func areaLabel(msgs locales.Messages, area areaID) string {
	switch area {
	case areaToday:
		return msgs.AreaToday()
	case areaWrite:
		return msgs.AreaWrite()
	case areaRead:
		return msgs.AreaRead()
	case areaRecall:
		return msgs.AreaRecall()
	case areaStats:
		return msgs.AreaStats()
	case areaSettings:
		return msgs.AreaSettings()
	}
	return ""
}

// subLabel is one page's name in this frame's language.
func subLabel(msgs locales.Messages, sub subID) string {
	for _, p := range pageTable {
		if p.sub == sub {
			return p.label(msgs)
		}
	}
	return ""
}

// subName is the page's stable id, which the plugin contract's sub and a link
// spell.
func subName(sub subID) string {
	for _, p := range pageTable {
		if p.sub == sub {
			return p.name
		}
	}
	return ""
}

// subOfName is the page an id names within area, or the area's first page when
// the id names none of its own. The document answers to its own name: a link
// that names a note's page lands in it rather than on the list it was opened
// from.
func subOfName(area areaID, name string) subID {
	if p, ok := pageOfName(area, name); ok {
		return p.sub
	}
	return defaultSub(area)
}

// setArea is the one way the window changes area. Every control that navigates
// calls it or setSub; syncMode derives the plugin contract's mode from what they
// leave, and the page the new area reads is loaded here, never in a view.
func (a *App) setArea(area areaID, sub subID) {
	if !hasSub(area, sub) {
		sub = defaultSub(area)
	}
	if area != a.area || sub != a.sub {
		a.anim.mode++
	}
	a.area, a.sub = area, sub
	a.session = sessionNone
	a.syncMode()
	// A contribution's when expression reads the route, not only the mode: a
	// slot's items ask about the area they sit in and the page inside it, which
	// mode cannot say for the areas that share one. The route is the only writer
	// of that answer.
	a.setContributionArea(string(areaName(area)), subName(sub))
	a.loadCurrent()
}

// setSub switches the area's page, keeping the area — and closing whatever
// session was open over it, since choosing a page is leaving it.
func (a *App) setSub(sub subID) {
	a.goTo(a.area, sub)
}

// place is one page the window was on: the area and its page. It is what back
// returns to, so the page the user came from is remembered rather than derived
// from a mode.
type place struct {
	area areaID
	sub  subID
}

// historyDepth bounds what back remembers: enough for a session's digressions,
// not the whole run of a day.
const historyDepth = 16

// goTo moves the window to a page and remembers the one it left, so that back
// returns there. Asking for the page already on screen is not a move.
func (a *App) goTo(area areaID, sub subID) {
	if !hasSub(area, sub) {
		sub = defaultSub(area)
	}
	if area != a.area || sub != a.sub {
		a.pushPlace(place{a.area, a.sub})
	}
	a.setArea(area, sub)
}

// goBack returns to the page the window was on before this one, and does nothing
// when there is none: back is a request, not a command.
func (a *App) goBack() {
	if len(a.back) == 0 {
		return
	}
	last := a.back[len(a.back)-1]
	a.back = a.back[:len(a.back)-1]
	a.setArea(last.area, last.sub)
}

// land moves the window to a page with nothing behind it: a link from outside
// the window, the tray, a deep link. Such a request is a place to be, not a step
// away from somewhere, so back has nowhere to go.
func (a *App) land(area areaID, sub subID) {
	a.back = a.back[:0]
	a.setArea(area, sub)
}

// pushPlace notes where the window was, for back. A page a session was entered
// from is noted by enterSession, so leaving a session returns to it.
func (a *App) pushPlace(p place) {
	if len(a.back) > 0 && a.back[len(a.back)-1] == p {
		return
	}
	a.back = append(a.back, p)
	if len(a.back) > historyDepth {
		a.back = a.back[1:]
	}
}

// enterSession opens a session over the page on screen and remembers that page,
// so that the session's own way out returns there.
func (a *App) enterSession(kind sessionKind) {
	a.pushPlace(place{a.area, a.sub})
	a.session = kind
	a.syncMode()
	a.loadCurrent()
}

// escape is the one Escape handler below the overlays: a session leaves for the
// page it was entered from, and any other page goes back when there is somewhere
// to go. A page entered from outside the window has no history, so a detail page
// falls back to the list its area opens on.
func (a *App) escape() {
	if len(a.back) > 0 {
		a.goBack()
		return
	}
	if a.sub != defaultSub(a.area) {
		a.setArea(a.area, defaultSub(a.area))
	}
}

// syncMode derives a.mode from the area, the page and the session; it is the
// only writer, so the value cannot drift from what is on screen. A plugin's when
// expression reads it through modeName, whose values are the ones the contract
// has always had: notes, review, browse, stats, settings, and empty where no
// legacy screen answers — Today, the reading area (modeReading, as it was) and
// a plugin page.
func (a *App) syncMode() {
	switch a.session {
	case sessionReview:
		a.mode = modeReview
		return
	case sessionReading:
		a.mode = modeReading
		return
	}
	switch a.area {
	case areaWrite:
		if a.sub == subSearch {
			// The search page is the browse screen's, as it has always been:
			// the card search and the note search are one behaviour.
			a.mode = modeBrowse
			return
		}
		a.mode = modeNotes
	case areaRead:
		// Reading was never one of the five screens a when expression names:
		// modeName reads it empty, as it always has.
		a.mode = modeReading
	case areaRecall:
		if a.sub == subRecallCards {
			a.mode = modeBrowse
			return
		}
		a.mode = modeReview
	case areaStats:
		a.mode = modeStats
	case areaSettings:
		a.mode = modeSettings
	case areaPlugin:
		a.mode = modePlugin
	default:
		a.mode = modeNone
	}
}

// takingOver reports whether the screen takes the window whole: a session hides
// the rail, the top bar, the sub-navigation and the status line, so that the card
// or the note is all the user sees.
func (a *App) takingOver() bool {
	return a.lib != nil && !a.start && a.openErr == nil && a.session != sessionNone
}
