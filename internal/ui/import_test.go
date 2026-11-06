package ui

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/importanki"

	_ "modernc.org/sqlite" // the importer's driver: the fixture is a real collection
)

// The Anki collection schema the fixture writes, as Anki documents schema 11.
// It is the smallest collection the importer reads: one note, one card, no
// review history.
const (
	fixtureColSchema = `CREATE TABLE col (
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
	fixtureNotesSchema = `CREATE TABLE notes (
	id      integer primary key,
	guid    text,
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
	fixtureCardsSchema = `CREATE TABLE cards (
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
	fixtureRevlogSchema = `CREATE TABLE revlog (
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

	// One basic notetype with a front and a back, in Anki's JSON shape.
	fixtureModels = `{"1000": {"id": 1000, "name": "Basic", "type": 0,
		"flds": [{"name": "Front", "ord": 0}, {"name": "Back", "ord": 1}],
		"tmpls": [{"name": "Card 1", "ord": 0, "qfmt": "{{Front}}",
			"afmt": "{{FrontSide}}\n<hr id=answer>\n{{Back}}"}]}}`
	fixtureDecks = `{"1": {"id": 1, "name": "Default"}}`
)

// buildAPKG writes a real Anki package: a SQLite collection inside a zip, which
// is what the importer reads. The note carries no GUID on purpose, so the
// import reports the one warning it makes of that.
func buildAPKG(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	dbPath := filepath.Join(work, "collection.anki2")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open fixture collection: %v", err)
	}
	for _, schema := range []string{fixtureColSchema, fixtureNotesSchema, fixtureCardsSchema, fixtureRevlogSchema} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("create fixture table: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO col (id, crt, mod, scm, ver, dty, usn, ls, conf, models, decks, dconf, tags)
		VALUES (1, 0, 0, 0, 11, 0, 0, 0, '{}', ?, ?, '{}', '{}')`, fixtureModels, fixtureDecks); err != nil {
		t.Fatalf("insert fixture collection: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO notes (id, guid, mid, mod, usn, tags, flds, sfld, csum, flags, data)
		VALUES (1001, NULL, 1000, 0, 0, '', ?, 0, 0, 0, '')`, "Capital of France?\x1fParis"); err != nil {
		t.Fatalf("insert fixture note: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO cards (id, nid, did, ord, mod, usn, type, queue, due, ivl, factor, reps, lapses, left, odue, odid, flags, data)
		VALUES (101, 1001, 1, 0, 0, 0, 0, 0, 0, 0, 2500, 0, 0, 0, 0, 0, 0, '')`); err != nil {
		t.Fatalf("insert fixture card: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture collection: %v", err)
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read fixture collection: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entry, err := zw.Create("collection.anki2")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	path := filepath.Join(work, "geography.apkg")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write fixture package: %v", err)
	}
	return path
}

// The palette's import action opens a package and imports it into the library
// on screen: the notes and cards grow, the window reloads onto them, and the
// report says what was written while the warnings go to the log.
func TestImportAnkiFromThePalette(t *testing.T) {
	dir := t.TempDir()
	seedLibrary(t, dir)
	pkg := buildAPKG(t)

	chosen := 0
	a := NewApp(Options{LibraryDir: dir, ChooseAnki: func() (string, error) {
		chosen++
		return pkg, nil
	}})
	notes, cards := len(a.Library().Notes()), len(a.Library().Cards())

	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit

	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("anki")
	tt.Key(0, ui.KeyEnter)

	if chosen != 1 {
		t.Fatalf("the package picker ran %d times", chosen)
	}
	if got := len(a.Library().Notes()); got != notes+1 {
		t.Fatalf("the library holds %d notes, want %d", got, notes+1)
	}
	if got := len(a.Library().Cards()); got != cards+1 {
		t.Fatalf("the library holds %d cards, want %d", got, cards+1)
	}
	if !tt.HasText("Imported 1 notes, 1 cards, 0 reviews, skipped 0 rows.") {
		t.Fatalf("the report is not on the window; texts %q", tt.Texts())
	}
	if !tt.HasText("Capital of France?") {
		t.Fatalf("the window did not reload onto the imported note; texts %q", tt.Texts())
	}
	if !strings.Contains(logged.String(), "no GUID") {
		t.Errorf("the import's warning is not in the log: %s", logged.String())
	}
	tt.Advance(5 * time.Second) // past the toast
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A canceled dialog imports nothing and says nothing: a click that was taken
// back is not an outcome.
func TestImportAnkiCanceled(t *testing.T) {
	dir := t.TempDir()
	seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir, ChooseAnki: func() (string, error) { return "", nil }})
	notes := len(a.Library().Notes())

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("anki")
	tt.Key(0, ui.KeyEnter)

	if got := len(a.Library().Notes()); got != notes {
		t.Errorf("a canceled import wrote %d notes", got-notes)
	}
	if tt.HasText("Imported") {
		t.Errorf("a canceled import reported one; texts %q", tt.Texts())
	}
}

// A package that will not import says why where the action was, and leaves the
// library as it was.
func TestImportAnkiFailure(t *testing.T) {
	dir := t.TempDir()
	seedLibrary(t, dir)
	a := NewApp(Options{
		LibraryDir: dir,
		ChooseAnki: func() (string, error) { return "/tmp/broken.apkg", nil },
		ImportAnki: func(string) (importanki.Report, error) {
			return importanki.Report{}, errors.New("not a zip archive")
		},
	})
	notes := len(a.Library().Notes())

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("anki")
	tt.Key(0, ui.KeyEnter)

	if !tt.HasText("The import failed: not a zip archive") {
		t.Fatalf("the failure is not on the window; texts %q", tt.Texts())
	}
	if got := len(a.Library().Notes()); got != notes {
		t.Errorf("a failed import wrote %d notes", got-notes)
	}
}
