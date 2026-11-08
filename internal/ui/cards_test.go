package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/locales"
)

// newNoteTyped starts a fresh note and writes body into its document, which is
// where the card gesture happens.
func newNoteTyped(t *testing.T, a *App, tt *ui.Tester, body string) {
	t.Helper()
	notesMode(t, a, tt)
	if err := tt.Click("New note"); err != nil {
		t.Fatalf("new note: %v", err)
	}
	if err := tt.Click("Note"); err != nil {
		t.Fatalf("open the document: %v", err)
	}
	tt.Type(body)
	tt.Advance(320 * time.Millisecond)
}

// Typing the separator is what makes a paragraph a card: the badge appears with
// the count, and the card points at the paragraph. The library holds the card,
// never the paragraph's faces.
func TestSeparatorMakesACard(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)
	newNoteTyped(t, a, tt, "What is spacing? :: Practice spread over days.")

	badge := msgs.CardOnParagraph(1, 1)
	if !tt.HasText(badge) {
		t.Fatalf("the paragraph's badge did not appear; texts %q", tt.Texts())
	}
	cards := a.Library().Cards()
	if len(cards) != 5 {
		t.Fatalf("the library holds %d cards, want the seeded four and the new one", len(cards))
	}
	// The card reads its paragraph, and its faces live nowhere on disk.
	note, ok := a.Library().Note(a.notes.Draft.ID)
	if !ok {
		t.Fatal("the new note is gone")
	}
	onDisk, err := os.ReadFile(filepath.Join(seed.dir, "notes", noteFile(a, note.ID)))
	if err == nil {
		if strings.Contains(string(onDisk), `"what is spacing?`) {
			t.Errorf("the note file holds a card face:\n%s", onDisk)
		}
	}
	card, ok := a.Library().Card(cards[len(cards)-1].ID)
	if !ok {
		t.Fatal("the new card is gone")
	}
	prompt, answer, state := a.faces(card)
	if state != core.LinkLive || prompt != "What is spacing?" || answer != "Practice spread over days." {
		t.Errorf("the card reads %q / %q (%s)", prompt, answer, state)
	}
}

// Taking the separator out again takes the card out of the queue: it is
// suspended, its history kept, and its badge is gone.
func TestRemovingTheSeparatorSuspendsTheCard(t *testing.T) {
	a, _, tt := openApp(t)
	newNoteTyped(t, a, tt, "What is spacing? :: Practice spread over days.")

	cards := a.Library().Cards()
	if len(cards) == 0 {
		t.Fatal("no card was made")
	}
	made := cards[len(cards)-1]

	if err := tt.Click("Note"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("What is spacing?")
	tt.Advance(320 * time.Millisecond)

	after, ok := a.Library().Card(made.ID)
	if !ok {
		t.Fatal("the card was deleted; the rules keep it")
	}
	// The paragraph is still there, so the card keeps its reference, but it no
	// longer asks anything: the reconcile takes it out of the queue.
	if after.Status != core.StatusSuspended {
		t.Errorf("the card is %s, want suspended", after.Status)
	}
	if tt.HasText(locales.In("en").CardOnParagraph(1, 1)) {
		t.Errorf("the badge stayed after the separator left; texts %q", tt.Texts())
	}
}

// The badge edits the card's own prompt and answer, and what it writes is what
// the store holds.
func TestCardBadgeEditsOverrides(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	newNoteTyped(t, a, tt, "What is spacing? :: Practice spread over days.")

	cards := a.Library().Cards()
	if len(cards) == 0 {
		t.Fatal("no card was made")
	}
	made := cards[len(cards)-1]

	if err := tt.Click(msgs.CardOnParagraph(1, 1)); err != nil {
		t.Fatalf("open the card's overrides: %v", err)
	}
	if !tt.HasText(msgs.EditCard()) {
		t.Fatalf("the dialog did not open; texts %q", tt.Texts())
	}
	if err := tt.Click("Front"); err != nil {
		t.Fatal(err)
	}
	tt.Type("What is spaced practice?")
	if err := tt.Click(msgs.Save()); err != nil {
		t.Fatal(err)
	}

	after, ok := a.Library().Card(made.ID)
	if !ok {
		t.Fatal("the card is gone")
	}
	if after.Fields["Front"] != "What is spaced practice?" || !after.Edited {
		t.Errorf("the override is %v, want what the dialog took", after.Fields)
	}
	if prompt, _, _ := a.faces(after); prompt != "What is spaced practice?" {
		t.Errorf("the card reads %q, want the override", prompt)
	}
}

// noteFile is the note's file name under notes/: the slug and the id.
func noteFile(a *App, id core.ID) string {
	entries, err := os.ReadDir(filepath.Join(a.Settings().LibraryDir, "notes"))
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "-"+string(id)+".md") {
			return entry.Name()
		}
	}
	return ""
}

// A paragraph with two blanks is two cards, each asking its own blank, and both
// can be graded.
func TestClozeMakesOneCardPerBlank(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	newNoteTyped(t, a, tt, "The {{first}} blank asks {{second}}.")

	if !tt.HasText(msgs.CardOnParagraph(1, 2)) {
		t.Fatalf("the paragraph's badge does not say two cards; texts %q", tt.Texts())
	}
	var cloze []core.Card
	for _, card := range a.Library().Cards() {
		if card.Type == core.TypeCloze {
			cloze = append(cloze, card)
		}
	}
	if len(cloze) != 2 {
		t.Fatalf("the paragraph made %d cloze cards, want 2", len(cloze))
	}
	// Each asks its own blank; the faces follow the paragraph, so nothing about
	// the pair is stored twice.
	prompt, answer, _ := a.faces(cloze[0])
	if prompt != "The […] blank asks second." || answer != "The first blank asks second." {
		t.Errorf("the first card asks %q / %q", prompt, answer)
	}
	prompt, answer, _ = a.faces(cloze[1])
	if prompt != "The first blank asks […]." || answer != "The first blank asks second." {
		t.Errorf("the second card asks %q / %q", prompt, answer)
	}

	// Both are due, so the queue holds both and each takes a grade.
	due, err := a.Library().Due(time.Now(), 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, card := range due {
		if card.Type == core.TypeCloze {
			seen++
		}
	}
	if seen != 2 {
		t.Fatalf("the queue holds %d cloze cards, want 2", seen)
	}
}

// A note's block becomes a card in the library, split at its first ::, and
// joins the queue.
func TestMakeCard(t *testing.T) {
	_, seed, tt := openApp(t)
	tt.Key(ui.Cmd, ui.Key2)

	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("make card")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("Card made from block 1") {
		t.Fatalf("no outcome for making a card; texts %q", tt.Texts())
	}
	cards, err := os.ReadDir(filepath.Join(seed.dir, "cards"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 5 {
		t.Errorf("the library holds %d card files, want 5", len(cards))
	}

	tt.Advance(5 * time.Second) // past the toast
	tt.Key(ui.Cmd, ui.Key1)
	if !tt.HasText("5 cards left") {
		t.Fatalf("the card made now is not in today's queue; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the card's entry
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
