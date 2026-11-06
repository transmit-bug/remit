// Package store keeps a Remit library on disk.
//
// A library is one directory (ADR 0003):
//
//	remit.json            format version and FSRS parameters
//	notes/<slug>-<id>.md  plaintext note; a referencable block ends with ^block_…
//	cards/<card-id>.json  card definition: the block it renders, overrides, status
//	reviews.jsonl         append-only review events, the only scheduling source
//	.remit/index.sqlite   derived index: delete it and it comes back
//
// The text files are the truth. The index is a cache of what they say, so a
// rebuild costs a replay of the log and a walk over the notes, never a loss.
// Nothing here reads the clock: time arrives through Options.Now.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
)

const (
	configName = "remit.json"
	notesDir   = "notes"
	cardsDir   = "cards"
	logName    = "reviews.jsonl"
	indexDir   = ".remit"
	indexName  = "index.sqlite"

	// formatVersion is what this build writes, and the only layout it reads.
	formatVersion = "remit/1"

	noteKind  = "note"
	cardKind  = "card"
	maxSlug   = 48
	filePerms = 0o644
	dirPerms  = 0o755
)

// Errors a caller is expected to handle rather than report as a bug.
var (
	// ErrClosed is returned when a library is used after Close.
	ErrClosed = errors.New("store: library is closed")
	// ErrNoReview is returned by UndoLast when a card has nothing to undo.
	ErrNoReview = errors.New("store: no review to undo")
	// ErrExists is returned by Create when the directory already holds a library.
	ErrExists = errors.New("store: directory already holds a library")
)

// Options configures a library.
type Options struct {
	// Now is the library's clock. A zero value means time.Now.
	Now func() time.Time
	// Params is the schedule a new library starts with. An existing library
	// keeps the one in its remit.json.
	Params schedule.Params
}

// Library is an open library. It is safe for concurrent use; every write takes
// the lock, and a read only ever returns values it has already loaded.
type Library struct {
	mu     sync.Mutex
	dir    string
	now    func() time.Time
	params schedule.Params

	index   *index
	applier *schedule.Applier
	nextSeq int64

	notes  map[core.ID]noteEntry
	cards  map[core.ID]cardEntry
	blocks map[core.ID]blockRef

	// events is the review log as loaded, kept so a caller can ask what
	// happened without re-reading the file. Reviews append to it.
	events []schedule.Event
}

// noteEntry is a note and where its source lives.
type noteEntry struct {
	note core.Note
	slug string
	path string
}

// cardEntry is a card and the moment it was created, which the card itself does
// not carry.
type cardEntry struct {
	card    core.Card
	created time.Time
}

// blockRef is where a referencable block was found: which note, the block
// itself, and its position in the note, which is where its marker would go.
type blockRef struct {
	note  core.ID
	block core.Block
	at    int
}

// configFile is remit.json.
type configFile struct {
	Format string          `json:"format"`
	FSRS   schedule.Params `json:"fsrs"`
}

// Blocks reports whether a Library can stand in for a card's source of text.
var _ core.Blocks = (*Library)(nil)

// Create makes a new library in dir, which must not already hold one.
func Create(dir string, opts Options) (*Library, error) {
	path := filepath.Join(dir, configName)
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrExists, dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	library := newLibrary(dir, opts)
	library.params = opts.Params.Normalized()
	if err := os.MkdirAll(dir, dirPerms); err != nil {
		return nil, err
	}
	config, err := encodeConfig(library.params)
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(path, config); err != nil {
		return nil, err
	}
	if err := library.load(); err != nil {
		library.Close()
		return nil, err
	}
	return library, nil
}

// Open opens the library in dir. The scheduling parameters come from the
// library's own remit.json; Options.Params is the fallback for a library that
// declares none.
func Open(dir string, opts Options) (*Library, error) {
	data, err := os.ReadFile(filepath.Join(dir, configName))
	if err != nil {
		return nil, err
	}
	params, err := decodeConfig(data)
	if err != nil {
		return nil, err
	}
	if params.IsZero() {
		params = opts.Params.Normalized()
	}

	library := newLibrary(dir, opts)
	library.params = params
	if err := library.load(); err != nil {
		library.Close()
		return nil, err
	}
	return library, nil
}

func newLibrary(dir string, opts Options) *Library {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Library{
		dir:    dir,
		now:    now,
		notes:  map[core.ID]noteEntry{},
		cards:  map[core.ID]cardEntry{},
		blocks: map[core.ID]blockRef{},
	}
}

