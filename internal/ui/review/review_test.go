package review_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ui/review"
	"github.com/transmit-bug/remit/locales"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"default", 1024, 720},
	{"minimum", 720, 520},
}

// fake is the app the screen talks to: a queue of cards and the calls it made,
// with no library, no clock and no disk behind it.
type fake struct {
	cards  map[core.ID]review.Card
	state  *review.State
	graded []struct {
		id core.ID
		r  review.Rating
	}
	undone    int
	suspended []core.ID
	relinked  []core.ID
	edited    []core.ID
	home      int
	preview   func() ([4]review.Interval, bool)
	summary   func() (review.Summary, bool)
}

func (f *fake) deps() review.Deps {
	return review.Deps{
		Card: func(id core.ID) (review.Card, bool) {
			card, ok := f.cards[id]
			return card, ok
		},
		Grade: func(id core.ID, r review.Rating) {
			f.graded = append(f.graded, struct {
				id core.ID
				r  review.Rating
			}{id, r})
		},
		Undo:     func() { f.undone++ },
		Suspend:  func(id core.ID) { f.suspended = append(f.suspended, id) },
		Relink:   func(id core.ID) { f.relinked = append(f.relinked, id) },
		EditNote: func(id core.ID) { f.edited = append(f.edited, id) },
		Home:     func() { f.home++ },
		Preview:  f.preview,
		Summary:  f.summary,
	}
}

// open builds the screen with a queue of cards, in the order they were given.
func open(t *testing.T, order ...core.ID) (*fake, *review.State, func(*ui.Context)) {
	t.Helper()
	f := &fake{cards: map[core.ID]review.Card{}, state: &review.State{}}
	for _, id := range order {
		f.cards[id] = review.Card{
			ID:     id,
			Prompt: "Prompt " + string(id),
			Answer: "Answer " + string(id),
		}
	}
	f.state.Sync(order)
	state, deps := f.state, f.deps()
	return f, state, func(c *ui.Context) { review.View(c, state, deps) }
}

// at draws the screen once at a size and lets it settle.
func at(t *testing.T, view func(*ui.Context), w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(view, w, h)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// One card takes the window: its prompt, the way to reveal the answer, and a
// status bar that says how many are left — at both the sizes the window opens
// at, and with no navigation of its own.
func TestCardOnScreen(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			_, state, view := open(t, "c1", "c2")
			tt := at(t, view, size.w, size.h)
			for _, want := range []string{"Prompt c1", msgs.ShowAnswer(), msgs.Remaining(2)} {
				if !tt.HasText(want) {
					t.Fatalf("missing %q; texts %q", want, tt.Texts())
				}
			}
			if tt.HasText("Answer c1") {
				t.Error("the answer is up before it was asked for")
			}
			if state.Remaining() != 2 {
				t.Errorf("remaining is %d, want 2", state.Remaining())
			}
		})
	}
}

// Revealing shows the answer and the four grades; the grades hand the rating
// back to the app.
func TestRevealAndGrade(t *testing.T) {
	msgs := locales.In("en")
	f, state, view := open(t, "c1")
	tt := at(t, view, 1024, 720)

	if err := tt.Click(msgs.ShowAnswer()); err != nil {
		t.Fatal(err)
	}
	if !state.Answered() {
		t.Fatal("revealing did not mark the answer as shown")
	}
	if !tt.HasText("Answer c1") {
		t.Fatalf("the answer is not on screen; texts %q", tt.Texts())
	}
	for _, want := range []string{msgs.Again(), msgs.Hard(), msgs.Good(), msgs.Easy()} {
		if !tt.HasText(want) {
			t.Errorf("the grade %q is missing; texts %q", want, tt.Texts())
		}
	}

	if err := tt.Click(msgs.Easy()); err != nil {
		t.Fatal(err)
	}
	if len(f.graded) != 1 {
		t.Fatalf("the grade reached the app %d times, want 1", len(f.graded))
	}
	if got := f.graded[0]; got.id != "c1" || got.r != review.Easy {
		t.Errorf("the app was told %q/%v, want c1/Easy", got.id, got.r)
	}
}

