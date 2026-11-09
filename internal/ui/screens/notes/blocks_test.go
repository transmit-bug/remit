package notes

import (
	"testing"

	"github.com/egoist/mygo/ui/richtext"
)

// The current paragraph follows the pointer in the gutter, then the caret.
func TestActiveBlockFollowsHoverThenCaret(t *testing.T) {
	boxes := []richtext.ParagraphBox{
		{Index: 0, Start: 0},
		{Index: 1, Start: 10},
	}
	if got := activeBlock(boxes, 5, -1); got != 0 {
		t.Errorf("caret 5 marks block %d, want 0", got)
	}
	if got := activeBlock(boxes, 15, -1); got != 1 {
		t.Errorf("caret 15 marks block %d, want 1", got)
	}
	if got := activeBlock(boxes, 15, 0); got != 0 {
		t.Errorf("hover marks block %d, want the hovered 0", got)
	}
}
