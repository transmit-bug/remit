package importanki_test

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/ext"
	"github.com/transmit-bug/remit/internal/ext/builtin"
)

// fixedNow is the library's clock, so a created card reads the same on every
// machine.
var fixedNow = time.Date(2026, 11, 6, 8, 0, 0, 0, time.UTC)

// t0 is the first review moment in the fixtures.
var t0 = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

func newLibrary(t *testing.T) (*store.Library, string) {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return fixedNow }})
	if err != nil {
		t.Fatalf("store.Create: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib, dir
}

// The fixture is built from the collection schema Anki documents for schema
// 11: col, notes, cards and revlog.
const schemaCol = `CREATE TABLE col (
	id      integer primary key,
	crt     integer not null,
	mod     integer not null,
	scm     integer not null,
	ver     integer not null,
	dty     integer not null,
	usn     integer not null,
	ls      integer not null,
	conf    text not null,
	models  text not null,
	decks   text not null,
	dconf   text not null,
	tags    text not null
)`

const schemaNotes = `CREATE TABLE notes (
	id      integer primary key,
	guid    text not null,
	mid     integer not null,
	mod     integer not null,
	usn     integer not null,
	tags    text not null,
	flds    text not null,
	sfld    integer not null,
	csum    integer not null,
	flags   integer not null,
	data    text not null
)`

const schemaCards = `CREATE TABLE cards (
	id      integer primary key,
	nid     integer not null,
	did     integer not null,
	ord     integer not null,
	mod     integer not null,
	usn     integer not null,
	type    integer not null,
	queue   integer not null,
	due     integer not null,
	ivl     integer not null,
	factor  integer not null,
	reps    integer not null,
	lapses  integer not null,
	left    integer not null,
	odue    integer not null,
	odid    integer not null,
	flags   integer not null,
	data    text not null
)`

const schemaRevlog = `CREATE TABLE revlog (
	id      integer primary key,
	cid     integer not null,
	usn     integer not null,
	ease    integer not null,
	ivl     integer not null,
	lastIvl integer not null,
	factor  integer not null,
	time    integer not null,
	type    integer not null
)`

type fixture struct {
	models     map[string]any
	decks      map[string]any
	notes      []noteRow
	cards      []cardRow
	revlog     []revlogRow
	media      map[string]string // zip entry -> file name
	mediaFiles map[string]string // zip entry -> contents
	omit       []string          // tables to leave out
	collection string            // zip entry to use for the database
}

type noteRow struct {
	id     int64
	guid   string
	mid    int64
	tags   string
	fields []string
}

type cardRow struct {
	id    int64
	note  int64
	deck  int64
	ord   int
	kind  int
	queue int
}

type revlogRow struct {
	id   int64
	card int64
	ease int
	kind int
}

// buildPackage writes a real SQLite collection into a real zip, so the test
// exercises archive/zip and the SQLite driver the same way an .apkg does.
func buildPackage(t *testing.T, f fixture) string {
	t.Helper()
	collection := buildLegacyDB(t, f)
	path := filepath.Join(t.TempDir(), "deck.apkg")
	writePackage(t, path, func(zw *zip.Writer) {
		entry := f.collection
		if entry == "" {
			entry = "collection.anki2"
		}
		writeEntry(t, zw, entry, collection)
		writeMedia(t, zw, f.media, f.mediaFiles)
	})
	return path
}

// buildLegacyDB writes the schema 11 collection Anki wrote, with its note
// types and decks as JSON in the col table, and returns the database bytes.
func buildLegacyDB(t *testing.T, f fixture) []byte {
	t.Helper()
	omit := make(map[string]bool, len(f.omit))
	for _, table := range f.omit {
		omit[table] = true
	}

	dbPath := filepath.Join(t.TempDir(), "collection.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	for table, schema := range map[string]string{"col": schemaCol, "notes": schemaNotes, "cards": schemaCards, "revlog": schemaRevlog} {
		if omit[table] {
			continue
		}
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("create %s: %v", table, err)
		}
	}

	models, err := json.Marshal(f.models)
	if err != nil {
		t.Fatalf("models: %v", err)
	}
	decks, err := json.Marshal(f.decks)
	if err != nil {
		t.Fatalf("decks: %v", err)
	}
	if !omit["col"] {
		if _, err := db.Exec(
			`INSERT INTO col (id, crt, mod, scm, ver, dty, usn, ls, conf, models, decks, dconf, tags)
			 VALUES (1, 0, 0, 0, 11, 0, 0, 0, '{}', ?, ?, '{}', '{}')`,
			string(models), string(decks)); err != nil {
			t.Fatalf("insert col: %v", err)
		}
	}
	insertRows(t, db, f, omit)
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture database: %v", err)
	}
	collection, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read fixture database: %v", err)
	}
	return collection
}

