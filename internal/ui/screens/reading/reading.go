// Package reading is the reading screen: one note at a time, the block the
// reader stopped at, the three ways a reading digested, and the excerpt that
// turns a passage into a note of its own.
//
// It owns the session's queue order and draws what the queue says; the app owns
// the store, and Deps is the whole of what the screen asks of it. Nothing here
// reads the store, a clock or the disk, so the screen is driven in a test with
// fakes and one window size.
package reading

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/internal/ui/maths"
	"github.com/transmit-bug/remit/locales"
)

// Note is what the screen shows of a note: its title, its blocks in file order,
// and the block the last session stopped at.
type Note struct {
	ID     core.ID
	Title  string
	Blocks []core.Block
	// Position is where the note was left: the block a session resumes at.
	Position core.ID
}

// Block returns the block with the given ID.
func (n Note) Block(id core.ID) (core.Block, bool) {
	for _, block := range n.Blocks {
		if block.ID == id {
			return block, true
		}
	}
	return core.Block{}, false
}

// Index is a block's place in the note, counted from zero, and -1 when the note
// does not hold it.
func (n Note) Index(id core.ID) int {
	for i, block := range n.Blocks {
		if block.ID == id {
			return i
		}
	}
	return -1
}

// Rating is how a reading digested: the difficulty the note took, which is what
// the schedule follows. It is three of FSRS's four answers — a reading was hard,
// just right, or shallow — and the store reads them as Again, Good and Easy.
type Rating uint8

const (
	// TooHard means the note was hard to take in: it comes back soon.
	TooHard Rating = iota
	// JustRight means it digested as it should.
	JustRight
	// TooShallow means it was easy, and need not come back so soon.
	TooShallow
)

// Schedule is the store's rating for this reading.
func (r Rating) Schedule() schedule.Rating {
	return [...]schedule.Rating{schedule.Again, schedule.Good, schedule.Easy}[r]
}

// Key is the keyboard key that records this reading.
func (r Rating) Key() string { return [...]string{"1", "2", "3"}[r] }

func (r Rating) label(msgs locales.Messages) string {
	switch r {
	case TooHard:
		return msgs.ReadingTooHard()
	case JustRight:
		return msgs.ReadingJustRight()
	default:
		return msgs.ReadingTooShallow()
	}
}

func (r Rating) colour(t *ui.Theme) ui.Color {
	return [...]ui.Color{t.Danger, t.Success, t.Accent}[r]
}

// Deps is what the screen asks the app for: the note to read, and what the user
// can do with it. Each runs on the UI thread; the app does the writing.
type Deps struct {
	// Note is the note on screen: its blocks and where it was left.
	Note func(core.ID) (Note, bool)
	// Read records a session: the difficulty and the block it stopped at.
	Read func(core.ID, Rating, core.ID)
	// Skip moves on without recording anything, so an interrupted reading is
	// not a failure the schedule counts.
	Skip func(core.ID)
	// Drop takes the note out of the reading queue: it stops coming back.
	Drop func(core.ID)
	// Excerpt takes a block out into a child note, making a card of it too when
	// withCard is set.
	Excerpt func(core.ID, core.ID, bool)
	// Source is the note a block's reference link names, if it names one.
	Source func(core.Block) (core.ID, bool)
	// Open shows a note in the notebook.
	Open func(core.ID)
	// EditNote opens the note on screen in the notebook, so a mistake found
	// while reading is fixed where it is.
	EditNote func(core.ID)
	// Review shows the other side of the dual mode.
	Review func()
	// Home leaves for the notebook.
	Home func()
}

// View draws the screen: the note, its paragraphs, and what can be done with
// them.
func View(c *ui.Context, s *State, d Deps) {
	id, hasNote := s.Current()
	var (
		note   Note
		cursor core.ID
	)
	if hasNote {
		note, hasNote = d.Note(id)
		cursor = s.Cursor(note)
	}
	ui.Column(c).Fill().Children(func() {
		statusBar(c, s, note)
		ui.Scroll(c).Grow(1).Children(func() {
			pad := float32(design.SpaceXXL)
			if w, _ := c.Size(); w < 800 {
				pad = design.SpaceXL
			}
			ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(pad).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(design.ReadingWidth).Gap(design.SpaceL).Children(func() {
					if !hasNote {
						empty(c, d)
						return
					}
					noteBody(c, s, note, cursor)
					difficulties(c, d, note, cursor)
					actions(c, s, d, note)
					reference(c, d, note, cursor)
				})
			})
		})
	})
}

// statusBar is the one line the taking-over screen keeps: which note of how
// many, where in it, and the keys that move it.
func statusBar(c *ui.Context, s *State, note Note) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	done, total := s.Progress()
	ui.Row(c).Height(design.SpaceXL+design.SpaceXS).Padding(design.SpaceXS, design.SpaceL).
		AlignItems(ui.Center).Gap(design.SpaceS).Children(func() {
		ui.Text(c, msgs.ReadingProgress(done, total)).
			FontSize(design.TypeUI).TextColor(t.TextMuted)
		if total > 0 {
			ui.Progress(c, float64(done)/float64(total)).Grow(1)
		}
		ui.Spacer(c)
		if at, total := s.At(note); total > 0 {
			ui.Text(c, msgs.ReadingWhere(at, total)).
				FontSize(design.TypeUI).TextColor(t.TextMuted)
		}
		ui.Row(c).Gap(design.SpaceXS).AlignItems(ui.Center).Children(func() {
			for _, k := range []struct{ key, label string }{
				{"J/K", msgs.ReadingMove()},
				{"E", msgs.Excerpt()},
				{"1–3", msgs.ReadingDigest()},
				{"S", msgs.Skip()},
				{"X", msgs.RemoveReading()},
				{"M", msgs.ReadingEdit()},
				{"Esc", msgs.ModeNotes()},
			} {
				kit.Keycap(c, k.key).Tooltip(k.label)
			}
		})
	})
}

