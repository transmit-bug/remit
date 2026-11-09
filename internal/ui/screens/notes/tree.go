package notes

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// TreeNode is one note in the excerpt tree: the excerpts taken from it, and how
// much of its own it carries.
type TreeNode struct {
	Note  core.ID
	Title string
	// Cards is how many cards the note's blocks feed.
	Cards int
	// Children are the excerpts taken from this note.
	Children []TreeNode
	// Orphan says the note is itself an excerpt whose source is gone: it stands
	// as a root rather than under a note that no longer holds what it quotes.
	Orphan bool
}

// TreeView draws the excerpt tree (S10): every excerpt under the note it was
// taken from, as the tree the widget draws — folding, keyboard and screen reader
// included — with the excerpts whose source is gone in a section of their own.
func TreeView(c *ui.Context, s *State, d Deps) {
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	var tree []TreeNode
	if d.Excerpts != nil {
		tree = d.Excerpts()
	}
	var roots, orphans []TreeNode
	for _, node := range tree {
		if node.Orphan {
			orphans = append(orphans, node)
			continue
		}
		roots = append(roots, node)
	}

	ui.Column(c).Grow(1).AlignItems(ui.Center).Padding(tk.SpaceXL).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(tk.ListWidth).Grow(1).Gap(tk.SpaceL).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.End).Gap(tk.SpaceM).Children(func() {
				ui.Column(c).Grow(1).Gap(tk.SpaceXS).Children(func() {
					ui.Text(c, msgs.WriteTreeTitle()).FontSize(tk.TypeH1).Bold()
					ui.Text(c, msgs.WriteTreeHint()).
						FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
				})
				if d.OpenReadingQueue != nil {
					ui.Button(c, msgs.WriteTreeContinue()).MinHeight(tk.Control).
						OnClick(d.OpenReadingQueue)
				}
			})
			if len(roots) == 0 && len(orphans) == 0 {
				kit.Empty(c, "quote", msgs.WriteTreeEmpty(), func() {
					if d.OpenReadingQueue == nil {
						return
					}
					ui.Button(c, msgs.WriteTreeStart()).MinHeight(tk.Control).
						OnClick(d.OpenReadingQueue)
				})
				return
			}
			if len(roots) > 0 {
				ui.Tree(c, func() {
					for _, node := range roots {
						s.treeNode(c, tk, msgs, node, d)
					}
				}).Grow(1).Label(msgs.WriteTreeTitle())
			}
			if len(orphans) > 0 {
				orphanSection(c, s, tk, msgs, orphans, d)
			}
		})
	})
}

// treeNode is one note of the tree and, while it is open, the excerpts taken
// from it: its title, the cards it feeds and how many excerpts sit under it.
func (s *State) treeNode(c *ui.Context, tk design.Tokens, msgs locales.Messages, node TreeNode, d Deps) {
	open := s.open(node.Note)
	item := ui.TreeItem(c, node.Title, open, func() {
		for _, child := range node.Children {
			s.treeNode(c, tk, msgs, child, d)
		}
	}).FillWidth().Gap(tk.SpaceS).Height(tk.Control)
	if item.Clicked() && d.OpenNote != nil {
		d.OpenNote(node.Note)
	}
	item.Children(func() {
		ui.Spacer(c)
		nodeCounts(c, tk, msgs, node)
	})
}

// orphanSection is the excerpts whose source is gone: a heading that says why
// they stand alone, then the same tree under it.
func orphanSection(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, orphans []TreeNode, d Deps) {
	ui.Column(c).FillWidth().Gap(tk.SpaceS).Children(func() {
		ui.Column(c).FillWidth().Gap(tk.SpaceXS).Children(func() {
			ui.Text(c, msgs.WriteTreeOrphans()).FontSize(tk.TypeUI).Bold().
				TextColor(tk.TextMuted)
			ui.Text(c, msgs.WriteTreeOrphanHint()).FontSize(tk.TypeLabel).
				TextColor(tk.TextFaint)
		})
		ui.Tree(c, func() {
			for _, node := range orphans {
				s.treeNode(c, tk, msgs, node, d)
			}
		}).Label(msgs.WriteTreeOrphans())
	})
}

// nodeCounts is what a node is worth at a glance: the cards it feeds and the
// excerpts taken from it.
func nodeCounts(c *ui.Context, tk design.Tokens, msgs locales.Messages, node TreeNode) {
	if node.Cards > 0 {
		ui.Badge(c, msgs.WriteTreeCards(node.Cards))
	}
	if n := len(node.Children); n > 0 {
		ui.Text(c, msgs.WriteTreeMore(n)).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted).SingleLine()
	}
}

// open is a node's fold, which outlives the frame: a node starts open, so that
// the tree shows the shape of the library the first time it is looked at.
func (s *State) open(note core.ID) *bool {
	if s.treeOpen == nil {
		s.treeOpen = map[core.ID]*bool{}
	}
	if open, ok := s.treeOpen[note]; ok {
		return open
	}
	open := true
	s.treeOpen[note] = &open
	return &open
}
