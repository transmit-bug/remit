// Package pluginpage draws a page a plugin declared: a tree of layout nodes and
// semantic leaves that the host renders with its own widgets, tokens and type
// scale. The plugin names what it wants to say and nothing about how it looks
// (docs/specs/plugins/ui.md §5); every spacing, colour, focus and reflow here is
// the host's.
//
// The page reads State and never calls a plugin: the app loads a page's tree
// before the frame that draws it, and hands the error here when the load failed.
package pluginpage

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/transmit-bug/remit/internal/ext/contrib"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

const (
	// narrowWidth is the content width below which the page lays itself out for
	// a narrow window: a grid collapses to one column and a split stacks, so
	// that 720×520 never scrolls sideways.
	narrowWidth float32 = 900
	// chartHeight is one plot's height, as the statistics page draws them.
	chartHeight float32 = 150
)

// State is the page on screen: what the app's last load left, and the choices
// the reader made in it. The app fills the first three; the rest lives here so
// that a tab, a tree branch and a parsed markdown document survive a frame.
type State struct {
	// Title is the page's own message, "%page.key%" or a literal.
	Title string
	// Tree is the node tree the page's data command answered.
	Tree contrib.Node
	// Err is why the page has no tree, "" when it has one. A page with an
	// error shows it and a retry instead of a blank.
	Err string
	// Loading is a load in flight: the page shows a skeleton and no error.
	Loading bool

	// Highlight is the board node the window was told to locate, "" for none,
	// and HighlightRun is how often that moved: a new run replays the pointer
	// once, and a second trip to the same node plays it again.
	Highlight    string
	HighlightRun uint64

	tabs map[string]*int
	open map[string]*bool
	md   map[string]*markdown.Options
}

// Deps is what the page asks of the app: the plugin's own strings in this
// frame's language, the command an action runs, and a retry when the data did
// not arrive. Everything else is the cache the app loaded before the frame.
type Deps struct {
	// Message resolves one of the plugin's messages in the frame's language.
	Message func(c *ui.Context, raw string) string
	// Run runs a command the page's actions name. It is a handler: the app
	// starts the command outside the frame.
	Run func(c *ui.Context, command string)
	// Retry asks for the page's data again: the action of a failed page, and
	// the refresh a reader asks for at any other time.
	Retry func(c *ui.Context)
	// Header is the page's own actions — the page.header slot's items, as the
	// app read them from its cache. The host's refresh follows them.
	Header []Action
	// Node answers what a board node says now: the note's title, the block's
	// first line and how many cards the block feeds, false when the note no
	// longer holds the block, which leaves the page's own words standing.
	Node func(note, block string) (NodeInfo, bool)
	// OpenBlock shows the note a board node names, read at its block.
	OpenBlock func(c *ui.Context, note, block string)
}

// Action is one control a page's header offers: the slot item it came from, in
// the frame's language, and what it runs.
type Action struct {
	Label string
	Run   func(c *ui.Context)
}

// NodeInfo is a board node as the library holds it now.
type NodeInfo struct {
	Title string
	Text  string
	Cards int
}

// View draws the page: its title, then the tree, or why there is none.
func View(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	w, _ := c.Size()
	narrow := w < narrowWidth

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(tk.PaperWidth).Gap(tk.SpaceL).Label(d.message(c, s.Title)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(tk.SpaceS).
					Label(msgs.PluginPageActions()).Children(func() {
					ui.Text(c, d.message(c, s.Title)).FontSize(tk.TypeH1).Bold()
					ui.Spacer(c)
					// The page's own actions, then the host's refresh: a page's data
					// is asked for on entering, on an explicit refresh and on an
					// event — never every frame — so the refresh is a control the
					// reader can press (ui.md §9).
					for _, action := range d.Header {
						run := action.Run
						ui.Button(c, action.Label).MinHeight(tk.Control).
							OnClick(func() { run(c) })
					}
					ui.Button(c, "").Label(msgs.PluginPageRefresh()).Tooltip(msgs.PluginPageRefresh()).
						MinHeight(tk.Control).
						Children(func() { tk.Icon(c, "undo-2", design.IconSmall) }).
						OnClick(func() { d.retry(c) })
				})
				switch {
				case s.Err != "":
					failed(c, s, d, msgs, tk)
				case s.Tree.Kind == "":
					loading(c, msgs, tk)
				default:
					draw(c, s, d, s.Tree, "page", tk, narrow)
				}
			})
		})
	})
}

