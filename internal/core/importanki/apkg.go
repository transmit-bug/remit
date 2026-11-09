package importanki

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"

	_ "modernc.org/sqlite" // pure-Go driver: CGO_ENABLED=0 keeps working
)

const (
	entryLegacy1 = "collection.anki2"
	entryLegacy2 = "collection.anki21"
	entryLatest  = "collection.anki21b"
	entryMedia   = "media"

	// fieldSeparator joins the fields of one note row.
	fieldSeparator = "\x1f"

	// maxCollectionBytes caps what a package may make this process read, and
	// with it what the zstd entry may decompress to.
	maxCollectionBytes = 512 << 20

	// sqliteHeader starts every SQLite database file.
	sqliteHeader = "SQLite format 3\x00"
)

// apkg is an opened package: its zip directory, the collection extracted to a
// temporary file, and anything odd worth a warning.
type apkg struct {
	zip      *zip.ReadCloser
	entries  map[string]*zip.File
	dir      string
	dbPath   string
	warnings []string
}

// openPackage reads the zip and extracts the collection database. Media stays
// in the archive: the library has nowhere to put it (ADR 0003).
//
// The collection entry is chosen newest first, so an export that carries the
// current zstd entry beside the legacy ones is read in its current form, and a
// package written by an older Anki still opens.
func openPackage(path string) (*apkg, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("importanki: %s: not an Anki package: %w", path, err)
	}
	p := &apkg{zip: zr, entries: make(map[string]*zip.File, len(zr.File))}
	for _, entry := range zr.File {
		p.entries[entry.Name] = entry
	}

	name := ""
	for _, candidate := range []string{entryLatest, entryLegacy2, entryLegacy1} {
		if _, ok := p.entries[candidate]; ok {
			name = candidate
			break
		}
	}
	if name == "" {
		p.close()
		return nil, fmt.Errorf("importanki: %s: no %s, %s or %s in the package", path, entryLatest, entryLegacy2, entryLegacy1)
	}

	dir, err := os.MkdirTemp("", "remit-apkg-")
	if err != nil {
		p.close()
		return nil, fmt.Errorf("importanki: %w", err)
	}
	p.dir = dir
	p.dbPath = filepath.Join(dir, entryLegacy1) // the driver does not care about the name
	if err := p.extractCollection(name); err != nil {
		p.close()
		return nil, err
	}
	if err := checkSQLite(p.dbPath); err != nil {
		p.close()
		return nil, fmt.Errorf("importanki: %s: %s: %w", path, name, err)
	}
	p.readMediaManifest()
	return p, nil
}

func (p *apkg) close() {
	if p == nil {
		return
	}
	if p.zip != nil {
		p.zip.Close()
	}
	if p.dir != "" {
		os.RemoveAll(p.dir)
	}
}

// extractCollection writes the chosen zip member to the database path,
// decompressing it first when it is the zstd-compressed collection of a
// current Anki export.
func (p *apkg) extractCollection(name string) error {
	entry := p.entries[name]
	rc, err := entry.Open()
	if err != nil {
		return fmt.Errorf("importanki: %s: %w", entry.Name, err)
	}
	defer rc.Close()
	if name == entryLatest {
		return extractZstd(entry.Name, p.dbPath, rc)
	}
	return writeCollection(entry.Name, p.dbPath, rc)
}

// extractZstd decompresses the zstd frame of collection.anki21b into path.
// The decoder is streamed and bounded to a collection's size, so a hostile
// frame can neither make the importer write more than a collection could hold
// nor reserve a window larger than one.
func extractZstd(name, path string, r io.Reader) error {
	dec, err := zstd.NewReader(r,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderLowmem(true),
		zstd.WithDecoderMaxMemory(maxCollectionBytes+1),
		zstd.WithDecoderMaxWindow(maxCollectionBytes+1),
	)
	if err != nil {
		return fmt.Errorf("importanki: %s: not a zstd frame: %w", name, err)
	}
	defer dec.Close()
	return writeCollection(name, path, dec)
}

