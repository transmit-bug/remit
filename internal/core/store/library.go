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
// The text files are the truth. The index is a cache of what they say: it holds
// every card and its replayed schedule, the text of every note block, and the
// watermark of the log it has applied. An open reads the sources only when the
// index has fallen behind them, so a library of a hundred thousand cards opens
// by reading one file and no cards. A rebuild costs a replay of the log and a
// walk over the notes, never a loss. Nothing here reads the clock: time arrives
// through Options.Now.
package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

	// ErrNothingToAsk is a block whose text makes no card: a paragraph without
	// a separator or a blank.
	ErrNothingToAsk = errors.New("store: the block asks nothing")
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
	// Scheduler names what schedules this library's reviews; every review it
	// records is stamped with it. Empty keeps whatever the library holds.
	Scheduler string
}

// Library is an open library. It is safe for concurrent use: every call takes
// the lock, so a reader never sees half of a write.
//
// What it keeps in memory is the index's answer to "what cards are there and
// when is each due" — the catalog — because that is what listing and the queue
// ask for on every write. Note and block text stay on disk, read when a caller
// names a note, a block or a card file.
type Library struct {
	mu        sync.Mutex
	dir       string
	now       func() time.Time
	params    schedule.Params
	scheduler string

	index   *index
	nextSeq int64
	digest  string

	// sources counts what this handle has read off the disk, which a test uses
	// to prove an open with a current index reads no source file.
	sources sourceReads

	cards map[core.ID]*cardMeta
	// byDue is the review queue as the index orders it: every card that can be
	// reviewed, soonest first. It is kept in step with the catalog one card at
	// a time, so a review does not re-sort a hundred thousand cards.
	byDue     []dueEntry
	dueOK     bool
	byCreated []core.ID
	createdOK bool

	extensions []string
	onEvent    func(Event)
}

// cardMeta is one card as the index holds it: the file's definition, the
// lifecycle the log gives it, and whether its block still resolves.
type cardMeta struct {
	card    core.Card   // Status is the effective one, not the file's
	stored  core.Status // the status the card file carries
	created time.Time
	live    bool
	state   schedule.State // zero when no live review
	lastSeq int64          // sequence of the newest live review
}

// dueEntry is one card in the queue order: when it comes up, and which card.
type dueEntry struct {
	at int64
	id core.ID
}

// configFile is remit.json.
type configFile struct {
	Format string          `json:"format"`
	FSRS   schedule.Params `json:"fsrs"`
	// Extensions are the extensions this library enabled, by manifest name.
	Extensions []string `json:"extensions,omitempty"`
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
	library.scheduler = firstNonEmpty(opts.Scheduler, library.params.Scheduler)
	if err := os.MkdirAll(dir, dirPerms); err != nil {
		return nil, err
	}
	config, err := encodeConfig(library.params, nil)
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
	params, extensions, err := decodeConfig(data)
	if err != nil {
		return nil, err
	}
	if params.IsZero() {
		params = opts.Params.Normalized()
	}

	library := newLibrary(dir, opts)
	library.params = params
	// What schedules this library is what the app says it schedules with, or
	// what the library was last written with.
	library.scheduler = firstNonEmpty(opts.Scheduler, params.Scheduler)
	library.extensions = extensions
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
		dir:   dir,
		now:   now,
		cards: map[core.ID]*cardMeta{},
	}
}

// Close releases the index. Everything a library answers comes from it, so a
// closed handle reports ErrClosed, and a read that has no error to report says
// it found nothing.
func (l *Library) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.index.close()
	l.index = nil
	l.cards = map[core.ID]*cardMeta{}
	l.byDue, l.byCreated = nil, nil
	l.dueOK, l.createdOK = false, false
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

// load opens the index and reads it. A first attempt that fails is retried once
// from an empty index: the index is a cache, never a reason to refuse a library.
func (l *Library) load() error {
	for _, dir := range []string{notesDir, cardsDir, indexDir} {
		if err := os.MkdirAll(filepath.Join(l.dir, dir), dirPerms); err != nil {
			return err
		}
	}
	err := l.loadIndex()
	if err == nil {
		return nil
	}
	l.index.close()
	l.index = nil
	if removeErr := os.RemoveAll(filepath.Join(l.dir, indexDir)); removeErr != nil {
		return removeErr
	}
	if retry := l.loadIndex(); retry != nil {
		return retry
	}
	return nil
}

