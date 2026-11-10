// Package shell is the window's frame: the navigation rail on the left, the
// top bar, the area's sub-navigation and the status line, with the stage for
// whatever page is up between them.
//
// It is a leaf. It reads State and Deps, draws, and reports what the user chose
// through callbacks; it never reaches the store, a screen or package ui. Every
// part of the frame is MyGo's own: ui.Sidebar for the rail, ui.Toolbar for the
// top bar, ui.TabsBase for the sub-navigation and ui.Breadcrumbs for the path —
// each carries its keyboard, focus, disabled and assistive-technology behaviour,
// which a hand-built row would have cost.
//
// One deliberate departure from the spec's rail vocabulary: the area chosen
// keeps ui.Sidebar's own choice — one tab stop, one ring, the arrows — while its
// face is the mark's wash, as the prototype draws it, rather than the widget's
// solid accent: a bar of accent down the rail would out-shout the page. The rail
// and the top bar still read as one surface, one step above the stage.
package shell

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
)

// Area is a rail item's stable id: one of the six areas, or a plugin page's
// "plugin/<name>/<page>" as the app names it.
type Area string

// The six areas of the workbench, in the order the rail shows them.
const (
	Today    Area = "today"
	Write    Area = "write"
	Read     Area = "read"
	Recall   Area = "recall"
	Stats    Area = "stats"
	Settings Area = "settings"
)

// Item is one entry on the rail: an area, or a page a plugin contributed.
type Item struct {
	ID    Area
	Label string
	// Icon is a name design.Icon resolves; an empty one draws no icon.
	Icon string
	// Count is the badge's number. Zero draws no badge: an empty queue is
	// quiet, not a zero to read.
	Count int
}

// Sub is one page of the area on screen, as the sub-navigation shows it.
type Sub struct {
	ID    string
	Label string
	// Count is the badge's number; zero draws none.
	Count int
}

// State is the frame's own memory across frames: what the user folded, which
// survives the page behind it changing.
type State struct {
	// Folded is the user's choice: the rail shows icons only. At
	// design.BreakpointNarrow and below it folds whatever the choice says.
	Folded bool
}

// Deps is everything the frame draws and every action it reports. The app
// resolves the catalog and the counts; the frame reads nothing else.
type Deps struct {
	// Title names the window in the bars' accessible labels.
	Title string

	// Areas are the six areas in rail order.
	Areas []Item
	// Pages are the plugin pages, the rail's second group; empty draws no
	// group and no heading for one.
	Pages []Item
	// Current is the item chosen: an area's id, or a page's.
	Current Area
	// OnSelect is told which item the user chose.
	OnSelect func(Area)

	// Subnav are the current area's pages; empty hides the row (Today has
	// none), and the stage gets the height back.
	Subnav     []Sub
	CurrentSub string
	OnSub      func(string)

	// Crumbs is the path to where the user is, the last one being the page
	// they are on. OnCrumb is told which earlier one was chosen.
	Crumbs  []string
	OnCrumb func(int)

	// Actions builds the current page's own controls in the top bar. Nil
	// leaves the bar to the frame's controls.
	Actions func(c *ui.Context, tk design.Tokens)
	// OnPalette opens the command palette; nil draws no search button.
	OnPalette func()
	// Overflow builds the ⋯ menu; nil draws no overflow button.
	Overflow func(menu *ui.Menu)

	// Status is the stage's status line, one entry per segment. Empty hides
	// the line, which is every page but a document's.
	Status []string
}

// View draws the frame around stage.
func View(c *ui.Context, st *State, d Deps, stage func()) {
	tk := design.NewTokens(c.Theme())
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(tk.Background).Children(func() {
		rail(c, tk, st, d)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			topbar(c, tk, st, d)
			subnav(c, tk, d)
			stage()
			statusbar(c, tk, d)
		})
	})
}

// collapsed reports whether the rail is showing icons only: the user folded it,
// or the window is too narrow for the labels.
func collapsed(c *ui.Context, st *State) bool {
	return st.Folded || narrow(c)
}

// RailWidth is the rail's width in a window this wide: the column the labels
// need, or the icon strip when the user folded it or the window is at
// BreakpointNarrow or below. It is the room every page subtracts from the window
// to know the stage it was given.
func RailWidth(w float32, folded bool) float32 {
	if folded || w <= design.BreakpointNarrow {
		return design.RailCollapsed
	}
	return design.RailWidth
}

// narrow reports whether the window is at design.BreakpointNarrow or below. The
// rail is the icon strip there whatever the user chose, so its fold controls
// wait for a window wide enough for them to change anything: a control that
// cannot act is not drawn. Widening the rail over the stage at this width is a
// later slice's (the same overlay the document panel uses).
func narrow(c *ui.Context) bool {
	w, _ := c.Size()
	return w <= design.BreakpointNarrow
}