// failed is a page whose data did not arrive: what went wrong, and the one
// action that asks again. It is never a blank page.
func failed(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	ui.Row(c).Gap(tk.SpaceM).AlignItems(ui.Center).Children(func() {
		ui.Text(c, msgs.PluginPageFailed(s.Err)).FontSize(tk.TypeUI).TextColor(tk.Danger)
		ui.Button(c, msgs.PluginPageRetry()).MinHeight(tk.Control).
			OnClick(func() { d.retry(c) })
	})
}

// loading is the page before its first tree: the title is up and the content
// says it is on its way.
func loading(c *ui.Context, msgs locales.Messages, tk design.Tokens) {
	ui.Row(c).Gap(tk.SpaceS).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c)
		ui.Text(c, msgs.PluginPageLoading()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
	})
}

// draw builds one node. path is where it sits in the tree, which is the key its
// own state — a chosen tab, an open branch — is kept by.
func draw(c *ui.Context, s *State, d Deps, n contrib.Node, path string, tk design.Tokens, narrow bool) {
	gap := gapOf(tk, n.Gap)
	switch n.Kind {
	case contrib.KindPage:
		ui.Column(c).FillWidth().Gap(tk.SpaceL).Children(func() {
			drawChildren(c, s, d, n.Children, path, tk, narrow)
		})
	case contrib.KindSection:
		ui.Column(c).FillWidth().Gap(gap).Children(func() {
			ui.Column(c).Gap(tk.SpaceXS).Children(func() {
				ui.Text(c, d.message(c, n.Title)).FontSize(tk.TypeH1).Bold()
				if n.Description != "" {
					ui.Text(c, d.message(c, n.Description)).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
				}
			})
			drawChildren(c, s, d, n.Children, path, tk, narrow)
		})
	case contrib.KindStack:
		if n.Direction == contrib.DirectionRow && !narrow {
			// A row of things that do not fit wraps instead of scrolling
			// sideways, which the contract forbids at the smallest window.
			ui.Row(c).FillWidth().Wrap().Gap(gap).AlignItems(ui.Start).Children(func() {
				drawChildren(c, s, d, n.Children, path, tk, narrow)
			})
			return
		}
		ui.Column(c).FillWidth().Gap(func() float32 {
			if n.Direction == contrib.DirectionRow {
				return tk.SpaceS
			}
			return gap
		}()).Children(func() {
			drawChildren(c, s, d, n.Children, path, tk, narrow)
		})
	case contrib.KindGrid:
		columns := n.Columns
		if narrow {
			columns = 1
		}
		if isBoard(n.Children) {
			// A grid whose cards each name a block is a board: the host draws the
			// places the page listed, and goes there itself (§6.4).
			drawBoard(c, s, d, n.Children, columns, tk)
			return
		}
		ui.Grid(c).FillWidth().Columns(columns).GapX(gap).GapY(gap).Children(func() {
			drawChildren(c, s, d, n.Children, path, tk, narrow)
		})
	case contrib.KindSplit:
		if narrow {
			ui.Column(c).FillWidth().Gap(tk.SpaceL).Children(func() {
				draw(c, s, d, n.Children[0], path+".0", tk, narrow)
				draw(c, s, d, n.Children[1], path+".1", tk, narrow)
			})
			return
		}
		ratio := n.Ratio
		if ratio < 1 {
			ratio = 1
		}
		ui.Grid(c).FillWidth().ColumnTracks(ui.Fr(1), ui.Fr(float32(ratio))).GapX(tk.SpaceL).Children(func() {
			draw(c, s, d, n.Children[0], path+".0", tk, narrow)
			draw(c, s, d, n.Children[1], path+".1", tk, narrow)
		})
	case contrib.KindTabs:
		chosen := s.tab(path, len(n.Tabs))
		labels := make([]string, len(n.Tabs))
		for i, tab := range n.Tabs {
			labels[i] = d.message(c, tab.Label)
		}
		ui.Tabs(c, chosen, labels...).Label(d.message(c, n.Title))
		if *chosen >= 0 && *chosen < len(n.Tabs) {
			ui.Column(c).FillWidth().Gap(tk.SpaceM).Children(func() {
				drawChildren(c, s, d, n.Tabs[*chosen].Children, path, tk, narrow)
			})
		}
	case contrib.KindCard:
		kit.Card(c, func() {
			ui.Column(c).FillWidth().Padding(tk.SpaceL).Gap(tk.SpaceS).Children(func() {
				if n.Title != "" {
					ui.Text(c, d.message(c, n.Title)).FontSize(tk.TypeH1).Bold()
				}
				drawChildren(c, s, d, n.Children, path, tk, narrow)
			})
		}).FillWidth()
	case contrib.KindMarkdown:
		markdown.View(c, d.message(c, n.Text), s.markdown(path))
	case contrib.KindText:
		ui.Text(c, d.message(c, n.Text)).FontSize(tk.TypeUI).TextColor(toneColor(tk, n.Tone))
	case contrib.KindStat:
		kit.Stat(c, d.message(c, n.Value), d.message(c, n.Label))
	case contrib.KindTable:
		drawTable(c, d, n, tk)
	case contrib.KindList:
		if len(n.Items) == 0 {
			ui.Text(c, d.message(c, n.Text)).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
			return
		}
		ui.List(c, nil, len(n.Items), func(i int) {
			ui.Row(c).FillWidth().Gap(tk.SpaceS).AlignItems(ui.Center).Padding(tk.SpaceXS, 0).Children(func() {
				ui.Text(c, d.message(c, n.Items[i].Label)).FontSize(tk.TypeUI)
				ui.Spacer(c)
				if value := d.message(c, n.Items[i].Value); value != "" {
					ui.Text(c, value).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
				}
			})
		}).Label(d.message(c, n.Title))
	case contrib.KindTree:
		ui.Tree(c, func() {
			drawTree(c, s, d, n.Items, path, tk)
		}).Label(d.message(c, n.Title))
	case contrib.KindForm:
		// A page's form is what the plugin says its settings are, read-only:
		// editing them is the settings page's host form, where the host owns
		// the write (docs/specs/plugins/ui.md §6).
		ui.Form(c, func() {
			for _, field := range n.Fields {
				ui.Field(c, d.message(c, field.Label), func() {
					ui.Text(c, d.message(c, field.Value)).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
				})
			}
		})
	case contrib.KindChart:
		drawChart(c, d, n, tk)
	case contrib.KindCallout:
		ui.Column(c).FillWidth().Gap(tk.SpaceXS).Padding(tk.SpaceM).
			Background(toneFill(tk, n.Tone)).Radius(tk.RadiusControl).
			Children(func() {
				if n.Title != "" {
					ui.Text(c, d.message(c, n.Title)).Bold()
				}
				ui.Text(c, d.message(c, n.Text)).FontSize(tk.TypeUI)
			})
	case contrib.KindActions:
		ui.Row(c).Wrap().Gap(tk.SpaceS).Children(func() {
			for _, action := range n.Items {
				ui.Button(c, d.message(c, action.Label)).MinHeight(tk.Control).
					OnClick(func() { d.run(c, action.Command) })
			}
		})
	default:
		unknown(c, n, tk)
	}
}

