package main

import (
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fmt"
	"github.com/transmit-bug/remit/internal/agentapi"
	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/core/stats"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
	appui "github.com/transmit-bug/remit/internal/ui"
)

// The files a client reads to find the API: the port the system chose, and the
// token a write needs. Both sit beside settings.json and are owner-only.
const (
	agentTokenFile   = "agent-token"
	agentAddressFile = "agent-address"
)

// agentService is the loopback agent API as the app runs it: one listener, one
// token file, and the files that tell a client where both are. It is the
// settings page's AgentControls, so the page's switch binds or drops the port
// now rather than at the next launch.
type agentService struct {
	dir string         // the user data directory
	lib *store.Library // the library open now, nil before one is
	ln  net.Listener   // the listener serving now, nil when stopped
}

// newAgentService returns a service that keeps its files in dir.
func newAgentService(dir string) *agentService { return &agentService{dir: dir} }

// setLibrary points the service at the library that just opened. A different
// library replaces the listener, so the API never serves a library the user
// left.
func (s *agentService) setLibrary(lib *store.Library) {
	if s.lib == lib {
		return
	}
	s.Stop()
	s.lib = lib
}

// startIfEnabled starts the listener when the settings say the API is on, which
// is how a launch honours the switch a previous run left on.
func (s *agentService) startIfEnabled(on bool) error {
	if !on {
		return nil
	}
	return s.SetEnabled(true)
}

// SetEnabled starts the listener, choosing a loopback port and writing it
// beside the token, or stops it and removes the address, so a client that
// finds no address knows no API is served. Starting one that runs does nothing.
func (s *agentService) SetEnabled(on bool) error {
	if !on {
		s.Stop()
		return nil
	}
	if s.ln != nil {
		return nil
	}
	if s.lib == nil {
		return errors.New("agent service: no library is open")
	}
	token, err := agentapi.LoadOrCreateToken(filepath.Join(s.dir, agentTokenFile))
	if err != nil {
		return err
	}
	ln, err := serveAgent(s.lib, token)
	if err != nil {
		return err
	}
	addr := ln.Addr().String()
	if err := os.WriteFile(filepath.Join(s.dir, agentAddressFile), []byte(addr+"\n"), 0o600); err != nil {
		ln.Close()
		return err
	}
	s.ln = ln
	slog.Info("remit: agent api listening", "addr", addr)
	return nil
}

// Stop closes the listener and forgets the address file.
func (s *agentService) Stop() {
	if s.ln == nil {
		return
	}
	if err := s.ln.Close(); err != nil {
		slog.Warn("remit: agent api stopped with an error", "err", err)
	}
	s.ln = nil
	if err := os.Remove(filepath.Join(s.dir, agentAddressFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("remit: agent api: address file", "err", err)
	}
	slog.Info("remit: agent api stopped")
}

// RotateToken writes a fresh token and restarts the listener with it, so the
// token that leaked stops working at once. A service that is not serving gets a
// new token for its next start, and opens nothing now.
func (s *agentService) RotateToken() error {
	running := s.ln != nil
	s.Stop()
	if _, err := agentapi.RotateToken(filepath.Join(s.dir, agentTokenFile)); err != nil {
		return err
	}
	if !running {
		return nil
	}
	return s.SetEnabled(true)
}

// Status is where the API listens and which file holds its token, read from the
// files this service writes: what a client sees, not what the listener knows.
func (s *agentService) Status() appui.AgentStatus {
	status := appui.AgentStatus{TokenPath: filepath.Join(s.dir, agentTokenFile)}
	data, err := os.ReadFile(filepath.Join(s.dir, agentAddressFile))
	if err == nil {
		status.Address = strings.TrimSpace(string(data))
	}
	return status
}

// cardSource is the library as the agent API sees it — and as a plugin reaches
// it: the shared read/write layer, plus the one write the store does not do
// itself, deriving a card from a block. A card is made by asking the shared
// resolver what the block asks, exactly as the notebook does: neither the API
// nor a plugin derives on its own.
type cardSource struct {
	*store.Library
	faces *cards.Resolver
}

// errNoCardPlugin is what a card write answers when the card plugin could not
// start: reads still work, and a card that cannot be derived says why.
var errNoCardPlugin = errors.New("remit: the card plugin did not start")

// newCardSource opens the card plugin and the library's types for the API's
// own use. A plugin that does not start leaves a source that reads the library
// and cannot make a card, so that what does not need derivation keeps working.
func newCardSource(lib *store.Library) (cardSource, error) {
	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		return cardSource{Library: lib}, err
	}
	return cardSource{Library: lib, faces: cards.New(runner, lib.Types())}, nil
}