func (l *Library) loadIndex() error {
	l.digest = paramsDigest(l.params)
	idx, err := openIndex(l.indexPath())
	if err != nil {
		return err
	}
	l.index = idx
	if err := l.sync(); err != nil {
		return err
	}
	if err := idx.checkpoint(); err != nil {
		return err
	}
	return l.loadCatalog()
}

// sync brings the index in step with the text sources. The index records what
// the sources looked like when it read them, so an open with nothing changed on
// disk reads no note, no card and no log line; a log that only grew costs its
// new lines, and anything else costs a rebuild.
func (l *Library) sync() error {
	marks, err := l.sourceMarks()
	if err != nil {
		return err
	}
	meta, err := l.index.readMeta()
	if err != nil {
		return l.rebuildIndex(marks)
	}
	if meta[metaSchema] != schemaVersion ||
		meta[metaParams] != paramsDigest(l.params) ||
		meta[metaNotes] != formatInt(marks.notes) ||
		meta[metaCards] != formatInt(marks.cards) {
		return l.rebuildIndex(marks)
	}
	applied, ok := parseLogMark(meta[metaLog])
	if !ok || marks.log.size < applied.size {
		return l.rebuildIndex(marks)
	}
	if marks.log == applied.fileMark {
		l.nextSeq = applied.seq
		return nil
	}
	return l.replayLogTail(applied)
}

// rebuildIndex derives the whole index from the text sources: the notes, the
// cards, and a replay of the log.
func (l *Library) rebuildIndex(marks sourceMarks) error {
	notes, err := l.readNotes()
	if err != nil {
		return err
	}
	cards, err := l.readCards()
	if err != nil {
		return err
	}
	events, applied, err := l.readLogFrom(0)
	if err != nil {
		return err
	}

	applier := schedule.NewApplier(l.params)
	if err := applier.Apply(events...); err != nil {
		return fmt.Errorf("store: review log: %w", err)
	}
	states := applier.States()

	live := make(map[core.ID]bool, len(notes)*4)
	for _, entry := range notes {
		for _, block := range entry.note.Blocks {
			if block.ID != "" {
				live[block.ID] = true
			}
		}
	}

	// The newest live review of each card is what an undo cancels, so it is
	// kept beside the state instead of being looked up in the log again.
	lastSeq := make(map[core.ID]int64, len(states))
	liveEvents := liveBetween(events, time.Time{}, time.Time{})
	for _, event := range liveEvents {
		lastSeq[event.Card] = event.Seq
	}
	l.nextSeq = 0
	if len(events) > 0 {
		l.nextSeq = events[len(events)-1].Seq
	}

	if err := l.index.reset(); err != nil {
		return err
	}
	written, logOK := l.logMarkAfter(applied, l.nextSeq)

	return l.index.write(func(e execer) error {
		for id, entry := range notes {
			if err := putNote(e, noteRow{id: id, slug: entry.slug, title: entry.note.Title, updated: entry.note.Updated}); err != nil {
				return err
			}
			for at, block := range entry.note.Blocks {
				if err := putBlock(e, id, at, block); err != nil {
					return err
				}
			}
			if err := putLinks(e, id, entry.note.Blocks); err != nil {
				return err
			}
			if err := putTags(e, id, entry.note.Blocks); err != nil {
				return err
			}
		}
		for id, entry := range cards {
			state := states[id]
			row := cardRow{
				card:    entry.card,
				stored:  entry.card.Status,
				created: entry.created,
				live:    entry.card.Ref.Frozen != nil || live[entry.card.Ref.Block],
				state:   state,
				lastSeq: lastSeq[id],
			}
			row.card.Status = effectiveStatus(entry.card.Status, state)
			if err := putCard(e, row); err != nil {
				return err
			}
			if state.Reviews > 0 {
				if err := putState(e, id, state, lastSeq[id]); err != nil {
					return err
				}
			}
		}
		if err := l.putMarks(e, marks); err != nil {
			return err
		}
		if logOK {
			return l.putLogMark(e, written)
		}
		return clearLogMark(e)
	})
}

