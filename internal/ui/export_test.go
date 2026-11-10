package ui

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/core/transfer"
	"github.com/transmit-bug/remit/locales"
)

// exportDialog opens the export dialog the way a user reaches it: the palette's
// export command, found by typing its name.
func exportDialog(t *testing.T, tt *ui.Tester, msgs locales.Messages) {
	t.Helper()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Type(msgs.ExportTitle())
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
}

// The export dialog writes what it offers and says what it wrote: a folder of
// Markdown files for the whole library, one per note, with the report on the
// window where the action was.
func TestExportWritesAndReports(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	out := filepath.Join(t.TempDir(), "md")

	folders := 0
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		folders++
		return out, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)
	if !tt.HasText(msgs.ExportTitle()) {
		t.Fatalf("the export dialog did not open; texts %q", tt.Texts())
	}
	for _, format := range []string{msgs.ExportMarkdown(), msgs.ExportArchive(), msgs.ExportCards()} {
		if !tt.HasText(format) {
			t.Errorf("the dialog offers no %q format; texts %q", format, tt.Texts())
		}
	}

	if err := tt.Click(msgs.ExportRun()); err != nil {
		t.Fatalf("the dialog has no export button: %v", err)
	}
	tt.Frame()

	if folders != 1 {
		t.Fatalf("the folder picker ran %d times", folders)
	}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != 3 {
		t.Fatalf("the export wrote %d files (%v), want 3", len(entries), err)
	}
	if !tt.HasText(msgs.ExportReport(3, 0)) {
		t.Errorf("the export is not reported; texts %q", tt.Texts())
	}
	if _, open := tt.Find(msgs.ExportTitle()); open {
		t.Error("the dialog stayed open after the export")
	}
	tt.Advance(5 * time.Second) // past the toast
	if tt.FramePending() {
		t.Error("view requests frames after the window should settle")
	}
}

