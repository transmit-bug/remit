package notes

// ShowAt shows the note read-only, placed at the paragraph named, counted from
// one: where a link from elsewhere in the window lands — a board node, a card's
// source. Read mode draws no per-block anchor yet, so the place is kept in the
// same jump the editor honours, and a reader who switches to writing lands on the
// paragraph the link named.
func (s *State) ShowAt(paragraph int) {
	s.View = Read
	s.jumpTo = paragraph
}

// At is the paragraph the note was last asked to show, zero for none.
func (s *State) At() int { return s.jumpTo }