// unknown is a node kind this host does not draw: it keeps the kind's name in
// place, so the page says what it could not show and renders the rest.
func unknown(c *ui.Context, n contrib.Node, tk design.Tokens) {
	ui.Text(c, locales.In(c.Locale()).PluginPageUnknownNode(string(n.Kind))).
		FontSize(tk.TypeUI).TextColor(tk.TextMuted).
		Padding(tk.SpaceS).Radius(tk.RadiusControl).
		Border(tk.Hairline, tk.Border)
}

func drawChildren(c *ui.Context, s *State, d Deps, children []contrib.Node, path string, tk design.Tokens, narrow bool) {
	for i, child := range children {
		draw(c, s, d, child, path+"."+itoa(i), tk, narrow)
	}
}

// drawTable is a read-only table: the plugin names columns and rows, and the
// host's table draws a header, wraps a narrow window's cells rather than
// scrolling sideways, and builds only the rows in view.
func drawTable(c *ui.Context, d Deps, n contrib.Node, tk design.Tokens) {
	columns := make([]ui.TableColumn, len(n.Headings))
	for i, heading := range n.Headings {
		columns[i] = ui.TableColumn{Title: d.message(c, heading)}
	}
	ui.Table(c, nil, columns, len(n.Rows), func(row, col int) {
		ui.Text(c, d.message(c, n.Rows[row][col])).FontSize(tk.TypeUI).SingleLine()
	}).Label(d.message(c, n.Title))
}

