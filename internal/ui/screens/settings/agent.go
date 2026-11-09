package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/locales"
)

// AgentStatus is what the agent API says about itself: where it listens, and
// the file that holds the bearer token. Both are the files cmd writes beside
// the token, so a client finds the API without a fixed port.
type AgentStatus struct {
	Address   string // host:port, empty while nothing is served
	TokenPath string // the file holding the token, empty when unknown
}

// AgentControls is what the settings pages need from whoever serves the agent
// API (cmd): start or stop the listener now, rotate the token, and say where it
// listens. It is an interface so that a test drives all three without binding a
// port. A nil AgentControls leaves the page saying so.
type AgentControls interface {
	// SetEnabled starts the listener, or stops it, and reports why it could
	// not. Starting one that already runs changes nothing.
	SetEnabled(on bool) error
	// RotateToken writes a fresh token and restarts the listener with it, so
	// the old token stops working at once.
	RotateToken() error
	// Status is where it listens and which file holds the token now.
	Status() AgentStatus
}

// The route table's measures: its height bounds a table nested in the page's
// scroll, and the two narrow columns hold the method and the path at the mono
// size, leaving the rest of the room to the description.
const (
	routeTableHeight float32 = 300
	routeMethodWidth float32 = 90
	routePathWidth   float32 = 240
)

// agentRoute is one row of the API's route table: the methods it answers and
// what it is for. It mirrors internal/agentapi's own route list, which is
// unexported — a route added there has to be added here too, and the two are
// the API's contract rather than anything this page computes.
type agentRoute struct {
	methods string
	path    string
	what    string
}

// agentRoutes is the loopback API as the page states it: reads open, writes
// gated by the token, no deletes.
func agentRoutes(msgs locales.Messages) []agentRoute {
	return []agentRoute{
		{"GET", "/v1/schema", msgs.SettingsRouteSchema()},
		{"GET", "/v1/library", msgs.SettingsRouteLibrary()},
		{"GET · POST", "/v1/notes", msgs.SettingsRouteNotes()},
		{"GET", "/v1/notes/{id}", msgs.SettingsRouteNote()},
		{"GET", "/v1/notes/{id}/backlinks", msgs.SettingsRouteBacklinks()},
		{"GET · POST", "/v1/cards", msgs.SettingsRouteCards()},
		{"GET", "/v1/cards/{id}", msgs.SettingsRouteCard()},
		{"PATCH", "/v1/cards/{id}", msgs.SettingsRouteCardPatch()},
		{"POST", "/v1/cards/{id}/reviews", msgs.SettingsRouteReview()},
		{"GET", "/v1/queue", msgs.SettingsRouteQueue()},
		{"GET", "/v1/search", msgs.SettingsRouteSearch()},
		{"GET", "/v1/tags", msgs.SettingsRouteTags()},
		{"GET", "/v1/types · /v1/types/{id}", msgs.SettingsRouteTypes()},
		{"GET", "/v1/stats/{summary,forecast,retention}", msgs.SettingsRouteStats()},
		{"GET", "/v1/events", msgs.SettingsRouteEvents()},
		{"GET · POST", "/v1/reading · /v1/reading/{id}", msgs.SettingsRouteReading()},
	}
}

// agentPage is the loopback agent API: whether it serves now, where it listens,
// the token a write needs — shown once when it is minted — and the routes it
// answers, so a client's reach is stated in the app rather than in a document.
func agentPage(c *ui.Context, s *State, d Deps, msgs locales.Messages, tk design.Tokens) {
	var toggled, rotated bool

	ui.Fieldset(c, msgs.SettingsAgent(), func() {
		ui.Field(c, msgs.SettingsAgentServe(), func() {
			ui.Column(c).Gap(tk.SpaceS).Children(func() {
				toggled = ui.Switch(c, &s.AgentOn).Changed()
				ui.Column(c).Gap(tk.SpaceXS).Children(func() {
					if s.Agent.Address == "" {
						ui.Text(c, msgs.SettingsAgentStopped()).FontSize(tk.TypeUI).TextColor(tk.TextMuted)
					} else {
						ui.Text(c, msgs.SettingsAgentAddress(s.Agent.Address)).
							Font("monospace").FontSize(tk.TypeMono).TextColor(tk.Text)
					}
					if s.Agent.TokenPath != "" {
						ui.Text(c, msgs.SettingsTokenPath(s.Agent.TokenPath)).
							Font("monospace").FontSize(tk.TypeMono).
							TextColor(tk.TextFaint).SingleLine()
					}
					if s.Token != "" {
						// The one sight of a token that is not in a file: it is
						// said to be shown once, beside the action that made it.
						ui.Text(c, s.Token).Font("monospace").FontSize(tk.TypeMono).
							TextColor(tk.Text).SingleLine()
						muted(c, tk, msgs.SettingsTokenOnce())
					}
				})
				rotated = ui.Button(c, msgs.RotateToken()).MinHeight(tk.Control).AlignSelf(ui.Start).
					Clicked()
			})
		}).Description(msgs.SettingsAgentBody())
	})

	ui.Fieldset(c, msgs.SettingsAgentRoutes(), func() {
		muted(c, tk, msgs.SettingsAgentRoutesBody())
		routes := agentRoutes(msgs)
		columns := []ui.TableColumn{
			{Title: msgs.SettingsRouteMethod(), Width: routeMethodWidth, Fixed: true},
			{Title: msgs.SettingsRoutePath(), Width: routePathWidth, Fixed: true},
			{Title: msgs.SettingsRouteWhat()},
		}
		ui.Table(c, &s.RouteTable, columns, len(routes), func(row, col int) {
			if row < 0 || row >= len(routes) {
				return
			}
			route := routes[row]
			switch col {
			case 0:
				ui.Text(c, route.methods).Font("monospace").FontSize(tk.TypeMono).SingleLine()
			case 1:
				ui.Text(c, route.path).Font("monospace").FontSize(tk.TypeMono).SingleLine()
			default:
				ui.Text(c, route.what).FontSize(tk.TypeUI).SingleLine()
			}
		}).Height(routeTableHeight).Label(msgs.SettingsAgentRoutes())
	})

	if toggled {
		d.setAgent(c, s.AgentOn)
	}
	if rotated {
		d.rotateToken(c)
	}
}

func (d Deps) setAgent(c *ui.Context, on bool) {
	if d.SetAgent != nil {
		d.SetAgent(c, on)
	}
}

func (d Deps) rotateToken(c *ui.Context) {
	if d.RotateToken != nil {
		d.RotateToken(c)
	}
}
