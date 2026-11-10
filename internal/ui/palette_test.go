package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/locales"
)

// The palette opens on Cmd+K, filters as the user types, runs what is chosen
// and returns the focus where it was.
func TestPalette(t *testing.T) {
	_, _, tt := openApp(t)
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(320 * time.Millisecond) // the review card's exit
	openNote(t, tt, "Memory and recall")

	const field = "Note title"
	if err := tt.Click(field); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused(field) {
		t.Fatalf("the note's title did not take the focus; texts %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.KeyK)
	for _, want := range []string{"Review cards", "Edit notes", "New note"} {
		if !tt.HasText(want) {
			t.Fatalf("palette is missing %q; texts %q", want, tt.Texts())
		}
	}

	tt.Type("new")
	if !tt.HasText("New note") {
		t.Fatalf("the filtered palette lost the action; texts %q", tt.Texts())
	}
	for _, gone := range []string{"Review cards", "Edit notes"} {
		if tt.HasText(gone) {
			t.Errorf("filter kept %q", gone)
		}
	}

	tt.Key(0, ui.KeyEscape)
	for _, gone := range []string{"Review cards", "Edit notes"} {
		if tt.HasText(gone) {
			t.Errorf("palette stayed open after Esc: %q", gone)
		}
	}
	if !tt.Focused(field) {
		t.Errorf("Esc did not return the focus to the block field; texts %q", tt.Texts())
	}

	// Typing filters, Enter runs the action on the selection.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("new")
	tt.Key(0, ui.KeyEnter)
	if tt.HasText("Review cards") {
		t.Error("the palette stayed open after running its action")
	}
	if field, ok := tt.Access("Note title"); !ok || field.Value != "Untitled note" {
		t.Fatalf("New note did not add a note: the title reads %q; texts %q", field.Value, tt.Texts())
	}
	if !tt.Focused("Note title") {
		t.Errorf("a new note's title did not take the caret; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}

// paletteRowsAt is what the palette draws for a query, read inside a frame: the
// groups, their titles and their order are the palette's own, not a guess at the
// window's text — the page behind the dialog has words of its own.
func paletteRowsAt(a *App, query string) []paletteRow {
	var rows []paletteRow
	tt := ui.NewTester(func(c *ui.Context) {
		if rows == nil {
			rows = paletteRows(c, a.paletteEntries(c, query))
		}
		a.View(c)
	}, 1024, 720)
	tt.Frame()
	return rows
}

// paletteHeaders is the group titles the rows carry, in order.
func paletteHeaders(rows []paletteRow) []string {
	var out []string
	for _, row := range rows {
		if row.header {
			out = append(out, row.title)
		}
	}
	return out
}

// The palette lists three groups — the commands, the notes, the cards — in that
// order, and each row opens what it names: the note's title the note, a card's
// prompt the card. A group with nothing behind it is not drawn at all.
func TestPaletteGroupsCommandsNotesAndCards(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)

	groups := paletteHeaders(paletteRowsAt(a, ""))
	if want := []string{msgs.PaletteCommands(), msgs.PaletteNotes(), msgs.PaletteCards()}; !slices.Equal(groups, want) {
		t.Fatalf("the palette groups are %q, want %q", groups, want)
	}
	if got := paletteHeaders(paletteRowsAt(a, "Writing cards")); !slices.Equal(got, []string{msgs.PaletteNotes()}) {
		t.Errorf("a note's query draws %q, want the notes group alone", got)
	}
	if got := paletteHeaders(paletteRowsAt(a, "Spacing beats cramming")); !slices.Equal(got, []string{msgs.PaletteCards()}) {
		t.Errorf("a card's query draws %q, want the cards group alone", got)
	}
	if got := paletteHeaders(paletteRowsAt(a, "new note")); !slices.Equal(got, []string{msgs.PaletteCommands()}) {
		t.Errorf("a command's query draws %q, want the commands group alone", got)
	}

	// A note is found by its title and opens in the notebook.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("Writing cards")
	tt.Key(0, ui.KeyEnter)
	if a.notes.Selected != string(seed.writing.ID) {
		t.Errorf("the palette opened %q, want the note it listed", a.notes.Selected)
	}

	// A card is found by its prompt and opens in the recall queue.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("Spacing beats cramming")
	tt.Key(0, ui.KeyEnter)
	if a.area != areaRecall || a.sub != subRecallQueue {
		t.Errorf("the palette went to %v/%v, want the recall queue", a.area, a.sub)
	}

	// A query nothing matches says so instead of showing an empty list.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type("zzzz")
	if !tt.HasText(msgs.PaletteEmpty()) {
		t.Fatalf("the empty state is not on the window; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
}

// The commands a plugin declares come after the app's own: what the window can
// do is what a user knows to look for, and a plugin's commands follow it.
func TestPaletteListsPluginCommandsLast(t *testing.T) {
	a, _ := reportApp(t, &fakeRunner{})
	rows := paletteRowsAt(a, "")

	section := paletteSection(0)
	commands, plugin := 0, -1
	for i, row := range rows {
		if row.header {
			continue
		}
		if row.entry.section < section {
			t.Fatalf("the entry %q is grouped before the group above it", row.entry.action.label)
		}
		section = row.entry.section
		if row.entry.section != paletteCommands {
			continue
		}
		commands++
		if row.entry.action.label == "Leech report" {
			plugin = i
		}
	}
	if commands < 2 {
		t.Fatalf("the palette holds %d commands, too few to tell an order", commands)
	}
	if plugin < 0 {
		t.Fatal("the palette does not list the plugin's command")
	}
	last := len(rows) - 1
	for i := range rows {
		if !rows[i].header && rows[i].entry.section == paletteCommands {
			last = i
		}
	}
	if plugin != last {
		t.Errorf("the plugin's command is at row %d of a group ending at %d, not last", plugin, last)
	}
}