// replayLogTail applies the lines appended since the index last read the log.
// A review advances the card's cached schedule on its own; a void sends the
// library back to a full replay, because undoing one event needs that card's
// whole history and a tail that carries a void is a crash recovery, not a path
// worth carrying state for.
func (l *Library) replayLogTail(from logMark) error {
	var tail []schedule.Event
	applied, err := l.streamLog(from.size, func(ev schedule.Event) error {
		tail = append(tail, ev)
		return nil
	})
	if err != nil {
		return err
	}
	for _, ev := range tail {
		if ev.IsVoid() {
			marks, err := l.sourceMarks()
			if err != nil {
				return err
			}
			return l.rebuildIndex(marks)
		}
	}

	marks, err := l.sourceMarks()
	if err != nil {
		return err
	}
	nextSeq := l.nextSeq
	for _, ev := range tail {
		if ev.Seq > nextSeq {
			nextSeq = ev.Seq
		}
	}
	written, logOK := l.logMarkAfter(applied, nextSeq)

	// The cards are read before the transaction: the index has one connection,
	// and a read inside the write would wait for itself.
	type advance struct {
		card   core.ID
		seq    int64
		state  schedule.State
		status core.Status
	}
	advances := make([]advance, 0, len(tail))
	for _, ev := range tail {
		row, ok := l.index.cardRow(ev.Card)
		if !ok {
			continue // the card file is gone; its reviews schedule nothing
		}
		state := l.params.Step(row.state, ev)
		advances = append(advances, advance{
			card:   ev.Card,
			seq:    ev.Seq,
			state:  state,
			status: effectiveStatus(row.stored, state),
		})
	}

	err = l.index.write(func(e execer) error {
		for _, next := range advances {
			if err := putState(e, next.card, next.state, next.seq); err != nil {
				return err
			}
			if err := setCardStatus(e, next.card, next.status); err != nil {
				return err
			}
		}
		if err := l.putMarks(e, marks); err != nil {
			return err
		}
		if logOK {
			return l.putLogMark(e, written)
		}
		return clearLogMark(e)
	})
	if err != nil {
		return err
	}
	l.nextSeq = nextSeq
	return nil
}

// appliedLogMark is the log mark to store right after this handle appended one
// event: the file's state now that the line is in it.
func (l *Library) appliedLogMark(seq int64) (logMark, bool) {
	mark, err := l.logFileMark()
	if err != nil {
		return logMark{}, false
	}
	return logMark{fileMark: mark, seq: seq}, true
}

// logMarkAfter is the mark to store once everything up to offset has been
// applied. The size stored is the offset, not the file's: a torn last line — a
// write that never finished — stays past the mark, so the next open re-reads
// those few bytes and ignores them again instead of rebuilding the index for a
// line the log cannot parse. A file that shrank under the read leaves no mark
// at all, so the next open replays it from the beginning.
func (l *Library) logMarkAfter(offset, seq int64) (logMark, bool) {
	mark, err := l.logFileMark()
	if err != nil || mark.size < offset {
		return logMark{}, false
	}
	mark.size = offset
	return logMark{fileMark: mark, seq: seq}, true
}

// loadCatalog reads the index into memory: every card, and the queue order.
func (l *Library) loadCatalog() error {
	rows, err := l.index.cardRows()
	if err != nil {
		return err
	}
	l.cards = make(map[core.ID]*cardMeta, len(rows))
	l.byCreated = make([]core.ID, 0, len(rows))
	l.byDue = make([]dueEntry, 0, len(rows))
	for _, row := range rows {
		meta := &cardMeta{
			card:    row.card,
			stored:  row.stored,
			created: row.created,
			live:    row.live,
			state:   row.state,
			lastSeq: row.lastSeq,
		}
		l.cards[meta.card.ID] = meta
		l.byCreated = append(l.byCreated, meta.card.ID)
		if eligible(meta) {
			l.byDue = append(l.byDue, dueEntry{at: dueNano(meta), id: meta.card.ID})
		}
	}
	sort.Slice(l.byDue, func(i, j int) bool { return dueLess(l.byDue[i], l.byDue[j]) })
	l.dueOK, l.createdOK = true, true
	return nil
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
		l.sources.notes++
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
		l.sources.cards++
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

// Note returns a note and its blocks, as the index read them.
func (l *Library) Note(id core.ID) (core.Note, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return core.Note{}, false
	}
	row, ok := l.index.note(id)
	if !ok {
		return core.Note{}, false
	}
	blocks, err := l.index.blocksOfNote(id)
	if err != nil {
		return core.Note{}, false
	}
	return core.Note{ID: id, Title: row.title, Blocks: blocks, Updated: row.updated}, true
}