// writeCollection copies a collection into path, capped at maxCollectionBytes.
func writeCollection(name, path string, r io.Reader) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("importanki: %w", err)
	}
	written, err := io.Copy(file, io.LimitReader(r, maxCollectionBytes+1))
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("importanki: %s: %w", name, err)
	}
	if closeErr != nil {
		return fmt.Errorf("importanki: %s: %w", name, closeErr)
	}
	if written > maxCollectionBytes {
		return fmt.Errorf("importanki: %s is larger than %d bytes", name, maxCollectionBytes)
	}
	if written == 0 {
		return fmt.Errorf("importanki: %s is empty", name)
	}
	return nil
}

// checkSQLite refuses a collection entry that is not an SQLite database, so a
// decompressed file of the wrong kind names the entry instead of surfacing as
// a driver error on the first table the importer asks for.
func checkSQLite(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("not a SQLite database: %w", err)
	}
	if string(header) != sqliteHeader {
		return errors.New("not a SQLite database")
	}
	return nil
}

// readMediaManifest checks the media map, not to copy files but to notice a
// package whose media the text cannot even name. The legacy map is the JSON
// object {"0": "foo.jpg"}.
func (p *apkg) readMediaManifest() {
	manifest, ok := p.entries[entryMedia]
	if !ok {
		if len(p.numberedMedia()) > 0 {
			p.warnings = append(p.warnings, "media: the package has numbered media files but no media map; file names stay as the fields spell them")
		}
		return
	}
	rc, err := manifest.Open()
	if err != nil {
		p.warnings = append(p.warnings, fmt.Sprintf("media: %s: %v", entryMedia, err))
		return
	}
	data, err := io.ReadAll(io.LimitReader(rc, 16<<20))
	rc.Close()
	if err != nil {
		p.warnings = append(p.warnings, fmt.Sprintf("media: %s: %v", entryMedia, err))
		return
	}
	var names map[string]string
	if err := json.Unmarshal(data, &names); err != nil {
		p.warnings = append(p.warnings, fmt.Sprintf("media: %s is not the expected JSON map (%v); file names stay as the fields spell them", entryMedia, err))
		return
	}
	if len(names) > 0 {
		p.warnings = append(p.warnings, fmt.Sprintf("media: the package carries %d file(s) that no Remit location can hold; their names are kept in the text", len(names)))
	}
}

// numberedMedia lists the zip members named like a media entry ("0", "1", …).
func (p *apkg) numberedMedia() []string {
	var names []string
	for name := range p.entries {
		if name != "" && strings.Trim(name, "0123456789") == "" {
			names = append(names, name)
		}
	}
	return names
}

// collection is the subset of an Anki collection this importer reads. Notes,
// cards and revlog keep their shape across schemas; only note types and decks
// moved, so whichever source a package holds is read into notetypes and decks
// (schema 18) or left as the JSON of col (schema 11).
type collection struct {
	notetypes  []dbNotetype
	decks      []dbDeck
	modelsJSON string
	decksJSON  string
	// split says the note types live in the schema 18 tables, so col.models
	// must not be consulted even when the col table is still there.
	split    bool
	notes    []dbNote
	cards    []dbCard
	revlog   []dbRevlog
	warnings []string
}

// dbNotetype is one row of the schema 18 split tables with its fields and
// templates: an ordered field list plus the templates that turn a note into
// card sides — the shape of a Remit card type.
type dbNotetype struct {
	id        int64
	name      string
	kind      int // Anki notetype kind: 1 is cloze
	fields    []dbField
	templates []dbTemplate
}

type dbField struct {
	ord  int
	name string
}

type dbTemplate struct {
	ord   int
	name  string
	front string
	back  string
}

type dbDeck struct {
	id   int64
	name string
}

type dbNote struct {
	id     int64
	guid   string
	mid    int64
	tags   string
	fields []string
}

type dbCard struct {
	id    int64
	note  int64
	deck  int64
	ord   int
	kind  int
	queue int
}

type dbRevlog struct {
	id   int64
	card int64
	ease int
	kind int
}