// insertRows writes the notes, cards and revlog rows both collection schemas
// share, skipping the tables the fixture leaves out.
func insertRows(t *testing.T, db *sql.DB, f fixture, omit map[string]bool) {
	t.Helper()
	if !omit["notes"] {
		for _, note := range f.notes {
			if _, err := db.Exec(
				`INSERT INTO notes (id, guid, mid, mod, usn, tags, flds, sfld, csum, flags, data)
				 VALUES (?, ?, ?, 0, 0, ?, ?, 0, 0, 0, '')`,
				note.id, note.guid, note.mid, note.tags, strings.Join(note.fields, "\x1f")); err != nil {
				t.Fatalf("insert note %d: %v", note.id, err)
			}
		}
	}
	if !omit["cards"] {
		for _, card := range f.cards {
			if _, err := db.Exec(
				`INSERT INTO cards (id, nid, did, ord, mod, usn, type, queue, due, ivl, factor, reps, lapses, left, odue, odid, flags, data)
				 VALUES (?, ?, ?, ?, 0, 0, ?, ?, 0, 0, 2500, 0, 0, 0, 0, 0, 0, '')`,
				card.id, card.note, card.deck, card.ord, card.kind, card.queue); err != nil {
				t.Fatalf("insert card %d: %v", card.id, err)
			}
		}
	}
	if !omit["revlog"] {
		for _, row := range f.revlog {
			if _, err := db.Exec(
				`INSERT INTO revlog (id, cid, usn, ease, ivl, lastIvl, factor, time, type)
				 VALUES (?, ?, 0, ?, 0, 0, 2500, 0, ?)`,
				row.id, row.card, row.ease, row.kind); err != nil {
				t.Fatalf("insert revlog %d: %v", row.id, err)
			}
		}
	}
}

// writePackage creates the zip at path, lets add fill it, and closes it.
func writePackage(t *testing.T, path string, add func(*zip.Writer)) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	zw := zip.NewWriter(file)
	add(zw)
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close package: %v", err)
	}
}

// writeMedia writes the package's media map and the files it names.
func writeMedia(t *testing.T, zw *zip.Writer, media, files map[string]string) {
	t.Helper()
	if media != nil {
		manifest, err := json.Marshal(media)
		if err != nil {
			t.Fatalf("media map: %v", err)
		}
		writeEntry(t, zw, "media", manifest)
	}
	for name, contents := range files {
		writeEntry(t, zw, name, []byte(contents))
	}
}

func writeEntry(t *testing.T, zw *zip.Writer, name string, data []byte) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip entry %s: %v", name, err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("zip entry %s: %v", name, err)
	}
}

// basicModels is Anki's Basic and Basic (optional reversed card) shapes, with
// the conditional reverse template and a cloze notetype beside them.
func basicModels() map[string]any {
	fields := func(names ...string) []any {
		out := make([]any, len(names))
		for i, name := range names {
			out[i] = map[string]any{"name": name, "ord": i}
		}
		return out
	}
	return map[string]any{
		"1000": map[string]any{
			"id": 1000, "name": "Basic", "type": 0,
			"flds": fields("Front", "Back", "Add Reverse"),
			"tmpls": []any{
				map[string]any{
					"name": "Card 1", "ord": 0, "qfmt": "{{Front}}",
					"afmt": "{{FrontSide}}\n<hr id=answer>\n{{Back}}",
				},
				map[string]any{
					"name": "Card 2", "ord": 1,
					"qfmt": "{{#Add Reverse}}{{Back}}{{/Add Reverse}}",
					"afmt": "{{FrontSide}}\n<hr id=answer>\n{{Front}}",
				},
			},
		},
		"2000": map[string]any{
			"id": 2000, "name": "Cloze", "type": 1,
			"flds": fields("Text", "Extra"),
			"tmpls": []any{
				map[string]any{
					"name": "Cloze", "ord": 0, "qfmt": "{{cloze:Text}}",
					"afmt": "{{cloze:Text}}<br>{{Extra}}",
				},
			},
		},
	}
}

