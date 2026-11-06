package ui

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/settings"
)

// windowSizes are the sizes every mode has to hold at: the one the window
// opens at and the smallest it goes.
var windowSizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// testLibrary is the library a test starts from: three notes and four cards,
// written through the real store into a temporary directory, so what the
// window reads is what a later run would read.
type testLibrary struct {
	dir     string
	memory  core.Note // "Memory and recall"
	writing core.Note // "Writing cards"
	habit   core.Note // "A daily habit"

	// The queue's order: the three reviewed cards by due time, then the card
	// that has never been reviewed.
	spacing core.Card // memory's first block
	frozen  core.Card // writing's first block, stopped following it
	testing core.Card // memory's second block
	pending core.Card // writing's second block, new
}

// seedLibrary creates a library in dir.
func seedLibrary(t *testing.T, dir string) testLibrary {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	lib, err := store.Create(dir, store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	save := func(title string, texts ...string) core.Note {
		blocks := make([]core.Block, len(texts))
		for i, text := range texts {
			blocks[i] = core.Block{Text: text}
		}
		note, err := lib.SaveNote(core.Note{Title: title, Blocks: blocks})
		if err != nil {
			t.Fatalf("save note %q: %v", title, err)
		}
		return note
	}
	memory := save("Memory and recall",
		"Spacing beats cramming :: reviewing just before you forget strengthens recall.",
		"The testing effect :: recalling an answer builds a stronger memory than rereading it.",
		"Interleaving :: mixing topics in one session trains you to tell them apart.",
	)
	writing := save("Writing cards",
		"A good card asks :: one question with one answer.",
		"A block divides at :: the first :: — prompt before, answer after.",
		"Cards point at blocks :: editing the note updates every card that follows it.",
	)
	habit := save("A daily habit",
		"When to review :: when the queue says a card is due, in one short sitting.",
		"What to do when stuck :: make the card smaller instead of repeating it.",
	)

	card := func(block core.ID) core.Card {
		c, err := lib.CreateCard(block)
		if err != nil {
			t.Fatalf("create card: %v", err)
		}
		return c
	}
	spacing := card(memory.Blocks[0].ID)
	frozen := card(writing.Blocks[0].ID)
	testing := card(memory.Blocks[1].ID)
	pending := card(writing.Blocks[1].ID)

	// A frozen card shows what its block said when it stopped following it,
	// which is no longer what the block says.
	frozen.Ref.Frozen = &core.Snapshot{Text: "A good card asks :: one question at a time.", At: now}
	if err := lib.SaveCard(frozen); err != nil {
		t.Fatalf("freeze card: %v", err)
	}

	// Learning cards are queued by when they are due, so reviews a day apart
	// put the queue in the order of this list.
	for i, c := range []core.Card{spacing, frozen, testing} {
		if err := lib.Review(c.ID, schedule.Again, now.Add(time.Duration(i-3)*24*time.Hour)); err != nil {
			t.Fatalf("seed review: %v", err)
		}
	}
	return testLibrary{
		dir: dir, memory: memory, writing: writing, habit: habit,
		spacing: spacing, frozen: frozen, testing: testing, pending: pending,
	}
}

// settle draws the window's first frame and lets its entrance finish, so that a
// test asserting the view is at rest starts from a window that is.
func settle(tt *ui.Tester) {
	tt.Frame()
	tt.Advance(300 * time.Millisecond) // the entrance: two beats, on a spring
}

// newTestApp opens a window on a freshly seeded library.
func newTestApp(t *testing.T) (*App, testLibrary) {
	t.Helper()
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	return NewApp(Options{LibraryDir: dir}), seed
}

// A library opens on today's review queue, and the notes are one key away:
// the two modes are one window, switched by the real keys.
func TestModes(t *testing.T) {
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)

			// The first screen is the review queue (decision #11).
			for _, want := range []string{"Show answer", "Spacing beats cramming", "4 cards left"} {
				if !tt.HasText(want) {
					t.Fatalf("review mode: missing %q; texts %q", want, tt.Texts())
				}
			}
			for _, gone := range []string{"Notebook", "Make card", "A daily habit"} {
				if tt.HasText(gone) {
					t.Errorf("review mode shows %q", gone)
				}
			}

			tt.Key(ui.Cmd, ui.Key2)
			tt.Advance(320 * time.Millisecond) // the card's exit
			for _, want := range []string{"Notebook", "Memory and recall", "Note title"} {
				if !tt.HasText(want) {
					t.Fatalf("notes mode: missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.HasText("Show answer") {
				t.Error("review stayed up after Cmd+2")
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}

			// Cmd+1 comes back, and Esc leaves review for the notes.
			tt.Key(ui.Cmd, ui.Key1)
			if !tt.HasText("Show answer") {
				t.Fatalf("Cmd+1 did not return to review; texts %q", tt.Texts())
			}
			tt.Advance(320 * time.Millisecond) // the card's entry
			tt.Key(0, ui.KeyEscape)
			tt.Advance(320 * time.Millisecond) // the card's exit
			if !tt.HasText("Notebook") {
				t.Fatalf("Esc did not return to notes; texts %q", tt.Texts())
			}
			if tt.FramePending() {
				t.Error("view still requests frames after it should settle")
			}
		})
	}
}

