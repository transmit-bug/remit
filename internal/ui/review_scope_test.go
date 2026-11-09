package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/locales"
)

// scopeSeed gives the harness library a tagged note with a card of its own, so a
// scope has something to name that the rest of the library does not match.
func scopeSeed(t *testing.T, a *App) core.Note {
	t.Helper()
	if a.lib == nil {
		t.Fatal("no library open")
	}
	note, err := a.lib.SaveNote(core.Note{Title: "A chapter of its own", Blocks: []core.Block{
		{Text: "Chapter asks :: chapter answers #chapter"},
		{Text: "Second asks :: second answers #chapter"},
	}})
	if err != nil {
		t.Fatalf("save scoped note: %v", err)
	}
	if _, err := a.lib.CreateCards(note.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}}); err != nil {
		t.Fatalf("make the scoped card: %v", err)
	}
	a.reload()
	return note
}

// scopeLabel is the scope button's name on the queue page: the scope it would
// take, which is the whole library until a dialog changes it.
func scopeButtonName(msgs locales.Messages, scope string) string {
	return msgs.ReviewScopeMenu() + ": " + scope
}

// clickRole clicks the element assistive technology knows by a role and a name,
// which is how a dialog's radios are reached: their labels are the notes' titles,
// and the page behind the dialog says the same words.
func clickRole(t *testing.T, tt *ui.Tester, role ui.Role, name string) {
	t.Helper()
	info, ok := tt.AccessByRole(role, name)
	if !ok {
		t.Fatalf("no %v named %q; texts %q", role, name, tt.Texts())
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	tt.Frame()
}

// The scope button names the scope the session draws from and opens the dialog
// that changes it, at both sizes the window holds.
func TestScopeDialogOpensFromTheQueuePage(t *testing.T) {
	msgs := locales.In("en")
	for _, size := range windowSizes {
		t.Run(size.name, func(t *testing.T) {
			a, _ := newTestApp(t)
			scopeSeed(t, a)
			tt := ui.NewTester(a.View, size.w, size.h)
			settle(tt)
			recallPage(tt)

			if err := tt.Click(scopeButtonName(msgs, msgs.ReviewScope())); err != nil {
				t.Fatalf("the page has no scope button: %v; texts %q", err, tt.Texts())
			}
			tt.Frame()
			for _, want := range []string{
				msgs.RecallScopeTitle(), msgs.RecallScopeAll(), msgs.RecallScopeNote(),
				msgs.RecallScopeTag(), msgs.RecallScopeAllBody(), msgs.RecallScopeTake(5),
			} {
				if !tt.HasText(want) {
					t.Fatalf("the dialog is missing %q; texts %q", want, tt.Texts())
				}
			}
			// Escape closes it and the page is where it was.
			tt.Key(0, ui.KeyEscape)
			tt.Advance(320 * time.Millisecond)
			if tt.HasText(msgs.RecallScopeAllBody()) {
				t.Fatalf("Escape left the scope dialog open; texts %q", tt.Texts())
			}
			if !tt.HasText(msgs.RecallStart()) {
				t.Fatalf("the queue page did not come back; texts %q", tt.Texts())
			}
		})
	}
}

// The dialog's scope is what the session takes: one note's cards instead of the
// library's, and the choice lives in the session — the library is untouched.
func TestScopeDialogNarrowsTheSession(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	note := scopeSeed(t, a)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(tt)

	if err := tt.Click(scopeButtonName(msgs, msgs.ReviewScope())); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clickRole(t, tt, ui.RoleRadio, msgs.RecallScopeNote())
	clickRole(t, tt, ui.RoleRadio, note.Title)
	if !tt.HasText(msgs.RecallScopeTake(1)) {
		t.Fatalf("the dialog does not count the note's cards; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.RecallScopeStart()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond)

	if !tt.HasText(msgs.Remaining(1)) {
		t.Fatalf("the session did not narrow to the note; texts %q", tt.Texts())
	}
	if a.reviewScope.Kind != queue.ScopeNote || a.reviewScope.Note != note.ID {
		t.Fatalf("the app did not take the scope: %+v", a.reviewScope)
	}
	// Nothing was written: the scope is the session's, not the library's.
	if got := len(a.Library().Cards()); got != 5 {
		t.Fatalf("the library holds %d cards after a scoped session, want 5", got)
	}

	// Back on the page the button names the scope, and Start takes it again.
	tt.Key(0, ui.KeyEscape)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(scopeButtonName(msgs, note.Title)) {
		t.Fatalf("the button does not name the scope; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.RecallStart()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(msgs.Remaining(1)) {
		t.Fatalf("the second sitting did not keep the scope; texts %q", tt.Texts())
	}

	// The whole library gives every card back.
	tt.Key(0, ui.KeyEscape)
	tt.Advance(320 * time.Millisecond)
	if err := tt.Click(scopeButtonName(msgs, note.Title)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clickRole(t, tt, ui.RoleRadio, msgs.RecallScopeAll())
	if !tt.HasText(msgs.RecallScopeTake(5)) {
		t.Fatalf("the whole library does not count every card; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.RecallScopeStart()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(msgs.Remaining(5)) {
		t.Fatalf("leaving the scope did not give the queue back; texts %q", tt.Texts())
	}
	if a.reviewScope.Kind != queue.ScopeAll {
		t.Fatalf("the scope did not reset: %+v", a.reviewScope)
	}
}

// A tag scope keeps the cards whose paragraphs carry the tag, and the dialog
// names the tag the session drew from.
func TestScopeDialogChoosesATag(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	scopeSeed(t, a)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(tt)

	if err := tt.Click(scopeButtonName(msgs, msgs.ReviewScope())); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clickRole(t, tt, ui.RoleRadio, msgs.RecallScopeTag())
	clickRole(t, tt, ui.RoleRadio, "#chapter")
	if !tt.HasText(msgs.RecallScopeTake(1)) {
		t.Fatalf("the dialog does not count the tag's cards; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.RecallScopeStart()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(320 * time.Millisecond)

	if !tt.HasText(msgs.Remaining(1)) {
		t.Fatalf("the tag scope did not narrow the session; texts %q", tt.Texts())
	}
	if a.reviewScope.Tag != "chapter" {
		t.Fatalf("the app did not take the tag scope: %+v", a.reviewScope)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(scopeButtonName(msgs, "#chapter")) {
		t.Fatalf("the button does not name the tag; texts %q", tt.Texts())
	}
}

// A scope with nothing due disables the dialog's action and says why: a scope of
// its own can name a note nobody owes today.
func TestScopeDialogRefusesAnEmptyScope(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	recallPage(tt)

	if err := tt.Click(scopeButtonName(msgs, msgs.ReviewScope())); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clickRole(t, tt, ui.RoleRadio, msgs.RecallScopeNote())
	// Every note is due, so the library gains one whose card waits for later.
	quiet, err := a.Library().SaveNote(core.Note{Title: "Nothing today", Blocks: []core.Block{
		{Text: "Quiet asks :: quiet answers"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	cards, err := a.Library().CreateCards(quiet.Blocks[0].ID, []core.Derived{{Type: core.TypeBasic}})
	if err != nil {
		t.Fatal(err)
	}
	cards[0].Status = core.StatusSuspended
	if err := a.Library().SaveCard(cards[0]); err != nil {
		t.Fatal(err)
	}
	a.reload()
	tt.Frame()
	clickRole(t, tt, ui.RoleRadio, quiet.Title)

	if !tt.HasText(msgs.RecallScopeTake(0)) || !tt.HasText(msgs.RecallStartEmpty()) {
		t.Fatalf("an empty scope does not say why it cannot start; texts %q", tt.Texts())
	}
	// The action is disabled, so the click lands nowhere: the dialog stays open
	// and no session starts.
	if err := tt.Click(msgs.RecallScopeStart()); err != nil {
		t.Fatalf("the action has no box: %v", err)
	}
	tt.Frame()
	if a.session != sessionNone {
		t.Errorf("an empty scope entered the session: %v", a.session)
	}
	if !tt.HasText(msgs.RecallScopeTake(0)) {
		t.Fatalf("the disabled action closed the dialog; texts %q", tt.Texts())
	}
}