// Notes returns every note, newest save first: the notebook lists them that way.
func (l *Library) Notes() []core.Note {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil
	}
	rows, err := l.index.notes()
	if err != nil {
		return nil
	}
	blocks, err := l.index.allBlocks()
	if err != nil {
		return nil
	}
	notes := make([]core.Note, 0, len(rows))
	for _, row := range rows {
		notes = append(notes, core.Note{ID: row.id, Title: row.title, Blocks: blocks[row.id], Updated: row.updated})
	}
	sort.Slice(notes, func(i, j int) bool {
		if !notes[i].Updated.Equal(notes[j].Updated) {
			return notes[i].Updated.After(notes[j].Updated)
		}
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

	entry, exists := l.index.note(note.ID)
	slug := slugify(note.Title)
	path := l.notePath(slug, note.ID)
	if exists {
		// The slug is minted once: a renamed note keeps its path.
		slug, path = entry.slug, l.notePath(entry.slug, note.ID)
	}
	note = normalizeNote(note)
	note.Updated = l.now().UTC()

	before, err := l.index.blocksOfNote(note.ID)
	if err != nil {
		return core.Note{}, err
	}
	if err := writeFileAtomic(path, renderNote(note)); err != nil {
		return core.Note{}, err
	}

	marks, err := l.sourceMarks()
	if err != nil {
		return core.Note{}, err
	}
	moved := movedBlocks(before, note.Blocks)
	err = l.index.write(func(e execer) error {
		if err := putNote(e, noteRow{id: note.ID, slug: slug, title: note.Title, updated: note.Updated}); err != nil {
			return err
		}
		if err := deleteBlocksOfNote(e, note.ID); err != nil {
			return err
		}
		for at, block := range note.Blocks {
			if err := putBlock(e, note.ID, at, block); err != nil {
				return err
			}
		}
		// Links are derived from the note's text, so they are rewritten with it
		// rather than rebuilt from the whole library.
		if err := deleteLinksOfNote(e, note.ID); err != nil {
			return err
		}
		if err := putLinks(e, note.ID, note.Blocks); err != nil {
			return err
		}
		if err := deleteTagsOfNote(e, note.ID); err != nil {
			return err
		}
		if err := putTags(e, note.ID, note.Blocks); err != nil {
			return err
		}
		if err := resolveBlocks(e, moved); err != nil {
			return err
		}
		return l.putMarks(e, marks)
	})
	if err != nil {
		return core.Note{}, err
	}
	if err := l.refreshCards(moved); err != nil {
		return core.Note{}, err
	}
	l.emit(EventNoteSaved, note.ID)
	return cloneNote(note), nil
}

// movedBlocks is the block IDs a note save can have changed the reachability
// of: every card pointing at one of these may have lost or found its source.
func movedBlocks(before, after []core.Block) []core.ID {
	ids := make([]core.ID, 0, len(before)+len(after))
	for _, block := range before {
		if block.ID != "" {
			ids = append(ids, block.ID)
		}
	}
	for _, block := range after {
		if block.ID != "" {
			ids = append(ids, block.ID)
		}
	}
	return ids
}

// refreshCards re-reads the named cards from the index, which is what a note
// save changes: a block that is gone leaves its cards lost, a block that came
// back makes them live.
func (l *Library) refreshCards(blockIDs []core.ID) error {
	if len(blockIDs) == 0 {
		return nil
	}
	ids, err := l.index.cardsOfBlocks(blockIDs)
	if err != nil {
		return err
	}
	rows, err := l.index.cardRowsByID(ids)
	if err != nil {
		return err
	}
	if len(rows) > bulkQueueUpdate {
		l.markQueueStale()
	}
	for _, row := range rows {
		meta := &cardMeta{
			card:    row.card,
			stored:  row.stored,
			created: row.created,
			live:    row.live,
			state:   row.state,
			lastSeq: row.lastSeq,
		}
		l.storeCard(meta)
	}
	return nil
}

// CreateNote makes an empty note with a title.
func (l *Library) CreateNote(title string) (core.Note, error) {
	return l.SaveNote(core.Note{ID: core.NewID(noteKind), Title: title})
}

// Block returns a referencable block, so a card can render itself from the
// library it lives in. The text comes from the index; the note file is read
// only when a caller names the note.
func (l *Library) Block(id core.ID) (core.Block, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return core.Block{}, false
	}
	ref, ok := l.index.block(id)
	if !ok {
		return core.Block{}, false
	}
	return ref.block, true
}

