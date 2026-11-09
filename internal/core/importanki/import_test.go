package importanki_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/importanki"
	"github.com/transmit-bug/remit/internal/core/store"
)

func TestImportMinimalPackage(t *testing.T) {
	lib, dir := newLibrary(t)
	path := buildPackage(t, minimalFixture())

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 2 || report.Cards != 3 || report.Reviews != 5 {
		t.Fatalf("report = %+v, want 2 notes, 3 cards, 5 reviews", report)
	}
	if report.SkippedRows != 0 || report.UnreachableMedia != 0 {
		t.Errorf("skipped rows = %d, unreachable media = %d, want 0 and 0",
			report.SkippedRows, report.UnreachableMedia)
	}
	if got := len(lib.Notes()); got != 2 {
		t.Fatalf("library holds %d notes, want 2", got)
	}
	if got := len(lib.Cards()); got != 3 {
		t.Fatalf("library holds %d cards, want 3", got)
	}

	records := readReviews(t, dir)
	if len(records) != 5 {
		t.Fatalf("review log holds %d events, want 5", len(records))
	}
	cards := cardsByPrompt(t, lib)
	tests := []struct {
		prompt  string
		ratings []string
		reviews int
		last    time.Time
		due     time.Time
	}{
		{
			prompt:  "Capital of France?",
			ratings: []string{"good", "easy"},
			reviews: 2,
			last:    t0.Add(72 * time.Hour),
			due:     time.Date(2026, time.December, 3, 9, 0, 0, 0, time.UTC),
		},
		{
			prompt:  "Capital of Japan?",
			ratings: []string{"again", "good"},
			reviews: 2,
			last:    t0.Add(48 * time.Hour),
			due:     time.Date(2026, time.November, 10, 9, 0, 0, 0, time.UTC),
		},
		{
			// The reverse template names {{Back}}, so the card starts there.
			prompt:  "Tokyo",
			ratings: []string{"good"},
			reviews: 1,
			last:    t0.Add(96 * time.Hour),
			due:     time.Date(2026, time.November, 12, 9, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			card, ok := cards[tt.prompt]
			if !ok {
				t.Fatalf("no card has %q as its prompt; cards are %v", tt.prompt, prompts(t, lib))
			}
			if got := ratingsFor(records, card.ID); strings.Join(got, ",") != strings.Join(tt.ratings, ",") {
				t.Errorf("ratings = %v, want %v", got, tt.ratings)
			}
			state, ok := lib.CardState(card.ID)
			if !ok {
				t.Fatalf("card %q has no replayed state", card.ID)
			}
			if state.Reviews != tt.reviews {
				t.Errorf("replayed reviews = %d, want %d", state.Reviews, tt.reviews)
			}
			if !state.LastReview.Equal(tt.last) {
				t.Errorf("last review = %v, want %v", state.LastReview, tt.last)
			}
			if !state.Due.Equal(tt.due) {
				t.Errorf("due = %v, want %v", state.Due, tt.due)
			}
		})
	}

	// The prompt is the block and the answer is the rendered back template, as
	// the card plugin shapes them.
	card := cards["Capital of France?"]
	prompt, answer, state := cardFaces(t, lib, card)
	if prompt != "Capital of France?" || answer != "Paris" || state != core.LinkLive {
		t.Errorf("faces = (%q, %q, %v), want the block's live text", prompt, answer, state)
	}
	reverse := cards["Tokyo"]
	if prompt, answer, _ := cardFaces(t, lib, reverse); prompt != "Tokyo" || answer != "Capital of Japan?" {
		t.Errorf("reverse card = (%q, %q), want (Tokyo, Capital of Japan?)", prompt, answer)
	}

	var titles []string
	for _, note := range lib.Notes() {
		titles = append(titles, note.Title)
	}
	for _, want := range []string{"Capital of France?", "Capital of Japan?"} {
		if !contains(titles, want) {
			t.Errorf("notes %v do not include %q", titles, want)
		}
	}
}