// The library archive is one file the library reads back: transfer.Write packs
// it, and transfer.Read unpacks the same sources an import would open.
func TestExportWritesTheLibraryArchive(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	out := t.TempDir()
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		return out, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)
	if err := tt.Click(msgs.ExportArchive()); err != nil {
		t.Fatalf("the archive format is not clickable: %v", err)
	}
	tt.Frame()
	if err := tt.Click(msgs.ExportRun()); err != nil {
		t.Fatalf("the dialog has no export button: %v", err)
	}
	tt.Frame()

	name := archiveName(dir)
	path := filepath.Join(out, name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("the export wrote no archive: %v", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	unpacked := filepath.Join(t.TempDir(), "unpacked")
	if err := transfer.Read(file, info.Size(), unpacked); err != nil {
		t.Fatalf("the archive does not read back: %v", err)
	}
	if _, err := os.Stat(filepath.Join(unpacked, "remit.json")); err != nil {
		t.Errorf("the archive holds no library: %v", err)
	}
	if !tt.HasText(msgs.ExportArchiveReport(name)) {
		t.Errorf("the archive is not reported; texts %q", tt.Texts())
	}
}

// The card CSV is one file the card table reads back: a header naming every
// column, and one line per card, with what the card asks and the schedule it is
// on.
func TestExportWritesTheCardCSV(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seed := seedLibrary(t, dir)
	out := t.TempDir()
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		return out, nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)
	if err := tt.Click(msgs.ExportCards()); err != nil {
		t.Fatalf("the CSV format is not clickable: %v", err)
	}
	tt.Frame()
	if err := tt.Click(msgs.ExportRun()); err != nil {
		t.Fatalf("the dialog has no export button: %v", err)
	}
	tt.Frame()

	file, err := os.Open(filepath.Join(out, "cards.csv"))
	if err != nil {
		t.Fatalf("the export wrote no card CSV: %v", err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("the CSV does not read back: %v", err)
	}
	// Four cards in the seeded library, under the header.
	if len(records) != 5 {
		t.Fatalf("the CSV holds %d lines, want a header and 4 cards: %q", len(records), records)
	}
	if got, want := strings.Join(records[0], ","), strings.Join(cardCSVHeader, ","); got != want {
		t.Errorf("header = %q, want %q", got, want)
	}
	seen := map[string]bool{}
	for _, record := range records[1:] {
		if len(record) != len(cardCSVHeader) {
			t.Fatalf("line %q has %d columns, want %d", record, len(record), len(cardCSVHeader))
		}
		if !strings.HasPrefix(record[0], "card_") {
			t.Errorf("line %q names no card", record)
		}
		if record[1] == "" || record[4] == "" || record[5] == "" {
			t.Errorf("line %q has no note, no prompt or no answer", record)
		}
		seen[record[0]] = true
	}
	for _, card := range []core.Card{seed.spacing, seed.frozen, seed.testing, seed.pending} {
		if !seen[string(card.ID)] {
			t.Errorf("the CSV is missing card %s", card.ID)
		}
	}
	if !tt.HasText(msgs.ExportCardsReport("cards.csv", 4)) {
		t.Errorf("the export is not reported; texts %q", tt.Texts())
	}
}

// The CSV survives the fields that CSV separates on: a comma, a quote and a line
// break each come back as they went in, and a quote is doubled as the format
// asks.
func TestCardCSVEscapesWhatItCarries(t *testing.T) {
	rows := []cardCSVRow{{
		ID:       "card_00000000000000000000000000000000",
		Note:     "A note, with a comma",
		Block:    "block_00000000000000000000000000000000",
		Type:     "Basic",
		Prompt:   `What does "quoting" cost?`,
		Answer:   "A line,\nthen another",
		Status:   core.StatusReview,
		Due:      time.Date(2026, 11, 10, 7, 0, 0, 0, time.UTC),
		Interval: 12,
		Lapses:   3,
	}}
	var buf bytes.Buffer
	if err := writeCardCSV(&buf, rows); err != nil {
		t.Fatalf("writeCardCSV: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, `"What does ""quoting"" cost?"`) {
		t.Errorf("the quote is not escaped: %q", got)
	}
	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("the CSV does not read back: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("read back %d lines, want the header and one card: %q", len(records), records)
	}
	card := records[1]
	if card[1] != rows[0].Note || card[4] != rows[0].Prompt || card[5] != rows[0].Answer {
		t.Errorf("a field did not survive the round trip: %q", card)
	}
	if card[7] != "2026-11-10T07:00:00Z" {
		t.Errorf("due = %q, want the RFC 3339 stamp", card[7])
	}
}

// A library with notes but no cards offers the notes and refuses the CSV, with
// the reason beside the choice rather than a control that does nothing.
func TestExportRefusesTheCSVWithoutCards(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create a library: %v", err)
	}
	if _, err := lib.CreateNote("A note with no card"); err != nil {
		t.Fatalf("save a note: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	asked := 0
	a := NewApp(Options{LibraryDir: dir, ChooseExportFolder: func(string) (string, error) {
		asked++
		return t.TempDir(), nil
	}})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)

	if !tt.HasText(msgs.ExportNoCards()) {
		t.Fatalf("the CSV is not refused in words; texts %q", tt.Texts())
	}
	info, ok := tt.Access(msgs.ExportCards())
	if !ok {
		t.Fatalf("the CSV choice is not on the window; texts %q", tt.Texts())
	}
	if !info.Disabled {
		t.Error("the CSV choice is not disabled")
	}
	// A disabled radio takes no click; if the click lands anywhere, the choice
	// must not move.
	_ = tt.Click(msgs.ExportCards())
	tt.Frame()
	if a.export.format != exportMarkdown {
		t.Errorf("a disabled choice was chosen: %v", a.export.format)
	}
	if asked != 0 {
		t.Errorf("the folder picker ran for a refused format")
	}
}

// Every format the dialog offers is on the window at its smallest size: a choice
// laid out under the panel is a choice nobody can take.
func TestExportChoicesFitTheSmallestWindow(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	seedLibrary(t, dir)
	a := NewApp(Options{LibraryDir: dir})

	tt := ui.NewTester(a.View, 720, 520)
	settle(tt)
	exportDialog(t, tt, msgs)
	for _, choice := range []string{msgs.ExportMarkdown(), msgs.ExportArchive(), msgs.ExportCards()} {
		assertControl(t, tt, 720, 520, choice)
	}
	assertControl(t, tt, 720, 520, msgs.ExportRun())
}

// An empty library offers nothing to export, and every format says why.
func TestExportDisabledOnAnEmptyLibrary(t *testing.T) {
	msgs := locales.In("en")
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create an empty library: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close the empty library: %v", err)
	}
	a := NewApp(Options{LibraryDir: dir})

	tt := ui.NewTester(a.View, 1024, 720)
	settle(tt)
	exportDialog(t, tt, msgs)
	if !tt.HasText(msgs.ExportEmptyLibrary()) {
		t.Fatalf("the dialog does not say why it is disabled; texts %q", tt.Texts())
	}
	for _, choice := range []string{msgs.ExportMarkdown(), msgs.ExportArchive()} {
		info, ok := tt.Access(choice)
		if !ok {
			t.Fatalf("the %q choice is not on the window; texts %q", choice, tt.Texts())
		}
		if !info.Disabled {
			t.Errorf("the %q choice is live on an empty library", choice)
		}
	}
	// The CSV has its own count and its own reason: a library of notes and no
	// cards says that, not that it holds no notes.
	if !tt.HasText(msgs.ExportNoCards()) {
		t.Errorf("the CSV has no reason on an empty library; texts %q", tt.Texts())
	}
}
