package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/settings"
)

// notesMode opens the editor the way the user does, from the review queue the
// window opens on.
func notesMode(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(200 * time.Millisecond) // the review card's exit
}

// openNote chooses a note from the sidebar, which is a title no field holds
// while another note is up.
func openNote(t *testing.T, tt *ui.Tester, title string) {
	t.Helper()
	if err := tt.Click(title); err != nil {
		t.Fatalf("choose note %q: %v", title, err)
	}
}

// chooseFromMenu opens one block's menu and picks an item from it.
func chooseFromMenu(t *testing.T, tt *ui.Tester, block int, item string) {
	t.Helper()
	if err := tt.Click(blockActions(block)); err != nil {
		t.Fatalf("open block %d's menu: %v", block, err)
	}
	if !tt.HasText(item) {
		t.Fatalf("block %d's menu has no %q; texts %q", block, item, tt.Texts())
	}
	if err := tt.Click(item); err != nil {
		t.Fatalf("choose %q: %v", item, err)
	}
}

// blockActions is the accessible label of one block's menu button.
func blockActions(n int) string { return "Block " + strconv.Itoa(n) + " actions" }

// The title is a field: typing in it renames the note and reaches the sidebar,
// while the note file keeps its path.
func TestEditNoteTitle(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesMode(t, a, tt)

	if err := tt.Click("Note title"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("A better habit")

	note, ok := a.Library().Note(seed.habit.ID)
	if !ok {
		t.Fatal("the note is gone")
	}
	if note.Title != "A better habit" {
		t.Fatalf("the store holds the title %q; wants %q", note.Title, "A better habit")
	}
	if !tt.HasText("A better habit") {
		t.Fatalf("the sidebar does not show the new title; texts %q", tt.Texts())
	}
	if tt.HasText("A daily habit") {
		t.Errorf("the old title is still on screen; texts %q", tt.Texts())
	}
	// A renamed note keeps its file: the slug is minted once.
	if _, err := os.Stat(filepath.Join(seed.dir, "notes", "a-daily-habit-"+string(seed.habit.ID)+".md")); err != nil {
		t.Errorf("the note file moved with its title: %v", err)
	}
	tt.Advance(200 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// A block can be added after another one, from its menu, and Cmd+Enter in a
// field does the same without losing what was typed.
func TestAddBlock(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesMode(t, a, tt)

	chooseFromMenu(t, tt, 1, "Add block below")
	if !tt.Focused("Block 2 · A daily habit") {
		t.Fatalf("the new block did not take the caret; texts %q", tt.Texts())
	}
	if !tt.HasText("3 blocks · 0 cards") {
		t.Fatalf("the note does not count the new block; texts %q", tt.Texts())
	}

	// Type into the new block, then ask for another one with Cmd+Enter: the
	// text typed in this very pass has to survive the save.
	tt.Type("Clarity first :: one idea per card")
	tt.Key(ui.Cmd, ui.KeyEnter)
	if !tt.Focused("Block 3 · A daily habit") {
		t.Fatalf("Cmd+Enter did not add a block; texts %q", tt.Texts())
	}

	note, ok := a.Library().Note(seed.habit.ID)
	if !ok {
		t.Fatal("the note is gone")
	}
	if len(note.Blocks) != 4 {
		t.Fatalf("the note holds %d blocks, want 4", len(note.Blocks))
	}
	if got := note.Blocks[1].Text; got != "Clarity first :: one idea per card" {
		t.Errorf("block 2 holds %q", got)
	}
	// The library has one block more than before, each with an ID of its own.
	ids := map[core.ID]bool{}
	for _, block := range note.Blocks {
		if block.ID == "" || ids[block.ID] {
			t.Fatalf("block IDs are missing or shared: %+v", note.Blocks)
		}
		ids[block.ID] = true
	}
	tt.Advance(200 * time.Millisecond) // the screen's entrance
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// Splitting at the start moves the text, and the live references with it: the
// card keeps reading what it always read, from the block that now holds it.
func TestSplitBlockMovesReferences(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")

	const field = "Block 1 · Memory and recall"
	if err := tt.Click(field); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyHome)
	chooseFromMenu(t, tt, 1, "Split at cursor")

	note, ok := a.Library().Note(seed.memory.ID)
	if !ok {
		t.Fatal("the note is gone")
	}
	if len(note.Blocks) != 4 {
		t.Fatalf("the note holds %d blocks, want 4", len(note.Blocks))
	}
	if note.Blocks[0].Text != "" {
		t.Errorf("block 1 kept %q, want an empty block", note.Blocks[0].Text)
	}
	want := "Spacing beats cramming :: reviewing just before you forget strengthens recall."
	if note.Blocks[1].Text != want {
		t.Errorf("block 2 holds %q, want %q", note.Blocks[1].Text, want)
	}

	card, ok := a.Library().Card(seed.spacing.ID)
	if !ok {
		t.Fatal("the card is gone")
	}
	if card.Ref.Block != note.Blocks[1].ID {
		t.Errorf("the card points at %q, want the block that holds its text %q", card.Ref.Block, note.Blocks[1].ID)
	}
	if _, _, state := card.Sides(a.Library()); state != core.LinkLive {
		t.Errorf("the re-pointed card is %s", state)
	}
}

// A split in the middle leaves the original block with the text before the
// caret, and the card stays on it: ADR 0002's rule for the side that kept
// what the card read.
func TestSplitBlockKeepsReferences(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")

	const field = "Block 2 · Memory and recall"
	if err := tt.Click(field); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyHome)
	for range 3 {
		tt.Key(0, ui.KeyRight)
	}
	chooseFromMenu(t, tt, 2, "Split at cursor")

	note, ok := a.Library().Note(seed.memory.ID)
	if !ok {
		t.Fatal("the note is gone")
	}
	if len(note.Blocks) != 4 {
		t.Fatalf("the note holds %d blocks, want 4", len(note.Blocks))
	}
	// The store trims a block, so the space the split fell on is not in
	// either half; the words on each side are.
	const whole = "The testing effect :: recalling an answer builds a stronger memory than rereading it."
	if !strings.HasPrefix(whole, note.Blocks[1].Text+" ") || !strings.HasSuffix(whole, note.Blocks[2].Text) {
		t.Errorf("the halves %q | %q do not divide %q", note.Blocks[1].Text, note.Blocks[2].Text, whole)
	}
	if note.Blocks[1].ID != seed.testing.Ref.Block {
		t.Errorf("the split gave the side before the caret a new ID")
	}
	card, _ := a.Library().Card(seed.testing.ID)
	if card.Ref.Block != note.Blocks[1].ID {
		t.Errorf("the middle split moved the card to %q", card.Ref.Block)
	}
}

// A block moves up and down in its note, and the card that follows it keeps
// following the text.
func TestMoveBlock(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesMode(t, a, tt)
	openNote(t, tt, "Writing cards")

	chooseFromMenu(t, tt, 1, "Move down")
	note, _ := a.Library().Note(seed.writing.ID)
	if !strings.HasPrefix(note.Blocks[0].Text, "A block divides at") {
		t.Fatalf("block 1 is %q after moving it down", note.Blocks[0].Text)
	}
	if !strings.HasPrefix(note.Blocks[1].Text, "A good card asks") {
		t.Fatalf("block 2 is %q after the move", note.Blocks[1].Text)
	}
	if !tt.HasText("Block 1 · Writing cards") {
		t.Fatalf("the editor lost its blocks; texts %q", tt.Texts())
	}

	chooseFromMenu(t, tt, 2, "Move up")
	note, _ = a.Library().Note(seed.writing.ID)
	if !strings.HasPrefix(note.Blocks[0].Text, "A good card asks") {
		t.Fatalf("block 1 is %q after moving it back up", note.Blocks[0].Text)
	}
	// A moved block keeps its ID, so its card still follows it.
	card, _ := a.Library().Card(seed.frozen.ID)
	if card.Ref.Block != note.Blocks[0].ID {
		t.Errorf("the card follows %q, the block is %q", card.Ref.Block, note.Blocks[0].ID)
	}
	if _, _, state := card.Sides(a.Library()); state != core.LinkFrozen {
		t.Errorf("a frozen card became %s when its block moved", state)
	}
}

// Deleting a block loses the card that read it, and nothing else: the card
// file, its status and its reviews are still there, ready to be re-linked.
func TestDeleteBlockLosesTheCard(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesMode(t, a, tt)
	openNote(t, tt, "Memory and recall")

	before, ok := a.Library().CardState(seed.spacing.ID)
	if !ok || before.Reviews == 0 {
		t.Fatalf("the seed card has no reviews: %+v", before)
	}
	events := len(a.Library().Events())

	chooseFromMenu(t, tt, 1, "Delete block")
	if !tt.HasText("1 card lost its source") {
		t.Fatalf("deleting the block did not say what it cost; texts %q", tt.Texts())
	}

	note, _ := a.Library().Note(seed.memory.ID)
	if len(note.Blocks) != 2 {
		t.Fatalf("the note holds %d blocks, want 2", len(note.Blocks))
	}
	for _, block := range note.Blocks {
		if block.ID == seed.spacing.Ref.Block {
			t.Fatal("the deleted block is still in the note")
		}
	}

	card, ok := a.Library().Card(seed.spacing.ID)
	if !ok {
		t.Fatal("deleting the block deleted the card")
	}
	if _, _, state := card.Sides(a.Library()); state != core.LinkLost {
		t.Errorf("the card is %s, want lost", state)
	}
	after, ok := a.Library().CardState(seed.spacing.ID)
	if !ok || after.Reviews != before.Reviews || after.Due != before.Due {
		t.Errorf("the card's history changed: %+v, then %+v", before, after)
	}
	if got := len(a.Library().Events()); got != events {
		t.Errorf("the review log grew from %d to %d events", events, got)
	}
	if _, err := os.Stat(filepath.Join(seed.dir, "cards", string(seed.spacing.ID)+".json")); err != nil {
		t.Errorf("the card file is gone: %v", err)
	}

	// The lost card is the Lost filter's, and the window offers the repair.
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(200 * time.Millisecond) // the card's exit
	if err := tt.Click("Lost"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(lostBody) || !tt.HasText("Re-link") {
		t.Fatalf("the lost card has no repair in browse; texts %q", tt.Texts())
	}
}

// Every block action is in the command palette too, so the editor needs no
// pointer: add, split, move and delete run against the block the user was in.
func TestEditorPaletteActions(t *testing.T) {
	a, seed := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	notesMode(t, a, tt)
	openNote(t, tt, "Writing cards")

	run := func(query string) {
		t.Helper()
		tt.Key(ui.Cmd, ui.KeyK)
		tt.Type(query)
		tt.Key(0, ui.KeyEnter)
		if tt.HasText("Search actions") {
			t.Fatalf("the palette stayed open after %q", query)
		}
	}

	run("add block")
	note, _ := a.Library().Note(seed.writing.ID)
	if len(note.Blocks) != 4 {
		t.Fatalf("the palette's add made %d blocks, want 4", len(note.Blocks))
	}

	run("move block down")
	note, _ = a.Library().Note(seed.writing.ID)
	if !strings.HasPrefix(note.Blocks[1].Text, "A block divides at") {
		t.Errorf("the palette's move left block 2 as %q", note.Blocks[1].Text)
	}

	run("delete block")
	note, _ = a.Library().Note(seed.writing.ID)
	if len(note.Blocks) != 3 {
		t.Fatalf("the palette's delete left %d blocks, want 3", len(note.Blocks))
	}

	run("edit note title")
	if !tt.Focused("Note title") {
		t.Fatalf("the palette's edit-title did not reach the field; texts %q", tt.Texts())
	}
}

// The settings the app does not own survive remembering a library: only the
// path changes.
func TestSettingsKeepPreferences(t *testing.T) {
	data := t.TempDir()
	if err := settings.Save(data, settings.Settings{UpdateCheck: true, AgentServer: true}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "Remit")
	a := NewApp(Options{LibraryDir: dir, DataDir: data})
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	if err := tt.Click("Create library"); err != nil {
		t.Fatal(err)
	}
	s, err := settings.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.LibraryDir != dir {
		t.Errorf("the settings name %q, want %q", s.LibraryDir, dir)
	}
	if !s.UpdateCheck || !s.AgentServer {
		t.Errorf("remembering the library dropped a preference: %+v", s)
	}
}
