package store_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// clkStep is how far the tests move their clock between writes, so two notes do
// not share a save moment.
const clkStep = time.Minute

// sortedIDs is the order the rows carry a note in: by ID, so two reads agree.
func sortedIDs(ids ...core.ID) []core.ID {
	slices.Sort(ids)
	return ids
}

// orphanBody is an asset body nothing else shares, so PutAsset stores a file.
func orphanBody() []byte { return []byte("a file no note carries") }

// removeAsset deletes one file from the library's assets directory, which is what
// a person tidying the folder up does.
func removeAsset(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, "assets", name)); err != nil {
		t.Fatalf("remove asset %q: %v", name, err)
	}
}

// TestAssetRowsListsFilesAndTheirNotes is what the assets page shows: every file
// the library stores, its type and size, and the notes that reference it by
// relative path. A file nothing references is still a row; a reference to a file
// that is not there is not.
func TestAssetRowsListsFilesAndTheirNotes(t *testing.T) {
	lib, dir, clk := newLibrary(t)
	picture, err := lib.PutAsset([]byte("a picture"), "png")
	if err != nil {
		t.Fatalf("PutAsset: %v", err)
	}
	photo, err := lib.PutAsset([]byte("a photo"), "jpg")
	if err != nil {
		t.Fatalf("PutAsset: %v", err)
	}
	orphan, err := lib.PutAsset(orphanBody(), "gif")
	if err != nil {
		t.Fatalf("PutAsset: %v", err)
	}
	name := func(rel string) string { return strings.TrimPrefix(rel, "assets/") }

	illustrated := saveNote(t, lib, "Illustrated", "![a picture]("+picture+")")
	clk.advance(clkStep)
	mentioned := saveNote(t, lib, "Mentioned", "see "+photo+".", "and "+picture+" too")
	clk.advance(clkStep)
	saveNote(t, lib, "Missing", "![gone](assets/0000.png)")

	rows, err := lib.AssetRows()
	if err != nil {
		t.Fatalf("AssetRows: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want the three stored files", rows)
	}
	byName := map[string]store.AssetRow{}
	for _, row := range rows {
		byName[row.Name] = row
	}

	tests := []struct {
		label string
		name  string
		ext   string
		bytes int
		notes []core.ID
	}{
		{"a picture one note carries", name(picture), "png", len("a picture"), sortedIDs(illustrated.ID, mentioned.ID)},
		{"a photo mentioned by path", name(photo), "jpeg", len("a photo"), sortedIDs(mentioned.ID)},
		{"a file nothing references", name(orphan), "gif", len(orphanBody()), nil},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			row, ok := byName[tt.name]
			if !ok {
				t.Fatalf("%q is not listed: %+v", tt.name, rows)
			}
			if row.Ext != tt.ext {
				t.Errorf("ext = %q, want %q", row.Ext, tt.ext)
			}
			if row.Bytes != int64(tt.bytes) {
				t.Errorf("%d bytes, want %d", row.Bytes, tt.bytes)
			}
			if !slices.Equal(row.Notes, tt.notes) {
				t.Errorf("notes = %v, want %v", row.Notes, tt.notes)
			}
		})
	}

	t.Run("by name, so two reads agree", func(t *testing.T) {
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Name)
		}
		if !slices.IsSorted(names) {
			t.Errorf("names = %v, want them sorted", names)
		}
	})

	t.Run("a reference dropped is a note no longer listed", func(t *testing.T) {
		if _, err := lib.SaveNote(core.Note{ID: mentioned.ID, Title: mentioned.Title,
			Blocks: []core.Block{{Text: "no pictures now"}}}); err != nil {
			t.Fatalf("SaveNote: %v", err)
		}
		rows, err := lib.AssetRows()
		if err != nil {
			t.Fatalf("AssetRows: %v", err)
		}
		for _, row := range rows {
			if row.Name == name(picture) && !slices.Equal(row.Notes, sortedIDs(illustrated.ID)) {
				t.Errorf("picture notes = %v, want only the note that still carries it", row.Notes)
			}
			if row.Name == name(photo) && len(row.Notes) != 0 {
				t.Errorf("photo notes = %v, want none", row.Notes)
			}
		}
	})

	t.Run("a rebuilt index answers the same", func(t *testing.T) {
		before, err := lib.AssetRows()
		if err != nil {
			t.Fatalf("AssetRows: %v", err)
		}
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
		after, err := lib.AssetRows()
		if err != nil {
			t.Fatalf("AssetRows after a rebuild: %v", err)
		}
		if !slices.EqualFunc(before, after, sameAssetRow) {
			t.Errorf("rebuilt rows = %+v, want %+v", after, before)
		}
	})

	t.Run("a file deleted from the directory leaves the listing", func(t *testing.T) {
		removeAsset(t, dir, name(orphan))
		rows, err := lib.AssetRows()
		if err != nil {
			t.Fatalf("AssetRows: %v", err)
		}
		for _, row := range rows {
			if row.Name == name(orphan) {
				t.Fatalf("a deleted file is still listed: %+v", row)
			}
		}
	})
}

// TestAssetRowsInAnEmptyLibrary is the page's empty state, not an error.
func TestAssetRowsInAnEmptyLibrary(t *testing.T) {
	lib, _, _ := newLibrary(t)
	rows, err := lib.AssetRows()
	if err != nil {
		t.Fatalf("AssetRows: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
}

func sameAssetRow(a, b store.AssetRow) bool {
	return a.Name == b.Name && a.Ext == b.Ext && a.Bytes == b.Bytes && slices.Equal(a.Notes, b.Notes)
}
