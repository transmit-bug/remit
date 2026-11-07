package exttest

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
)

// DefaultNow is the clock a fixture run uses unless host.json sets one: a fixed
// moment, so a plugin that asks the time still answers the same thing twice.
const DefaultNow = "2026-11-08T09:00:00Z"

// errNoWrite is what the fixture host answers a write: a case compares an
// answer, not a library change, and a fixture that writes cannot be replayed.
var errNoWrite = errors.New("exttest: the fixture host does not write; a case compares an answer, not a library change")

// fixtureHost is the library a plugin reaches during a fixture run, fixed by
// fixtures/host.json: the same notes, the same clock, the same plugin store on
// every run. It answers reads and refuses writes.
type fixtureHost struct {
	now     time.Time
	notes   []core.Note
	data    string
	hasData bool
}

// defaultHost is a library with no notes, the default clock and no plugin data.
func defaultHost() *fixtureHost {
	at, err := time.Parse(time.RFC3339, DefaultNow)
	if err != nil {
		panic("exttest: DefaultNow is not RFC 3339: " + err.Error())
	}
	return &fixtureHost{now: at}
}

// hostFile is fixtures/host.json: what the plugin may reach while its hooks
// run. Every field is optional; the zero host has no notes and the fixed clock.
type hostFile struct {
	Now        string     `json:"now"`
	Notes      []noteWire `json:"notes"`
	PluginData string     `json:"pluginData"`
}

type noteWire struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Blocks []blockWire `json:"blocks"`
}

type blockWire struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// readHost reads fixtures/host.json.
func readHost(path string) (*fixtureHost, error) {
	var file hostFile
	if err := decode(path, &file); err != nil {
		return nil, fmt.Errorf("%s/%s: %w", FixturesDir, HostFile, err)
	}
	out := defaultHost()
	if file.Now != "" {
		at, err := time.Parse(time.RFC3339, file.Now)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: now: %w", FixturesDir, HostFile, err)
		}
		out.now = at
	}
	for _, note := range file.Notes {
		built := core.Note{ID: core.ID(note.ID), Title: note.Title}
		for _, block := range note.Blocks {
			built.Blocks = append(built.Blocks, core.Block{ID: core.ID(block.ID), Text: block.Text})
		}
		out.notes = append(out.notes, built)
	}
	out.data, out.hasData = file.PluginData, file.PluginData != ""
	return out, nil
}

// The reads a plugin may make of the fixture host.

func (h *fixtureHost) Notes() []core.Note { return h.notes }

func (h *fixtureHost) Note(id core.ID) (core.Note, bool) {
	for _, note := range h.notes {
		if note.ID == id {
			return note, true
		}
	}
	return core.Note{}, false
}

func (h *fixtureHost) SearchNotes(text string) ([]core.Note, error) {
	query := strings.ToLower(text)
	var out []core.Note
	for _, note := range h.notes {
		if strings.Contains(strings.ToLower(note.Title), query) {
			out = append(out, note)
			continue
		}
		for _, block := range note.Blocks {
			if strings.Contains(strings.ToLower(block.Text), query) {
				out = append(out, note)
				break
			}
		}
	}
	return out, nil
}

func (h *fixtureHost) Block(id core.ID) (core.Block, bool) {
	for _, note := range h.notes {
		if block, ok := note.Block(id); ok {
			return block, true
		}
	}
	return core.Block{}, false
}

func (h *fixtureHost) Cards() []core.Card { return nil }

func (h *fixtureHost) Card(core.ID) (core.Card, bool) { return core.Card{}, false }

func (h *fixtureHost) CardState(core.ID) (schedule.State, bool) { return schedule.State{}, false }

func (h *fixtureHost) History(core.ID) ([]schedule.Event, error) { return nil, nil }

// The reading queue is empty: a fixture host holds notes to read, not a session
// to read them in, and no plugin capability reaches it today.
func (h *fixtureHost) ReadingQueue() ([]store.Reading, error) { return nil, nil }

func (h *fixtureHost) ReadingFor(core.ID) (store.Reading, bool) { return store.Reading{}, false }

func (h *fixtureHost) ReadingEvents(core.ID) ([]schedule.Event, error) { return nil, nil }

func (h *fixtureHost) Due(time.Time, int) ([]core.Card, error) { return nil, nil }

func (h *fixtureHost) Remaining(time.Time) (int, error) { return 0, nil }

// Faces answers nothing: a fixture host holds no cards, and a plugin reads card
// faces through its page tree, not through this.
func (h *fixtureHost) Faces(core.Card) (front, back, link string) {
	return "", "", core.LinkLost.String()
}

func (h *fixtureHost) Now() time.Time { return h.now }

func (h *fixtureHost) ScheduleParams() schedule.Params { return schedule.DefaultParams() }

func (h *fixtureHost) PluginData(id string) (string, bool) {
	if !h.hasData {
		return "", false
	}
	return h.data, true
}

func (h *fixtureHost) SetPluginData(id, value string) error {
	h.data, h.hasData = value, true
	return nil
}

// The writes: a fixture run never changes a library.

func (h *fixtureHost) SaveNote(core.Note) (core.Note, error) { return core.Note{}, errNoWrite }

func (h *fixtureHost) CreateNote(string) (core.Note, error) { return core.Note{}, errNoWrite }

func (h *fixtureHost) CreateCard(core.ID) (core.Card, error) { return core.Card{}, errNoWrite }

func (h *fixtureHost) SaveCard(core.Card) error { return errNoWrite }

func (h *fixtureHost) Review(core.ID, schedule.Rating, time.Time) error { return errNoWrite }

func (h *fixtureHost) DeleteCard(core.ID) error { return errNoWrite }

func (h *fixtureHost) AddToReading(core.ID, time.Time) error { return errNoWrite }

// The fixture host is a host the loader may hand a plugin.
var _ ext.Host = (*fixtureHost)(nil)
