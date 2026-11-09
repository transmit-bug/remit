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

// At is the paragraph the note was last asked to show, zero for none: where a
// link from elsewhere in the window landed. Read mode keeps it as the paragraph
// on screen, and the writing view as the jump the editor honours.
func (s *State) At() int {
	if s.jumpTo > 0 {
		return s.jumpTo
	}
	if s.View == Read && s.readBlock >= 0 {
		return s.readBlock + 1
	}
	return 0
}
