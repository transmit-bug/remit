package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// linkedLibrary writes two notes, the second naming the first.
func linkedLibrary(t *testing.T) (*store.Library, string, core.Note, core.Note) {
	t.Helper()
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	target, err := lib.SaveNote(core.Note{Title: "Spacing", Blocks: []core.Block{{Text: "spacing :: the gaps between reviews"}}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := lib.SaveNote(core.Note{Title: "Habits", Blocks: []core.Block{
		{Text: "see [[Spacing]] for the why"},
		{Text: "another paragraph, no link"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return lib, dir, target, source
}

// A link is indexed when the note is saved, and the note it names can find it.
func TestBacklinksFollowTheSave(t *testing.T) {
	lib, _, target, source := linkedLibrary(t)
	defer lib.Close()

	links := lib.Backlinks(target.ID)
	if len(links) != 1 {
		t.Fatalf("the note has %d backlinks, want 1", len(links))
	}
	got := links[0]
	if got.Note != source.ID || got.NoteTitle != "Habits" || got.Block != source.Blocks[0].ID {
		t.Errorf("the backlink is %+v, want Habits' first paragraph", got)
	}
	if got.Paragraph != 1 {
		t.Errorf("the backlink is at paragraph %d, want 1", got.Paragraph)
	}
	if want := "see [[Spacing]] for the why"; got.Text != want {
		t.Errorf("the backlink reads %q, want %q", got.Text, want)
	}

	// A note does not link to itself, and an unlinked note has none.
	if self := lib.Backlinks(source.ID); len(self) != 0 {
		t.Errorf("the source links to itself: %+v", self)
	}

	// Taking the link out of the text takes it out of the index.
	source.Blocks[0].Text = "no link any more"
	if _, err := lib.SaveNote(source); err != nil {
		t.Fatal(err)
	}
	if links := lib.Backlinks(target.ID); len(links) != 0 {
		t.Errorf("the link outlived its text: %+v", links)
	}
}

// The index is a cache: deleting it and reopening rebuilds the same backlinks.
func TestBacklinksSurviveARebuiltIndex(t *testing.T) {
	lib, dir, target, _ := linkedLibrary(t)
	before := lib.Backlinks(target.ID)
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, ".remit", "index.sqlite")
	if err := os.Remove(path); err != nil {
		t.Fatalf("delete the index: %v", err)
	}
	again, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()

	after := again.Backlinks(target.ID)
	if len(after) != len(before) {
		t.Fatalf("the rebuilt index holds %d backlinks, was %d", len(after), len(before))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("backlink %d is %+v, was %+v", i, after[i], before[i])
		}
	}
}

// A title is matched the way it is written: trimmed, with runs of space
// collapsed and the case folded.
func TestBacklinksMatchTitlesLoosely(t *testing.T) {
	lib, _, target, _ := linkedLibrary(t)
	defer lib.Close()

	source, err := lib.SaveNote(core.Note{Title: "Loose", Blocks: []core.Block{{Text: "see [[  SPACING  ]] too"}}})
	if err != nil {
		t.Fatal(err)
	}
	links := lib.Backlinks(target.ID)
	if len(links) != 2 {
		t.Fatalf("the note has %d backlinks, want 2: %+v", len(links), links)
	}
	found := false
	for _, link := range links {
		if link.Note == source.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("the loosely written title did not link: %+v", links)
	}
}