// Close releases the index. Notes and cards stay readable; open handles see
// ErrClosed from anything that has to touch the disk.
func (l *Library) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.index.close()
	l.index = nil
	return err
}

// Rebuild throws the index away and derives it again from the text sources:
// a replay of the review log plus a walk over the notes and cards.
func (l *Library) Rebuild() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if err := l.index.close(); err != nil {
		return err
	}
	l.index = nil
	if err := os.RemoveAll(filepath.Join(l.dir, indexDir)); err != nil {
		return err
	}
	return l.load()
}

// load reads every source of truth and derives the index from them. It is the
// whole of what opening and rebuilding do, so the two cannot disagree.
func (l *Library) load() error {
	for _, dir := range []string{notesDir, cardsDir, indexDir} {
		if err := os.MkdirAll(filepath.Join(l.dir, dir), dirPerms); err != nil {
			return err
		}
	}

	notes, err := l.readNotes()
	if err != nil {
		return err
	}
	cards, err := l.readCards()
	if err != nil {
		return err
	}
	events, err := readLog(l.logPath())
	if err != nil {
		return err
	}

	applier := schedule.NewApplier(l.params)
	if err := applier.Apply(events...); err != nil {
		return fmt.Errorf("store: review log: %w", err)
	}

	l.notes = notes
	l.cards = cards
	l.applier = applier
	l.events = events
	l.nextSeq = 0
	for _, event := range events {
		if event.Seq > l.nextSeq {
			l.nextSeq = event.Seq
		}
	}
	l.blocks = map[core.ID]blockRef{}
	for id, entry := range l.notes {
		noteBlocks(&l.blocks, id, entry.note)
	}
	return l.deriveIndex()
}

// deriveIndex builds the index from what is loaded. A first attempt that fails
// on a damaged index file is retried once from an empty one: the index is a
// cache, never a reason to refuse a library.
func (l *Library) deriveIndex() error {
	err := l.buildIndex()
	if err == nil {
		return nil
	}
	if err := os.RemoveAll(filepath.Join(l.dir, indexDir)); err != nil {
		return err
	}
	if retry := l.buildIndex(); retry != nil {
		return err
	}
	return nil
}