func TestReimportAddsNothing(t *testing.T) {
	lib, dir := newLibrary(t)
	path := buildPackage(t, minimalFixture())

	first, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("first Import: %v", err)
	}
	if first.Notes != 2 || first.Cards != 3 || first.Reviews != 5 {
		t.Fatalf("first report = %+v", first)
	}

	// Reopen, so the second import proves the key survives on disk.
	if err := lib.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := store.Open(dir, store.Options{Now: func() time.Time { return fixedNow }})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { reopened.Close() })

	second, err := importanki.Import(path, reopened)
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if second.Notes != 0 || second.Cards != 0 || second.Reviews != 0 {
		t.Errorf("second import wrote %+v, want nothing", second)
	}
	if second.SkippedRows != 0 {
		t.Errorf("second import skipped %d rows, want 0", second.SkippedRows)
	}
	if got := len(reopened.Notes()); got != 2 {
		t.Errorf("library holds %d notes after re-import, want 2", got)
	}
	if got := len(reopened.Cards()); got != 3 {
		t.Errorf("library holds %d cards after re-import, want 3", got)
	}
	if got := len(readReviews(t, dir)); got != 5 {
		t.Errorf("review log holds %d events after re-import, want 5", got)
	}
}

func TestSuspendedAndBuriedCards(t *testing.T) {
	lib, _ := newLibrary(t)
	f := fixture{
		models: basicModels(),
		decks:  basicDecks(),
		notes: []noteRow{
			{id: 3001, guid: "guid-suspended", mid: 1000, fields: []string{"Suspended?", "Yes", "y"}},
		},
		cards: []cardRow{
			{id: 301, note: 3001, deck: 1, ord: 0, queue: -1},
			{id: 302, note: 3001, deck: 1, ord: 1, queue: -2},
		},
		revlog: []revlogRow{
			{id: t0.UnixMilli(), card: 301, ease: 3, kind: 1},
		},
	}
	path := buildPackage(t, f)

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Cards != 2 || report.Reviews != 1 {
		t.Fatalf("report = %+v, want 2 cards and 1 review", report)
	}

	cards := cardsByPrompt(t, lib)
	for _, prompt := range []string{"Suspended?", "Yes"} {
		card, ok := cards[prompt]
		if !ok {
			t.Fatalf("no card has %q as its prompt", prompt)
		}
		if card.Status != core.StatusSuspended {
			t.Errorf("card %q status = %v, want suspended", prompt, card.Status)
		}
	}
	// A suspended card keeps the history it had in Anki.
	state, ok := lib.CardState(cards["Suspended?"].ID)
	if !ok || state.Reviews != 1 {
		t.Errorf("suspended card state = (%+v, %v), want one review", state, ok)
	}
	due, err := lib.Due(fixedNow, 0)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(due) != 0 {
		t.Errorf("%d suspended cards are due, want none", len(due))
	}
}

func TestHTMLFieldsBecomeBlocks(t *testing.T) {
	lib, _ := newLibrary(t)
	f := fixture{
		models: basicModels(),
		decks:  basicDecks(),
		notes: []noteRow{{
			id: 4001, guid: "guid-html", mid: 1000,
			fields: []string{
				"<b>Q</b>&nbsp;one<br>second &amp; line",
				"A &amp; B <img src=\"diagram.png\">",
			},
		}},
		cards:      []cardRow{{id: 401, note: 4001, deck: 1, ord: 0}},
		media:      map[string]string{"0": "diagram.png"},
		mediaFiles: map[string]string{"0": "png-bytes"},
	}
	path := buildPackage(t, f)

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.UnreachableMedia != 1 {
		t.Errorf("unreachable media = %d, want 1", report.UnreachableMedia)
	}
	if !hasWarning(report, "no media directory") {
		t.Errorf("warnings do not mention the media directory: %q", report.Warnings)
	}

	notes := lib.Notes()
	if len(notes) != 1 {
		t.Fatalf("library holds %d notes, want 1", len(notes))
	}
	want := []string{"Q one", "second & line", "A & B [img: diagram.png]"}
	if len(notes[0].Blocks) != len(want) {
		t.Fatalf("blocks = %q, want %q", blockTexts(notes[0]), want)
	}
	for i, block := range notes[0].Blocks {
		if block.Text != want[i] {
			t.Errorf("block %d = %q, want %q", i, block.Text, want[i])
		}
	}

	card := cardsByPrompt(t, lib)["Q one\nsecond & line"]
	prompt, answer, _ := cardFaces(t, lib, card)
	if prompt != "Q one\nsecond & line" {
		t.Errorf("prompt = %q, want the whole multi-line field", prompt)
	}
	if answer != "A & B [img: diagram.png]" {
		t.Errorf("answer = %q, want the plain back field", answer)
	}
}