// drawTree is a tree leaf: its items are rows, and the ones with children open
// and close, kept by their path so a frame rebuilds them as the reader left
// them.
func drawTree(c *ui.Context, s *State, d Deps, items []contrib.Item, path string, tk design.Tokens) {
	for i, item := range items {
		key := path + "." + itoa(i)
		if len(item.Items) > 0 {
			open := s.branch(key)
			ui.TreeItem(c, d.message(c, item.Label), open, func() {
				drawTree(c, s, d, item.Items, key, tk)
			}).FontSize(tk.TypeUI)
			continue
		}
		ui.TreeItem(c, d.message(c, item.Label), nil, nil).FontSize(tk.TypeUI)
	}
}

// drawChart draws one bar per point on the host's band scale, so a plugin names
// values and the chart library decides where a pixel is.
func drawChart(c *ui.Context, d Deps, n contrib.Node, tk design.Tokens) {
	if len(n.Points) == 0 {
		return
	}
	labels := make([]string, len(n.Points))
	highest := 1.0
	for i, point := range n.Points {
		labels[i] = d.message(c, point.Label)
		highest = max(highest, point.Value)
	}
	x := chart.NewBand(labels...).Padding(0.3, 0.12)
	y := chart.NewLinear(0, highest)
	chart.Plot(c, chart.Axes{Left: y.Ticks(4), Bottom: x.Ticks()}, func(area ui.Element) {
		for i, point := range n.Points {
			chart.Rect(c, x.Band(i), y.Span(0, point.Value)).
				Background(tk.Accent).Radius(tk.RadiusBar).
				Tooltip(labels[i])
		}
	}).Height(chartHeight).Gap(tk.SpaceS).Label(d.message(c, n.Title))
}

// tab answers the tab chosen at path, and makes it 0 the first time. A choice
// outside the tabs the page now has — a reload with fewer — goes back to the
// first.
func (s *State) tab(path string, count int) *int {
	if s.tabs == nil {
		s.tabs = map[string]*int{}
	}
	chosen := s.tabs[path]
	if chosen == nil {
		first := 0
		s.tabs[path] = &first
		return &first
	}
	if *chosen < 0 || *chosen >= count {
		*chosen = 0
	}
	return chosen
}

// branch answers one tree item's open state, and makes it closed the first time.
func (s *State) branch(path string) *bool {
	if s.open == nil {
		s.open = map[string]*bool{}
	}
	if s.open[path] == nil {
		closed := false
		s.open[path] = &closed
	}
	return s.open[path]
}

// markdown answers the parse cache for one markdown leaf: each leaf keeps the
// document it made, so a frame reparses only what the plugin changed.
func (s *State) markdown(path string) *markdown.Options {
	if s.md == nil {
		s.md = map[string]*markdown.Options{}
	}
	if s.md[path] == nil {
		s.md[path] = &markdown.Options{}
	}
	return s.md[path]
}

func (d Deps) message(c *ui.Context, raw string) string {
	if d.Message == nil || raw == "" {
		return raw
	}
	return d.Message(c, raw)
}

func (d Deps) run(c *ui.Context, command string) {
	if d.Run != nil {
		d.Run(c, command)
	}
}

func (d Deps) retry(c *ui.Context) {
	if d.Retry != nil {
		d.Retry(c)
	}
}

// gapOf is the spacing step a node names, the host's default when it names
// none.
func gapOf(tk design.Tokens, gap contrib.Gap) float32 {
	switch gap {
	case contrib.GapXS:
		return tk.SpaceXS
	case contrib.GapS:
		return tk.SpaceS
	case contrib.GapL:
		return tk.SpaceL
	case contrib.GapM:
		return tk.SpaceM
	}
	return tk.SpaceM
}

// toneColor is a text node's tone, which is a meaning rather than a colour.
func toneColor(tk design.Tokens, tone contrib.Tone) ui.Color {
	switch tone {
	case contrib.ToneDanger:
		return tk.Danger
	case contrib.ToneWarn:
		return tk.Warning
	case contrib.ToneSuccess:
		return tk.Success
	case contrib.ToneInfo:
		return tk.Accent
	}
	return tk.Text
}

// toneFill is a callout's face: the tone mixed into the card, so the message is
// read as its meaning in either theme.
func toneFill(tk design.Tokens, tone contrib.Tone) ui.Color {
	switch tone {
	case contrib.ToneDanger, contrib.ToneWarn, contrib.ToneSuccess, contrib.ToneInfo:
		return tk.RatingFill(toneColor(tk, tone))
	}
	return tk.Sunken
}

// itoa is the index's digits, for a node's path key.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits [12]byte
	pos := len(digits)
	for i > 0 {
		pos--
		digits[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(digits[pos:])
}