// Faces is what a card asks, as its type's templates render it.
func (s cardSource) Faces(card core.Card) (front, back, link string) {
	if s.faces == nil {
		text, state := card.Ref.Text(blocksOf(s.Library))
		return text, "", state.String()
	}
	return facesOf(s.faces, s.Library, card)
}

// CreateCard makes the cards the block's text asks for, and answers with the
// first: what the API's contract asks for.
func (s cardSource) CreateCard(blockID core.ID) (core.Card, error) {
	if s.faces == nil {
		return core.Card{}, errNoCardPlugin
	}
	block, ok := s.Block(blockID)
	if !ok {
		return core.Card{}, fmt.Errorf("agent api: no block %q", blockID)
	}
	derived, ok := s.faces.Derive(block.Text)
	if !ok {
		return core.Card{}, fmt.Errorf("agent api: the card plugin could not derive")
	}
	cards, err := s.Library.CreateCards(blockID, derived)
	if err != nil {
		return core.Card{}, err
	}
	if len(cards) == 0 {
		return core.Card{}, fmt.Errorf("agent api: the block asks nothing")
	}
	return cards[0], nil
}

// serveAgent serves the agent API on a loopback port the system chooses, and
// returns the listener. Loopback only: nothing on the network can reach it.
func serveAgent(lib *store.Library, token string) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	source, err := newCardSource(lib)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	srv := agentapi.New(source, agentapi.Options{Token: token, Stats: source, Today: todayCount(lib)})
	go func() {
		// Closing the listener is how the service stops, so a closed
		// connection is the end of it, not a failure.
		if err := srv.Serve(ln); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Warn("remit: agent api stopped", "err", err)
		}
	}()
	return ln, nil
}

// todayCount is the agent API's today count: the lines internal/core/queue
// builds for one day, unbudgeted and with the reading introduction uncapped, so
// the number is what the day holds rather than what one sitting would take.
func todayCount(lib *store.Library) func(time.Time) (int, error) {
	return func(now time.Time) (int, error) {
		today, err := queue.Entries(lib, now, queue.TodayOptions{})
		if err != nil {
			return 0, err
		}
		return len(today.Entries), nil
	}
}

// facesOf asks the shared resolver what a card asks. It is the same door the
// notebook goes through — the plugin derives, the type's templates render —
// so the API and the app read a card alike.
func facesOf(faces *cards.Resolver, lib *store.Library, card core.Card) (front, back, link string) {
	answer := faces.Face(card, blocksOf(lib))
	return answer.Front, answer.Back, answer.State.String()
}

// Summary, Forecast and Retention are the statistics routes' answers: the log
// and the schedule read by the stats package over the library the API serves,
// so an agent sees the numbers the statistics view plots.
func (s cardSource) Summary(now time.Time) stats.Summary { return stats.Summarize(s.Library, now) }

func (s cardSource) Forecast(from time.Time, days int) []stats.DayCount {
	return stats.ForecastAll(s.Library, from, days)
}

func (s cardSource) Retention() stats.Retention {
	return stats.RetentionOf(s.Library, s.ScheduleParams())
}

// blocksOf is the library's blocks as the model reads them.
func blocksOf(lib *store.Library) core.Blocks {
	blocks := core.BlockSet{}
	for _, note := range lib.Notes() {
		for _, block := range note.Blocks {
			blocks[block.ID] = block
		}
	}
	return blocks
}
