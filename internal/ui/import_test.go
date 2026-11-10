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
	"github.com/transmit-bug/remit/internal/core/transfer"
	"github.com/transmit-bug/remit/locales"

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

// wizardFromPalette opens the import wizard the way a user reaches it: the
// palette's import command, found by typing its name.
func wizardFromPalette(t *testing.T, tt *ui.Tester, msgs locales.Messages) {
	t.Helper()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type(msgs.ImportTitle())
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
}

// The wizard walks its three steps: a source, a preview of what it holds, and
// the report of what the run wrote. The package's notes and cards grow the
// library on screen, its warnings go to the log, and the closing page says what
// was written.
func TestImportWizardWalksThreeSteps(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	pkg := buildAPKG(t)

	a := wizardApp(Options{LibraryDir: dir, ChooseAnki: func() (string, error) { return pkg, nil }})
	notes, cards := len(a.Library().Notes()), len(a.Library().Cards())

	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	wizardFromPalette(t, tt, msgs)

	// The first step: the three sources, and the reason Next is not live.
	if !tt.HasText(msgs.ImportTitle()) {
		t.Fatalf("the wizard did not open; texts %q", tt.Texts())
	}
	for _, source := range []string{msgs.ImportFromArchive(), msgs.ImportFromMarkdown(), msgs.ImportFromAnki()} {
		if !tt.HasText(source) {
			t.Errorf("the wizard offers no %q source; texts %q", source, tt.Texts())
		}
	}
	if !tt.HasText(msgs.ImportChooseFirst()) {
		t.Errorf("the step does not say why Next is disabled; texts %q", tt.Texts())
	}

	// The second step: what the package holds and where the run writes.
	if err := tt.Click(msgs.ImportFromAnki()); err != nil {
		t.Fatalf("the Anki source is not clickable: %v", err)
	}
	tt.Frame()
	for _, row := range []string{msgs.ImportRowSource(), msgs.ImportRowFiles(), msgs.ImportRowDestination()} {
		if !tt.HasText(row) {
			t.Errorf("the preview reports no %q; texts %q", row, tt.Texts())
		}
	}
	if !tt.HasText(msgs.ImportFromAnki()) {
		t.Errorf("the preview does not name the source chosen; texts %q", tt.Texts())
	}
	for _, option := range []string{msgs.ImportRebuild(), msgs.ImportQueue()} {
		if !tt.HasText(option) {
			t.Errorf("the preview offers no %q option; texts %q", option, tt.Texts())
		}
	}
	// An Anki note is matched by its GUID, so keeping existing notes is not a
	// choice this source has: the step names the reason instead of a switch
	// that would do nothing.
	if !tt.HasText(msgs.ImportKeepAnki()) {
		t.Errorf("the preview does not say why keeping is not a choice; texts %q", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleSwitch, msgs.ImportKeep()); ok {
		t.Error("the Anki preview offers a keep switch the import cannot obey")
	}

	// The third step: the write, and what it wrote.
	if err := tt.Click(msgs.ImportRun()); err != nil {
		t.Fatalf("the preview has no import button: %v", err)
	}
	tt.Frame()
	if got := len(a.Library().Notes()); got != notes+1 {
		t.Fatalf("the library holds %d notes, want %d", got, notes+1)
	}
	if got := len(a.Library().Cards()); got != cards+1 {
		t.Fatalf("the library holds %d cards, want %d", got, cards+1)
	}
	if !tt.HasText(msgs.ImportReport(1, 1, 0, 0)) {
		t.Fatalf("the report is not on the window; texts %q", tt.Texts())
	}
	// The reading option is on by default, and it is a switch the run obeys: the
	// note that arrived is in the queue the read area draws.
	queued, err := a.Library().ReadingQueue()
	if err != nil {
		t.Fatalf("the reading queue: %v", err)
	}
	if len(queued) != 1 {
		t.Errorf("the reading queue holds %d notes, want the imported one", len(queued))
	}
	if !strings.Contains(logged.String(), "no GUID") {
		t.Errorf("the import's warning is not in the log: %s", logged.String())
	}

	// The close gives the window back.
	if err := tt.Click(msgs.Close()); err != nil {
		t.Fatalf("the last step has no close button: %v", err)
	}
	tt.Frame()
	if tt.HasText(msgs.ImportTitle()) {
		t.Errorf("the wizard stayed open after its last step; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // past the toast
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A wizard with no source advances to nothing: Next is disabled, and the step
// says why. A picker that answers nothing leaves the wizard where it was — a
// click taken back is not a source, and an import of nothing writes nothing.
func TestImportWizardRefusesAnEmptySource(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	picked := 0
	a := wizardApp(Options{
		LibraryDir:     dir,
		ChooseMarkdown: func() ([]string, error) { picked++; return nil, nil },
	})
	notes := len(a.Library().Notes())

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	wizardFromPalette(t, tt, msgs)

	if !tt.HasText(msgs.ImportChooseFirst()) {
		t.Fatalf("the empty step says no reason; texts %q", tt.Texts())
	}
	// Next is disabled: clicking it leaves the wizard on its first step.
	if err := tt.Click(msgs.ImportNext()); err != nil {
		t.Fatalf("the first step has no Next: %v", err)
	}
	tt.Frame()
	if !tt.HasText(msgs.ImportChooseFirst()) {
		t.Fatalf("a disabled Next moved the wizard on; texts %q", tt.Texts())
	}
	// A canceled picker is not a source either.
	if err := tt.Click(msgs.ImportFromMarkdown()); err != nil {
		t.Fatalf("the Markdown source is not clickable: %v", err)
	}
	tt.Frame()
	if picked != 1 {
		t.Fatalf("the file picker ran %d times", picked)
	}
	if !tt.HasText(msgs.ImportChooseFirst()) {
		t.Errorf("a canceled picker advanced the wizard; texts %q", tt.Texts())
	}
	if got := len(a.Library().Notes()); got != notes {
		t.Errorf("a refused source wrote %d notes", got-notes)
	}
}

// A source that will not import says why where the run was started, and the
// wizard stays open with the library as it was: the dialog closes on a write,
// not on an error.
func TestImportWizardReportsAFailure(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	a := wizardApp(Options{
		LibraryDir: dir,
		ChooseAnki: func() (string, error) { return "/tmp/broken.apkg", nil },
		ImportAnki: func(string) (importanki.Report, error) {
			return importanki.Report{}, errors.New("not a zip archive")
		},
	})
	notes := len(a.Library().Notes())

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromAnki()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(msgs.ImportRun()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("not a zip archive") {
		t.Fatalf("the failure is not on the window; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.ImportTitle()) {
		t.Error("the wizard closed on a failure")
	}
	if got := len(a.Library().Notes()); got != notes {
		t.Errorf("a failed import wrote %d notes", got-notes)
	}
}

// A .remitlib is a library, not notes in the one open: the archive unpacks into
// a folder of its own and the window opens it.
func TestImportWizardUnpacksAnArchive(t *testing.T) {
	msgs := locales.In("en")
	// The archive is a library of its own, written the way an export writes it.
	source := t.TempDir()
	seedLibrary(t, source)
	backup := filepath.Join(t.TempDir(), "backup.remitlib")
	file, err := os.Create(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := transfer.Write(source, file); err != nil {
		t.Fatalf("write the archive: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	seedLibrary(t, dir)
	a := wizardApp(Options{LibraryDir: dir, ChooseArchive: func() (string, error) { return backup, nil }})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromArchive()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(msgs.ImportRun()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	unpacked := strings.TrimSuffix(backup, ".remitlib")
	if a.dir != unpacked {
		t.Fatalf("the window opened %q, want %q", a.dir, unpacked)
	}
	if _, err := os.Stat(filepath.Join(unpacked, "remit.json")); err != nil {
		t.Fatalf("the archive did not unpack a library: %v", err)
	}
	if !tt.HasText(msgs.ImportArchiveDone(shortPath(unpacked))) {
		t.Errorf("the write is not reported; texts %q", tt.Texts())
	}
}

// An archive never writes into the library open, so the wizard says so rather
// than offering a choice the run cannot take.
func TestImportWizardSaysAnArchiveKeepsTheLibrary(t *testing.T) {
	msgs := locales.In("en")
	source := t.TempDir()
	seedLibrary(t, source)
	backup := filepath.Join(t.TempDir(), "backup.remitlib")
	file, err := os.Create(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := transfer.Write(source, file); err != nil {
		t.Fatalf("write the archive: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	seedLibrary(t, dir)
	a := wizardApp(Options{LibraryDir: dir, ChooseArchive: func() (string, error) { return backup, nil }})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromArchive()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(msgs.ImportKeepArchive()) {
		t.Errorf("the preview does not say the library here is kept; texts %q", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleSwitch, msgs.ImportKeep()); ok {
		t.Error("the archive preview offers a keep switch the import cannot obey")
	}
}

// The write leaves the frame: the preview step becomes an activity bar while the
// importer works, so the window keeps drawing through a long package, the run
// cannot be started twice, and the report lands when the answer does.
func TestImportShowsProgressWhileItRuns(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	pkg := buildAPKG(t)
	release := make(chan struct{})
	a := NewApp(Options{
		LibraryDir: dir,
		ChooseAnki: func() (string, error) { return pkg, nil },
		ImportAnki: func(string) (importanki.Report, error) {
			<-release
			return importanki.Report{Notes: 2, Cards: 3}, nil
		},
	})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	wizardFromPalette(t, tt, msgs)
	if err := tt.Click(msgs.ImportFromAnki()); err != nil {
		t.Fatalf("the Anki source is not clickable: %v", err)
	}
	tt.Frame()

	// The click returns while the importer is still held: the run is off the
	// frame, not inside it.
	clicked := make(chan error, 1)
	go func() { clicked <- tt.Click(msgs.ImportRun()) }()
	select {
	case err := <-clicked:
		if err != nil {
			t.Fatalf("the preview has no import button: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the run did not leave the frame: the window waited on the importer")
	}
	tt.Frame()
	if !tt.HasText(msgs.ImportRunning()) {
		t.Fatalf("the running step says nothing about the wait; texts %q", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleProgress, msgs.ImportRunning()); !ok {
		t.Error("the running step shows no activity bar")
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, msgs.ImportRun()); ok {
		t.Error("the run can be started again while it is running")
	}

	close(release)
	// The answer is applied on the UI thread: wait for it, then draw.
	deadline := time.Now().Add(5 * time.Second)
	for {
		a.pending.mu.Lock()
		waiting := len(a.pending.jobs)
		a.pending.mu.Unlock()
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the import's answer never reached the window")
		}
		time.Sleep(time.Millisecond)
	}
	tt.Frame()
	if !tt.HasText(msgs.ImportReport(2, 3, 0, 0)) {
		t.Fatalf("the report is not on the window; texts %q", tt.Texts())
	}
}

// wizardApp is NewApp with the wizard's background job run on the frame: a test
// takes an import's answer at once, where the window would take it on a later
// frame. TestImportShowsProgressWhileItRuns keeps the app's own threading, which
// is the part the activity bar exists for.
func wizardApp(o Options) *App {
	a := NewApp(o)
	a.runBackground = func(c *ui.Context, job func() func(*ui.Context)) { job()(c) }
	return a
}