func (l *Library) buildIndex() error {
	if err := l.index.close(); err != nil {
		return err
	}
	l.index = nil

	idx, err := openIndex(l.indexPath())
	if err != nil {
		return err
	}
	if err := idx.reset(); err != nil {
		idx.close()
		return err
	}
	l.index = idx

	return idx.write(func(e execer) error {
		for id, entry := range l.notes {
			for _, block := range entry.note.Blocks {
				if block.ID == "" {
					continue
				}
				if err := putBlock(e, id, block); err != nil {
					return err
				}
			}
		}
		for id, entry := range l.cards {
			if err := putCard(e, l.effective(entry.card), entry.created); err != nil {
				return err
			}
			if state, ok := l.applier.State(id); ok {
				if err := putState(e, id, state); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (l *Library) readNotes() (map[core.ID]noteEntry, error) {
	entries, err := os.ReadDir(filepath.Join(l.dir, notesDir))
	if err != nil {
		return nil, err
	}
	notes := make(map[core.ID]noteEntry, len(entries))
	for _, entry := range entries {
		id, slug, ok := splitNoteName(entry.Name())
		if entry.IsDir() || !ok {
			continue
		}
		path := filepath.Join(l.dir, notesDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		title, blocks := parseNote(data)
		note := core.Note{ID: id, Title: title}
		for _, block := range blocks {
			note.Blocks = append(note.Blocks, block.Block)
		}
		notes[id] = noteEntry{note: note, slug: slug, path: path}
	}
	return notes, nil
}

func (l *Library) readCards() (map[core.ID]cardEntry, error) {
	entries, err := os.ReadDir(filepath.Join(l.dir, cardsDir))
	if err != nil {
		return nil, err
	}
	cards := make(map[core.ID]cardEntry, len(entries))
	for _, entry := range entries {
		id, ok := splitCardName(entry.Name())
		if entry.IsDir() || !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(l.dir, cardsDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		card, created, err := decodeCard(id, data)
		if err != nil {
			return nil, err
		}
		cards[id] = cardEntry{card: card, created: created}
	}
	return cards, nil
}

// splitNoteName reads the ID and the frozen slug out of <slug>-<id>.md.
func splitNoteName(name string) (core.ID, string, bool) {
	base, ok := strings.CutSuffix(name, ".md")
	if !ok {
		return "", "", false
	}
	cut := strings.LastIndex(base, "-")
	if cut < 0 {
		return "", "", false
	}
	id := core.ID(base[cut+1:])
	if id.Kind() != noteKind || !id.Valid() {
		return "", "", false
	}
	return id, base[:cut], true
}

func splitCardName(name string) (core.ID, bool) {
	base, ok := strings.CutSuffix(name, ".json")
	if !ok {
		return "", false
	}
	id := core.ID(base)
	if id.Kind() != cardKind || !id.Valid() {
		return "", false
	}
	return id, true
}

// Note returns a note and its blocks.
func (l *Library) Note(id core.ID) (core.Note, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.notes[id]
	if !ok {
		return core.Note{}, false
	}
	return cloneNote(entry.note), true
}

// Notes returns every note, by title.
func (l *Library) Notes() []core.Note {
	l.mu.Lock()
	defer l.mu.Unlock()
	notes := make([]core.Note, 0, len(l.notes))
	for _, entry := range l.notes {
		notes = append(notes, cloneNote(entry.note))
	}
	sort.Slice(notes, func(i, j int) bool {
		a, b := strings.ToLower(notes[i].Title), strings.ToLower(notes[j].Title)
		if a != b {
			return a < b
		}
		return notes[i].ID < notes[j].ID
	})
	return notes
}

// SaveNote writes a note's source and returns it as stored. A block presented
// without an ID is minted one, so every block a library holds is referencable;
// the note's slug is minted once and never follows a title change, so the path
// stays stable while the file is browsable. A block's text may not contain a
// blank line: that is where one block ends and the next begins in the file.
func (l *Library) SaveNote(note core.Note) (core.Note, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return core.Note{}, ErrClosed
	}
	if note.ID == "" {
		note.ID = core.NewID(noteKind)
	}
	if note.ID.Kind() != noteKind || !note.ID.Valid() {
		return core.Note{}, fmt.Errorf("store: %q is not a note ID", note.ID)
	}

	entry, exists := l.notes[note.ID]
	slug := slugify(note.Title)
	path := l.notePath(slug, note.ID)
	if exists {
		// The slug is minted once: a renamed note keeps its path, and a file
		// renamed by hand is written back where it is.
		slug, path = entry.slug, entry.path
	}
	note = normalizeNote(note)

	if err := writeFileAtomic(path, renderNote(note)); err != nil {
		return core.Note{}, err
	}
	if exists {
		for _, block := range entry.note.Blocks {
			if block.ID != "" {
				delete(l.blocks, block.ID)
			}
		}
	}
	l.notes[note.ID] = noteEntry{note: note, slug: slug, path: path}
	noteBlocks(&l.blocks, note.ID, note)

	err := l.index.write(func(e execer) error {
		if err := deleteBlocksOfNote(e, note.ID); err != nil {
			return err
		}
		for _, block := range note.Blocks {
			if block.ID == "" {
				continue
			}
			if err := putBlock(e, note.ID, block); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return core.Note{}, err
	}
	return cloneNote(note), nil
}

// CreateNote makes an empty note with a title.
func (l *Library) CreateNote(title string) (core.Note, error) {
	return l.SaveNote(core.Note{ID: core.NewID(noteKind), Title: title})
}

// Block returns a referencable block, so a card can render itself from the
// library it lives in.
func (l *Library) Block(id core.ID) (core.Block, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ref, ok := l.blocks[id]
	return ref.block, ok
}

// Card returns a card with the lifecycle status it is in now.
func (l *Library) Card(id core.ID) (core.Card, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.cards[id]
	if !ok {
		return core.Card{}, false
	}
	return l.effective(entry.card), true
}

// Cards returns every card, oldest first.
func (l *Library) Cards() []core.Card {
	l.mu.Lock()
	defer l.mu.Unlock()
	entries := make([]cardEntry, 0, len(l.cards))
	for _, entry := range l.cards {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].created.Equal(entries[j].created) {
			return entries[i].created.Before(entries[j].created)
		}
		return entries[i].card.ID < entries[j].card.ID
	})
	cards := make([]core.Card, len(entries))
	for i, entry := range entries {
		cards[i] = l.effective(entry.card)
	}
	return cards
}

// SaveCard writes a card's definition. Its history is untouched: a card file
// says what a card shows, never how well it is known.
func (l *Library) SaveCard(card core.Card) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if card.ID.Kind() != cardKind || !card.ID.Valid() {
		return fmt.Errorf("store: %q is not a card ID", card.ID)
	}

	entry, exists := l.cards[card.ID]
	created := l.now().UTC()
	if exists {
		created = entry.created
	}
	encoded, err := encodeCard(card, created)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(l.cardPath(card.ID), encoded); err != nil {
		return err
	}
	l.cards[card.ID] = cardEntry{card: card, created: created}
	return l.reindexCard(card.ID)
}

// CreateCard makes the first card for a block, minting the block's marker in
// the note source when the file does not carry it yet.
func (l *Library) CreateCard(blockID core.ID) (core.Card, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return core.Card{}, ErrClosed
	}
	ref, ok := l.blocks[blockID]
	if !ok {
		return core.Card{}, fmt.Errorf("store: no block %q", blockID)
	}
	entry, ok := l.notes[ref.note]
	if !ok {
		return core.Card{}, fmt.Errorf("store: no note %q", ref.note)
	}
	if err := l.markBlock(entry, ref); err != nil {
		return core.Card{}, err
	}

	card := core.Card{
		ID:     core.NewID(cardKind),
		Ref:    core.Reference{Block: blockID},
		Status: core.StatusNew,
	}
	created := l.now().UTC()
	encoded, err := encodeCard(card, created)
	if err != nil {
		return core.Card{}, err
	}
	if err := writeFileAtomic(l.cardPath(card.ID), encoded); err != nil {
		return core.Card{}, err
	}
	l.cards[card.ID] = cardEntry{card: card, created: created}
	if err := l.reindexCard(card.ID); err != nil {
		return core.Card{}, err
	}
	return card, nil
}

// markBlock appends a block's marker to its note source, leaving every other
// byte of the file as it was.
func (l *Library) markBlock(entry noteEntry, ref blockRef) error {
	data, err := os.ReadFile(entry.path)
	if err != nil {
		return err
	}
	marked, err := withMarker(data, ref.block.ID, ref.block.Text, ref.at)
	if err != nil {
		return err
	}
	if bytes.Equal(marked, data) {
		return nil
	}
	return writeFileAtomic(entry.path, marked)
}

// Review records that a card was reviewed, and schedules it again.
func (l *Library) Review(cardID core.ID, rating schedule.Rating, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if _, ok := l.cards[cardID]; !ok {
		return fmt.Errorf("store: no card %q", cardID)
	}
	if !rating.Valid() {
		return fmt.Errorf("store: unknown rating %d", uint8(rating))
	}
	event := schedule.Event{Seq: l.nextSeq + 1, Card: cardID, Rating: rating, At: l.instant(at)}
	return l.record(event, cardID)
}

// UndoLast cancels a card's newest review by appending the void that names it.
// The log is never rewritten, so an undo is as durable as the review it undoes.
func (l *Library) UndoLast(cardID core.ID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	if _, ok := l.cards[cardID]; !ok {
		return fmt.Errorf("store: no card %q", cardID)
	}
	last, ok := l.applier.Last(cardID)
	if !ok {
		return fmt.Errorf("%w: card %q", ErrNoReview, cardID)
	}
	return l.record(schedule.Event{Seq: l.nextSeq + 1, Void: last.Seq}, cardID)
}

// record appends an event to the log and brings the card's schedule with it.
func (l *Library) record(event schedule.Event, card core.ID) error {
	if err := appendLog(l.logPath(), event); err != nil {
		return err
	}
	l.nextSeq = event.Seq
	l.events = append(l.events, event)
	if err := l.applier.Apply(event); err != nil {
		return err
	}
	return l.reindexCard(card)
}

// Events returns the review log in order, void events included, so a caller
// can compute what the schedule cannot answer on its own (counts per day,
// predictions against outcomes).
func (l *Library) Events() []schedule.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]schedule.Event(nil), l.events...)
}

// CardState returns a card's replayed schedule, and whether it has been
// reviewed at all.
func (l *Library) CardState(cardID core.ID) (schedule.State, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.applier.State(cardID)
}

// Due returns the cards to review now, soonest first. A limit of zero or less
// returns the whole queue.
func (l *Library) Due(now time.Time, limit int) ([]core.Card, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	ids, err := l.index.due(l.instant(now), limit)
	if err != nil {
		return nil, err
	}
	return l.cardsByID(ids), nil
}

// Remaining counts the cards due now, without materialising them.
func (l *Library) Remaining(now time.Time) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return 0, ErrClosed
	}
	return l.index.remaining(l.instant(now))
}

