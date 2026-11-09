package store_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

// TestQueryNoteRowsDescribesEachRow is what the notebook's list shows per note:
// the first block as one line, how many paragraphs and cards there are, where
// the reader stopped, and whether the note is an excerpt.
func TestQueryNoteRowsDescribesEachRow(t *testing.T) {
	lib, _, clk := newLibrary(t)
	biology := saveNote(t, lib, "Biology",
		"spacing beats cramming:: yes\n  repeated   recall   helps",
		"the forgetting curve :: decays")
	physics := saveNote(t, lib, "Physics", "entropy :: grows")
	saveNote(t, lib, "Mention", "see [[Biology]] for more")
	clk.advance(time.Hour)

	for _, block := range biology.Blocks {
		if _, err := firstCard(lib, block.ID); err != nil {
			t.Fatalf("card: %v", err)
		}
	}
	excerpt, err := lib.Excerpt(biology.ID, biology.Blocks[1].ID, "the forgetting curve", "Excerpt of Biology")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}
	if err := lib.AddToReading(biology.ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	clk.advance(time.Hour)
	if err := lib.ReadNote(biology.ID, schedule.Good, biology.Blocks[1].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}

	rows, total, err := lib.QueryNoteRows(store.NoteRowQuery{})
	if err != nil {
		t.Fatalf("QueryNoteRows: %v", err)
	}
	if total != 4 || len(rows) != 4 {
		t.Fatalf("rows = %d of %d, want the four notes", len(rows), total)
	}
	byTitle := map[string]store.NoteRow{}
	for _, row := range rows {
		byTitle[row.Title] = row
	}
	if _, ok := byTitle[physics.Title]; !ok {
		t.Fatalf("Physics is not in the listing: %+v", rows)
	}

	tests := []struct {
		name      string
		title     string
		summary   string
		blocks    int
		cards     int
		excerpt   bool
		readingAt int
		updated   time.Time
	}{
		{
			name:    "a multi-line first block is one line",
			title:   "Biology",
			summary: "spacing beats cramming:: yes repeated recall helps",
			blocks:  2,
			cards:   2,
			// The reader stopped at the second paragraph, counted from one.
			readingAt: 2,
			updated:   biology.Updated,
		},
		{
			name:    "a note with no card",
			title:   "Physics",
			summary: "entropy :: grows",
			blocks:  1,
			updated: physics.Updated,
		},
		{
			name:    "an excerpt is a note like any other",
			title:   excerpt.Title,
			summary: "the forgetting curve",
			blocks:  1,
			excerpt: true,
			updated: excerpt.Updated,
		},
		{
			name:    "a mention is not an excerpt",
			title:   "Mention",
			summary: "see [[Biology]] for more",
			blocks:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, ok := byTitle[tt.title]
			if !ok {
				t.Fatalf("%q is not in the listing", tt.title)
			}
			if !strings.HasPrefix(row.Summary, tt.summary) {
				t.Errorf("summary = %q, want it to start with %q", row.Summary, tt.summary)
			}
			if row.Blocks != tt.blocks || row.Cards != tt.cards {
				t.Errorf("%d blocks and %d cards, want %d and %d", row.Blocks, row.Cards, tt.blocks, tt.cards)
			}
			if row.Excerpt != tt.excerpt {
				t.Errorf("excerpt = %v, want %v", row.Excerpt, tt.excerpt)
			}
			if tt.readingAt == 0 {
				if row.ReadingPos != "" || row.ReadingBlocks != 0 {
					t.Errorf("reading = %q at %d, want none: the note is not in the queue",
						row.ReadingPos, row.ReadingBlocks)
				}
				return
			}
			if row.ReadingPos != biology.Blocks[tt.readingAt-1].ID {
				t.Errorf("reading position = %q, want the %dth block %q",
					row.ReadingPos, tt.readingAt, biology.Blocks[tt.readingAt-1].ID)
			}
			if row.ReadingBlocks != tt.readingAt {
				t.Errorf("reading paragraph = %d, want %d", row.ReadingBlocks, tt.readingAt)
			}
			if !row.Updated.Equal(tt.updated) {
				t.Errorf("updated = %s, want the note's last save %s", row.Updated, tt.updated)
			}
		})
	}
}

