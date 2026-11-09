package doctor_test

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/doctor"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

var t0 = time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)

// library is a whole small library: two notes, a reviewed card, a frozen card,
// an image a note carries and both kinds of log event. Every defect case starts
// from it and breaks one thing, so a finding it reports is the one the case
// made.
type library struct {
	dir    string
	note   string // the first note's file, relative to dir
	second string // the second note's file
	card   string // the reviewed card's file
	block  core.ID
	asset  string // the image the first note carries
}

func newLibrary(t *testing.T) library {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	asset, err := lib.PutAsset([]byte("\x89PNG\r\n\x1a\npixels"), "png")
	if err != nil {
		t.Fatalf("asset: %v", err)
	}
	first, err := lib.SaveNote(core.Note{Title: "Alpha", Blocks: []core.Block{
		{Text: "spacing :: beats cramming"},
		{Text: "look: ![](" + asset + ")"},
	}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	cards, err := lib.CreateCards(first.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		t.Fatalf("cards: %v", err)
	}

	second, err := lib.SaveNote(core.Note{Title: "Beta", Blocks: []core.Block{
		{Text: "the forgetting curve ::: decays"},
	}})
	if err != nil {
		t.Fatalf("second note: %v", err)
	}
	secondCards, err := lib.CreateCards(second.Blocks[0].ID, []core.Derived{{Type: core.TypeBidirectional}})
	if err != nil {
		t.Fatalf("second cards: %v", err)
	}
	frozen := secondCards[0]
	frozen.Ref = frozen.Ref.Freeze(lib, t0)
	if err := lib.SaveCard(frozen); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	if err := lib.Review(cards[0].ID, schedule.Good, t0); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := lib.AddToReading(second.ID, t0); err != nil {
		t.Fatalf("reading: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	return library{
		dir:    dir,
		note:   noteFile(t, dir, first.ID),
		second: noteFile(t, dir, second.ID),
		card:   "cards/" + string(cards[0].ID) + ".json",
		block:  first.Blocks[0].ID,
		asset:  asset,
	}
}

// noteFile is the file a note was written to, relative to the library.
func noteFile(t *testing.T, dir string, id core.ID) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "notes", "*-"+string(id)+".md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("note %s: %v (%d files)", id, err, len(matches))
	}
	rel, err := filepath.Rel(dir, matches[0])
	if err != nil {
		t.Fatalf("relative %s: %v", matches[0], err)
	}
	return filepath.ToSlash(rel)
}

func TestAWholeLibraryHasNoFindings(t *testing.T) {
	fx := newLibrary(t)
	before := snapshot(t, fx.dir, true)

	report, err := doctor.Check(fx.dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !report.Clean() {
		t.Fatalf("findings in a whole library: %+v", report.Findings)
	}
	if report.Notes != 2 || report.Cards != 2 || report.Reviews != 2 {
		t.Errorf("report counts %d notes, %d cards, %d events; want 2, 2, 2",
			report.Notes, report.Cards, report.Reviews)
	}

	// A check is a read: it must not create the index's write-ahead log, seed a
	// type or rewrite a file it looked at.
	sameTree(t, before, snapshot(t, fx.dir, true))
}

// Each case breaks one thing in a whole library and names the one code that
// follows, with the file the finding must point at.
func TestEachDefectIsNamedByItsCode(t *testing.T) {
	cases := []struct {
		name   string
		defect func(t *testing.T, fx library) string
		want   string
	}{
		{
			name: "no remit.json",
			defect: func(t *testing.T, fx library) string {
				remove(t, filepath.Join(fx.dir, "remit.json"))
				return "remit.json"
			},
			want: doctor.CodeConfigMissing,
		},
		{
			name: "another format version",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, "remit.json"), `{"format": "remit/1"}`)
				return "remit.json"
			},
			want: doctor.CodeConfigFormat,
		},
		{
			name: "a note renamed out of the shape an open reads",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, "notes", "loose.md"), "# Loose\n\ntext\n")
				return "notes/loose.md"
			},
			want: doctor.CodeNoteMisnamed,
		},
		{
			name: "a marker that is not a block id",
			defect: func(t *testing.T, fx library) string {
				appendTo(t, filepath.Join(fx.dir, filepath.FromSlash(fx.note)),
					"\n\nanother paragraph ^block_nothing\n")
				return fx.note
			},
			want: doctor.CodeNoteMarker,
		},
		{
			name: "one block id in two notes",
			defect: func(t *testing.T, fx library) string {
				appendTo(t, filepath.Join(fx.dir, filepath.FromSlash(fx.second)),
					"\n\na copy of the first block "+core.BlockMarker(fx.block)+"\n")
				return fx.second
			},
			want: doctor.CodeBlockDuplicate,
		},
		{
			name: "a card named out of the shape an open reads",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, "cards", "loose.json"), cardBody(string(fx.block), core.TypeBasic))
				return "cards/loose.json"
			},
			want: doctor.CodeCardMisnamed,
		},
		{
			name: "a card that is not JSON a card",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, filepath.FromSlash(fx.card)), `{"block": "left without a type"}`)
				return fx.card
			},
			want: doctor.CodeCardInvalid,
		},
		{
			name: "a card that names no block id",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, filepath.FromSlash(fx.card)), cardBody("not-an-id", core.TypeBasic))
				return fx.card
			},
			want: doctor.CodeCardReference,
		},
		{
			name: "a card whose type is gone",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, filepath.FromSlash(fx.card)),
					cardBody(string(fx.block), core.ID("custom.missing")))
				return fx.card
			},
			want: doctor.CodeCardType,
		},
		{
			name: "a card whose block is gone and that keeps no snapshot",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, filepath.FromSlash(fx.card)),
					cardBody(string(core.NewID(core.BlockKind)), core.TypeBasic))
				return fx.card
			},
			want: doctor.CodeCardLost,
		},
		{
			name: "a type file that is not a type",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, "types", "broken.json"), `{}`)
				return "types/broken.json"
			},
			want: doctor.CodeTypeInvalid,
		},
		{
			name: "a type file whose name and id disagree",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, "types", "other.json"),
					`{"id": "custom.other", "name": "Other", "fields": ["Front"], "front": "{{Front}}", "back": "{{Front}}"}`)
				return "types/other.json"
			},
			want: doctor.CodeTypeMisnamed,
		},
		{
			name: "a log line that is not an event",
			defect: func(t *testing.T, fx library) string {
				appendTo(t, filepath.Join(fx.dir, "reviews.jsonl"), `{"seq": 9, "rating": "wrong"}`+"\n")
				return "reviews.jsonl"
			},
			want: doctor.CodeReviewMalformed,
		},
		{
			name: "a last line a write never finished",
			defect: func(t *testing.T, fx library) string {
				appendTo(t, filepath.Join(fx.dir, "reviews.jsonl"), `{"seq": 9`)
				return "reviews.jsonl"
			},
			want: doctor.CodeReviewTruncated,
		},
		{
			name: "a sequence that does not count up",
			defect: func(t *testing.T, fx library) string {
				appendTo(t, filepath.Join(fx.dir, "reviews.jsonl"),
					`{"seq": 1, "card": "`+string(fx.block)+`", "rating": "good", "at": "2026-11-08T09:00:00Z"}`+"\n")
				return "reviews.jsonl"
			},
			want: doctor.CodeReviewSeq,
		},
		{
			name: "a void that cancels no review",
			defect: func(t *testing.T, fx library) string {
				appendTo(t, filepath.Join(fx.dir, "reviews.jsonl"), `{"seq": 3, "void": 99}`+"\n")
				return "reviews.jsonl"
			},
			want: doctor.CodeReviewVoid,
		},
		{
			name: "a note that names an asset the library does not hold",
			defect: func(t *testing.T, fx library) string {
				remove(t, filepath.Join(fx.dir, filepath.FromSlash(fx.asset)))
				return fx.note
			},
			want: doctor.CodeAssetMissing,
		},
		{
			name: "an asset no note names",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, "assets", "orphan.png"), "nobody's picture")
				return "assets/orphan.png"
			},
			want: doctor.CodeAssetOrphan,
		},
		{
			name: "no derived index",
			defect: func(t *testing.T, fx library) string {
				removeDir(t, filepath.Join(fx.dir, ".remit"))
				return ".remit/index.sqlite"
			},
			want: doctor.CodeIndexMissing,
		},
		{
			name: "an index another build wrote",
			defect: func(t *testing.T, fx library) string {
				// The index's own format, written as an older build would have:
				// the checker reads the schema marker an open would rebuild for.
				db, err := sql.Open("sqlite", "file:"+filepath.Join(fx.dir, ".remit", "index.sqlite"))
				if err != nil {
					t.Fatalf("open index: %v", err)
				}
				if _, err := db.Exec(`UPDATE meta SET value = '1' WHERE key = 'schema'`); err != nil {
					db.Close()
					t.Fatalf("downgrade schema: %v", err)
				}
				if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
					db.Close()
					t.Fatalf("checkpoint: %v", err)
				}
				if err := db.Close(); err != nil {
					t.Fatalf("close index: %v", err)
				}
				return ".remit/index.sqlite"
			},
			want: doctor.CodeIndexSchema,
		},
		{
			name: "a temporary file a write left behind",
			defect: func(t *testing.T, fx library) string {
				write(t, filepath.Join(fx.dir, ".remit", ".tmp-4417"), "half a write")
				return ".remit/.tmp-4417"
			},
			want: doctor.CodeIndexTemp,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newLibrary(t)
			wantPath := tc.defect(t, fx)

			report, err := doctor.Check(fx.dir)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if len(report.Findings) != 1 {
				t.Fatalf("findings = %+v, want exactly one", report.Findings)
			}
			found := report.Findings[0]
			if found.Code != tc.want {
				t.Errorf("code = %q, want %q (%s)", found.Code, tc.want, found.Detail)
			}
			if found.Path != wantPath {
				t.Errorf("path = %q, want %q", found.Path, wantPath)
			}
			if found.Detail == "" {
				t.Error("the finding says nothing about what it saw")
			}
		})
	}
}