// Card returns a card as its file says, with the lifecycle its log gives it.
// It is the one read that opens the file: a listing, a search or the queue
// answers from the index, and a caller asking for one card gets what is on
// disk, including an edit made by hand since the index was read.
func (l *Library) Card(id core.ID) (core.Card, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return core.Card{}, false
	}
	data, err := os.ReadFile(l.cardPath(id))
	if err != nil {
		return core.Card{}, false
	}
	l.sources.cards++
	card, _, err := decodeCard(id, data)
	if err != nil {
		return core.Card{}, false
	}
	if meta, ok := l.cards[id]; ok {
		card.Status = effectiveStatus(card.Status, meta.state)
	}
	return card, true
}

// Cards returns every card, oldest first.
func (l *Library) Cards() []core.Card {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil
	}
	l.ensureCreated()
	cards := make([]core.Card, 0, len(l.byCreated))
	for _, id := range l.byCreated {
		if meta, ok := l.cards[id]; ok {
			cards = append(cards, meta.card)
		}
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

	meta, exists := l.cards[card.ID]
	created := l.now().UTC()
	stored := card.Status
	if exists {
		created = meta.created
	}
	encoded, err := encodeCard(card, created)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(l.cardPath(card.ID), encoded); err != nil {
		return err
	}

	state := schedule.State{}
	lastSeq := int64(0)
	if exists {
		state, lastSeq = meta.state, meta.lastSeq
	}
	live := card.Ref.Frozen != nil
	if !live {
		_, live = l.index.block(card.Ref.Block)
	}
	row := cardRow{card: card, stored: stored, created: created, live: live, state: state}
	row.card.Status = effectiveStatus(stored, state)
	row.lastSeq = lastSeq

	marks, err := l.sourceMarks()
	if err != nil {
		return err
	}
	err = l.index.write(func(e execer) error {
		if err := putCard(e, row); err != nil {
			return err
		}
		if state.Reviews > 0 {
			if err := putState(e, card.ID, state, lastSeq); err != nil {
				return err
			}
		}
		return l.putMarks(e, marks)
	})
	if err != nil {
		return err
	}
	l.storeCard(&cardMeta{
		card:    row.card,
		stored:  stored,
		created: created,
		live:    live,
		state:   state,
		lastSeq: lastSeq,
	})
	return nil
}

// CreateCards makes the cards a block's text asks for: one for a separator, one
// per blank of a cloze, both directions of a paragraph written both ways. A
// block whose text asks nothing makes no card.
func (l *Library) CreateCards(blockID core.ID, derived []core.Derived) ([]core.Card, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	ref, ok := l.index.block(blockID)
	if !ok {
		return nil, fmt.Errorf("store: no block %q", blockID)
	}
	entry, ok := l.index.note(ref.note)
	if !ok {
		return nil, fmt.Errorf("store: no note %q", ref.note)
	}
	if len(derived) == 0 {
		return nil, ErrNothingToAsk
	}
	if err := l.markBlock(entry.slug, ref); err != nil {
		return nil, err
	}
	marks, err := l.sourceMarks()
	if err != nil {
		return nil, err
	}

	out := make([]core.Card, 0, len(derived))
	for _, d := range derived {
		created := l.now().UTC()
		card := core.Card{
			ID:      core.NewID(cardKind),
			Ref:     core.Reference{Block: blockID},
			Status:  core.StatusNew,
			Kind:    d.Kind,
			Ordinal: d.Ordinal,
		}
		encoded, err := encodeCard(card, created)
		if err != nil {
			return nil, err
		}
		if err := writeFileAtomic(l.cardPath(card.ID), encoded); err != nil {
			return nil, err
		}
		row := cardRow{card: card, stored: card.Status, created: created, live: true}
		err = l.index.write(func(e execer) error {
			if err := putCard(e, row); err != nil {
				return err
			}
			return l.putMarks(e, marks)
		})
		if err != nil {
			return nil, err
		}
		l.storeCard(&cardMeta{card: row.card, stored: card.Status, created: created, live: true})
		l.emit(EventCardCreated, card.ID)
		out = append(out, card)
	}
	return out, nil
}

