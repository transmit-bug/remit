package importanki_test

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/store"
)

// The schema a current Anki writes: the col table keeps its legacy columns
// (empty JSON) and note types, fields, templates and decks move into their own
// tables. A notetype's kind and a template's two formats live in protobuf
// config blobs.
const (
	schemaCol18 = `CREATE TABLE col (
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

	schemaNotetypes18 = `CREATE TABLE notetypes (
	id         integer primary key not null,
	name       text not null,
	mtime_secs integer not null,
	usn        integer not null,
	config     blob not null
)`

	schemaFields18 = `CREATE TABLE fields (
	ntid   integer not null,
	ord    integer not null,
	name   text not null,
	config blob not null,
	primary key (ntid, ord)
)`

	schemaTemplates18 = `CREATE TABLE templates (
	ntid       integer not null,
	ord        integer not null,
	name       text not null,
	mtime_secs integer not null,
	usn        integer not null,
	config     blob not null,
	primary key (ntid, ord)
)`

	schemaDecks18 = `CREATE TABLE decks (
	id         integer primary key not null,
	name       text not null,
	mtime_secs integer not null,
	usn        integer not null,
	common     blob not null,
	kind       blob not null
)`
)

// notetype18 is one schema 18 notetype as the fixture writes it.
type notetype18 struct {
	id        int64
	name      string
	kind      int // 1 is cloze
	fields    []string
	templates []template18
}

type template18 struct {
	name  string
	front string
	back  string
}

type deck18 struct {
	id   int64
	name string
}

// package18 is a package whose newest entry is a zstd-compressed schema 18
// collection, with any legacy entry a test wants beside it.
type package18 struct {
	notetypes  []notetype18
	decks      []deck18
	notes      []noteRow
	cards      []cardRow
	revlog     []revlogRow
	media      map[string]string
	mediaFiles map[string]string
	// payload replaces the database bytes before compression, so a test can
	// hand the importer a zstd entry that is not SQLite.
	payload []byte
	// legacy, when set, is written as collection.anki21 beside the newest
	// entry, the way an export with legacy support does.
	legacy *fixture
}

// buildPackage18 writes a real schema 18 SQLite collection, compresses it the
// way a current export does, and puts it in a real zip.
func buildPackage18(t *testing.T, f package18) string {
	t.Helper()
	payload := f.payload
	if payload == nil {
		payload = buildDB18(t, f)
	}
	path := filepath.Join(t.TempDir(), "deck.apkg")
	writePackage(t, path, func(zw *zip.Writer) {
		writeEntry(t, zw, "collection.anki21b", zstdCompress(t, payload))
		if f.legacy != nil {
			writeEntry(t, zw, "collection.anki21", buildLegacyDB(t, *f.legacy))
		}
		writeMedia(t, zw, f.media, f.mediaFiles)
	})
	return path
}

// buildDB18 writes the schema 18 collection and returns its bytes.
func buildDB18(t *testing.T, f package18) []byte {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "collection.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	for _, schema := range []string{
		schemaCol18, schemaNotes, schemaCards, schemaRevlog,
		schemaNotetypes18, schemaFields18, schemaTemplates18, schemaDecks18,
	} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("create schema 18 table: %v", err)
		}
	}
	// Schema 18 keeps the col row with empty JSON, which a reader must ignore.
	if _, err := db.Exec(
		`INSERT INTO col (id, crt, mod, scm, ver, dty, usn, ls, conf, models, decks, dconf, tags)
		 VALUES (1, 0, 0, 0, 18, 0, 0, 0, '{}', '{}', '{}', '{}', '{}')`); err != nil {
		t.Fatalf("insert col: %v", err)
	}
	for _, nt := range f.notetypes {
		if _, err := db.Exec(
			`INSERT INTO notetypes (id, name, mtime_secs, usn, config) VALUES (?, ?, 0, 0, ?)`,
			nt.id, nt.name, notetypeConfig18(nt.kind)); err != nil {
			t.Fatalf("insert notetype %d: %v", nt.id, err)
		}
		for ord, name := range nt.fields {
			if _, err := db.Exec(
				`INSERT INTO fields (ntid, ord, name, config) VALUES (?, ?, ?, x'')`,
				nt.id, ord, name); err != nil {
				t.Fatalf("insert field %q: %v", name, err)
			}
		}
		for ord, tmpl := range nt.templates {
			if _, err := db.Exec(
				`INSERT INTO templates (ntid, ord, name, mtime_secs, usn, config) VALUES (?, ?, ?, 0, 0, ?)`,
				nt.id, ord, tmpl.name, templateConfig18(tmpl.front, tmpl.back)); err != nil {
				t.Fatalf("insert template %q: %v", tmpl.name, err)
			}
		}
	}
	for _, deck := range f.decks {
		if _, err := db.Exec(
			`INSERT INTO decks (id, name, mtime_secs, usn, common, kind) VALUES (?, ?, 0, 0, x'', x'')`,
			deck.id, deck.name); err != nil {
			t.Fatalf("insert deck %q: %v", deck.name, err)
		}
	}
	insertRows(t, db, fixture{notes: f.notes, cards: f.cards, revlog: f.revlog}, nil)
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture database: %v", err)
	}
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read fixture database: %v", err)
	}
	return data
}

// zstdCompress compresses a collection the way a current Anki export does.
func zstdCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd writer: %v", err)
	}
	if _, err := enc.Write(data); err != nil {
		t.Fatalf("zstd write: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("zstd close: %v", err)
	}
	return buf.Bytes()
}

// notetypeConfig18 encodes the one field this importer reads from
// anki.notetypes.Notetype.Config: the kind, as a varint.
func notetypeConfig18(kind int) []byte {
	if kind == 0 {
		return []byte{}
	}
	return protobufVarint18(1, uint64(kind))
}

// templateConfig18 encodes anki.notetypes.Notetype.Template.Config's two
// formats, which is all a card side needs.
func templateConfig18(front, back string) []byte {
	config := protobufBytes18(1, front)
	return append(config, protobufBytes18(2, back)...)
}

// protobufBytes18 encodes one length-delimited protobuf field.
func protobufBytes18(field int, value string) []byte {
	out := protobufVarint18Bytes(uint64(field)<<3 | 2)
	out = append(out, protobufVarint18Bytes(uint64(len(value)))...)
	return append(out, value...)
}

// protobufVarint18 encodes one varint protobuf field.
func protobufVarint18(field int, value uint64) []byte {
	out := protobufVarint18Bytes(uint64(field) << 3)
	return append(out, protobufVarint18Bytes(value)...)
}

func protobufVarint18Bytes(value uint64) []byte {
	var out []byte
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

// basicNotetypes18 is Anki's Basic and Cloze shapes in the split tables: what
// basicModels spells as JSON, with qfmt and afmt in the template config.
func basicNotetypes18() []notetype18 {
	return []notetype18{
		{
			id: 1000, name: "Basic",
			fields: []string{"Front", "Back", "Add Reverse"},
			templates: []template18{
				{
					name:  "Card 1",
					front: "{{Front}}",
					back:  "{{FrontSide}}\n<hr id=answer>\n{{Back}}",
				},
				{
					name:  "Card 2",
					front: "{{#Add Reverse}}{{Back}}{{/Add Reverse}}",
					back:  "{{FrontSide}}\n<hr id=answer>\n{{Front}}",
				},
			},
		},
		{
			id: 2000, name: "Cloze", kind: 1,
			fields: []string{"Text", "Extra"},
			templates: []template18{
				{name: "Cloze", front: "{{cloze:Text}}", back: "{{cloze:Text}}<br>{{Extra}}"},
			},
		},
	}
}

func schema18Fixture() package18 {
	minimal := minimalFixture()
	return package18{
		notetypes: basicNotetypes18(),
		decks:     []deck18{{id: 1, name: "Default"}, {id: 10, name: "Geography"}},
		notes:     minimal.notes,
		cards:     minimal.cards,
		revlog:    minimal.revlog,
	}
}

// TestSchema18MatchesLegacy is the parity claim of the two schemas: the same
// deck read from a legacy collection and from a zstd schema 18 collection
// yields the same notes, faces, statuses, review history and due dates, and
// importing the other schema afterwards finds everything it would have added.
func TestSchema18MatchesLegacy(t *testing.T) {
	legacyPath := buildPackage(t, minimalFixture())
	schemaPath := buildPackage18(t, schema18Fixture())

	legacyLib, legacyDir := newLibrary(t)
	legacyReport, err := importanki.Import(legacyPath, legacyLib)
	if err != nil {
		t.Fatalf("Import legacy: %v", err)
	}
	schemaLib, schemaDir := newLibrary(t)
	schemaReport, err := importanki.Import(schemaPath, schemaLib)
	if err != nil {
		t.Fatalf("Import schema 18: %v", err)
	}

	if legacyReport.Notes != schemaReport.Notes || legacyReport.Cards != schemaReport.Cards ||
		legacyReport.Reviews != schemaReport.Reviews || legacyReport.SkippedRows != schemaReport.SkippedRows {
		t.Fatalf("legacy report = %+v, schema 18 report = %+v", legacyReport, schemaReport)
	}

	legacyCards := cardsByPrompt(t, legacyLib)
	schemaCards := cardsByPrompt(t, schemaLib)
	if len(legacyCards) != len(schemaCards) {
		t.Fatalf("legacy library has %d prompts, schema 18 has %d", len(legacyCards), len(schemaCards))
	}
	for prompt, legacyCard := range legacyCards {
		schemaCard, ok := schemaCards[prompt]
		if !ok {
			t.Fatalf("schema 18 has no card %q; cards are %v", prompt, prompts(t, schemaLib))
		}
		if legacyCard.ID != schemaCard.ID {
			t.Errorf("card %q identity: legacy %q, schema 18 %q", prompt, legacyCard.ID, schemaCard.ID)
		}
		if schemaCard.Type != legacyCard.Type || schemaCard.Status != legacyCard.Status ||
			schemaCard.Edited != legacyCard.Edited || !reflect.DeepEqual(schemaCard.Fields, legacyCard.Fields) {
			t.Errorf("card %q: legacy %+v, schema 18 %+v", prompt, legacyCard, schemaCard)
		}
		legacyState, legacyOK := legacyLib.CardState(legacyCard.ID)
		schemaState, schemaOK := schemaLib.CardState(schemaCard.ID)
		if legacyOK != schemaOK || !reflect.DeepEqual(legacyState, schemaState) {
			t.Errorf("card %q state: legacy (%+v, %v), schema 18 (%+v, %v)",
				prompt, legacyState, legacyOK, schemaState, schemaOK)
		}
		legacyPrompt, legacyAnswer, legacyLink := cardFaces(t, legacyLib, legacyCard)
		schemaPrompt, schemaAnswer, schemaLink := cardFaces(t, schemaLib, schemaCard)
		if legacyPrompt != schemaPrompt || legacyAnswer != schemaAnswer || legacyLink != schemaLink {
			t.Errorf("card %q faces: legacy (%q, %q, %v), schema 18 (%q, %q, %v)",
				prompt, legacyPrompt, legacyAnswer, legacyLink, schemaPrompt, schemaAnswer, schemaLink)
		}
	}

	legacyLog, schemaLog := readReviews(t, legacyDir), readReviews(t, schemaDir)
	if !reflect.DeepEqual(legacyLog, schemaLog) {
		t.Errorf("review log: legacy %+v, schema 18 %+v", legacyLog, schemaLog)
	}

	// The same deck in the other schema is the same Remit library.
	again, err := importanki.Import(schemaPath, legacyLib)
	if err != nil {
		t.Fatalf("Import schema 18 into the legacy library: %v", err)
	}
	if again.Notes != 0 || again.Cards != 0 || again.Reviews != 0 {
		t.Errorf("importing the other schema wrote %+v, want nothing", again)
	}
}

// TestImportSchema18Collection maps the split tables onto Remit: ordered
// fields and both templates of a notetype, the deck name, a cloze notetype's
// cards, and the revlog ratings. A package with no media entry names no media.
func TestImportSchema18Collection(t *testing.T) {
	lib, dir := newLibrary(t)
	// Basic (and reversed card) has two fields and two templates. The reverse
	// template names {{Deck}}, so its answer proves the decks table was read
	// rather than the deck number guessed.
	notetypes := []notetype18{
		{
			id: 1000, name: "Basic",
			fields: []string{"Front", "Back"},
			templates: []template18{
				{name: "Card 1", front: "{{Front}}", back: "{{FrontSide}}\n<hr id=answer>\n{{Back}}"},
				{name: "Card 2", front: "{{Back}}", back: "{{FrontSide}}<hr id=answer>{{Front}} [{{Deck}}]"},
			},
		},
		{
			id: 2000, name: "Cloze", kind: 1,
			fields:    []string{"Text", "Extra"},
			templates: []template18{{name: "Cloze", front: "{{cloze:Text}}", back: "{{cloze:Text}}<br>{{Extra}}"}},
		},
	}
	f := package18{
		notetypes: notetypes,
		decks:     []deck18{{id: 10, name: "Geography"}},
		notes: []noteRow{
			{id: 1001, guid: "guid-france", mid: 1000, fields: []string{"Capital of France?", "Paris"}},
			{id: 1002, guid: "guid-cloze", mid: 2000, fields: []string{"{{c1::Paris}} is in {{c2::France}}", "Europe"}},
		},
		cards: []cardRow{
			{id: 101, note: 1001, deck: 10, ord: 0},
			{id: 102, note: 1001, deck: 10, ord: 1},
			{id: 201, note: 1002, deck: 10, ord: 1},
			{id: 202, note: 1002, deck: 10, ord: 2},
		},
		revlog: []revlogRow{
			{id: t0.UnixMilli(), card: 101, ease: 3, kind: 1},
			{id: t0.Add(24 * time.Hour).UnixMilli(), card: 201, ease: 1, kind: 1},
			{id: t0.Add(48 * time.Hour).UnixMilli(), card: 202, ease: 4, kind: 1},
		},
	}
	path := buildPackage18(t, f)

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 2 || report.Cards != 4 || report.Reviews != 3 {
		t.Fatalf("report = %+v, want 2 notes, 4 cards and 3 reviews", report)
	}
	if report.SkippedRows != 0 || report.UnreachableMedia != 0 {
		t.Errorf("skipped rows = %d, unreachable media = %d, want 0 and 0",
			report.SkippedRows, report.UnreachableMedia)
	}
	for _, unwanted := range []string{"media", "deck"} {
		if hasWarning(report, unwanted) {
			t.Errorf("warnings mention %q: %q", unwanted, report.Warnings)
		}
	}

	notes := lib.Notes()
	if len(notes) != 2 {
		t.Fatalf("library holds %d notes, want 2", len(notes))
	}
	clozeNote := noteByTitle(t, lib, "Paris is in France")
	if want := []string{"Paris is in France", "Europe"}; !reflect.DeepEqual(blockTexts(clozeNote), want) {
		t.Errorf("cloze note blocks = %q, want %q", blockTexts(clozeNote), want)
	}

	cards := cardsByPrompt(t, lib)
	answers := map[string]string{
		"Capital of France?": "Paris",
		"Paris":              "Capital of France? [Geography]", // the second template and the deck table
		"[...] is in France": "Paris is in France",
		"Paris is in [...]":  "Paris is in France",
	}
	for prompt, want := range answers {
		card, ok := cards[prompt]
		if !ok {
			t.Fatalf("no card has %q as its prompt; cards are %v", prompt, prompts(t, lib))
		}
		got, answer, _ := cardFaces(t, lib, card)
		if got != prompt || answer != want {
			t.Errorf("card %q faces = (%q, %q), want (%q, %q)", prompt, got, answer, prompt, want)
		}
	}

	records := readReviews(t, dir)
	for prompt, want := range map[string]string{
		"Capital of France?": "good",
		"[...] is in France": "again",
		"Paris is in [...]":  "easy",
	} {
		card, ok := cards[prompt]
		if !ok {
			t.Fatalf("no card has %q as its prompt", prompt)
		}
		if got := ratingsFor(records, card.ID); len(got) != 1 || got[0] != want {
			t.Errorf("card %q ratings = %v, want [%s]", prompt, got, want)
		}
		if state, ok := lib.CardState(card.ID); !ok || state.Reviews != 1 {
			t.Errorf("card %q state = (%+v, %v), want one review", prompt, state, ok)
		}
	}
}

// TestSchema18ClozeIsFlattened covers what Anki's cloze markup can hold beyond
// one blank: a same-number group hides every blank of the card, a hint is
// dropped, and a nested deletion keeps its text. Each is named in the report.
func TestSchema18ClozeIsFlattened(t *testing.T) {
	lib, _ := newLibrary(t)
	f := package18{
		notetypes: []notetype18{{
			id: 2000, name: "Cloze", kind: 1,
			fields:    []string{"Text"},
			templates: []template18{{name: "Cloze", front: "{{cloze:Text}}", back: "{{cloze:Text}}"}},
		}},
		decks: []deck18{{id: 1, name: "Default"}},
		notes: []noteRow{
			{id: 6001, guid: "guid-group", mid: 2000, fields: []string{"{{c1::Paris}} and {{c1::Lyon}} are in {{c2::France::the country}}"}},
			{id: 6002, guid: "guid-nested", mid: 2000, fields: []string{"{{c1::A {{c2::deep}} B}}"}},
		},
		cards: []cardRow{
			{id: 601, note: 6001, deck: 1, ord: 1},
			{id: 602, note: 6001, deck: 1, ord: 2},
			{id: 603, note: 6002, deck: 1, ord: 1},
			{id: 604, note: 6002, deck: 1, ord: 2},
		},
	}
	path := buildPackage18(t, f)

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 2 || report.Cards != 4 || report.SkippedRows != 0 {
		t.Fatalf("report = %+v, want 2 notes and 4 cards", report)
	}
	for _, wanted := range []string{"hint", "nested cloze", "hides them all"} {
		if !hasWarning(report, wanted) {
			t.Errorf("warnings do not mention %q: %q", wanted, report.Warnings)
		}
	}

	for _, want := range [][]string{
		{"Paris and Lyon are in France"},
		{"A deep B"},
	} {
		if !hasBlocks(lib, want) {
			t.Errorf("no note holds the blocks %q; notes are %v", want, noteBlockTexts(lib))
		}
	}

	cards := cardsByPrompt(t, lib)
	answers := map[string]string{
		"[...] and [...] are in France": "Paris and Lyon are in France",
		"Paris and Lyon are in [...]":   "Paris and Lyon are in France",
		"[...]":                         "A deep B",
		"A [...] B":                     "A deep B",
	}
	for prompt, want := range answers {
		card, ok := cards[prompt]
		if !ok {
			t.Fatalf("no card has %q as its prompt; cards are %v", prompt, prompts(t, lib))
		}
		if _, answer, _ := cardFaces(t, lib, card); answer != want {
			t.Errorf("card %q answer = %q, want %q", prompt, answer, want)
		}
	}
	if got := len(cards); got != len(answers) {
		t.Errorf("%d cards imported, want %d", got, len(answers))
	}
}

// TestZstdEntryWithLegacyTablesIsReadAsLegacy proves the schema is chosen by
// the tables a collection has: a zstd entry holding a legacy collection still
// reads note types out of col.models.
func TestZstdEntryWithLegacyTablesIsReadAsLegacy(t *testing.T) {
	lib, _ := newLibrary(t)
	collection := buildLegacyDB(t, minimalFixture())
	path := filepath.Join(t.TempDir(), "deck.apkg")
	writePackage(t, path, func(zw *zip.Writer) {
		writeEntry(t, zw, "collection.anki21b", zstdCompress(t, collection))
	})

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 2 || report.Cards != 3 || report.Reviews != 5 || report.SkippedRows != 0 {
		t.Errorf("report = %+v, want the legacy collection's 2 notes, 3 cards and 5 reviews", report)
	}
}

// TestSchema18EntryIsNotSQLite is the hostile entry: zstd decompresses, and
// what comes out is not a database. The import says so and adds nothing.
func TestSchema18EntryIsNotSQLite(t *testing.T) {
	lib, _ := newLibrary(t)
	path := buildPackage18(t, package18{payload: []byte("this decompressed but is not a database")})

	if _, err := importanki.Import(path, lib); err == nil {
		t.Fatal("Import accepted a zstd entry that is not SQLite")
	} else if !strings.Contains(err.Error(), "collection.anki21b") || !strings.Contains(err.Error(), "not a SQLite database") {
		t.Errorf("error = %q, want it to name the entry and why it was refused", err)
	}
	if got := len(lib.Notes()) + len(lib.Cards()); got != 0 {
		t.Errorf("a failed import left %d entities behind", got)
	}
}

// TestSchema18EntryWinsOverLegacy proves the newest entry is the one read when
// an export carries the legacy collection beside it.
func TestSchema18EntryWinsOverLegacy(t *testing.T) {
	lib, _ := newLibrary(t)
	legacy := fixture{
		models: basicModels(),
		decks:  basicDecks(),
		notes:  []noteRow{{id: 9001, guid: "guid-old", mid: 1000, fields: []string{"Older", "ignored"}}},
		cards:  []cardRow{{id: 901, note: 9001, deck: 1, ord: 0}},
	}
	f := package18{
		notetypes: basicNotetypes18(),
		decks:     []deck18{{id: 1, name: "Default"}},
		notes:     []noteRow{{id: 9101, guid: "guid-new", mid: 1000, fields: []string{"Newest", "kept"}}},
		cards:     []cardRow{{id: 911, note: 9101, deck: 1, ord: 0}},
		legacy:    &legacy,
	}
	path := buildPackage18(t, f)

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 1 || report.Cards != 1 {
		t.Fatalf("report = %+v, want the newest entry's one note and card", report)
	}
	notes := lib.Notes()
	if len(notes) != 1 || !reflect.DeepEqual(blockTexts(notes[0]), []string{"Newest", "kept"}) {
		t.Fatalf("imported notes = %v, want the schema 18 entry", noteBlockTexts(lib))
	}
}

// noteByTitle finds the imported note a reader would call by that title.
func noteByTitle(t *testing.T, lib *store.Library, title string) core.Note {
	t.Helper()
	for _, note := range lib.Notes() {
		if note.Title == title {
			return note
		}
	}
	t.Fatalf("no note is titled %q", title)
	return core.Note{}
}

// hasBlocks reports whether some note holds exactly these blocks.
func hasBlocks(lib *store.Library, want []string) bool {
	for _, note := range lib.Notes() {
		if reflect.DeepEqual(blockTexts(note), want) {
			return true
		}
	}
	return false
}

// noteBlockTexts lists every note's blocks, for a failure message.
func noteBlockTexts(lib *store.Library) [][]string {
	var out [][]string
	for _, note := range lib.Notes() {
		out = append(out, blockTexts(note))
	}
	return out
}