// The same damage always prints the same report, in path-and-code order.
func TestFindingsAreOrderedAndRepeatable(t *testing.T) {
	fx := newLibrary(t)
	write(t, filepath.Join(fx.dir, "notes", "zz-loose.md"), "# Loose\n\nno id\n")
	write(t, filepath.Join(fx.dir, "cards", "aa-loose.json"), "{}")
	appendTo(t, filepath.Join(fx.dir, "reviews.jsonl"), `{"seq": 9, "rating": "wrong"}`+"\n")
	write(t, filepath.Join(fx.dir, ".remit", ".tmp-1"), "half a write")

	first, err := doctor.Check(fx.dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(first.Findings) < 4 {
		t.Fatalf("findings = %+v, want the four defects", first.Findings)
	}
	for i := 1; i < len(first.Findings); i++ {
		a, b := first.Findings[i-1], first.Findings[i]
		if a.Path > b.Path || a.Path == b.Path && a.Code > b.Code {
			t.Errorf("finding %d (%s %s) sorts before %d (%s %s)",
				i-1, a.Path, a.Code, i, b.Path, b.Code)
		}
	}

	second, err := doctor.Check(fx.dir)
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if fmt.Sprint(first.Findings) != fmt.Sprint(second.Findings) {
		t.Errorf("two checks disagree:\n%+v\n%+v", first.Findings, second.Findings)
	}
}

func TestFixRebuildsTheIndexAndKeepsEverySourceByte(t *testing.T) {
	fx := newLibrary(t)
	before := snapshot(t, fx.dir, false)
	removeDir(t, filepath.Join(fx.dir, ".remit"))

	report, err := doctor.Check(fx.dir)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Code != doctor.CodeIndexMissing || !report.Findings[0].Fixable {
		t.Fatalf("findings = %+v, want the one repairable missing index", report.Findings)
	}

	fixed, err := doctor.Fix(fx.dir)
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !fixed.Clean() {
		t.Fatalf("findings after the repair: %+v", fixed.Findings)
	}
	if _, err := os.Stat(filepath.Join(fx.dir, ".remit", "index.sqlite")); err != nil {
		t.Fatalf("the index was not built again: %v", err)
	}
	sameTree(t, before, snapshot(t, fx.dir, false))
}

func TestFixLeavesADamagedSourceAlone(t *testing.T) {
	fx := newLibrary(t)
	cardPath := filepath.Join(fx.dir, filepath.FromSlash(fx.card))
	write(t, cardPath, "{ not a card at all\n")
	before := snapshot(t, fx.dir, true)

	report, err := doctor.Fix(fx.dir)
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !hasCode(report, doctor.CodeCardInvalid) {
		t.Fatalf("findings = %+v, want the damaged card", report.Findings)
	}
	for _, f := range report.Findings {
		if f.Fixable {
			t.Errorf("%s is called fixable, but its source is never rewritten", f.Code)
		}
	}
	sameTree(t, before, snapshot(t, fx.dir, true))
}

func TestFixRefusesToBuildAnIndexOverADamagedSource(t *testing.T) {
	fx := newLibrary(t)
	write(t, filepath.Join(fx.dir, filepath.FromSlash(fx.card)), "{ not a card at all\n")
	removeDir(t, filepath.Join(fx.dir, ".remit"))

	report, err := doctor.Fix(fx.dir)
	if err == nil {
		t.Fatal("Fix built an index from a card the store cannot read")
	}
	if !hasCode(report, doctor.CodeCardInvalid) {
		t.Errorf("the report lost the reason: %+v", report.Findings)
	}
}

func TestFixDoesNotSeedAMissingBuiltInType(t *testing.T) {
	fx := newLibrary(t)
	typeFile := filepath.Join(fx.dir, "types", string(core.TypeBasic)+".json")
	remove(t, typeFile)
	removeDir(t, filepath.Join(fx.dir, ".remit"))

	report, err := doctor.Fix(fx.dir)
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !hasCode(report, doctor.CodeCardType) {
		t.Fatalf("findings = %+v, want the card whose type is gone", report.Findings)
	}
	// An open seeds the built-ins; a repair does not, because a type is a
	// source file and Fix never writes one.
	if _, err := os.Stat(typeFile); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Fix wrote a type file an open would have seeded: %v", err)
	}
}

