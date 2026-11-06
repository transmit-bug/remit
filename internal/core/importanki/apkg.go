package importanki

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // pure-Go driver: CGO_ENABLED=0 keeps working
)

const (
	entryLegacy1 = "collection.anki2"
	entryLegacy2 = "collection.anki21"
	entryLatest  = "collection.anki21b"
	entryMedia   = "media"

	// fieldSeparator joins the fields of one note row.
	fieldSeparator = "\x1f"

	// maxCollectionBytes caps what a package may make this process read.
	maxCollectionBytes = 512 << 20
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
	for _, candidate := range []string{entryLegacy2, entryLegacy1} {
		if _, ok := p.entries[candidate]; ok {
			name = candidate
			break
		}
	}
	if name == "" {
		p.close()
		if _, ok := p.entries[entryLatest]; ok {
			return nil, fmt.Errorf("importanki: %s: %s is zstd-compressed (Anki schema 18); re-export the deck with legacy support", path, entryLatest)
		}
		return nil, fmt.Errorf("importanki: %s: no %s or %s in the package", path, entryLegacy1, entryLegacy2)
	}

	dir, err := os.MkdirTemp("", "remit-apkg-")
	if err != nil {
		p.close()
		return nil, fmt.Errorf("importanki: %w", err)
	}
	p.dir = dir
	p.dbPath = filepath.Join(dir, entryLegacy1)
	if err := extractEntry(p.entries[name], p.dbPath); err != nil {
		p.close()
		return nil, err
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

// extractEntry writes one zip member to path, refusing a member larger than a
// collection could reasonably be.
func extractEntry(entry *zip.File, path string) error {
	rc, err := entry.Open()
	if err != nil {
		return fmt.Errorf("importanki: %s: %w", entry.Name, err)
	}
	defer rc.Close()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("importanki: %w", err)
	}
	written, err := io.Copy(file, io.LimitReader(rc, maxCollectionBytes+1))
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("importanki: %s: %w", entry.Name, err)
	}
	if closeErr != nil {
		return fmt.Errorf("importanki: %s: %w", entry.Name, closeErr)
	}
	if written > maxCollectionBytes {
		return fmt.Errorf("importanki: %s is larger than %d bytes", entry.Name, maxCollectionBytes)
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

// collection is the subset of an Anki collection this importer reads.
type collection struct {
	modelsJSON string
	decksJSON  string
	notes      []dbNote
	cards      []dbCard
	revlog     []dbRevlog
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
	if err := db.QueryRow(`SELECT models, decks FROM col LIMIT 1`).Scan(&col.modelsJSON, &col.decksJSON); err != nil {
		return nil, fmt.Errorf("importanki: col: %w", err)
	}

	notes, err := db.Query(`SELECT id, guid, mid, tags, flds FROM notes ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("importanki: notes: %w", err)
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
			return nil, fmt.Errorf("importanki: notes: %w", err)
		}
		note.guid = guid.String
		note.tags = tags.String
		note.fields = strings.Split(flds, fieldSeparator)
		col.notes = append(col.notes, note)
	}
	if err := notes.Err(); err != nil {
		return nil, fmt.Errorf("importanki: notes: %w", err)
	}
	if err := notes.Close(); err != nil {
		return nil, fmt.Errorf("importanki: notes: %w", err)
	}

	cards, err := db.Query(`SELECT id, nid, did, ord, type, queue FROM cards ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("importanki: cards: %w", err)
	}
	for cards.Next() {
		card := dbCard{}
		if err := cards.Scan(&card.id, &card.note, &card.deck, &card.ord, &card.kind, &card.queue); err != nil {
			cards.Close()
			return nil, fmt.Errorf("importanki: cards: %w", err)
		}
		col.cards = append(col.cards, card)
	}
	if err := cards.Err(); err != nil {
		return nil, fmt.Errorf("importanki: cards: %w", err)
	}
	if err := cards.Close(); err != nil {
		return nil, fmt.Errorf("importanki: cards: %w", err)
	}

	revlog, err := db.Query(`SELECT id, cid, ease, type FROM revlog ORDER BY id, cid`)
	if err != nil {
		return nil, fmt.Errorf("importanki: revlog: %w", err)
	}
	for revlog.Next() {
		row := dbRevlog{}
		if err := revlog.Scan(&row.id, &row.card, &row.ease, &row.kind); err != nil {
			revlog.Close()
			return nil, fmt.Errorf("importanki: revlog: %w", err)
		}
		col.revlog = append(col.revlog, row)
	}
	if err := revlog.Err(); err != nil {
		return nil, fmt.Errorf("importanki: revlog: %w", err)
	}
	if err := revlog.Close(); err != nil {
		return nil, fmt.Errorf("importanki: revlog: %w", err)
	}
	return col, nil
}
