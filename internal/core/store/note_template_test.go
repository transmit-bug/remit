package store

import (
	"errors"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
)

func templateLibrary(t *testing.T) *Library {
	t.Helper()
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	lib, err := Create(t.TempDir(), Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return lib
}

// saveNoteWithTags writes a note whose blocks carry the tags given in their
// text, answering with it as stored.
func saveNoteWithTags(t *testing.T, lib *Library, title string, texts ...string) core.Note {
	t.Helper()
	blocks := make([]core.Block, len(texts))
	for i, text := range texts {
		blocks[i] = core.Block{Text: text}
	}
	note, err := lib.SaveNote(core.Note{Title: title, Blocks: blocks})
	if err != nil {
		t.Fatalf("save %q: %v", title, err)
	}
	return note
}

// IsTemplate is the #template tag on any block, and nothing else: a note
// without it is a document, one with it is a starting point.
func TestIsTemplate(t *testing.T) {
	plain := core.Note{Blocks: []core.Block{{Text: "a plain paragraph"}, {Text: "another"}}}
	if core.IsTemplate(plain) {
		t.Error("a plain note reads as a template")
	}
	tagged := core.Note{Blocks: []core.Block{
		{Text: "prose #template"},
		{Text: "Reading asks :: answers"},
	}}
	if !core.IsTemplate(tagged) {
		t.Error("a #template note does not read as a template")
	}
	// A # inside a code span is code, not a tag: the note stays a document.
	coded := core.Note{Blocks: []core.Block{{Text: "run `echo #template`"}}}
	if core.IsTemplate(coded) {
		t.Error("a #template inside code made the note a template")
	}
}

// NoteFromTemplate copies the template's text under fresh IDs: the marker-only
// block is the template's role and is not copied, content blocks lose only the
// tag, the template's blocks keep their IDs, the copy holds none of them, and
// one template starts two independent notes.
func TestNoteFromTemplateRecastsIDs(t *testing.T) {
	lib := templateLibrary(t)
	template := saveNoteWithTags(t, lib, "Reading excerpt",
		"#template",
		"Source :: answer one",
		"Chapter asks #template :: chapter answers",
	)

	first, err := lib.NoteFromTemplate(template.ID, "One")
	if err != nil {
		t.Fatalf("first copy: %v", err)
	}
	second, err := lib.NoteFromTemplate(template.ID, "Two")
	if err != nil {
		t.Fatalf("second copy: %v", err)
	}

	if first.ID == second.ID || first.ID == template.ID {
		t.Fatalf("copies share an identity: %q %q %q", template.ID, first.ID, second.ID)
	}
	// The marker-only block is gone; the tagged content block keeps its words.
	if len(first.Blocks) != 2 || len(second.Blocks) != 2 {
		t.Fatalf("a copy holds %d and %d blocks, want the template's two content blocks", len(first.Blocks), len(second.Blocks))
	}
	if first.Blocks[0].Text != "Source :: answer one" {
		t.Fatalf("the copy's first block reads %q", first.Blocks[0].Text)
	}
	if core.IsTemplate(first) || core.IsTemplate(second) {
		t.Fatal("a copy is itself a template")
	}
	content := template.Blocks[2].Text
	if want := "Chapter asks :: chapter answers"; first.Blocks[1].Text != want {
		t.Fatalf("the tagged block copies as %q, want %q (from %q)", first.Blocks[1].Text, want, content)
	}
	for i, block := range template.Blocks {
		for _, copied := range first.Blocks {
			if copied.ID == block.ID {
				t.Errorf("copy's block %d kept the template's ID %q", i, block.ID)
			}
		}
	}
	for i := range first.Blocks {
		if first.Blocks[i].ID == second.Blocks[i].ID {
			t.Errorf("the two copies share block ID %q", first.Blocks[i].ID)
		}
	}

	// The template itself is untouched, so it can start another note later.
	after, ok := lib.Note(template.ID)
	if !ok {
		t.Fatal("the template vanished")
	}
	if len(after.Blocks) != 3 {
		t.Fatalf("the template holds %d blocks, want 3", len(after.Blocks))
	}
}

// A template is library data like any note: the reading queue and the tags
// still reach it; only the notes search leaves it out.
func TestTemplateFilteringFaces(t *testing.T) {
	lib := templateLibrary(t)
	template := saveNoteWithTags(t, lib, "Reading excerpt",
		"#template",
		"a distinct phrase to find :: answer",
	)
	plain := saveNoteWithTags(t, lib, "Plain", "another distinct phrase :: answer")

	found, err := lib.SearchNotes("distinct phrase")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(found) != 1 || found[0].ID != plain.ID {
		t.Fatalf("search = %v, want only the plain note", found)
	}

	// Tags still count the template: the sidebar's counts are not a filter face.
	tags := lib.Tags()
	hasTemplateTag := false
	for _, tag := range tags {
		if tag.Tag == core.TemplateTag {
			hasTemplateTag = tag.Notes == 1
		}
	}
	if !hasTemplateTag {
		t.Fatalf("tags = %v, want one note carrying #template", tags)
	}
	if got := lib.NotesWithTag(core.TemplateTag); len(got) != 1 || got[0] != template.ID {
		t.Fatalf("tag lookup = %v, want the template", got)
	}

	// Every note is still listed: the notebook's own filter is the UI's, the
	// store lists everything so an export or the agent API sees the truth.
	if got := lib.Notes(); len(got) != 2 {
		t.Fatalf("Notes = %d notes, want both", len(got))
	}
}

// Copying from something that is not a template is refused, and a missing
// template is an error, not an empty note.
func TestNoteFromTemplateRefusesNonTemplates(t *testing.T) {
	lib := templateLibrary(t)
	plain := saveNoteWithTags(t, lib, "Plain", "just words")

	if _, err := lib.NoteFromTemplate(plain.ID, "Copy"); !errors.Is(err, ErrNotATemplate) {
		t.Fatalf("copying a plain note: err = %v, want ErrNotATemplate", err)
	}
	if _, err := lib.NoteFromTemplate("note_nosuch", "Copy"); err == nil {
		t.Fatal("copying a missing note gave no error")
	}
	if got := lib.Notes(); len(got) != 1 {
		t.Fatalf("refusals wrote notes: %d notes held", len(got))
	}
}

// WithoutTemplateTag takes the marker out and nothing else: a text without the
// tag is unchanged, one with it loses only the tag's token.
func TestWithoutTemplateTag(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"untouched", "a plain paragraph", "a plain paragraph"},
		{"alone on the line", "#template", ""},
		{"leading", "#template Source :: answer", "Source :: answer"},
		{"trailing", "Chapter asks #template", "Chapter asks"},
		{"mid-line", "one #template two", "one two"},
		{"lookalike kept", "the #templateTag is not the marker", "the #templateTag is not the marker"},
		{"code kept", "run `echo #template`", "run `echo #template`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := core.WithoutTemplateTag(tc.text); got != tc.want {
				t.Fatalf("WithoutTemplateTag(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}
