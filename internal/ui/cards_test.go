package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
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

// A paragraph's badge opens its inspector, and a card row there opens the card's
// own detail: the road from the paper to a card's prompt and answer.
func TestCardBadgeReachesTheCardDetail(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	newNoteTyped(t, a, tt, "What is spacing? :: Practice spread over days.")

	cards := a.Library().Cards()
	if len(cards) == 0 {
		t.Fatal("no card was made")
	}
	made := cards[len(cards)-1]

	if err := tt.Click(msgs.CardOnParagraph(1, 1)); err != nil {
		t.Fatalf("open the block inspector: %v", err)
	}
	if !tt.HasText(msgs.BlockInspector()) {
		t.Fatalf("the inspector did not open; texts %q", tt.Texts())
	}
	if err := tt.Click("What is spacing?"); err != nil {
		t.Fatalf("the inspector has no card row: %v", err)
	}
	if !tt.HasText(msgs.CardHistory()) {
		t.Fatalf("the card detail did not open; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.EditCard()); err != nil {
		t.Fatalf("the card has no way to its fields: %v", err)
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

	if !tt.HasText(msgs.CardOnParagraph(2, 1)) {
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
	openNote(t, tt, "A daily habit")

	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("make card")
	tt.Key(0, ui.KeyEnter)
	// The note opens with the caret at its end, so the card is made from its
	// last paragraph.
	if !tt.HasText("Card made from block 2") {
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
	recallQueue(tt)
	if !tt.HasText("5 cards left") {
		t.Fatalf("the card made now is not in today's queue; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the card's entry
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// The editor's fields are its type's: choosing another type rebuilds them from
// that type's template, what the fields hold is what the store keeps, and a
// paragraph that is not a card yet says so instead of offering a delete. The
// smallest window and the other language are the same dialog, because the panel
// is a measure rather than a column.
func TestCardEditorWritesTheFieldsOfItsType(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			locale := "en"
			if size.name == "minimum" {
				locale = "zh-Hans"
			}
			msgs := locales.In(locale)
			a, seed := newTestApp(t)

			// The badge and the block inspector open this dialog in the app; the
			// test opens it the way they do, on the paragraph the card reads.
			edit := openCardEditor(a, seed.memory.Blocks[0].ID)
			tt := ui.NewTester(edit, size.w, size.h)
			tt.SetLocale(locale)
			settle(tt)

			if !tt.HasText(msgs.EditCard()) {
				t.Fatalf("the editor did not open; texts %q", tt.Texts())
			}
			// A basic card asks two things, so the editor holds two fields.
			assertControl(t, tt, size.w, size.h, msgs.CardType())
			for _, field := range []string{"Front", "Back"} {
				assertControl(t, tt, size.w, size.h, field)
			}

			// The type decides the fields: a cloze card asks one, and the two the
			// basic card asked are gone.
			if err := tt.Click("Basic"); err != nil {
				t.Fatalf("open the type selector: %v", err)
			}
			if err := tt.Click("Cloze"); err != nil {
				t.Fatalf("choose a type: %v", err)
			}
			tt.Frame()
			if _, ok := tt.Find("Front"); ok {
				t.Fatalf("the old type's fields are still drawn; texts %q", tt.Texts())
			}
			assertControl(t, tt, size.w, size.h, "Text")

			if err := tt.Click("Text"); err != nil {
				t.Fatalf("the cloze field: %v", err)
			}
			tt.Type("Spacing beats cramming")
			if err := tt.Click(msgs.Save()); err != nil {
				t.Fatalf("save the card: %v", err)
			}

			card, ok := a.Library().Card(seed.spacing.ID)
			if !ok {
				t.Fatal("the card is gone")
			}
			if card.Type != core.TypeCloze {
				t.Errorf("the card is %s, want the type the dialog chose", card.Type)
			}
			if card.Fields["Text"] != "Spacing beats cramming" {
				t.Errorf("the card's fields are %v, want what the dialog took", card.Fields)
			}
			if tt.FramePending() {
				t.Error("the window still asks for frames after the dialog closed")
			}

			// A paragraph that feeds no card has nothing to delete, and says so
			// rather than leaving a control that does nothing.
			plain := seed.habit.Blocks[1]
			empty := openCardEditor(a, plain.ID)
			bare := ui.NewTester(empty, size.w, size.h)
			bare.SetLocale(locale)
			bare.Frame()
			if !bare.HasText(msgs.CardNotACard()) {
				t.Fatalf("a paragraph without a card does not say so; texts %q", bare.Texts())
			}
			assertControl(t, bare, size.w, size.h, msgs.CardDelete())
			// The delete is there and disabled: pressing it asks nothing.
			if err := bare.Click(msgs.CardDelete()); err != nil {
				t.Fatalf("the delete control: %v", err)
			}
			if a.cardEdit.asking {
				t.Error("a delete was offered for a paragraph that is not a card")
			}
		})
	}
}

// openCardEditor is a view that opens the card editor on a block before the
// window draws, which is what the badge and the block inspector do from a
// handler. A view is how a test reaches a dialog with no control of its own in
// the frame it is opened from.
func openCardEditor(a *App, block core.ID) func(*ui.Context) {
	opened := false
	return func(c *ui.Context) {
		if !opened {
			opened = true
			a.openCardEditor(c, block)
		}
		a.View(c)
	}
}

// flipSwitch turns a switch on by its accessible name: a switch carries no text
// of its own, so its name is what the pointer is aimed through.
func flipSwitch(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	info, ok := tt.AccessByRole(ui.RoleSwitch, label)
	if !ok {
		t.Fatalf("no switch is named %q", label)
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
}

// A card's detail is its source and its schedule history: every review it has
// had, oldest first, with the answer it was given and the interval that answer
// scheduled. A card nobody answered says so instead of showing an empty list.
func TestCardDetailListsTheScheduleHistory(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			locale := "en"
			if size.name == "minimum" {
				locale = "zh-Hans"
			}
			msgs := locales.In(locale)
			a, seed := newTestApp(t)
			// The card table's row and the block inspector's card row open this
			// dialog in the app; the test opens it the way they will.
			answered := false
			tt := ui.NewTester(func(c *ui.Context) {
				if !answered {
					answered = true
					a.openCardDetail(c, seed.spacing.ID)
				}
				a.View(c)
			}, size.w, size.h)
			tt.SetLocale(locale)
			settle(tt)

			if !tt.HasText(msgs.CardDetail()) {
				t.Fatalf("the card's detail did not open; texts %q", tt.Texts())
			}
			// The paragraph the card reads is shown before leaving for it.
			first := seed.memory.Blocks[0].Text
			if !tt.HasText(first) {
				t.Errorf("the detail does not show the card's source; texts %q", tt.Texts())
			}
			assertControl(t, tt, size.w, size.h, msgs.CardBackToSource())

			if len(a.cardDetail.rows) != 1 {
				t.Fatalf("the history holds %d rows, want the one review", len(a.cardDetail.rows))
			}
			if a.cardDetail.rows[0].Rating != schedule.Again {
				t.Errorf("the row says %v, want the answer that was given", a.cardDetail.rows[0].Rating)
			}
			if a.cardDetail.rows[0].Interval < 1 {
				t.Errorf("the row schedules %d days, want the interval the answer set", a.cardDetail.rows[0].Interval)
			}
			if !tt.HasText(msgs.Again()) {
				t.Errorf("the list does not name the answer; texts %q", tt.Texts())
			}
			if !tt.HasText(msgs.CardHistoryInterval(a.cardDetail.rows[0].Interval)) {
				t.Errorf("the list does not say what the answer scheduled; texts %q", tt.Texts())
			}

			// Back to source leaves the dialog and shows the note read at the
			// card's paragraph.
			if err := tt.Click(msgs.CardBackToSource()); err != nil {
				t.Fatalf("back to source: %v", err)
			}
			tt.Frame()
			if a.notes.View != notes.Read || a.notes.At() != 1 || a.notes.Draft.ID != seed.memory.ID {
				t.Errorf("the window is not on the card's paragraph: note %q, view %v, at %d",
					a.notes.Draft.ID, a.notes.View, a.notes.At())
			}
			if a.cardDetail.open {
				t.Error("the detail stayed open over the note")
			}

			// A card nobody answered yet says so, rather than showing an empty
			// list and looking broken.
			unanswered := false
			fresh := ui.NewTester(func(c *ui.Context) {
				if !unanswered {
					unanswered = true
					a.openCardDetail(c, seed.pending.ID)
				}
				a.View(c)
			}, size.w, size.h)
			fresh.SetLocale(locale)
			fresh.Frame()
			if !fresh.HasText(msgs.CardHistoryEmpty()) {
				t.Errorf("an unanswered card shows no empty state; texts %q", fresh.Texts())
			}
		})
	}
}

// The editor's other two controls write: the freeze switch stops the card
// following its paragraph, and the delete asks before it takes the card — and
// the history that goes with it — while the paragraph stays.
func TestCardEditorFreezesAndDeletesTheCard(t *testing.T) {
	msgs := locales.In("en")
	a, seed := newTestApp(t)
	tt := ui.NewTester(openCardEditor(a, seed.memory.Blocks[0].ID), 1024, 720)
	settle(tt)

	flipSwitch(t, tt, msgs.Frozen())
	tt.Frame()
	frozen, ok := a.Library().Card(seed.spacing.ID)
	if !ok || frozen.Ref.Frozen == nil {
		t.Fatalf("the card did not freeze: %+v", frozen.Ref)
	}
	if !tt.HasText(msgs.Frozen()) {
		t.Errorf("the editor does not say the card is frozen; texts %q", tt.Texts())
	}

	// The delete asks first, and a card that is not answered stays.
	if err := tt.Click(msgs.CardDelete()); err != nil {
		t.Fatalf("the delete: %v", err)
	}
	tt.Frame()
	if !tt.HasText(msgs.CardDeleteTitle()) {
		t.Fatalf("the delete did not ask; texts %q", tt.Texts())
	}
	if _, ok := a.Library().Card(seed.spacing.ID); !ok {
		t.Fatal("the card went before the question was answered")
	}
	if err := tt.Click(msgs.CardDeleteConfirm()); err != nil {
		t.Fatalf("the answer: %v", err)
	}
	tt.Frame()
	if _, ok := a.Library().Card(seed.spacing.ID); ok {
		t.Error("the card is still in the library after the answer")
	}
	if _, ok := a.Library().Note(seed.memory.ID); !ok {
		t.Error("the paragraph's note went with the card")
	}
	if a.cardEdit.open {
		t.Error("the editor stayed open over a card that is gone")
	}
}