// LostCards returns the cards whose block is gone and which kept no snapshot,
// so the UI can offer to re-point them. They keep their schedule meanwhile.
func (l *Library) LostCards() ([]core.Card, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	ids, err := l.index.lost()
	if err != nil {
		return nil, err
	}
	return l.cardsByID(ids), nil
}

func (l *Library) cardsByID(ids []core.ID) []core.Card {
	cards := make([]core.Card, 0, len(ids))
	for _, id := range ids {
		if entry, ok := l.cards[id]; ok {
			cards = append(cards, l.effective(entry.card))
		}
	}
	return cards
}

// effective is the lifecycle a card is in. A review never rewrites a card file
// (ADR 0003), so the step out of "new" is derived from the log: a card with live
// reviews has been learned, and the last rating says whether it is still being
// learned.
func (l *Library) effective(card core.Card) core.Card {
	if card.Status != core.StatusNew && card.Status != core.StatusLearning {
		return card
	}
	last, ok := l.applier.Last(card.ID)
	if !ok {
		return card
	}
	if last.Rating == schedule.Again {
		card.Status = core.StatusLearning
	} else {
		card.Status = core.StatusReview
	}
	return card
}

func (l *Library) reindexCard(cardID core.ID) error {
	return l.index.write(func(e execer) error {
		entry, ok := l.cards[cardID]
		if !ok {
			return deleteCard(e, cardID)
		}
		if err := putCard(e, l.effective(entry.card), entry.created); err != nil {
			return err
		}
		state, ok := l.applier.State(cardID)
		if !ok {
			return deleteState(e, cardID)
		}
		return putState(e, cardID, state)
	})
}

