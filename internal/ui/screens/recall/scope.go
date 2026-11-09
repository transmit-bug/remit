package recall

import (
	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core/queue"
	"github.com/transmit-bug/remit/internal/ui/design"
	"github.com/transmit-bug/remit/internal/ui/kit"
	"github.com/transmit-bug/remit/locales"
)

// ScopeDialog draws the review scope over the window, while the page's scope
// button has it open: the three ways to narrow a sitting, the note or tag it
// names, how many cards that would take, and the one action that takes them. The
// choice lives in the session, never in the library, and Escape closes the
// dialog, which DialogBase keeps.
//
// The notes and tags are radio groups rather than lists of toggles: one of them
// is chosen, which is the meaning a radio group carries and a toggle does not
// (mygo-components: "one of a few, all visible"). Checked against MyGo v0.8.1.
func ScopeDialog(c *ui.Context, s *State, d Deps) {
	if !s.open {
		return
	}
	msgs := locales.In(c.Locale())
	tk := design.NewTokens(c.Theme())
	ui.DialogBase(c, &s.open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Scrim)
		panel = kit.StylePanel(c, panel.FillWidth().MaxWidth(tk.SettingsWidth).
			Padding(tk.SpaceL).Gap(tk.SpaceM)).Label(msgs.RecallScopeTitle())
		panel.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
				ui.Text(c, msgs.RecallScopeTitle()).FontSize(tk.TypeH2).Bold().Grow(1)
				ui.Button(c, "").Label(msgs.Close()).Tooltip(msgs.Close()).
					Children(func() { tk.Icon(c, "x", design.IconSmall) }).
					OnClick(func() { s.open = false })
			})
			segments(c, s, msgs)
			switch queue.ScopeKind(s.segment) {
			case queue.ScopeNote:
				notes(c, s, tk, msgs, d)
			case queue.ScopeTag:
				tags(c, s, tk, msgs, d)
			default:
				ui.Text(c, msgs.RecallScopeAllBody()).FontSize(tk.TypeUI).
					TextColor(tk.TextMuted)
			}
			ui.Divider(c)
			count := d.ScopeCounts().Count(s.draft)
			ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Wrap().Children(func() {
				ui.Text(c, msgs.RecallScopeTake(count)).FontSize(tk.TypeUI).
					TextColor(tk.TextMuted).Grow(1)
				if count == 0 {
					ui.Text(c, msgs.RecallStartEmpty()).FontSize(tk.TypeLabel).
						TextColor(tk.TextMuted)
				}
				start := ui.PrimaryButton(c, msgs.RecallScopeStart()).MinHeight(tk.Control)
				if count == 0 {
					start.Disabled(true)
					return
				}
				// The dialog closes and the app takes the scope in the same
				// gesture: a dialog that closed first would need a second one to
				// start the sitting.
				start.OnClick(func() {
					s.open = false
					if d.ApplyScope != nil {
						d.ApplyScope(s.draft)
					}
				})
			})
		})
	})
}

// segments is the dialog's first choice: the whole library, one note, or one
// tag. It is Segmented — a switch between three views — and the draft's kind
// follows from it.
func segments(c *ui.Context, s *State, msgs locales.Messages) {
	choice := ui.Segmented(c, &s.segment,
		msgs.RecallScopeAll(), msgs.RecallScopeNote(), msgs.RecallScopeTag()).
		Label(msgs.RecallScopeTitle())
	if choice.Changed() {
		s.draft = queue.Scope{Kind: queue.ScopeKind(s.segment)}
	}
}

// notes is the notes the dialog can name, one radio each with what a session
// scoped to it would take. The chosen note is the draft's, so the choice
// survives the frame without a second copy of the state.
func notes(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, d Deps) {
	all := d.ScopeNotes()
	counts := d.ScopeCounts()
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, msgs.RecallScopeNotes()).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted)
		if len(all) == 0 {
			ui.Text(c, msgs.RecallScopeNoNotes()).FontSize(tk.TypeUI).
				TextColor(tk.TextMuted)
			return
		}
		ui.Scroll(c).MaxHeight(tk.Control * 6).Children(func() {
			ui.RadioGroup(c, func() {
				for _, note := range all {
					note := note
					ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
						ui.Radio(c, &s.draft.Note, note.ID, note.Title).Grow(1)
						ui.Text(c, msgs.ReadCardsMark(counts.Notes[note.ID])).
							FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
					})
				}
			}).Label(msgs.RecallScopeNotes())
		})
	})
}

// tags is the tags the dialog can name, one radio each. A tag is a word, so the
// count beside the group says what the chosen one takes.
func tags(c *ui.Context, s *State, tk design.Tokens, msgs locales.Messages, d Deps) {
	all := d.ScopeTags()
	counts := d.ScopeCounts()
	ui.Column(c).Gap(tk.SpaceS).Children(func() {
		ui.Text(c, msgs.RecallScopeTags()).FontSize(tk.TypeLabel).
			TextColor(tk.TextMuted)
		if len(all) == 0 {
			ui.Text(c, msgs.RecallScopeNoTags()).FontSize(tk.TypeUI).
				TextColor(tk.TextMuted)
			return
		}
		ui.Scroll(c).MaxHeight(tk.Control * 6).Children(func() {
			ui.RadioGroup(c, func() {
				for _, tag := range all {
					tag := tag
					ui.Row(c).AlignItems(ui.Center).Gap(tk.SpaceS).Children(func() {
						ui.Radio(c, &s.draft.Tag, tag, "#"+tag).Grow(1)
						ui.Text(c, msgs.ReadCardsMark(counts.Tags[tag])).
							FontSize(tk.TypeLabel).TextColor(tk.TextMuted)
					})
				}
			}).Label(msgs.RecallScopeTags())
		})
	})
}
