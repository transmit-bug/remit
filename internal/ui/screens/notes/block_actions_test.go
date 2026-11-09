package notes_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// Moving a block reorders the note without moving the card to the old
// position: block identity, not paragraph order, is what a card points at.
func TestBlockMenuMovesParagraphWithItsIdentity(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	_, tt := at(t, f, 1024, 720)

	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	tt.Move(body.X-44, body.Y+12)
	tt.Frame()
	if err := tt.Click(msgs.BlockActionsFor(1)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(msgs.MoveBlockDown()); err != nil {
		t.Fatalf("the block menu does not offer a move: %v", err)
	}
	tt.Frame()

	if len(f.documents) == 0 {
		t.Fatal("the moved block was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	want := []string{"block-2", "block-1", "block-3", "block-4"}
	if len(saved.Blocks) != len(want) {
		t.Fatalf("saved %d blocks, want %d", len(saved.Blocks), len(want))
	}
	for i, id := range want {
		if got := string(saved.Blocks[i].ID); got != id {
			t.Errorf("block %d has id %q, want %q", i, got, id)
		}
	}
}

// Deleting a block that feeds a card asks first, says what will be lost, and
// leaves the note alone when the user cancels.
func TestDeleteBlockWarnsBeforeCardLoss(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	s, tt := at(t, f, 1024, 720)
	text := notes.DocumentText(s.Draft)
	at := notes.ParagraphOffset(text, 2)
	s.Doc.SetSelection(at, at)
	tt.Frame()

	chooseDelete := func() {
		t.Helper()
		if err := tt.Click(msgs.BlockActionsFor(2)); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if err := tt.Click(msgs.DeleteBlock()); err != nil {
			t.Fatalf("the block menu does not offer deletion: %v", err)
		}
		tt.Frame()
	}

	chooseDelete()
	if !tt.HasText(msgs.DeleteBlockTitle()) {
		t.Fatalf("no warning before deleting a card's block; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.DeleteBlockWarning(1)) {
		t.Errorf("the warning does not count the card; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.Cancel()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(f.documents) != 0 {
		t.Fatalf("canceling still saved the note: %+v", f.documents)
	}
	if len(s.Draft.Blocks) != len(notebook[0].Blocks) {
		t.Errorf("canceling changed the draft to %d blocks", len(s.Draft.Blocks))
	}

	chooseDelete()
	if err := tt.Click(msgs.DeleteBlock()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(f.documents) == 0 {
		t.Fatal("the deletion was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	if len(saved.Blocks) != len(notebook[0].Blocks)-1 {
		t.Fatalf("saved %d blocks, want %d", len(saved.Blocks), len(notebook[0].Blocks)-1)
	}
	for _, block := range saved.Blocks {
		if block.ID == "block-2" {
			t.Error("the deleted block survived")
		}
	}
	if !tt.HasText(msgs.BlockDeleted(1)) {
		t.Errorf("the deletion does not say a card was lost; texts %q", tt.Texts())
	}
}

// Dragging a block's grip to another block's gutter moves it there, preserving
// the identities of every block it passes.
func TestBlockGripDragReorders(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	s, tt := at(t, f, 1024, 720)
	s.Doc.SetSelection(0, 0)
	tt.Frame()

	source, ok := tt.Find(msgs.BlockActionsFor(1))
	if !ok {
		t.Fatalf("the first block has no grip; texts %q", tt.Texts())
	}
	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	boxes := s.Doc.VisibleParagraphs()
	if len(boxes) < 2 {
		t.Fatalf("the editor shows %d paragraphs, want at least 2", len(boxes))
	}
	targetX, targetY := source.X+source.W/2, body.Y+boxes[1].Top+12
	fromX, fromY := source.X+source.W/2, source.Y+source.H/2
	tt.Press(fromX, fromY)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		tt.Move(fromX+(targetX-fromX)*f, fromY+(targetY-fromY)*f)
	}
	tt.Release(targetX, targetY)
	tt.Frame()

	if len(f.documents) == 0 {
		t.Fatal("the dragged block was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	if len(saved.Blocks) != len(notebook[0].Blocks) {
		t.Fatalf("saved %d blocks, want %d", len(saved.Blocks), len(notebook[0].Blocks))
	}
	if got := string(saved.Blocks[0].ID); got != "block-2" {
		t.Errorf("the moved block is first with id %q, want block-2", got)
	}
	if got := string(saved.Blocks[1].ID); got != "block-1" {
		t.Errorf("the displaced block is second with id %q, want block-1", got)
	}
}

// A slash at the start of an empty block opens a filterable menu; Enter turns
// the block into the chosen shape and leaves no command text behind.
func TestSlashMenuFiltersAndInsertsFormula(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	_, tt := at(t, f, 1024, 720)
	insertEmptyBlock(t, tt, msgs)

	tt.Type("/")
	tt.Frame()
	if !tt.HasText(msgs.SlashFormula()) {
		t.Fatalf("the slash menu did not open; texts %q", tt.Texts())
	}
	tt.Type("formula")
	tt.Frame()
	if tt.HasText(msgs.StyleTitle()) {
		t.Errorf("the query did not filter out unrelated commands; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()

	if len(f.documents) == 0 {
		t.Fatal("the inserted formula was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	found := false
	for _, block := range saved.Blocks {
		if block.Text == "$$ $$" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("saved blocks %q hold no formula", blockTexts(saved.Blocks))
	}
	for _, block := range saved.Blocks {
		if strings.Contains(block.Text, "formula") || strings.Contains(block.Text, "/") {
			t.Errorf("the slash query survived in block %q", block.Text)
		}
	}
}

// The insertion menu still opens on the smallest window, anchored inside the
// paper rather than off the edge.
func TestSlashMenuAtMinimumSize(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	_, tt := at(t, f, 720, 520)
	insertEmptyBlock(t, tt, msgs)
	tt.Type("/")
	tt.Frame()
	if !tt.HasText(msgs.SlashTable()) {
		t.Fatalf("the slash menu is missing at 720×520; texts %q", tt.Texts())
	}
}

// The image command opens the injected picker, stores the chosen file as an
// asset and leaves its relative reference in the block.
func TestSlashMenuInsertsPickedImage(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(path, []byte("not decoded here"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := newFake()
	picked := 0
	f.chooseImage = func() (string, error) {
		picked++
		return path, nil
	}
	f.putAsset = func(_ []byte, ext string) (string, error) {
		if ext != "png" {
			t.Errorf("stored the picture as %q, want png", ext)
		}
		return "assets/shot.png", nil
	}
	_, tt := at(t, f, 1024, 720)
	insertEmptyBlock(t, tt, msgs)

	tt.Type("/image")
	tt.Frame()
	if !tt.HasText(msgs.SlashImage()) {
		t.Fatalf("the slash menu has no image command; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()

	if picked != 1 {
		t.Fatalf("the image picker ran %d times, want once", picked)
	}
	if len(f.documents) == 0 {
		t.Fatal("the inserted image was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	found := false
	for _, block := range saved.Blocks {
		if strings.Contains(block.Text, "](assets/shot.png)") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("saved blocks %q hold no image reference", blockTexts(saved.Blocks))
	}
}

// A table command inserts one GFM block and shows it as a read-only table
// placeholder instead of letting the rich-text serializer flatten it.
func TestSlashMenuInsertsTable(t *testing.T) {
	msgs := locales.In("en")
	f := newFake()
	_, tt := at(t, f, 1024, 720)
	insertEmptyBlock(t, tt, msgs)

	tt.Type("/table")
	tt.Frame()
	if !tt.HasText(msgs.SlashTable()) {
		t.Fatalf("the slash menu has no table command; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()

	if len(f.documents) == 0 {
		t.Fatal("the inserted table was not saved")
	}
	saved := f.documents[len(f.documents)-1]
	found := false
	for _, block := range saved.Blocks {
		if block.Text == tableSkeletonText() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("saved blocks %q hold no table", blockTexts(saved.Blocks))
	}
}

// insertEmptyBlock puts the caret in a fresh block after the first one, where
// a slash command can replace its whole paragraph.
func insertEmptyBlock(t *testing.T, tt *ui.Tester, msgs locales.Messages) {
	t.Helper()
	body, ok := tt.Find(msgs.NoteBody())
	if !ok {
		t.Fatalf("the note body is missing; texts %q", tt.Texts())
	}
	tt.Move(body.X-44, body.Y+12)
	tt.Frame()
	if err := tt.Click(msgs.AddBlockBelow()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
}

func blockTexts(blocks []core.Block) []string {
	out := make([]string, len(blocks))
	for i, block := range blocks {
		out[i] = block.Text
	}
	return out
}

func tableSkeletonText() string {
	return "|   |   |   |\n| --- | --- | --- |\n|     |     |     |\n|     |     |     |"
}
