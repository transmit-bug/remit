// Package reading is the reading screen: one note at a time, the block the
// reader stopped at, the three ways a reading digested, the excerpt that turns a
// passage into a note of its own, and the three decisions that end a note —
// archived, dismissed, postponed.
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
// the block the last session stopped at, and what each paragraph already
// carries.
type Note struct {
	ID     core.ID
	Title  string
	Blocks []core.Block
	// Position is where the note was left: the block a session resumes at.
	Position core.ID
	// Marks are what a paragraph carries beside its text, by block. A block with
	// no entry carries nothing.
	Marks map[core.ID]Mark
}

// Mark is what a paragraph says about itself: how many cards quote it, whether
// an excerpt was taken from it, and whether it is where the last session
// stopped. Three different kinds of fact, so the reader can tell "already kept"
// from "already asked".
type Mark struct {
	Cards     int
	Excerpted bool
	LastStop  bool
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
	// Archive puts the note on screen away for good: it leaves the queue, keeps
	// its history and its position, and comes back only if it is queued again.
	Archive func()
	// Dismiss says the note on screen is not worth reading: the other terminal
	// state, restored the same way.
	Dismiss func()
	// Postpone brings the note on screen back tomorrow.
	Postpone func()
	// QueueNote puts a note back in the reading queue and clears a postpone:
	// what undoing an archive, a dismissal or a postpone does.
	QueueNote func(core.ID)
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
	// The smallest window keeps the card, its paragraphs and the whole action
	// row on screen: the page tightens its pad rather than pushing the actions
	// under the fold.
	narrow := false
	if w, _ := c.Size(); w <= design.BreakpointSmall {
		narrow = true
	}
	ui.Column(c).Fill().Children(func() {
		sessionBar(c, s, note, hasNote, d)
		ui.Scroll(c).Grow(1).Children(func() {
			pad := float32(design.SpaceXXL)
			if narrow {
				pad = design.SpaceL
			}
			ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(pad).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(design.ReadingWidth).Gap(design.SpaceS).Children(func() {
					if !hasNote {
						empty(c, d)
						return
					}
					noteBody(c, s, note, cursor, narrow)
					actions(c, s, d, note, cursor)
					reference(c, d, note, cursor)
				})
			})
		})
	})
}

// sessionBar is the one line the taking-over screen keeps: which note of how
// many, its title, where in it the reader is, and the way out.
func sessionBar(c *ui.Context, s *State, note Note, hasNote bool, d Deps) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	done, total := s.Progress()
	ui.Row(c).MinHeight(design.Hit).Padding(design.SpaceXS, design.SpaceL).
		AlignItems(ui.Center).Gap(design.SpaceS).Children(func() {
		ui.Text(c, msgs.ReadingProgress(done, total)).NoWrap().
			FontSize(design.TypeUI).TextColor(t.TextMuted)
		if hasNote {
			// The title gives way first: the numbers beside it are what the
			// reader steers by.
			ui.Text(c, note.Title).SingleLine().Shrink(1).
				FontSize(design.TypeUI).TextColor(t.Text)
		}
		ui.Spacer(c)
		if at, of := s.At(note); hasNote && of > 0 {
			ui.Text(c, msgs.ReadingWhere(at, of)).NoWrap().
				FontSize(design.TypeUI).TextColor(t.TextMuted)
			// The note's progress fills as the bar mounts, once: the reader
			// sees how far in they are without the bar travelling on every
			// paragraph they choose.
			kit.Progress(c, kit.ProgressOptions{
				Value: float64(at) / float64(of),
				Width: design.MeterWidth,
				Label: msgs.ReadingWhere(at, of),
			})
		}
		if d.Home != nil {
			ui.Button(c, msgs.BackToNotes()).MinHeight(design.ControlHeight).OnClick(d.Home)
		}
	})
}

// noteBody is the note as it reads: its title and every paragraph in file
// order, with the block the reader is on marked.
func noteBody(c *ui.Context, s *State, note Note, cursor core.ID, narrow bool) {
	pad := float32(design.SpaceXL)
	if narrow {
		pad = design.SpaceL
	}
	kit.StyleCard(c, ui.Column(c).Key(string(note.ID)).
		Padding(pad).Gap(design.SpaceS)).Children(func() {
		ui.Text(c, note.Title).FontSize(design.TypeH2).Bold()
		ui.Divider(c)
		ui.Column(c).Gap(design.SpaceXS).Children(func() {
			for i, block := range note.Blocks {
				paragraph(c, s, block, block.ID == cursor, i+1, note.Marks[block.ID])
			}
		})
	})
}