func TestReviewRatings(t *testing.T) {
	lib, dir := newLibrary(t)
	at := func(hours int) int64 { return t0.Add(time.Duration(hours) * time.Hour).UnixMilli() }
	f := fixture{
		models: basicModels(),
		decks:  basicDecks(),
		notes: []noteRow{
			{id: 5001, guid: "guid-review", mid: 1000, fields: []string{"Review?", "yes"}},
			{id: 5002, guid: "guid-learn", mid: 1000, fields: []string{"Learn?", "yes"}},
			{id: 5003, guid: "guid-skip", mid: 1000, fields: []string{"Skip?", "yes"}},
		},
		cards: []cardRow{
			{id: 501, note: 5001, deck: 1, ord: 0},
			{id: 502, note: 5002, deck: 1, ord: 0},
			{id: 503, note: 5003, deck: 1, ord: 0},
		},
		revlog: []revlogRow{
			{id: at(0), card: 501, ease: 1, kind: 1},
			{id: at(1), card: 501, ease: 2, kind: 1},
			{id: at(2), card: 501, ease: 3, kind: 1},
			{id: at(3), card: 501, ease: 4, kind: 1},
			{id: at(4), card: 502, ease: 1, kind: 0},
			{id: at(5), card: 502, ease: 2, kind: 0},
			{id: at(6), card: 502, ease: 3, kind: 0},
			{id: at(7), card: 502, ease: 1, kind: 2},
			{id: at(8), card: 502, ease: 4, kind: 2},
			{id: at(9), card: 503, ease: 3, kind: 3},
			{id: at(10), card: 503, ease: 3, kind: 4},
			{id: at(11), card: 503, ease: 3, kind: 5},
		},
	}
	path := buildPackage(t, f)

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Reviews != 9 || report.SkippedRows != 3 {
		t.Fatalf("report = %+v, want 9 reviews and 3 skipped rows", report)
	}

	cards := cardsByPrompt(t, lib)
	records := readReviews(t, dir)
	tests := []struct {
		prompt  string
		ratings []string
	}{
		{"Review?", []string{"again", "hard", "good", "easy"}},
		{"Learn?", []string{"again", "good", "good", "again", "good"}},
		{"Skip?", nil},
	}
	for _, tt := range tests {
		card, ok := cards[tt.prompt]
		if !ok {
			t.Fatalf("no card has %q as its prompt", tt.prompt)
		}
		got := ratingsFor(records, card.ID)
		if strings.Join(got, ",") != strings.Join(tt.ratings, ",") {
			t.Errorf("%s ratings = %v, want %v", tt.prompt, got, tt.ratings)
		}
	}
	if !hasWarning(report, "not graded reviews") {
		t.Errorf("warnings do not explain the skipped rows: %q", report.Warnings)
	}
}

