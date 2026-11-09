package store_test

import (
	"os"
	"slices"
	"testing"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

// TestExcerptTreeNestsChildren is the tree the excerpt page draws: a note that
// was excerpted is a root, the excerpt taken from it is its child, and an excerpt
// of an excerpt is a child of that — one level per act of cutting a passage out.
func TestExcerptTreeNestsChildren(t *testing.T) {
	lib, _, _ := newLibrary(t)
	source := saveNote(t, lib, "Memory", "spacing :: beats cramming", "the forgetting curve :: decays")
	child, err := lib.Excerpt(source.ID, source.Blocks[0].ID, "spacing :: beats cramming", "Excerpt of Memory")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}
	grandchild, err := lib.Excerpt(child.ID, child.Blocks[0].ID, "spacing", "Excerpt of an excerpt")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}
	// A second excerpt from the same source is a second child of it.
	sibling, err := lib.Excerpt(source.ID, source.Blocks[1].ID, "the forgetting curve", "A second passage")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}
	for _, block := range source.Blocks {
		if _, err := firstCard(lib, block.ID); err != nil {
			t.Fatalf("card: %v", err)
		}
	}

	tree := lib.ExcerptTree()
	if len(tree) != 1 {
		t.Fatalf("roots = %+v, want the one note the others were taken from", tree)
	}
	root := tree[0]
	if root.Note != source.ID || root.Title != "Memory" {
		t.Errorf("root = %q (%s), want Memory", root.Title, root.Note)
	}
	if root.Cards != 2 {
		t.Errorf("root carries %d cards, want 2", root.Cards)
	}
	// Children are ordered by title, so two reads and a rebuild agree.
	if got, want := childTitles(root.Children), []string{"A second passage", "Excerpt of Memory"}; !slices.Equal(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	if root.Children[0].Note != sibling.ID {
		t.Errorf("first child = %s, want the second passage %s", root.Children[0].Note, sibling.ID)
	}
	inner := root.Children[1]
	if inner.Note != child.ID {
		t.Fatalf("second child = %s, want %s", inner.Note, child.ID)
	}
	if got := childTitles(inner.Children); !slices.Equal(got, []string{grandchild.Title}) {
		t.Errorf("grandchildren = %v, want %q", got, grandchild.Title)
	}
	if inner.Cards != 0 {
		t.Errorf("the excerpt carries %d cards, want none", inner.Cards)
	}

	t.Run("a rebuilt index answers the same", func(t *testing.T) {
		if err := lib.Rebuild(); err != nil {
			t.Fatalf("Rebuild: %v", err)
		}
		if after := lib.ExcerptTree(); !slices.EqualFunc(tree, after, sameExcerptNode) {
			t.Errorf("rebuilt tree = %+v, want %+v", after, tree)
		}
	})
}

// TestExcerptTreeListsOrphans is the tree's other half: an excerpt whose source
// is gone is still a note with its own history, so it stands as a root of its own
// rather than vanishing from the page.
func TestExcerptTreeListsOrphans(t *testing.T) {
	lib, dir, _ := newLibrary(t)
	source := saveNote(t, lib, "Doomed", "a passage :: worth keeping")
	orphan, err := lib.Excerpt(source.ID, source.Blocks[0].ID, "a passage", "Orphan")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}
	// The source note is deleted by hand, which is what leaves an excerpt
	// pointing into a block nothing holds any more.
	if err := os.Remove(notePath(dir, source)); err != nil {
		t.Fatalf("remove the source note: %v", err)
	}
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	tree := lib.ExcerptTree()
	if len(tree) != 1 || tree[0].Note != orphan.ID {
		t.Fatalf("tree = %+v, want the orphan as a root", tree)
	}
	if tree[0].Title != orphan.Title {
		t.Errorf("root = %q, want %q", tree[0].Title, orphan.Title)
	}

	t.Run("an excerpt of the orphan nests under it", func(t *testing.T) {
		inner, err := lib.Excerpt(orphan.ID, orphan.Blocks[0].ID, "a passage", "Inner")
		if err != nil {
			t.Fatalf("Excerpt: %v", err)
		}
		tree := lib.ExcerptTree()
		if len(tree) != 1 || tree[0].Note != orphan.ID {
			t.Fatalf("tree = %+v, want the orphan as the one root", tree)
		}
		if got := childTitles(tree[0].Children); !slices.Equal(got, []string{inner.Title}) {
			t.Errorf("children = %v, want %q", got, inner.Title)
		}
	})
}

// TestExcerptTreeBreaksCycles is the hand-edited case: two notes that excerpt
// each other by link are still two nodes and never a loop, and every note is in
// the tree exactly once.
func TestExcerptTreeBreaksCycles(t *testing.T) {
	lib, _, _ := newLibrary(t)
	first := saveNote(t, lib, "First", "one :: two")
	second := saveNote(t, lib, "Second", "three :: four")
	// Hand-written anchored links, the shape an excerpt writes.
	if _, err := lib.SaveNote(core.Note{ID: first.ID, Title: first.Title, Blocks: []core.Block{
		first.Blocks[0],
		{Text: "back to [[Second#^" + string(second.Blocks[0].ID) + "]]"},
	}}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if _, err := lib.SaveNote(core.Note{ID: second.ID, Title: second.Title, Blocks: []core.Block{
		second.Blocks[0],
		{Text: "back to [[First#^" + string(first.Blocks[0].ID) + "]]"},
	}}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	tree := lib.ExcerptTree()
	seen := map[core.ID]int{}
	var walk func(nodes []store.ExcerptNode, depth int)
	walk = func(nodes []store.ExcerptNode, depth int) {
		if depth > 4 {
			t.Fatalf("the tree does not end: %+v", tree)
		}
		for _, node := range nodes {
			seen[node.Note]++
			walk(node.Children, depth+1)
		}
	}
	walk(tree, 0)
	if seen[first.ID] != 1 || seen[second.ID] != 1 {
		t.Errorf("each note appears %v, want once each", seen)
	}
}

// TestExcerptTreeIgnoresMentions is the difference the tree is built on: a note
// named by title alone is a mention, not an excerpt of one of its blocks.
func TestExcerptTreeIgnoresMentions(t *testing.T) {
	lib, _, _ := newLibrary(t)
	saveNote(t, lib, "Memory", "spacing beats cramming")
	saveNote(t, lib, "Mentioning", "see [[Memory]] for more")
	if tree := lib.ExcerptTree(); len(tree) != 0 {
		t.Errorf("tree = %+v, want none: a mention is not an excerpt", tree)
	}
}

// childTitles is what a node's children are called, in order.
func childTitles(nodes []store.ExcerptNode) []string {
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.Title)
	}
	return out
}

func sameExcerptNode(a, b store.ExcerptNode) bool {
	if a.Note != b.Note || a.Title != b.Title || a.Cards != b.Cards || len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !sameExcerptNode(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}
