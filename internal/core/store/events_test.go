package store_test

import (
	"testing"
	"time"

	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/core/store"
)

var eventNow = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

// A subscriber is additive with the window's handler: one write reaches the
// handler a frame is retaken from and every stream that asked, and cancelling
// one stream leaves the other listening.
func TestSubscribersHearWhatTheHandlerHears(t *testing.T) {
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return eventNow }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()

	first := make(chan store.Event, 4)
	second := make(chan store.Event, 4)
	cancelFirst := lib.Subscribe(func(ev store.Event) { first <- ev })
	defer cancelFirst()
	cancelSecond := lib.Subscribe(func(ev store.Event) { second <- ev })
	defer cancelSecond()

	var handled []string
	lib.SetEventHandler(func(ev store.Event) { handled = append(handled, ev.Kind) })

	note, err := lib.SaveNote(core.Note{Title: "Biology", Blocks: []core.Block{{Text: "q :: a"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}

	for name, events := range map[string]chan store.Event{"first": first, "second": second} {
		select {
		case ev := <-events:
			if ev.Kind != store.EventNoteSaved || ev.ID != note.ID {
				t.Errorf("%s subscriber heard %+v, want the saved note", name, ev)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s subscriber heard nothing", name)
		}
	}
	if len(handled) != 1 || handled[0] != store.EventNoteSaved {
		t.Errorf("the handler was told %v, want the saved note", handled)
	}

	cancelFirst()
	if _, err := lib.SaveNote(core.Note{Title: "Genetics", Blocks: []core.Block{{Text: "q :: a"}}}); err != nil {
		t.Fatalf("save note: %v", err)
	}
	select {
	case ev := <-second:
		if ev.Kind != store.EventNoteSaved {
			t.Errorf("the remaining subscriber heard %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling one subscriber silenced the other")
	}
	select {
	case ev := <-first:
		t.Errorf("a cancelled subscriber heard %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
}

// A subscriber that stops reading loses the stream, never the write: the
// library finishes every save while the listener sits on its first event.
func TestASlowSubscriberDoesNotHoldUpTheWriter(t *testing.T) {
	lib, err := store.Create(t.TempDir(), store.Options{Now: func() time.Time { return eventNow }})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer lib.Close()

	stalled := make(chan struct{})
	cancel := lib.Subscribe(func(store.Event) { <-stalled })
	defer func() {
		close(stalled)
		cancel()
	}()

	// Well past what one subscription buffers: if the library waited for the
	// listener, the write after the queue filled would never return.
	const writes = 200
	done := make(chan error, 1)
	go func() {
		for i := 0; i < writes; i++ {
			if _, err := lib.SaveNote(core.Note{Title: "note", Blocks: []core.Block{{Text: "a paragraph"}}}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("save: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("a subscriber that stopped reading held up the writer")
	}
}