func basicDecks() map[string]any {
	return map[string]any{
		"1":  map[string]any{"id": 1, "name": "Default"},
		"10": map[string]any{"id": 10, "name": "Geography"},
	}
}

// minimalFixture has 2 notes, 3 cards and 5 graded revlog rows: two reviews on
// each of two cards, one on the third.
func minimalFixture() fixture {
	return fixture{
		models: basicModels(),
		decks:  basicDecks(),
		notes: []noteRow{
			{id: 1001, guid: "guid-france", mid: 1000, fields: []string{"Capital of France?", "Paris"}},
			{id: 1002, guid: "guid-japan", mid: 1000, fields: []string{"Capital of Japan?", "Tokyo", "y"}},
		},
		cards: []cardRow{
			{id: 101, note: 1001, deck: 10, ord: 0},
			{id: 201, note: 1002, deck: 10, ord: 0},
			{id: 202, note: 1002, deck: 10, ord: 1},
		},
		revlog: []revlogRow{
			{id: t0.UnixMilli(), card: 101, ease: 3, kind: 1},
			{id: t0.Add(24 * time.Hour).UnixMilli(), card: 201, ease: 1, kind: 1},
			{id: t0.Add(48 * time.Hour).UnixMilli(), card: 201, ease: 3, kind: 2},
			{id: t0.Add(72 * time.Hour).UnixMilli(), card: 101, ease: 4, kind: 1},
			{id: t0.Add(96 * time.Hour).UnixMilli(), card: 202, ease: 3, kind: 0},
		},
	}
}

type logRecord struct {
	Seq    int64  `json:"seq"`
	Card   string `json:"card"`
	Rating string `json:"rating"`
	At     string `json:"at"`
}

func readReviews(t *testing.T, dir string) []logRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "reviews.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read reviews.jsonl: %v", err)
	}
	var records []logRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var record logRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("review log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

// resolver opens the shared resolver over the built-in plugin, the same door
// the app reads a card through.
func resolver(t *testing.T, lib *store.Library) *cards.Resolver {
	t.Helper()
	runner, err := builtin.Cards(nil, ext.JSLimits{})
	if err != nil {
		t.Fatalf("open the card plugin: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	return cards.New(runner, lib.Types())
}

// blocksOf is the library's blocks as the model reads them.
func blocksOf(lib *store.Library) core.BlockSet {
	blocks := core.BlockSet{}
	for _, note := range lib.Notes() {
		for _, block := range note.Blocks {
			blocks[block.ID] = block
		}
	}
	return blocks
}

// cardsByPrompt groups a library's cards by the face the resolver renders for
// them: the shapes belong to the plugin and the type's templates, so a test
// asks rather than splitting text itself.
func cardsByPrompt(t *testing.T, lib *store.Library) map[string]core.Card {
	t.Helper()
	faces := resolver(t, lib)
	cards := make(map[string]core.Card)
	for _, card := range lib.Cards() {
		cards[faces.Face(card, blocksOf(lib)).Front] = card
	}
	return cards
}

func ratingsFor(records []logRecord, card core.ID) []string {
	var ratings []string
	for _, record := range records {
		if record.Card == string(card) {
			ratings = append(ratings, record.Rating)
		}
	}
	return ratings
}

func hasWarning(report importanki.Report, want string) bool {
	for _, warning := range report.Warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}

// cardFaces is what a card asks, as the resolver renders it: the shapes are
// the plugin's and the type's, so a test asks rather than splitting text
// itself.
func cardFaces(t *testing.T, lib *store.Library, card core.Card) (prompt, answer string, state core.LinkState) {
	t.Helper()
	got := resolver(t, lib).Face(card, blocksOf(lib))
	return got.Front, got.Back, got.State
}