// readCollection opens the extracted database read-only and reads every table
// this importer needs. A missing table is an error naming it, never a panic.
func (p *apkg) readCollection() (*collection, error) {
	db, err := sql.Open("sqlite", "file:"+p.dbPath+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return nil, fmt.Errorf("importanki: open collection: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	col := &collection{}
	split, err := tableExists(db, "notetypes")
	if err != nil {
		return nil, fmt.Errorf("importanki: %w", err)
	}
	if split {
		err = readSchema18(db, col)
	} else {
		err = readColJSON(db, col)
	}
	col.split = split
	if err != nil {
		return nil, err
	}
	if err := readNotes(db, col); err != nil {
		return nil, err
	}
	if err := readCards(db, col); err != nil {
		return nil, err
	}
	if err := readRevlog(db, col); err != nil {
		return nil, err
	}
	return col, nil
}

// readSchema18 reads the note types, their fields and templates, and the deck
// names of a current collection. A notetype's kind and a template's formats
// live in protobuf config blobs, so each is decoded rather than guessed.
func readSchema18(db *sql.DB, col *collection) error {
	rows, err := db.Query(`SELECT id, name, config FROM notetypes ORDER BY id`)
	if err != nil {
		return fmt.Errorf("importanki: notetypes: %w", err)
	}
	for rows.Next() {
		nt := dbNotetype{}
		var config []byte
		if err := rows.Scan(&nt.id, &nt.name, &config); err != nil {
			rows.Close()
			return fmt.Errorf("importanki: notetypes: %w", err)
		}
		nt.kind = notetypeKind(config)
		col.notetypes = append(col.notetypes, nt)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("importanki: notetypes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("importanki: notetypes: %w", err)
	}

	byID := make(map[int64]*dbNotetype, len(col.notetypes))
	for i := range col.notetypes {
		byID[col.notetypes[i].id] = &col.notetypes[i]
	}

	rows, err = db.Query(`SELECT ntid, ord, name FROM fields ORDER BY ntid, ord`)
	if err != nil {
		return fmt.Errorf("importanki: fields: %w", err)
	}
	for rows.Next() {
		var (
			ntid int64
			f    dbField
		)
		if err := rows.Scan(&ntid, &f.ord, &f.name); err != nil {
			rows.Close()
			return fmt.Errorf("importanki: fields: %w", err)
		}
		if nt, ok := byID[ntid]; ok {
			nt.fields = append(nt.fields, f)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("importanki: fields: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("importanki: fields: %w", err)
	}

	// A template's two formats are a protobuf config blob in the schema Anki
	// writes. A collection that kept them as columns is read as well, so the
	// schema is chosen by the columns present rather than by the file name.
	columns, err := tableColumns(db, "templates")
	if err != nil {
		return fmt.Errorf("importanki: templates: %w", err)
	}
	byColumn := columns["qfmt"] && columns["afmt"]
	query := `SELECT ntid, ord, name, config FROM templates ORDER BY ntid, ord`
	if byColumn {
		query = `SELECT ntid, ord, name, qfmt, afmt FROM templates ORDER BY ntid, ord`
	}
	rows, err = db.Query(query)
	if err != nil {
		return fmt.Errorf("importanki: templates: %w", err)
	}
	for rows.Next() {
		var (
			ntid int64
			t    dbTemplate
			q, a string
			blob []byte
		)
		if byColumn {
			if err := rows.Scan(&ntid, &t.ord, &t.name, &q, &a); err != nil {
				rows.Close()
				return fmt.Errorf("importanki: templates: %w", err)
			}
			t.front, t.back = q, a
		} else {
			if err := rows.Scan(&ntid, &t.ord, &t.name, &blob); err != nil {
				rows.Close()
				return fmt.Errorf("importanki: templates: %w", err)
			}
			if front, back, ok := templateFormats(blob); ok {
				t.front, t.back = front, back
			} else {
				col.warnings = append(col.warnings,
					fmt.Sprintf("templates: notetype %d template %q has a config this importer cannot read; its cards fall back to the first field", ntid, t.name))
			}
		}
		if nt, ok := byID[ntid]; ok {
			nt.templates = append(nt.templates, t)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("importanki: templates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("importanki: templates: %w", err)
	}

	rows, err = db.Query(`SELECT id, name FROM decks ORDER BY id`)
	if err != nil {
		return fmt.Errorf("importanki: decks: %w", err)
	}
	for rows.Next() {
		var deck dbDeck
		if err := rows.Scan(&deck.id, &deck.name); err != nil {
			rows.Close()
			return fmt.Errorf("importanki: decks: %w", err)
		}
		col.decks = append(col.decks, deck)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("importanki: decks: %w", err)
	}
	return rows.Close()
}

// readColJSON reads the note types and decks of a legacy collection, whose
// schema keeps both as JSON in the col table.
func readColJSON(db *sql.DB, col *collection) error {
	columns, err := tableColumns(db, "col")
	if err != nil {
		return fmt.Errorf("importanki: col: %w", err)
	}
	if !columns["models"] {
		return errors.New("importanki: the collection has neither a notetypes table nor col.models; it holds no note types")
	}
	if err := db.QueryRow(`SELECT models, decks FROM col LIMIT 1`).Scan(&col.modelsJSON, &col.decksJSON); err != nil {
		return fmt.Errorf("importanki: col: %w", err)
	}
	return nil
}

func readNotes(db *sql.DB, col *collection) error {
	notes, err := db.Query(`SELECT id, guid, mid, tags, flds FROM notes ORDER BY id`)
	if err != nil {
		return fmt.Errorf("importanki: notes: %w", err)
	}
	for notes.Next() {
		var (
			note = dbNote{}
			guid sql.NullString
			tags sql.NullString
			flds string
		)
		if err := notes.Scan(&note.id, &guid, &note.mid, &tags, &flds); err != nil {
			notes.Close()
			return fmt.Errorf("importanki: notes: %w", err)
		}
		note.guid = guid.String
		note.tags = tags.String
		note.fields = strings.Split(flds, fieldSeparator)
		col.notes = append(col.notes, note)
	}
	if err := notes.Err(); err != nil {
		return fmt.Errorf("importanki: notes: %w", err)
	}
	if err := notes.Close(); err != nil {
		return fmt.Errorf("importanki: notes: %w", err)
	}
	return nil
}

func readCards(db *sql.DB, col *collection) error {
	cards, err := db.Query(`SELECT id, nid, did, ord, type, queue FROM cards ORDER BY id`)
	if err != nil {
		return fmt.Errorf("importanki: cards: %w", err)
	}
	for cards.Next() {
		card := dbCard{}
		if err := cards.Scan(&card.id, &card.note, &card.deck, &card.ord, &card.kind, &card.queue); err != nil {
			cards.Close()
			return fmt.Errorf("importanki: cards: %w", err)
		}
		col.cards = append(col.cards, card)
	}
	if err := cards.Err(); err != nil {
		return fmt.Errorf("importanki: cards: %w", err)
	}
	if err := cards.Close(); err != nil {
		return fmt.Errorf("importanki: cards: %w", err)
	}
	return nil
}

func readRevlog(db *sql.DB, col *collection) error {
	revlog, err := db.Query(`SELECT id, cid, ease, type FROM revlog ORDER BY id, cid`)
	if err != nil {
		return fmt.Errorf("importanki: revlog: %w", err)
	}
	for revlog.Next() {
		row := dbRevlog{}
		if err := revlog.Scan(&row.id, &row.card, &row.ease, &row.kind); err != nil {
			revlog.Close()
			return fmt.Errorf("importanki: revlog: %w", err)
		}
		col.revlog = append(col.revlog, row)
	}
	if err := revlog.Err(); err != nil {
		return fmt.Errorf("importanki: revlog: %w", err)
	}
	if err := revlog.Close(); err != nil {
		return fmt.Errorf("importanki: revlog: %w", err)
	}
	return nil
}

// tableExists reports whether the database has a table of that name.
func tableExists(db *sql.DB, name string) (bool, error) {
	var found int
	err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("sqlite_master: %w", err)
	}
	return found > 0, nil
}

// tableColumns names the columns of a table. A table that is not there has no
// columns rather than an error, so callers can ask about both schemas.
func tableColumns(db *sql.DB, name string) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns[column] = true
	}
	return columns, rows.Err()
}