// markBlock appends a block's marker to its note source, leaving every other
// byte of the file as it was.
func (l *Library) markBlock(slug string, ref blockRef) error {
	path := l.notePath(slug, ref.note)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	l.sources.notes++
	marked, err := withMarker(data, ref.block.ID, ref.block.Text, ref.at)
	if err != nil {
		return err
	}
	if bytes.Equal(marked, data) {
		return nil
	}
	return writeFileAtomic(path, marked)
}

// Review records that a card was reviewed, and schedules it again.
// Scheduler is what schedules this library's reviews now.
func (l *Library) Scheduler() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.scheduler
}

// ScheduleParams is the schedule this library reviews with.
func (l *Library) ScheduleParams() schedule.Params {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.params
}

func (l *Library) Review(cardID core.ID, rating schedule.Rating, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	meta, ok := l.cards[cardID]
	if !ok {
		return fmt.Errorf("store: no card %q", cardID)
	}
	if !rating.Valid() {
		return fmt.Errorf("store: unknown rating %d", uint8(rating))
	}
	event := schedule.Event{Seq: l.nextSeq + 1, Card: cardID, Rating: rating,
		At: l.instant(at), Scheduler: l.scheduler}
	return l.record(event, meta)
}

// UndoLast cancels a card's newest review by appending the void that names it.
// The log is never rewritten, so an undo is as durable as the review it undoes.
func (l *Library) UndoLast(cardID core.ID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	meta, ok := l.cards[cardID]
	if !ok {
		return fmt.Errorf("store: no card %q", cardID)
	}
	if meta.lastSeq == 0 {
		return fmt.Errorf("%w: card %q", ErrNoReview, cardID)
	}
	event := schedule.Event{Seq: l.nextSeq + 1, Void: meta.lastSeq}
	return l.record(event, meta)
}

// record appends an event to the log and brings the card's schedule with it.
// The log line is written first: a review that survives a crash is replayed,
// and one that does not was never confirmed to the caller.
func (l *Library) record(event schedule.Event, meta *cardMeta) error {
	if err := appendLog(l.logPath(), event); err != nil {
		return err
	}

	var (
		state  schedule.State
		status core.Status
		seq    int64
	)
	if event.IsVoid() {
		// An undo needs the card's whole history. Read it before the write, so
		// the void that was just appended is in it.
		history, last, err := l.cardHistory(meta.card.ID)
		if err != nil {
			return err
		}
		state, seq = history, last
	} else {
		state = l.params.Step(meta.state, event)
		seq = event.Seq
	}
	status = effectiveStatus(meta.stored, state)

	marks, err := l.sourceMarks()
	if err != nil {
		return err
	}
	applied, ok := l.appliedLogMark(event.Seq)
	err = l.index.write(func(e execer) error {
		if state.Reviews == 0 {
			if err := deleteState(e, meta.card.ID); err != nil {
				return err
			}
		} else if err := putState(e, meta.card.ID, state, seq); err != nil {
			return err
		}
		if err := setCardStatus(e, meta.card.ID, status); err != nil {
			return err
		}
		if err := l.putMarks(e, marks); err != nil {
			return err
		}
		if !ok {
			return clearLogMark(e)
		}
		return l.putLogMark(e, applied)
	})
	if err != nil {
		return err
	}

	l.nextSeq = event.Seq
	updated := *meta
	updated.state = state
	updated.lastSeq = seq
	updated.card.Status = status
	l.storeCard(&updated)
	l.emit(EventReviewRecorded, meta.card.ID)
	return nil
}

// cardHistory replays one card's live reviews out of the log, and names the
// newest of them: the sequence the next undo cancels.
func (l *Library) cardHistory(cardID core.ID) (schedule.State, int64, error) {
	var events []schedule.Event
	if _, err := l.streamLog(0, func(ev schedule.Event) error {
		if ev.IsVoid() || ev.Card == cardID {
			events = append(events, ev)
		}
		return nil
	}); err != nil {
		return schedule.State{}, 0, err
	}
	applier := schedule.NewApplier(l.params)
	if err := applier.Apply(events...); err != nil {
		return schedule.State{}, 0, fmt.Errorf("store: review log: %w", err)
	}
	state, _ := applier.State(cardID)
	last, _ := applier.Last(cardID)
	return state, last.Seq, nil
}