// paragraph is one block of the note. The block the reader is on carries the
// reading position, which is what a session records and an excerpt takes; the
// note opens on the block the last session stopped at, so that is where it
// starts. The marks on its right say what the block already carries, so the
// reader does not excerpt a passage twice or make a second card by accident.
//
// It is a wrapper, not a copied widget: ui.Text renders the text, and the box
// around it is what the pointer and the focus move.
func paragraph(c *ui.Context, s *State, block core.Block, current bool, number int, mark Mark) {
	msgs := locales.In(c.Locale())
	t := c.Theme()
	face := ui.Column(c).Key(string(block.ID)).Padding(design.SpaceS, design.SpaceM).
		Radius(design.RadiusControl).Label(msgs.ReadingParagraph(number))
	if current {
		face.Background(t.Surface).Border(design.Hairline, t.Accent).Bind(&s.cursor)
		face.Label(msgs.ReadingCurrent(number))
	}
	face.OnClick(func() { s.Choose(block.ID) }).Children(func() {
		ui.Row(c).Gap(design.SpaceM).AlignItems(ui.Start).Children(func() {
			// The paragraph's formulas are set for reading; the note keeps the
			// source.
			ui.Text(c, maths.Render(block.Text)).FontSize(design.TypeBody).
				TextColor(t.Text).Grow(1)
			if mark.Cards > 0 || mark.Excerpted || mark.LastStop {
				ui.Row(c).Gap(design.SpaceXS).AlignItems(ui.Center).Shrink(0).
					Children(func() {
						if mark.Cards > 0 {
							ui.Badge(c, msgs.ReadCardsMark(mark.Cards))
						}
						if mark.Excerpted {
							ui.Badge(c, msgs.ReadExcerptedMark())
						}
						if mark.LastStop {
							ui.Badge(c, msgs.ReadLastStopMark()).
								Background(t.Accent.Alpha(0.12)).TextColor(t.Accent)
						}
					})
			}
		})
	})
}

// actions are what the reader can do with the note, in two groups: how the
// reading digested, kept as three equal answers across the card, and what to do
// with the note itself — excerpt it, move on, end it, or go and fix it. A narrow
// window wraps the actions rather than shortening them, and a terminal action
// never lands beside a difficulty.
func actions(c *ui.Context, s *State, d Deps, note Note, cursor core.ID) {
	msgs := locales.In(c.Locale())
	ui.Column(c).Gap(design.SpaceS).Children(func() {
		ui.Row(c).Gap(design.SpaceS).AlignItems(ui.Center).Children(func() {
			for r := TooHard; r <= TooShallow; r++ {
				difficulty(c, r, func() { d.Read(note.ID, r, cursor) })
			}
		})
		ui.Row(c).Gap(design.SpaceS).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Checkbox(c, &s.excerptCard, msgs.ExcerptWithCard()).
				Label(msgs.ExcerptWithCard())
			ui.PrimaryButton(c, msgs.Excerpt()).MinHeight(design.ControlHeight).
				OnClick(func() { s.Excerpt(c, note, d) })
			ui.Button(c, msgs.Skip()).MinHeight(design.ControlHeight).
				OnClick(func() { d.Skip(note.ID) })
			ui.Button(c, msgs.ReadArchive()).MinHeight(design.ControlHeight).
				OnClick(func() { s.End(c, d, note, ArchiveDecision) })
			ui.Button(c, msgs.ReadDismiss()).MinHeight(design.ControlHeight).
				OnClick(func() { s.End(c, d, note, DismissDecision) })
			ui.Button(c, msgs.ReadPostpone()).MinHeight(design.ControlHeight).
				OnClick(func() { s.End(c, d, note, PostponeDecision) })
			ui.Button(c, msgs.RemoveReading()).MinHeight(design.ControlHeight).
				OnClick(func() { d.Drop(note.ID) })
			ui.Button(c, msgs.ReadingEdit()).MinHeight(design.ControlHeight).
				OnClick(func() { d.EditNote(note.ID) })
		})
		ui.Row(c).Gap(design.SpaceXS).AlignItems(ui.Center).Children(func() {
			kit.Keycap(c, "J/K")
			ui.Text(c, msgs.ReadingMove()).FontSize(design.TypeCaption).
				TextColor(c.Theme().TextMuted)
		})
	})
}

// difficulty is one of the three ways a reading digested, in its own colour and
// with the key that records it.
func difficulty(c *ui.Context, r Rating, choose func()) {
	t := c.Theme()
	msgs := locales.In(c.Locale())
	face := r.colour(t)
	label := r.label(msgs)
	key := kit.HoverFace(c, ui.ButtonBase(c).Grow(1).MinHeight(design.ControlHeight).
		Gap(design.SpaceS).Padding(design.SpaceXS, design.SpaceS).
		Background(t.Surface.Mix(face, 0.16)).Border(design.Hairline, t.Border.Mix(face, 0.5)).
		Radius(design.RadiusControl).Label(label).
		OnClick(choose).
		Children(func() {
			kit.Keycap(c, r.Key())
			ui.Text(c, label).FontSize(design.TypeUI).TextColor(t.Text)
		}))
	if key.Hovered() {
		key.Background(t.Surface.Mix(face, 0.24))
	}
	// The press shrinks the key on the face, without changing the room it takes.
	kit.Press(key, key.Pressed())
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