// The footer's actions reach the app, and undo is offered only once something
// has been graded.
func TestFooterActions(t *testing.T) {
	msgs := locales.In("en")
	f, state, view := open(t, "c1")
	tt := at(t, view, 1024, 720)

	if err := tt.Click(msgs.EditNote()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(msgs.Suspend()); err != nil {
		t.Fatal(err)
	}
	if len(f.edited) != 1 || f.edited[0] != "c1" {
		t.Errorf("edit note reached the app as %q, want c1", f.edited)
	}
	if len(f.suspended) != 1 || f.suspended[0] != "c1" {
		t.Errorf("suspend reached the app as %q, want c1", f.suspended)
	}

	state.Sync([]core.ID{"c1"})
	state.Reveal()
	state.Advance(true)
	if _, ok := state.Last(); !ok {
		t.Fatal("grading nothing left nothing to undo")
	}
	state.Restore("c1")
}

// A lost card says so and offers the two ways out, and no grades.
func TestLostCard(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{cards: map[core.ID]review.Card{
		"c1": {ID: "c1", Prompt: "Prompt c1", State: core.LinkLost},
	}, state: &review.State{}}
	f.state.Sync([]core.ID{"c1"})
	state, deps := f.state, f.deps()
	tt := at(t, func(c *ui.Context) { review.View(c, state, deps) }, 1024, 720)

	for _, want := range []string{msgs.Lost(), msgs.LostBody(), msgs.Relink(), msgs.SuspendCard()} {
		if !tt.HasText(want) {
			t.Fatalf("the lost card is missing %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText(msgs.ShowAnswer()) {
		t.Error("a lost card offers to reveal an answer it does not have")
	}

	if err := tt.Click(msgs.Relink()); err != nil {
		t.Fatal(err)
	}
	if len(f.relinked) != 1 || f.relinked[0] != "c1" {
		t.Errorf("re-link reached the app as %q, want c1", f.relinked)
	}
}

// A frozen card says when its snapshot was taken, and still reviews.
func TestFrozenCard(t *testing.T) {
	msgs := locales.In("en")
	at0 := time.Date(2026, time.March, 4, 0, 0, 0, 0, time.UTC)
	f := &fake{cards: map[core.ID]review.Card{
		"c1": {ID: "c1", Prompt: "Prompt c1", Answer: "Answer c1", State: core.LinkFrozen, Frozen: at0},
	}, state: &review.State{}}
	f.state.Sync([]core.ID{"c1"})
	state, deps := f.state, f.deps()
	tt := at(t, func(c *ui.Context) { review.View(c, state, deps) }, 1024, 720)

	if !tt.HasText(msgs.Frozen()) || !tt.HasText(msgs.SnapshotTaken("4 March 2026")) {
		t.Fatalf("the frozen card does not say when it stopped following; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.ShowAnswer()) {
		t.Error("a frozen card cannot be reviewed")
	}
}

// The queue's end: one sentence and one way back, and it works.
func TestEmptyQueue(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			f := &fake{cards: map[core.ID]review.Card{}, state: &review.State{}}
			state, deps := f.state, f.deps()
			tt := at(t, func(c *ui.Context) { review.View(c, state, deps) }, size.w, size.h)
			if !tt.HasText(msgs.NoCards()) || !tt.HasText(msgs.BackToNotes()) {
				t.Fatalf("the empty queue is missing its words; texts %q", tt.Texts())
			}
			if err := tt.Click(msgs.BackToNotes()); err != nil {
				t.Fatal(err)
			}
			if f.home != 1 {
				t.Errorf("leaving reached the app %d times, want 1", f.home)
			}
		})
	}
}

// The queue keeps its session order when the store's due list changes under it,
// and a card that became due joins the end.
func TestQueueOrderSurvivesSync(t *testing.T) {
	state := &review.State{}
	state.Sync([]core.ID{"a", "b", "c"})
	state.Sync([]core.ID{"c", "a", "b", "d"})
	if got := state.Remaining(); got != 4 {
		t.Fatalf("remaining is %d, want 4", got)
	}
	for _, want := range []core.ID{"a", "b", "c", "d"} {
		id, ok := state.Current()
		if !ok || id != want {
			t.Fatalf("the head is %q, want %q", id, want)
		}
		state.Advance(false)
	}
}

// withPreview gives the screen the four intervals the app would show, as the
// schedule would report them.
func withPreview(f *fake, waits [4]time.Duration) {
	f.preview = func() ([4]review.Interval, bool) {
		var out [4]review.Interval
		for i, w := range waits {
			out[i] = review.Interval{Rating: review.Rating(i), Wait: w}
		}
		return out, true
	}
}

// A preview puts each answer's interval on its key: minutes, hours, days.
func TestGradeKeysShowIntervals(t *testing.T) {
	msgs := locales.In("en")
	f, state, _ := open(t, "c1")
	withPreview(f, [4]time.Duration{
		10 * time.Minute, 6 * time.Hour, 3 * 24 * time.Hour, 120 * 24 * time.Hour,
	})
	state.Reveal()
	deps := f.deps()
	tt := at(t, func(c *ui.Context) { review.View(c, state, deps) }, 1024, 720)

	for _, want := range []string{
		msgs.IntervalMinutes(10), msgs.IntervalHours(6), msgs.IntervalDays(3), msgs.IntervalDays(120),
	} {
		if !tt.HasText(want) {
			t.Errorf("the grades do not show the interval %q; texts %q", want, tt.Texts())
		}
	}
}

// Without a preview the keys fall back to their own labels, and no placeholder
// is drawn in the interval's place.
func TestGradeKeysWithoutPreview(t *testing.T) {
	msgs := locales.In("en")
	f, state, _ := open(t, "c1")
	state.Reveal()
	deps := f.deps()
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			tt := at(t, func(c *ui.Context) { review.View(c, state, deps) }, size.w, size.h)
			for _, want := range []string{msgs.Again(), msgs.Hard(), msgs.Good(), msgs.Easy()} {
				if !tt.HasText(want) {
					t.Fatalf("the grade %q is missing; texts %q", want, tt.Texts())
				}
			}
			for _, text := range tt.Texts() {
				if len(text) > 1 && (text[len(text)-1] == 'm' || text[len(text)-1] == 'h' || text[len(text)-1] == 'd') {
					digits := true
					for _, r := range text[:len(text)-1] {
						if r < '0' || r > '9' {
							digits = false
						}
					}
					if digits {
						t.Errorf("a placeholder interval %q is drawn without a preview", text)
					}
				}
			}
		})
	}
}