// noteBody is the note as it reads: its title and every paragraph in file
// order, with the block the reader is on marked.
func noteBody(c *ui.Context, s *State, note Note, cursor core.ID) {
	kit.StyleCard(c, ui.Column(c).Key(string(note.ID)).
		Padding(design.SpaceXL).Gap(design.SpaceL)).Children(func() {
		ui.Text(c, note.Title).FontSize(design.TypeH2).Bold()
		ui.Divider(c)
		ui.Column(c).Gap(design.SpaceXS).Children(func() {
			for i, block := range note.Blocks {
				paragraph(c, s, block, block.ID == cursor, i+1)
			}
		})
	})
}

// paragraph is one block of the note. The block the reader is on carries the
// reading position, which is what a session records and an excerpt takes; the
// note opens on the block the last session stopped at, so that is where it
// starts.
//
// It is a wrapper, not a copied widget: ui.Text renders the text, and the box
// around it is what the pointer and the focus move.
func paragraph(c *ui.Context, s *State, block core.Block, current bool, number int) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	face := ui.Column(c).Key(string(block.ID)).Padding(design.SpaceS, design.SpaceM).
		Radius(design.RadiusControl).Label(msgs.ReadingParagraph(number))
	if current {
		face.Background(t.Surface).Border(design.Hairline, t.Accent).Bind(&s.cursor)
		face.Label(msgs.ReadingCurrent(number))
	}
	face.OnClick(func() { s.Choose(block.ID) }).Children(func() {
		// The paragraph's formulas are set for reading; the note keeps the
		// source.
		ui.Text(c, maths.Render(block.Text)).FontSize(design.TypeBody).TextColor(t.Text)
	})
}

// difficulties is the three ways a reading digested, each in its own colour.
func difficulties(c *ui.Context, d Deps, note Note, cursor core.ID) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	ui.Row(c).Gap(design.SpaceM).Children(func() {
		for r := TooHard; r <= TooShallow; r++ {
			face := r.colour(t)
			label := r.label(msgs)
			key := ui.ButtonBase(c).Grow(1).MinHeight(design.ControlHeight).
				Gap(design.SpaceS).Padding(design.SpaceXS, design.SpaceS).
				Background(t.Surface.Mix(face, 0.16)).Border(design.Hairline, t.Border.Mix(face, 0.5)).
				Radius(design.RadiusControl).Label(label).
				OnClick(func() { d.Read(note.ID, r, cursor) }).
				Children(func() {
					kit.Keycap(c, r.Key())
					ui.Text(c, label).FontSize(design.TypeUI).TextColor(t.Text)
				})
			if key.Hovered() {
				key.Background(t.Surface.Mix(face, 0.24)).
					Transition(ui.ElementTransition{Colors: true, Animation: &design.Hover})
			}
		}
	})
}

// actions are what the reader can do besides finishing: excerpt the block on
// screen — into a note alone, or into a note and a card — move on, take the note
// out of the queue, or go and fix the note.
func actions(c *ui.Context, s *State, d Deps, note Note) {
	msgs := locales.In(c.Locale())
	ui.Row(c).Gap(design.SpaceS).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Checkbox(c, &s.excerptCard, msgs.ExcerptWithCard()).
			Label(msgs.ExcerptWithCard())
		ui.PrimaryButton(c, msgs.Excerpt()).MinHeight(design.ControlHeight).
			OnClick(func() { s.Excerpt(c, note, d) })
		ui.Button(c, msgs.Skip()).MinHeight(design.ControlHeight).
			OnClick(func() { d.Skip(note.ID) })
		ui.Button(c, msgs.RemoveReading()).MinHeight(design.ControlHeight).
			OnClick(func() { d.Drop(note.ID) })
		ui.Button(c, msgs.ReadingEdit()).MinHeight(design.ControlHeight).
			OnClick(func() { d.EditNote(note.ID) })
	})
}

// reference is the way back to where an excerpt came from: a block that carries
// a reference link names the note it points into, and opening it is one click.
func reference(c *ui.Context, d Deps, note Note, cursor core.ID) {
	if d.Source == nil || d.Open == nil {
		return
	}
	block, ok := note.Block(cursor)
	if !ok {
		return
	}
	source, ok := d.Source(block)
	if !ok {
		return
	}
	msgs := locales.In(c.Locale())
	ui.Row(c).Gap(design.SpaceS).AlignItems(ui.Center).Children(func() {
		ui.Text(c, msgs.ReadingReference()).FontSize(design.TypeUI).
			TextColor(c.Theme().TextMuted)
		ui.Button(c, msgs.OpenSource()).MinHeight(design.ControlHeight).
			OnClick(func() { d.Open(source) })
	})
}

// empty is the state after the last note: the way to the other queue, and back
// to the notebook.
func empty(c *ui.Context, d Deps) {
	msgs := locales.In(c.Locale())
	kit.Card(c, func() {
		kit.Empty(c, "book-open", msgs.NoReading(), func() {
			ui.Row(c).Gap(design.SpaceS).Children(func() {
				ui.PrimaryButton(c, msgs.ModeReview()).MinHeight(design.ControlHeight).
					OnClick(d.Review)
				ui.Button(c, msgs.BackToNotes()).MinHeight(design.ControlHeight).
					OnClick(d.Home)
			})
		})
	})
}
