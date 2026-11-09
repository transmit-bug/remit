package store

import (
	"github.com/transmit-bug/remit/internal/core"
)

// A library announces what happens to it through one handler and any number of
// subscribers. The handler is the window's — it runs on the writing thread so
// the frame that shows the write can be retaken — and a subscriber is a
// listener like the agent API's event stream, which reads at its own pace on a
// goroutine of its own. Neither ever waits for the other.

// subscriptionBuffer is how many events a subscriber may fall behind before it
// is dropped. A stream that stops reading loses its place rather than the
// library's ability to write.
const subscriptionBuffer = 64

// Event is something that happened to the library, which an extension may want
// to see. It carries an ID and nothing else: an extension reads the rest
// through the agent API.
type Event struct {
	Kind string
	ID   core.ID
}

const (
	// EventNoteSaved fires after a note's file is written.
	EventNoteSaved = "note.saved"
	// EventCardCreated fires after a card file is written.
	EventCardCreated = "card.created"
	// EventReviewRecorded fires after a review enters the log.
	EventReviewRecorded = "review.recorded"
	// EventReadingRecorded fires after a reading session enters the log.
	EventReadingRecorded = "reading.recorded"
	// EventReadingQueued fires after a note joins the reading queue.
	EventReadingQueued = "reading.queued"
	// EventReadingDropped fires after a note leaves the reading queue.
	EventReadingDropped = "reading.dropped"
)

// subscription is one subscriber's half of the fan-out: a bounded queue the
// writer hands events to, and the goroutine that calls the subscriber in the
// order the library announced them.
type subscription struct {
	events chan Event
	closed bool
}

// SetEventHandler registers the function told about events, replacing any
// previous one. The handler runs on the writing thread, so it must not block.
// Subscribers registered with Subscribe are told beside it, and they are not
// replaced by it.
func (l *Library) SetEventHandler(fn func(Event)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onEvent = fn
}

// Subscribe registers fn to be told about every event from now on, and returns
// the function that stops it. It is the additive half of SetEventHandler: the
// window's handler and an event stream listen at the same time.
//
// Nothing is replayed, so a subscriber hears what happens after it subscribes.
// fn runs on its own goroutine and may block; a subscriber that lets its queue
// fill is dropped instead of holding up the write it fell behind.
func (l *Library) Subscribe(fn func(Event)) (cancel func()) {
	if fn == nil {
		return func() {}
	}
	sub := &subscription{events: make(chan Event, subscriptionBuffer)}
	l.mu.Lock()
	l.subscribers[sub] = struct{}{}
	l.mu.Unlock()

	go func() {
		for event := range sub.events {
			fn(event)
		}
	}()
	return func() { l.unsubscribe(sub) }
}

// unsubscribe ends one subscription. Stopping one that was dropped, or stopping
// it twice, does nothing.
func (l *Library) unsubscribe(sub *subscription) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dropLocked(sub)
}

// dropLocked ends a subscription and takes it out of the fan-out, assuming the
// lock is held.
func (l *Library) dropLocked(sub *subscription) {
	if sub.closed {
		return
	}
	sub.closed = true
	close(sub.events)
	delete(l.subscribers, sub)
}

// closeSubscribers ends every subscription, so nothing outlives the library.
func (l *Library) closeSubscribers() {
	for sub := range l.subscribers {
		l.dropLocked(sub)
	}
}

// emit tells the handler and every subscriber about an event. It runs on the
// writing thread with the library's lock held, so it never waits for a
// listener: the handler is called where it stands, and a subscriber whose queue
// is full is dropped.
func (l *Library) emit(kind string, id core.ID) {
	event := Event{Kind: kind, ID: id}
	if l.onEvent != nil {
		l.onEvent(event)
	}
	for sub := range l.subscribers {
		select {
		case sub.events <- event:
		default:
			// The subscriber is not keeping up. A write never waits for one, so
			// it loses the rest of the stream rather than the library its write.
			l.dropLocked(sub)
		}
	}
}