// TestQueryNoteRowsSummaryIsOneBoundedLine is the row's one line: runs of space
// and the breaks inside a block become single spaces, and a long paragraph is cut
// so a page of them stays cheap to read.
func TestQueryNoteRowsSummaryIsOneBoundedLine(t *testing.T) {
	lib, _, _ := newLibrary(t)
	saveNote(t, lib, "Spaced", "one\n\ntwo", "third")
	long := strings.Repeat("long ", store.SummaryRunes) + "tail"
	saveNote(t, lib, "Long", long)

	rows, _, err := lib.QueryNoteRows(store.NoteRowQuery{Sort: store.NoteSortTitle})
	if err != nil {
		t.Fatalf("QueryNoteRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if got, want := rows[0].Summary, "long long long"; !strings.HasPrefix(got, want) {
		t.Errorf("summary = %q, want it to start with %q", got, want)
	}
	if n := len([]rune(rows[0].Summary)); n > store.SummaryRunes {
		t.Errorf("summary holds %d runes, want at most %d", n, store.SummaryRunes)
	}
	if strings.HasSuffix(rows[0].Summary, " ") {
		t.Errorf("summary %q ends with a space", rows[0].Summary)
	}
	if strings.Contains(rows[0].Summary, "tail") {
		t.Errorf("summary holds the end of a long paragraph: %q", rows[0].Summary)
	}
	if got, want := rows[1].Summary, "one two"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

// TestQueryNoteRowsFilters says what each toggle keeps: a word, a tag, the notes
// in the reading queue, the notes that feed cards, the excerpts. A note template
// is in none of them — the notebook's list offers templates where notes are
// started, not as documents (#154).
func TestQueryNoteRowsFilters(t *testing.T) {
	lib, _, clk := newLibrary(t)
	studied := saveNote(t, lib, "Studied", "spacing :: beats cramming #study")
	saveNote(t, lib, "JustAWord", "entropy grows")
	templated := saveNote(t, lib, "Template", "a starting point #template")
	excerpt, err := lib.Excerpt(studied.ID, studied.Blocks[0].ID, "spacing", "Excerpt of Studied")
	if err != nil {
		t.Fatalf("Excerpt: %v", err)
	}
	if err := lib.AddToReading(saveNote(t, lib, "Queued", "waiting").ID, clk.Now()); err != nil {
		t.Fatalf("AddToReading: %v", err)
	}
	if _, err := firstCard(lib, studied.Blocks[0].ID); err != nil {
		t.Fatalf("card: %v", err)
	}
	if _, err := lib.SaveNote(core.Note{ID: templated.ID, Title: templated.Title,
		Blocks: []core.Block{{Text: "a starting point #template"}}}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	tests := []struct {
		name  string
		query store.NoteRowQuery
		want  []string
	}{
		{"no filter", store.NoteRowQuery{Sort: store.NoteSortTitle},
			[]string{"Excerpt of Studied", "JustAWord", "Queued", "Studied"}},
		{"a word in a block", store.NoteRowQuery{Text: "entropy"}, []string{"JustAWord"}},
		{"a word in a title", store.NoteRowQuery{Text: "justaw"}, []string{"JustAWord"}},
		{"a tag", store.NoteRowQuery{Tag: "study"}, []string{"Studied"}},
		{"in the reading queue", store.NoteRowQuery{Reading: true}, []string{"Queued"}},
		{"feeding cards", store.NoteRowQuery{HasCards: true}, []string{"Studied"}},
		{"an excerpt", store.NoteRowQuery{Excerpt: true}, []string{excerpt.Title}},
		{"a tag and a word of another note", store.NoteRowQuery{Tag: "study", Text: "entropy"}, nil},
		{"a word nothing carries", store.NoteRowQuery{Text: "quantum"}, nil},
		{"a template is never listed", store.NoteRowQuery{Text: "starting point"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, total, err := lib.QueryNoteRows(tt.query)
			if err != nil {
				t.Fatalf("QueryNoteRows: %v", err)
			}
			got := make([]string, 0, len(rows))
			for _, row := range rows {
				got = append(got, row.Title)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("titles = %v, want %v", got, tt.want)
			}
			if total != len(tt.want) {
				t.Errorf("total = %d, want %d", total, len(tt.want))
			}
		})
	}
}

// TestQueryNoteRowsSorts pins the orders the list offers: the newest save first,
// the title ignoring case, and the most cards first.
func TestQueryNoteRowsSorts(t *testing.T) {
	lib, _, clk := newLibrary(t)
	first := saveNote(t, lib, "alpha", "a :: 1", "b :: 2")
	clk.advance(time.Hour)
	bravo := saveNote(t, lib, "Bravo", "c :: 3")
	clk.advance(time.Hour)
	saveNote(t, lib, "charlie", "d :: 4")
	// Two of the alphabetically first note's blocks feed cards, so the card
	// order and the title order disagree.
	for _, block := range first.Blocks {
		if _, err := firstCard(lib, block.ID); err != nil {
			t.Fatalf("card: %v", err)
		}
	}
	clk.advance(time.Hour)
	// The note that is already oldest is saved again, with paragraph text a
	// card can be made from: recent and title order disagree too.
	again, err := lib.SaveNote(core.Note{ID: first.ID, Title: "alpha",
		Blocks: []core.Block{{Text: "a :: 1"}, {Text: "b :: 2"}}})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	for _, block := range again.Blocks {
		if _, err := firstCard(lib, block.ID); err != nil {
			t.Fatalf("card: %v", err)
		}
	}
	if _, err := firstCard(lib, bravo.Blocks[0].ID); err != nil {
		t.Fatalf("card: %v", err)
	}

	tests := []struct {
		name  string
		order store.NoteSort
		want  []string
	}{
		{"the newest save first", store.NoteSortUpdated, []string{"alpha", "charlie", "Bravo"}},
		{"by title, ignoring case", store.NoteSortTitle, []string{"alpha", "Bravo", "charlie"}},
		{"the most cards first", store.NoteSortCards, []string{"alpha", "Bravo", "charlie"}},
		{"the oldest save first", store.NoteSortCreated, []string{"Bravo", "charlie", "alpha"}},
		{"an order the index does not know", "", []string{"alpha", "charlie", "Bravo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, _, err := lib.QueryNoteRows(store.NoteRowQuery{Sort: tt.order})
			if err != nil {
				t.Fatalf("QueryNoteRows: %v", err)
			}
			got := make([]string, 0, len(rows))
			for _, row := range rows {
				got = append(got, row.Title)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("order = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestQueryNoteRowsPagesWithoutRepeats is the paging guarantee on the row query:
// notes whose sort key ties still come out in one order, so page two begins where
// page one ended and the total is the matches, not the page.
func TestQueryNoteRowsPagesWithoutRepeats(t *testing.T) {
	lib, _, _ := newLibrary(t)
	var want []string
	for range 5 {
		note := saveNote(t, lib, "Same", "body")
		want = append(want, string(note.ID))
	}
	slices.Sort(want)

	var seen []string
	total := -1
	for offset := 0; ; offset += 2 {
		page, matches, err := lib.QueryNoteRows(store.NoteRowQuery{Sort: store.NoteSortCards, Limit: 2, Offset: offset})
		if err != nil {
			t.Fatalf("QueryNoteRows: %v", err)
		}
		if total >= 0 && matches != total {
			t.Fatalf("the total moved from %d to %d at offset %d", total, matches, offset)
		}
		total = matches
		if len(page) == 0 {
			break
		}
		for _, row := range page {
			seen = append(seen, string(row.ID))
		}
	}
	if total != len(want) {
		t.Errorf("total = %d, want %d", total, len(want))
	}
	if !slices.Equal(seen, want) {
		t.Errorf("paged rows = %v, want %v", seen, want)
	}
	if _, matches, err := lib.QueryNoteRows(store.NoteRowQuery{Offset: 100}); err != nil || matches != len(want) {
		t.Errorf("past the end: %d matches (%v), want the total %d", matches, err, len(want))
	}
}

// TestQueryNoteRowsSurvivesARebuild is the index-is-a-cache promise for the row
// query: derived again from the notes, the cards and the log, it says the same
// thing.
func TestQueryNoteRowsSurvivesARebuild(t *testing.T) {
	lib, _, clk := newLibrary(t)
	studied := saveNote(t, lib, "Studied", "spacing :: beats cramming #study", "recall :: helps")
	clk.advance(time.Hour)
	saveNote(t, lib, "Other", "entropy :: grows")
	for _, block := range studied.Blocks {
		if _, err := firstCard(lib, block.ID); err != nil {
			t.Fatalf("card: %v", err)
		}
	}
	if _, err := lib.Excerpt(studied.ID, studied.Blocks[0].ID, "spacing", "Excerpt of Studied"); err != nil {
		t.Fatalf("Excerpt: %v", err)
	}
	if err := lib.ReadNote(studied.ID, schedule.Good, studied.Blocks[1].ID, clk.Now()); err != nil {
		t.Fatalf("ReadNote: %v", err)
	}

	query := store.NoteRowQuery{Sort: store.NoteSortTitle}
	before, totalBefore, err := lib.QueryNoteRows(query)
	if err != nil {
		t.Fatalf("QueryNoteRows: %v", err)
	}
	if err := lib.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	after, totalAfter, err := lib.QueryNoteRows(query)
	if err != nil {
		t.Fatalf("QueryNoteRows after a rebuild: %v", err)
	}
	if totalAfter != totalBefore || len(after) != len(before) {
		t.Fatalf("rebuilt rows = %d of %d, want %d of %d", len(after), totalAfter, len(before), totalBefore)
	}
	// Updated is left out of the comparison: a rebuilt index holds no note save
	// time (store.readNotes reads the note files without their modification
	// time), which is that file's to fix and not this query's.
	for i := range before {
		if before[i].ID != after[i].ID || before[i].Title != after[i].Title ||
			before[i].Summary != after[i].Summary || before[i].Blocks != after[i].Blocks ||
			before[i].Cards != after[i].Cards || before[i].ReadingPos != after[i].ReadingPos ||
			before[i].ReadingBlocks != after[i].ReadingBlocks || before[i].Excerpt != after[i].Excerpt {
			t.Errorf("rebuilt row %d = %+v, want %+v", i, after[i], before[i])
		}
	}
}
