package store_test

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/schedule"
	"github.com/transmit-bug/remit/internal/core/store"
)

func TestEnabledExtensionsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	lib, err := store.Create(dir, store.Options{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := lib.EnabledExtensions(); len(got) != 0 {
		t.Errorf("a new library enables %v", got)
	}
	if err := lib.SetEnabledExtensions([]string{"linker", "reader"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := lib.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := store.Open(dir, store.Options{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reopened.Close()
	got := reopened.EnabledExtensions()
	if len(got) != 2 || got[0] != "linker" || got[1] != "reader" {
		t.Errorf("enabled = %v after reopen", got)
	}
}

func TestEventsFireOnWrites(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()

	var kinds []string
	lib.SetEventHandler(func(e store.Event) { kinds = append(kinds, e.Kind) })

	note, err := lib.SaveNote(core.Note{Title: "n", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	card, err := lib.CreateCard(note.Blocks[0].ID)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	if err := lib.Review(card.ID, schedule.Good, now); err != nil {
		t.Fatalf("review: %v", err)
	}

	want := []string{store.EventNoteSaved, store.EventCardCreated, store.EventReviewRecorded}
	if len(kinds) != len(want) {
		t.Fatalf("events = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("event %d = %q, want %q", i, kinds[i], want[i])
		}
	}
}
