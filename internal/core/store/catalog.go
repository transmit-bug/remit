package store

import (
	"sort"
	"time"

	"github.com/transmit-bug/remit/internal/core"
)

// The catalog is the index as the library reads it while it is open: one entry
// per card, plus the queue in the order the index puts it. It is a cache of
// derived state, never a second source of truth: it is loaded from the index at
// open and every write updates it after the index commit, so a failure leaves
// the index and the catalog agreeing with the files.

// storeCard puts a card into the catalog, keeping the queue order in step one
// card at a time. When the order is already known to be stale it is left alone;
// the next read rebuilds it.
func (l *Library) storeCard(meta *cardMeta) {
	old := l.cards[meta.card.ID]
	if l.dueOK {
		if old != nil {
			l.dropDue(old)
		}
		if l.dueOK && eligible(meta) {
			l.insertDue(meta)
		}
	}
	l.cards[meta.card.ID] = meta
	if old == nil {
		l.createdOK = false
	}
}

// removeCard takes a card out of the catalog.
func (l *Library) removeCard(id core.ID) {
	if old, ok := l.cards[id]; ok && l.dueOK {
		l.dropDue(old)
	}
	delete(l.cards, id)
	l.createdOK = false
}

// insertDue places a card in the queue order.
func (l *Library) insertDue(meta *cardMeta) {
	entry := dueEntry{at: dueNano(meta), id: meta.card.ID}
	at := sort.Search(len(l.byDue), func(i int) bool { return !dueLess(l.byDue[i], entry) })
	l.byDue = append(l.byDue, dueEntry{})
	copy(l.byDue[at+1:], l.byDue[at:])
	l.byDue[at] = entry
}

// dropDue takes a card out of the queue order. An entry that is not where the
// catalog says it should be means the two have drifted, which is repaired by
// rebuilding the order rather than by guessing.
func (l *Library) dropDue(meta *cardMeta) {
	entry := dueEntry{at: dueNano(meta), id: meta.card.ID}
	at := sort.Search(len(l.byDue), func(i int) bool { return !dueLess(l.byDue[i], entry) })
	if at >= len(l.byDue) || l.byDue[at] != entry {
		l.dueOK = false
		return
	}
	l.byDue = append(l.byDue[:at], l.byDue[at+1:]...)
}

// ensureDue rebuilds the queue order when a bulk change — a note save that
// moved many blocks, or an undo — left it stale.
func (l *Library) ensureDue() {
	if l.dueOK {
		return
	}
	if cap(l.byDue) < len(l.cards) {
		l.byDue = make([]dueEntry, 0, len(l.cards))
	}
	for _, meta := range l.cards {
		if eligible(meta) {
			l.byDue = append(l.byDue, dueEntry{at: dueNano(meta), id: meta.card.ID})
		}
	}
	sort.Slice(l.byDue, func(i, j int) bool { return dueLess(l.byDue[i], l.byDue[j]) })
	l.dueOK = true
}

// dueBefore is how many of the queue's cards are due by now.
func (l *Library) dueBefore(now time.Time) int {
	l.ensureDue()
	at := now.UnixNano()
	return sort.Search(len(l.byDue), func(i int) bool { return l.byDue[i].at > at })
}

// ensureCreated rebuilds the oldest-first order every listing that is not the
// queue uses.
func (l *Library) ensureCreated() {
	if l.createdOK {
		return
	}
	l.byCreated = make([]core.ID, 0, len(l.cards))
	for id := range l.cards {
		l.byCreated = append(l.byCreated, id)
	}
	sort.Slice(l.byCreated, func(i, j int) bool {
		a, b := l.cards[l.byCreated[i]], l.cards[l.byCreated[j]]
		if !a.created.Equal(b.created) {
			return a.created.Before(b.created)
		}
		return a.card.ID < b.card.ID
	})
	l.createdOK = true
}

// bulkQueueUpdate is how many cards one write may touch before keeping the
// queue order in step card by card costs more than re-sorting it once.
const bulkQueueUpdate = 8

// markQueueStale forgets the queue order, for a change that touched too many
// cards for one-at-a-time updates to be cheaper than a rebuild.
func (l *Library) markQueueStale() {
	l.dueOK = false
}
