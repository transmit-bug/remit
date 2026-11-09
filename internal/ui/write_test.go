package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/ui/screens/notes"
	"github.com/transmit-bug/remit/locales"
)

// writePage opens one of the write area's pages by its tab: the notebook is
// where the area's key lands, and its list and the five pages beside it are one
// click away in the sub-navigation.
func writePage(t *testing.T, tt *ui.Tester, page string) {
	t.Helper()
	tt.Key(ui.Cmd, ui.Key2)
	tt.Frame()
	if err := tt.Click(page); err != nil {
		t.Fatalf("the write area has no %q tab: %v", page, err)
	}
	tt.Advance(320 * time.Millisecond) // the page's entrance
}

// tagNotes gives the open library two tagged notes, so a page can be narrowed
// to one of them, and returns them.
func tagNotes(t *testing.T, a *App) (learning, habit core.Note) {
	t.Helper()
	learning, err := a.Library().SaveNote(core.Note{Title: "Spacing notes", Blocks: []core.Block{
		{Text: "the gaps between reviews #learning"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	habit, err = a.Library().SaveNote(core.Note{Title: "Habit notes", Blocks: []core.Block{
		{Text: "one short sitting #habit"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	a.reload()
	return learning, habit
}

// The notes page is the library's rows: a title, the line that opens it, how
// many cards it feeds, and how many notes the page holds in all.
func TestNotesListShowsSummaryAndCards(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)

	writePage(t, tt, msgs.ModeNotes())

	if !tt.HasText(msgs.WriteListCount(3, 4)) {
		t.Fatalf("the page does not count the library; texts %q", tt.Texts())
	}
	// The row's second line is the note's first paragraph, not its title.
	if !tt.HasText("Spacing beats cramming :: reviewing just before you forget strengthens recall.") {
		t.Fatalf("the rows carry no summary; texts %q", tt.Texts())
	}
	for _, title := range []string{seed.memory.Title, seed.writing.Title, seed.habit.Title} {
		if !tt.HasText(title) {
			t.Errorf("the list is missing the note %q; texts %q", title, tt.Texts())
		}
	}
	// Memory and recall feeds two of the four cards, and the row says so to
	// assistive technology too.
	if _, ok := tt.Access(msgs.WriteCardsCount(2)); !ok {
		t.Errorf("the row carries no card count; texts %q", tt.Texts())
	}
	// The note rows are the store's rows: a card's note is not one of them.
	if tt.HasText(msgs.WriteRowNoSummary()) {
		t.Errorf("a row says its note has no text; texts %q", tt.Texts())
	}

	// A row says where the reader left the note, once a session has read it.
	if err := a.Library().ReadNote(seed.memory.ID, schedule.Good, seed.memory.Blocks[1].ID, time.Now()); err != nil {
		t.Fatalf("read note: %v", err)
	}
	a.reload()
	tt.Frame()
	if _, ok := tt.Access(msgs.WriteRowProgress(2, 3)); !ok {
		t.Errorf("the row does not say where the reader left it; texts %q", tt.Texts())
	}

	// The row is the way into the note.
	if err := tt.Click(seed.writing.Title); err != nil {
		t.Fatalf("the note's row is not a control: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.sub != subNotes || a.notes.Loaded != string(seed.writing.ID) {
		t.Fatalf("the row opened %v on %q, want the note itself", a.sub, a.notes.Loaded)
	}
}

// A tag narrows the list to the notes that carry it, and the chip above the list
// puts the whole notebook back.
func TestNotesListFiltersByTag(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	learning, habit := tagNotes(t, a)

	writePage(t, tt, msgs.WriteTagsTitle())
	if err := tt.Click("#" + core.Tags(learning.Blocks[0].Text)[0]); err != nil {
		t.Fatalf("the tags page has no row for the tag: %v", err)
	}
	tt.Advance(320 * time.Millisecond)

	if a.area != areaWrite || a.sub != subNoteList {
		t.Fatalf("the tag went to %v/%v, want the notes list", a.area, a.sub)
	}
	if a.notes.TagFilter == "" {
		t.Fatal("the tag did not reach the list's filter")
	}
	if !tt.HasText(learning.Title) {
		t.Fatalf("the tagged note is not listed; texts %q", tt.Texts())
	}
	for _, gone := range []string{habit.Title, "Memory and recall"} {
		if tt.HasText(gone) {
			t.Errorf("%q is listed under a tag it does not carry", gone)
		}
	}

	// The chip says which tag the list is narrowed to, and clears it.
	if err := tt.Click(msgs.ClearFilter()); err != nil {
		t.Fatalf("the chip is not on the page: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.notes.TagFilter != "" {
		t.Fatalf("the filter is still %q", a.notes.TagFilter)
	}
	for _, want := range []string{learning.Title, habit.Title, "Memory and recall"} {
		if !tt.HasText(want) {
			t.Errorf("clearing the tag did not bring %q back; texts %q", want, tt.Texts())
		}
	}
}

// The three filters are the store's: With cards keeps the notes whose
// paragraphs feed one, and All puts the notebook back.
func TestNotesListFilterKeepsOnlyNotesWithCards(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)

	writePage(t, tt, msgs.ModeNotes())
	if err := tt.Click(msgs.WriteFilterCards()); err != nil {
		t.Fatalf("the page has no cards filter: %v", err)
	}
	tt.Advance(320 * time.Millisecond)

	if !a.notes.Filter.HasCards {
		t.Fatal("the filter did not reach the screen")
	}
	if tt.HasText(seed.habit.Title) {
		t.Errorf("a note that feeds no card is still listed; texts %q", tt.Texts())
	}
	for _, want := range []string{seed.memory.Title, seed.writing.Title} {
		if !tt.HasText(want) {
			t.Errorf("the filter dropped the note %q that feeds cards; texts %q", want, tt.Texts())
		}
	}

	if err := tt.Click(msgs.WriteFilterAll()); err != nil {
		t.Fatalf("the page has no All: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.notes.Filter.Any() {
		t.Fatalf("All left the filter %+v", a.notes.Filter)
	}
	if !tt.HasText(seed.habit.Title) {
		t.Errorf("All did not bring the note back; texts %q", tt.Texts())
	}
}

// The order the user chose is the screen's, not the load's: a reload keeps it,
// and it is the store that sorts by it.
func TestNotesSortCyclesAndSurvivesReload(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)

	writePage(t, tt, msgs.ModeNotes())
	if err := tt.Click(msgs.WriteSort()); err != nil {
		t.Fatalf("the page has no sort menu: %v", err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem(msgs.WriteSortTitle()); err != nil {
		t.Fatalf("the sort menu has no title order: %v; items %q", err, tt.Menu())
	}
	tt.Advance(320 * time.Millisecond)
	if got := topRow(t, tt, "A daily habit", "Memory and recall", "Writing cards"); got != "A daily habit" {
		t.Fatalf("the title order starts with %q", got)
	}
	if a.notes.Sort != notes.SortTitle {
		t.Fatalf("the order is %v, want title", a.notes.Sort)
	}

	// The order is the store's answer, not the screen's guess: by cards, the
	// note that feeds none is last.
	if err := tt.Click(msgs.WriteSort()); err != nil {
		t.Fatalf("the page has no sort menu: %v", err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem(msgs.WriteSortCards()); err != nil {
		t.Fatalf("the sort menu has no cards order: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.notes.Sort != notes.SortCards {
		t.Fatalf("the order is %v, want cards", a.notes.Sort)
	}
	last := bottomRow(t, tt, "A daily habit", "Memory and recall", "Writing cards")
	if last != "A daily habit" {
		t.Fatalf("the note that feeds no card is not last, but %q is", last)
	}

	// A reload — what every write ends with — keeps the order the user chose,
	// and the rows come back in it.
	a.reload()
	tt.Frame()
	if a.notes.Sort != notes.SortCards {
		t.Fatalf("the reload lost the order: %v", a.notes.Sort)
	}
	if got := bottomRow(t, tt, "A daily habit", "Memory and recall", "Writing cards"); got != "A daily habit" {
		t.Fatalf("the rows came back out of order, ending with %q", got)
	}
}

// bottomRow is which of the titles is drawn lowest.
func bottomRow(t *testing.T, tt *ui.Tester, titles ...string) string {
	t.Helper()
	bottom, y := "", float32(-1)
	for _, title := range titles {
		box, ok := tt.Find(title)
		if !ok {
			t.Fatalf("%q has no box; texts %q", title, tt.Texts())
		}
		if y < 0 || box.Y > y {
			bottom, y = title, box.Y
		}
	}
	return bottom
}

// topRow is which of the titles is drawn highest: the lists are rows, not a
// sorted column of numbers, so the order is what the eye reads.
func topRow(t *testing.T, tt *ui.Tester, titles ...string) string {
	t.Helper()
	top, y := "", float32(-1)
	for _, title := range titles {
		box, ok := tt.Find(title)
		if !ok {
			t.Fatalf("%q has no box; texts %q", title, tt.Texts())
		}
		if y < 0 || box.Y < y {
			top, y = title, box.Y
		}
	}
	return top
}

// The search page keeps the notes and the cards apart: a note hit opens the
// note, and a card hit opens the note the card renders from, where its text is.
func TestSearchGroupsNotesAndCards(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)

	a.Search("question")
	tt.Frame()

	if !tt.HasText(msgs.WriteSearchCount(a.write.hitsTotal, "question")) {
		t.Fatalf("the page does not count the results; texts %q", tt.Texts())
	}
	if a.write.hitsTotal != len(a.write.hits)+len(a.write.cardHits) {
		t.Errorf("the total %d is not the two groups' rows %d+%d",
			a.write.hitsTotal, len(a.write.hits), len(a.write.cardHits))
	}
	for _, group := range []string{msgs.WriteSearchNotes(), msgs.WriteSearchCards()} {
		if _, ok := tt.Access(group); !ok {
			t.Errorf("the page has no %q group; texts %q", group, tt.Texts())
		}
	}
	// The card's row is the card's own, with the note it came from: choosing it
	// shows that note with the card's text on the page.
	card := msgs.WriteSearchCards() + " · " + seed.writing.Title
	if err := tt.Click(card); err != nil {
		t.Fatalf("the card hit is not a row: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.sub != subNotes {
		t.Fatalf("the card hit went to %v, want the notebook", a.sub)
	}
	if a.notes.Loaded != string(seed.writing.ID) {
		t.Fatalf("the card hit opened %q, want its note", a.notes.Loaded)
	}
	if a.notes.View != notes.Read {
		t.Errorf("the card hit opened the note in view %v, want it read whole", a.notes.View)
	}
}

// ⌘F from another area opens the search page with the caret in its field, so
// what is typed next is the query.
func TestSearchFromAnotherAreaPutsTheCaretInTheField(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Advance(320 * time.Millisecond)
	if a.area != areaWrite || a.sub != subSearch {
		t.Fatalf("⌘F went to %v/%v, want the search page", a.area, a.sub)
	}
	if !tt.Focused(msgs.WriteSearchPlaceholder()) {
		t.Fatalf("the caret is not in the search field; texts %q", tt.Texts())
	}

	tt.Type("cramming")
	tt.Advance(320 * time.Millisecond)
	if !tt.HasText(seed.memory.Title) {
		t.Fatalf("what was typed did not reach the field; texts %q", tt.Texts())
	}
	if tt.HasText(seed.writing.Title) {
		t.Errorf("the query kept a note that does not match")
	}
}

// The tags page answers which tags the library carries and how many notes hold
// each, and choosing one opens the notes page narrowed to it.
func TestTagPageOpensTheFilteredList(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	learning, habit := tagNotes(t, a)

	writePage(t, tt, msgs.WriteTagsTitle())

	for _, want := range []string{"#" + "learning", "#" + "habit", msgs.WriteTagNotes(1)} {
		if !tt.HasText(want) {
			t.Fatalf("the tags page is missing %q; texts %q", want, tt.Texts())
		}
	}
	// The row says what the tag is about: the notes that carry it.
	if !tt.HasText(learning.Title) {
		t.Errorf("the tag row names none of its notes; texts %q", tt.Texts())
	}
	if err := tt.Click("#" + "learning"); err != nil {
		t.Fatalf("the tag row is not a control: %v", err)
	}
	tt.Advance(320 * time.Millisecond)

	if a.area != areaWrite || a.sub != subNoteList || a.notes.TagFilter != "learning" {
		t.Fatalf("the tag opened %v/%v filtered by %q, want the notes page on learning",
			a.area, a.sub, a.notes.TagFilter)
	}
	if !tt.HasText(learning.Title) || tt.HasText(habit.Title) {
		t.Fatalf("the list is not narrowed to the tag; texts %q", tt.Texts())
	}
}

// The assets page names the notes that carry a file, and says which files no
// note references.
func TestAssetsListNamesTheReferencingNotes(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)

	rel, err := a.Library().PutAsset([]byte("not quite a png, but bytes"), "png")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}
	orphan, err := a.Library().PutAsset([]byte("no note carries this one"), "wav")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}
	carrier, err := a.Library().SaveNote(core.Note{Title: "Pictures of the day", Blocks: []core.Block{
		{Text: "![a picture](" + rel + ")"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	a.reload()

	writePage(t, tt, msgs.WriteAssetsTitle())

	for _, want := range []string{
		rel[len("assets/"):],
		orphan[len("assets/"):],
		msgs.WriteAssetRefs(1) + " · " + carrier.Title,
		msgs.WriteAssetUnreferenced(),
		msgs.WriteKindImage(),
		msgs.WriteKindSound(),
	} {
		if !tt.HasText(want) {
			t.Fatalf("the assets page is missing %q; texts %q", want, tt.Texts())
		}
	}

	// The row is the way into the note that carries the file.
	if err := tt.Click(rel[len("assets/"):]); err != nil {
		t.Fatalf("the asset is not a row: %v", err)
	}
	tt.Advance(320 * time.Millisecond)
	if a.notes.Loaded != string(carrier.ID) {
		t.Fatalf("the row opened %q, want the note that carries the file", a.notes.Loaded)
	}
}

// A template starts a note: the copy keeps the text, loses the marker, and the
// template is still there for the next note.
func TestTemplateStartsANoteWithoutTheMarker(t *testing.T) {
	msgs := locales.In("en")
	a, _ := newTestApp(t)
	templateID := saveTemplate(t, a)
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)

	writePage(t, tt, msgs.WriteTemplatesTitle())
	if !tt.HasText("Reading excerpt") {
		t.Fatalf("the template is not on its page; texts %q", tt.Texts())
	}
	if err := tt.Click(msgs.WriteTemplateNew() + " Reading excerpt"); err != nil {
		t.Fatalf("the template is not a control: %v", err)
	}
	tt.Advance(320 * time.Millisecond)

	if a.notes.Draft.ID == "" || a.notes.Draft.ID == templateID {
		t.Fatalf("the template did not start a note; the draft is %q", a.notes.Draft.ID)
	}
	for _, block := range a.notes.Draft.Blocks {
		if block.Text == "#"+core.TemplateTag {
			t.Fatalf("the copy kept the template marker: %+v", a.notes.Draft.Blocks)
		}
	}
	if len(a.notes.Draft.Blocks) == 0 ||
		a.notes.Draft.Blocks[0].Text != "Chapter asks :: chapter answers" {
		t.Fatalf("the copy lost the template's text: %+v", a.notes.Draft.Blocks)
	}
	// The copy is the note the notebook shows, and the template is still a
	// template.
	if !tt.HasText("Untitled note") {
		t.Errorf("the copy is not the note on screen; texts %q", tt.Texts())
	}
	if _, ok := a.data.note(templateID); !ok {
		t.Error("the template is gone after starting a note from it")
	}
}

// An excerpt sits under the note it was taken from, indented as a child of it.
func TestExcerptTreeNestsChildren(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)

	source, err := a.Library().Excerpt(seed.memory.ID, seed.memory.Blocks[0].ID,
		"Spacing beats cramming", "Spacing, quoted")
	if err != nil {
		t.Fatalf("excerpt: %v", err)
	}
	a.reload()

	writePage(t, tt, msgs.WriteTreeTitle())

	parent, ok := tt.Find(seed.memory.Title)
	if !ok {
		t.Fatalf("the tree is missing the note the excerpt came from; texts %q", tt.Texts())
	}
	child, ok := tt.Find(source.Title)
	if !ok {
		t.Fatalf("the tree is missing the excerpt; texts %q", tt.Texts())
	}
	if child.Y <= parent.Y {
		t.Errorf("the excerpt at %v is not under its source at %v", child.Y, parent.Y)
	}
	if child.X <= parent.X {
		t.Errorf("the excerpt at %v is not indented under its source at %v", child.X, parent.X)
	}
	// The tree is a tree to assistive technology too.
	if _, ok := tt.AccessByRole(ui.RoleTree, msgs.WriteTreeTitle()); !ok {
		t.Errorf("the tree is not a tree; texts %q", tt.Texts())
	}
}

// An excerpt whose source block is gone stands in a section of its own: it is
// still a note, and the page says why it is a root.
func TestExcerptTreeListsOrphans(t *testing.T) {
	msgs := locales.In("en")
	a, seed, tt := openApp(t)

	source, err := a.Library().Excerpt(seed.memory.ID, seed.memory.Blocks[0].ID,
		"Spacing beats cramming", "Spacing, quoted")
	if err != nil {
		t.Fatalf("excerpt: %v", err)
	}
	// The paragraph the excerpt quotes is edited away: the anchor names a block
	// nothing holds any more.
	edited := seed.memory
	edited.Blocks = edited.Blocks[1:]
	if _, err := a.Library().SaveNote(edited); err != nil {
		t.Fatal(err)
	}
	a.reload()

	writePage(t, tt, msgs.WriteTreeTitle())

	if !tt.HasText(msgs.WriteTreeOrphans()) {
		t.Fatalf("the tree has no orphans section; texts %q", tt.Texts())
	}
	if !tt.HasText(msgs.WriteTreeOrphanHint()) {
		t.Errorf("the orphans section does not say why they stand alone; texts %q", tt.Texts())
	}
	orphan, ok := tt.Find(source.Title)
	if !ok {
		t.Fatalf("the orphaned excerpt is not listed; texts %q", tt.Texts())
	}
	heading, ok := tt.Find(msgs.WriteTreeOrphans())
	if !ok || orphan.Y <= heading.Y {
		t.Errorf("the excerpt at %v is not under the orphans heading at %v", orphan.Y, heading.Y)
	}
}

// Every write page holds at the smallest window: its title, the control that
// fills it, and no row outside the window's width.
func TestWritePagesAtTheMinimumSize(t *testing.T) {
	msgs := locales.In("en")
	a, _, tt := openApp(t)
	tt.SetSize(720, 520)
	tt.Frame()
	tagNotes(t, a)

	pages := []struct {
		tab     string
		title   string
		control string // what the page is for: the one control it must keep in reach
	}{
		{msgs.ModeNotes(), msgs.ModeNotes(), msgs.WriteSort()},
		{msgs.SearchPage(), msgs.WriteSearch(), msgs.WriteSearchPlaceholder()},
		{msgs.WriteTagsTitle(), msgs.WriteTagsTitle(), "#learning"},
		{msgs.WriteAssetsTitle(), msgs.WriteAssetsTitle(), msgs.WriteAssetsImport()},
		{msgs.WriteTemplatesTitle(), msgs.WriteTemplatesTitle(), msgs.WriteTemplateBlank()},
		{msgs.WriteTreeTitle(), msgs.WriteTreeTitle(), msgs.WriteTreeContinue()},
	}
	for _, page := range pages {
		writePage(t, tt, page.tab)
		assertControl(t, tt, 720, 520, page.control)
		box, ok := tt.Find(page.title)
		if !ok {
			t.Fatalf("the %q page has no title; texts %q", page.tab, tt.Texts())
		}
		if box.W <= 0 || box.H <= 0 {
			t.Errorf("the %q page's title has an empty box", page.tab)
		}
		if box.X < 0 || box.X+box.W > 720 {
			t.Errorf("the %q page's title is outside the 720-wide window: %+v", page.tab, box)
		}
		if tt.FramePending() {
			t.Errorf("the %q page still requests frames", page.tab)
		}
	}
}

// The pages are the app's own language: in Chinese every one of them says what
// it is, from the catalog rather than from a second code path.
func TestWritePagesInChinese(t *testing.T) {
	msgs := locales.In("zh-Hans")
	a, _, tt := openApp(t)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	tagNotes(t, a)

	for _, page := range []struct {
		tab   string
		title string
		more  string
	}{
		{msgs.ModeNotes(), msgs.ModeNotes(), msgs.WriteFilterReading()},
		{msgs.SearchPage(), msgs.WriteSearch(), msgs.WriteSearchPlaceholder()},
		{msgs.WriteTagsTitle(), msgs.WriteTagsTitle(), msgs.WriteTagNotes(1)},
		{msgs.WriteAssetsTitle(), msgs.WriteAssetsTitle(), msgs.WriteAssetsImport()},
		{msgs.WriteTemplatesTitle(), msgs.WriteTemplatesTitle(), msgs.WriteTemplateBlank()},
		{msgs.WriteTreeTitle(), msgs.WriteTreeTitle(), msgs.WriteTreeContinue()},
	} {
		writePage(t, tt, page.tab)
		for _, want := range []string{page.title, page.more} {
			if !tt.HasText(want) {
				t.Fatalf("the %q page is missing %q; texts %q", page.tab, want, tt.Texts())
			}
		}
	}
}

// A row's day follows the frame's language, not the app's: the notes written in
// this run read as today, in the words of the language on screen.
func TestNotesListDayFollowsTheFrameLanguage(t *testing.T) {
	msgs := locales.In("zh-Hans")
	_, _, tt := openApp(t)
	tt.SetLocale("zh-Hans")
	tt.Frame()

	writePage(t, tt, msgs.ModeNotes())
	if !tt.HasText(msgs.ReadDayToday()) {
		t.Fatalf("the rows' day is not Chinese; texts %q", tt.Texts())
	}
}

// A search link from outside the window opens the write area's search page on
// its query, its field ready for typing.
func TestSearchLink(t *testing.T) {
	a, _ := newTestApp(t)

	// The link may arrive before the window is drawn: the focus request waits
	// for the field's first build.
	a.Search("question")
	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	for _, want := range []string{"Search", "Writing cards"} {
		if !tt.HasText(want) {
			t.Fatalf("the search link is missing %q; texts %q", want, tt.Texts())
		}
	}
	if a.area != areaWrite || a.sub != subSearch {
		t.Fatalf("the link went to %v/%v, want the search page", a.area, a.sub)
	}

	// Typing reaches the field the link focused.
	tt.Type("zzz")
	if !tt.HasText("No note or card matches.") {
		t.Fatalf("the search field did not take the text; texts %q", tt.Texts())
	}
	if tt.FramePending() {
		t.Error("view still requests frames after it should settle")
	}
}