func TestFixRemovesAStaleTemporaryFile(t *testing.T) {
	fx := newLibrary(t)
	temp := filepath.Join(fx.dir, ".remit", ".tmp-77")
	write(t, temp, "half a write")

	report, err := doctor.Fix(fx.dir)
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !report.Clean() {
		t.Fatalf("findings after the repair: %+v", report.Findings)
	}
	if _, err := os.Stat(temp); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the temporary file is still there: %v", err)
	}
}

// cardBody is a card file a person could have written: the store's format, with
// only the fields DecodeCard insists on.
func cardBody(block string, typ core.ID) string {
	return fmt.Sprintf(`{
  "block": %q,
  "type": %q,
  "status": "new",
  "created": "2026-11-08T09:00:00Z"
}
`, block, typ)
}

func hasCode(report doctor.Report, code string) bool {
	for _, f := range report.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("make %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func appendTo(t *testing.T, path, body string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if _, err := file.WriteString(body); err != nil {
		file.Close()
		t.Fatalf("append %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

func remove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

func removeDir(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

// snapshot reads every file under dir, by relative path. The derived index is
// left out unless withIndex asks for it: a repair may rewrite it, and a source
// may not.
func snapshot(t *testing.T, dir string, withIndex bool) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if !withIndex && rel == ".remit" {
				return fs.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return files
}

// sameTree fails for every file whose content differs, and for every file that
// appeared or vanished.
func sameTree(t *testing.T, want, got map[string][]byte) {
	t.Helper()
	for path, data := range want {
		next, ok := got[path]
		if !ok {
			t.Errorf("%s is gone", path)
			continue
		}
		if string(next) != string(data) {
			t.Errorf("%s was rewritten:\n%q\n%q", path, data, next)
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("%s appeared", path)
		}
	}
}
