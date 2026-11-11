package cards

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/transmit-bug/remit/internal/core"
)

// DefaultBound is how many texts one generation of a cache holds: a long
// review's cards with the page on screen beside them, and two generations of
// those is a few megabytes of field maps at worst.
const DefaultBound = 1024

// Cache is what card plugins answered, kept across the resolvers that ask. A
// resolver is cheap and is built again whenever a library is retaken; the
// answers are not, because asking a plugin is a call into another runtime or
// process and the same paragraph is asked again on every frame it is on screen.
//
// An answer belongs to an identity — the plugin that derived it and the type
// table it was asked with — and to the block's text, so a resolver built for the
// same identity reads what was already asked, where an edited type or a switched
// plugin hits nothing. A failure is an answer too: a plugin that throws is asked
// once, not once a frame (docs/specs/plugin-system.md P4).
//
// One cache is safe for every goroutine that asks it. The agent API's source is
// reached by every HTTP handler at once, and it shares its cache with every
// other resolver built for the same plugin and types.
type Cache struct {
	mu    sync.Mutex
	bound int
	// current is the generation being written, older the one before it: a read
	// looks in both, a write only in current. A full current is not emptied — it
	// becomes older and a fresh one starts — so what the screen just asked keeps
	// answering, and only what two generations back asks again.
	current, older map[key]answer
	// deriving holds the derivations in flight, one per key, so a text being
	// asked is waited for rather than asked a second time — by this resolver or
	// the one beside it.
	deriving map[key]*derivation
}

// key is one derivation's identity, digested so that a long paragraph is not
// held as a key and two keys compare in constant time: the plugin and type table
// that answer, and the block's text.
type key struct {
	identity [sha256.Size]byte
	text     [sha256.Size]byte
}

// answer is the cached reply for one text.
type answer struct {
	cards []core.Derived
	ok    bool
}

// derivation is one key's plugin call in flight; done closes once its answer is
// in the cache.
type derivation struct {
	done  chan struct{}
	cards []core.Derived
	ok    bool
}

// NewCache answers a cache holding at most bound texts in one generation, and
// the same again in the one before it. A bound of zero or less is DefaultBound.
func NewCache(bound int) *Cache {
	if bound <= 0 {
		bound = DefaultBound
	}
	return &Cache{
		bound:    bound,
		current:  map[key]answer{},
		older:    map[key]answer{},
		deriving: map[key]*derivation{},
	}
}

// Len is how many answers the cache holds, both generations together: the one
// being written and the one before it. It is what a test asserts the bound
// through; no load path in the app reads it.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.current) + len(c.older)
}

// derive answers what run derives for k, asking once: a key already in flight is
// waited for, and what comes back — a failure too — is remembered for the next
// ask.
func (c *Cache) derive(k key, run func() ([]core.Derived, bool)) (derived []core.Derived, ok bool) {
	c.mu.Lock()
	if got, hit := c.answers(k); hit {
		c.mu.Unlock()
		return got.cards, got.ok
	}
	if wait, busy := c.deriving[k]; busy {
		c.mu.Unlock()
		<-wait.done
		return wait.cards, wait.ok
	}
	wait := &derivation{done: make(chan struct{})}
	c.deriving[k] = wait
	c.mu.Unlock()

	// The wait is settled however the plugin call ends — a panic included — so a
	// waiter is never left blocked on a text nobody is asking.
	defer func() { c.remember(k, wait, derived, ok) }()
	return run()
}

// answers is the cache's answer for k: the generation being written first, then
// the one before it — an answer that fell out of the current one is still right.
func (c *Cache) answers(k key) (answer, bool) {
	if got, hit := c.current[k]; hit {
		return got, true
	}
	got, hit := c.older[k]
	return got, hit
}

// remember files an answer, ends its derivation and settles its waiters. A full
// generation becomes the previous one and a fresh one starts: what is dropped is
// the generation two back, never the answers the page on screen just asked.
func (c *Cache) remember(k key, wait *derivation, derived []core.Derived, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	wait.cards, wait.ok = derived, ok
	if len(c.current) >= c.bound {
		c.older = c.current
		c.current = make(map[key]answer, c.bound)
	}
	c.current[k] = answer{cards: derived, ok: ok}
	delete(c.deriving, k)
	// Closed under the lock so that one answer's bookkeeping is one critical
	// section; a waiter reads wait's fields only after this close.
	close(wait.done)
}

// identity digests what a derivation depends on: the plugin that answers, and
// the type table it is asked with — each type's id, version and field names. The
// templates are not in it: a template decides how a card renders, not what a
// plugin answers, so an edited template re-renders from the same answers.
func identity(producer core.Producer, types []core.Type) [sha256.Size]byte {
	h := sha256.New()
	field(h, producer.ID)
	field(h, producer.Version)
	table := slices.Clone(types)
	slices.SortFunc(table, func(a, b core.Type) int { return strings.Compare(string(a.ID), string(b.ID)) })
	for _, typ := range table {
		field(h, string(typ.ID))
		field(h, strconv.Itoa(typ.Version))
		for _, name := range typ.Fields {
			field(h, name)
		}
		field(h, "") // the field list's end, so a type cannot run into the next
	}
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

// field writes one value with its length, so two type tables that differ only in
// where a name ends cannot digest alike.
func field(h hash.Hash, s string) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(s)))
	h.Write(n[:])
	h.Write([]byte(s))
}