// Review runs on keys: reveal, grade, the next card, undo, suspend and the
// remaining count, on the real view over the real store.
func TestReviewKeys(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key1)

	answer := "reviewing just before you forget strengthens recall."
	if !tt.HasText("Spacing beats cramming") {
		t.Fatalf("review did not open on the first card; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	if !tt.HasText(answer) {
		t.Fatalf("Space did not reveal the answer; texts %q", tt.Texts())
	}

	// Grading advances at once, to the frozen card.
	tt.Key(0, ui.Key1)
	if tt.HasText(answer) {
		t.Error("the graded card's answer is still shown")
	}
	for _, want := range []string{"Frozen", "A good card asks", "3 cards left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after grading; texts %q", want, tt.Texts())
		}
	}

	// A card cannot be graded before it is revealed.
	tt.Key(0, ui.Key3)
	if !tt.HasText("3 cards left") || !tt.HasText("Frozen") {
		t.Error("a grade moved the queue before the answer was revealed")
	}

	// The frozen card shows its snapshot, not the block's current text.
	if tt.HasText("one question with one answer.") {
		t.Error("the frozen card follows its block instead of its snapshot")
	}
	tt.Key(0, ui.KeySpace)
	if !tt.HasText("one question at a time.") {
		t.Fatalf("the frozen snapshot's answer is missing; texts %q", tt.Texts())
	}

	// Undo goes back one card, answer shown, and says so once.
	tt.Key(0, ui.KeyU)
	for _, want := range []string{answer, "4 cards left", "Grade undone"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after undo; texts %q", want, tt.Texts())
		}
	}

	// Grade again, then Easy moves on without the answer coming back up.
	tt.Key(0, ui.Key1)
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key4)
	for _, want := range []string{"The testing effect", "2 cards left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after grading; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("one question at a time.") {
		t.Error("the answer stayed up on the next card")
	}

	// S suspends the card shown; it leaves the queue without a review.
	tt.Key(0, ui.KeyS)
	for _, want := range []string{"A block divides at", "1 card left"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q after suspend; texts %q", want, tt.Texts())
		}
	}
	suspended, err := os.ReadFile(filepath.Join(seed.dir, "cards", string(seed.testing.ID)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(suspended), `"status": "suspended"`) {
		t.Errorf("suspending did not reach the card file: %s", suspended)
	}

	// E opens the note the card came from.
	tt.Key(0, ui.KeyE)
	if !tt.HasText("Notebook") || a.editor.loaded != string(seed.writing.ID) {
		t.Fatalf("E did not open the card's note; texts %q", tt.Texts())
	}

	tt.Advance(5 * time.Second) // past the undo toast and the card's exit
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A grade is a review event in the library, so a later run finds the card
// scheduled rather than due.
func TestReviewPersists(t *testing.T) {
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key1)
	tt.Key(0, ui.KeySpace)
	tt.Key(0, ui.Key3) // Good
	if !tt.HasText("3 cards left") {
		t.Fatalf("Good did not move the queue on; texts %q", tt.Texts())
	}

	// The log is the source of truth, and it holds the review.
	logged, err := os.ReadFile(filepath.Join(dir, "reviews.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), string(seed.spacing.ID)) || !strings.Contains(string(logged), `"rating":"good"`) {
		t.Fatalf("the review is not in the log: %s", logged)
	}

	// A second app on the same directory opens it without the graded card, so
	// the due date survived the write.
	next := NewApp(Options{LibraryDir: dir})
	tt = ui.NewTester(next.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key1)
	for _, want := range []string{"A good card asks", "3 cards left"} {
		if !tt.HasText(want) {
			t.Fatalf("reopened review is missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("Spacing beats cramming") {
		t.Error("the graded card is due again in the reopened library")
	}
}

// The palette opens on Cmd+K, filters as the user types, runs what is chosen
// and returns the focus where it was.
func TestPalette(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit

	const field = "Note title"
	if err := tt.Click(field); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused(field) {
		t.Fatalf("the note's title did not take the focus; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.KeyK)
	for _, want := range []string{"Review cards", "Edit notes", "New note"} {
		if !tt.HasText(want) {
			t.Fatalf("palette is missing %q; texts %q", want, tt.Texts())
		}
	}

	tt.Type("new")
	if !tt.HasText("New note") {
		t.Fatalf("the filtered palette lost the action; texts %q", tt.Texts())
	}
	for _, gone := range []string{"Review cards", "Edit notes"} {
		if tt.HasText(gone) {
			t.Errorf("filter kept %q", gone)
		}
	}

	tt.Key(0, ui.KeyEscape)
	for _, gone := range []string{"Search actions", "Review cards", "Edit notes"} {
		if tt.HasText(gone) {
			t.Errorf("palette stayed open after Esc: %q", gone)
		}
	}
	if !tt.Focused(field) {
		t.Errorf("Esc did not return the focus to the block field; texts %q", tt.Texts())
	}

	// Typing filters, Enter runs the action on the selection.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("new")
	tt.Key(0, ui.KeyEnter)
	if tt.HasText("Search actions") {
		t.Error("the palette stayed open after running its action")
	}
	if !tt.HasText("Untitled note") {
		t.Fatalf("New note did not add a note; texts %q", tt.Texts())
	}
	if !tt.Focused("Note title") {
		t.Errorf("a new note's title did not take the caret; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Editing a block's text is a note save, so the card that follows the block
// shows the new sides and the note file holds them.
func TestEditDocument(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")

	if err := tt.Click("Note"); err != nil {
		t.Fatalf("open the note's document: %v", err)
	}
	tt.Type("Read this first. ")
	tt.Advance(320 * time.Millisecond)

	source, err := os.ReadFile(filepath.Join(seed.dir, "notes", "memory-and-recall-"+string(seed.memory.ID)+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "Read this first.") {
		t.Fatalf("the edit did not reach the file:\n%s", source)
	}
	// Every paragraph keeps the marker that names it, so the card that points
	// at one still finds it.
	for _, block := range seed.memory.Blocks {
		if !strings.Contains(string(source), core.BlockMarker(block.ID)) {
			t.Errorf("block %s lost its marker:\n%s", block.ID, source)
		}
	}
	card, ok := a.Library().Card(seed.spacing.ID)
	if !ok {
		t.Fatal("the card is gone")
	}
	prompt, _, state := card.Sides(a.Library())
	if state != core.LinkLive {
		t.Errorf("the card is %s, want live", state)
	}
	// The caret sits at the document's end, so what was typed joined the last
	// paragraph; the card's own paragraph is untouched and still found.
	if !strings.Contains(string(source), "Read this first.") {
		t.Errorf("the typed text is not in the last paragraph:\n%s", source)
	}
	if prompt != "Spacing beats cramming" {
		t.Errorf("the card reads %q, want its own paragraph", prompt)
	}
}

// Choosing a note in the list shows that note's blocks.
func TestNotesList(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit

	if err := tt.Click("Writing cards"); err != nil {
		t.Fatal(err)
	}
	if a.editor.doc == nil || !strings.Contains(a.editor.doc.Text().String(), "A good card asks") {
		t.Fatalf("the editor did not follow the list; texts %q", tt.Texts())
	}
	if strings.Contains(a.editor.doc.Text().String(), "When to review") {
		t.Error("the editor still shows the note chosen before")
	}
	if !tt.HasText("2 blocks") {
		t.Errorf("the note's paragraph count is missing; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A note's block becomes a card in the library, split at its first ::, and
// joins the queue.
func TestMakeCard(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	tt.Key(ui.Cmd, ui.Key2)

	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("make card")
	tt.Key(0, ui.KeyEnter)
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
	tt.Key(ui.Cmd, ui.Key1)
	if !tt.HasText("5 cards left") {
		t.Fatalf("the card made now is not in today's queue; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the card's entry
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// At the smallest window every control is on screen and clickable, not only
// present as text.
func TestMinimumSizeControls(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)

	// The queue the window opens on is usable at the smallest size.
	assertControl(t, tt, 720, 520, "Show answer")
	tt.Advance(320 * time.Millisecond) // the card's entry

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	for _, label := range []string{"New note", "Notebook", "A daily habit", "Note title"} {
		assertControl(t, tt, 720, 520, label)
	}

	// Making a card is a command now, reachable at the smallest window too.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("make card")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("Card made from block 2") {
		t.Fatalf("Make card did not run at the minimum; texts %q", tt.Texts())
	}
	tt.Advance(5 * time.Second) // let the toast go

	tt.Key(ui.Cmd, ui.Key1)
	assertControl(t, tt, 720, 520, "Show answer")
	if err := tt.Click("Show answer"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Again", "Hard", "Good", "Easy", "Undo (U)", "Suspend (S)", "Edit note (E)"} {
		assertControl(t, tt, 720, 520, label)
	}
	if err := tt.Click("Good"); err != nil {
		t.Fatal(err)
	}
	assertControl(t, tt, 720, 520, "Show answer")

	tt.Key(ui.Cmd, ui.KeyK)
	assertControl(t, tt, 720, 520, "Search actions")
	assertControl(t, tt, 720, 520, "Review cards")
	if err := tt.Click("Review cards"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("Search actions") {
		t.Error("the palette row was not clickable at the minimum")
	}
	tt.Advance(320 * time.Millisecond) // the card's transition

	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// One page in the dark: the same views draw from the dark theme.
func TestDark(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	tt.SetDark(true)
	tt.Frame()
	if !tt.HasText("Show answer") {
		t.Fatalf("dark review mode is incomplete; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the card's exit
	if !tt.HasText("Notebook") || !tt.HasText("Note title") {
		t.Fatalf("dark notes mode is incomplete; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.Key1)
	tt.Key(0, ui.KeySpace)
	if !tt.HasText("reviewing just before you forget strengthens recall.") {
		t.Fatalf("dark review mode lost its content; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond)
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A first run has no library: the window asks whether to make one or open one,
// and only makes the one the user chose. The choice is remembered, so the next
// start opens the library without asking again.
func TestFirstRun(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Remit")
	data := t.TempDir()
	a := NewApp(Options{LibraryDir: dir, DataDir: data})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	if _, err := os.Stat(filepath.Join(dir, "remit.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the window made a library before it was asked: %v", err)
	}
	for _, want := range []string{"Start your library", "Create library", "Open library…", dir} {
		if !tt.HasText(want) {
			t.Fatalf("the start view is missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText("Notebook") {
		t.Error("the start view drew a library that is not there")
	}

	if err := tt.Click("Create library"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "remit.json")); err != nil {
		t.Fatalf("Create library did not make one: %v", err)
	}
	// A finished queue reports the session, not a bare "nothing here".
	if !tt.HasText("Today's review") || !tt.HasText("Back to notes") {
		t.Fatalf("a new library did not open on its empty queue; texts %q", tt.Texts())
	}

	// The choice is in the settings for the next start.
	s, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.LibraryDir != dir {
		t.Fatalf("the settings name %q, want %q", s.LibraryDir, dir)
	}

	// The next start opens the remembered library, not the start view.
	next := NewApp(Options{DataDir: data})
	tt = ui.NewTester(next.View, 1024, 720)
	tt.Frame()
	if tt.HasText("Start your library") {
		t.Fatalf("the remembered library was not opened; texts %q", tt.Texts())
	}
	if !tt.HasText("Today's review") {
		t.Fatalf("the remembered library's queue is not up; texts %q", tt.Texts())
	}

	if err := tt.Click("Back to notes"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("No notes yet.") {
		t.Fatalf("the empty library does not say it is empty; texts %q", tt.Texts())
	}
	if err := tt.Click("New note"); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused("Note title") {
		t.Fatalf("the first note did not open for typing; texts %q", tt.Texts())
	}

	third := NewApp(Options{DataDir: data})
	tt = ui.NewTester(third.View, 1024, 720)
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	if !tt.HasText("Untitled note") || !tt.HasText("Note title") {
		t.Fatalf("the new note did not survive a restart; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A library that cannot be opened is one line and one action, not a crash;
// the action opens it once the problem is gone.
func TestCorruptLibrary(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "remit.json")
	if err := os.WriteFile(config, []byte(`{"format": "remit/9"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewApp(Options{LibraryDir: dir})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	if !tt.HasText("Remit could not open its library") || !tt.HasText("remit/9") {
		t.Fatalf("the error state does not say what happened; texts %q", tt.Texts())
	}
	if tt.HasText("Notebook") {
		t.Error("the window drew a library it could not open")
	}
	assertControl(t, tt, 1024, 720, "Try again")

	if err := os.WriteFile(config, []byte(`{"format": "remit/1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Try again"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Today's review") || tt.HasText("Remit could not open its library") {
		t.Fatalf("trying again did not open the repaired library; texts %q", tt.Texts())
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// The About page names the app, shows its version and links its license at the
// size the window opens at.
func TestAbout(t *testing.T) {
	tt := ui.NewTester(About, aboutWidth, aboutHeight)
	settle(tt)

	for _, want := range []string{Name, "Version " + Version, "GNU AGPL-3.0"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("GNU AGPL-3.0"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 1 || got[0] != licenseURL {
		t.Errorf("license link opened %q, want %q", got, licenseURL)
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Packaging reads mygo.json's version and the About page shows Version: the
// two must not drift apart.
func TestVersionMatchesConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("mygo.json: %v", err)
	}
	if cfg.Version != Version {
		t.Errorf("mygo.json version %q, package Version %q", cfg.Version, Version)
	}
}

// assertControl checks that s has a box of its own inside the window, which is
// what makes it visible and clickable rather than only present as text.
func assertControl(t *testing.T, tt *ui.Tester, w, h int, s string) {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("%q has no box; texts %q", s, tt.Texts())
	}
	if r.W <= 0 || r.H <= 0 {
		t.Errorf("%q has an empty box %.1fx%.1f", s, r.W, r.H)
	}
	if r.X < 0 || r.Y < 0 || r.X+r.W > float32(w) || r.Y+r.H > float32(h) {
		t.Errorf("%q is outside the window: %.1f,%.1f %.1fx%.1f in %dx%d", s, r.X, r.Y, r.W, r.H, w, h)
	}
}

// Every message comes from a catalog, so switching the language switches what
// the view says, without a restart and without a second code path.
func TestLocaleSwitchesTheLanguage(t *testing.T) {
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)

	tt.SetLocale("zh-Hans")
	tt.Frame()
	for _, want := range []string{"显示答案", "撤销 (U)"} {
		if !tt.HasText(want) {
			t.Errorf("missing Chinese text %q; texts %q", want, tt.Texts())
		}
	}

	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	for _, want := range []string{"笔记", "笔记本"} {
		if !tt.HasText(want) {
			t.Errorf("missing Chinese text %q in the notes; texts %q", want, tt.Texts())
		}
	}

	tt.SetLocale("en")
	tt.Frame()
	for _, want := range []string{"Notes", "Notebook", "New note"} {
		if !tt.HasText(want) {
			t.Errorf("missing English text %q after switching back; texts %q", want, tt.Texts())
		}
	}
	tt.Advance(320 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after a locale switch")
	}
}