// At the smallest window the interval sits inside the window and beside its
// label rather than over it.
func TestIntervalsFitTheSmallestWindow(t *testing.T) {
	msgs := locales.In("en")
	f, state, _ := open(t, "c1")
	withPreview(f, [4]time.Duration{10 * time.Minute, 6 * time.Hour, 3 * 24 * time.Hour, 120 * 24 * time.Hour})
	state.Reveal()
	deps := f.deps()
	tt := at(t, func(c *ui.Context) { review.View(c, state, deps) }, 720, 520)

	wait := msgs.IntervalDays(120)
	box, ok := tt.Find(wait)
	if !ok {
		t.Fatalf("the longest interval %q is not drawn; texts %q", wait, tt.Texts())
	}
	if box.X < 0 || box.X+box.W > 720 || box.Y < 0 || box.Y+box.H > 520 {
		t.Errorf("the interval box %+v is outside the 720x520 window", box)
	}
	label, ok := tt.Find(msgs.Easy())
	if !ok {
		t.Fatalf("the Easy label is missing; texts %q", tt.Texts())
	}
	// The label is the key itself: the interval sits inside it, past the name.
	if box.X < label.X || box.X+box.W > label.X+label.W {
		t.Errorf("the interval %+v is not inside its key %+v", box, label)
	}
}

// A finished queue reports what the app says it is worth reporting: the day's
// answers, tomorrow's work and how many were right.
func TestSummaryWords(t *testing.T) {
	msgs := locales.In("en")
	f := &fake{cards: map[core.ID]review.Card{}, state: &review.State{}}
	f.summary = func() (review.Summary, bool) {
		return review.Summary{Reviewed: 24, DueTomorrow: 6, Accuracy: 0.87}, true
	}
	state, deps := f.state, f.deps()
	tt := at(t, func(c *ui.Context) { review.View(c, state, deps) }, 1024, 720)

	for _, want := range []string{
		msgs.SummaryTitle(),
		msgs.SummaryReviewed(24),
		msgs.SummaryDueTomorrow(6),
		msgs.SummaryAccuracy(87),
	} {
		if !tt.HasText(want) {
			t.Errorf("the summary is missing %q; texts %q", want, tt.Texts())
		}
	}

	// Leaving is never held up by the summary.
	if err := tt.Click(msgs.BackToNotes()); err != nil {
		t.Fatal(err)
	}
	if f.home != 1 {
		t.Errorf("leaving reached the app %d times, want 1", f.home)
	}
}

// Both tones render the same words; the celebrated one arrives on the design
// language's motion and then rests, the quiet one is simply there.
func TestSummaryTones(t *testing.T) {
	for _, tc := range []struct {
		name string
		tone review.Tone
	}{
		{"quiet", review.Quiet},
		{"celebrated", review.Celebrated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msgs := locales.In("en")
			f := &fake{cards: map[core.ID]review.Card{}, state: &review.State{}}
			f.summary = func() (review.Summary, bool) {
				return review.Summary{Reviewed: 3, DueTomorrow: 1, Accuracy: 1, Tone: tc.tone}, true
			}
			state, deps := f.state, f.deps()
			tt := ui.NewTester(func(c *ui.Context) { review.View(c, state, deps) }, 1024, 720)
			tt.Frame()

			if !tt.HasText(msgs.SummaryTitle()) {
				t.Fatalf("the %s summary is not on screen; texts %q", tc.name, tt.Texts())
			}
			// The tone is what the summary does, not only what it says: the
			// celebrated one is still arriving on the first frame.
			if wantPending := tc.tone == review.Celebrated; tt.FramePending() != wantPending {
				t.Errorf("first frame: pending %v, want %v for the %s tone", tt.FramePending(), wantPending, tc.name)
			}
			tt.Advance(320 * time.Millisecond)
			if tt.FramePending() {
				t.Errorf("the %s summary never settles", tc.name)
			}
		})
	}
}

// A formula is set on the card: review reads the answer as a formula, and never
// as the TeX source.
func TestFormulaIsSetOnTheCard(t *testing.T) {
	f := &fake{cards: map[core.ID]review.Card{
		"c1": {ID: "c1", Prompt: `the area of a circle`, Answer: `$\pi r^2$`},
	}, state: &review.State{}}
	f.state.Sync([]core.ID{"c1"})
	state, deps := f.state, f.deps()
	tt := at(t, func(c *ui.Context) { review.View(c, state, deps) }, 1024, 720)

	if err := tt.Click(locales.In("en").ShowAnswer()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("πr²") {
		t.Fatalf("the answer is not set as a formula; texts %q", tt.Texts())
	}
	if tt.HasText(`$\pi r^2$`) {
		t.Error("the card shows the TeX source")
	}
}