func TestClozeCards(t *testing.T) {
	lib, _ := newLibrary(t)
	f := fixture{
		models: basicModels(),
		decks:  basicDecks(),
		notes: []noteRow{{
			id: 6001, guid: "guid-cloze", mid: 2000,
			fields: []string{"{{c1::Paris}} is in {{c2::France}}", "Europe"},
		}},
		cards: []cardRow{
			{id: 601, note: 6001, deck: 1, ord: 1},
			{id: 602, note: 6001, deck: 1, ord: 2},
		},
	}
	path := buildPackage(t, f)

	report, err := importanki.Import(path, lib)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.Notes != 1 || report.Cards != 2 {
		t.Fatalf("report = %+v, want 1 note and 2 cards", report)
	}
	if !hasWarning(report, "cloze") {
		t.Errorf("warnings do not explain the cloze mapping: %q", report.Warnings)
	}

	notes := lib.Notes()
	if len(notes) != 1 || len(notes[0].Blocks) != 2 {
		t.Fatalf("note = %+v, want two blocks", notes)
	}
	if notes[0].Blocks[0].Text != "Paris is in France" {
		t.Errorf("cloze block = %q, want the unwrapped text", notes[0].Blocks[0].Text)
	}

	cards := cardsByPrompt(t, lib)
	for prompt, want := range map[string]string{
		"[...] is in France": "Paris is in France",
		"Paris is in [...]":  "Paris is in France",
	} {
		card, ok := cards[prompt]
		if !ok {
			t.Fatalf("no card has %q as its prompt", prompt)
		}
		got, answer, _ := cardFaces(t, lib, card)
		if got != prompt {
			t.Errorf("prompt = %q, want %q", got, prompt)
		}
		if answer != want {
			t.Errorf("answer = %q, want %q", answer, want)
		}
		if card.Ref.Block != notes[0].Blocks[0].ID {
			t.Errorf("cloze card references %q, want the cloze field's block", card.Ref.Block)
		}
	}
}

func TestImportErrors(t *testing.T) {
	corrupt := filepath.Join(t.TempDir(), "corrupt.apkg")
	if err := os.WriteFile(corrupt, []byte("this is not a zip"), 0o644); err != nil {
		t.Fatalf("write corrupt package: %v", err)
	}
	// A zstd entry that holds an uncompressed database: the entry name says
	// schema 18, the bytes do not.
	uncompressed := buildPackage(t, fixture{collection: "collection.anki21b"})

	fixtures := map[string]fixture{
		"missing cards table":  {models: basicModels(), decks: basicDecks(), omit: []string{"cards"}},
		"missing notes table":  {models: basicModels(), decks: basicDecks(), omit: []string{"notes"}},
		"missing col table":    {models: basicModels(), decks: basicDecks(), omit: []string{"col"}},
		"missing revlog table": {models: basicModels(), decks: basicDecks(), omit: []string{"revlog"}},
		"unknown notetype": {
			models: basicModels(), decks: basicDecks(),
			notes: []noteRow{{id: 7001, guid: "guid-unknown", mid: 9999, fields: []string{"a"}}},
		},
		"revlog without a card": {
			models: basicModels(), decks: basicDecks(),
			notes: []noteRow{{id: 7002, guid: "guid-orphan", mid: 1000, fields: []string{"a", "b"}}},
			cards: []cardRow{{id: 701, note: 7002, deck: 1, ord: 0}},
			revlog: []revlogRow{
				{id: t0.UnixMilli(), card: 999, ease: 3, kind: 1},
			},
		},
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "corrupt zip", path: corrupt, want: "not an Anki package"},
		{name: "anki21b that is not zstd", path: uncompressed, want: "collection.anki21b"},
	}
	for name, f := range fixtures {
		path := buildPackage(t, f)
		want := strings.TrimSuffix(strings.TrimPrefix(name, "missing "), " table")
		if name == "unknown notetype" {
			want = "unknown notetype"
		}
		if name == "revlog without a card" {
			want = "revlog row"
		}
		tests = append(tests, struct {
			name string
			path string
			want string
		}{name: name, path: path, want: want})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib, _ := newLibrary(t)
			_, err := importanki.Import(tt.path, lib)
			if err == nil {
				t.Fatalf("Import accepted %s", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to name %q", err, tt.want)
			}
			if got := len(lib.Notes()) + len(lib.Cards()); got != 0 {
				t.Errorf("a failed import left %d entities behind", got)
			}
		})
	}
}

func blockTexts(note core.Note) []string {
	texts := make([]string, len(note.Blocks))
	for i, block := range note.Blocks {
		texts[i] = block.Text
	}
	return texts
}

func prompts(t *testing.T, lib *store.Library) []string {
	t.Helper()
	var out []string
	for _, card := range lib.Cards() {
		prompt, _, _ := cardFaces(t, lib, card)
		out = append(out, prompt)
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