// Events returns the review log in order, void events included, so a caller
// can compute what the schedule cannot answer on its own (counts per day,
// predictions against outcomes). Every call reads the log: a caller that only
// wants a range asks LiveReviews, and one that wants one card asks CardHistory.
func (l *Library) Events() []schedule.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil
	}
	events, _, err := l.readLogFrom(0)
	if err != nil {
		return nil
	}
	return events
}

// LiveReviews returns the reviews in [from, to) that the log still counts:
// void events and the reviews they cancel are left out. A zero bound is open —
// LiveReviews(time.Time{}, time.Time{}) is the whole live history.
func (l *Library) LiveReviews(from, to time.Time) ([]schedule.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	events, _, err := l.readLogFrom(0)
	if err != nil {
		return nil, err
	}
	return liveBetween(events, from, to), nil
}

// CardHistory returns one card's live reviews, oldest first: the reviews an
// undo or a "what happened" view asks for.
func (l *Library) CardHistory(cardID core.ID) ([]schedule.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	events, _, err := l.readLogFrom(0)
	if err != nil {
		return nil, err
	}
	live := liveBetween(events, time.Time{}, time.Time{})
	out := make([]schedule.Event, 0, 4)
	for _, ev := range live {
		if ev.Card == cardID {
			out = append(out, ev)
		}
	}
	return out, nil
}

// CardState returns a card's replayed schedule, and whether it has been
// reviewed at all.
// Preview is what each of the four answers would schedule for a card, as the
// grade keys show it before the user picks one — the scheduling interface's own
// answer, from the parameters this library reviews with. It changes nothing.
func (l *Library) Preview(cardID core.ID, now time.Time) ([]schedule.Outlook, bool) {
	l.mu.Lock()
	meta, ok := l.cards[cardID]
	params := l.params
	l.mu.Unlock()
	if !ok {
		return nil, false
	}
	return params.Preview(meta.state, now), true
}

func (l *Library) CardState(cardID core.ID) (schedule.State, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	meta, ok := l.cards[cardID]
	if !ok || meta.state.Reviews == 0 {
		return schedule.State{}, false
	}
	return meta.state, true
}

// CountReviewed reports how many cards have a live review in [from, to). The
// queue asks it for the day's new-card allowance: a card counts as started on
// the day of its newest review.
func (l *Library) CountReviewed(from, to time.Time) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return 0, ErrClosed
	}
	count := 0
	for _, meta := range l.cards {
		if meta.state.Reviews == 0 {
			continue
		}
		at := meta.state.LastReview
		if at.Before(from) || !at.Before(to) {
			continue
		}
		count++
	}
	return count, nil
}

// Scheduled is a card the queue can show, with the schedule its log gives it.
// State is zero for a card that has never been reviewed.
type Scheduled struct {
	Card  core.Card
	State schedule.State
}

// Scheduled returns the cards a queue can hold — stored as new, learning or
// review, with a live or frozen source — soonest first, each with its schedule.
// It is what the queue and the forecast read: one pass over the index's answer,
// with no card file opened and no log line replayed.
func (l *Library) Scheduled() ([]Scheduled, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	l.ensureDue()
	out := make([]Scheduled, 0, len(l.byDue))
	for _, entry := range l.byDue {
		if meta, ok := l.cards[entry.id]; ok {
			out = append(out, Scheduled{Card: meta.card, State: meta.state})
		}
	}
	return out, nil
}

// Due returns the cards to review now, soonest first. A limit of zero or less
// returns the whole queue.
func (l *Library) Due(now time.Time, limit int) ([]core.Card, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	cut := l.dueBefore(l.instant(now))
	if limit > 0 && cut > limit {
		cut = limit
	}
	cards := make([]core.Card, 0, cut)
	for _, entry := range l.byDue[:cut] {
		if meta, ok := l.cards[entry.id]; ok {
			cards = append(cards, meta.card)
		}
	}
	return cards, nil
}

