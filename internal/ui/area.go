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
	// The write area: the notebook, then the pages beside it.
	subNotes
	subSearch
	// The read area.
	subReadQueue
	subReadArchive
	// The recall area.
	subRecallQueue
	subRecallCards
	// The statistics and settings areas, each one page until its workflow
	// splits it.
	subStatsOverview
	subSettingsAppearance
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
// An empty list hides the row and gives its height back to the stage.
func subsOf(area areaID) []subID {
	switch area {
	case areaWrite:
		return []subID{subNotes, subSearch}
	case areaRead:
		return []subID{subReadQueue, subReadArchive}
	case areaRecall:
		return []subID{subRecallQueue, subRecallCards}
	case areaStats:
		return []subID{subStatsOverview}
	case areaSettings:
		return []subID{subSettingsAppearance}
	}
	return nil
}

// defaultSub is the page an area opens on.
func defaultSub(area areaID) subID {
	if subs := subsOf(area); len(subs) > 0 {
		return subs[0]
	}
	return subNone
}

// hasSub reports whether area shows sub; a request for a page it does not have
// opens the area at its first page instead of nowhere.
func hasSub(area areaID, sub subID) bool {
	for _, candidate := range subsOf(area) {
		if candidate == sub {
			return true
		}
	}
	return false
}

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
	switch sub {
	case subNotes:
		return msgs.ModeNotes()
	case subSearch:
		return msgs.SearchPage()
	case subReadQueue, subRecallQueue:
		return msgs.QueuePage()
	case subReadArchive:
		return msgs.ArchivePage()
	case subRecallCards:
		return msgs.CardsPage()
	case subStatsOverview:
		return msgs.OverviewPage()
	case subSettingsAppearance:
		return msgs.AppearancePage()
	}
	return ""
}

// subName is the page's stable id, which the plugin contract's sub reads.
func subName(sub subID) string {
	switch sub {
	case subNotes:
		return "notes"
	case subSearch:
		return "search"
	case subReadQueue:
		return "queue"
	case subReadArchive:
		return "archive"
	case subRecallQueue:
		return "queue"
	case subRecallCards:
		return "cards"
	case subStatsOverview:
		return "overview"
	case subSettingsAppearance:
		return "appearance"
	}
	return ""
}

// subOfName is the page an id names within area, or the area's first page when
// the id names none of its own.
func subOfName(area areaID, name string) subID {
	for _, candidate := range subsOf(area) {
		if subName(candidate) == name {
			return candidate
		}
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
	a.loadCurrent()
}

// setSub switches the area's page, keeping the area — and closing whatever
// session was open over it, since choosing a page is leaving it.
func (a *App) setSub(sub subID) {
	a.setArea(a.area, sub)
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

// areaOfMode is where an action phrased in modes lands: the screens' callbacks
// and the palette still speak of the five screens, and each one is a page of an
// area. It keeps the mapping in one place so that the write path stays setArea.
func areaOfMode(m mode) (areaID, subID) {
	switch m {
	case modeNone:
		return areaToday, subNone
	case modeReview:
		return areaRecall, subRecallQueue
	case modeReading:
		return areaRead, subReadQueue
	case modeBrowse:
		return areaRecall, subRecallCards
	case modeStats:
		return areaStats, subStatsOverview
	case modeSettings:
		return areaSettings, subSettingsAppearance
	}
	return areaWrite, subNotes
}

// setMode is the bridge for the call sites that still speak in modes — the
// screens' callbacks, the palette and the settings page's return. It writes
// through setArea, so the area remains the only navigation state.
func (a *App) setMode(m mode) {
	area, sub := areaOfMode(m)
	a.setArea(area, sub)
}

// takingOver reports whether the screen takes the window whole: a session hides
// the rail, the top bar, the sub-navigation and the status line, so that the card
// or the note is all the user sees.
func (a *App) takingOver() bool {
	return a.lib != nil && !a.start && a.openErr == nil && a.session != sessionNone
}