// instant is the moment to act at: the one given, or now when none was. Every
// stored time is UTC, so a log written on one machine replays the same on
// another.
func (l *Library) instant(at time.Time) time.Time {
	if at.IsZero() {
		at = l.now()
	}
	return at.UTC()
}

func (l *Library) notePath(slug string, id core.ID) string {
	return filepath.Join(l.dir, notesDir, slug+"-"+string(id)+".md")
}

func (l *Library) cardPath(id core.ID) string {
	return filepath.Join(l.dir, cardsDir, string(id)+".json")
}

func (l *Library) logPath() string {
	return filepath.Join(l.dir, logName)
}

func (l *Library) indexPath() string {
	return filepath.Join(l.dir, indexDir, indexName)
}

// noteBlocks indexes a note's referencable blocks.
func noteBlocks(into *map[core.ID]blockRef, note core.ID, n core.Note) {
	for i, block := range n.Blocks {
		if block.ID == "" {
			continue
		}
		(*into)[block.ID] = blockRef{note: note, block: block, at: i}
	}
}

func cloneNote(note core.Note) core.Note {
	note.Blocks = append([]core.Block(nil), note.Blocks...)
	return note
}

// slugify makes a title safe to put in a file name, keeping its letters so the
// name stays browsable.
func slugify(title string) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			out.WriteRune(r)
			dash = false
		case !dash && out.Len() > 0:
			out.WriteByte('-')
			dash = true
		}
		if out.Len() >= maxSlug {
			break
		}
	}
	slug := strings.Trim(out.String(), "-")
	if slug == "" {
		return "note"
	}
	return slug
}

// encodeConfig renders remit.json, the library's declaration of how it
// schedules.
func encodeConfig(params schedule.Params) ([]byte, error) {
	data, err := json.MarshalIndent(configFile{Format: formatVersion, FSRS: params}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: %s: %w", configName, err)
	}
	return append(data, '\n'), nil
}

func decodeConfig(data []byte) (schedule.Params, error) {
	var config configFile
	if err := json.Unmarshal(data, &config); err != nil {
		return schedule.Params{}, fmt.Errorf("store: %s: %w", configName, err)
	}
	if config.Format != formatVersion {
		return schedule.Params{}, fmt.Errorf("store: %s: format %q, want %q", configName, config.Format, formatVersion)
	}
	return config.FSRS.Normalized(), nil
}