// Remaining counts the cards due now, without materialising them.
func (l *Library) Remaining(now time.Time) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return 0, ErrClosed
	}
	return l.dueBefore(l.instant(now)), nil
}

// LostCards returns the cards whose block is gone and which kept no snapshot,
// so the UI can offer to re-point them. They keep their schedule meanwhile.
func (l *Library) LostCards() ([]core.Card, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, ErrClosed
	}
	l.ensureCreated()
	var cards []core.Card
	for _, id := range l.byCreated {
		meta, ok := l.cards[id]
		if !ok || meta.live {
			continue
		}
		cards = append(cards, meta.card)
	}
	return cards, nil
}

// effective is the lifecycle a card is in. A review never rewrites a card file
// (ADR 0003), so the step out of "new" is derived from the log: a card with live
// reviews has been learned, and the last rating says whether it is still being
// learned.
func effectiveStatus(stored core.Status, state schedule.State) core.Status {
	if stored != core.StatusNew && stored != core.StatusLearning {
		return stored
	}
	if state.Reviews == 0 {
		return stored
	}
	if state.LastRating == schedule.Again {
		return core.StatusLearning
	}
	return core.StatusReview
}

// eligible reports whether a card belongs in the review queue.
func eligible(meta *cardMeta) bool {
	return meta.card.Status.Scheduled() && meta.live
}

// dueNano is when a card comes up: its schedule's due time, or the moment it
// was created when it has never been reviewed.
func dueNano(meta *cardMeta) int64 {
	if meta.state.Reviews > 0 {
		return meta.state.Due.UnixNano()
	}
	return meta.created.UnixNano()
}

func dueLess(a, b dueEntry) bool {
	if a.at != b.at {
		return a.at < b.at
	}
	return a.id < b.id
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
func encodeConfig(params schedule.Params, extensions []string) ([]byte, error) {
	data, err := json.MarshalIndent(configFile{Format: formatVersion, FSRS: params, Extensions: extensions}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: %s: %w", configName, err)
	}
	return append(data, '\n'), nil
}

func decodeConfig(data []byte) (schedule.Params, []string, error) {
	var config configFile
	if err := json.Unmarshal(data, &config); err != nil {
		return schedule.Params{}, nil, fmt.Errorf("store: %s: %w", configName, err)
	}
	if config.Format != formatVersion {
		return schedule.Params{}, nil, fmt.Errorf("store: %s: format %q, want %q", configName, config.Format, formatVersion)
	}
	return config.FSRS.Normalized(), config.Extensions, nil
}

// paramsDigest fingerprints the scheduling parameters: a library that changed
// its FSRS weights has to be replayed, because every due date moves with them.
func paramsDigest(params schedule.Params) string {
	data, err := json.Marshal(params.Normalized())
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

// Event is something that happened to the library, which an extension may want
// to see. It carries an ID and nothing else: an extension reads the rest
// through the agent API.
type Event struct {
	Kind string
	ID   core.ID
}

const (
	// EventNoteSaved fires after a note's file is written.
	EventNoteSaved = "note.saved"
	// EventCardCreated fires after a card file is written.
	EventCardCreated = "card.created"
	// EventReviewRecorded fires after a review enters the log.
	EventReviewRecorded = "review.recorded"
)

// SetEventHandler registers the function told about events, replacing any
// previous one. The handler runs on the writing thread, so it must not block.
func (l *Library) SetEventHandler(fn func(Event)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onEvent = fn
}

// EnabledExtensions lists the extensions this library enabled, by name.
func (l *Library) EnabledExtensions() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.extensions...)
}

// SetEnabledExtensions records which extensions this library runs, in
// remit.json. Enabling is per library and deliberate: an extension installed
// on the machine does nothing until a library names it.
func (l *Library) SetEnabledExtensions(names []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return ErrClosed
	}
	data, err := encodeConfig(l.params, names)
	if err != nil {
		return err
	}
	path := filepath.Join(l.dir, configName)
	if err := writeFileAtomic(path, data); err != nil {
		return err
	}
	l.extensions = append([]string(nil), names...)
	return nil
}

// emit tells the handler about an event, if one is registered.
func (l *Library) emit(kind string, id core.ID) {
	if l.onEvent == nil {
		return
	}
	l.onEvent(Event{Kind: kind, ID: id})
}

// firstNonEmpty is the first of the values that has anything in it.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
